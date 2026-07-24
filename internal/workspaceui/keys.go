package workspaceui

import (
	"strings"

	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	quit            key.Binding
	back            key.Binding
	cancel          key.Binding
	confirm         key.Binding
	refresh         key.Binding
	filter          key.Binding
	cycleForward    key.Binding
	cycleBackward   key.Binding
	up              key.Binding
	down            key.Binding
	pageUp          key.Binding
	pageDown        key.Binding
	featurePageUp   key.Binding
	featurePageDown key.Binding
	halfPageUp      key.Binding
	halfPageDown    key.Binding
	top             key.Binding
	bottom          key.Binding
	left            key.Binding
	right           key.Binding
	previous        key.Binding
	next            key.Binding
	open            key.Binding
	archive         key.Binding
	attach          key.Binding
	character       key.Binding
}

func bindingHelp(binding key.Binding) string {
	help := binding.Help()
	return helpItem(help.Key, help.Desc)
}

func combinedBindingHelp(description string, bindings ...key.Binding) string {
	labels := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		labels = append(labels, binding.Help().Key)
	}
	return helpItem(strings.Join(labels, " / "), description)
}

var keys = keyMap{
	quit:            key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	back:            key.NewBinding(key.WithKeys("esc", "b"), key.WithHelp("esc / b", "back")),
	cancel:          key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	confirm:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	refresh:         key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	filter:          key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter work")),
	cycleForward:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane / file")),
	cycleBackward:   key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous pane / file")),
	up:              key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑ / k", "up")),
	down:            key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓ / j", "down")),
	pageUp:          key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
	pageDown:        key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
	featurePageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+b"), key.WithHelp("pgup / ctrl+b", "feature page up")),
	featurePageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+f"), key.WithHelp("pgdn / ctrl+f", "feature page down")),
	halfPageUp:      key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half page up")),
	halfPageDown:    key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half page down")),
	top:             key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	bottom:          key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
	left:            key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "previous")),
	right:           key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "next")),
	previous:        key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("← / h", "previous")),
	next:            key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→ / l", "next")),
	open:            key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	archive:         key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "show / hide archived")),
	attach:          key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "attach")),
	character:       key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "worker character sheet")),
}

// HelpSections returns Workspace guidance for the unified dashboard help.
func HelpSections() []terminalui.HelpSection {
	return []terminalui.HelpSection{
		{Title: "WORKSPACE · NAVIGATE", Entries: []terminalui.HelpEntry{
			terminalui.HelpEntryFor(keys.up),
			terminalui.HelpEntryFor(keys.down),
			terminalui.HelpEntryFor(keys.open),
		}},
		{Title: "WORKSPACE · VIEW", Entries: []terminalui.HelpEntry{
			{Keys: keys.pageUp.Help().Key + " / " + keys.pageDown.Help().Key, Description: "scroll pages"},
			{Keys: keys.top.Help().Key + " / " + keys.bottom.Help().Key, Description: "top / bottom"},
			{Keys: keys.left.Help().Key + " / " + keys.right.Help().Key, Description: "select workflow stage"},
		}},
		{Title: "WORKSPACE · ACT", Entries: []terminalui.HelpEntry{
			terminalui.HelpEntryFor(keys.filter),
			terminalui.HelpEntryFor(keys.attach),
			terminalui.HelpEntryFor(keys.character),
			terminalui.HelpEntryFor(keys.back),
			terminalui.HelpEntryFor(keys.quit),
		}},
	}
}
