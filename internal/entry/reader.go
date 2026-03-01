package entry

import (
	"bufio"
	"io"
)

// ReadLines reads lines from r and sends parsed entries to ch.
// It closes ch when the reader is exhausted.
func ReadLines(r io.Reader, ch chan<- Entry) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		ch <- ParseLine(line, lineNum)
	}
	close(ch)
}
