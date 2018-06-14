package fuzz

import (
	"strconv"
	"strings"
)

func escapeParam(s string) string {
	if strings.ContainsAny(s, " ") {
		return strconv.Quote(s)
	}

	return s
}

// recreateCommandline tries to reconstruct the command-line used to call the binary.
func recreateCommandline(args []string) (s string) {
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
