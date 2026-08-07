package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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

	changes  []*change // newest first, one per file
	selected int
	focus    pane
	mode     diffMode
	wordDiff bool

	filesVP viewport.Model
	diffVP  viewport.Model
	filter  textinput.Model
	help    help.Model
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
		mode:         modeUnified,
		wordDiff:     true,
		totalScanned: len(snaps),
		started:      time.Now(),
		filesWidth:   30,
	}
	m.help.Styles.ShortKey = m.st.footerKey
	m.help.Styles.ShortDesc = m.st.footerDesc
	m.help.Styles.ShortSeparator = m.st.footerDesc

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
	return tea.Batch(m.waitForEvent(), m.tick())
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

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
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
	if existed && old.binary == binary && old.content == content {
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
// list newest-first, and selects it.
func (m *model) upsertChange(c *change) {
	for i, ex := range m.changes {
		if ex.rel == c.rel {
			m.changes = append(m.changes[:i], m.changes[i+1:]...)
			break
		}
	}
	m.changes = append([]*change{c}, m.changes...)
	for i, ex := range m.visibleChanges() {
		if ex.rel == c.rel {
			m.selected = i
			break
		}
	}
	m.syncViews()
}

// visibleChanges applies the filter query to the change list.
func (m *model) visibleChanges() []*change {
	q := strings.TrimSpace(strings.ToLower(m.filter.Value()))
	if q == "" {
		return m.changes
	}
	var out []*change
	for _, c := range m.changes {
		if strings.Contains(strings.ToLower(c.rel), q) {
			out = append(out, c)
		}
	}
	return out
}

func (m *model) selectedChange() *change {
	vis := m.visibleChanges()
	if len(vis) == 0 || m.selected >= len(vis) {
		return nil
	}
	return vis[m.selected]
}

// layout recomputes component sizes from the terminal size.
func (m *model) layout() {
	if !m.ready {
		return
	}
	const chromeH = 3 // header + stats + footer
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

	m.filesVP.Width = m.filesWidth
	m.filesVP.Height = bodyH - 2 // title row + its underline
	m.diffVP.Width = diffW
	m.diffVP.Height = bodyH - 2
	m.help.Width = m.width
	m.syncViews()
}

// syncViews re-renders both viewports from current state.
func (m *model) syncViews() {
	if !m.ready {
		return
	}
	m.filesVP.SetContent(m.renderFileList(m.filesVP.Width))
	m.diffVP.SetContent(m.renderDiff(m.diffVP.Width))
	m.ensureSelectionVisible()
}

// selectionChanged re-renders and resets the diff scroll to the top,
// used whenever the user moves the file selection.
func (m *model) selectionChanged() {
	m.diffVP.YOffset = 0
	m.syncViews()
}

func (m *model) ensureSelectionVisible() {
	if m.selected < m.filesVP.YOffset {
		m.filesVP.YOffset = m.selected
	}
	if bottom := m.filesVP.YOffset + m.filesVP.Height - 1; m.selected > bottom {
		m.filesVP.YOffset = m.selected - m.filesVP.Height + 1
	}
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			m.selected = 0
			m.syncViews()
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.selected = 0
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
		m.mode = (m.mode + 1) % 3
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
		m.selected = 0
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
			if m.selected < len(m.visibleChanges())-1 {
				m.selected++
				m.selectionChanged()
			}
			return m, nil
		case key.Matches(msg, m.keys.Top):
			m.selected = 0
			m.selectionChanged()
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

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	dividerX := m.filesWidth // 0-based column of the divider
	switch msg.Button {
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionMotion {
			if m.dragging {
				m.filesWidth = msg.X + 1
				m.layout()
			}
			return m, nil
		}
		if msg.Action == tea.MouseActionPress {
			if msg.X >= dividerX-1 && msg.X <= dividerX+1 && msg.Y > 2 {
				m.dragging = true
				return m, nil
			}
			if msg.X < m.filesWidth && msg.Y >= 4 {
				row := msg.Y - 4 + m.filesVP.YOffset
				if row >= 0 && row < len(m.visibleChanges()) {
					m.selected = row
					m.focus = paneFiles
					m.syncViews()
				}
			}
		}
		if msg.Action == tea.MouseActionRelease {
			m.dragging = false
		}
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
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
