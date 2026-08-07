// Package main implements flashdiff, a live file-diff watcher TUI.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
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

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "flashdiff: %v\n", err)
		os.Exit(1)
	}
}
