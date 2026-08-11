package main

import (
	"strings"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "cmd/main.go", true}, // bare name matches any segment
		{"**/*.go", "cmd/main.go", true},
		{"cmd/**", "cmd/sub/x.go", true},
		{"cmd/*.go", "cmd/main.go", true},
		{"cmd/*.go", "cmd/sub/x.go", false},
		{"/exact.txt", "exact.txt", true},
		{"/exact.txt", "sub/exact.txt", false},
		{"", "x", false},
	}
	for _, c := range cases {
		if got := matchGlob(c.pattern, c.path); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestIgnoreSetIncludes(t *testing.T) {
	s := newIgnoreSet([]string{"**/*.go"}, nil)
	if s.ignored("main.go") {
		t.Error("main.go should not be ignored")
	}
	if !s.ignored("README.md") {
		t.Error("README.md should be ignored when includes are set")
	}
	s.addExclude("cmd/**")
	if !s.ignored("cmd/x.go") {
		t.Error("cmd/x.go should be excluded")
	}
}

// Regression: include globs must not prune ancestor directories, or
// nested includes would never be reached.
func TestIncludeDoesNotSkipAncestorDirs(t *testing.T) {
	s := newIgnoreSet([]string{"**/*.go"}, nil)
	if s.dirSkippedByInclude("cmd") {
		t.Error("cmd/ must not be skipped; **/*.go can match below it")
	}
	if s.dirSkippedByInclude("internal") {
		t.Error("internal/ must not be skipped")
	}
	// no includes → never skip
	if newIgnoreSet(nil, nil).dirSkippedByInclude("anything") {
		t.Error("empty includes should never skip dirs")
	}
}

func TestScanWithIncludeFindsNested(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/main.go", "package main")
	writeFile(t, dir+"/internal/deep/x.go", "package deep")
	writeFile(t, dir+"/internal/deep/y.md", "doc")

	snaps, err := scan(dir, newIgnoreSet([]string{"**/*.go"}, nil), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snaps["internal/deep/x.go"]; !ok {
		t.Error("nested .go file should be included")
	}
	if _, ok := snaps["internal/deep/y.md"]; ok {
		t.Error(".md file should be excluded by include glob")
	}
}

// Regression: --no-vcs must actually disable gitignore handling.
func TestNoVCSFlag(t *testing.T) {
	cfg, err := parseArgs([]string{"--no-vcs", "/tmp"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.respectVCS {
		t.Error("--no-vcs should set respectVCS=false")
	}
	cfg2, _ := parseArgs([]string{"/tmp"})
	if !cfg2.respectVCS {
		t.Error("default should respect VCS ignores")
	}
}

func TestScanRespectsGitignore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/.gitignore", "ignored.txt\n")
	writeFile(t, dir+"/ignored.txt", "x")
	writeFile(t, dir+"/kept.txt", "x")

	on, _ := scan(dir, newIgnoreSet(nil, nil), true)
	if _, ok := on["ignored.txt"]; ok {
		t.Error("gitignored file should be skipped when respectVCS=true")
	}
	off, _ := scan(dir, newIgnoreSet(nil, nil), false)
	if _, ok := off["ignored.txt"]; !ok {
		t.Error("gitignored file should be present when respectVCS=false")
	}
}

func TestComputeDiffAddDel(t *testing.T) {
	d := computeDiff("f.txt", "a\nb\nc\n", "a\nx\nc\n", false)
	if d.adds != 1 || d.dels != 1 {
		t.Fatalf("adds=%d dels=%d, want 1/1", d.adds, d.dels)
	}
	var ops []op
	for _, l := range d.lines {
		ops = append(ops, l.op)
	}
	// expect: context(a), del(b), add(x), context(c)
	want := []op{opContext, opDel, opAdd, opContext}
	if len(ops) != len(want) {
		t.Fatalf("got %d lines %v, want %v", len(ops), ops, want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("line %d op=%v, want %v (all: %v)", i, ops[i], want[i], ops)
		}
	}
}

func TestComputeDiffWordSegs(t *testing.T) {
	d := computeDiff("f.txt", "hello world\n", "hello there\n", false)
	var del, add *diffLine
	for i := range d.lines {
		switch d.lines[i].op {
		case opDel:
			del = &d.lines[i]
		case opAdd:
			add = &d.lines[i]
		}
	}
	if del == nil || add == nil {
		t.Fatal("expected paired del/add")
	}
	if len(del.segs) == 0 || len(add.segs) == 0 {
		t.Fatal("expected word segments on paired lines")
	}
	changed := ""
	for _, s := range add.segs {
		if s.changed {
			changed += s.text
		}
	}
	if !strings.Contains(changed, "there") {
		t.Errorf("changed segment %q should contain 'there'", changed)
	}
}

func TestFoldContext(t *testing.T) {
	var rows []diffRow
	for i := 0; i < 20; i++ {
		rows = append(rows, diffRow{op: opContext, num: i, text: "x"})
	}
	rows = append(rows, diffRow{op: opAdd, num: 21, text: "y"})
	out := foldContext(rows, 3)
	// 3 + marker + 3 + add = 8
	if len(out) != 8 {
		t.Fatalf("folded to %d rows, want 8", len(out))
	}
	if !strings.Contains(out[3].text, "unchanged") {
		t.Errorf("expected unchanged marker, got %q", out[3].text)
	}
}

func TestRenderSplitPairing(t *testing.T) {
	d := computeDiff("f.txt", "a\nb\nc\n", "a\nx\ny\nc\n", false)
	rows := renderSplit(d, false)
	// context(a) | del(b)/add(x) | blank/add(y) | context(c) = 4 rows
	if len(rows) != 4 {
		t.Fatalf("got %d split rows, want 4", len(rows))
	}
	if rows[1][0] == nil || rows[1][0].op != opDel {
		t.Error("row 1 left should be del")
	}
	if rows[1][1] == nil || rows[1][1].op != opAdd {
		t.Error("row 1 right should be add")
	}
	if rows[2][0] != nil {
		t.Error("row 2 left should be blank")
	}
	if rows[2][1] == nil || rows[2][1].op != opAdd {
		t.Error("row 2 right should be add")
	}
}

func TestIgnorer(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/.gitignore", "*.log\nbuild/\n!keep.log\n")
	writeFile(t, dir+"/keep.log", "")
	writeFile(t, dir+"/a.log", "")
	ig, err := loadIgnorer(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if !ig.ignored("a.log", false) {
		t.Error("a.log should be ignored")
	}
	if ig.ignored("keep.log", false) {
		t.Error("keep.log should be negated")
	}
	if !ig.ignored("build", true) {
		t.Error("build dir should be ignored")
	}
	if ig.ignored("main.go", false) {
		t.Error("main.go should not be ignored")
	}
}

func TestIsBinary(t *testing.T) {
	if !isBinary([]byte{'a', 0, 'b'}) {
		t.Error("NUL-containing data should be binary")
	}
	if isBinary([]byte("plain text\n")) {
		t.Error("plain text should not be binary")
	}
}

func TestDefaultExcludesTempJunk(t *testing.T) {
	s := newIgnoreSet(nil, nil)
	for _, p := range []string{
		"foo.tmp", "nested/x.swp", "a.bak", "dir/file~",
		"sedAb12cd", "nested/sedXYZ99",
	} {
		if !s.ignored(p) {
			t.Errorf("%q should be ignored by default excludes", p)
		}
	}
	for _, p := range []string{"main.go", "sed.go", "README.md", "cmd/sed/main.go"} {
		if s.ignored(p) {
			t.Errorf("%q must NOT be ignored by default excludes", p)
		}
	}
}
