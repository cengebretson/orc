package parking

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

const PolicyVersion = 1

type Policy struct {
	AutoPark []string
	WakeOn   []string
}

type Observation struct {
	Ticket    string
	Status    string
	Stage     string
	Attention string
}

type Decision struct {
	Parked     bool
	Woken      bool
	WakeReason string
}

type PolicyRecord struct {
	Ticket          string    `json:"ticket"`
	Parked          bool      `json:"parked"`
	ParkedStatus    string    `json:"parked_status"`
	ParkedStage     string    `json:"parked_stage"`
	ParkedAttention string    `json:"parked_attention,omitempty"`
	ParkedAt        time.Time `json:"parked_at"`
	WakeReason      string    `json:"wake_reason,omitempty"`
	WokenAt         time.Time `json:"woken_at,omitempty"`
}

type PolicyState struct {
	Version   int                     `json:"version"`
	Workspace string                  `json:"workspace"`
	PolicyKey string                  `json:"policy_key,omitempty"`
	Tickets   map[string]PolicyRecord `json:"tickets"`
}

func PolicyPath(root, home string) (string, error) {
	parkedPath, err := Path(root, home)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(filepath.Dir(parkedPath)), "parking-policy", filepath.Base(parkedPath)), nil
}

// ApplyPolicy evaluates wake rules before park rules and persists the small
// display-policy state needed to make parking reversible across refreshes.
func ApplyPolicy(path, workspace string, policy Policy, observations []Observation, now time.Time) (map[string]Decision, error) {
	var decisions map[string]Decision
	var warning error
	err := withPolicyLock(path, func() error {
		state, loadErr := loadPolicyState(path)
		missing := errors.Is(loadErr, os.ErrNotExist)
		if loadErr != nil && !missing {
			warning = fmt.Errorf("parking policy state was reset: %w", loadErr)
			var invalid *invalidPolicyStateError
			if errors.As(loadErr, &invalid) {
				if quarantineErr := quarantinePolicyState(path, now); quarantineErr != nil {
					warning = errors.Join(warning, fmt.Errorf("quarantine invalid parking state: %w", quarantineErr))
				}
			}
		}
		key := policyKey(policy)
		if loadErr != nil || state.Tickets == nil || state.Workspace != workspace || state.PolicyKey != key {
			state = PolicyState{
				Version: PolicyVersion, Workspace: workspace, PolicyKey: key,
				Tickets: make(map[string]PolicyRecord),
			}
		}
		before := clonePolicyState(state)
		decisions = evaluatePolicy(&state, policy, observations, now)
		if missing || loadErr != nil || !reflect.DeepEqual(before, state) {
			if saveErr := savePolicyState(path, state); saveErr != nil {
				warning = errors.Join(warning, fmt.Errorf("save parking policy state: %w", saveErr))
			}
		}
		return nil
	})
	if err != nil {
		warning = errors.Join(warning, err)
	}
	return decisions, warning
}

func evaluatePolicy(state *PolicyState, policy Policy, observations []Observation, now time.Time) map[string]Decision {
	autoPark := stringSet(policy.AutoPark)
	wakeOn := stringSet(policy.WakeOn)
	decisions := make(map[string]Decision, len(observations))
	seen := make(map[string]bool, len(observations))
	for _, observation := range observations {
		seen[observation.Ticket] = true
		record, exists := state.Tickets[observation.Ticket]
		parkable := autoPark[normalize(observation.Status)]
		if exists && record.Parked && !parkable {
			delete(state.Tickets, observation.Ticket)
			exists = false
			record = PolicyRecord{}
		}
		if exists && record.Parked {
			if reason := wakeReason(record, observation, wakeOn); reason != "" {
				record.Parked = false
				record.WakeReason = reason
				record.WokenAt = now
				state.Tickets[observation.Ticket] = record
				decisions[observation.Ticket] = Decision{Woken: true, WakeReason: reason}
				continue
			}
			decisions[observation.Ticket] = Decision{Parked: true}
			continue
		}

		// A woken ticket remains visible while it is still in the status that
		// originally parked it. Leaving that status rearms the policy.
		if exists && record.WakeReason != "" && strings.EqualFold(observation.Status, record.ParkedStatus) {
			decisions[observation.Ticket] = Decision{Woken: true, WakeReason: record.WakeReason}
			continue
		}
		if parkable && wakeOn["attention"] && attentionNeeded(observation.Attention) {
			record = PolicyRecord{
				Ticket: observation.Ticket, ParkedStatus: observation.Status, ParkedStage: observation.Stage,
				ParkedAttention: observation.Attention, WakeReason: "attention", WokenAt: now,
			}
			state.Tickets[observation.Ticket] = record
			decisions[observation.Ticket] = Decision{Woken: true, WakeReason: "attention"}
			continue
		}
		if parkable {
			record = PolicyRecord{
				Ticket: observation.Ticket, Parked: true, ParkedStatus: observation.Status,
				ParkedStage: observation.Stage, ParkedAttention: observation.Attention, ParkedAt: now,
			}
			state.Tickets[observation.Ticket] = record
			decisions[observation.Ticket] = Decision{Parked: true}
			continue
		}
		delete(state.Tickets, observation.Ticket)
	}
	for ticket := range state.Tickets {
		if !seen[ticket] {
			delete(state.Tickets, ticket)
		}
	}
	return decisions
}

func wakeReason(record PolicyRecord, observation Observation, wakeOn map[string]bool) string {
	if wakeOn["status_change"] && !strings.EqualFold(observation.Status, record.ParkedStatus) {
		return "status_change"
	}
	if wakeOn["attention"] && attentionNeeded(observation.Attention) && !attentionNeeded(record.ParkedAttention) {
		return "attention"
	}
	if wakeOn["stage_change"] && observation.Stage != record.ParkedStage {
		return "stage_change"
	}
	return ""
}

func attentionNeeded(value string) bool {
	switch normalize(value) {
	case "input", "blocked", "review":
		return true
	default:
		return false
	}
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		if value = normalize(value); value != "" {
			set[value] = true
		}
	}
	return set
}

func normalize(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func policyKey(policy Policy) string {
	normalized := func(values []string) []string {
		set := stringSet(values)
		result := make([]string, 0, len(set))
		for value := range set {
			result = append(result, value)
		}
		sort.Strings(result)
		return result
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized(policy.AutoPark), "\x00") + "\x01" + strings.Join(normalized(policy.WakeOn), "\x00")))
	return fmt.Sprintf("%x", sum)
}

func clonePolicyState(state PolicyState) PolicyState {
	clone := state
	clone.Tickets = make(map[string]PolicyRecord, len(state.Tickets))
	for ticket, record := range state.Tickets {
		clone.Tickets[ticket] = record
	}
	return clone
}

type invalidPolicyStateError struct{ err error }

func (e *invalidPolicyStateError) Error() string { return e.err.Error() }
func (e *invalidPolicyStateError) Unwrap() error { return e.err }

func loadPolicyState(path string) (PolicyState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PolicyState{}, err
	}
	var state PolicyState
	if err := json.Unmarshal(data, &state); err != nil {
		return PolicyState{}, &invalidPolicyStateError{err: fmt.Errorf("decode parking policy state: %w", err)}
	}
	if state.Version != PolicyVersion {
		return PolicyState{}, &invalidPolicyStateError{err: errors.New("unsupported parking policy state version")}
	}
	return state, nil
}

func quarantinePolicyState(path string, now time.Time) error {
	quarantine := fmt.Sprintf("%s.corrupt-%s", path, now.UTC().Format("20060102T150405.000000000Z"))
	return os.Rename(path, quarantine)
}

func savePolicyState(path string, state PolicyState) error {
	state.Version = PolicyVersion
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".parking-policy-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
