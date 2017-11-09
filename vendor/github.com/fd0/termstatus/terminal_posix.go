package termstatus

const (
	posixMoveCursorHome = "\r"
	posixMoveCursorUp   = "\x1b[1A"
	posixClearLine      = "\x1b[2K"
)

// posixClearLines will clear the current line and the n lines above.
// Afterwards the cursor is positioned at the start of the first cleared line.
func posixClearLines(wr TerminalWriter, n int) error {
	// clear current line
	_, err := wr.Write([]byte(posixMoveCursorHome + posixClearLine))
	if err != nil {
		return err
	}

	for ; n > 0; n-- {
		// clear current line and move on line up
		_, err := wr.Write([]byte(posixMoveCursorUp + posixClearLine))
		if err != nil {
			return err
		}
	}

	return nil
}
