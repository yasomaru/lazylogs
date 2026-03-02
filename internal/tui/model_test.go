package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/syasoda/lazylogs/internal/entry"
)

func makeModel(entries []entry.Entry) *Model {
	ch := make(chan entry.Entry)
	close(ch)
	m := NewModel(ch, nil)
	m.entries = entries
	m.width = 120
	m.height = 40
	m.refilter()
	return m
}

func makeModelWithColumns(entries []entry.Entry, cols []string) *Model {
	ch := make(chan entry.Entry)
	close(ch)
	m := NewModel(ch, cols)
	m.entries = entries
	m.width = 120
	m.height = 40
	m.refilter()
	return m
}

func sampleEntries() []entry.Entry {
	base := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC)
	return []entry.Entry{
		{Raw: `{"level":"info","msg":"server starting"}`, Level: entry.LevelInfo, Message: "server starting", Timestamp: base, Line: 1,
			Fields: []entry.Field{{Key: "version", Value: "1.0"}}},
		{Raw: `{"level":"debug","msg":"loading config"}`, Level: entry.LevelDebug, Message: "loading config", Timestamp: base.Add(1 * time.Second), Line: 2},
		{Raw: `{"level":"warn","msg":"slow query"}`, Level: entry.LevelWarn, Message: "slow query", Timestamp: base.Add(2 * time.Second), Line: 3},
		{Raw: `{"level":"error","msg":"connection failed"}`, Level: entry.LevelError, Message: "connection failed", Timestamp: base.Add(3 * time.Second), Line: 4},
		{Raw: `{"level":"fatal","msg":"out of memory"}`, Level: entry.LevelFatal, Message: "out of memory", Timestamp: base.Add(4 * time.Second), Line: 5},
	}
}

func keyMsg(s string) tea.KeyMsg {
	// Map common key names to tea.KeyType.
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// --- NewModel ---

func TestNewModel(t *testing.T) {
	ch := make(chan entry.Entry)
	close(ch)
	m := NewModel(ch, []string{"time", "level", "msg"})

	if m.columns == nil || len(m.columns) != 3 {
		t.Fatalf("expected 3 columns, got %v", m.columns)
	}
	if !m.following {
		t.Fatal("expected following to be true initially")
	}
	for l, v := range m.levels {
		if !v {
			t.Fatalf("expected level %d to be enabled", l)
		}
	}
}

// --- Update ---

func TestUpdateWindowSize(t *testing.T) {
	m := makeModel(nil)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	if m.width != 200 || m.height != 50 {
		t.Fatalf("expected 200x50, got %dx%d", m.width, m.height)
	}
}

func TestUpdateBatchMsg(t *testing.T) {
	m := makeModel(nil)
	m.width = 120
	m.height = 40

	batch := batchMsg{
		{Raw: "line1", Level: entry.LevelInfo, Line: 1},
		{Raw: "line2", Level: entry.LevelError, Line: 2},
	}
	m.Update(batch)

	if len(m.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m.entries))
	}
	if len(m.filtered) != 2 {
		t.Fatalf("expected 2 filtered, got %d", len(m.filtered))
	}
}

func TestUpdateBatchMsgMemoryCap(t *testing.T) {
	m := makeModel(nil)
	m.width = 120
	m.height = 40

	// Fill to max.
	big := make(batchMsg, maxEntries+100)
	for i := range big {
		big[i] = entry.Entry{Raw: "line", Level: entry.LevelInfo, Line: i}
	}
	m.Update(big)

	if len(m.entries) != maxEntries {
		t.Fatalf("expected %d entries after cap, got %d", maxEntries, len(m.entries))
	}
}

func TestUpdateEntriesDone(t *testing.T) {
	m := makeModel(nil)
	m.Update(entriesDoneMsg{})
	if !m.done {
		t.Fatal("expected done=true after entriesDoneMsg")
	}
}

func TestUpdateClearError(t *testing.T) {
	m := makeModel(nil)
	m.errorMsg = "test error"
	m.Update(clearErrorMsg{})
	if m.errorMsg != "" {
		t.Fatalf("expected empty errorMsg, got %q", m.errorMsg)
	}
}

// --- handleKey: List mode ---

func TestHandleKeyScrollDown(t *testing.T) {
	m := makeModel(sampleEntries())
	m.cursor = 0
	m.following = false

	m.handleKey(keyMsg("j"))
	if m.cursor != 1 {
		t.Fatalf("expected cursor=1 after j, got %d", m.cursor)
	}

	m.handleKey(keyMsg("down"))
	if m.cursor != 2 {
		t.Fatalf("expected cursor=2 after down, got %d", m.cursor)
	}
}

func TestHandleKeyScrollUp(t *testing.T) {
	m := makeModel(sampleEntries())
	m.cursor = 3
	m.following = false

	m.handleKey(keyMsg("k"))
	if m.cursor != 2 {
		t.Fatalf("expected cursor=2 after k, got %d", m.cursor)
	}
	if m.following {
		t.Fatal("scrolling up should disable following")
	}
}

func TestHandleKeyPageScroll(t *testing.T) {
	m := makeModel(sampleEntries())
	m.height = 10
	m.cursor = 4

	m.handleKey(keyMsg("pgup"))
	if m.cursor >= 4 {
		t.Fatalf("expected cursor < 4 after pgup, got %d", m.cursor)
	}
}

func TestHandleKeyHomeEnd(t *testing.T) {
	m := makeModel(sampleEntries())
	m.cursor = 2

	m.handleKey(keyMsg("g"))
	if m.cursor != 0 {
		t.Fatalf("expected cursor=0 after g, got %d", m.cursor)
	}
	if m.following {
		t.Fatal("g should disable following")
	}

	m.handleKey(keyMsg("G"))
	if m.cursor != 4 {
		t.Fatalf("expected cursor=4 after G, got %d", m.cursor)
	}
	if !m.following {
		t.Fatal("G should enable following")
	}
}

func TestHandleKeyEnterDetail(t *testing.T) {
	m := makeModel(sampleEntries())
	m.cursor = 2

	m.handleKey(keyMsg("enter"))
	if m.mode != modeDetail {
		t.Fatalf("expected modeDetail, got %d", m.mode)
	}
}

func TestHandleKeyFollow(t *testing.T) {
	m := makeModel(sampleEntries())
	m.following = false

	m.handleKey(keyMsg("f"))
	if !m.following {
		t.Fatal("expected following=true after f")
	}

	m.handleKey(keyMsg("f"))
	if m.following {
		t.Fatal("expected following=false after second f")
	}
}

func TestHandleKeySearchMode(t *testing.T) {
	m := makeModel(sampleEntries())

	m.handleKey(keyMsg("/"))
	if m.mode != modeSearch {
		t.Fatalf("expected modeSearch, got %d", m.mode)
	}
}

func TestHandleKeyTimeFilterMode(t *testing.T) {
	m := makeModel(sampleEntries())

	m.handleKey(keyMsg("t"))
	if m.mode != modeTimeFilter {
		t.Fatalf("expected modeTimeFilter, got %d", m.mode)
	}
}

func TestHandleKeyLevelToggle(t *testing.T) {
	m := makeModel(sampleEntries())

	// Toggle error/fatal with "1".
	m.handleKey(keyMsg("1"))
	for _, idx := range m.filtered {
		l := m.entries[idx].Level
		if l == entry.LevelError || l == entry.LevelFatal {
			t.Fatal("error/fatal should be hidden after pressing 1")
		}
	}

	// Reset with "0".
	m.handleKey(keyMsg("0"))
	if len(m.filtered) != 5 {
		t.Fatalf("expected all 5 after 0, got %d", len(m.filtered))
	}
}

func TestHandleKeyEscClearsSearch(t *testing.T) {
	m := makeModel(sampleEntries())
	m.setSearch("connection")
	m.refilter()

	m.handleKey(keyMsg("esc"))
	if m.search != "" {
		t.Fatalf("expected empty search after esc, got %q", m.search)
	}
	if len(m.filtered) != 5 {
		t.Fatalf("expected all entries after clearing search, got %d", len(m.filtered))
	}
}

func TestHandleKeyQuit(t *testing.T) {
	m := makeModel(sampleEntries())
	_, cmd := m.handleKey(keyMsg("q"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// --- handleKey: Detail mode ---

func TestHandleKeyDetailBack(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeDetail

	m.handleKey(keyMsg("esc"))
	if m.mode != modeList {
		t.Fatalf("expected modeList after esc in detail, got %d", m.mode)
	}

	m.mode = modeDetail
	m.handleKey(keyMsg("enter"))
	if m.mode != modeList {
		t.Fatalf("expected modeList after enter in detail, got %d", m.mode)
	}

	m.mode = modeDetail
	m.handleKey(keyMsg("q"))
	if m.mode != modeList {
		t.Fatalf("expected modeList after q in detail, got %d", m.mode)
	}
}

// --- handleKey: Search mode ---

func TestHandleKeySearchSubmit(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeSearch
	m.searchInput.SetValue("memory")

	m.handleKey(keyMsg("enter"))
	if m.mode != modeList {
		t.Fatalf("expected modeList after enter, got %d", m.mode)
	}
	if m.search != "memory" {
		t.Fatalf("expected search='memory', got %q", m.search)
	}
	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 match, got %d", len(m.filtered))
	}
}

func TestHandleKeySearchCancel(t *testing.T) {
	m := makeModel(sampleEntries())
	m.search = "old"
	m.mode = modeSearch

	m.handleKey(keyMsg("esc"))
	if m.mode != modeList {
		t.Fatalf("expected modeList after esc, got %d", m.mode)
	}
	if m.search != "old" {
		t.Fatal("esc should not clear existing search")
	}
}

// --- handleKey: Time filter mode ---

func TestHandleKeyTimeFilterPresets(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeTimeFilter

	m.handleKey(keyMsg("1"))
	if m.mode != modeList {
		t.Fatalf("expected modeList after preset, got %d", m.mode)
	}
	if m.timeFrom.IsZero() {
		t.Fatal("expected non-zero timeFrom after preset 1")
	}
}

func TestHandleKeyTimeFilterCustom(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeTimeFilter

	m.handleKey(keyMsg("5"))
	if m.mode != modeTimeCustom {
		t.Fatalf("expected modeTimeCustom, got %d", m.mode)
	}
}

func TestHandleKeyTimeFilterReset(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeTimeFilter
	m.timeFrom = time.Now()
	m.timeTo = time.Now()

	m.handleKey(keyMsg("0"))
	if !m.timeFrom.IsZero() || !m.timeTo.IsZero() {
		t.Fatal("expected time filter reset")
	}
}

func TestHandleKeyTimeFilterCancel(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeTimeFilter

	m.handleKey(keyMsg("esc"))
	if m.mode != modeList {
		t.Fatalf("expected modeList after esc, got %d", m.mode)
	}
}

// --- handleKey: Time custom mode ---

func TestHandleKeyTimeCustomSubmit(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeTimeCustom
	m.timeInput.SetValue("10:00:01-10:00:03")

	m.handleKey(keyMsg("enter"))
	if m.mode != modeList {
		t.Fatalf("expected modeList, got %d", m.mode)
	}
	if m.timeFrom.IsZero() {
		t.Fatal("expected timeFrom to be set")
	}
}

func TestHandleKeyTimeCustomInvalid(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeTimeCustom
	m.timeInput.SetValue("not-a-time")

	m.handleKey(keyMsg("enter"))
	if m.errorMsg == "" {
		t.Fatal("expected error message for invalid time")
	}
}

// --- Level filter ---

func TestLevelFilter(t *testing.T) {
	m := makeModel(sampleEntries())

	if len(m.filtered) != 5 {
		t.Fatalf("expected 5 filtered, got %d", len(m.filtered))
	}

	m.toggleLevels(entry.LevelError, entry.LevelFatal)
	if len(m.filtered) != 3 {
		t.Fatalf("after hiding error/fatal: expected 3, got %d", len(m.filtered))
	}

	m.toggleLevels(entry.LevelError, entry.LevelFatal)
	if len(m.filtered) != 5 {
		t.Fatalf("after restoring: expected 5, got %d", len(m.filtered))
	}

	m.toggleLevels(entry.LevelInfo)
	for _, idx := range m.filtered {
		if m.entries[idx].Level == entry.LevelInfo {
			t.Fatal("info entries should be filtered out")
		}
	}
}

// --- Search ---

func TestSearchLiteral(t *testing.T) {
	m := makeModel(sampleEntries())

	m.setSearch("connection")
	m.refilter()
	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 match for 'connection', got %d", len(m.filtered))
	}
}

func TestSearchRegex(t *testing.T) {
	m := makeModel(sampleEntries())

	m.setSearch("server|memory")
	m.refilter()
	if len(m.filtered) != 2 {
		t.Fatalf("expected 2 matches for 'server|memory', got %d", len(m.filtered))
	}

	m.setSearch("SERVER")
	m.refilter()
	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 match for 'SERVER' (case-insensitive), got %d", len(m.filtered))
	}
}

func TestSearchInvalidRegexFallsBack(t *testing.T) {
	m := makeModel(sampleEntries())

	m.setSearch("[invalid")
	if m.searchRegex != nil {
		t.Fatal("expected nil regex for invalid pattern")
	}
	m.refilter()
	if len(m.filtered) != 0 {
		t.Fatalf("expected 0 matches for invalid regex literal, got %d", len(m.filtered))
	}
}

func TestSearchClear(t *testing.T) {
	m := makeModel(sampleEntries())
	m.setSearch("test")
	m.setSearch("")
	if m.searchRegex != nil {
		t.Fatal("expected nil regex after clearing search")
	}
}

// --- Time filter ---

func TestTimeFilter(t *testing.T) {
	m := makeModel(sampleEntries())

	m.applyRelativeTimeFilter(2 * time.Second)
	if len(m.filtered) != 3 {
		t.Fatalf("expected 3 entries in last 2s, got %d", len(m.filtered))
	}

	m.timeFrom = time.Time{}
	m.timeTo = time.Time{}
	m.refilter()
	if len(m.filtered) != 5 {
		t.Fatalf("after reset: expected 5, got %d", len(m.filtered))
	}
}

func TestCustomTimeFilterInvalid(t *testing.T) {
	m := makeModel(sampleEntries())

	cmd := m.applyCustomTimeFilter("garbage")
	if m.errorMsg == "" {
		t.Fatal("expected error message for invalid time")
	}
	if cmd == nil {
		t.Fatal("expected clear error command")
	}
}

func TestCustomTimeFilterValid(t *testing.T) {
	m := makeModel(sampleEntries())

	cmd := m.applyCustomTimeFilter("10:00:02-10:00:04")
	if m.errorMsg != "" {
		t.Fatalf("unexpected error: %s", m.errorMsg)
	}
	if cmd != nil {
		t.Fatal("expected nil command for valid input")
	}
	if len(m.filtered) != 3 {
		t.Fatalf("expected 3 entries in time range, got %d", len(m.filtered))
	}
}

func TestCustomTimeFilterEmpty(t *testing.T) {
	m := makeModel(sampleEntries())
	cmd := m.applyCustomTimeFilter("")
	if cmd != nil {
		t.Fatal("expected nil command for empty input")
	}
}

func TestCustomTimeFilterInvalidEnd(t *testing.T) {
	m := makeModel(sampleEntries())
	cmd := m.applyCustomTimeFilter("10:00-garbage")
	if m.errorMsg == "" {
		t.Fatal("expected error for invalid end time")
	}
	if cmd == nil {
		t.Fatal("expected clear error command")
	}
}

func TestCustomTimeFilterOpenEnded(t *testing.T) {
	m := makeModel(sampleEntries())
	cmd := m.applyCustomTimeFilter("10:00:02")
	if m.errorMsg != "" {
		t.Fatalf("unexpected error: %s", m.errorMsg)
	}
	if cmd != nil {
		t.Fatal("expected nil command")
	}
	if m.timeFrom.IsZero() {
		t.Fatal("expected timeFrom to be set")
	}
	if !m.timeTo.IsZero() {
		t.Fatal("expected timeTo to be zero for open-ended")
	}
}

func TestRelativeTimeFilterNoTimestamps(t *testing.T) {
	entries := []entry.Entry{
		{Raw: "no time", Level: entry.LevelInfo, Line: 1},
	}
	m := makeModel(entries)
	m.applyRelativeTimeFilter(1 * time.Minute)
	// Should not crash, timeFrom should still be zero.
	if !m.timeFrom.IsZero() {
		t.Fatal("expected zero timeFrom when no timestamps")
	}
}

// --- Memory cap ---

func TestMemoryCap(t *testing.T) {
	m := makeModel(nil)

	entries := make([]entry.Entry, maxEntries+500)
	for i := range entries {
		entries[i] = entry.Entry{Raw: "test line", Level: entry.LevelInfo, Line: i + 1}
	}
	m.entries = entries

	if len(m.entries) > maxEntries {
		m.entries = m.entries[len(m.entries)-maxEntries:]
		m.refilter()
		m.clampCursor()
	}

	if len(m.entries) != maxEntries {
		t.Fatalf("expected %d entries after cap, got %d", maxEntries, len(m.entries))
	}
	if len(m.filtered) != maxEntries {
		t.Fatalf("expected %d filtered after cap, got %d", maxEntries, len(m.filtered))
	}
}

// --- Scrolling ---

func TestScrolling(t *testing.T) {
	m := makeModel(sampleEntries())
	m.height = 10

	m.scrollToBottom()
	if m.cursor != 4 {
		t.Fatalf("expected cursor at 4, got %d", m.cursor)
	}

	m.cursor--
	m.ensureVisible()
	if m.cursor != 3 {
		t.Fatalf("expected cursor at 3, got %d", m.cursor)
	}

	m.cursor = 0
	m.offset = 0
	if m.cursor != 0 || m.offset != 0 {
		t.Fatalf("expected cursor=0 offset=0")
	}
}

func TestScrollToBottomEmpty(t *testing.T) {
	m := makeModel(nil)
	m.scrollToBottom()
	if m.cursor != 0 || m.offset != 0 {
		t.Fatalf("expected 0,0 for empty model")
	}
}

func TestEnsureVisibleScrollsDown(t *testing.T) {
	m := makeModel(sampleEntries())
	m.height = 5 // listHeight = 3
	m.offset = 0
	m.cursor = 4

	m.ensureVisible()
	if m.offset <= 0 {
		t.Fatalf("expected offset > 0 to show cursor, got %d", m.offset)
	}
}

func TestClampCursor(t *testing.T) {
	m := makeModel(sampleEntries())

	m.cursor = 100
	m.clampCursor()
	if m.cursor != 4 {
		t.Fatalf("expected cursor clamped to 4, got %d", m.cursor)
	}

	m.cursor = -5
	m.clampCursor()
	if m.cursor != 0 {
		t.Fatalf("expected cursor clamped to 0, got %d", m.cursor)
	}
}

func TestResetLevels(t *testing.T) {
	m := makeModel(sampleEntries())

	m.toggleLevels(entry.LevelInfo)
	m.toggleLevels(entry.LevelDebug, entry.LevelTrace)
	m.toggleLevels(entry.LevelWarn)
	m.toggleLevels(entry.LevelError, entry.LevelFatal)
	if len(m.filtered) != 0 {
		t.Fatalf("expected 0 after hiding all, got %d", len(m.filtered))
	}

	for l := range m.levels {
		m.levels[l] = true
	}
	m.refilter()
	if len(m.filtered) != 5 {
		t.Fatalf("expected 5 after reset, got %d", len(m.filtered))
	}
}

func TestListHeight(t *testing.T) {
	m := makeModel(nil)
	m.height = 40

	if h := m.listHeight(); h != 38 {
		t.Fatalf("expected listHeight=38, got %d", h)
	}

	m.columns = []string{"time", "level", "msg"}
	if h := m.listHeight(); h != 37 {
		t.Fatalf("expected listHeight=37 with columns, got %d", h)
	}

	m.height = 2
	m.columns = nil
	if h := m.listHeight(); h != 1 {
		t.Fatal("listHeight should be at least 1")
	}
}

// --- View rendering ---

func TestViewLoading(t *testing.T) {
	m := makeModel(nil)
	m.width = 0
	m.height = 0
	v := m.View()
	if v != "Loading..." {
		t.Fatalf("expected 'Loading...' got %q", v)
	}
}

func TestViewList(t *testing.T) {
	m := makeModel(sampleEntries())
	m.done = true
	v := m.View()

	if !strings.Contains(v, "server starting") {
		t.Fatal("expected list view to contain 'server starting'")
	}
	if !strings.Contains(v, "5/5") {
		t.Fatal("expected status bar to show '5/5'")
	}
	if !strings.Contains(v, "EOF") {
		t.Fatal("expected EOF in status bar")
	}
}

func TestViewListWithSearch(t *testing.T) {
	m := makeModel(sampleEntries())
	m.setSearch("error")
	m.refilter()
	v := m.View()

	if !strings.Contains(v, "connection failed") {
		t.Fatal("expected filtered view to show error entry")
	}
	if !strings.Contains(v, `search:"error"`) {
		t.Fatal("expected search indicator in status bar")
	}
}

func TestViewDetail(t *testing.T) {
	m := makeModel(sampleEntries())
	m.cursor = 0
	m.mode = modeDetail
	v := m.View()

	if !strings.Contains(v, "Entry #1") {
		t.Fatal("expected 'Entry #1' in detail view")
	}
	if !strings.Contains(v, "server starting") {
		t.Fatal("expected message in detail view")
	}
	if !strings.Contains(v, "version") {
		t.Fatal("expected field 'version' in detail view")
	}
	if !strings.Contains(v, "1.0") {
		t.Fatal("expected field value '1.0' in detail view")
	}
}

func TestViewDetailNoEntry(t *testing.T) {
	m := makeModel(nil)
	m.mode = modeDetail
	m.cursor = 5
	v := m.View()
	if !strings.Contains(v, "No entry selected") {
		t.Fatal("expected 'No entry selected'")
	}
}

func TestViewSearch(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeSearch
	v := m.View()

	if !strings.Contains(v, "/") {
		t.Fatal("expected '/' in search view")
	}
}

func TestViewTimeFilter(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeTimeFilter
	v := m.View()

	if !strings.Contains(v, "Time:") {
		t.Fatal("expected 'Time:' in time filter view")
	}
	if !strings.Contains(v, "=1m") {
		t.Fatal("expected preset labels")
	}
}

func TestViewTimeCustom(t *testing.T) {
	m := makeModel(sampleEntries())
	m.mode = modeTimeCustom
	v := m.View()

	if !strings.Contains(v, "Time range:") {
		t.Fatal("expected 'Time range:' in custom time view")
	}
}

func TestViewColumns(t *testing.T) {
	m := makeModelWithColumns(sampleEntries(), []string{"time", "level", "msg"})
	v := m.View()

	if !strings.Contains(v, "TIME") {
		t.Fatal("expected column header 'TIME'")
	}
	if !strings.Contains(v, "LEVEL") {
		t.Fatal("expected column header 'LEVEL'")
	}
	if !strings.Contains(v, "MSG") {
		t.Fatal("expected column header 'MSG'")
	}
}

func TestViewStatusBarFollow(t *testing.T) {
	m := makeModel(sampleEntries())
	m.following = true
	v := m.viewStatusBar()
	if !strings.Contains(v, "FOLLOW") {
		t.Fatal("expected FOLLOW in status bar")
	}
}

func TestViewStatusBarLoading(t *testing.T) {
	m := makeModel(sampleEntries())
	m.done = false
	v := m.viewStatusBar()
	if !strings.Contains(v, "loading") {
		t.Fatal("expected 'loading' in status bar")
	}
}

func TestViewStatusBarError(t *testing.T) {
	m := makeModel(sampleEntries())
	m.errorMsg = "test error"
	v := m.viewStatusBar()
	if !strings.Contains(v, "test error") {
		t.Fatal("expected error in status bar")
	}
}

func TestViewStatusBarTimeFilter(t *testing.T) {
	m := makeModel(sampleEntries())
	m.timeFrom = time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC)
	m.timeTo = time.Date(2025, 3, 1, 11, 0, 0, 0, time.UTC)
	v := m.viewStatusBar()
	if !strings.Contains(v, "time:") {
		t.Fatal("expected time indicator in status bar")
	}
}

func TestViewStatusBarHiddenLevels(t *testing.T) {
	m := makeModel(sampleEntries())
	m.levels[entry.LevelError] = false
	m.levels[entry.LevelFatal] = false
	v := m.viewStatusBar()
	if !strings.Contains(v, "-err") {
		t.Fatal("expected '-err' in status bar")
	}
}

func TestViewHelp(t *testing.T) {
	m := makeModel(nil)
	v := m.viewHelp()
	for _, key := range []string{"j/k", "enter", "/", "t", "q"} {
		if !strings.Contains(v, key) {
			t.Fatalf("expected %q in help bar", key)
		}
	}
}

// --- View helpers ---

func TestPadRight(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  int // expected display width
	}{
		{"hello", 10, 10},
		{"hi", 5, 5},
		{"longstring", 4, 4}, // truncated
		{"", 5, 5},
	}
	for _, tt := range tests {
		result := padRight(tt.input, tt.width)
		got := len(result) // for ASCII, len == display width
		if got != tt.want {
			t.Errorf("padRight(%q, %d) len=%d, want %d", tt.input, tt.width, got, tt.want)
		}
	}
}

func TestFormatEntry(t *testing.T) {
	m := makeModel(nil)
	e := entry.Entry{
		Timestamp: time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC),
		Level:     entry.LevelError,
		Message:   "test message",
		Fields:    []entry.Field{{Key: "status", Value: "500"}},
	}
	result := m.formatEntry(e)

	if !strings.Contains(result, "10:00:00.000") {
		t.Fatal("expected timestamp in formatted entry")
	}
	if !strings.Contains(result, "ERR") {
		t.Fatal("expected level in formatted entry")
	}
	if !strings.Contains(result, "test message") {
		t.Fatal("expected message in formatted entry")
	}
	if !strings.Contains(result, "status") {
		t.Fatal("expected field key in formatted entry")
	}
}

func TestFormatEntryNoTimestamp(t *testing.T) {
	m := makeModel(nil)
	e := entry.Entry{
		Level:   entry.LevelUnknown,
		Raw:     "raw text",
		Message: "",
	}
	result := m.formatEntry(e)
	if !strings.Contains(result, "raw text") {
		t.Fatal("expected raw text when no message")
	}
}

func TestGetFieldValue(t *testing.T) {
	m := makeModel(nil)
	e := entry.Entry{
		Fields: []entry.Field{
			{Key: "status", Value: "200"},
			{Key: "latency", Value: "5ms"},
		},
	}

	if v := m.getFieldValue(e, "status"); v != "200" {
		t.Fatalf("expected '200', got %q", v)
	}
	if v := m.getFieldValue(e, "missing"); v != "" {
		t.Fatalf("expected empty for missing field, got %q", v)
	}
}

func TestColumnWidths(t *testing.T) {
	m := makeModelWithColumns(nil, []string{"time", "level", "msg", "status"})
	widths := m.columnWidths()

	if widths[0] != 12 {
		t.Fatalf("time width: expected 12, got %d", widths[0])
	}
	if widths[1] != 5 {
		t.Fatalf("level width: expected 5, got %d", widths[1])
	}
	if widths[2] < 10 {
		t.Fatalf("msg width: expected >= 10, got %d", widths[2])
	}
	if widths[3] != 15 {
		t.Fatalf("status width: expected 15, got %d", widths[3])
	}
}

func TestColumnWidthsNoMsg(t *testing.T) {
	m := makeModelWithColumns(nil, []string{"time", "level", "status"})
	widths := m.columnWidths()

	total := 0
	for _, w := range widths {
		total += w
	}
	// Should use all available width (minus separators).
	if total+2 > m.width+1 { // 2 separators
		t.Fatalf("column widths exceed available width: %d > %d", total+2, m.width)
	}
}

// --- LevelStyle ---

func TestLevelStyle(t *testing.T) {
	levels := []entry.Level{
		entry.LevelFatal,
		entry.LevelError,
		entry.LevelWarn,
		entry.LevelInfo,
		entry.LevelDebug,
		entry.LevelTrace,
		entry.LevelUnknown,
	}
	for _, l := range levels {
		s := levelStyle(l)
		// Just verify it doesn't panic and returns a style.
		_ = s.Render("test")
	}
}
