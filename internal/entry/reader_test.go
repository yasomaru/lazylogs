package entry

import (
	"context"
	"strings"
	"testing"
)

func TestReadLines(t *testing.T) {
	input := `{"level":"info","msg":"hello"}
{"level":"error","msg":"fail"}
`
	ch := make(chan Entry, 10)
	ReadLines(context.Background(), strings.NewReader(input), ch)

	var entries []Entry
	for e := range ch {
		entries = append(entries, e)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Level != LevelInfo {
		t.Errorf("entry 0: expected info, got %v", entries[0].Level)
	}
	if entries[1].Level != LevelError {
		t.Errorf("entry 1: expected error, got %v", entries[1].Level)
	}
}

func TestReadLinesSkipsEmpty(t *testing.T) {
	input := "line1=val1 line2=val2\n\n\nline3=val3 line4=val4\n"
	ch := make(chan Entry, 10)
	ReadLines(context.Background(), strings.NewReader(input), ch)

	var entries []Entry
	for e := range ch {
		entries = append(entries, e)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (skipping empty), got %d", len(entries))
	}
}

func TestReadLinesContextCancel(t *testing.T) {
	r := strings.NewReader("")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	ch := make(chan Entry, 10)
	ReadLines(ctx, r, ch)

	// Channel should be closed.
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed")
	}
}

func TestReadLinesMultiline(t *testing.T) {
	input := `{"level":"error","msg":"panic occurred"}
	at java.lang.Thread.run(Thread.java:748)
	at com.example.Main.main(Main.java:10)
{"level":"info","msg":"recovered"}
`
	ch := make(chan Entry, 10)
	ReadLines(context.Background(), strings.NewReader(input), ch)

	var entries []Entry
	for e := range ch {
		entries = append(entries, e)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (multiline merged), got %d", len(entries))
	}

	// First entry should contain the stack trace.
	if !strings.Contains(entries[0].Raw, "java.lang.Thread") {
		t.Error("expected first entry to contain stack trace")
	}
	if entries[1].Message != "recovered" {
		t.Errorf("expected second entry message 'recovered', got %q", entries[1].Message)
	}
}

func TestIsContinuation(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"\tat java.lang.Thread.run", true},
		{"  File \"/app/main.py\"", true},
		{"normal line", false},
		{"{\"level\":\"info\"}", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isContinuation(tt.line); got != tt.want {
			t.Errorf("isContinuation(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}
