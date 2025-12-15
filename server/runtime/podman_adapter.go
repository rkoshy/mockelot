package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type PodmanRuntime struct {
	client *client.Client
}

func NewPodmanRuntime() *PodmanRuntime {
	return &PodmanRuntime{}
}

func (p *PodmanRuntime) Initialize() error {
	// Podman socket detection
	socketPath := getPodmanSocketPath()

	podmanClient, err := client.NewClientWithOpts(
		client.WithHost(socketPath),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("failed to create Podman client: %w", err)
	}

	// Test connection
	ctx := context.Background()
	_, err = podmanClient.Ping(ctx)
	if err != nil {
		podmanClient.Close()
		return fmt.Errorf("Podman service not responding: %w", err)
	}

	p.client = podmanClient
	return nil
}

func (p *PodmanRuntime) Name() string {
	return "podman"
}

func (p *PodmanRuntime) IsAvailable() bool {
	if p.client == nil {
		return false
	}
	ctx := context.Background()
	_, err := p.client.Ping(ctx)
	return err == nil
}

func (p *PodmanRuntime) PullImage(ctx context.Context, imageName string) (io.ReadCloser, error) {
	return p.client.ImagePull(ctx, imageName, image.PullOptions{})
}

func (p *PodmanRuntime) CreateContainer(ctx context.Context, config *ContainerCreateConfig) (string, error) {
	// Convert to Podman-specific config (same as Docker)
	portSet := nat.PortSet{}
	portBindings := nat.PortMap{}

	for containerPort, hostPort := range config.PortBindings {
		natPort := nat.Port(containerPort)
		portSet[natPort] = struct{}{}
		portBindings[natPort] = []nat.PortBinding{{HostPort: hostPort}}
	}

	mounts := []mount.Mount{}
	for _, m := range config.Mounts {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.Source,
			Target:   m.Target,
			ReadOnly: m.ReadOnly,
		})
	}

	containerConfig := &container.Config{
		Image:        config.Image,
		Env:          config.Env,
		ExposedPorts: portSet,
	}

	hostConfig := &container.HostConfig{
		Mounts:       mounts,
		PortBindings: portBindings,
	}

	resp, err := p.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, config.Name)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (p *PodmanRuntime) StartContainer(ctx context.Context, containerID string) error {
	return p.client.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (p *PodmanRuntime) StopContainer(ctx context.Context, containerID string, timeout int) error {
	return p.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

func (p *PodmanRuntime) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	return p.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: force})
}

func (p *PodmanRuntime) InspectContainer(ctx context.Context, containerID string) (*ContainerInfo, error) {
	inspect, err := p.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	info := &ContainerInfo{
		ID:      inspect.ID,
		Running: inspect.State.Running,
		Status:  inspect.State.Status,
		Ports:   make(map[string]string),
	}

	// Extract port mappings
	for portKey, bindings := range inspect.NetworkSettings.Ports {
		if len(bindings) > 0 {
			info.Ports[string(portKey)] = bindings[0].HostPort
		}
	}

	return info, nil
}

func (p *PodmanRuntime) FindContainerByName(ctx context.Context, name string) (string, error) {
	containers, err := p.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}

	// Match by name (container names have leading slash)
	for _, c := range containers {
		for _, cName := range c.Names {
			if cName == "/"+name || cName == name {
				return c.ID, nil
			}
		}
	}

	return "", fmt.Errorf("container not found: %s", name)
}

func (p *PodmanRuntime) GetContainerStats(ctx context.Context, containerID string) (*ContainerStats, error) {
	stats, err := p.client.ContainerStats(ctx, containerID, false) // false = get stats once, not stream
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()

	// Decode stats JSON
	var v container.StatsResponse
	if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
		return nil, err
	}

	// Calculate CPU percentage
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	// Calculate memory
	memUsageMB := float64(v.MemoryStats.Usage) / 1024.0 / 1024.0
	memLimitMB := float64(v.MemoryStats.Limit) / 1024.0 / 1024.0
	memPercent := 0.0
	if v.MemoryStats.Limit > 0 {
		memPercent = (float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit)) * 100.0
	}

	// Calculate network I/O
	var netRx, netTx uint64
	for _, netStats := range v.Networks {
		netRx += netStats.RxBytes
		netTx += netStats.TxBytes
	}

	// Calculate block I/O
	var blockRead, blockWrite uint64
	for _, bioEntry := range v.BlkioStats.IoServiceBytesRecursive {
		if bioEntry.Op == "Read" || bioEntry.Op == "read" {
			blockRead += bioEntry.Value
		} else if bioEntry.Op == "Write" || bioEntry.Op == "write" {
			blockWrite += bioEntry.Value
		}
	}

	return &ContainerStats{
		CPUPercent:      cpuPercent,
		MemoryUsageMB:   memUsageMB,
		MemoryLimitMB:   memLimitMB,
		MemoryPercent:   memPercent,
		NetworkRxBytes:  netRx,
		NetworkTxBytes:  netTx,
		BlockReadBytes:  blockRead,
		BlockWriteBytes: blockWrite,
		PIDs:            v.PidsStats.Current,
	}, nil
}

func (p *PodmanRuntime) ValidateImage(ctx context.Context, imageName string) error {
	_, _, err := p.client.ImageInspectWithRaw(ctx, imageName)
	return err
}

func (p *PodmanRuntime) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	tailStr := fmt.Sprintf("%d", tail)
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tailStr,
	}

	logs, err := p.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer logs.Close()

	// Read all logs
	logBytes, err := io.ReadAll(logs)
	if err != nil {
		return "", err
	}

	return string(logBytes), nil
}

// getPodmanSocketPath returns the Podman socket path based on OS
func getPodmanSocketPath() string {
	// Linux: unix:///run/user/{UID}/podman/podman.sock
	// Windows/WSL: Check multiple locations
	// macOS: unix:///Users/{user}/.local/share/containers/podman/machine/podman.sock

	if isWSL() {
		// WSL-specific Podman socket detection
		return detectWSLPodmanSocket()
	}

	// Default Linux path
	return "unix:///run/podman/podman.sock"
}

func detectWSLPodmanSocket() string {
	// Check common WSL Podman socket locations
	candidates := []string{
		"unix:///run/user/1000/podman/podman.sock",
		"unix:///run/podman/podman.sock",
	}

	for _, path := range candidates {
		if socketExists(path) {
			return path
		}
	}

	return candidates[0] // Default to first
}

func socketExists(socketPath string) bool {
	// Remove "unix://" prefix
	path := strings.TrimPrefix(socketPath, "unix://")
	_, err := os.Stat(path)
	return err == nil
}
