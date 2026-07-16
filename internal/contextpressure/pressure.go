// Package contextpressure classifies provider context usage for presentation.
// It does not mutate durable workflow state or control session lifecycle.
package contextpressure

import (
	"fmt"
	"math"
	"math/bits"
)

type Level string

const (
	LevelUnavailable Level = "unavailable"
	LevelNeutral     Level = "neutral"
	LevelGreen       Level = "green"
	LevelYellow      Level = "yellow"
	LevelRed         Level = "red"
)

type Thresholds struct {
	Green  int
	Yellow int
	Red    int
}

func DefaultThresholds() Thresholds {
	return Thresholds{Green: 0, Yellow: 70, Red: 90}
}

func (t Thresholds) Valid() bool {
	return t.Green >= 0 && t.Green < t.Yellow && t.Yellow < t.Red && t.Red <= 100
}

type Pressure struct {
	Observed  bool
	Available bool
	Percent   uint64
	Level     Level
}

func Evaluate(used, limit uint64, thresholds Thresholds) Pressure {
	if !thresholds.Valid() {
		thresholds = DefaultThresholds()
	}
	if limit == 0 {
		return Pressure{Observed: true, Level: LevelUnavailable}
	}
	percent := percentage(used, limit)
	level := LevelNeutral
	switch {
	case percent >= uint64(thresholds.Red):
		level = LevelRed
	case percent >= uint64(thresholds.Yellow):
		level = LevelYellow
	case percent >= uint64(thresholds.Green):
		level = LevelGreen
	}
	return Pressure{Observed: true, Available: true, Percent: percent, Level: level}
}

func (p Pressure) Label() string {
	if !p.Observed {
		return "-"
	}
	if !p.Available {
		return "n/a"
	}
	return fmt.Sprintf("%d%%", p.Percent)
}

func percentage(used, limit uint64) uint64 {
	whole := used / limit
	if whole > math.MaxUint64/100 {
		return math.MaxUint64
	}
	hi, lo := bits.Mul64(used%limit, 100)
	fraction, _ := bits.Div64(hi, lo, limit)
	return whole*100 + fraction
}
