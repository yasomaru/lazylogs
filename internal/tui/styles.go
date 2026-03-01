package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/syasoda/lazylogs/internal/entry"
)

var (
	styleError = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true)
	styleWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
	styleInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("#55AAFF"))
	styleDebug = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	styleTrace = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	styleFatal = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true).Reverse(true)

	styleSelected  = lipgloss.NewStyle().Background(lipgloss.Color("#333333"))
	styleStatusBar = lipgloss.NewStyle().Background(lipgloss.Color("#333355")).Foreground(lipgloss.Color("#CCCCCC"))
	styleTimestamp = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	styleFieldKey  = lipgloss.NewStyle().Foreground(lipgloss.Color("#88AACC"))
	styleFieldVal  = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	styleHelpKey   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	styleHelpDesc  = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	styleTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#CCCCFF"))
	styleColHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#AAAADD")).Underline(true)
	styleTimePrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC55")).Bold(true)
)

func levelStyle(l entry.Level) lipgloss.Style {
	switch l {
	case entry.LevelFatal:
		return styleFatal
	case entry.LevelError:
		return styleError
	case entry.LevelWarn:
		return styleWarn
	case entry.LevelInfo:
		return styleInfo
	case entry.LevelDebug:
		return styleDebug
	case entry.LevelTrace:
		return styleTrace
	default:
		return lipgloss.NewStyle()
	}
}
