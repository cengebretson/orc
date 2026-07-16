package contextpressure

import "testing"

func TestEvaluateThresholdBoundaries(t *testing.T) {
	thresholds := Thresholds{Green: 40, Yellow: 70, Red: 90}
	tests := []struct {
		name  string
		used  uint64
		level Level
	}{
		{name: "below green", used: 39, level: LevelNeutral},
		{name: "green boundary", used: 40, level: LevelGreen},
		{name: "yellow boundary", used: 70, level: LevelYellow},
		{name: "red boundary", used: 90, level: LevelRed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.used, 100, thresholds)
			if !got.Observed || !got.Available || got.Percent != tt.used || got.Level != tt.level {
				t.Fatalf("Evaluate(%d, 100) = %+v, want percent %d level %q", tt.used, got, tt.used, tt.level)
			}
		})
	}
}

func TestEvaluateUnknownLimitIsUnavailable(t *testing.T) {
	got := Evaluate(42, 0, DefaultThresholds())
	if !got.Observed || got.Available || got.Level != LevelUnavailable || got.Label() != "n/a" {
		t.Fatalf("Evaluate(42, 0) = %+v, want observed unavailable pressure", got)
	}
}

func TestUnobservedPressureUsesDash(t *testing.T) {
	if got := (Pressure{}).Label(); got != "-" {
		t.Fatalf("zero Pressure label = %q, want -", got)
	}
}

func TestEvaluateAvoidsPercentageOverflow(t *testing.T) {
	got := Evaluate(^uint64(0), ^uint64(0), DefaultThresholds())
	if got.Percent != 100 {
		t.Fatalf("max uint ratio = %d%%, want 100%%", got.Percent)
	}
}
