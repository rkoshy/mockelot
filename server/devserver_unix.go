//go:build !windows

package server

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcAttr configures the child process to run in its own process group
// so we can kill the entire tree (npm → node → vite, etc.).
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// gracefulKill sends SIGINT to the process group and waits up to 5 seconds,
// then force-kills with SIGKILL if the process hasn't exited.
func gracefulKill(proc *devServerProcess) {
	if proc.cmd == nil || proc.cmd.Process == nil {
		return
	}

	// Send SIGINT to the entire process group (kills npm + child node/vite/etc.)
	pgid, err := syscall.Getpgid(proc.pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGINT)
	} else {
		_ = proc.cmd.Process.Signal(syscall.SIGINT)
	}

	// Wait up to 5 seconds for graceful shutdown
	done := make(chan struct{})
	go func() {
		proc.cmd.Wait() //nolint:errcheck
		close(done)
	}()

	select {
	case <-done:
		// Exited cleanly
	case <-time.After(5 * time.Second):
		// Force-kill the process group
		if pgid, err := syscall.Getpgid(proc.pid); err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = proc.cmd.Process.Kill()
		}
	}
}
