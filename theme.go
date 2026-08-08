package main

import (
	"github.com/alecthomas/chroma/v2"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// ThemeName is the chroma style the palette is derived from.
const ThemeName = "catppuccin-mocha"

// palette is the single source of truth for colors. It is derived from
// the chroma catppuccin-mocha style so the TUI matches that theme.
type palette struct {
	bg       lipgloss.Color // base
	surface  lipgloss.Color // surface0/1
	border   lipgloss.Color // surface2
	borderHi lipgloss.Color // overlay0
	primary  lipgloss.Color // blue (was keyword mauve accents)
	accent   lipgloss.Color // mauve
	text     lipgloss.Color // text
	muted    lipgloss.Color // overlay1 (comments)
	add      lipgloss.Color // green
	addBg    lipgloss.Color // green on surface
	del      lipgloss.Color // red
	delBg    lipgloss.Color // red on surface
	warn     lipgloss.Color // peach
}

// chromaColor resolves a token's foreground color from the style,
// falling back to the provided default when unset.
func chromaColor(st *chroma.Style, tok chroma.TokenType, def string) lipgloss.Color {
	if e := st.Get(tok); e.Colour.IsSet() {
		return lipgloss.Color(e.Colour.String())
	}
	return lipgloss.Color(def)
}

func chromaBg(st *chroma.Style, def string) lipgloss.Color {
	if e := st.Get(chroma.Background); e.Background.IsSet() {
		return lipgloss.Color(e.Background.String())
	}
	return lipgloss.Color(def)
}

func newPalette() palette {
	st := chromastyles.Get(ThemeName)
	// Derived from the chroma catppuccin-mocha style tokens; the
	// fallbacks are the canonical Catppuccin Mocha hex values.
	bg := chromaBg(st, "#1e1e2e") // base
	return palette{
		bg:       bg,
		surface:  lipgloss.Color("#313244"),                          // surface0
		border:   lipgloss.Color("#45475a"),                          // surface1
		borderHi: lipgloss.Color("#585b70"),                          // surface2
		primary:  chromaColor(st, chroma.NameFunction, "#89b4fa"),    // blue
		accent:   chromaColor(st, chroma.Keyword, "#cba6f7"),         // mauve
		text:     chromaColor(st, chroma.Text, "#cdd6f4"),            // text
		muted:    chromaColor(st, chroma.Comment, "#6c7086"),         // overlay1
		add:      chromaColor(st, chroma.GenericInserted, "#a6e3a1"), // green
		addBg:    lipgloss.Color("#24312b"),                          // green-tinted surface
		del:      chromaColor(st, chroma.GenericDeleted, "#f38ba8"),  // red
		delBg:    lipgloss.Color("#322430"),                          // red-tinted surface
		warn:     chromaColor(st, chroma.LiteralNumber, "#fab387"),   // peach
	}
}

// styles holds every lipgloss style used by the app, built once from p.
type styles struct {
	p palette

	header       lipgloss.Style
	headerBrand  lipgloss.Style
	headerPath   lipgloss.Style
	stats        lipgloss.Style
	paneTitle    lipgloss.Style // unfocused pane title (muted label)
	paneTitleOn  lipgloss.Style // focused pane title (primary label)
	rule         lipgloss.Style // separator under unfocused pane title
	ruleActive   lipgloss.Style // separator under focused pane title
	fileRow      lipgloss.Style
	fileRowSel   lipgloss.Style
	fileRowNew   lipgloss.Style
	fileRowDel   lipgloss.Style
	fileRowBin   lipgloss.Style
	gutter       lipgloss.Style
	addLine      lipgloss.Style
	addWord      lipgloss.Style
	delLine      lipgloss.Style
	delWord      lipgloss.Style
	ctxLine      lipgloss.Style
	diffHeader   lipgloss.Style
	divider      lipgloss.Style
	footer       lipgloss.Style
	footerKey    lipgloss.Style
	footerDesc   lipgloss.Style
	empty        lipgloss.Style
	helpOverlay  lipgloss.Style
	helpKey      lipgloss.Style
	filterPrompt lipgloss.Style
	badgeNew     lipgloss.Style
	badgeDel     lipgloss.Style
	badgeBin     lipgloss.Style
}

func newStyles(p palette) styles {
	base := lipgloss.NewStyle().Background(p.bg)
	return styles{
		p: p,
		// The status bar's top rule is drawn manually in renderStatusBar so it
		// can carry a ┴ junction where the vertical divider meets it.
		header: lipgloss.NewStyle().
			Foreground(p.muted).
			Padding(0, 1),
		headerBrand: lipgloss.NewStyle().
			Foreground(p.primary).
			Bold(true),
		headerPath: lipgloss.NewStyle().
			Foreground(p.muted),
		stats: base.
			Foreground(p.muted).
			Padding(0, 1),
		// Pane titles: plain label row. The focused pane's label is bold
		// primary; the unfocused pane's is muted. A separator rule is drawn
		// beneath the titles by the view (see rule / ruleActive).
		paneTitle: lipgloss.NewStyle().
			Foreground(p.muted).
			Padding(0, 1),
		paneTitleOn: lipgloss.NewStyle().
			Foreground(p.primary).
			Bold(true).
			Padding(0, 1),
		rule: lipgloss.NewStyle().
			Foreground(p.border),
		ruleActive: lipgloss.NewStyle().
			Foreground(p.primary),
		fileRow: base.
			Foreground(p.text).
			Padding(0, 1),
		fileRowSel: lipgloss.NewStyle().
			Background(p.surface).
			Foreground(p.primary).
			Bold(true).
			Padding(0, 1),
		fileRowNew: base.Foreground(p.add).Padding(0, 1),
		fileRowDel: base.Foreground(p.del).Padding(0, 1),
		fileRowBin: base.Foreground(p.accent).Padding(0, 1),
		gutter:     base.Foreground(p.muted),
		addLine: lipgloss.NewStyle().
			Foreground(p.add).
			Background(p.addBg),
		addWord: lipgloss.NewStyle().
			Foreground(p.bg).
			Background(p.add).
			Bold(true),
		delLine: lipgloss.NewStyle().
			Foreground(p.del).
			Background(p.delBg),
		delWord: lipgloss.NewStyle().
			Foreground(p.bg).
			Background(p.del).
			Bold(true),
		ctxLine:    base.Foreground(p.text),
		diffHeader: base.Foreground(p.accent).Bold(true),
		divider:    base.Foreground(p.border),
		footer: base.
			Foreground(p.muted).
			Padding(0, 1),
		footerKey: lipgloss.NewStyle().
			Foreground(p.primary).
			Bold(true),
		footerDesc: lipgloss.NewStyle().
			Foreground(p.muted),
		empty: base.
			Foreground(p.muted).
			Italic(true),
		helpOverlay: lipgloss.NewStyle().
			Foreground(p.text).
			Background(p.surface).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.primary).
			Padding(1, 2),
		helpKey: lipgloss.NewStyle().
			Foreground(p.primary).
			Bold(true),
		filterPrompt: lipgloss.NewStyle().
			Foreground(p.primary).
			Bold(true),
		badgeNew: lipgloss.NewStyle().Foreground(p.add).Bold(true),
		badgeDel: lipgloss.NewStyle().Foreground(p.del).Bold(true),
		badgeBin: lipgloss.NewStyle().Foreground(p.accent).Bold(true),
	}
}
