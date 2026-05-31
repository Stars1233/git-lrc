//go:build !windows

package appcore

import (
	"os/exec"
	"syscall"
)

// setupSessionDetach configures cmd to run as a new session leader so the
// background worker survives after the parent process exits.
func setupSessionDetach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
