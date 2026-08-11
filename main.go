// Package main implements flashdiff, a live file-diff watcher TUI.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "flashdiff: %v\n", err)
		os.Exit(2)
	}

	m, err := newModel(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flashdiff: %v\n", err)
		os.Exit(1)
	}

	// Alt-screen and mouse cell motion are requested on each View() in v2.
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "flashdiff: %v\n", err)
		os.Exit(1)
	}
}
