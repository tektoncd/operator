//go:build !windows
// +build !windows

package term

import (
	"os"
)

func enableVirtualTerminalProcessing(f *os.File) error {
	return nil
}

func openTTY() (*os.File, error) {
	return os.Open("/dev/tty")
}
