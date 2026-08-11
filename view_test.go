package main

import (
	"regexp"
	"strings"
	"testing"
)

// stripANSI removes ANSI escape sequences so tests can check visible text.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// newTestModel builds a minimal model with styles for view-layer tests.
// It does NOT start a watcher; it only provides enough state for the
// render functions.
func newTestModel() model {
	p := newPalette()
	return model{
		st:         newStyles(p),
		mode:       modeCompact,
		hl:         newHighlighter(),
		filesWidth: 30,
		width:      80,
		height:     24,
		ready:      true,
	}
}

// renderRows helper: render unified rows at a fixed width, return lines
// with ANSI escape codes stripped.
func renderRows(rows []diffRow, width int) []string {
	m := newTestModel()
	m.diffVP.Width = width
	m.diffVP.Height = 40
	out := m.renderUnifiedRows(rows, width, "test.go")
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		lines[i] = stripANSI(l)
	}
	return lines
}

func TestRenderUnifiedRowsGutterSeparator(t *testing.T) {
	// 2-digit line numbers to verify right-alignment.
	rows := []diffRow{
		{op: opContext, num: 8, text: "context line"},
		{op: opDel, num: 9, text: "old line"},
		{op: opAdd, num: 9, text: "new line"},
		{op: opContext, num: 10, text: "trailing"},
	}
	lines := renderRows(rows, 40)

	// Every content row must contain the │ separator.
	for i, l := range lines {
		if !strings.Contains(l, "│") {
			t.Fatalf("row %d missing │ separator: %q", i, l)
		}
	}

	// Line numbers must be right-aligned: single-digit (8,9) preceded
	// by a space, double-digit (10) not.
	for i, want := range []string{" 8", " 9", " 9", "10"} {
		if !strings.Contains(lines[i], want+" │") {
			t.Fatalf("row %d: expected %q before │, got %q", i, want, lines[i])
		}
	}
}

func TestRenderUnifiedRowsMarkerColumn(t *testing.T) {
	rows := []diffRow{
		{op: opContext, num: 1, text: "keep"},
		{op: opDel, num: 2, text: "remove"},
		{op: opAdd, num: 2, text: "insert"},
	}
	lines := renderRows(rows, 40)

	// Context: marker column is a space (so "  " before content).
	// Del: "- " before content.
	// Add: "+ " before content.
	if !strings.Contains(lines[0], "│  keep") {
		t.Fatalf("context row: expected '│  keep', got %q", lines[0])
	}
	if !strings.Contains(lines[1], "│- remove") {
		t.Fatalf("del row: expected '│- remove', got %q", lines[1])
	}
	if !strings.Contains(lines[2], "│+ insert") {
		t.Fatalf("add row: expected '│+ insert', got %q", lines[2])
	}
}

func TestRenderUnifiedRowsContentAlignment(t *testing.T) {
	// The content text should start at the same offset from the │
	// separator for context, add, and del lines (after the marker column).
	rows := []diffRow{
		{op: opContext, num: 1, text: "foo"},
		{op: opDel, num: 2, text: "bar"},
		{op: opAdd, num: 2, text: "baz"},
	}
	lines := renderRows(rows, 40)

	// Find the column of "foo", "bar", "baz" relative to the │ separator.
	offsets := make([]int, 3)
	for i, want := range []string{"foo", "bar", "baz"} {
		sepIdx := strings.Index(lines[i], "│")
		if sepIdx < 0 {
			t.Fatalf("row %d: no │ separator", i)
		}
		contentIdx := strings.Index(lines[i], want)
		if contentIdx < 0 {
			t.Fatalf("row %d: %q not found in %q", i, want, lines[i])
		}
		offsets[i] = contentIdx - sepIdx
	}
	// All content should start at the same offset from │ (marker + space).
	if offsets[0] != offsets[1] || offsets[1] != offsets[2] {
		t.Fatalf("content offsets from │ differ: context=%d del=%d add=%d",
			offsets[0], offsets[1], offsets[2])
	}
}

func TestRenderBinaryMessage(t *testing.T) {
	// Binary diff should show "binary file rebuilt", not attempt a diff.
	d := fileDiff{rel: "app", binary: true}
	rows := renderUnified(d, true)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row for binary, got %d", len(rows))
	}
	if rows[0].text != "binary file rebuilt" {
		t.Fatalf("expected 'binary file rebuilt', got %q", rows[0].text)
	}

	// New binary file
	d = fileDiff{rel: "app", binary: true, isNew: true}
	rows = renderUnified(d, true)
	if rows[0].text != "new binary file" {
		t.Fatalf("expected 'new binary file', got %q", rows[0].text)
	}

	// Deleted binary file
	d = fileDiff{rel: "app", binary: true, isDel: true}
	rows = renderUnified(d, true)
	if rows[0].text != "binary file removed" {
		t.Fatalf("expected 'binary file removed', got %q", rows[0].text)
	}
}

func TestRenderSplitBinaryMessage(t *testing.T) {
	d := fileDiff{rel: "app", binary: true}
	cells := renderSplit(d, true)
	if len(cells) != 1 {
		t.Fatalf("expected 1 row for binary split, got %d", len(cells))
	}
	if cells[0][0] == nil || cells[0][0].text != "binary file rebuilt" {
		t.Fatalf("expected 'binary file rebuilt', got %v", cells[0][0])
	}
}

func TestThemeChangesPalette(t *testing.T) {
	// Save and restore
	orig := ThemeName
	defer func() { ThemeName = orig }()

	ThemeName = "catppuccin-mocha"
	p1 := newPalette()

	ThemeName = "dracula"
	p2 := newPalette()

	// The palettes should differ — at least the primary color.
	if p1.primary == p2.primary {
		t.Fatalf("expected different primary colors for mocha vs dracula, both=%s", p1.primary)
	}
}

func TestThemeValidation(t *testing.T) {
	for _, name := range []string{
		"catppuccin-mocha", "dracula", "gruvbox", "monokai", "nord",
		"solarized-dark", "tokyonight-night", "onedark",
	} {
		if !knownThemes[name] {
			t.Errorf("theme %q should be known", name)
		}
	}
	if knownThemes["nonexistent"] {
		t.Error("nonexistent theme should not be known")
	}
}
