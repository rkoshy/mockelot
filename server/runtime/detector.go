package runtime

import (
	"fmt"
	"log"
	"os"
	goruntime "runtime"
	"strings"
)

// DetectRuntime detects and initializes the best available container runtime
func DetectRuntime() (ContainerRuntime, error) {
	// Environment variable override: CONTAINER_RUNTIME=docker|podman
	if envRuntime := os.Getenv("CONTAINER_RUNTIME"); envRuntime != "" {
		return initializeSpecificRuntime(envRuntime)
	}

	// Auto-detect: try Docker first, fallback to Podman
	dockerRuntime := NewDockerRuntime()
	if err := dockerRuntime.Initialize(); err == nil {
		log.Printf("Container runtime: Docker detected")
		return dockerRuntime, nil
	}

	podmanRuntime := NewPodmanRuntime()
	if err := podmanRuntime.Initialize(); err == nil {
		log.Printf("Container runtime: Podman detected")
		return podmanRuntime, nil
	}

	return nil, fmt.Errorf("no container runtime available (tried Docker and Podman)")
}

func initializeSpecificRuntime(name string) (ContainerRuntime, error) {
	switch strings.ToLower(name) {
	case "docker":
		runtime := NewDockerRuntime()
		if err := runtime.Initialize(); err != nil {
			return nil, fmt.Errorf("Docker runtime not available: %w", err)
		}
		return runtime, nil
	case "podman":
		runtime := NewPodmanRuntime()
		if err := runtime.Initialize(); err != nil {
			return nil, fmt.Errorf("Podman runtime not available: %w", err)
		}
		return runtime, nil
	default:
		return nil, fmt.Errorf("unknown container runtime: %s", name)
	}
}

// isWSL detects if running under WSL
func isWSL() bool {
	if goruntime.GOOS != "linux" {
		return false
	}

	// Check for WSL-specific files
	if _, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop"); err == nil {
		return true
	}

	// Check /proc/version for WSL signature
	data, err := os.ReadFile("/proc/version")
	if err == nil && strings.Contains(strings.ToLower(string(data)), "microsoft") {
		return true
	}

	return false
}
