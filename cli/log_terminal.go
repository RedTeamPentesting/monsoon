package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/fd0/termstatus"
)

// taken from here (MIT licensed)
// https://github.com/acarl005/stripansi/blob/5a71ef0e047df0427e87a79f27009029921f1f9b/stripansi.go#L7
var ansiEscapeSequenceRegEx = regexp.MustCompile(
	"[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|" +
		"(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

// LogTerminal writes data to a second writer in addition to the terminal.
type LogTerminal struct {
	*termstatus.Terminal
	io.Writer
}

// Printf prints a message with formatting.
func (lt *LogTerminal) Printf(msg string, data ...interface{}) {
	lt.Print(fmt.Sprintf(msg, data...))
}

// Print prints a message.
func (lt *LogTerminal) Print(msg string) {
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}

	lt.Terminal.Print(msg)

	strippedMsg := ansiEscapeSequenceRegEx.ReplaceAllString(msg, "")
	_, _ = lt.Writer.Write([]byte(strippedMsg))
}
