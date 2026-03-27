//go:build windows

package redteam

import (
	"os"

	"golang.org/x/sys/windows"
)

// init enables virtual terminal processing on Windows 10+ so that ANSI escape
// codes are interpreted by the console host.
func init() {
	enableVT(os.Stdout)
	enableVT(os.Stderr)
}

func enableVT(f *os.File) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return
	}
	_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
