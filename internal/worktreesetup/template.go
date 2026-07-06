package worktreesetup

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var placeholderPattern = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\}\}`)

var allowedPlaceholders = map[string]bool{
	"branch":        true,
	"repo_name":     true,
	"repo_path":     true,
	"slug":          true,
	"ticket":        true,
	"workspace":     true,
	"worktree_path": true,
}

// Values are the placeholders available to a repo worktree setup command.
type Values map[string]string

// UnknownPlaceholders returns placeholders not supported by the setup template.
func UnknownPlaceholders(template string) []string {
	seen := map[string]bool{}
	for _, match := range placeholderPattern.FindAllStringSubmatch(template, -1) {
		if len(match) < 2 {
			continue
		}
		name := match[1]
		if allowedPlaceholders[name] {
			continue
		}
		seen[name] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ReferencesWorktreePath reports whether the template names the destination
// worktree path. Setup commands should normally include it so the created
// worktree lands where orc state expects it.
func ReferencesWorktreePath(template string) bool {
	return strings.Contains(template, "{{worktree_path}}")
}

// Expand replaces all supported placeholders in template.
func Expand(template string, values Values) (string, error) {
	var missing []string
	out := placeholderPattern.ReplaceAllStringFunc(template, func(token string) string {
		matches := placeholderPattern.FindStringSubmatch(token)
		if len(matches) < 2 {
			return token
		}
		name := matches[1]
		if !allowedPlaceholders[name] {
			return token
		}
		value, ok := values[name]
		if !ok {
			missing = append(missing, name)
			return token
		}
		return value
	})
	if unknown := UnknownPlaceholders(template); len(unknown) > 0 {
		return "", fmt.Errorf("unknown placeholder(s): %s", strings.Join(unknown, ", "))
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return "", fmt.Errorf("missing placeholder value(s): %s", strings.Join(missing, ", "))
	}
	return out, nil
}
