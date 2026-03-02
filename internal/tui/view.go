package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/syasoda/lazylogs/internal/entry"
)

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.mode {
	case modeDetail:
		return m.viewDetail()
	case modeSearch:
		return m.viewSearch()
	case modeTimeFilter:
		return m.viewTimeFilter()
	case modeTimeCustom:
		return m.viewTimeCustom()
	default:
		return m.viewList()
	}
}

// --- List view ---

func (m *Model) viewList() string {
	var b strings.Builder

	if len(m.columns) > 0 {
		b.WriteString(m.renderColumnHeader())
		b.WriteByte('\n')
	}

	h := m.listHeight()
	for i := 0; i < h; i++ {
		idx := m.offset + i
		if idx >= len(m.filtered) {
			b.WriteByte('\n')
			continue
		}

		e := m.entries[m.filtered[idx]]
		var line string
		if len(m.columns) > 0 {
			line = m.formatColumns(e)
		} else {
			line = m.formatEntry(e)
		}

		if lipgloss.Width(line) > m.width {
			line = lipgloss.NewStyle().MaxWidth(m.width).Render(line)
		}

		if idx == m.cursor {
			line = styleSelected.Width(m.width).Render(line)
		}

		b.WriteString(line)
		b.WriteByte('\n')
	}

	b.WriteString(m.viewStatusBar())
	b.WriteByte('\n')
	b.WriteString(m.viewHelp())

	return b.String()
}

// --- Column rendering ---

func (m *Model) columnWidths() []int {
	n := len(m.columns)
	if n == 0 {
		return nil
	}

	widths := make([]int, n)
	remaining := m.width - (n - 1) // subtract separators

	// Fixed-width columns.
	msgIdx := -1
	for i, col := range m.columns {
		switch col {
		case "time", "timestamp", "ts":
			widths[i] = 12
		case "level", "lvl":
			widths[i] = 5
		case "msg", "message":
			msgIdx = i
		default:
			widths[i] = 15
		}
		remaining -= widths[i]
	}

	// Give remaining space to msg column (or distribute if no msg).
	if msgIdx >= 0 {
		w := remaining
		if w < 10 {
			w = 10
		}
		widths[msgIdx] = w
	} else if remaining > 0 {
		// Distribute extra to last column.
		widths[n-1] += remaining
	}

	return widths
}

func (m *Model) renderColumnHeader() string {
	widths := m.columnWidths()
	var parts []string
	for i, col := range m.columns {
		w := widths[i]
		header := strings.ToUpper(col)
		parts = append(parts, styleColHeader.Render(padRight(header, w)))
	}
	return strings.Join(parts, " ")
}

func (m *Model) formatColumns(e entry.Entry) string {
	widths := m.columnWidths()
	var parts []string

	for i, col := range m.columns {
		w := widths[i]
		var val string
		var style lipgloss.Style

		switch col {
		case "time", "timestamp", "ts":
			if !e.Timestamp.IsZero() {
				val = e.Timestamp.Format("15:04:05.000")
			}
			style = styleTimestamp
		case "level", "lvl":
			if e.Level != entry.LevelUnknown {
				val = e.Level.String()
			}
			style = levelStyle(e.Level)
		case "msg", "message":
			val = e.Message
			style = lipgloss.NewStyle()
		default:
			val = m.getFieldValue(e, col)
			style = styleFieldVal
		}

		cell := padRight(val, w)
		parts = append(parts, style.Render(cell))
	}

	return strings.Join(parts, " ")
}

func (m *Model) getFieldValue(e entry.Entry, key string) string {
	for _, f := range e.Fields {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}

// --- Default (non-column) formatting ---

func (m *Model) formatEntry(e entry.Entry) string {
	var parts []string

	if !e.Timestamp.IsZero() {
		parts = append(parts, styleTimestamp.Render(e.Timestamp.Format("15:04:05.000")))
	}

	if e.Level != entry.LevelUnknown {
		parts = append(parts, levelStyle(e.Level).Render(fmt.Sprintf("%-3s", e.Level.String())))
	}

	if e.Message != "" {
		parts = append(parts, e.Message)
	} else if e.Raw != "" {
		parts = append(parts, e.Raw)
	}

	for _, f := range e.Fields {
		parts = append(parts, styleFieldKey.Render(f.Key)+"="+styleFieldVal.Render(f.Value))
	}

	return strings.Join(parts, " ")
}

// --- Detail view ---

func (m *Model) viewDetail() string {
	if m.cursor >= len(m.filtered) {
		return "No entry selected"
	}
	e := m.entries[m.filtered[m.cursor]]

	var b strings.Builder

	b.WriteString(styleStatusBar.Width(m.width).Render(fmt.Sprintf(" Entry #%d ", e.Line)))
	b.WriteByte('\n')
	b.WriteByte('\n')

	if !e.Timestamp.IsZero() {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			styleFieldKey.Render("time "),
			e.Timestamp.Format("2006-01-02 15:04:05.000")))
	}
	if e.Level != entry.LevelUnknown {
		b.WriteString(fmt.Sprintf("  %s %s\n",
			styleFieldKey.Render("level"),
			levelStyle(e.Level).Render(e.Level.String())))
	}
	if e.Message != "" {
		b.WriteString(fmt.Sprintf("  %s   %s\n",
			styleFieldKey.Render("msg"),
			e.Message))
	}

	if len(e.Fields) > 0 {
		b.WriteByte('\n')
		b.WriteString("  " + styleTitle.Render("Fields") + "\n")
		for _, f := range e.Fields {
			b.WriteString(fmt.Sprintf("    %s = %s\n",
				styleFieldKey.Render(f.Key),
				styleFieldVal.Render(f.Value)))
		}
	}

	b.WriteByte('\n')
	b.WriteString("  " + styleTitle.Render("Raw") + "\n")
	raw := e.Raw
	maxW := m.width - 4
	if maxW < 20 {
		maxW = 20
	}
	for len(raw) > maxW {
		b.WriteString("  " + styleFieldVal.Render(raw[:maxW]) + "\n")
		raw = raw[maxW:]
	}
	if raw != "" {
		b.WriteString("  " + styleFieldVal.Render(raw) + "\n")
	}

	lines := strings.Count(b.String(), "\n")
	for i := lines; i < m.height-1; i++ {
		b.WriteByte('\n')
	}

	b.WriteString(styleHelpKey.Render("  esc/enter") + " " + styleHelpDesc.Render("back"))

	return b.String()
}

// --- Search view ---

func (m *Model) viewSearch() string {
	list := m.viewList()
	lines := strings.Split(list, "\n")
	if len(lines) > 0 {
		lines[len(lines)-1] = styleHelpKey.Render("/") + " " + m.searchInput.View()
	}
	return strings.Join(lines, "\n")
}

// --- Time filter views ---

func (m *Model) viewTimeFilter() string {
	list := m.viewList()
	lines := strings.Split(list, "\n")
	if len(lines) > 0 {
		prompt := styleTimePrompt.Render("Time:") + " " +
			styleHelpKey.Render("1") + styleHelpDesc.Render("=1m ") +
			styleHelpKey.Render("2") + styleHelpDesc.Render("=5m ") +
			styleHelpKey.Render("3") + styleHelpDesc.Render("=15m ") +
			styleHelpKey.Render("4") + styleHelpDesc.Render("=1h ") +
			styleHelpKey.Render("5") + styleHelpDesc.Render("=custom ") +
			styleHelpKey.Render("0") + styleHelpDesc.Render("=reset ") +
			styleHelpKey.Render("esc") + styleHelpDesc.Render("=cancel")
		lines[len(lines)-1] = prompt
	}
	return strings.Join(lines, "\n")
}

func (m *Model) viewTimeCustom() string {
	list := m.viewList()
	lines := strings.Split(list, "\n")
	if len(lines) > 0 {
		lines[len(lines)-1] = styleTimePrompt.Render("Time range: ") + m.timeInput.View()
	}
	return strings.Join(lines, "\n")
}

// --- Status bar ---

func (m *Model) viewStatusBar() string {
	shown := len(m.filtered)
	total := len(m.entries)
	left := fmt.Sprintf(" %d/%d", shown, total)

	if !m.done {
		left += " loading..."
	}

	// Show error message if present.
	if m.errorMsg != "" {
		left += " " + styleErrorMsg.Render(m.errorMsg)
	}

	var right []string
	if m.search != "" {
		right = append(right, fmt.Sprintf("search:%q", m.search))
	}
	if !m.timeFrom.IsZero() {
		tf := m.timeFrom.Format("15:04:05")
		if !m.timeTo.IsZero() {
			tf += "-" + m.timeTo.Format("15:04:05")
		} else {
			tf += "-"
		}
		right = append(right, "time:"+tf)
	}
	if m.following {
		right = append(right, "FOLLOW")
	}
	if m.done && len(m.entries) > 0 {
		right = append(right, "EOF")
	}

	var hidden []string
	if !m.levels[entry.LevelError] || !m.levels[entry.LevelFatal] {
		hidden = append(hidden, "-err")
	}
	if !m.levels[entry.LevelWarn] {
		hidden = append(hidden, "-warn")
	}
	if !m.levels[entry.LevelInfo] {
		hidden = append(hidden, "-info")
	}
	if !m.levels[entry.LevelDebug] || !m.levels[entry.LevelTrace] {
		hidden = append(hidden, "-dbg")
	}
	if len(hidden) > 0 {
		right = append(right, strings.Join(hidden, " "))
	}

	rightStr := ""
	if len(right) > 0 {
		rightStr = strings.Join(right, " | ") + " "
	}

	pad := m.width - lipgloss.Width(left) - lipgloss.Width(rightStr)
	if pad < 1 {
		pad = 1
	}

	return styleStatusBar.Width(m.width).Render(left + strings.Repeat(" ", pad) + rightStr)
}

// --- Help bar ---

func (m *Model) viewHelp() string {
	helps := []struct{ key, desc string }{
		{"j/k", "scroll"},
		{"enter", "detail"},
		{"/", "search"},
		{"t", "time"},
		{"1-4", "level"},
		{"0", "reset"},
		{"f", "follow"},
		{"q", "quit"},
	}
	var parts []string
	for _, h := range helps {
		parts = append(parts, styleHelpKey.Render(h.key)+" "+styleHelpDesc.Render(h.desc))
	}
	return " " + strings.Join(parts, "  ")
}

// --- Helpers ---

// padRight pads a string to width w using display width (handles CJK, emoji).
func padRight(s string, w int) string {
	sw := runewidth.StringWidth(s)
	if sw >= w {
		return runewidth.Truncate(s, w, "")
	}
	return s + strings.Repeat(" ", w-sw)
}
