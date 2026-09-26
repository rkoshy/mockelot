//go:build windows

package server

import (
	"io"
	"log"
	"os/exec"
	"strconv"
	"time"
)

// setProcAttr is a no-op on Windows — process groups work differently and
// taskkill /T handles the tree kill instead.
func setProcAttr(cmd *exec.Cmd) {}

// gracefulKill terminates the process tree on Windows using taskkill /T /F.
func gracefulKill(proc *devServerProcess) {
	if proc.cmd == nil || proc.cmd.Process == nil {
		return
	}
	pid := proc.pid

	// taskkill /T kills the entire process tree; /F forces termination
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	kill.Stdout = io.Discard
	kill.Stderr = io.Discard
	if err := kill.Run(); err != nil {
		log.Printf("[DevServer] taskkill failed for PID %d: %v — trying direct kill", pid, err)
		_ = proc.cmd.Process.Kill()
	}

	// Give the process tree a moment to die before returning
	time.Sleep(1 * time.Second)
}
