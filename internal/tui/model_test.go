package tui

import (
	"testing"
	"time"

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

func sampleEntries() []entry.Entry {
	base := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC)
	return []entry.Entry{
		{Raw: `{"level":"info","msg":"server starting"}`, Level: entry.LevelInfo, Message: "server starting", Timestamp: base, Line: 1},
		{Raw: `{"level":"debug","msg":"loading config"}`, Level: entry.LevelDebug, Message: "loading config", Timestamp: base.Add(1 * time.Second), Line: 2},
		{Raw: `{"level":"warn","msg":"slow query"}`, Level: entry.LevelWarn, Message: "slow query", Timestamp: base.Add(2 * time.Second), Line: 3},
		{Raw: `{"level":"error","msg":"connection failed"}`, Level: entry.LevelError, Message: "connection failed", Timestamp: base.Add(3 * time.Second), Line: 4},
		{Raw: `{"level":"fatal","msg":"out of memory"}`, Level: entry.LevelFatal, Message: "out of memory", Timestamp: base.Add(4 * time.Second), Line: 5},
	}
}

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

func TestLevelFilter(t *testing.T) {
	m := makeModel(sampleEntries())

	if len(m.filtered) != 5 {
		t.Fatalf("expected 5 filtered, got %d", len(m.filtered))
	}

	// Toggle error/fatal off.
	m.toggleLevels(entry.LevelError, entry.LevelFatal)
	if len(m.filtered) != 3 {
		t.Fatalf("after hiding error/fatal: expected 3, got %d", len(m.filtered))
	}

	// Toggle error/fatal back on.
	m.toggleLevels(entry.LevelError, entry.LevelFatal)
	if len(m.filtered) != 5 {
		t.Fatalf("after restoring: expected 5, got %d", len(m.filtered))
	}

	// Toggle info off.
	m.toggleLevels(entry.LevelInfo)
	for _, idx := range m.filtered {
		if m.entries[idx].Level == entry.LevelInfo {
			t.Fatal("info entries should be filtered out")
		}
	}
}

func TestSearchLiteral(t *testing.T) {
	m := makeModel(sampleEntries())

	m.setSearch("connection")
	m.refilter()
	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 match for 'connection', got %d", len(m.filtered))
	}
	if m.entries[m.filtered[0]].Message != "connection failed" {
		t.Fatalf("expected 'connection failed', got %q", m.entries[m.filtered[0]].Message)
	}
}

func TestSearchRegex(t *testing.T) {
	m := makeModel(sampleEntries())

	m.setSearch("server|memory")
	m.refilter()
	if len(m.filtered) != 2 {
		t.Fatalf("expected 2 matches for 'server|memory', got %d", len(m.filtered))
	}

	// Regex is case-insensitive.
	m.setSearch("SERVER")
	m.refilter()
	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 match for 'SERVER' (case-insensitive), got %d", len(m.filtered))
	}
}

func TestSearchInvalidRegexFallsBack(t *testing.T) {
	m := makeModel(sampleEntries())

	// Invalid regex pattern - should fall back to literal search.
	m.setSearch("[invalid")
	if m.searchRegex != nil {
		t.Fatal("expected nil regex for invalid pattern")
	}
	m.refilter()
	// "[invalid" won't match anything literally.
	if len(m.filtered) != 0 {
		t.Fatalf("expected 0 matches for invalid regex literal, got %d", len(m.filtered))
	}
}

func TestTimeFilter(t *testing.T) {
	m := makeModel(sampleEntries())

	// Filter to last 2 seconds from latest timestamp.
	m.applyRelativeTimeFilter(2 * time.Second)
	// Latest is base+4s. So from base+2s to base+4s → lines 3,4,5.
	if len(m.filtered) != 3 {
		t.Fatalf("expected 3 entries in last 2s, got %d", len(m.filtered))
	}

	// Reset.
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

func TestMemoryCap(t *testing.T) {
	m := makeModel(nil)

	// Add more than maxEntries.
	entries := make([]entry.Entry, maxEntries+500)
	for i := range entries {
		entries[i] = entry.Entry{
			Raw:   "test line",
			Level: entry.LevelInfo,
			Line:  i + 1,
		}
	}
	m.entries = entries

	// Simulate the cap check from Update.
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

func TestScrolling(t *testing.T) {
	m := makeModel(sampleEntries())
	m.height = 10

	// Start at bottom (following mode).
	m.scrollToBottom()
	if m.cursor != 4 {
		t.Fatalf("expected cursor at 4, got %d", m.cursor)
	}

	// Move up.
	m.cursor--
	m.ensureVisible()
	if m.cursor != 3 {
		t.Fatalf("expected cursor at 3, got %d", m.cursor)
	}

	// Jump to top.
	m.cursor = 0
	m.offset = 0
	if m.cursor != 0 || m.offset != 0 {
		t.Fatalf("expected cursor=0 offset=0")
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

	// Hide everything except unknown.
	m.toggleLevels(entry.LevelInfo)
	m.toggleLevels(entry.LevelDebug, entry.LevelTrace)
	m.toggleLevels(entry.LevelWarn)
	m.toggleLevels(entry.LevelError, entry.LevelFatal)
	if len(m.filtered) != 0 {
		t.Fatalf("expected 0 after hiding all, got %d", len(m.filtered))
	}

	// Reset all levels.
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

	// No columns.
	if h := m.listHeight(); h != 38 {
		t.Fatalf("expected listHeight=38, got %d", h)
	}

	// With columns.
	m.columns = []string{"time", "level", "msg"}
	if h := m.listHeight(); h != 37 {
		t.Fatalf("expected listHeight=37 with columns, got %d", h)
	}
}
