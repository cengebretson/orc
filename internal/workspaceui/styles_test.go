package workspaceui

import (
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/charmbracelet/x/ansi"
)

func TestAssignWorkerAccentColorsGivesEachWorkerADistinctColor(t *testing.T) {
	allWorkers := []*workers.Worker{
		{ID: "default:ada"},
		{ID: "default:bob"},
		{ID: "default:brian"},
		{ID: "default:fred"},
		{ID: "default:zach"},
	}
	assignWorkerAccentColors(allWorkers)

	seen := map[string]string{}
	for _, w := range allWorkers {
		color := workerAccentColor(w.ID)
		if other, ok := seen[color]; ok {
			t.Fatalf("worker colors collide: %s and %s both got %s", w.ID, other, color)
		}
		seen[color] = w.ID
	}
}

func TestWorkerAccentColorIsStableAcrossCalls(t *testing.T) {
	allWorkers := []*workers.Worker{{ID: "default:ada"}, {ID: "default:bob"}}
	assignWorkerAccentColors(allWorkers)

	first := workerAccentColor("default:ada")
	for range 3 {
		if got := workerAccentColor("default:ada"); got != first {
			t.Fatalf("workerAccentColor(default:ada) = %q, want stable %q", got, first)
		}
	}
}

func TestWorkerAccentColorFallsBackForUnknownWorker(t *testing.T) {
	assignWorkerAccentColors(nil)
	if got := workerAccentColor(""); got != activeTheme.Palette.Overlay0 {
		t.Fatalf("empty worker ID = %q, want dim Overlay0", got)
	}
	if got := workerAccentColor("ghost-worker"); got == "" {
		t.Fatal("unknown worker ID should still get a non-empty fallback color")
	}
}

func TestSparklineScalesSamplesToBlockHeight(t *testing.T) {
	got := sparkline([]uint64{0, 50, 100})
	want := string([]rune{sparkBlocks[0], sparkBlocks[3], sparkBlocks[7]})
	if got != want {
		t.Fatalf("sparkline([0,50,100]) = %q, want %q", got, want)
	}
	if got := sparkline(nil); got != "" {
		t.Fatalf("sparkline(nil) = %q, want empty", got)
	}
}

func TestRenderContextSparklineNeedsAtLeastTwoSamples(t *testing.T) {
	pressure := contextpressure.Pressure{Observed: true, Available: true, Percent: 42, Level: contextpressure.LevelGreen}
	if got := renderContextSparkline(nil, pressure); got != "" {
		t.Fatalf("renderContextSparkline(nil) = %q, want empty", got)
	}
	if got := renderContextSparkline([]uint64{42}, pressure); got != "" {
		t.Fatalf("renderContextSparkline(single sample) = %q, want empty", got)
	}
	got := ansi.Strip(renderContextSparkline([]uint64{10, 42}, pressure))
	if !strings.HasSuffix(got, "42%") {
		t.Fatalf("renderContextSparkline should end with the current percentage: %q", got)
	}
}
