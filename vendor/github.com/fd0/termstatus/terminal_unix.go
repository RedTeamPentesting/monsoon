// +build !windows

package termstatus

import (
	"syscall"
	"unsafe"

	isatty "github.com/mattn/go-isatty"
)

// clearLines will clear the current line and the n lines above. Afterwards the
// cursor is positioned at the start of the first cleared line.
func clearLines(wr TerminalWriter) func(TerminalWriter, int) error {
	return posixClearLines
}

// canUpdateStatus returns true if status lines can be printed, the process
// output is not redirected to a file or pipe.
func canUpdateStatus(wr TerminalWriter) bool {
	return isatty.IsTerminal(wr.Fd())
}

// getTermSize returns the dimensions of the given terminal.
// the code is taken from "golang.org/x/crypto/ssh/terminal"
func getTermSize(wr TerminalWriter) (width, height int, err error) {
	var dimensions [4]uint16

	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(wr.Fd()), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&dimensions)), 0, 0, 0); err != 0 {
		return -1, -1, err
	}
	return int(dimensions[1]), int(dimensions[0]), nil
}
