package workspaceui

import (
	"testing"

	"github.com/cengebretson/orc/internal/workers"
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
