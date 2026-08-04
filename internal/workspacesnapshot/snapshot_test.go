package workspacesnapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/sessionlist"
	"github.com/cengebretson/orc/internal/telemetry"
)

func TestLoadReturnsInvalidConfigErrorInsteadOfPanicking(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "orc.yaml"), []byte("unknown_key: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "unknown_key") {
		t.Fatalf("Load error = %v, want unknown-key configuration error", err)
	}
}

func TestBuildItemsJoinsRuntimeTelemetryAndContext(t *testing.T) {
	featureDir := filepath.Join("workspace", "features", "ORC-1")
	feature := &featurelist.Feature{FeatureDir: featureDir}
	lastActive := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	live := telemetry.Live{
		ProviderSessionID: "session-1",
		ContextUsed:       75,
		ContextLimit:      100,
		LastActive:        lastActive,
	}

	items := buildItems(
		[]*featurelist.Feature{feature},
		map[string]sessionlist.ManagedRuntime{filepath.Clean(featureDir): {Live: live, HasTelemetry: true}},
		contextpressure.DefaultThresholds(),
	)
	if len(items) != 1 || items[0].Feature != feature || !items[0].HasTelemetry {
		t.Fatalf("buildItems() = %+v, want one enriched item", items)
	}
	if items[0].Live.ProviderSessionID != "session-1" || items[0].Live.LastActive != lastActive {
		t.Fatalf("runtime telemetry = %+v, want session-1 at %s", items[0].Live, lastActive)
	}
	if !items[0].Context.Observed || !items[0].Context.Available || items[0].Context.Percent != 75 {
		t.Fatalf("context = %+v, want observed 75%%", items[0].Context)
	}
}

func TestBuildItemsPreservesFeatureWithoutTelemetry(t *testing.T) {
	feature := &featurelist.Feature{FeatureDir: "features/ORC-2"}
	items := buildItems([]*featurelist.Feature{feature}, nil, contextpressure.DefaultThresholds())
	if len(items) != 1 || items[0].Feature != feature || items[0].HasTelemetry || items[0].Context.Observed {
		t.Fatalf("buildItems() = %+v, want durable-only item", items)
	}
}
