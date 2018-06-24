// +build !windows

package main

import (
	"golang.org/x/sys/unix"
)

// getTermWidth tries to find out how many columns there are on file
// descriptor. If an error is encountered, it'll return a default value of 80.
func getTermWidth(fd int) int {
	width := 80
	size, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err == nil {
		width = int(size.Col)
	}
	return width
}
