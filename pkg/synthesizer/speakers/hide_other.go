//go:build !windows && !darwin

package speakers

import "os/exec"

func hideWindow(_ *exec.Cmd) {}
