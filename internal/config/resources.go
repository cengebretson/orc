package config

import "strings"

// ResourceFileName maps a resource ID to the flat markdown filename used in
// workspace runtime folders. Pack resources use "pack:name" IDs but materialize
// as "pack-name.md" so stages/ and workers/ remain shell-friendly.
func ResourceFileName(id string) string {
	return strings.ReplaceAll(id, ":", "-") + ".md"
}
