package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"mockelot/models"
	"mockelot/server/runtime"

	"github.com/dop251/goja"
)

// EventSender interface for sending events to the frontend
type EventSender interface {
	SendEvent(source string, data interface{})
}

// ContainerHandler handles container endpoint requests
type ContainerHandler struct {
	runtime        runtime.ContainerRuntime
	logger         RequestLogger
	eventSender    EventSender // For progress and status events
	healthStatus   map[string]*models.HealthStatus
	containerStatus map[string]*models.ContainerStatus // Track container running state
	containerStats  map[string]*models.ContainerStats  // Track container resource usage
	healthMutex    sync.RWMutex
	statusMutex    sync.RWMutex // Mutex for container status map
	statsMutex     sync.RWMutex // Mutex for container stats map
	stopStatusPoll chan struct{} // Channel to signal status polling goroutine to stop
	stopStatsPoll  chan struct{} // Channel to signal stats polling goroutine to stop
}

// sanitizeContainerName converts endpoint name to valid container name
// Container names must match [a-zA-Z0-9][a-zA-Z0-9_.-]*
func sanitizeContainerName(endpointName string) string {
	// Convert to lowercase
	name := strings.ToLower(endpointName)

	// Replace invalid characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9_.-]+`)
	name = reg.ReplaceAllString(name, "-")

	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Add mockelot prefix
	return "mockelot-" + name
}

// NewContainerHandler creates a new container handler
func NewContainerHandler(logger RequestLogger, eventSender EventSender) *ContainerHandler {
	// Detect runtime instead of hardcoding Docker
	containerRuntime, err := runtime.DetectRuntime()
	if err != nil {
		log.Printf("Warning: Failed to detect container runtime: %v. Container endpoints will not be available.", err)
		return &ContainerHandler{
			logger:          logger,
			eventSender:     eventSender,
			healthStatus:    make(map[string]*models.HealthStatus),
			containerStatus: make(map[string]*models.ContainerStatus),
			containerStats:  make(map[string]*models.ContainerStats),
		}
	}

	log.Printf("Using container runtime: %s", containerRuntime.Name())

	return &ContainerHandler{
		runtime:         containerRuntime,
		logger:          logger,
		eventSender:     eventSender,
		healthStatus:    make(map[string]*models.HealthStatus),
		containerStatus: make(map[string]*models.ContainerStatus),
		containerStats:  make(map[string]*models.ContainerStats),
		stopStatusPoll:  make(chan struct{}),
		stopStatsPoll:   make(chan struct{}),
	}
}

// StartContainer pulls image, creates and starts a container
func (c *ContainerHandler) StartContainer(ctx context.Context, endpoint *models.Endpoint) error {
	if c.runtime == nil {
		return fmt.Errorf("container runtime not available")
	}

	cfg := endpoint.ContainerConfig
	if cfg == nil {
		return fmt.Errorf("container configuration missing")
	}

	// Generate container name from endpoint name
	containerName := sanitizeContainerName(endpoint.Name)
	log.Printf("Using container name: %s", containerName)

	var containerID string
	var cleanupNeeded bool

	// Cleanup on error or cancellation
	defer func() {
		if cleanupNeeded && containerID != "" {
			log.Printf("Cleaning up partial container: %s (%s)", containerName, containerID[:12])
			c.emitProgress(endpoint.ID, "error", "Cleaning up partial container...", 0)
			cleanupCtx := context.Background() // Use fresh context for cleanup
			c.runtime.StopContainer(cleanupCtx, containerID, 5)
			c.runtime.RemoveContainer(cleanupCtx, containerID, true)
			cfg.ContainerID = ""
		}
	}()

	// Check for existing container with same name and remove it
	existingID, err := c.runtime.FindContainerByName(context.Background(), containerName)
	if err == nil {
		log.Printf("Found existing container %s (%s), removing...", containerName, existingID[:12])
		c.runtime.StopContainer(context.Background(), existingID, 5)
		c.runtime.RemoveContainer(context.Background(), existingID, true)
	}

	// Emit start event
	c.emitProgress(endpoint.ID, "pulling", "Initializing container startup...", 0)

	// Pull image if requested
	if cfg.PullOnStartup {
		c.emitProgress(endpoint.ID, "pulling", "Pulling container image: "+cfg.ImageName, 10)
		log.Printf("Pulling container image: %s", cfg.ImageName)
		reader, err := c.runtime.PullImage(ctx, cfg.ImageName)
		if err != nil {
			c.emitProgress(endpoint.ID, "error", "Failed to pull image: "+err.Error(), 0)
			return fmt.Errorf("failed to pull image: %w", err)
		}

		// Stream pull progress
		if err := c.streamPullProgress(ctx, reader, endpoint.ID); err != nil {
			reader.Close()
			c.emitProgress(endpoint.ID, "error", "Pull failed: "+err.Error(), 0)
			return fmt.Errorf("failed to pull image: %w", err)
		}
		reader.Close()

		log.Printf("Image pulled successfully: %s", cfg.ImageName)
		c.emitProgress(endpoint.ID, "pulling", "Image pulled successfully", 40)
	}

	// Prepare environment variables
	c.emitProgress(endpoint.ID, "creating", "Preparing container configuration...", 50)
	env, err := c.prepareEnvironment(cfg.Environment)
	if err != nil {
		c.emitProgress(endpoint.ID, "error", "Failed to prepare environment: "+err.Error(), 0)
		return fmt.Errorf("failed to prepare environment: %w", err)
	}

	// Prepare volume mounts (with WSL path translation)
	mounts := c.prepareMounts(cfg.Volumes)

	// Create runtime-agnostic container config
	createConfig := &runtime.ContainerCreateConfig{
		Name:         containerName,
		Image:        cfg.ImageName,
		Env:          env,
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", cfg.ContainerPort)},
		PortBindings: map[string]string{
			fmt.Sprintf("%d/tcp", cfg.ContainerPort): "0", // Random host port
		},
		Mounts: mounts,
	}

	// Create container
	c.emitProgress(endpoint.ID, "creating", "Creating container...", 60)
	createdContainerID, err := c.runtime.CreateContainer(ctx, createConfig)
	if err != nil {
		c.emitProgress(endpoint.ID, "error", "Failed to create container: "+err.Error(), 0)
		return fmt.Errorf("failed to create container: %w", err)
	}

	containerID = createdContainerID
	cleanupNeeded = true // Enable cleanup for partial container
	cfg.ContainerID = containerID
	log.Printf("Container created: %s (image: %s, runtime: %s)", containerID[:12], cfg.ImageName, c.runtime.Name())

	// Start container
	c.emitProgress(endpoint.ID, "starting", "Starting container...", 75)
	if err := c.runtime.StartContainer(ctx, containerID); err != nil {
		c.emitProgress(endpoint.ID, "error", "Failed to start container: "+err.Error(), 0)
		return fmt.Errorf("failed to start container: %w", err)
	}

	log.Printf("Container started: %s", containerID[:12])
	c.emitProgress(endpoint.ID, "ready", "Container ready", 100)

	// Startup successful, disable cleanup
	cleanupNeeded = false

	// Start health checks
	if cfg.HealthCheckEnabled {
		go c.healthCheckLoop(endpoint)
	}

	return nil
}

// StopContainer stops and removes a container
func (c *ContainerHandler) StopContainer(ctx context.Context, endpoint *models.Endpoint) error {
	if c.runtime == nil {
		return nil
	}

	if endpoint.ContainerConfig == nil {
		return nil
	}

	var containerID string
	containerName := sanitizeContainerName(endpoint.Name)

	// Try to get container ID from config
	if endpoint.ContainerConfig.ContainerID != "" {
		containerID = endpoint.ContainerConfig.ContainerID
	} else {
		// Try to find by name
		foundID, err := c.runtime.FindContainerByName(ctx, containerName)
		if err != nil {
			// Container not found, nothing to stop
			return nil
		}
		containerID = foundID
	}

	log.Printf("Stopping container: %s (%s)", containerName, containerID[:12])

	timeout := 10
	if err := c.runtime.StopContainer(ctx, containerID, timeout); err != nil {
		log.Printf("Error stopping container: %v", err)
	}

	// Remove container
	if err := c.runtime.RemoveContainer(ctx, containerID, true); err != nil {
		log.Printf("Error removing container: %v", err)
		return err
	}

	log.Printf("Container removed: %s (%s)", containerName, containerID[:12])
	endpoint.ContainerConfig.ContainerID = ""
	return nil
}

// prepareEnvironment evaluates JS expressions and builds environment variable list
func (c *ContainerHandler) prepareEnvironment(envVars []models.EnvironmentVar) ([]string, error) {
	vm := goja.New()
	var result []string

	for _, envVar := range envVars {
		value := envVar.Value

		if envVar.Expression != "" {
			// Evaluate JS expression
			jsResult, err := vm.RunString(envVar.Expression)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate expression for %s: %w", envVar.Name, err)
			}
			value = jsResult.String()
		}

		result = append(result, fmt.Sprintf("%s=%s", envVar.Name, value))
	}

	return result, nil
}

// prepareMounts converts VolumeMapping to runtime mount specifications
func (c *ContainerHandler) prepareMounts(volumes []models.VolumeMapping) []runtime.Mount {
	var mounts []runtime.Mount

	for _, vol := range volumes {
		// Apply WSL path translation
		hostPath := runtime.TranslatePath(vol.HostPath)

		mounts = append(mounts, runtime.Mount{
			Source:   hostPath,
			Target:   vol.ContainerPath,
			ReadOnly: vol.ReadOnly,
		})
	}

	return mounts
}

// responseCapture wraps http.ResponseWriter to capture status code, headers, and body
type responseCapture struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func (rc *responseCapture) WriteHeader(statusCode int) {
	rc.statusCode = statusCode
	rc.ResponseWriter.WriteHeader(statusCode)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	rc.body = append(rc.body, b...)
	return rc.ResponseWriter.Write(b)
}

// ServeHTTP proxies requests to the running container
func (c *ContainerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, endpoint *models.Endpoint, translatedPath string) {
	if c.runtime == nil {
		http.Error(w, "Container runtime not available", http.StatusServiceUnavailable)
		return
	}

	cfg := endpoint.ContainerConfig
	if cfg == nil || cfg.ContainerID == "" {
		http.Error(w, "Container not running", http.StatusServiceUnavailable)
		return
	}

	// Get container info
	info, err := c.runtime.InspectContainer(context.Background(), cfg.ContainerID)
	if err != nil {
		http.Error(w, "Container inspection failed", http.StatusServiceUnavailable)
		c.logRequest(endpoint, r, 503, "Container inspection failed: "+err.Error(),
			nil, "", nil, nil, "")
		return
	}

	portKey := fmt.Sprintf("%d/tcp", cfg.ContainerPort)
	hostPort, ok := info.Ports[portKey]
	if !ok || hostPort == "" {
		http.Error(w, "Container port not bound", http.StatusServiceUnavailable)
		c.logRequest(endpoint, r, 503, "Container port not bound",
			nil, "", nil, nil, "")
		return
	}

	// Capture original request data for logging
	var requestBody string
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		requestBody = string(bodyBytes)
		// Create new body reader for proxy
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Capture original request headers
	requestHeaders := make(map[string][]string, len(r.Header))
	for name, values := range r.Header {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		requestHeaders[name] = valuesCopy
	}

	// Capture query parameters
	queryParams := make(map[string][]string)
	for key, values := range r.URL.Query() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		queryParams[key] = valuesCopy
	}

	// Build container URL
	containerURL := fmt.Sprintf("http://127.0.0.1:%s%s", hostPort, translatedPath)
	if r.URL.RawQuery != "" {
		containerURL += "?" + r.URL.RawQuery
	}

	backendURL, err := url.Parse(containerURL)
	if err != nil {
		http.Error(w, "Invalid container URL", http.StatusInternalServerError)
		return
	}

	// Create response capture wrapper
	capture := &responseCapture{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default to 200 if not explicitly set
		body:           []byte{},
	}

	// Proxy to container
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	proxy.ServeHTTP(capture, r)

	// Capture final response headers
	finalRespHeaders := make(map[string][]string, len(w.Header()))
	for name, values := range w.Header() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		finalRespHeaders[name] = valuesCopy
	}

	// Log request with response details
	c.logRequest(endpoint, r, capture.statusCode, "Proxied to container",
		requestHeaders, requestBody, queryParams,
		finalRespHeaders, string(capture.body))
}

// healthCheckLoop runs periodic health checks for a container endpoint
func (c *ContainerHandler) healthCheckLoop(endpoint *models.Endpoint) {
	cfg := endpoint.ContainerConfig
	interval := time.Duration(cfg.HealthCheckInterval) * time.Second
	if interval == 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		healthy, errMsg := c.performHealthCheck(endpoint)

		c.healthMutex.Lock()
		c.healthStatus[endpoint.ID] = &models.HealthStatus{
			EndpointID:   endpoint.ID,
			Healthy:      healthy,
			LastCheck:    time.Now().Format(time.RFC3339),
			ErrorMessage: errMsg,
		}
		c.healthMutex.Unlock()
	}
}

// performHealthCheck checks container state and optionally performs HTTP health check
func (c *ContainerHandler) performHealthCheck(endpoint *models.Endpoint) (bool, string) {
	if c.runtime == nil {
		return false, "Container runtime not available"
	}

	cfg := endpoint.ContainerConfig
	if cfg == nil || cfg.ContainerID == "" {
		return false, "Container not configured"
	}

	// Check container state
	info, err := c.runtime.InspectContainer(context.Background(), cfg.ContainerID)
	if err != nil {
		return false, err.Error()
	}

	if !info.Running {
		return false, fmt.Sprintf("Container not running (status: %s)", info.Status)
	}

	// HTTP health check if path specified
	if cfg.HealthCheckPath != "" {
		portKey := fmt.Sprintf("%d/tcp", cfg.ContainerPort)
		hostPort, ok := info.Ports[portKey]
		if !ok || hostPort == "" {
			return false, "Container port not bound"
		}

		healthURL := fmt.Sprintf("http://127.0.0.1:%s%s", hostPort, cfg.HealthCheckPath)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(healthURL)
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()

		// Accept status codes 200-499 (client errors are not backend down)
		healthy := resp.StatusCode >= 200 && resp.StatusCode < 500
		if !healthy {
			return false, fmt.Sprintf("Status code %d", resp.StatusCode)
		}
	}

	return true, ""
}

// GetHealthStatus returns the health status for an endpoint
func (c *ContainerHandler) GetHealthStatus(endpointID string) *models.HealthStatus {
	c.healthMutex.RLock()
	defer c.healthMutex.RUnlock()
	return c.healthStatus[endpointID]
}

// logRequest logs a container request
func (c *ContainerHandler) logRequest(endpoint *models.Endpoint, r *http.Request, statusCode int, message string,
	requestHeaders map[string][]string, requestBody string, queryParams map[string][]string,
	responseHeaders map[string][]string, responseBody string) {
	if c.logger != nil {
		log := models.RequestLog{
			ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
			Timestamp:  time.Now().Format(time.RFC3339),
			Method:     r.Method,
			Path:       r.URL.Path,
			StatusCode: statusCode,
			SourceIP:   r.RemoteAddr,
			EndpointID: endpoint.ID,
			Protocol:   r.Proto,
			UserAgent:  r.Header.Get("User-Agent"),
			// Original request data
			Headers:     requestHeaders,
			Body:        requestBody,
			QueryParams: queryParams,
			// Response data
			ResponseHeaders: responseHeaders,
			ResponseBody:    responseBody,
			// For container endpoints, backend response is same as final response (no transformation)
			BackendResponseBody: responseBody,
		}
		c.logger.LogRequest(log)
	}
}

// emitProgress emits a container startup progress event to the frontend
func (c *ContainerHandler) emitProgress(endpointID, stage, message string, progress int) {
	if c.eventSender == nil {
		log.Printf("WARNING: eventSender is nil, cannot emit progress event")
		return
	}

	event := models.ContainerStartProgress{
		EndpointID: endpointID,
		Stage:      stage,
		Message:    message,
		Progress:   progress,
	}

	log.Printf("Emitting container progress: endpoint=%s, stage=%s, progress=%d%%, message=%s", endpointID, stage, progress, message)
	c.eventSender.SendEvent("ctr:progress", event)
}

// streamPullProgress parses Docker/Podman pull progress and emits updates
func (c *ContainerHandler) streamPullProgress(ctx context.Context, reader io.ReadCloser, endpointID string) error {
	decoder := json.NewDecoder(reader)
	lastProgress := 10
	layerStatus := make(map[string]string)

	for {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var event map[string]interface{}
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			// Not all runtimes return valid JSON, just log and continue
			log.Printf("Pull progress parse warning: %v", err)
			continue
		}

		// Extract status and layer ID
		status, _ := event["status"].(string)
		id, _ := event["id"].(string)
		progress, _ := event["progress"].(string)

		// Track layer status
		if id != "" && status != "" {
			layerStatus[id] = status
		}

		// Emit progress updates for meaningful status changes
		if status != "" {
			message := status
			if id != "" && progress != "" {
				message = fmt.Sprintf("%s: %s %s", status, id, progress)
			} else if id != "" {
				message = fmt.Sprintf("%s: %s", status, id)
			}

			// Calculate overall progress (10-40% range for pulling)
			progressPercent := c.calculatePullProgress(layerStatus)
			if progressPercent > lastProgress {
				lastProgress = progressPercent
				c.emitProgress(endpointID, "pulling", message, progressPercent)
			}
		}
	}

	return nil
}

// calculatePullProgress estimates pull progress based on layer statuses
func (c *ContainerHandler) calculatePullProgress(layerStatus map[string]string) int {
	if len(layerStatus) == 0 {
		return 10
	}

	// Count completed layers
	completed := 0
	downloading := 0
	total := len(layerStatus)

	for _, status := range layerStatus {
		switch status {
		case "Pull complete", "Already exists":
			completed++
		case "Downloading", "Extracting":
			downloading++
		}
	}

	// Calculate percentage (10-40% range)
	ratio := float64(completed) / float64(total)
	progress := 10 + int(ratio*30)

	// Add partial credit for downloading
	if downloading > 0 && completed < total {
		progress += 5
	}

	if progress > 40 {
		progress = 40
	}

	return progress
}

// updateContainerStatus updates container status and emits event
func (c *ContainerHandler) updateContainerStatus(endpointID string, running bool, status string, gone bool) {
	c.statusMutex.Lock()
	c.containerStatus[endpointID] = &models.ContainerStatus{
		EndpointID: endpointID,
		Running:    running,
		Status:     status,
		Gone:       gone,
		LastCheck:  time.Now().Format(time.RFC3339),
	}
	c.statusMutex.Unlock()

	// Emit event to frontend
	if c.eventSender != nil {
		c.eventSender.SendEvent("ctr:status", c.containerStatus[endpointID])
	}
}

// GetContainerStatus returns the runtime status for an endpoint
func (c *ContainerHandler) GetContainerStatus(endpointID string) *models.ContainerStatus {
	c.statusMutex.RLock()
	defer c.statusMutex.RUnlock()
	return c.containerStatus[endpointID]
}

// StartContainerStatusPolling starts polling container status
// First 60 seconds: poll every 1 second (for startup monitoring)
// After 60 seconds: poll every 5 seconds (for steady-state monitoring)
func (c *ContainerHandler) StartContainerStatusPolling(endpoints []*models.Endpoint) {
	// Reinitialize stop channel to allow restart after stop
	c.stopStatusPoll = make(chan struct{})

	// Poll immediately on start
	c.pollAllContainerStatuses(endpoints)

	go func() {
		// Fast polling for first 60 seconds (1 second interval)
		fastTicker := time.NewTicker(1 * time.Second)
		defer fastTicker.Stop()
		fastTimer := time.NewTimer(60 * time.Second)
		defer fastTimer.Stop()

		fastPollingActive := true
		for fastPollingActive {
			select {
			case <-fastTicker.C:
				c.pollAllContainerStatuses(endpoints)
			case <-fastTimer.C:
				fastPollingActive = false
			case <-c.stopStatusPoll:
				log.Println("Container status polling stopped during fast polling phase")
				return
			}
		}

		// Slow polling after 60 seconds (5 second interval)
		slowTicker := time.NewTicker(5 * time.Second)
		defer slowTicker.Stop()
		for {
			select {
			case <-slowTicker.C:
				c.pollAllContainerStatuses(endpoints)
			case <-c.stopStatusPoll:
				log.Println("Container status polling stopped")
				return
			}
		}
	}()
}

// pollAllContainerStatuses polls status for all container endpoints
func (c *ContainerHandler) pollAllContainerStatuses(endpoints []*models.Endpoint) {
	for _, endpoint := range endpoints {
		if endpoint.Type == models.EndpointTypeContainer && endpoint.ContainerConfig != nil {
			c.pollContainerStatus(endpoint)
		}
	}
}

// pollContainerStatus checks and updates container status
func (c *ContainerHandler) pollContainerStatus(endpoint *models.Endpoint) {
	if c.runtime == nil {
		return
	}

	cfg := endpoint.ContainerConfig
	if cfg == nil || cfg.ContainerID == "" {
		// Container not started yet
		c.updateContainerStatus(endpoint.ID, false, "not started", false)
		return
	}

	// Inspect container to get current state
	info, err := c.runtime.InspectContainer(context.Background(), cfg.ContainerID)
	if err != nil {
		// Container doesn't exist (gone)
		c.updateContainerStatus(endpoint.ID, false, "gone", true)
		return
	}

	c.updateContainerStatus(endpoint.ID, info.Running, info.Status, false)
}

// StartContainerStatsPolling starts polling container stats every 5 seconds
func (c *ContainerHandler) StartContainerStatsPolling(endpoints []*models.Endpoint) {
	// Reinitialize stop channel to allow restart after stop
	c.stopStatsPoll = make(chan struct{})

	// Poll immediately on start
	c.pollAllContainerStats(endpoints)

	// Then poll every 5 seconds
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.pollAllContainerStats(endpoints)
			case <-c.stopStatsPoll:
				log.Println("Container stats polling stopped")
				return
			}
		}
	}()
}

// pollAllContainerStats polls stats for all container endpoints
func (c *ContainerHandler) pollAllContainerStats(endpoints []*models.Endpoint) {
	for _, endpoint := range endpoints {
		if endpoint.Type == models.EndpointTypeContainer && endpoint.ContainerConfig != nil {
			c.pollContainerStats(endpoint)
		}
	}
}

// pollContainerStats collects and updates container stats
func (c *ContainerHandler) pollContainerStats(endpoint *models.Endpoint) {
	if c.runtime == nil {
		return
	}

	cfg := endpoint.ContainerConfig
	if cfg == nil || cfg.ContainerID == "" {
		// No stats available for non-running containers
		return
	}

	// Get container stats from runtime
	stats, err := c.runtime.GetContainerStats(context.Background(), cfg.ContainerID)
	if err != nil {
		// Container might be stopped or removed, skip stats collection
		return
	}

	// Create stats record with endpoint ID
	endpointStats := &models.ContainerStats{
		EndpointID:      endpoint.ID,
		CPUPercent:      stats.CPUPercent,
		MemoryUsageMB:   stats.MemoryUsageMB,
		MemoryLimitMB:   stats.MemoryLimitMB,
		MemoryPercent:   stats.MemoryPercent,
		NetworkRxBytes:  stats.NetworkRxBytes,
		NetworkTxBytes:  stats.NetworkTxBytes,
		BlockReadBytes:  stats.BlockReadBytes,
		BlockWriteBytes: stats.BlockWriteBytes,
		PIDs:            stats.PIDs,
		LastCheck:       time.Now().Format(time.RFC3339),
	}

	c.statsMutex.Lock()
	c.containerStats[endpoint.ID] = endpointStats
	c.statsMutex.Unlock()

	// Emit event to frontend
	if c.eventSender != nil {
		c.eventSender.SendEvent("ctr:stats", endpointStats)
	}
}

// GetContainerStats returns the resource usage stats for an endpoint
func (c *ContainerHandler) GetContainerStats(endpointID string) *models.ContainerStats {
	c.statsMutex.RLock()
	defer c.statsMutex.RUnlock()
	return c.containerStats[endpointID]
}

// StopPolling stops all container polling goroutines
func (c *ContainerHandler) StopPolling() {
	// Close stop channels to signal goroutines to exit
	// This is safe to call multiple times - closing a closed channel panics,
	// but we'll recreate channels on next Start call
	if c.stopStatusPoll != nil {
		close(c.stopStatusPoll)
	}
	if c.stopStatsPoll != nil {
		close(c.stopStatsPoll)
	}
	log.Println("Container polling stop signal sent")
}
