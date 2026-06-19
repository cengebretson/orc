package resume

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/state"
)

type Context struct {
	Ticket       string
	Slug         string
	Stage        string
	Status       string
	StageFiles   []string
	HasDecisions bool
	Prompt       string
}

// Build reads the feature directory and assembles a recovery context.
func Build(root, featureDir string) (*Context, error) {
	s, err := state.Load(featureDir)
	if err != nil {
		return nil, err
	}

	ctx := &Context{
		Ticket: s.Ticket,
		Slug:   s.Slug,
		Stage:  s.Stage.Name,
		Status: s.Status,
	}

	// Partial outputs from the current stage folder.
	stageDir := filepath.Join(featureDir, s.Stage.Name)
	if entries, err := os.ReadDir(stageDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				ctx.StageFiles = append(ctx.StageFiles, e.Name())
			}
		}
	}

	// DECISIONS.md presence.
	if _, err := os.Stat(filepath.Join(featureDir, "DECISIONS.md")); err == nil {
		ctx.HasDecisions = true
	}

	ctx.Prompt = buildPrompt(root, featureDir, s, ctx)
	return ctx, nil
}

func buildPrompt(root, featureDir string, s *state.State, ctx *Context) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Before starting: read AGENTS.md and ORC.md. Run `orc mark %s resume` to mark active.\n\n", s.Ticket)
	fmt.Fprintf(&b, "## Resuming %s — stage: %s\n\n", s.Ticket, s.Stage.Name)
	fmt.Fprintf(&b, "This session was interrupted. Status was `%s` when it stopped.\n\n", s.Status)

	// Last few history entries for context.
	b.WriteString("### Recent history\n\n")
	entries := s.History
	if len(entries) > 5 {
		entries = entries[len(entries)-5:]
	}
	for _, h := range entries {
		fmt.Fprintf(&b, "- **%s** (%s, %s): %s\n", h.Stage, h.Worker, h.At, h.Result)
	}
	b.WriteString("\n")

	// What the stage has produced so far.
	if len(ctx.StageFiles) > 0 {
		fmt.Fprintf(&b, "### Partial outputs in %s/\n\n", s.Stage.Name)
		for _, f := range ctx.StageFiles {
			fmt.Fprintf(&b, "- `%s/%s`\n", s.Stage.Name, f)
		}
		b.WriteString("\n")
		b.WriteString("Read these files to understand what was completed before the interruption.\n\n")
	} else {
		fmt.Fprintf(&b, "No output files found in `%s/` — the stage may not have started yet.\n\n", s.Stage.Name)
	}

	// Key context files.
	b.WriteString("### Context to read\n\n")
	fmt.Fprintf(&b, "- `features/%s/STATE.yaml` — current state and history\n", s.Slug)
	fmt.Fprintf(&b, "- `features/%s/TICKET.md` — original ticket\n", s.Slug)
	fmt.Fprintf(&b, "- `features/%s/SPEC.md` — scope and requirements\n", s.Slug)
	if ctx.HasDecisions {
		fmt.Fprintf(&b, "- `features/%s/DECISIONS.md` — decisions made so far\n", s.Slug)
	}
	fmt.Fprintf(&b, "- `stages/%s` — stage instructions\n", config.ResourcePath(s.Stage.Name))
	b.WriteString("\n")

	// End-of-session instruction.
	b.WriteString("### When done\n\n")
	b.WriteString("Run one of:\n")
	fmt.Fprintf(&b, "  orc mark %s next --worker <worker-id> --result \"<summary>\"\n", s.Ticket)
	fmt.Fprintf(&b, "  orc mark %s pause \"<what you need from the human or what is blocking>\"\n", s.Ticket)

	_ = root // reserved for future use (e.g. resolving relative paths)
	return b.String()
}
