package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m model) View() tea.View {
	content := "starting…"
	if m.ready {
		body := m.renderBody()
		status := m.renderStatusBar()
		content = lipgloss.JoinVertical(lipgloss.Left, body, status)
		if m.showHelp {
			content = m.renderHelpOverlay(content)
		}
	}
	v := tea.NewView(content)
	v.AltScreen = true
	// Enable mouse cell motion so drag/click/wheel work like v1 WithMouseCellMotion.
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// renderStatusBar is the single bottom chrome line: brand + path on the
// left, stats + key hints on the right, separated from the body by a top
// border. It replaces the former header, stats bar, and footer.
func (m model) renderStatusBar() string {
	left := m.spinner.View() + m.st.headerBrand.Render(" flashdiff") +
		m.st.headerPath.Render("  "+m.root)

	var rightParts []string
	if m.filtering || m.filter.Value() != "" {
		rightParts = append(rightParts, m.st.filterPrompt.Render("filter: ")+m.filter.View())
	}
	rightParts = append(rightParts,
		m.styledStat(fmt.Sprintf("%d", m.totalScanned)+" tracked"),
		lipgloss.NewStyle().Foreground(m.st.p.add).Render(fmt.Sprintf("%d", m.eventCount))+
			m.st.headerPath.Render(" changes"),
	)
	if !m.lastEvent.IsZero() {
		ago := time.Since(m.lastEvent).Truncate(time.Second)
		rightParts = append(rightParts, m.st.headerPath.Render("⧗ ")+
			lipgloss.NewStyle().Foreground(m.st.p.primary).Render(ago.String()))
	}
	sep := m.st.headerPath.Render("  ·  ")
	right := strings.Join(rightParts, sep)

	// Right side: key hints normally; an apply/cancel hint while filtering.
	if m.filtering {
		right += m.st.headerPath.Render("  │  ") +
			m.st.footerKey.Render("enter") + m.st.footerDesc.Render(" apply  ") +
			m.st.footerKey.Render("esc") + m.st.footerDesc.Render(" cancel")
	} else {
		right += m.st.headerPath.Render("  │  ") + m.renderKeyHints()
	}

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	inner := m.width - 2 // account for header padding
	if leftW+rightW+1 > inner {
		// Too wide: drop the root path, keep just the brand.
		left = m.spinner.View() + m.st.headerBrand.Render(" flashdiff")
		leftW = lipgloss.Width(left)
	}
	if leftW+rightW+1 > inner {
		// Still too wide: drop key hints / extra stats from the right.
		right = strings.Join(rightParts, sep)
		rightW = lipgloss.Width(right)
	}
	gap := inner - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right

	// Top rule separating the status bar from the body, with a ┴ junction
	// where the vertical divider meets it so the two read as one joint.
	rule := []rune(strings.Repeat("─", m.width))
	if m.filesWidth >= 0 && m.filesWidth < len(rule) {
		rule[m.filesWidth] = '┴'
	}
	ruleLine := lipgloss.NewStyle().Foreground(m.st.p.border).Render(string(rule))

	return ruleLine + "\n" + m.st.header.Render(line)
}

// styledStat colors a stat value with the primary color.
func (m model) styledStat(s string) string {
	return lipgloss.NewStyle().Foreground(m.st.p.primary).Render(s)
}

// renderKeyHints renders the short key help inline (no background).
func (m model) renderKeyHints() string {
	var b strings.Builder
	for i, kb := range m.keys.ShortHelp() {
		if i > 0 {
			b.WriteString(m.st.footerDesc.Render("  "))
		}
		b.WriteString(m.st.footerKey.Render(kb.Help().Key))
		b.WriteString(m.st.footerDesc.Render(" " + kb.Help().Desc))
	}
	return b.String()
}

// --- body ---

func (m model) renderBody() string {
	// Chrome: the bottom status bar is 2 rows (its top-border rule + the
	// status text). Everything above that is body.
	const chromeH = 2
	bodyH := m.height - chromeH
	if bodyH < 3 {
		bodyH = 3
	}

	filesPane := m.renderFilesPane(bodyH)
	diffPane := m.renderDiffPane(bodyH)

	// Vertical divider: plain │ running the full body height, crossing the
	// title rule (┼). Its bottom end meets the status bar's top rule, which
	// draws the ┴ junction (see renderStatusBar).
	div := make([]string, bodyH)
	for i := range div {
		div[i] = "│"
	}
	if bodyH >= 2 {
		div[1] = "┼" // title rule row
	}
	divider := m.st.divider.Render(strings.Join(div, "\n"))

	return lipgloss.JoinHorizontal(lipgloss.Top, filesPane, divider, diffPane)
}

func (m model) renderFilesPane(h int) string {
	label := "FILES"
	if q := m.filter.Value(); q != "" {
		label += m.st.stats.Render(fmt.Sprintf(" (%d/%d)", len(m.visibleChanges()), len(m.changes)))
	}
	labelSt, ruleSt := m.st.paneTitle, m.st.rule
	if m.focus == paneFiles {
		labelSt, ruleSt = m.st.paneTitleOn, m.st.ruleActive
	}
	title := labelSt.Render(label)
	rule := ruleSt.Render(strings.Repeat("─", m.filesWidth))
	return lipgloss.JoinVertical(lipgloss.Left, title, rule, m.filesVP.View())
}

func (m model) renderDiffPane(h int) string {
	left := "DIFF"
	if c := m.selectedChange(); c != nil {
		p := m.st.p
		adds := lipgloss.NewStyle().Foreground(p.add).Render(fmt.Sprintf("+%d", c.diff.adds))
		dels := lipgloss.NewStyle().Foreground(p.del).Render(fmt.Sprintf("-%d", c.diff.dels))
		left += " " + c.rel + " " + adds + " " + dels
	}

	// Right-aligned cluster: diff mode, word granularity, scroll position.
	right := m.mode.String()
	if m.wordDiff {
		right += " · words"
	}
	if m.diffVP.TotalLineCount() > m.diffVP.Height() {
		pct := int(m.diffVP.ScrollPercent() * 100)
		right += fmt.Sprintf(" · %d%%", pct)
	}

	diffW := m.width - m.filesWidth - 1
	labelSt, ruleSt := m.st.paneTitle, m.st.rule
	if m.focus == paneDiff {
		labelSt, ruleSt = m.st.paneTitleOn, m.st.ruleActive
	}

	// Balance the title bar: "DIFF …" on the left, the mode cluster pushed
	// to the right edge of the pane. Fall back to a single space when the
	// pane is too narrow to hold both.
	leftR := labelSt.Render(left)
	rightR := m.st.stats.Render(right)
	gap := diffW - lipgloss.Width(leftR) - lipgloss.Width(rightR)
	if gap < 1 {
		gap = 1
	}
	title := leftR + strings.Repeat(" ", gap) + rightR
	rule := ruleSt.Render(strings.Repeat("─", diffW))
	return lipgloss.JoinVertical(lipgloss.Left, title, rule, m.diffVP.View())
}

// --- file list content ---

func (m model) renderFileList(width int) string {
	text := m.visibleTextChanges()
	bins := m.visibleBinaryChanges()
	if len(text) == 0 && len(bins) == 0 {
		msg := "◌ watching for changes…"
		if m.filter.Value() != "" {
			msg = "no files match the filter"
		}
		return m.st.empty.Width(width).Render(msg)
	}

	var b strings.Builder
	// Flat selection index: text[0..n) then bins[0..m). A section header
	// is rendered between the groups and is not selectable.
	idx := 0
	for _, c := range text {
		if idx > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.renderFileRow(c, idx == m.selected, width))
		idx++
	}
	if len(bins) > 0 {
		if idx > 0 {
			b.WriteByte('\n')
		}
		label := fmt.Sprintf("BINARIES (%d)", len(bins))
		b.WriteString(m.st.gutter.Render(padRight(label, width)))
		for _, c := range bins {
			b.WriteByte('\n')
			b.WriteString(m.renderFileRow(c, idx == m.selected, width))
			idx++
		}
	}
	return b.String()
}

func (m model) renderFileRow(c *change, selected bool, width int) string {
	icon, badge, st := "●", "M", m.st.fileRow
	switch {
	case c.deleted:
		icon, badge, st = "✖", "D", m.st.fileRowDel
	case c.isNew:
		icon, badge, st = "✚", "A", m.st.fileRowNew
	case c.binary:
		icon, badge, st = "◆", "B", m.st.fileRowBin
	}
	if selected {
		st = m.st.fileRowSel
	}
	name := truncate(c.rel, max(6, width-4))
	row := padRight(icon+" "+name, width-1) + m.st.gutter.Render(badge)
	return st.Render(row)
}

// --- diff content ---

func (m model) renderDiff(width int) string {
	c := m.selectedChange()
	if c == nil {
		return m.st.empty.Render("◌ diffs appear here as files are written")
	}
	if m.mode == modeSplit {
		return m.renderSplitDiff(*c, width)
	}
	rows := renderUnified(c.diff, m.wordDiff)
	if m.mode == modeCompact {
		rows = foldContext(rows, 3)
	}
	return m.renderUnifiedRows(rows, width, c.rel)
}

func (m model) renderUnifiedRows(rows []diffRow, width int, rel string) string {
	numW := 1
	for _, r := range rows {
		if n := len(itoa(r.num)); n > numW && !r.gutter {
			numW = n
		}
	}
	// Gutter block: right-aligned line number, then a │ rule, then a 1-cell
	// marker column (+/-/space) kept out of the content itself.
	sep := m.st.divider.Render("│")
	contentW := width - numW - 4 // num + space + │ + marker

	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		if r.gutter {
			b.WriteString(m.st.diffHeader.Render(truncate(r.text, width)))
			continue
		}
		gutter := m.st.gutter.Render(padLeft(itoa(r.num), numW)) + " " + sep
		textW := contentW
		var line string
		switch r.op {
		case opAdd:
			line = m.renderStyledLine(r, m.st.addLine, m.st.addWord, textW, "+")
		case opDel:
			line = m.renderStyledLine(r, m.st.delLine, m.st.delWord, textW, "-")
		default:
			line = m.renderContextLine(rel, r.text, textW)
		}
		b.WriteString(gutter + line)
	}
	return b.String()
}

// renderContextLine renders an unchanged line with chroma syntax
// highlighting when a lexer matches the file; otherwise plain text. The two
// leading spaces keep it aligned with the add/del lines: one for the marker
// column (blank for context), one for the marker/content gap.
func (m model) renderContextLine(rel, text string, textW int) string {
	if m.hl != nil {
		if painted := m.hl.paint(rel, truncate(text, textW)); painted != "" {
			return "  " + painted
		}
	}
	return m.st.ctxLine.Render("  " + truncate(text, textW))
}

// renderStyledLine renders one add/del row with optional word-level
// highlights, padded to the full content width so the background
// extends across the row.
func (m model) renderStyledLine(r diffRow, lineSt, wordSt lipgloss.Style, textW int, marker string) string {
	text := truncate(r.text, textW-1)
	if len(r.segs) == 0 {
		return lineSt.Render(marker + " " + padRight(text, textW-1))
	}
	var sb strings.Builder
	sb.WriteString(lineSt.Render(marker + " "))
	used := 0
	for _, s := range r.segs {
		if used >= textW-1 {
			break
		}
		t := truncate(s.text, textW-1-used)
		used += len(t)
		if s.changed {
			sb.WriteString(wordSt.Render(t))
		} else {
			sb.WriteString(lineSt.Render(t))
		}
	}
	if pad := textW - 1 - used; pad > 0 {
		sb.WriteString(lineSt.Render(strings.Repeat(" ", pad)))
	}
	return sb.String()
}
func (m model) renderSplitDiff(c change, width int) string {
	rows := renderSplit(c.diff, m.wordDiff)
	colW := (width - 1) / 2
	if colW < 10 {
		// not enough room: fall back to unified
		return m.renderUnifiedRows(renderUnified(c.diff, m.wordDiff), width, c.rel)
	}
	numW := 4
	var b strings.Builder
	for i, pair := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.renderSplitCell(pair[0], colW, numW, true))
		b.WriteString(m.st.divider.Render("│"))
		b.WriteString(m.renderSplitCell(pair[1], colW, numW, false))
	}
	return b.String()
}

func (m model) renderSplitCell(cell *splitCell, colW, numW int, left bool) string {
	sep := m.st.divider.Render("│")
	textW := colW - numW - 4 // num + space + │ + space
	if textW < 2 {
		textW = 2
	}
	if cell == nil {
		return strings.Repeat(" ", colW)
	}
	gutter := m.st.gutter.Render(padLeft(itoa(cell.num), numW)) + " " + sep + " "
	text := truncate(cell.text, textW)
	var body string
	switch cell.op {
	case opAdd:
		body = m.renderCellText(cell, m.st.addLine, m.st.addWord, text, textW)
	case opDel:
		body = m.renderCellText(cell, m.st.delLine, m.st.delWord, text, textW)
	default:
		body = m.st.ctxLine.Render(padRight(text, textW))
	}
	return gutter + body
}

func (m model) renderCellText(cell *splitCell, lineSt, wordSt lipgloss.Style, text string, textW int) string {
	if len(cell.segs) == 0 {
		return lineSt.Render(padRight(text, textW))
	}
	var sb strings.Builder
	used := 0
	for _, s := range cell.segs {
		if used >= textW {
			break
		}
		t := truncate(s.text, textW-used)
		used += len(t)
		if s.changed {
			sb.WriteString(wordSt.Render(t))
		} else {
			sb.WriteString(lineSt.Render(t))
		}
	}
	if pad := textW - used; pad > 0 {
		sb.WriteString(lineSt.Render(strings.Repeat(" ", pad)))
	}
	return sb.String()
}

// --- help overlay ---

func (m model) renderHelpOverlay(bg string) string {
	title := lipgloss.NewStyle().Foreground(m.st.p.primary).Bold(true).Render("flashdiff — keys")
	body := m.help.FullHelpView(m.keys.FullHelp())
	box := m.st.helpOverlay.Render(title + "\n\n" + body + "\n\n" + m.st.empty.Render("esc to close"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
