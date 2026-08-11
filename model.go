package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

type pane int

const (
	paneFiles pane = iota
	paneDiff
)

type diffMode int

const (
	modeUnified diffMode = iota
	modeCompact
	modeSplit
)

func (m diffMode) String() string {
	switch m {
	case modeCompact:
		return "compact"
	case modeSplit:
		return "split"
	default:
		return "unified"
	}
}

// next cycles diff modes starting from the compact default:
// compact → unified → split → compact.
func (m diffMode) next() diffMode {
	switch m {
	case modeCompact:
		return modeUnified
	case modeUnified:
		return modeSplit
	default: // modeSplit
		return modeCompact
	}
}

// change is one observed file modification event.
type change struct {
	rel     string
	diff    fileDiff
	when    time.Time
	deleted bool
	isNew   bool
	binary  bool
}

type tickMsg time.Time

type rescanDoneMsg struct {
	count int
}

type model struct {
	cfg     config
	root    string
	st      styles
	store   *fileStore
	watcher *watcher
	ign     *ignoreSet
	events  chan fileEventMsg

	changes []*change // newest first, one per file
	// selected is an index into visibleChanges(), or -1 when nothing
	// should drive the DIFF pane (e.g. only binary noise so far).
	selected int
	focus    pane
	mode     diffMode
	wordDiff bool

	filesVP viewport.Model
	diffVP  viewport.Model
	filter  textinput.Model
	help    help.Model
	spinner spinner.Model
	keys    keyMap
	hl      *highlighter

	filtering bool
	showHelp  bool

	width, height int
	filesWidth    int // outer width incl. border; draggable

	totalScanned int
	eventCount   int
	started      time.Time
	lastEvent    time.Time

	dragging bool
	ready    bool
}

func newModel(cfg config) (model, error) {
	root, err := filepath.Abs(cfg.path)
	if err != nil {
		return model{}, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return model{}, err
	}
	if !info.IsDir() {
		return model{}, &os.PathError{Op: "watch", Path: cfg.path, Err: os.ErrInvalid}
	}

	if cfg.theme != "" {
		ThemeName = cfg.theme
	}
	p := newPalette()
	ign := newIgnoreSet(cfg.includes, cfg.excludes)
	store := newFileStore()

	snaps, err := scan(root, ign, cfg.respectVCS)
	if err != nil {
		return model{}, err
	}
	store.reset(snaps)

	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "filter files…"
	ti.CharLimit = 128

	m := model{
		cfg:          cfg,
		root:         root,
		st:           newStyles(p),
		store:        store,
		ign:          ign,
		events:       make(chan fileEventMsg, 256),
		filter:       ti,
		help:         help.New(),
		keys:         newKeyMap(),
		hl:           newHighlighter(),
		mode:         modeCompact,
		wordDiff:     true,
		selected:     -1,
		totalScanned: len(snaps),
		started:      time.Now(),
		filesWidth:   30,
	}
	m.help.Styles.ShortKey = m.st.footerKey
	m.help.Styles.ShortDesc = m.st.footerDesc
	m.help.Styles.ShortSeparator = m.st.footerDesc

	// A small "pulse" spinner signals that the watcher is live. It sits in
	// the status bar where the static brand icon used to be. The default FPS
	// is a quick flash; slow it down so it reads as a calm pulse.
	m.spinner = spinner.New(spinner.WithSpinner(spinner.Pulse))
	m.spinner.Spinner.FPS = time.Millisecond * 350
	m.spinner.Style = m.spinner.Style.Foreground(p.primary)

	w, err := startWatcher(root, ign, cfg.respectVCS, func(msg fileEventMsg) {
		select {
		case m.events <- msg:
		default: // queue full: drop rather than block the watcher
		}
	})
	if err != nil {
		return model{}, err
	}
	m.watcher = w
	return m, nil
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.waitForEvent(), m.tick(), m.spinner.Tick)
}

// waitForEvent blocks until the watcher delivers the next fs event.
func (m model) waitForEvent() tea.Cmd {
	return func() tea.Msg {
		return <-m.events
	}
}

func (m model) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.layout()
		return m, nil

	case fileEventMsg:
		m.applyFileEvent(msg)
		return m, m.waitForEvent()

	case rescanDoneMsg:
		m.totalScanned = msg.count
		return m, nil

	case tickMsg:
		return m, m.tick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseMotionMsg, tea.MouseWheelMsg:
		return m.handleMouse(msg)

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *model) applyFileEvent(ev fileEventMsg) {
	abs := filepath.Join(m.root, filepath.FromSlash(ev.rel))

	if ev.deleted {
		old, ok := m.store.get(ev.rel)
		m.store.remove(ev.rel)
		if !ok || old.binary {
			m.upsertChange(&change{rel: ev.rel, when: time.Now(), deleted: true, binary: true,
				diff: fileDiff{rel: ev.rel, binary: true, isDel: true}})
		} else {
			d := computeDiff(ev.rel, old.content, "", false)
			d.isDel = true
			m.upsertChange(&change{rel: ev.rel, when: time.Now(), deleted: true, diff: d})
		}
		m.bumpEvent()
		return
	}

	content, binary, err := readFile(abs)
	if err != nil {
		// The file vanished between event and read (or is unreadable):
		// record it as a deletion so the baseline never goes stale.
		if os.IsNotExist(err) {
			m.applyFileEvent(fileEventMsg{rel: ev.rel, deleted: true})
		}
		return
	}
	old, existed := m.store.get(ev.rel)
	// Binary files have no stored content (always ""), so comparing
	// content would suppress all binary changes. Treat any event as a
	// real change when either side is binary.
	if existed && !binary && old.binary == binary && old.content == content {
		return // no effective change (e.g. touch with same content)
	}
	m.store.put(ev.rel, snapshot{content: content, binary: binary})

	c := &change{rel: ev.rel, when: time.Now(), isNew: !existed, binary: binary}
	if binary {
		c.diff = fileDiff{rel: ev.rel, binary: true, isNew: !existed}
	} else {
		c.diff = computeDiff(ev.rel, old.content, content, false)
		c.diff.isNew = !existed
	}
	m.upsertChange(c)
	m.bumpEvent()
}

func (m *model) bumpEvent() {
	m.eventCount++
	m.lastEvent = time.Now()
}

// upsertChange inserts or replaces the entry for c.rel, keeping the
// list newest-first. Only text (diffable) changes auto-select and drive
// the DIFF pane. Binary changes land in the BINARIES segment and never
// steal focus or fill the right pane — selected stays on the last text
// file (or -1) until the next text change or manual navigation.
func (m *model) upsertChange(c *change) {
	for i, ex := range m.changes {
		if ex.rel == c.rel {
			m.changes = append(m.changes[:i], m.changes[i+1:]...)
			break
		}
	}
	m.changes = append([]*change{c}, m.changes...)

	if !c.binary {
		for i, ex := range m.visibleChanges() {
			if ex.rel == c.rel {
				m.selected = i
				break
			}
		}
	} else {
		// Keep DIFF on the previous text selection. If the old index is
		// now out of range or points at a binary, fall back to the first
		// text row, or -1 when there is no text change at all.
		m.reanchorSelectionAfterBinary()
	}
	m.syncViews()
}

// reanchorSelectionAfterBinary keeps the DIFF pane on text content after
// a binary list update. selected == -1 means "show the empty DIFF state".
func (m *model) reanchorSelectionAfterBinary() {
	textN := len(m.visibleTextChanges())
	visN := len(m.visibleChanges())
	if textN == 0 {
		m.selected = -1
		return
	}
	if m.selected < 0 || m.selected >= visN || m.selected >= textN {
		m.selected = 0 // first text row (text is always listed first)
	}
}

// matchFilter reports whether rel passes the current filter query.
func (m *model) matchFilter(rel string) bool {
	q := strings.TrimSpace(strings.ToLower(m.filter.Value()))
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(rel), q)
}

// visibleTextChanges is the primary FILES list: non-binary changes only.
func (m *model) visibleTextChanges() []*change {
	var out []*change
	for _, c := range m.changes {
		if !c.binary && m.matchFilter(c.rel) {
			out = append(out, c)
		}
	}
	return out
}

// visibleBinaryChanges is the dedicated BINARIES segment under FILES.
func (m *model) visibleBinaryChanges() []*change {
	var out []*change
	for _, c := range m.changes {
		if c.binary && m.matchFilter(c.rel) {
			out = append(out, c)
		}
	}
	return out
}

// visibleChanges is text then binaries — the flat order used for selection
// and keyboard navigation. The renderer inserts a section header between
// the two groups; headers are not selectable.
func (m *model) visibleChanges() []*change {
	text := m.visibleTextChanges()
	bins := m.visibleBinaryChanges()
	if len(bins) == 0 {
		return text
	}
	out := make([]*change, 0, len(text)+len(bins))
	out = append(out, text...)
	out = append(out, bins...)
	return out
}

func (m *model) selectedChange() *change {
	vis := m.visibleChanges()
	if m.selected < 0 || len(vis) == 0 || m.selected >= len(vis) {
		return nil
	}
	return vis[m.selected]
}

// layout recomputes component sizes from the terminal size.
func (m *model) layout() {
	if !m.ready {
		return
	}
	const chromeH = 2 // bottom status bar (rule + text)
	bodyH := m.height - chromeH
	if bodyH < 3 {
		bodyH = 3
	}
	if m.filesWidth < 18 {
		m.filesWidth = 18
	}
	if maxFiles := m.width * 45 / 100; m.filesWidth > maxFiles {
		m.filesWidth = maxFiles
	}
	diffW := m.width - m.filesWidth - 1 // 1-col divider
	if diffW < 20 {
		diffW = 20
	}

	m.filesVP.SetWidth(m.filesWidth)
	m.filesVP.SetHeight(bodyH - 2) // title row + its separator
	m.diffVP.SetWidth(diffW)
	m.diffVP.SetHeight(bodyH - 2)
	m.help.SetWidth(m.width)
	m.syncViews()
}

// syncViews re-renders both viewports from current state.
func (m *model) syncViews() {
	if !m.ready {
		return
	}
	m.filesVP.SetContent(m.renderFileList(m.filesVP.Width()))
	m.diffVP.SetContent(m.renderDiff(m.diffVP.Width()))
	m.ensureSelectionVisible()
}

// selectionChanged re-renders and resets the diff scroll to the top,
// used whenever the user moves the file selection.
func (m *model) selectionChanged() {
	m.diffVP.SetYOffset(0)
	m.syncViews()
}

func (m *model) ensureSelectionVisible() {
	if m.selected < 0 {
		return
	}
	// Visual row accounts for the non-selectable BINARIES header. The header
	// is shown whenever any binary is listed (even if there is no text group).
	textN := len(m.visibleTextChanges())
	visRow := m.selected
	if len(m.visibleBinaryChanges()) > 0 && m.selected >= textN {
		visRow = m.selected + 1
	}
	if visRow < m.filesVP.YOffset() {
		m.filesVP.SetYOffset(visRow)
	}
	if bottom := m.filesVP.YOffset() + m.filesVP.Height() - 1; visRow > bottom {
		m.filesVP.SetYOffset(visRow - m.filesVP.Height() + 1)
	}
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch msg.String() {
		case "enter":
			m.filtering = false
			m.filter.Blur()
			return m, nil
		case "esc":
			m.filtering = false
			m.filter.Blur()
			m.filter.SetValue("")
			m.selected = -1
			m.syncViews()
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		// Prefer first text match after filter edits; -1 if none.
		if len(m.visibleTextChanges()) > 0 {
			m.selected = 0
		} else {
			m.selected = -1
		}
		m.syncViews()
		return m, cmd
	}

	if m.showHelp {
		switch msg.String() {
		case "q", "esc", "?":
			m.showHelp = false
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		if m.watcher != nil {
			m.watcher.close()
		}
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		if m.focus == paneFiles {
			m.focus = paneDiff
		} else {
			m.focus = paneFiles
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		m.focus = paneDiff
		return m, nil

	case key.Matches(msg, m.keys.Mode):
		m.mode = m.mode.next()
		m.syncViews()
		return m, nil

	case key.Matches(msg, m.keys.WordDiff):
		m.wordDiff = !m.wordDiff
		m.syncViews()
		return m, nil

	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
		return m, m.filter.Focus()

	case key.Matches(msg, m.keys.Clear):
		m.changes = nil
		m.selected = -1
		m.syncViews()
		return m, nil

	case key.Matches(msg, m.keys.Rescan):
		return m, m.rescan()
	}

	if m.focus == paneFiles {
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
				m.selectionChanged()
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			n := len(m.visibleChanges())
			if n == 0 {
				return m, nil
			}
			if m.selected < 0 {
				m.selected = 0
				m.selectionChanged()
			} else if m.selected < n-1 {
				m.selected++
				m.selectionChanged()
			}
			return m, nil
		case key.Matches(msg, m.keys.Top):
			if len(m.visibleChanges()) > 0 {
				m.selected = 0
				m.selectionChanged()
			}
			return m, nil
		case key.Matches(msg, m.keys.Bottom):
			if n := len(m.visibleChanges()); n > 0 {
				m.selected = n - 1
				m.selectionChanged()
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.filesVP, cmd = m.filesVP.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, m.keys.Top):
		m.diffVP.GotoTop()
		return m, nil
	case key.Matches(msg, m.keys.Bottom):
		m.diffVP.GotoBottom()
		return m, nil
	}
	var cmd tea.Cmd
	m.diffVP, cmd = m.diffVP.Update(msg)
	return m, cmd
}

func (m model) handleMouse(msg tea.Msg) (tea.Model, tea.Cmd) {
	dividerX := m.filesWidth // 0-based column of the divider
	switch msg := msg.(type) {
	case tea.MouseMotionMsg:
		if m.dragging {
			m.filesWidth = msg.X + 1
			m.layout()
		}
		return m, nil
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if msg.X >= dividerX-1 && msg.X <= dividerX+1 && msg.Y > 2 {
				m.dragging = true
				return m, nil
			}
			if msg.X < m.filesWidth && msg.Y >= 4 {
				// Map viewport row -> selection index, skipping the
				// non-selectable "BINARIES" header when present.
				row := msg.Y - 4 + m.filesVP.YOffset()
				textN := len(m.visibleTextChanges())
				binsN := len(m.visibleBinaryChanges())
				sel := -1
				if row >= 0 && row < textN {
					sel = row
				} else if binsN > 0 {
					// text rows, then 1 header row, then binary rows
					binRow := row - textN - 1
					if binRow >= 0 && binRow < binsN {
						sel = textN + binRow
					}
				}
				if sel >= 0 {
					m.selected = sel
					m.focus = paneFiles
					m.syncViews()
				}
			}
		}
		return m, nil
	case tea.MouseReleaseMsg:
		m.dragging = false
		return m, nil
	case tea.MouseWheelMsg:
		if msg.X < m.filesWidth {
			var cmd tea.Cmd
			m.filesVP, cmd = m.filesVP.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.diffVP, cmd = m.diffVP.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) rescan() tea.Cmd {
	root, ign, respectVCS, store := m.root, m.ign, m.cfg.respectVCS, m.store
	return func() tea.Msg {
		snaps, err := scan(root, ign, respectVCS)
		if err != nil {
			return rescanDoneMsg{}
		}
		store.reset(snaps)
		return rescanDoneMsg{count: len(snaps)}
	}
}
