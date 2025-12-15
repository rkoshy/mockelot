package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type DockerRuntime struct {
	client *client.Client
}

func NewDockerRuntime() *DockerRuntime {
	return &DockerRuntime{}
}

func (d *DockerRuntime) Initialize() error {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test connection with ping
	ctx := context.Background()
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		dockerClient.Close()
		return fmt.Errorf("Docker daemon not responding: %w", err)
	}

	d.client = dockerClient
	return nil
}

func (d *DockerRuntime) Name() string {
	return "docker"
}

func (d *DockerRuntime) IsAvailable() bool {
	if d.client == nil {
		return false
	}
	ctx := context.Background()
	_, err := d.client.Ping(ctx)
	return err == nil
}

func (d *DockerRuntime) PullImage(ctx context.Context, imageName string) (io.ReadCloser, error) {
	return d.client.ImagePull(ctx, imageName, image.PullOptions{})
}

func (d *DockerRuntime) CreateContainer(ctx context.Context, config *ContainerCreateConfig) (string, error) {
	// Convert to Docker-specific config
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

	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, config.Name)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (d *DockerRuntime) StartContainer(ctx context.Context, containerID string) error {
	return d.client.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (d *DockerRuntime) StopContainer(ctx context.Context, containerID string, timeout int) error {
	return d.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

func (d *DockerRuntime) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	return d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: force})
}

func (d *DockerRuntime) InspectContainer(ctx context.Context, containerID string) (*ContainerInfo, error) {
	inspect, err := d.client.ContainerInspect(ctx, containerID)
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

func (d *DockerRuntime) FindContainerByName(ctx context.Context, name string) (string, error) {
	containers, err := d.client.ContainerList(ctx, container.ListOptions{All: true})
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

func (d *DockerRuntime) GetContainerStats(ctx context.Context, containerID string) (*ContainerStats, error) {
	stats, err := d.client.ContainerStats(ctx, containerID, false) // false = get stats once, not stream
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

func (d *DockerRuntime) ValidateImage(ctx context.Context, imageName string) error {
	_, _, err := d.client.ImageInspectWithRaw(ctx, imageName)
	return err
}

func (d *DockerRuntime) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	tailStr := fmt.Sprintf("%d", tail)
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tailStr,
	}

	logs, err := d.client.ContainerLogs(ctx, containerID, options)
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
