//go:build windows

package speakers

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps the Piper console window from flashing up on each call.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
