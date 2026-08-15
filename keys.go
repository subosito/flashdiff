package main

import "charm.land/bubbles/v2/key"

type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	Top          key.Binding
	Bottom       key.Binding
	Tab          key.Binding
	Enter        key.Binding
	Mode         key.Binding
	WordDiff     key.Binding
	Filter       key.Binding
	Rescan       key.Binding
	Clear        key.Binding
	CompactFiles key.Binding
	Help         key.Binding
	Quit         key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G", "bottom"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "pane"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter", "l"),
			key.WithHelp("enter", "focus diff"),
		),
		Mode: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "diff mode"),
		),
		WordDiff: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "word diff"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Rescan: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "rescan"),
		),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear"),
		),
		CompactFiles: key.NewBinding(
			key.WithKeys("\\"),
			key.WithHelp("\\", "icons"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp is the minimal set of hints shown in the status bar.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Filter, k.Help, k.Quit}
}

// FullHelp is the help-overlay column set.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom, k.Tab, k.Enter},
		{k.Mode, k.WordDiff, k.Filter, k.CompactFiles, k.Rescan, k.Clear},
		{k.Help, k.Quit},
	}
}
