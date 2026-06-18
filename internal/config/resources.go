package config

import "strings"

// ResourcePath maps a resource ID to the markdown path used under workspace
// runtime folders. Pack resources use "pack:name" IDs and materialize as
// "pack/name.md" so the runtime layout mirrors the pack namespace.
func ResourcePath(id string) string {
	return strings.ReplaceAll(id, ":", "/") + ".md"
}
