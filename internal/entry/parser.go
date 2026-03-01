package entry

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ParseLine parses a raw log line into an Entry.
func ParseLine(raw string, lineNum int) Entry {
	raw = strings.TrimRight(raw, "\r\n")
	if raw == "" {
		return Entry{Raw: raw, Line: lineNum, Level: LevelUnknown}
	}

	trimmed := strings.TrimSpace(raw)

	// Try JSON first.
	if len(trimmed) > 0 && trimmed[0] == '{' {
		if e, ok := parseJSON(trimmed, lineNum); ok {
			return e
		}
	}

	// Try logfmt.
	if e, ok := parseLogfmt(raw, lineNum); ok {
		return e
	}

	// Fallback: plain text with level detection.
	return parsePlain(raw, lineNum)
}

func parseJSON(raw string, lineNum int) (Entry, bool) {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return Entry{}, false
	}

	e := Entry{Raw: raw, Line: lineNum}

	// Extract level.
	for _, key := range []string{"level", "lvl", "severity", "log_level", "loglevel"} {
		if v, ok := m[key]; ok {
			e.Level = parseLevel(fmt.Sprintf("%v", v))
			delete(m, key)
			break
		}
	}

	// Extract message.
	for _, key := range []string{"msg", "message", "text", "body"} {
		if v, ok := m[key]; ok {
			e.Message = fmt.Sprintf("%v", v)
			delete(m, key)
			break
		}
	}

	// Extract timestamp.
	for _, key := range []string{"time", "timestamp", "ts", "@timestamp", "t", "datetime"} {
		if v, ok := m[key]; ok {
			e.Timestamp = parseTimestamp(v)
			delete(m, key)
			break
		}
	}

	// Remaining fields.
	for k, v := range m {
		var val string
		switch tv := v.(type) {
		case string:
			val = tv
		default:
			b, _ := json.Marshal(v)
			val = string(b)
		}
		e.Fields = append(e.Fields, Field{Key: k, Value: val})
	}

	sort.Slice(e.Fields, func(i, j int) bool {
		return e.Fields[i].Key < e.Fields[j].Key
	})

	return e, true
}

func parseLogfmt(raw string, lineNum int) (Entry, bool) {
	pairs := 0
	e := Entry{Raw: raw, Line: lineNum}
	remaining := raw

	for remaining != "" {
		remaining = strings.TrimLeft(remaining, " \t")
		if remaining == "" {
			break
		}

		eqIdx := strings.IndexByte(remaining, '=')
		if eqIdx <= 0 {
			break
		}

		key := remaining[:eqIdx]
		if strings.ContainsAny(key, " \t") {
			break
		}

		remaining = remaining[eqIdx+1:]

		var value string
		if len(remaining) > 0 && remaining[0] == '"' {
			end := 1
			for end < len(remaining) {
				if remaining[end] == '\\' && end+1 < len(remaining) {
					end += 2
					continue
				}
				if remaining[end] == '"' {
					break
				}
				end++
			}
			if end < len(remaining) {
				value = remaining[1:end]
				remaining = remaining[end+1:]
			} else {
				value = remaining[1:]
				remaining = ""
			}
		} else {
			spIdx := strings.IndexAny(remaining, " \t")
			if spIdx == -1 {
				value = remaining
				remaining = ""
			} else {
				value = remaining[:spIdx]
				remaining = remaining[spIdx:]
			}
		}

		pairs++
		switch key {
		case "level", "lvl", "severity":
			e.Level = parseLevel(value)
		case "msg", "message":
			e.Message = value
		case "time", "timestamp", "ts":
			e.Timestamp = parseTimestamp(value)
		default:
			e.Fields = append(e.Fields, Field{Key: key, Value: value})
		}
	}

	if pairs < 2 {
		return Entry{}, false
	}

	return e, true
}

func parsePlain(raw string, lineNum int) Entry {
	e := Entry{
		Raw:     raw,
		Line:    lineNum,
		Message: raw,
		Level:   LevelUnknown,
	}

	upper := strings.ToUpper(raw)
	switch {
	case strings.Contains(upper, "FATAL") || strings.Contains(upper, "PANIC"):
		e.Level = LevelFatal
	case strings.Contains(upper, "ERROR"):
		e.Level = LevelError
	case strings.Contains(upper, "WARN"):
		e.Level = LevelWarn
	case strings.Contains(upper, "INFO"):
		e.Level = LevelInfo
	case strings.Contains(upper, "DEBUG"):
		e.Level = LevelDebug
	case strings.Contains(upper, "TRACE"):
		e.Level = LevelTrace
	}

	return e
}

func parseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace", "trc":
		return LevelTrace
	case "debug", "dbg":
		return LevelDebug
	case "info", "inf", "information":
		return LevelInfo
	case "warn", "wrn", "warning":
		return LevelWarn
	case "error", "err":
		return LevelError
	case "fatal", "ftl", "critical", "panic", "dpanic":
		return LevelFatal
	default:
		return LevelUnknown
	}
}

func parseTimestamp(v any) time.Time {
	switch tv := v.(type) {
	case string:
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006/01/02 15:04:05",
			time.UnixDate,
		} {
			if t, err := time.Parse(layout, tv); err == nil {
				return t
			}
		}
	case float64:
		if tv > 1e12 {
			return time.UnixMilli(int64(tv))
		}
		return time.Unix(int64(tv), 0)
	}
	return time.Time{}
}
