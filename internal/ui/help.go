package ui

import "github.com/charmbracelet/bubbles/key"

// HelpEntry is display-ready key guidance derived from an executable binding.
type HelpEntry struct {
	Keys        string
	Description string
}

// HelpSection groups related key guidance for a screen or interaction mode.
type HelpSection struct {
	Title   string
	Entries []HelpEntry
}

// HelpEntryFor keeps help text tied to the binding that implements it.
func HelpEntryFor(binding key.Binding) HelpEntry {
	help := binding.Help()
	return HelpEntry{Keys: help.Key, Description: help.Desc}
}
