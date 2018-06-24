// +build windows

package main

import (
	"golang.org/x/sys/windows"
)

// getTermWidth tries to find out how many columns there are on the current
// terminal. If an error is encountered, it'll return a default value
// of 80.
func getTermWidth(fd int) int {
	width := 80

	var info windows.ConsoleScreenBufferInfo
	err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info)
	if err == nil {
		width = int(info.Size.X)
	}
	return width
}
