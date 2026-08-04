// Package agentdetect applies conservative, presentation-only terminal rules.
// Hook lifecycle metadata remains authoritative; detections from this package
// must never satisfy control waits or mutate durable workflow state.
package agentdetect

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	Version        = 1
	OverrideDir    = "agent-detection"
	MaxRegionLines = 80
)

//go:embed rules/v1/*.yaml
var embeddedRules embed.FS

type Manifest struct {
	Version int    `yaml:"version"`
	Engine  string `yaml:"engine"`
	Rules   []Rule `yaml:"rules"`
}

type Rule struct {
	ID          string `yaml:"id"`
	Source      string `yaml:"source"`
	Lifecycle   string `yaml:"lifecycle"`
	Attention   string `yaml:"attention,omitempty"`
	Priority    int    `yaml:"priority"`
	RegionLines int    `yaml:"region_lines,omitempty"`
	Pattern     string `yaml:"pattern"`
	matcher     *regexp.Regexp
}

type Result struct {
	Lifecycle string
	Attention string
	Source    string
	RuleID    string
	Priority  int
	Title     string
}

// Load reads the embedded versioned manifest, or a complete workspace-local
// replacement at agent-detection/v1/<engine>.yaml when present.
func Load(root, engine string) (*Manifest, error) {
	engine = strings.ToLower(strings.TrimSpace(engine))
	if engine == "" || strings.ContainsAny(engine, `/\\`) {
		return nil, fmt.Errorf("invalid detection engine %q", engine)
	}
	path := filepath.Join(root, OverrideDir, fmt.Sprintf("v%d", Version), engine+".yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data, err = embeddedRules.ReadFile(fmt.Sprintf("rules/v%d/%s.yaml", Version, engine))
	}
	if err != nil {
		return nil, fmt.Errorf("load %s detection rules: %w", engine, err)
	}
	var manifest Manifest
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("parse %s detection rules: %w", engine, err)
	}
	if err := validate(&manifest, engine); err != nil {
		return nil, err
	}
	sort.SliceStable(manifest.Rules, func(i, j int) bool {
		return manifest.Rules[i].Priority > manifest.Rules[j].Priority
	})
	return &manifest, nil
}

func validate(manifest *Manifest, engine string) error {
	if manifest.Version != Version {
		return fmt.Errorf("%s detection rules use version %d, want %d", engine, manifest.Version, Version)
	}
	if strings.ToLower(strings.TrimSpace(manifest.Engine)) != engine {
		return fmt.Errorf("%s detection rules declare engine %q", engine, manifest.Engine)
	}
	seen := make(map[string]bool)
	for i := range manifest.Rules {
		rule := &manifest.Rules[i]
		if rule.ID == "" || seen[rule.ID] {
			return fmt.Errorf("%s detection rule %d has an empty or duplicate id", engine, i)
		}
		seen[rule.ID] = true
		if rule.Source != "title" && rule.Source != "screen" {
			return fmt.Errorf("%s detection rule %s has invalid source %q", engine, rule.ID, rule.Source)
		}
		switch rule.Lifecycle {
		case "idle", "working", "blocked", "unknown":
		default:
			return fmt.Errorf("%s detection rule %s has invalid lifecycle %q", engine, rule.ID, rule.Lifecycle)
		}
		if rule.Source == "screen" && (rule.RegionLines < 1 || rule.RegionLines > MaxRegionLines) {
			return fmt.Errorf("%s detection rule %s region_lines must be between 1 and %d", engine, rule.ID, MaxRegionLines)
		}
		matcher, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return fmt.Errorf("%s detection rule %s pattern: %w", engine, rule.ID, err)
		}
		rule.matcher = matcher
	}
	return nil
}

func (m *Manifest) Detect(source, value string) Result {
	if m == nil {
		return Result{Lifecycle: "unknown", Source: source}
	}
	for _, rule := range m.Rules {
		candidate := value
		if source == "screen" {
			candidate = boundedTail(value, rule.RegionLines)
		}
		if rule.Source == source && rule.matcher.MatchString(candidate) {
			return Result{
				Lifecycle: rule.Lifecycle, Attention: rule.Attention, Source: source,
				RuleID: rule.ID, Priority: rule.Priority, Title: displayTitle(value),
			}
		}
	}
	return Result{Lifecycle: "unknown", Source: source, Title: displayTitle(value)}
}

func boundedTail(value string, lines int) string {
	if lines <= 0 {
		return value
	}
	value = strings.TrimRight(value, "\r\n")
	parts := strings.Split(value, "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n")
}

func (m *Manifest) MaxScreenLines() int {
	maximum := 0
	for _, rule := range m.Rules {
		if rule.Source == "screen" && rule.RegionLines > maximum {
			maximum = rule.RegionLines
		}
	}
	return maximum
}

func displayTitle(title string) string {
	title = strings.TrimSpace(title)
	return strings.TrimSpace(strings.TrimLeft(title, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏·✢✳✶✻✽ "))
}
