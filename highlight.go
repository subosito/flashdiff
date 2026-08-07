package main

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
)

// highlighter applies chroma syntax highlighting (catppuccin-mocha) to
// source lines shown in the diff view, keyed by file extension.
type highlighter struct {
	style *chroma.Style
}

func newHighlighter() *highlighter {
	return &highlighter{style: chromastyles.Get(ThemeName)}
}

// lexerFor picks a chroma lexer from the file's extension/name.
func lexerFor(rel string) chroma.Lexer {
	if l := lexers.Match(filepath.Base(rel)); l != nil {
		return chroma.Coalesce(l)
	}
	if ext := filepath.Ext(rel); ext != "" {
		if l := lexers.Get(ext[1:]); l != nil {
			return chroma.Coalesce(l)
		}
	}
	return nil
}

// paint renders a source line through chroma and returns ANSI-colored
// text for the given token range of one line. It returns "" when no
// lexer applies (caller falls back to plain styling).
//
// Because lexing is line-oriented, we lex the single line; this is fast
// and good enough for diff rows (multi-line constructs may color
// slightly differently, which is acceptable for a minimal TUI).
func (h *highlighter) paint(rel, line string) string {
	lx := lexerFor(rel)
	if lx == nil {
		return ""
	}
	it, err := lx.Tokenise(nil, line)
	if err != nil {
		return ""
	}
	var buf bytes.Buffer
	if err := formatters.TTY16m.Format(&buf, h.style, it); err != nil {
		return ""
	}
	return strings.TrimSuffix(buf.String(), "\n")
}
