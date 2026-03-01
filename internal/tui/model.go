package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/syasoda/lazylogs/internal/entry"
)

type viewMode int

const (
	modeList viewMode = iota
	modeDetail
	modeSearch
	modeTimeFilter
	modeTimeCustom
)

// Model is the bubbletea model for lazylogs.
type Model struct {
	entries  []entry.Entry
	filtered []int
	cursor   int
	offset   int
	width    int
	height   int

	// Filters.
	search   string
	levels   map[entry.Level]bool
	timeFrom time.Time
	timeTo   time.Time

	// Display.
	columns []string // empty = default, non-empty = table mode

	// State.
	mode      viewMode
	following bool
	done      bool

	// Components.
	searchInput textinput.Model
	timeInput   textinput.Model
	entryChan   <-chan entry.Entry
}

// NewModel creates a new lazylogs TUI model.
func NewModel(ch <-chan entry.Entry, columns []string) *Model {
	si := textinput.New()
	si.Placeholder = "search..."
	si.CharLimit = 256

	ti := textinput.New()
	ti.Placeholder = "HH:MM-HH:MM or HH:MM:SS-HH:MM:SS"
	ti.CharLimit = 64

	levels := map[entry.Level]bool{
		entry.LevelUnknown: true,
		entry.LevelTrace:   true,
		entry.LevelDebug:   true,
		entry.LevelInfo:    true,
		entry.LevelWarn:    true,
		entry.LevelError:   true,
		entry.LevelFatal:   true,
	}

	return &Model{
		entryChan:   ch,
		levels:      levels,
		columns:     columns,
		following:   true,
		searchInput: si,
		timeInput:   ti,
	}
}

// --- Messages ---

type batchMsg []entry.Entry
type entriesDoneMsg struct{}

func readBatch(ch <-chan entry.Entry) tea.Cmd {
	return func() tea.Msg {
		var batch []entry.Entry
		timeout := time.After(16 * time.Millisecond)
		for {
			select {
			case e, ok := <-ch:
				if !ok {
					if len(batch) > 0 {
						return batchMsg(batch)
					}
					return entriesDoneMsg{}
				}
				batch = append(batch, e)
				if len(batch) >= 1000 {
					return batchMsg(batch)
				}
			case <-timeout:
				if len(batch) > 0 {
					return batchMsg(batch)
				}
				// Channel still open but no data yet; retry.
				return batchMsg(nil)
			}
		}
	}
}

func (m *Model) Init() tea.Cmd {
	return readBatch(m.entryChan)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case batchMsg:
		if len(msg) == 0 {
			return m, readBatch(m.entryChan)
		}
		startIdx := len(m.entries)
		m.entries = append(m.entries, []entry.Entry(msg)...)
		m.filterRange(startIdx)
		if m.following {
			m.scrollToBottom()
		}
		return m, readBatch(m.entryChan)

	case entriesDoneMsg:
		m.done = true
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Search input mode.
	if m.mode == modeSearch {
		switch msg.String() {
		case "enter":
			m.search = m.searchInput.Value()
			m.mode = modeList
			m.refilter()
			m.cursor = 0
			m.offset = 0
			return m, nil
		case "esc":
			m.mode = modeList
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}
	}

	// Time custom input mode.
	if m.mode == modeTimeCustom {
		switch msg.String() {
		case "enter":
			m.applyCustomTimeFilter(m.timeInput.Value())
			m.mode = modeList
			return m, nil
		case "esc":
			m.mode = modeList
			return m, nil
		default:
			var cmd tea.Cmd
			m.timeInput, cmd = m.timeInput.Update(msg)
			return m, cmd
		}
	}

	// Time filter preset mode.
	if m.mode == modeTimeFilter {
		switch msg.String() {
		case "1":
			m.applyRelativeTimeFilter(1 * time.Minute)
			m.mode = modeList
		case "2":
			m.applyRelativeTimeFilter(5 * time.Minute)
			m.mode = modeList
		case "3":
			m.applyRelativeTimeFilter(15 * time.Minute)
			m.mode = modeList
		case "4":
			m.applyRelativeTimeFilter(1 * time.Hour)
			m.mode = modeList
		case "5":
			m.mode = modeTimeCustom
			m.timeInput.SetValue("")
			m.timeInput.Focus()
			return m, textinput.Blink
		case "0":
			m.timeFrom = time.Time{}
			m.timeTo = time.Time{}
			m.refilter()
			m.mode = modeList
		case "esc", "t", "q":
			m.mode = modeList
		}
		return m, nil
	}

	// Detail mode.
	if m.mode == modeDetail {
		switch msg.String() {
		case "q", "esc", "enter", "backspace":
			m.mode = modeList
		}
		return m, nil
	}

	// List mode.
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		m.following = false
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
		}

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.ensureVisible()
		}

	case "pgup", "ctrl+u":
		m.following = false
		m.cursor -= m.listHeight() / 2
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureVisible()

	case "pgdown", "ctrl+d":
		m.cursor += m.listHeight() / 2
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureVisible()

	case "home", "g":
		m.following = false
		m.cursor = 0
		m.offset = 0

	case "end", "G":
		m.following = true
		m.scrollToBottom()

	case "enter":
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			m.mode = modeDetail
		}

	case "f":
		m.following = !m.following
		if m.following {
			m.scrollToBottom()
		}

	case "/":
		m.mode = modeSearch
		m.searchInput.SetValue(m.search)
		m.searchInput.Focus()
		return m, textinput.Blink

	case "t":
		m.mode = modeTimeFilter
		return m, nil

	case "esc":
		if m.search != "" {
			m.search = ""
			m.refilter()
			m.cursor = 0
			m.offset = 0
		}

	case "1":
		m.toggleLevels(entry.LevelError, entry.LevelFatal)
	case "2":
		m.toggleLevels(entry.LevelWarn)
	case "3":
		m.toggleLevels(entry.LevelInfo)
	case "4":
		m.toggleLevels(entry.LevelDebug, entry.LevelTrace)
	case "0":
		for l := range m.levels {
			m.levels[l] = true
		}
		m.refilter()
	}

	return m, nil
}

// --- Time filter ---

func (m *Model) applyRelativeTimeFilter(d time.Duration) {
	latest := m.latestTimestamp()
	if latest.IsZero() {
		return
	}
	m.timeFrom = latest.Add(-d)
	m.timeTo = latest
	m.refilter()
	m.clampCursor()
}

func (m *Model) applyCustomTimeFilter(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	parts := strings.SplitN(input, "-", 2)
	refDate := m.latestTimestamp()
	if refDate.IsZero() {
		refDate = time.Now()
	}
	base := time.Date(refDate.Year(), refDate.Month(), refDate.Day(), 0, 0, 0, 0, refDate.Location())

	from, ok := parseTimeOfDay(parts[0], base)
	if !ok {
		return
	}
	m.timeFrom = from

	if len(parts) == 2 {
		to, ok := parseTimeOfDay(parts[1], base)
		if !ok {
			return
		}
		m.timeTo = to
	} else {
		m.timeTo = time.Time{} // open-ended
	}

	m.refilter()
	m.clampCursor()
}

func parseTimeOfDay(s string, base time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return base.Add(time.Duration(t.Hour())*time.Hour +
				time.Duration(t.Minute())*time.Minute +
				time.Duration(t.Second())*time.Second), true
		}
	}
	return time.Time{}, false
}

func (m *Model) latestTimestamp() time.Time {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if !m.entries[i].Timestamp.IsZero() {
			return m.entries[i].Timestamp
		}
	}
	return time.Time{}
}

// --- Level filter ---

func (m *Model) toggleLevels(levels ...entry.Level) {
	allShown := true
	for _, l := range levels {
		if !m.levels[l] {
			allShown = false
			break
		}
	}
	for _, l := range levels {
		m.levels[l] = !allShown
	}
	m.levels[entry.LevelUnknown] = true
	m.refilter()
	m.clampCursor()
}

// --- Filtering ---

// refilter rescans all entries. Used when filter criteria change.
func (m *Model) refilter() {
	m.filtered = m.filtered[:0]
	m.filterRange(0)
}

// filterRange appends entries from startIdx that match current filters.
// Used incrementally when new entries arrive.
func (m *Model) filterRange(startIdx int) {
	search := strings.ToLower(m.search)
	for i := startIdx; i < len(m.entries); i++ {
		if m.matchesFilter(m.entries[i], search) {
			m.filtered = append(m.filtered, i)
		}
	}
}

func (m *Model) matchesFilter(e entry.Entry, search string) bool {
	if !m.levels[e.Level] {
		return false
	}
	if search != "" && !strings.Contains(strings.ToLower(e.Raw), search) {
		return false
	}
	if !m.timeFrom.IsZero() && !e.Timestamp.IsZero() && e.Timestamp.Before(m.timeFrom) {
		return false
	}
	if !m.timeTo.IsZero() && !e.Timestamp.IsZero() && e.Timestamp.After(m.timeTo) {
		return false
	}
	return true
}

// --- Scrolling ---

func (m *Model) clampCursor() {
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) scrollToBottom() {
	if len(m.filtered) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	m.cursor = len(m.filtered) - 1
	m.ensureVisible()
}

func (m *Model) ensureVisible() {
	h := m.listHeight()
	if h <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
}

func (m *Model) listHeight() int {
	h := m.height - 2
	if len(m.columns) > 0 {
		h-- // column header row
	}
	if h < 1 {
		h = 1
	}
	return h
}
