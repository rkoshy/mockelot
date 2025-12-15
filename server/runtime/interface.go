package runtime

import (
	"context"
	"io"
)

// ContainerRuntime abstracts Docker/Podman operations
type ContainerRuntime interface {
	// Initialize checks if runtime is available and initializes client
	Initialize() error

	// Name returns the runtime name ("docker" or "podman")
	Name() string

	// IsAvailable checks if runtime is installed and accessible
	IsAvailable() bool

	// PullImage pulls a container image
	PullImage(ctx context.Context, imageName string) (io.ReadCloser, error)

	// CreateContainer creates a container with given config
	CreateContainer(ctx context.Context, config *ContainerCreateConfig) (containerID string, err error)

	// StartContainer starts a container
	StartContainer(ctx context.Context, containerID string) error

	// StopContainer stops a container
	StopContainer(ctx context.Context, containerID string, timeout int) error

	// RemoveContainer removes a container
	RemoveContainer(ctx context.Context, containerID string, force bool) error

	// InspectContainer gets container details
	InspectContainer(ctx context.Context, containerID string) (*ContainerInfo, error)

	// FindContainerByName finds a container by name
	FindContainerByName(ctx context.Context, name string) (containerID string, err error)

	// GetContainerStats gets real-time resource usage statistics
	GetContainerStats(ctx context.Context, containerID string) (*ContainerStats, error)

	// ValidateImage checks if image exists locally
	ValidateImage(ctx context.Context, imageName string) error

	// GetContainerLogs gets container stdout/stderr logs
	GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error)
}

// ContainerCreateConfig contains container creation parameters
type ContainerCreateConfig struct {
	Name         string            // Container name (e.g., "mockelot-myendpoint")
	Image        string
	Env          []string
	ExposedPorts []string          // e.g., "8080/tcp"
	PortBindings map[string]string // containerPort -> hostPort (e.g., "8080/tcp" -> "0")
	Mounts       []Mount
}

// Mount represents a volume mount
type Mount struct {
	Source   string // Host path
	Target   string // Container path
	ReadOnly bool
}

// ContainerInfo contains container runtime information
type ContainerInfo struct {
	ID      string
	Running bool
	Status  string
	Ports   map[string]string // containerPort -> hostPort
}

// ContainerStats contains container resource usage statistics
type ContainerStats struct {
	CPUPercent      float64 // CPU usage percentage (0-100+)
	MemoryUsageMB   float64 // Memory usage in MB
	MemoryLimitMB   float64 // Memory limit in MB (0 if unlimited)
	MemoryPercent   float64 // Memory usage percentage
	NetworkRxBytes  uint64  // Network bytes received
	NetworkTxBytes  uint64  // Network bytes transmitted
	BlockReadBytes  uint64  // Block I/O bytes read
	BlockWriteBytes uint64  // Block I/O bytes written
	PIDs            uint64  // Number of processes
}
