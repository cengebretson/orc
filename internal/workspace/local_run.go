package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	"github.com/cengebretson/orc/internal/state"
)

const LocalWorkflow = "default:adhoc"

const localSequenceFilename = ".local-sequence"

type LocalRunOptions struct {
	Root        string
	Instruction string
	Slug        string
	Worker      string
	RepoName    string
	RepoPath    string
}

// LocalRun creates a normal feature for local work that has no external ticket.
func LocalRun(opts LocalRunOptions) (*WorkResult, error) {
	instruction := strings.TrimSpace(opts.Instruction)
	if instruction == "" {
		return nil, fmt.Errorf("instruction is required")
	}
	if strings.TrimSpace(opts.Worker) == "" {
		return nil, fmt.Errorf("worker is required")
	}

	slugSource := instruction
	if strings.TrimSpace(opts.Slug) != "" {
		slugSource = opts.Slug
	}
	suffix := PromptSlug(slugSource)
	if suffix == "" {
		return nil, fmt.Errorf("slug must contain at least one letter or number")
	}

	ticket, err := allocateLocalTicket(opts.Root)
	if err != nil {
		return nil, err
	}
	fullSlug := buildSlug(ticket, suffix)
	prompt := fmt.Sprintf(
		"Complete this local task:\n\n%s\n\nFeature context: features/%s/STATE.yaml\nStage: stages/default/adhoc.md\nWhen finished: orc mark %s done --result \"<summary of what was done>\"",
		instruction,
		fullSlug,
		ticket,
	)

	var initialRepos map[string]state.Repo
	initialCWD := ""
	if opts.RepoName != "" {
		initialRepos = map[string]state.Repo{
			opts.RepoName: {Main: opts.RepoPath},
		}
		initialCWD = opts.RepoPath
	}

	return Work(WorkOptions{
		Root:          opts.Root,
		Ticket:        ticket,
		Slug:          suffix,
		Workflow:      LocalWorkflow,
		InitialWorker: opts.Worker,
		InitialPrompt: prompt,
		HistoryResult: "local feature created by orc run",
		TicketSummary: instruction,
		InitialRepos:  initialRepos,
		InitialCWD:    initialCWD,
	})
}

// PromptSlug derives a short filesystem-safe slug from an instruction or an
// explicit override. It keeps complete ASCII words where possible and caps the
// result at 48 characters.
func PromptSlug(input string) string {
	var words []string
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		words = append(words, current.String())
		current.Reset()
	}
	for _, r := range strings.ToLower(strings.TrimSpace(input)) {
		if r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()

	const maxLength = 48
	var slug strings.Builder
	for _, word := range words {
		extra := len(word)
		if slug.Len() > 0 {
			extra++
		}
		if slug.Len()+extra > maxLength {
			if slug.Len() == 0 {
				slug.WriteString(word[:maxLength])
			}
			break
		}
		if slug.Len() > 0 {
			slug.WriteByte('-')
		}
		slug.WriteString(word)
	}
	return slug.String()
}

func allocateLocalTicket(root string) (string, error) {
	featuresDir := filepath.Join(root, "features")
	if err := os.MkdirAll(featuresDir, 0o755); err != nil {
		return "", fmt.Errorf("creating features directory: %w", err)
	}
	path := filepath.Join(featuresDir, localSequenceFilename)
	sequence, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return "", fmt.Errorf("opening local feature sequence: %w", err)
	}
	defer sequence.Close() //nolint:errcheck
	if err := syscall.Flock(int(sequence.Fd()), syscall.LOCK_EX); err != nil {
		return "", fmt.Errorf("locking local feature sequence: %w", err)
	}
	defer syscall.Flock(int(sequence.Fd()), syscall.LOCK_UN) //nolint:errcheck

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading local feature sequence: %w", err)
	}
	current := 0
	if value := strings.TrimSpace(string(data)); value != "" {
		current, err = strconv.Atoi(value)
		if err != nil || current < 0 {
			return "", fmt.Errorf("invalid local feature sequence %q in %s", value, path)
		}
	}
	next := current + 1
	if err := sequence.Truncate(0); err != nil {
		return "", fmt.Errorf("resetting local feature sequence: %w", err)
	}
	if _, err := sequence.Seek(0, 0); err != nil {
		return "", fmt.Errorf("seeking local feature sequence: %w", err)
	}
	if _, err := fmt.Fprintf(sequence, "%d\n", next); err != nil {
		return "", fmt.Errorf("writing local feature sequence: %w", err)
	}
	if err := sequence.Sync(); err != nil {
		return "", fmt.Errorf("syncing local feature sequence: %w", err)
	}
	return fmt.Sprintf("LOCAL-%d", next), nil
}

func writeTicketSummary(featureDir, ticket, instruction string) error {
	content := fmt.Sprintf(`# TICKET.md — %s

## Summary

%s

## Source

Local feature created by `+"`orc run`"+`; no external tracker ticket.
`, ticket, strings.TrimSpace(instruction))
	return os.WriteFile(filepath.Join(featureDir, "TICKET.md"), []byte(content), 0o644)
}
