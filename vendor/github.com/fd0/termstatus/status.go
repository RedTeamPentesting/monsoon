package termstatus

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

// Terminal is used to write messages and display status lines which can be
// updated. When the output is redirected to a file, the status lines are not
// printed.
type Terminal struct {
	dst             TerminalWriter
	buf             *bytes.Buffer
	msg             chan message
	status          chan status
	finish          chan chan error
	canUpdateStatus bool
	clearLines      func(TerminalWriter, int) error
}

// TerminalWriter is an io.Writer which also has a file descriptor.
type TerminalWriter interface {
	io.Writer
	Fd() uintptr
}

type message struct {
	line string
	ch   chan<- error
}

type status struct {
	lines []string
	ch    chan<- error
}

// New returns a new Terminal for dst. A goroutine is started to update the
// terminal. It is terminated when ctx is cancelled. When dst is redirected to
// a file (e.g. via shell output redirection), no status lines are printed.
func New(ctx context.Context, dst TerminalWriter) *Terminal {
	t := &Terminal{
		buf:             bytes.NewBuffer(nil),
		dst:             dst,
		msg:             make(chan message),
		status:          make(chan status),
		finish:          make(chan chan error),
		canUpdateStatus: canUpdateStatus(dst),
		clearLines:      clearLines(dst),
	}

	if t.canUpdateStatus {
		go t.run(ctx)
	} else {
		go t.runWithoutStatus(ctx)
	}

	return t
}

func countLines(buf []byte) int {
	lines := 0
	sc := bufio.NewScanner(bytes.NewReader(buf))
	for sc.Scan() {
		lines++
	}
	return lines
}

type stringWriter interface {
	WriteString(string) (int, error)
}

// run listens on the channels and updates the terminal screen.
func (t *Terminal) run(ctx context.Context) {
	statusBuf := bytes.NewBuffer(nil)
	statusLines := 0
	for {
		select {
		case <-ctx.Done():
			t.undoStatus(statusLines)
			return
		case errch := <-t.finish:
			errch <- t.undoStatus(statusLines)
			return
		case msg := <-t.msg:
			err := t.undoStatus(statusLines)
			if err != nil {
				msg.ch <- err
				continue
			}

			if w, ok := t.dst.(stringWriter); ok {
				_, err = w.WriteString(msg.line)
			} else {
				_, err = t.dst.Write([]byte(msg.line))
			}

			if err != nil {
				msg.ch <- err
				continue
			}

			_, err = t.dst.Write(statusBuf.Bytes())
			if err != nil {
				msg.ch <- err
				continue
			}

			msg.ch <- nil

		case stat := <-t.status:
			err := t.undoStatus(statusLines)
			if err != nil {
				stat.ch <- err
				continue
			}

			statusBuf.Reset()
			for _, line := range stat.lines {
				statusBuf.WriteString(line)
			}
			statusLines = len(stat.lines)

			_, err = t.dst.Write(statusBuf.Bytes())
			stat.ch <- err
		}
	}
}

// runWithoutStatus listens on the channels and just prints out the messages,
// without status lines.
func (t *Terminal) runWithoutStatus(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case errch := <-t.finish:
			errch <- nil
			return
		case msg := <-t.msg:
			var err error
			if w, ok := t.dst.(stringWriter); ok {
				_, err = w.WriteString(msg.line)
			} else {
				_, err = t.dst.Write([]byte(msg.line))
			}

			msg.ch <- err

		case msg := <-t.status:
			// discard status lines
			msg.ch <- nil
		}
	}
}

func (t *Terminal) undoStatus(lines int) error {
	if lines == 0 {
		return nil
	}

	lines--
	return t.clearLines(t.dst, lines)
}

// Print writes a line to the terminal.
func (t *Terminal) Print(line string) error {
	// make sure the line ends with a line break
	if line[len(line)-1] != '\n' {
		line += "\n"
	}

	ch := make(chan error, 1)
	t.msg <- message{line: line, ch: ch}
	return <-ch
}

// Printf uses fmt.Sprintf to write a line to the terminal.
func (t *Terminal) Printf(msg string, args ...interface{}) error {
	s := fmt.Sprintf(msg, args...)
	return t.Print(s)
}

// SetStatus updates the status lines.
func (t *Terminal) SetStatus(lines []string) error {
	if len(lines) == 0 {
		return nil
	}

	width, _, err := getTermSize(t.dst)
	if err != nil {
		// use 80 columns by default
		width = 80
	}

	// make sure that all lines have a line break and are not too long
	for i, line := range lines {
		line = strings.TrimRight(line, "\n")

		if len(line) > width {
			line = line[:width-1]
		}
		line += "\n"
		lines[i] = line
	}

	// make sure the last line does not have a line break
	last := len(lines) - 1
	lines[last] = strings.TrimRight(lines[last], "\n")

	ch := make(chan error, 1)
	t.status <- status{lines: lines, ch: ch}
	return <-ch
}

// Finish removes the status lines.
func (t *Terminal) Finish() error {
	ch := make(chan error)
	t.finish <- ch
	return <-ch
}
