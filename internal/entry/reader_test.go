package entry

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
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

func TestReadLinesMultilineTabIndent(t *testing.T) {
	input := "level=error msg=\"exception\"\n\tat line1\n\tat line2\nlevel=info msg=\"ok\"\n"
	ch := make(chan Entry, 10)
	ReadLines(context.Background(), strings.NewReader(input), ch)

	var entries []Entry
	for e := range ch {
		entries = append(entries, e)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Raw, "\tat line1") {
		t.Error("expected tab-indented continuation in first entry")
	}
}

func TestReadLinesSingleEntry(t *testing.T) {
	input := `{"level":"info","msg":"only one"}`
	ch := make(chan Entry, 10)
	ReadLines(context.Background(), strings.NewReader(input), ch)

	var entries []Entry
	for e := range ch {
		entries = append(entries, e)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Message != "only one" {
		t.Fatalf("expected 'only one', got %q", entries[0].Message)
	}
}

func TestReadLinesSingleEntryWithContinuation(t *testing.T) {
	input := "level=error msg=\"boom\"\n  details here\n  more details\n"
	ch := make(chan Entry, 10)
	ReadLines(context.Background(), strings.NewReader(input), ch)

	var entries []Entry
	for e := range ch {
		entries = append(entries, e)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 merged entry, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Raw, "details here") {
		t.Error("expected continuation in raw")
	}
	if !strings.Contains(entries[0].Raw, "more details") {
		t.Error("expected second continuation in raw")
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

func TestReadLinesTail(t *testing.T) {
	// Create a temp file.
	f, err := os.CreateTemp("", "lazylogs-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	// Write initial content.
	f.WriteString("{\"level\":\"info\",\"msg\":\"line1\"}\n")
	f.WriteString("{\"level\":\"error\",\"msg\":\"line2\"}\n")
	f.Sync()

	// Open for reading.
	rf, err := os.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan Entry, 100)

	go ReadLinesTail(ctx, rf, ch)

	// Read initial entries.
	var entries []Entry
	timeout := time.After(2 * time.Second)
	for len(entries) < 2 {
		select {
		case e := <-ch:
			entries = append(entries, e)
		case <-timeout:
			t.Fatalf("timeout waiting for initial entries, got %d", len(entries))
		}
	}

	if entries[0].Message != "line1" {
		t.Fatalf("expected 'line1', got %q", entries[0].Message)
	}
	if entries[1].Message != "line2" {
		t.Fatalf("expected 'line2', got %q", entries[1].Message)
	}

	// Append new content to the file.
	f.WriteString("{\"level\":\"warn\",\"msg\":\"line3\"}\n")
	f.Sync()

	// Read the new entry.
	timeout = time.After(2 * time.Second)
	select {
	case e := <-ch:
		if e.Message != "line3" {
			t.Fatalf("expected 'line3', got %q", e.Message)
		}
	case <-timeout:
		t.Fatal("timeout waiting for tailed entry")
	}

	// Cancel and verify channel closes.
	cancel()
	time.Sleep(200 * time.Millisecond)
	_, ok := <-ch
	if ok {
		// Drain any remaining.
		for range ch {
		}
	}
}

func TestReadLinesTailContextCancel(t *testing.T) {
	f, err := os.CreateTemp("", "lazylogs-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("{\"level\":\"info\",\"msg\":\"hello\"}\n")
	f.Sync()

	rf, err := os.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan Entry, 100)

	go ReadLinesTail(ctx, rf, ch)

	// Read initial entry.
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	// Cancel should stop the reader.
	cancel()

	// Channel should eventually close.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // Success.
			}
		case <-deadline:
			t.Fatal("channel not closed after cancel")
		}
	}
}

func TestReadLinesTailMultiline(t *testing.T) {
	f, err := os.CreateTemp("", "lazylogs-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("{\"level\":\"error\",\"msg\":\"exception\"}\n")
	f.WriteString("\tat Stack.trace(Line:1)\n")
	f.WriteString("{\"level\":\"info\",\"msg\":\"ok\"}\n")
	f.Sync()

	rf, err := os.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ch := make(chan Entry, 100)

	go ReadLinesTail(ctx, rf, ch)

	var entries []Entry
	timeout := time.After(2 * time.Second)
	for len(entries) < 2 {
		select {
		case e := <-ch:
			entries = append(entries, e)
		case <-timeout:
			t.Fatalf("timeout, got %d entries", len(entries))
		}
	}

	if !strings.Contains(entries[0].Raw, "Stack.trace") {
		t.Error("expected multiline merge in tail mode")
	}
	if entries[1].Message != "ok" {
		t.Fatalf("expected 'ok', got %q", entries[1].Message)
	}
}
