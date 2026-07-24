package dashboard

import (
	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	previous key.Binding
	next     key.Binding
	help     key.Binding
	back     key.Binding
	quit     key.Binding
}

var keys = keyMap{
	previous: key.NewBinding(key.WithKeys("[", "shift+tab"), key.WithHelp("[ / shift+tab", "previous tab")),
	next:     key.NewBinding(key.WithKeys("]", "tab"), key.WithHelp("] / tab", "next tab")),
	help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close help")),
	quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

func dashboardHelpSection() terminalui.HelpSection {
	return terminalui.HelpSection{
		Title: "DASHBOARD",
		Entries: []terminalui.HelpEntry{
			{Keys: keys.previous.Help().Key + " / " + keys.next.Help().Key, Description: "previous / next tab"},
			{Keys: "1–5", Description: "Live / Workflows / Workers / Repositories / Health"},
			{Keys: keys.help.Help().Key + " / " + keys.back.Help().Key, Description: "close help"},
			terminalui.HelpEntryFor(keys.quit),
		},
	}
}
