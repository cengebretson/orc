package tmuxattention

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const WorktreeContextFile = "tmux-attention-context"

// DisplaySlug removes a repeated ticket prefix and normalizes the remainder
// to the compact kebab-case label tmux-attention renders beside the ticket.
func DisplaySlug(ticket, featureSlug string) string {
	slug := strings.TrimSpace(featureSlug)
	if len(slug) >= len(ticket) && strings.EqualFold(slug[:len(ticket)], ticket) {
		remainder := slug[len(ticket):]
		if remainder == "" || isSeparator(rune(remainder[0])) {
			slug = strings.TrimLeftFunc(remainder, isSeparator)
		}
	}

	var normalized strings.Builder
	separator := false
	for _, char := range strings.ToLower(slug) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			if separator && normalized.Len() > 0 {
				normalized.WriteByte('-')
			}
			normalized.WriteRune(char)
			separator = false
			continue
		}
		separator = true
	}
	return normalized.String()
}

func isSeparator(char rune) bool {
	switch {
	case char >= 'a' && char <= 'z':
		return false
	case char >= 'A' && char <= 'Z':
		return false
	case char >= '0' && char <= '9':
		return false
	default:
		return true
	}
}

// WriteWorktreeContext publishes Orc's durable ticket identity through
// tmux-attention's worktree-local metadata contract. The file lives in the
// absolute Git directory, so it does not dirty the checkout or get committed.
func WriteWorktreeContext(worktreeDir, project, featureSlug string) error {
	if strings.TrimSpace(project) == "" {
		return fmt.Errorf("tmux-attention context requires a project")
	}
	if invalidValue(project) || invalidValue(featureSlug) {
		return fmt.Errorf("tmux-attention context contains a control character")
	}

	branch, err := gitOutput(worktreeDir, "branch", "--show-current")
	if err != nil {
		return err
	}
	if branch == "" {
		return fmt.Errorf("tmux-attention context requires an attached Git branch")
	}
	gitDir, err := gitOutput(worktreeDir, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}

	content := []byte(fmt.Sprintf("branch=%s\nproject=%s\nslug=%s\n", branch, project, DisplaySlug(project, featureSlug)))
	contextPath := filepath.Join(gitDir, WorktreeContextFile)
	if existing, readErr := os.ReadFile(contextPath); readErr == nil && bytes.Equal(existing, content) {
		return nil
	}

	temp, err := os.CreateTemp(gitDir, ".tmux-attention-context-*")
	if err != nil {
		return fmt.Errorf("create tmux-attention context: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath) //nolint:errcheck
	if _, err := temp.Write(content); err != nil {
		temp.Close() //nolint:errcheck
		return fmt.Errorf("write tmux-attention context: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close tmux-attention context: %w", err)
	}
	if err := os.Rename(tempPath, contextPath); err != nil {
		return fmt.Errorf("publish tmux-attention context: %w", err)
	}
	return nil
}

func invalidValue(value string) bool {
	return strings.ContainsAny(value, "\r\n\t")
}

func gitOutput(worktreeDir string, args ...string) (string, error) {
	commandArgs := append([]string{"-C", worktreeDir}, args...)
	out, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return strings.TrimSpace(string(out)), nil
}
