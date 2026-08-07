package main

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type op int

const (
	opContext op = iota
	opAdd
	opDel
)

// diffLine is one rendered line of a unified diff.
type diffLine struct {
	op     op
	oldNum int // 1-based; 0 when not applicable
	newNum int
	text   string
	// word-level change segments (only when word diff is enabled):
	// pairs of (text, changed) computed against the paired line.
	segs []segment
}

type segment struct {
	text    string
	changed bool
}

// fileDiff is the computed diff for one file change.
type fileDiff struct {
	rel    string
	binary bool
	isNew  bool
	isDel  bool
	lines  []diffLine
	adds   int
	dels   int
}

const maxContextLines = 12000

// computeDiff builds a line-level unified diff between old and new.
func computeDiff(rel, oldStr, newStr string, binary bool) fileDiff {
	d := fileDiff{rel: rel, binary: binary}
	oldLines := splitLines(oldStr)
	newLines := splitLines(newStr)

	dmp := diffmatchpatch.New()
	a, b, arr := dmp.DiffLinesToChars(oldStr, newStr)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, arr)

	oldNum, newNum := 1, 1
	// pending run of del/add lines for word-level pairing
	var pendDel, pendAdd []diffLine
	flush := func() {
		pairWords(pendDel, pendAdd)
		d.lines = append(d.lines, pendDel...)
		d.lines = append(d.lines, pendAdd...)
		pendDel, pendAdd = nil, nil
	}
	for _, df := range diffs {
		for _, ln := range splitLines(df.Text) {
			if len(d.lines)+len(pendDel)+len(pendAdd) >= maxContextLines {
				flush()
				d.lines = append(d.lines, diffLine{op: opContext, text: "… (diff truncated)"})
				d.adds += len(pendAdd)
				d.dels += len(pendDel)
				return d
			}
			switch df.Type {
			case diffmatchpatch.DiffDelete:
				pendDel = append(pendDel, diffLine{op: opDel, oldNum: oldNum, text: ln})
				oldNum++
				d.dels++
			case diffmatchpatch.DiffInsert:
				pendAdd = append(pendAdd, diffLine{op: opAdd, newNum: newNum, text: ln})
				newNum++
				d.adds++
			case diffmatchpatch.DiffEqual:
				flush()
				d.lines = append(d.lines, diffLine{op: opContext, oldNum: oldNum, newNum: newNum, text: ln})
				oldNum++
				newNum++
			}
		}
	}
	flush()
	_ = oldLines
	_ = newLines
	return d
}

// pairWords computes word-level highlights between paired del/add lines.
// Lines are paired in order (1st del with 1st add, etc.).
func pairWords(dels, adds []diffLine) {
	n := len(dels)
	if len(adds) < n {
		n = len(adds)
	}
	dmp := diffmatchpatch.New()
	for i := 0; i < n; i++ {
		oldT, newT := dels[i].text, adds[i].text
		if oldT == newT {
			continue
		}
		// Word granularity: diff on space-separated tokens instead of
		// raw characters so highlights land on whole words.
		oldToks := strings.Split(oldT, " ")
		newToks := strings.Split(newT, " ")
		a := strings.Join(oldToks, "\n")
		b := strings.Join(newToks, "\n")
		ra, rb, arr := dmp.DiffLinesToRunes(a, b)
		diffs := dmp.DiffMainRunes(ra, rb, false)
		diffs = dmp.DiffCharsToLines(diffs, arr)
		dels[i].segs = segsFor(diffs, diffmatchpatch.DiffDelete)
		adds[i].segs = segsFor(diffs, diffmatchpatch.DiffInsert)
	}
}

func segsFor(diffs []diffmatchpatch.Diff, want diffmatchpatch.Operation) []segment {
	var out []segment
	for _, d := range diffs {
		// tokens were joined with "\n" in pairWords; restore spaces
		text := strings.ReplaceAll(d.Text, "\n", " ")
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			out = append(out, segment{text: text})
		case want:
			out = append(out, segment{text: text, changed: true})
		}
	}
	return mergeSegs(out)
}

func mergeSegs(in []segment) []segment {
	var out []segment
	for _, s := range in {
		if s.text == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].changed == s.changed {
			out[n-1].text += s.text
			continue
		}
		out = append(out, s)
	}
	return out
}

// --- rendering into plain (pre-styled) rows for the viewport ---

// diffRow is one terminal row of rendered diff, carrying the data the
// view needs to style it.
type diffRow struct {
	op     op
	num    int // line number shown in the gutter (0 = none)
	marker string
	segs   []segment // when nil, plain text in text
	text   string
	gutter bool // part of the file header block
}

// renderUnified lays out d as unified diff rows.
func renderUnified(d fileDiff, wordDiff bool) []diffRow {
	var rows []diffRow
	if d.binary {
		return append(rows, diffRow{op: opContext, text: "binary file changed", gutter: true})
	}
	if d.isNew {
		rows = append(rows, diffRow{op: opAdd, text: "new file", gutter: true})
	}
	if d.isDel {
		rows = append(rows, diffRow{op: opDel, text: "file deleted", gutter: true})
	}
	for _, ln := range d.lines {
		r := diffRow{op: ln.op, segs: ln.segs, text: ln.text}
		switch ln.op {
		case opAdd:
			r.marker = "+"
			r.num = ln.newNum
			if !wordDiff {
				r.segs = nil
			}
		case opDel:
			r.marker = "-"
			r.num = ln.oldNum
			if !wordDiff {
				r.segs = nil
			}
		default:
			r.marker = " "
			r.num = ln.newNum
			r.segs = nil
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		rows = append(rows, diffRow{op: opContext, text: "(no textual changes)", gutter: true})
	}
	return rows
}

// renderSplit lays out d as side-by-side rows: left old, right new.
// Each returned row is [left, right]; either side may be nil (blank).
type splitCell struct {
	op   op
	num  int
	segs []segment
	text string
}

func renderSplit(d fileDiff, wordDiff bool) [][2]*splitCell {
	var rows [][2]*splitCell
	if d.binary {
		return [][2]*splitCell{{{op: opContext, text: "binary file changed"}, nil}}
	}
	var dels, adds []diffLine
	flush := func() {
		n := len(dels)
		if len(adds) > n {
			n = len(adds)
		}
		for i := 0; i < n; i++ {
			var l, r *splitCell
			if i < len(dels) {
				l = &splitCell{op: opDel, num: dels[i].oldNum, text: dels[i].text}
				if wordDiff {
					l.segs = dels[i].segs
				}
			}
			if i < len(adds) {
				r = &splitCell{op: opAdd, num: adds[i].newNum, text: adds[i].text}
				if wordDiff {
					r.segs = adds[i].segs
				}
			}
			rows = append(rows, [2]*splitCell{l, r})
		}
		dels, adds = nil, nil
	}
	for _, ln := range d.lines {
		switch ln.op {
		case opDel:
			dels = append(dels, ln)
		case opAdd:
			adds = append(adds, ln)
		default:
			flush()
			c := &splitCell{op: opContext, num: ln.newNum, text: ln.text}
			rows = append(rows, [2]*splitCell{c, c})
		}
	}
	flush()
	if len(rows) == 0 {
		c := &splitCell{op: opContext, text: "(no textual changes)"}
		rows = append(rows, [2]*splitCell{c, nil})
	}
	return rows
}

// foldContext collapses runs of context lines longer than 2*keep into
// a "N unchanged lines" marker. Used by the compact unified mode.
func foldContext(rows []diffRow, keep int) []diffRow {
	var out []diffRow
	i := 0
	for i < len(rows) {
		if rows[i].op != opContext || rows[i].gutter {
			out = append(out, rows[i])
			i++
			continue
		}
		j := i
		for j < len(rows) && rows[j].op == opContext && !rows[j].gutter {
			j++
		}
		run := rows[i:j]
		if len(run) <= keep*2+1 {
			out = append(out, run...)
		} else {
			out = append(out, run[:keep]...)
			out = append(out, diffRow{op: opContext, gutter: true,
				text: "⋮ " + itoa(len(run)-2*keep) + " unchanged lines"})
			out = append(out, run[len(run)-keep:]...)
		}
		i = j
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// truncate shortens s to at most n terminal cells (rune-based),
// appending an ellipsis when truncated.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// padRight right-pads s with spaces to n terminal cells (rune-based).
func padRight(s string, n int) string {
	w := len([]rune(s))
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}
