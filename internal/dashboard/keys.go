package dashboard

import (
	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	live      key.Binding
	workspace key.Binding
	help      key.Binding
	back      key.Binding
	quit      key.Binding
}

var keys = keyMap{
	live:      key.NewBinding(key.WithKeys("["), key.WithHelp("[", "Live")),
	workspace: key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "Workspace")),
	help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close help")),
	quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

func dashboardHelpSection() terminalui.HelpSection {
	return terminalui.HelpSection{
		Title: "DASHBOARD",
		Entries: []terminalui.HelpEntry{
			{Keys: keys.live.Help().Key + " / " + keys.workspace.Help().Key, Description: "Live / Workspace"},
			{Keys: keys.help.Help().Key + " / " + keys.back.Help().Key, Description: "close help"},
			terminalui.HelpEntryFor(keys.quit),
		},
	}
}
