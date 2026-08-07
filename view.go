package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if !m.ready {
		return "starting…"
	}
	header := m.renderHeader()
	stats := m.renderStats()
	body := m.renderBody()
	footer := m.renderFooter()

	out := lipgloss.JoinVertical(lipgloss.Left, header, stats, body, footer)
	if m.showHelp {
		out = m.renderHelpOverlay(out)
	}
	return out
}

// --- header / stats / footer ---

func (m model) renderHeader() string {
	brand := m.st.headerBrand.Render("◉ flashdiff")
	path := m.st.headerPath.Render("  " + m.root)
	right := ""
	if m.filtering || m.filter.Value() != "" {
		right = m.st.filterPrompt.Render("filter: ") + m.filter.View()
	}
	gap := m.width - lipgloss.Width(brand) - lipgloss.Width(path) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := brand + path + strings.Repeat(" ", gap) + right
	return m.st.header.Width(m.width).Render(line)
}

func (m model) renderStats() string {
	p := m.st.p
	parts := []string{
		m.styledStat(fmt.Sprintf("%d", m.totalScanned)+" tracked", 0),
		lipgloss.NewStyle().Foreground(p.add).Render(fmt.Sprintf("%d", m.eventCount)) +
			m.st.stats.Render(" changes"),
	}
	if !m.lastEvent.IsZero() {
		ago := time.Since(m.lastEvent).Truncate(time.Second)
		parts = append(parts, m.st.stats.Render("last ")+
			lipgloss.NewStyle().Foreground(p.primary).Render(ago.String()+" ago"))
	}
	return m.st.stats.Width(m.width).Render(strings.Join(parts, m.st.stats.Render("  ·  ")))
}

func (m model) styledStat(s string, _ int) string {
	return lipgloss.NewStyle().Foreground(m.st.p.primary).Render(s)
}

func (m model) renderFooter() string {
	if m.filtering {
		hint := m.st.footerDesc.Render("enter: apply · esc: cancel")
		return m.st.footer.Width(m.width).Render(m.filter.View() + "  " + hint)
	}
	return m.help.View(m.keys)
}

// --- body ---

func (m model) renderBody() string {
	const chromeH = 3
	bodyH := m.height - chromeH
	if bodyH < 3 {
		bodyH = 3
	}

	filesPane := m.renderFilesPane(bodyH)
	diffPane := m.renderDiffPane(bodyH)

	divider := m.st.divider.Render(strings.Repeat("│\n", bodyH-1) + "│")

	return lipgloss.JoinHorizontal(lipgloss.Top, filesPane, divider, diffPane)
}

func (m model) renderFilesPane(h int) string {
	label := "FILES"
	if q := m.filter.Value(); q != "" {
		label += m.st.stats.Render(fmt.Sprintf(" (%d/%d)", len(m.visibleChanges()), len(m.changes)))
	}
	titleSt := m.st.paneTitle
	if m.focus == paneFiles {
		titleSt = m.st.paneTitleOn
	}
	title := titleSt.Width(m.filesWidth).Render(label)
	return lipgloss.JoinVertical(lipgloss.Left, title, m.filesVP.View())
}

func (m model) renderDiffPane(h int) string {
	label := "DIFF"
	if c := m.selectedChange(); c != nil {
		p := m.st.p
		adds := lipgloss.NewStyle().Foreground(p.add).Render(fmt.Sprintf("+%d", c.diff.adds))
		dels := lipgloss.NewStyle().Foreground(p.del).Render(fmt.Sprintf("-%d", c.diff.dels))
		label += " " + c.rel + " " + adds + " " + dels
	}
	label += m.st.stats.Render(fmt.Sprintf("  · %s", m.mode)) +
		m.st.stats.Render(map[bool]string{true: " · words", false: ""}[m.wordDiff])
	// scroll position indicator
	if m.diffVP.TotalLineCount() > m.diffVP.Height {
		pct := int(m.diffVP.ScrollPercent() * 100)
		label += m.st.stats.Render(fmt.Sprintf("  · %d%%", pct))
	}

	diffW := m.width - m.filesWidth - 1
	titleSt := m.st.paneTitle
	if m.focus == paneDiff {
		titleSt = m.st.paneTitleOn
	}
	title := titleSt.Width(diffW).Render(label)
	return lipgloss.JoinVertical(lipgloss.Left, title, m.diffVP.View())
}

// --- file list content ---

func (m model) renderFileList(width int) string {
	vis := m.visibleChanges()
	if len(vis) == 0 {
		msg := "◌ watching for changes…"
		if m.filter.Value() != "" {
			msg = "no files match the filter"
		}
		return m.st.empty.Width(width).Render(msg)
	}
	var b strings.Builder
	for i, c := range vis {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.renderFileRow(c, i == m.selected, width))
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
	contentW := width - numW - 3 // gutter + marker + spaces

	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		if r.gutter {
			b.WriteString(m.st.diffHeader.Render(truncate(r.text, width)))
			continue
		}
		gutter := m.st.gutter.Render(padRight(itoa(r.num), numW))
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
		b.WriteString(gutter + " " + line)
	}
	return b.String()
}

// renderContextLine renders an unchanged line with chroma syntax
// highlighting when a lexer matches the file; otherwise plain text.
func (m model) renderContextLine(rel, text string, textW int) string {
	if m.hl != nil {
		if painted := m.hl.paint(rel, truncate(text, textW)); painted != "" {
			return " " + painted
		}
	}
	return m.st.ctxLine.Render(" " + truncate(text, textW))
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
	textW := colW - numW - 2
	if textW < 2 {
		textW = 2
	}
	if cell == nil {
		return strings.Repeat(" ", colW)
	}
	gutter := m.st.gutter.Render(padRight(itoa(cell.num), numW))
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
	return gutter + " " + body
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
