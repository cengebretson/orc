package tmux

import (
	"fmt"
	"os"
	"strings"

	"github.com/cengebretson/orc/internal/shellquote"
)

func shellJoin(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, arg := range argv {
		parts = append(parts, shellquote.Word(arg))
	}
	return strings.Join(parts, " ")
}

func writeScript(runDir string, argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("launch command is empty")
	}
	f, err := os.CreateTemp("", "orc-launch-*.sh")
	if err != nil {
		return "", fmt.Errorf("temp script: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var parts []string
	for _, arg := range argv {
		parts = append(parts, shellquote.Word(arg))
	}
	// cd to the right directory, remove the script, and replace the shell with
	// the provider so tmux reports the provider PID for exact correlation.
	// The cd must not fall through: if runDir was removed, launching the agent
	// from the wrong directory would silently run repo commands against the
	// wrong tree, so exit instead.
	if _, err := fmt.Fprintf(f, "#!/usr/bin/env bash\ntrap 'rm -f %s' EXIT\ncd %s || exit 1\nrm -f %s\ntrap - EXIT\nexec %s\n",
		shellquote.Word(f.Name()),
		shellquote.Word(runDir),
		shellquote.Word(f.Name()),
		strings.Join(parts, " "),
	); err != nil {
		return "", fmt.Errorf("write script: %w", err)
	}
	return f.Name(), nil
}
