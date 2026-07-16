package telemetry

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type transcriptSnapshot struct {
	identity     fileIdentity
	size         int64
	modified     time.Time
	offset       int64
	discarding   bool
	metadata     Live
	codexWorking bool
}

// Discoverer keeps metadata-only transcript snapshots between refreshes.
// It is safe for concurrent callers, though refreshes are serialized so they
// share one coherent byte and time budget.
type Discoverer struct {
	mu       sync.Mutex
	config   discoverConfig
	cache    map[string]transcriptSnapshot
	lastRead int64
}

// NewDiscoverer constructs a provider telemetry discoverer with bounded
// refresh work and process-local incremental transcript cursors.
func NewDiscoverer() *Discoverer {
	return newDiscoverer(defaultDiscoverConfig())
}

func newDiscoverer(config discoverConfig) *Discoverer {
	if config.maxRefreshBytes <= 0 {
		config.maxRefreshBytes = defaultRefreshBytes
	}
	if config.maxRefreshTime <= 0 {
		config.maxRefreshTime = defaultRefreshDuration
	}
	if config.headBytes <= 0 {
		config.headBytes = defaultHeadBytes
	}
	if config.tailBytes <= 0 {
		config.tailBytes = defaultTailBytes
	}
	if config.now == nil {
		config.now = time.Now
	}
	return &Discoverer{config: config, cache: make(map[string]transcriptSnapshot)}
}

var defaultDiscoverer = NewDiscoverer()

// Discover reads recent provider metadata without retaining prompt or response
// bodies. One total byte and time budget is shared across both providers.
func (d *Discoverer) Discover(home string) ([]Live, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	resolved, err := resolveHome(home)
	if err != nil {
		return nil, err
	}
	budget := newRefreshBudget(d.config)
	claude, err := d.discoverClaude(resolved, budget)
	if err != nil {
		return nil, err
	}
	codex, err := d.discoverCodex(resolved, budget)
	if err != nil {
		return nil, err
	}
	d.lastRead = budget.read

	all := append(claude, codex...)
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].LastActive.After(all[j].LastActive)
	})
	return all, nil
}

// DiscoverClaude reads Claude telemetry with this discoverer's cache.
func (d *Discoverer) DiscoverClaude(home string) ([]Live, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	resolved, err := resolveHome(home)
	if err != nil {
		return nil, err
	}
	budget := newRefreshBudget(d.config)
	out, err := d.discoverClaude(resolved, budget)
	d.lastRead = budget.read
	return out, err
}

// DiscoverCodex reads Codex telemetry with this discoverer's cache.
func (d *Discoverer) DiscoverCodex(home string) ([]Live, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	resolved, err := resolveHome(home)
	if err != nil {
		return nil, err
	}
	budget := newRefreshBudget(d.config)
	out, err := d.discoverCodex(resolved, budget)
	d.lastRead = budget.read
	return out, err
}

func resolveHome(home string) (string, error) {
	if home != "" {
		return home, nil
	}
	return os.UserHomeDir()
}

func (d *Discoverer) transcript(path string, engine string, budget *refreshBudget) (Live, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Live{}, false, err
	}
	identity := identityFor(info)
	cached, found := d.cache[path]
	unchanged := found && cached.identity == identity && info.Size() == cached.size &&
		info.ModTime().Equal(cached.modified)
	if unchanged {
		return cached.metadata, cached.codexWorking, nil
	}
	appendOnly := found && cached.identity == identity && info.Size() > cached.size &&
		!info.ModTime().Before(cached.modified)

	snapshot := transcriptSnapshot{identity: identity, size: info.Size(), modified: info.ModTime()}
	if appendOnly {
		snapshot = cached
		snapshot.size = info.Size()
		snapshot.modified = info.ModTime()
		result, scanErr := d.scanTranscript(path, engine, snapshot.offset, info.Size(), snapshot.discarding, budget, &snapshot)
		if scanErr != nil {
			return Live{}, false, scanErr
		}
		if result.malformed {
			// Do not commit state derived across a malformed append. Keep the
			// previous safe summary for this refresh and rebuild next time.
			delete(d.cache, path)
			return cached.metadata, cached.codexWorking, nil
		} else {
			snapshot.offset = result.offset
			snapshot.discarding = result.discarding
		}
	} else {
		if err := d.initializeTranscript(path, engine, info.Size(), budget, &snapshot); err != nil {
			return Live{}, false, err
		}
	}
	d.cache[path] = snapshot
	return snapshot.metadata, snapshot.codexWorking, nil
}

func (d *Discoverer) initializeTranscript(
	path string,
	engine string,
	size int64,
	budget *refreshBudget,
	snapshot *transcriptSnapshot,
) error {
	headEnd := min(size, d.config.headBytes)
	result, err := d.scanTranscript(path, engine, 0, headEnd, false, budget, snapshot)
	if err != nil {
		return err
	}
	snapshot.offset = result.offset
	snapshot.discarding = result.discarding
	if headEnd == size {
		return nil
	}

	tailStart := max(headEnd, size-d.config.tailBytes)
	tailDiscard := tailStart > result.offset
	result, err = d.scanTranscript(path, engine, tailStart, size, tailDiscard, budget, snapshot)
	if err != nil {
		return err
	}
	snapshot.offset = result.offset
	snapshot.discarding = result.discarding
	return nil
}

func (d *Discoverer) scanTranscript(
	path string,
	engine string,
	start int64,
	end int64,
	discarding bool,
	budget *refreshBudget,
	snapshot *transcriptSnapshot,
) (scanResult, error) {
	return scanJSONLRange(path, start, end, discarding, budget, func(line []byte) bool {
		switch engine {
		case "claude":
			return parseClaudeLine(line, &snapshot.metadata)
		case "codex":
			return parseCodexLine(line, &snapshot.metadata, &snapshot.codexWorking)
		default:
			return false
		}
	})
}

func (d *Discoverer) prune(root string, seen map[string]bool) {
	prefix := filepath.Clean(root) + string(os.PathSeparator)
	for path := range d.cache {
		if strings.HasPrefix(filepath.Clean(path), prefix) && !seen[path] {
			delete(d.cache, path)
		}
	}
}

func mergeTranscript(base Live, metadata Live, codexWorking bool) Live {
	if metadata.ProviderSessionID != "" {
		base.ProviderSessionID = metadata.ProviderSessionID
	}
	if metadata.CWD != "" {
		base.CWD = metadata.CWD
	}
	if metadata.Model != "" {
		base.Model = metadata.Model
	}
	if metadata.Effort != "" {
		base.Effort = metadata.Effort
	}
	if metadata.ContextUsed != 0 {
		base.ContextUsed = metadata.ContextUsed
	}
	if metadata.ContextLimit != 0 {
		base.ContextLimit = metadata.ContextLimit
	}
	if metadata.LastActive.After(base.LastActive) {
		base.LastActive = metadata.LastActive
	}
	if base.Engine == "codex" && codexWorking {
		base.State = "working"
	}
	return base
}

func sessionIDFromFilename(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for start := len(base) - 36; start >= 0; start-- {
		candidate := base[start : start+36]
		if isUUIDLike(candidate) {
			return candidate
		}
	}
	return ""
}
