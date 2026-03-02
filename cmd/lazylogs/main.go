package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/syasoda/lazylogs/internal/entry"
	"github.com/syasoda/lazylogs/internal/tui"
)

var version = "dev"

func main() {
	var columns string
	var showVersion bool
	var followFile bool

	flag.StringVar(&columns, "columns", "", "comma-separated columns (e.g. time,level,msg,status,latency)")
	flag.StringVar(&columns, "c", "", "comma-separated columns (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&showVersion, "v", false, "print version (shorthand)")
	flag.BoolVar(&followFile, "f", false, "follow file for new data (like tail -f)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `lazylogs - TUI structured log viewer

Usage:
  lazylogs [flags] [file]
  command | lazylogs

Flags:
  -c, --columns string   comma-separated columns for table display
                          (e.g. time,level,msg,status,latency)
  -f                     follow file (like tail -f)
  -v, --version          print version and exit

Supported formats:
  JSON Lines   {"level":"info","msg":"hello","time":"..."}
  logfmt       level=info msg=hello time=2024-01-01T00:00:00Z
  Plain text   2024-01-01 INFO hello world

Keys:
  j/k, ↑/↓    scroll
  enter        detail view
  /            search (supports regex)
  t            time range filter
  1            toggle error/fatal
  2            toggle warn
  3            toggle info
  4            toggle debug/trace
  0            show all levels
  f            toggle follow mode
  g/G          top/bottom
  q            quit

Examples:
  lazylogs app.log
  lazylogs -f app.log
  lazylogs --columns time,level,msg,status,latency app.log
  cat app.log | lazylogs
  kubectl logs -f pod | lazylogs
  docker logs container 2>&1 | lazylogs
`)
	}
	flag.Parse()

	if showVersion {
		fmt.Printf("lazylogs %s\n", version)
		os.Exit(0)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var reader io.Reader
	var fileForTail *os.File
	var ttyInput *os.File

	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if followFile {
			fileForTail = f
		} else {
			reader = f
		}
	} else {
		stat, err := os.Stdin.Stat()
		if err != nil || stat.Mode()&os.ModeCharDevice != 0 {
			flag.Usage()
			os.Exit(2)
		}
		reader = os.Stdin
		var ttyErr error
		ttyInput, ttyErr = os.Open("/dev/tty")
		if ttyErr != nil {
			fmt.Fprintf(os.Stderr, "error: cannot open /dev/tty: %v\n", ttyErr)
			os.Exit(1)
		}
		defer ttyInput.Close()
	}

	var cols []string
	if columns != "" {
		for _, c := range strings.Split(columns, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				cols = append(cols, c)
			}
		}
	}

	ch := make(chan entry.Entry, 4096)
	if fileForTail != nil {
		go entry.ReadLinesTail(ctx, fileForTail, ch)
	} else {
		go entry.ReadLines(ctx, reader, ch)
	}

	model := tui.NewModel(ch, cols)

	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if ttyInput != nil {
		opts = append(opts, tea.WithInput(ttyInput))
	}

	p := tea.NewProgram(model, opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
