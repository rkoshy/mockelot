package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

// TranslatePath converts Windows paths to WSL paths for volume mounts
func TranslatePath(hostPath string) string {
	if !isWSL() {
		return hostPath
	}

	// Check if path is already a Unix path
	if strings.HasPrefix(hostPath, "/") {
		return hostPath
	}

	// Convert Windows path to WSL path
	// C:\Users\foo -> /mnt/c/Users/foo
	if len(hostPath) >= 2 && hostPath[1] == ':' {
		drive := strings.ToLower(string(hostPath[0]))
		rest := filepath.ToSlash(hostPath[2:])
		return "/mnt/" + drive + rest
	}

	return hostPath
}

// GetWSLHostIP returns the Windows host IP from WSL perspective
func GetWSLHostIP() (string, error) {
	if !isWSL() {
		return "127.0.0.1", nil
	}

	// Read nameserver from /etc/resolv.conf (points to Windows host)
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return "127.0.0.1", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1], nil
			}
		}
	}

	return "127.0.0.1", nil
}
