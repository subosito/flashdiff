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
	m.diffVP.SetWidth(width)
	m.diffVP.SetHeight(40)
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

func TestVisibleChangesSplitsBinary(t *testing.T) {
	m := newTestModel()
	m.changes = []*change{
		{rel: "a.go", binary: false},
		{rel: "bin", binary: true},
		{rel: "b.txt", binary: false},
		{rel: "app", binary: true},
	}
	text := m.visibleTextChanges()
	bins := m.visibleBinaryChanges()
	if len(text) != 2 || len(bins) != 2 {
		t.Fatalf("text=%d bins=%d, want 2/2", len(text), len(bins))
	}
	vis := m.visibleChanges()
	// text first, then binaries
	if vis[0].rel != "a.go" || vis[1].rel != "b.txt" {
		t.Fatalf("text order wrong: %v %v", vis[0].rel, vis[1].rel)
	}
	if vis[2].rel != "bin" || vis[3].rel != "app" {
		t.Fatalf("bin order wrong: %v %v", vis[2].rel, vis[3].rel)
	}

	// File list should contain a BINARIES header between groups, and stay
	// hidden when there are no binaries.
	out := stripANSI(m.renderFileList(40))
	if !strings.Contains(out, "◆ BINARIES") {
		t.Fatalf("expected ◆ BINARIES header: %q", out)
	}
	// Rule separator should appear when text + binaries are both present.
	if !strings.Contains(out, "─") {
		t.Fatalf("expected section rule above BINARIES: %q", out)
	}
	// Header should appear after text files and before binary names.
	iText := strings.Index(out, "a.go")
	iHdr := strings.Index(out, "BINARIES")
	iBin := strings.Index(out, "bin")
	if iText < 0 || iHdr <= iText || iBin <= iHdr {
		t.Fatalf("expected text < header < binary order, got %d %d %d\n%s", iText, iHdr, iBin, out)
	}

	// Hidden by default when no binaries.
	m2 := newTestModel()
	m2.changes = []*change{{rel: "only.go", binary: false}}
	out2 := stripANSI(m2.renderFileList(40))
	if strings.Contains(out2, "BINARIES") {
		t.Fatalf("BINARIES section must be hidden with no binaries: %q", out2)
	}
}

func TestBinaryDoesNotStealTextSelection(t *testing.T) {
	m := newTestModel()
	m.selected = -1
	m.upsertChange(&change{rel: "src.go", binary: false})
	if m.selectedChange() == nil || m.selectedChange().rel != "src.go" {
		t.Fatalf("expected src.go selected")
	}
	m.upsertChange(&change{rel: "out", binary: true})
	// Text selection should stick; DIFF pane stays on src.go.
	if c := m.selectedChange(); c == nil || c.rel != "src.go" {
		t.Fatalf("binary stole selection: got %v", c)
	}
	if len(m.visibleBinaryChanges()) != 1 {
		t.Fatalf("expected 1 binary change")
	}
}

func TestBinaryAloneDoesNotFillDiffPane(t *testing.T) {
	// Binary-only noise must not auto-drive the DIFF pane.
	m := newTestModel()
	m.selected = -1
	m.upsertChange(&change{rel: "a.bin", binary: true})
	m.upsertChange(&change{rel: "b.bin", binary: true})
	if c := m.selectedChange(); c != nil {
		t.Fatalf("binary-only list must leave DIFF empty, got %v", c)
	}
	// First text change takes the pane.
	m.upsertChange(&change{rel: "a.go", binary: false})
	if c := m.selectedChange(); c == nil || c.rel != "a.go" || c.binary {
		t.Fatalf("want a.go selected after text change, got %v", c)
	}
	// Later binaries still do not steal.
	m.upsertChange(&change{rel: "c.bin", binary: true})
	if c := m.selectedChange(); c == nil || c.rel != "a.go" {
		t.Fatalf("binary stole after text: got %v", c)
	}
}
