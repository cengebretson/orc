package tmux

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cengebretson/orc/internal/agentdetect"
	"github.com/cengebretson/orc/internal/mux"
)

const idleDebounceObservations = 2

var observationDebounce = struct {
	sync.Mutex
	idle map[string]int
}{idle: make(map[string]int)}

// ObserveFallback adds presentation-only title and bounded-screen inference.
// It never changes hook-owned lifecycle metadata.
func ObserveFallback(root string, panes []mux.Pane) ([]mux.Pane, error) {
	result := append([]mux.Pane(nil), panes...)
	manifests := make(map[string]*agentdetect.Manifest)
	for i := range result {
		pane := &result[i]
		if pane.Backend != "tmux" || !pane.Agent || pane.LifecycleSource == "hook" {
			continue
		}
		// tmux-attention says a turn is running. That is the agent reporting
		// through its own hook rather than a guess about a picture, so it beats
		// both inference tiers and skips the screen capture entirely. It stays
		// an observation because Orc cannot verify who wrote it.
		if pane.ContextActive {
			before := *pane
			applyObservation(pane, agentdetect.Result{
				Lifecycle: mux.LifecycleWorking, Source: mux.SourceContext,
			}, time.Now())
			if observationChanged(before, *pane) {
				if err := publishObservation(*pane); err != nil {
					return nil, err
				}
			}
			continue
		}
		engine := strings.ToLower(firstValue(pane.ProviderEngine, pane.Engine))
		if engine != "codex" && engine != "claude" {
			before := *pane
			applyObservation(pane, agentdetect.Result{Lifecycle: mux.LifecycleUnknown, Source: "title"}, time.Now())
			if observationChanged(before, *pane) {
				if err := publishObservation(*pane); err != nil {
					return nil, err
				}
			}
			continue
		}
		manifest := manifests[engine]
		if manifest == nil {
			loaded, err := agentdetect.Load(root, engine)
			if err != nil {
				return nil, err
			}
			manifest = loaded
			manifests[engine] = manifest
		}
		detection := manifest.Detect("title", pane.Title)
		if pane.Title == "" || detection.Lifecycle == mux.LifecycleUnknown {
			screen, err := captureObservationScreen(pane.ID, manifest.MaxScreenLines())
			if err == nil {
				detection = manifest.Detect("screen", screen)
			} else {
				detection = agentdetect.Result{Lifecycle: mux.LifecycleUnknown, Source: "screen"}
			}
		}
		before := *pane
		detection = debounceObservation(*pane, detection)
		applyObservation(pane, detection, time.Now())
		if observationChanged(before, *pane) {
			if err := publishObservation(*pane); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func observationChanged(before, after mux.Pane) bool {
	return before.ObservedLifecycle != after.ObservedLifecycle ||
		before.ObservationSource != after.ObservationSource ||
		before.ObservationRule != after.ObservationRule ||
		before.ObservationSince != after.ObservationSince ||
		before.Attention != after.Attention ||
		before.AttentionSource != after.AttentionSource ||
		before.AttentionSince != after.AttentionSince
}

func captureObservationScreen(pane string, lines int) (string, error) {
	if lines <= 0 {
		return "", nil
	}
	out, err := newCommand("tmux", "capture-pane", "-p", "-t", pane, "-S", "-"+strconv.Itoa(lines)).Output()
	if err != nil {
		return "", fmt.Errorf("capture fallback region for pane %s: %w", pane, err)
	}
	return string(out), nil
}

func debounceObservation(pane mux.Pane, detection agentdetect.Result) agentdetect.Result {
	key := pane.ID + "\x00" + pane.AgentInstance
	observationDebounce.Lock()
	defer observationDebounce.Unlock()
	if pane.ObservedLifecycle == mux.LifecycleWorking && detection.Lifecycle == mux.LifecycleIdle {
		observationDebounce.idle[key]++
		if observationDebounce.idle[key] < idleDebounceObservations {
			return agentdetect.Result{
				Lifecycle: mux.LifecycleWorking, Source: pane.ObservationSource,
				RuleID: pane.ObservationRule, Title: detection.Title,
			}
		}
	}
	delete(observationDebounce.idle, key)
	return detection
}

func applyObservation(pane *mux.Pane, detection agentdetect.Result, now time.Time) {
	if detection.Lifecycle == "" {
		detection.Lifecycle = mux.LifecycleUnknown
	}
	changed := pane.ObservedLifecycle != detection.Lifecycle || pane.ObservationSource != detection.Source || pane.ObservationRule != detection.RuleID
	pane.ObservedLifecycle = detection.Lifecycle
	pane.ObservationSource = detection.Source
	pane.ObservationRule = detection.RuleID
	pane.DisplayTitle = detection.Title
	if changed || pane.ObservationSince == 0 {
		pane.ObservationSince = now.Unix()
	}
	if pane.AttentionSource != "hook" {
		pane.Attention = detection.Attention
		pane.AttentionSource = detection.Source
		if detection.Attention == "" {
			pane.AttentionSince = 0
		} else if changed || pane.AttentionSince == 0 {
			pane.AttentionSince = now.Unix()
		}
	}
}

func publishObservation(pane mux.Pane) error {
	updates := [][2]string{
		{"@orc_agent_observed_state", pane.ObservedLifecycle},
		{"@orc_agent_observed_source", pane.ObservationSource},
		{"@orc_agent_observed_since", strconv.FormatInt(pane.ObservationSince, 10)},
		{"@orc_agent_observed_rule", pane.ObservationRule},
	}
	if pane.AttentionSource != "hook" {
		updates = append(updates, attentionUpdates(
			pane.Attention,
			optionalTimestamp(pane.AttentionSince),
			pane.AttentionSource,
		)...)
	}
	for _, update := range updates {
		if err := setPaneOption(pane.ID, update[0], update[1]); err != nil {
			return fmt.Errorf("publish fallback observation: %w", err)
		}
	}
	return nil
}

func optionalTimestamp(value int64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
