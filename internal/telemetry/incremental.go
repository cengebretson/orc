package telemetry

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

const (
	defaultRefreshBytes    = 32 * 1024 * 1024
	defaultRefreshDuration = 250 * time.Millisecond
	defaultHeadBytes       = 16 * 1024
	defaultTailBytes       = 256 * 1024
)

type discoverConfig struct {
	maxRefreshBytes int64
	maxRefreshTime  time.Duration
	headBytes       int64
	tailBytes       int64
	now             func() time.Time
}

func defaultDiscoverConfig() discoverConfig {
	return discoverConfig{
		maxRefreshBytes: defaultRefreshBytes,
		maxRefreshTime:  defaultRefreshDuration,
		headBytes:       defaultHeadBytes,
		tailBytes:       defaultTailBytes,
		now:             time.Now,
	}
}

type refreshBudget struct {
	remaining int64
	deadline  time.Time
	now       func() time.Time
	read      int64
}

func newRefreshBudget(config discoverConfig) *refreshBudget {
	now := config.now
	if now == nil {
		now = time.Now
	}
	return &refreshBudget{
		remaining: config.maxRefreshBytes,
		deadline:  now().Add(config.maxRefreshTime),
		now:       now,
	}
}

func (b *refreshBudget) available(requested int64) int64 {
	if requested <= 0 || b.remaining <= 0 || !b.now().Before(b.deadline) {
		return 0
	}
	if requested < b.remaining {
		return requested
	}
	return b.remaining
}

func (b *refreshBudget) consume(size int64) {
	b.read += size
	b.remaining -= size
	if b.remaining < 0 {
		b.remaining = 0
	}
}

type fileIdentity struct {
	device uint64
	inode  uint64
}

func identityFor(info os.FileInfo) fileIdentity {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileIdentity{}
	}
	return fileIdentity{device: uint64(stat.Dev), inode: uint64(stat.Ino)}
}

type scanResult struct {
	offset     int64
	discarding bool
	malformed  bool
}

type countingReader struct {
	reader io.Reader
	read   int64
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	r.read += int64(n)
	return n, err
}

// scanJSONLRange visits complete JSONL records within [start, end). Its offset
// always points to a safe restart boundary. Partial records are neither visited
// nor retained between refreshes.
func scanJSONLRange(
	path string,
	start int64,
	end int64,
	discarding bool,
	budget *refreshBudget,
	visit func([]byte) bool,
) (scanResult, error) {
	result := scanResult{offset: start, discarding: discarding}
	allowed := budget.available(end - start)
	if allowed == 0 {
		return result, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close() //nolint:errcheck
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return result, fmt.Errorf("seek JSONL: %w", err)
	}

	counter := &countingReader{reader: io.LimitReader(file, allowed)}
	reader := bufio.NewReader(counter)
	position := start
	if result.discarding {
		complete, err := discardJSONLRecord(reader)
		position = start + counter.read - int64(reader.Buffered())
		if complete {
			result.offset = position
			result.discarding = false
		}
		if err != nil && err != io.EOF {
			budget.consume(counter.read)
			return result, err
		}
		if !complete {
			result.offset = position
			budget.consume(counter.read)
			return result, nil
		}
	}

	for position < start+allowed && budget.now().Before(budget.deadline) {
		lineStart := position
		line, over, readErr := readLineCapped(reader, maxJSONLLineBytes)
		// bufio may prefetch several records. Only bytes no longer buffered
		// belong to the record just consumed and are safe cursor progress.
		position = start + counter.read - int64(reader.Buffered())
		complete := readErr == nil
		if complete {
			result.offset = position
			if !over && len(bytes.TrimSpace(line)) > 0 && !visit(line) {
				result.malformed = true
			}
		} else if over {
			// An oversized record can be discarded without retaining its body.
			result.offset = position
			result.discarding = true
		} else {
			// Retry an ordinary partial record from its beginning after append.
			result.offset = lineStart
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			budget.consume(counter.read)
			return result, readErr
		}
	}
	budget.consume(counter.read)
	return result, nil
}

func discardJSONLRecord(reader *bufio.Reader) (bool, error) {
	for {
		_, err := reader.ReadSlice('\n')
		switch err {
		case nil:
			return true, nil
		case bufio.ErrBufferFull:
			continue
		case io.EOF:
			return false, io.EOF
		default:
			return false, fmt.Errorf("discard JSONL: %w", err)
		}
	}
}
