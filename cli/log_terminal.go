package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/fd0/termstatus"
)

// LogTerminal writes data to a second writer in addition to the terminal.
type LogTerminal struct {
	*termstatus.Terminal
	io.Writer
}

// Printf prints a messsage with formatting.
func (lt *LogTerminal) Printf(msg string, data ...interface{}) {
	lt.Print(fmt.Sprintf(msg, data...))
}

// Print prints a message.
func (lt *LogTerminal) Print(msg string) {
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	lt.Terminal.Print(msg)
	lt.Writer.Write([]byte(msg))
}
