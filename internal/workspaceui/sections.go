package workspaceui

// paneID identifies the two focus regions on the Workspace overview.
type paneID uint8

const (
	paneFeatures paneID = iota
	paneSection
)

// sectionID is the stable identity for a Workspace overview section. Keeping
// this typed prevents display labels and map keys from becoming navigation
// state by accident.
type sectionID uint8

const (
	sectionNone sectionID = iota
	sectionHealth
	sectionWorkflows
	sectionWorkers
	sectionRepositories
)

type sectionSpec struct {
	id              sectionID
	shortcut        string
	title           string
	defaultExpanded bool
	alwaysNavigable bool
}

var workspaceSections = []sectionSpec{
	{id: sectionHealth, shortcut: "1", title: "Health", alwaysNavigable: true},
	{id: sectionWorkflows, shortcut: "2", title: "Workflows"},
	{id: sectionWorkers, shortcut: "3", title: "Workers"},
	{id: sectionRepositories, shortcut: "4", title: "Repositories"},
}

func sectionSpecFor(id sectionID) sectionSpec {
	for _, spec := range workspaceSections {
		if spec.id == id {
			return spec
		}
	}
	return sectionSpec{id: sectionNone}
}

func sectionForShortcut(shortcut string) (sectionID, bool) {
	for _, spec := range workspaceSections {
		if spec.shortcut == shortcut {
			return spec.id, true
		}
	}
	return sectionNone, false
}

func defaultSectionExpansion() map[sectionID]bool {
	expanded := make(map[sectionID]bool, len(workspaceSections))
	for _, spec := range workspaceSections {
		expanded[spec.id] = spec.defaultExpanded
	}
	return expanded
}

func sectionLabel(id sectionID) string {
	return sectionSpecFor(id).title
}
