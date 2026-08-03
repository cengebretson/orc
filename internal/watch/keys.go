package watch

import (
	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	quit      key.Binding
	back      key.Binding
	help      key.Binding
	filter    key.Binding
	up        key.Binding
	down      key.Binding
	pageUp    key.Binding
	pageDown  key.Binding
	top       key.Binding
	bottom    key.Binding
	details   key.Binding
	confirm   key.Binding
	refresh   key.Binding
	attach    key.Binding
	attention key.Binding
	view      key.Binding
	petLayout key.Binding
	parking   key.Binding
}

var keys = keyMap{
	quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back / clear")),
	help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	filter:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter work")),
	up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k / ↑", "move / scroll up")),
	down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j / ↓", "move / scroll down")),
	pageUp:    key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup / ctrl+u", "scroll half page up")),
	pageDown:  key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn / ctrl+d", "scroll half page down")),
	top:       key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	bottom:    key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
	details:   key.NewBinding(key.WithKeys("enter", "n"), key.WithHelp("enter / n", "details / back")),
	confirm:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh now")),
	attach:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "attach selected")),
	attention: key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "focus attention")),
	view:      key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "rail / pets")),
	petLayout: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "pet layout")),
	parking:   key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "expand parked")),
}

// HelpSections returns Live guidance for both the standalone and unified help.
func HelpSections() []terminalui.HelpSection {
	return []terminalui.HelpSection{
		{Title: "LIVE · NAVIGATE", Entries: []terminalui.HelpEntry{
			terminalui.HelpEntryFor(keys.up),
			terminalui.HelpEntryFor(keys.down),
			terminalui.HelpEntryFor(keys.details),
			terminalui.HelpEntryFor(keys.filter),
		}},
		{Title: "LIVE · DETAILS", Entries: []terminalui.HelpEntry{
			terminalui.HelpEntryFor(keys.pageUp),
			terminalui.HelpEntryFor(keys.pageDown),
			{Keys: keys.top.Help().Key + " / " + keys.bottom.Help().Key, Description: "top / bottom"},
		}},
		{Title: "LIVE · ACT", Entries: []terminalui.HelpEntry{
			terminalui.HelpEntryFor(keys.attach),
			terminalui.HelpEntryFor(keys.attention),
			terminalui.HelpEntryFor(keys.refresh),
		}},
		{Title: "LIVE · VIEW", Entries: []terminalui.HelpEntry{
			terminalui.HelpEntryFor(keys.view),
			terminalui.HelpEntryFor(keys.petLayout),
			terminalui.HelpEntryFor(keys.parking),
			{Keys: keys.help.Help().Key + " / " + keys.back.Help().Key, Description: "close help"},
			terminalui.HelpEntryFor(keys.quit),
		}},
	}
}
