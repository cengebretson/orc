package watch

import (
	"testing"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/state"
)

func TestRowFromFeatureCarriesExactAgentIdentityForActions(t *testing.T) {
	s := &state.State{
		Ticket: "ORC-9",
		Stage:  state.Stage{Name: "develop", Worker: "default:bob"},
		Runtime: state.Runtime{
			Mux:   &state.MuxRuntime{Backend: "tmux", Workspace: "orc", Tab: "develop", Pane: "%7"},
			Agent: &state.AgentRuntime{ID: "agent-9", Instance: "instance-9"},
		},
	}
	got := rowFromFeature(&featurelist.Feature{State: s, Stage: "develop", TmuxLive: true}, nil)
	if got.backend != "tmux" || got.session != "orc" || got.window != "develop" || got.pane != "%7" || got.agentID != "agent-9" || got.agentInstance != "instance-9" {
		t.Fatalf("rowFromFeature() target = %#v", got)
	}
}
