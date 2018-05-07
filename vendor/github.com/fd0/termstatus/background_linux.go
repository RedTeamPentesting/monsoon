package termstatus

import (
	"syscall"
	"unsafe"
)

// IsProcessBackground reports whether the current process is running in the background.
func IsProcessBackground() bool {
	var pid int
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(syscall.Stdin), syscall.TIOCGPGRP, uintptr(unsafe.Pointer(&pid)))

	if err != 0 {
		return false
	}

	return pid != syscall.Getpgrp()
}
