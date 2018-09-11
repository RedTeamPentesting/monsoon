package shell

import (
	"strconv"
	"strings"
)

func escapeParam(s string) string {
	if strings.ContainsAny(s, "$& ") {
		return strconv.Quote(s)
	}

	return s
}

// Join returns a shell command line to run the program.
func Join(args []string) (s string) {
	first := true
	for _, arg := range args {
		if !first {
			s += " "
		}
		s += escapeParam(arg)
		first = false
	}
	return s
}
