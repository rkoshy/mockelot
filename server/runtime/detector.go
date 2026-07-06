package runtime

import (
	"fmt"
	"log"
	"os"
	goruntime "runtime"
	"strings"
)

// RuntimeDetectionError carries the per-runtime failure reasons so callers can
// build actionable error messages for the user.
type RuntimeDetectionError struct {
	DockerError  string
	PodmanError  string
}

func (e *RuntimeDetectionError) Error() string {
	return fmt.Sprintf("no container runtime available (Docker: %s; Podman: %s)", e.DockerError, e.PodmanError)
}

// DetectRuntime detects and initializes the best available container runtime
func DetectRuntime() (ContainerRuntime, error) {
	// Environment variable override: CONTAINER_RUNTIME=docker|podman
	if envRuntime := os.Getenv("CONTAINER_RUNTIME"); envRuntime != "" {
		return initializeSpecificRuntime(envRuntime)
	}

	// Auto-detect: try Docker first, fallback to Podman
	var dockerErrMsg, podmanErrMsg string

	dockerRuntime := NewDockerRuntime()
	if err := dockerRuntime.Initialize(); err == nil {
		log.Printf("Container runtime: Docker detected")
		return dockerRuntime, nil
	} else {
		dockerErrMsg = err.Error()
		log.Printf("Container runtime: Docker not available: %v", err)
	}

	podmanRuntime := NewPodmanRuntime()
	if err := podmanRuntime.Initialize(); err == nil {
		log.Printf("Container runtime: Podman detected")
		return podmanRuntime, nil
	} else {
		podmanErrMsg = err.Error()
		log.Printf("Container runtime: Podman not available: %v", err)
	}

	return nil, &RuntimeDetectionError{DockerError: dockerErrMsg, PodmanError: podmanErrMsg}
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
