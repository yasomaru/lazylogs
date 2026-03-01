package entry

import (
	"testing"
	"time"
)

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		level   Level
		message string
		hasTime bool
		fields  int
	}{
		{
			name:    "slog style",
			input:   `{"time":"2025-03-01T10:00:00Z","level":"info","msg":"hello world","count":42}`,
			level:   LevelInfo,
			message: "hello world",
			hasTime: true,
			fields:  1,
		},
		{
			name:    "zap style",
			input:   `{"level":"error","ts":1709287200.0,"msg":"failed","error":"timeout"}`,
			level:   LevelError,
			message: "failed",
			hasTime: true,
			fields:  1,
		},
		{
			name:    "zerolog style",
			input:   `{"level":"warn","time":"2025-03-01T10:00:00Z","message":"slow query","duration":"2s"}`,
			level:   LevelWarn,
			message: "slow query",
			hasTime: true,
			fields:  1,
		},
		{
			name:    "fatal level",
			input:   `{"level":"fatal","msg":"out of memory"}`,
			level:   LevelFatal,
			message: "out of memory",
			hasTime: false,
			fields:  0,
		},
		{
			name:    "debug level",
			input:   `{"level":"debug","msg":"cache hit","key":"user:1"}`,
			level:   LevelDebug,
			message: "cache hit",
			hasTime: false,
			fields:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := ParseLine(tt.input, 1)
			if e.Level != tt.level {
				t.Errorf("level: got %v, want %v", e.Level, tt.level)
			}
			if e.Message != tt.message {
				t.Errorf("message: got %q, want %q", e.Message, tt.message)
			}
			if tt.hasTime && e.Timestamp.IsZero() {
				t.Error("expected non-zero timestamp")
			}
			if !tt.hasTime && !e.Timestamp.IsZero() {
				t.Error("expected zero timestamp")
			}
			if len(e.Fields) != tt.fields {
				t.Errorf("fields: got %d, want %d", len(e.Fields), tt.fields)
			}
		})
	}
}

func TestParseLogfmt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		level   Level
		message string
		fields  int
	}{
		{
			name:    "basic logfmt",
			input:   `level=info msg="server started" port=8080`,
			level:   LevelInfo,
			message: "server started",
			fields:  1,
		},
		{
			name:    "with timestamp",
			input:   `ts=2025-03-01T10:00:00Z level=error msg="connection failed" host=db.local`,
			level:   LevelError,
			message: "connection failed",
			fields:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := ParseLine(tt.input, 1)
			if e.Level != tt.level {
				t.Errorf("level: got %v, want %v", e.Level, tt.level)
			}
			if e.Message != tt.message {
				t.Errorf("message: got %q, want %q", e.Message, tt.message)
			}
			if len(e.Fields) != tt.fields {
				t.Errorf("fields: got %d, want %d", len(e.Fields), tt.fields)
			}
		})
	}
}

func TestParsePlainText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		level Level
	}{
		{"error line", "2025-03-01 ERROR something broke", LevelError},
		{"warn line", "WARNING: disk usage high", LevelWarn},
		{"info line", "INFO: server started", LevelInfo},
		{"debug line", "DEBUG checking cache", LevelDebug},
		{"unknown line", "just some text", LevelUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := ParseLine(tt.input, 1)
			if e.Level != tt.level {
				t.Errorf("level: got %v, want %v", e.Level, tt.level)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	e := ParseLine(`{"time":"2025-03-01T10:30:00Z","level":"info","msg":"test"}`, 1)
	expected := time.Date(2025, 3, 1, 10, 30, 0, 0, time.UTC)
	if !e.Timestamp.Equal(expected) {
		t.Errorf("timestamp: got %v, want %v", e.Timestamp, expected)
	}
}

func TestParseUnixTimestamp(t *testing.T) {
	// 1709287200 = 2024-03-01T10:00:00Z
	e := ParseLine(`{"ts":1709287200,"level":"info","msg":"test"}`, 1)
	if e.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp for unix seconds")
	}
}

func TestParseUnixMilliTimestamp(t *testing.T) {
	e := ParseLine(`{"ts":1709287200000,"level":"info","msg":"test"}`, 1)
	if e.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp for unix milliseconds")
	}
}

func TestEmptyLine(t *testing.T) {
	e := ParseLine("", 1)
	if e.Level != LevelUnknown {
		t.Errorf("expected unknown level for empty line, got %v", e.Level)
	}
}
