//go:build windows
// +build windows

package main

import (
	"os"

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

func prepareTerminal() (reset func()) {
	var mode uint32

	stdoutHandle := windows.Handle(os.Stdout.Fd())

	err := windows.GetConsoleMode(stdoutHandle, &mode)
	if err != nil {
		return func() {}
	}

	err = windows.SetConsoleMode(stdoutHandle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	if err != nil {
		return func() {}
	}

	stderrHandle := windows.Handle(os.Stderr.Fd())

	err = windows.GetConsoleMode(stderrHandle, &mode)
	if err != nil {
		return func() {
			_ = windows.SetConsoleMode(stdoutHandle, mode)
		}
	}

	err = windows.SetConsoleMode(stderrHandle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	if err != nil {
		return func() {
			_ = windows.SetConsoleMode(stdoutHandle, mode)
		}
	}

	return func() {
		_ = windows.SetConsoleMode(stdoutHandle, mode)
		_ = windows.SetConsoleMode(stderrHandle, mode)
	}
}
