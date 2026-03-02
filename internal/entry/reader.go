package entry

import (
	"bufio"
	"context"
	"io"
	"os"
	"time"
)

// ReadLines reads lines from r and sends parsed entries to ch.
// It closes ch when the reader is exhausted or context is cancelled.
// Scanner errors are reported as error-level entries.
func ReadLines(ctx context.Context, r io.Reader, ch chan<- Entry) {
	defer close(ch)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNum := 0
	var pending *Entry

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := scanner.Text()
		if line == "" {
			continue
		}
		lineNum++

		if isContinuation(line) && pending != nil {
			pending.Raw += "\n" + line
			pending.Message += "\n" + line
			continue
		}

		if pending != nil {
			select {
			case ch <- *pending:
			case <-ctx.Done():
				return
			}
		}
		e := ParseLine(line, lineNum)
		pending = &e
	}

	// Flush last entry.
	if pending != nil {
		select {
		case ch <- *pending:
		case <-ctx.Done():
			return
		}
	}

	// Report scanner errors.
	if err := scanner.Err(); err != nil {
		select {
		case ch <- Entry{
			Raw:     "read error: " + err.Error(),
			Level:   LevelError,
			Message: "read error: " + err.Error(),
			Line:    lineNum + 1,
		}:
		case <-ctx.Done():
		}
	}
}

// ReadLinesTail reads a file and follows it for new data, like tail -f.
// It closes ch when context is cancelled.
func ReadLinesTail(ctx context.Context, f *os.File, ch chan<- Entry) {
	defer close(ch)

	reader := bufio.NewReaderSize(f, 64*1024)
	lineNum := 0
	var pending *Entry

	flush := func() bool {
		if pending != nil {
			select {
			case ch <- *pending:
			case <-ctx.Done():
				return false
			}
			pending = nil
		}
		return true
	}

	for {
		line, err := readLine(reader)
		if err != nil {
			if err == io.EOF {
				if !flush() {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
					continue
				}
			}
			// Real error.
			flush()
			select {
			case ch <- Entry{
				Raw:     "read error: " + err.Error(),
				Level:   LevelError,
				Message: "read error: " + err.Error(),
				Line:    lineNum + 1,
			}:
			case <-ctx.Done():
			}
			return
		}

		if line == "" {
			continue
		}
		lineNum++

		if isContinuation(line) && pending != nil {
			pending.Raw += "\n" + line
			pending.Message += "\n" + line
			continue
		}

		if !flush() {
			return
		}
		e := ParseLine(line, lineNum)
		pending = &e
	}
}

// readLine reads a complete line from a bufio.Reader, handling long lines.
func readLine(r *bufio.Reader) (string, error) {
	var line []byte
	for {
		part, isPrefix, err := r.ReadLine()
		if err != nil {
			if len(line) > 0 {
				return string(line), nil
			}
			return "", err
		}
		line = append(line, part...)
		if !isPrefix {
			return string(line), nil
		}
	}
}

// isContinuation returns true if a line appears to be a continuation
// of a previous log entry (e.g., stack trace, indented output).
func isContinuation(line string) bool {
	if len(line) == 0 {
		return false
	}
	return line[0] == ' ' || line[0] == '\t'
}
