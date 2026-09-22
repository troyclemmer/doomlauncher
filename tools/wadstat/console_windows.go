package main

import (
	"bufio"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// ownsConsole reports whether this process is the only one attached to its
// console window. That is true when the exe was double-clicked or had a file
// dropped on it, and false when it was run from an existing terminal - which
// tells us whether the window is about to vanish before anything can be read.
func ownsConsole() bool {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")
	if proc.Find() != nil {
		return false
	}
	var pids [8]uint32
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return n == 1
}

// pauseIfLaunchedFromExplorer keeps the window open long enough to read.
func pauseIfLaunchedFromExplorer() {
	if !ownsConsole() {
		return
	}
	fmt.Print("\nPress Enter to close...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}
