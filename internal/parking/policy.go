package parking

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	state, err := loadPolicyState(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if state.Tickets == nil || state.Workspace != workspace {
		state = PolicyState{Version: PolicyVersion, Workspace: workspace, Tickets: make(map[string]PolicyRecord)}
	}
	autoPark := stringSet(policy.AutoPark)
	wakeOn := stringSet(policy.WakeOn)
	decisions := make(map[string]Decision, len(observations))
	seen := make(map[string]bool, len(observations))
	for _, observation := range observations {
		seen[observation.Ticket] = true
		record, exists := state.Tickets[observation.Ticket]
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
		if autoPark[normalize(observation.Status)] && wakeOn["attention"] && attentionNeeded(observation.Attention) {
			record = PolicyRecord{
				Ticket: observation.Ticket, ParkedStatus: observation.Status, ParkedStage: observation.Stage,
				ParkedAttention: observation.Attention, WakeReason: "attention", WokenAt: now,
			}
			state.Tickets[observation.Ticket] = record
			decisions[observation.Ticket] = Decision{Woken: true, WakeReason: "attention"}
			continue
		}
		if autoPark[normalize(observation.Status)] {
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
	if err := savePolicyState(path, state); err != nil {
		return nil, err
	}
	return decisions, nil
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

func loadPolicyState(path string) (PolicyState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PolicyState{}, err
	}
	var state PolicyState
	if err := json.Unmarshal(data, &state); err != nil {
		return PolicyState{}, err
	}
	if state.Version != PolicyVersion {
		return PolicyState{}, errors.New("unsupported parking policy state version")
	}
	return state, nil
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
