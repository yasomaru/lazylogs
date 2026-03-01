package entry

import "time"

// Level represents log severity.
type Level int

const (
	LevelUnknown Level = iota
	LevelTrace
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "TRC"
	case LevelDebug:
		return "DBG"
	case LevelInfo:
		return "INF"
	case LevelWarn:
		return "WRN"
	case LevelError:
		return "ERR"
	case LevelFatal:
		return "FTL"
	default:
		return "???"
	}
}

// Entry represents a parsed log line.
type Entry struct {
	Raw       string
	Level     Level
	Timestamp time.Time
	Message   string
	Fields    []Field
	Line      int
}

// Field is a key-value pair from a structured log entry.
type Field struct {
	Key   string
	Value string
}
