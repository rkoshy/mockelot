package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
	proxyHandler   *ProxyHandler // For header manipulation
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
func NewContainerHandler(logger RequestLogger, eventSender EventSender, proxyHandler *ProxyHandler) *ContainerHandler {
	// Detect runtime instead of hardcoding Docker
	containerRuntime, err := runtime.DetectRuntime()
	if err != nil {
		log.Printf("Warning: Failed to detect container runtime: %v. Container endpoints will not be available.", err)
		return &ContainerHandler{
			logger:          logger,
			eventSender:     eventSender,
			proxyHandler:    proxyHandler,
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
		proxyHandler:    proxyHandler,
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

		c.emitProgress(endpoint.ID, "pulling", "Image pulled successfully", 40)
	}

	// Check for cancellation after image pull
	select {
	case <-ctx.Done():
		c.emitProgress(endpoint.ID, "error", "Startup cancelled by user", 0)
		return ctx.Err()
	default:
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

	// Check for cancellation after container creation
	select {
	case <-ctx.Done():
		c.emitProgress(endpoint.ID, "error", "Startup cancelled by user", 0)
		return ctx.Err()
	default:
	}

	// Start container
	c.emitProgress(endpoint.ID, "starting", "Starting container...", 75)
	if err := c.runtime.StartContainer(ctx, containerID); err != nil {
		c.emitProgress(endpoint.ID, "error", "Failed to start container: "+err.Error(), 0)
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Check for cancellation after container start
	select {
	case <-ctx.Done():
		c.emitProgress(endpoint.ID, "error", "Startup cancelled by user", 0)
		return ctx.Err()
	default:
	}

	c.emitProgress(endpoint.ID, "ready", "Container ready", 100)

	// Startup successful, disable cleanup
	cleanupNeeded = false

	// Start health checks
	if cfg.ProxyConfig.HealthCheckEnabled {
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

	timeout := 10
	if err := c.runtime.StopContainer(ctx, containerID, timeout); err != nil {
		log.Printf("Error stopping container: %v", err)
	}

	// Remove container
	if err := c.runtime.RemoveContainer(ctx, containerID, true); err != nil {
		log.Printf("Error removing container: %v", err)
		return err
	}

	endpoint.ContainerConfig.ContainerID = ""

	// Update status to "gone" so frontend UI updates immediately
	c.updateContainerStatus(endpoint.ID, containerID, false, "deleted", true)

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
		c.logErrorRequest(endpoint, r, 503, "Container inspection failed: "+err.Error())
		return
	}

	portKey := fmt.Sprintf("%d/tcp", cfg.ContainerPort)
	hostPort, ok := info.Ports[portKey]
	if !ok || hostPort == "" {
		http.Error(w, "Container port not bound", http.StatusServiceUnavailable)
		c.logErrorRequest(endpoint, r, 503, "Container port not bound")
		return
	}

	// Capture client request start time
	clientStartTime := time.Now()

	// Capture original request data for logging
	var requestBody string
	var bodyReader io.Reader
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		requestBody = string(bodyBytes)
		bodyReader = bytes.NewReader(bodyBytes)
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

	// Build client full URL (scheme://host:port/path?query)
	clientScheme := "http"
	if r.TLS != nil {
		clientScheme = "https"
	}
	clientFullURL := clientScheme + "://" + r.Host + r.URL.RequestURI()

	// Build container URL (backend URL)
	containerURL := fmt.Sprintf("http://127.0.0.1:%s%s", hostPort, translatedPath)
	if r.URL.RawQuery != "" {
		containerURL += "?" + r.URL.RawQuery
	}

	backendURL, err := url.Parse(containerURL)
	if err != nil {
		http.Error(w, "Invalid container URL", http.StatusInternalServerError)
		return
	}
	backendFullURL := containerURL

	// Build backend query params
	backendQueryParams := make(map[string][]string)
	for key, values := range backendURL.Query() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		backendQueryParams[key] = valuesCopy
	}

	// Generate request ID for tracking
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Log request immediately as pending (before waiting for response)
	c.logPendingRequest(requestID, endpoint, r, clientFullURL, requestHeaders, requestBody, queryParams)

	// Create backend request
	backendReq, err := http.NewRequest(r.Method, backendFullURL, bodyReader)
	if err != nil {
		http.Error(w, "Failed to create backend request", http.StatusInternalServerError)
		return
	}

	// Copy headers to backend request
	for name, values := range r.Header {
		for _, value := range values {
			backendReq.Header.Add(name, value)
		}
	}

	// Apply inbound header manipulation using shared ProxyHandler
	// This handles hop-by-hop header filtering, Host header setting, and X-Forwarded-* headers
	customContext := map[string]interface{}{
		"hostPort": hostPort,
	}
	c.proxyHandler.applyHeaderManipulationWithContext(backendReq.Header, cfg.ProxyConfig.InboundHeaders, r, customContext)

	// Capture backend request headers
	backendReqHeaders := make(map[string][]string, len(backendReq.Header))
	for name, values := range backendReq.Header {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		backendReqHeaders[name] = valuesCopy
	}

	// Execute backend request and measure timing
	// Note: Don't follow redirects - pass them through to the client
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects, return redirect response to client
		},
	}
	backendStartTime := time.Now()
	backendResp, err := client.Do(backendReq)
	backendFirstByteTime := time.Now() // Response headers received

	if err != nil {
		// Log detailed error information for debugging
		log.Printf("Container request failed for endpoint '%s' (ID: %s): %v",
			endpoint.Name, endpoint.ID, err)
		log.Printf("  Backend URL: %s", containerURL)
		log.Printf("  Container ID: %s", cfg.ContainerID[:12])

		// Log to transaction log so it appears in UI
		c.logErrorRequest(endpoint, r, 502, fmt.Sprintf("Container request failed: %v", err))

		http.Error(w, "Container request failed", http.StatusBadGateway)
		return
	}
	defer backendResp.Body.Close()

	// Read backend response body
	backendBodyBytes, err := io.ReadAll(backendResp.Body)
	if err != nil {
		log.Printf("Failed to read container response body for endpoint '%s': %v", endpoint.Name, err)
		c.logErrorRequest(endpoint, r, 502, fmt.Sprintf("Failed to read container response: %v", err))
		http.Error(w, "Failed to read container response", http.StatusBadGateway)
		return
	}
	backendCompletionTime := time.Now() // Full response received

	// Calculate backend timing metrics
	backendDelayMs := backendFirstByteTime.Sub(backendStartTime).Milliseconds()
	backendRTTMs := backendCompletionTime.Sub(backendStartTime).Milliseconds()

	// Capture backend response headers
	backendRespHeaders := make(map[string][]string, len(backendResp.Header))
	for name, values := range backendResp.Header {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		backendRespHeaders[name] = valuesCopy
	}

	backendStatusCode := backendResp.StatusCode
	backendStatusText := http.StatusText(backendResp.StatusCode)
	backendRespBody := string(backendBodyBytes)

	// Copy backend response headers to client response
	for name, values := range backendResp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Rewrite redirect Location headers to route back through our proxy
	if backendStatusCode >= 300 && backendStatusCode < 400 {
		if location := backendResp.Header.Get("Location"); location != "" {
			rewrittenLocation := c.rewriteRedirectLocation(location, containerURL, r.URL.Path, translatedPath, endpoint, r)
			if rewrittenLocation != location {
				w.Header().Set("Location", rewrittenLocation)
				log.Printf("Container redirect rewrite: %s -> %s", location, rewrittenLocation)
			}
		}
	}

	// Capture final response headers for logging
	finalRespHeaders := make(map[string][]string, len(w.Header()))
	for name, values := range w.Header() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		finalRespHeaders[name] = valuesCopy
	}

	// Capture time before sending first byte to client
	clientFirstByteTime := time.Now()

	// Write response to client
	w.WriteHeader(backendStatusCode)
	w.Write(backendBodyBytes)

	// Capture client completion time
	clientCompletionTime := time.Now()

	// Calculate client timing metrics
	clientDelayMs := clientFirstByteTime.Sub(clientStartTime).Milliseconds()
	clientRTTMs := clientCompletionTime.Sub(clientStartTime).Milliseconds()

	// Log request with full details (both client and backend sides)
	c.logRequest(requestID, endpoint, r,
		clientFullURL, requestHeaders, requestBody, queryParams,
		backendStatusCode, finalRespHeaders, backendRespBody, clientDelayMs, clientRTTMs,
		backendFullURL, translatedPath, backendQueryParams, backendReqHeaders,
		backendStatusCode, backendStatusText, backendRespHeaders, backendRespBody, backendDelayMs, backendRTTMs)
}

// rewriteRedirectLocation rewrites redirect Location headers to route back through our proxy
func (c *ContainerHandler) rewriteRedirectLocation(locationHeader, containerURL, originalPath, translatedPath string, endpoint *models.Endpoint, r *http.Request) string {
	// Parse the redirect location URL
	locationURL, err := url.Parse(locationHeader)
	if err != nil {
		// Can't parse, return as-is
		return locationHeader
	}

	// Parse the container URL
	backendURL, err := url.Parse(containerURL)
	if err != nil {
		// Can't parse backend URL, return location as-is
		return locationHeader
	}

	// Check if redirect is to the container (same scheme + host)
	// If it's an external redirect, don't rewrite
	if locationURL.Scheme != "" && locationURL.Host != "" {
		if locationURL.Scheme != backendURL.Scheme || locationURL.Host != backendURL.Host {
			// External redirect, leave as-is
			return locationHeader
		}
	}

	// Get the redirect path
	redirectPath := locationURL.Path

	// Strip backend base path if it exists
	if backendURL.Path != "" && backendURL.Path != "/" {
		if strings.HasPrefix(redirectPath, backendURL.Path) {
			redirectPath = strings.TrimPrefix(redirectPath, backendURL.Path)
			// Ensure it starts with /
			if !strings.HasPrefix(redirectPath, "/") {
				redirectPath = "/" + redirectPath
			}
		}
	}

	// Now reverse-translate the path
	var newPath string

	if strings.HasPrefix(redirectPath, translatedPath) {
		// Simple case: redirect path starts with what we sent to backend
		// Replace the translated prefix with the original prefix
		suffix := strings.TrimPrefix(redirectPath, translatedPath)
		newPath = originalPath + suffix
	} else {
		// Complex case: backend redirected to a different path
		switch endpoint.TranslationMode {
		case models.TranslationModeStrip:
			// We stripped the prefix, so prepend it back
			newPath = endpoint.PathPrefix + redirectPath
		case models.TranslationModeNone:
			// No translation, use as-is
			newPath = redirectPath
		default:
			// For regex/translate, we can't reverse-translate unknown paths
			// Best effort: if the redirect is relative, try to maintain it
			newPath = redirectPath
		}
	}

	// Build the new location URL
	// Preserve query string and fragment
	if locationURL.RawQuery != "" {
		newPath += "?" + locationURL.RawQuery
	}
	if locationURL.Fragment != "" {
		newPath += "#" + locationURL.Fragment
	}

	// If the original location was absolute, return absolute
	// Otherwise return relative
	if locationURL.Scheme != "" && locationURL.Host != "" {
		// Determine the scheme to use for the client redirect
		// Priority:
		// 1. If backend explicitly redirects to HTTPS, preserve that (security upgrade)
		// 2. If backend redirects to HTTP and client used HTTPS, preserve HTTPS
		// 3. Otherwise use the backend's redirect scheme
		scheme := locationURL.Scheme
		if r.TLS != nil && scheme == "http" {
			// Client used HTTPS, don't downgrade to HTTP
			scheme = "https"
		}
		// Note: If client used HTTP but backend redirects to HTTPS, we honor the upgrade
		return scheme + "://" + r.Host + newPath
	}

	// Return relative path
	return newPath
}

// healthCheckLoop runs periodic health checks for a container endpoint
func (c *ContainerHandler) healthCheckLoop(endpoint *models.Endpoint) {
	cfg := endpoint.ContainerConfig
	interval := time.Duration(cfg.ProxyConfig.HealthCheckInterval) * time.Second
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
	if cfg.ProxyConfig.HealthCheckPath != "" {
		portKey := fmt.Sprintf("%d/tcp", cfg.ContainerPort)
		hostPort, ok := info.Ports[portKey]
		if !ok || hostPort == "" {
			return false, "Container port not bound"
		}

		healthURL := fmt.Sprintf("http://127.0.0.1:%s%s", hostPort, cfg.ProxyConfig.HealthCheckPath)
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

// logRequest logs a container request with full backend details using new nested structure
// This updates the existing pending log entry with complete response data
func (c *ContainerHandler) logRequest(requestID string, endpoint *models.Endpoint, r *http.Request,
	clientFullURL string, clientReqHeaders map[string][]string, clientReqBody string, clientQueryParams map[string][]string,
	clientStatusCode int, clientRespHeaders map[string][]string, clientRespBody string, clientDelayMs int64, clientRTTMs int64,
	backendFullURL string, backendPath string, backendQueryParams map[string][]string, backendReqHeaders map[string][]string,
	backendStatusCode int, backendStatusText string, backendRespHeaders map[string][]string, backendRespBody string, backendDelayMs int64, backendRTTMs int64) {
	if c.logger != nil {
		// Create RequestLog with new nested structure
		requestLog := models.RequestLog{
			ID:         requestID,
			Timestamp:  time.Now().Format(time.RFC3339),
			EndpointID: endpoint.ID,
		}

		// Populate client request
		requestLog.ClientRequest.Method = r.Method
		requestLog.ClientRequest.FullURL = clientFullURL
		requestLog.ClientRequest.Path = r.URL.Path
		requestLog.ClientRequest.QueryParams = clientQueryParams
		requestLog.ClientRequest.Headers = clientReqHeaders
		requestLog.ClientRequest.Body = clientReqBody
		requestLog.ClientRequest.Protocol = r.Proto
		requestLog.ClientRequest.SourceIP = r.RemoteAddr
		requestLog.ClientRequest.UserAgent = r.Header.Get("User-Agent")

		// Populate client response
		requestLog.ClientResponse.StatusCode = &clientStatusCode
		requestLog.ClientResponse.StatusText = http.StatusText(clientStatusCode)
		requestLog.ClientResponse.Headers = clientRespHeaders
		requestLog.ClientResponse.Body = clientRespBody
		requestLog.ClientResponse.DelayMs = &clientDelayMs
		requestLog.ClientResponse.RTTMs = &clientRTTMs

		// Populate backend request (pointer struct)
		requestLog.BackendRequest = &struct {
			Method      string              `json:"method"`
			FullURL     string              `json:"full_url"`
			Path        string              `json:"path"`
			QueryParams map[string][]string `json:"query_params,omitempty"`
			Headers     map[string][]string `json:"headers,omitempty"`
			Body        string              `json:"body,omitempty"`
		}{
			Method:      r.Method,
			FullURL:     backendFullURL,
			Path:        backendPath,
			QueryParams: backendQueryParams,
			Headers:     backendReqHeaders,
			Body:        clientReqBody, // Same as client request body (proxied through)
		}

		// Populate backend response (pointer struct)
		requestLog.BackendResponse = &struct {
			StatusCode *int                `json:"status_code,omitempty"`
			StatusText string              `json:"status_text,omitempty"`
			Headers    map[string][]string `json:"headers,omitempty"`
			Body       string              `json:"body,omitempty"`
			DelayMs    *int64              `json:"delay_ms,omitempty"`
			RTTMs      *int64              `json:"rtt_ms,omitempty"`
		}{
			StatusCode: &backendStatusCode,
			StatusText: backendStatusText,
			Headers:    backendRespHeaders,
			Body:       backendRespBody,
			DelayMs:    &backendDelayMs,
			RTTMs:      &backendRTTMs,
		}

		c.logger.LogRequest(requestLog)
	}
}

// logErrorRequest logs a container request that failed before reaching the backend
func (c *ContainerHandler) logErrorRequest(endpoint *models.Endpoint, r *http.Request, statusCode int, errorMessage string) {
	if c.logger == nil {
		return
	}

	// Capture request body
	var requestBody string
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		requestBody = string(bodyBytes)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Restore body for http.Error
	}

	// Capture request headers
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

	// Build client full URL
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	clientFullURL := scheme + "://" + r.Host + r.URL.RequestURI()

	// Create RequestLog with error response
	requestLog := models.RequestLog{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp:  time.Now().Format(time.RFC3339),
		EndpointID: endpoint.ID,
	}

	// Populate client request
	requestLog.ClientRequest.Method = r.Method
	requestLog.ClientRequest.FullURL = clientFullURL
	requestLog.ClientRequest.Path = r.URL.Path
	requestLog.ClientRequest.QueryParams = queryParams
	requestLog.ClientRequest.Headers = requestHeaders
	requestLog.ClientRequest.Body = requestBody
	requestLog.ClientRequest.Protocol = r.Proto
	requestLog.ClientRequest.SourceIP = r.RemoteAddr
	requestLog.ClientRequest.UserAgent = r.Header.Get("User-Agent")

	// Populate client response with error
	requestLog.ClientResponse.StatusCode = &statusCode
	requestLog.ClientResponse.StatusText = http.StatusText(statusCode)
	requestLog.ClientResponse.Headers = make(map[string][]string)
	requestLog.ClientResponse.Body = errorMessage
	zero := int64(0)
	requestLog.ClientResponse.DelayMs = &zero
	requestLog.ClientResponse.RTTMs = &zero

	// Backend fields are nil (never reached backend)

	c.logger.UpdateRequestLog(requestLog)
}

// logPendingRequest logs a request immediately when received (before waiting for response)
func (c *ContainerHandler) logPendingRequest(requestID string, endpoint *models.Endpoint, r *http.Request,
	clientFullURL string, clientReqHeaders map[string][]string, clientReqBody string, clientQueryParams map[string][]string) {
	if c.logger != nil {
		// Create RequestLog with pending status
		requestLog := models.RequestLog{
			ID:         requestID,
			Timestamp:  time.Now().Format(time.RFC3339),
			EndpointID: endpoint.ID,
		}

		// Populate client request (we have this data immediately)
		requestLog.ClientRequest.Method = r.Method
		requestLog.ClientRequest.FullURL = clientFullURL
		requestLog.ClientRequest.Path = r.URL.Path
		requestLog.ClientRequest.QueryParams = clientQueryParams
		requestLog.ClientRequest.Headers = clientReqHeaders
		requestLog.ClientRequest.Body = clientReqBody
		requestLog.ClientRequest.Protocol = r.Proto
		requestLog.ClientRequest.SourceIP = r.RemoteAddr
		requestLog.ClientRequest.UserAgent = r.Header.Get("User-Agent")

		// Client response is empty (pending)
		requestLog.ClientResponse.StatusCode = nil
		requestLog.ClientResponse.StatusText = ""
		requestLog.ClientResponse.Headers = nil
		requestLog.ClientResponse.Body = ""
		requestLog.ClientResponse.DelayMs = nil
		requestLog.ClientResponse.RTTMs = nil

		// Backend data is nil (pending)
		requestLog.BackendRequest = nil
		requestLog.BackendResponse = nil

		c.logger.LogRequest(requestLog)
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
func (c *ContainerHandler) updateContainerStatus(endpointID string, containerID string, running bool, status string, gone bool) {
	c.statusMutex.Lock()
	c.containerStatus[endpointID] = &models.ContainerStatus{
		EndpointID:  endpointID,
		ContainerID: containerID,
		Running:     running,
		Status:      status,
		Gone:        gone,
		LastCheck:   time.Now().Format(time.RFC3339),
	}
	c.statusMutex.Unlock()

	// Emit event to frontend
	if c.eventSender != nil {
		c.eventSender.SendEvent("ctr:status", c.containerStatus[endpointID])
	} else {
		log.Printf("WARNING: eventSender is nil, cannot emit container status event for %s", endpointID)
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
		log.Println("WARNING: Container runtime is nil during status poll")
		return
	}

	cfg := endpoint.ContainerConfig
	if cfg == nil {
		c.updateContainerStatus(endpoint.ID, "", false, "not started", false)
		return
	}

	// If ContainerID is not set, try to find container by name (fallback for pre-existing containers)
	if cfg.ContainerID == "" {
		containerName := sanitizeContainerName(endpoint.Name)
		foundID, err := c.runtime.FindContainerByName(context.Background(), containerName)
		if err != nil {
			// Container doesn't exist by name either
			// Check if container was explicitly deleted (status already "gone")
			// Don't reset to "not started" if it was intentionally removed
			currentStatus := c.GetContainerStatus(endpoint.ID)
			if currentStatus != nil && currentStatus.Gone {
				return
			}

			// Set to "not started" only if not already gone
			c.updateContainerStatus(endpoint.ID, "", false, "not started", false)
			return
		}
		// Found the container! Store the ID for future polls
		cfg.ContainerID = foundID
	}

	// Inspect container to get current state
	info, err := c.runtime.InspectContainer(context.Background(), cfg.ContainerID)
	if err != nil {
		// Container doesn't exist (gone)
		c.updateContainerStatus(endpoint.ID, cfg.ContainerID, false, "gone", true)
		return
	}

	c.updateContainerStatus(endpoint.ID, cfg.ContainerID, info.Running, info.Status, false)
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

// GetContainerLogs retrieves container stdout/stderr logs
func (c *ContainerHandler) GetContainerLogs(ctx context.Context, endpointID string, tail int) (string, error) {
	if c.runtime == nil {
		return "", fmt.Errorf("container runtime not available")
	}

	// Get container status to find container ID
	c.statusMutex.RLock()
	status := c.containerStatus[endpointID]
	c.statusMutex.RUnlock()

	if status == nil || status.ContainerID == "" {
		return "", fmt.Errorf("container not found for endpoint %s", endpointID)
	}

	// Retrieve logs from runtime
	return c.runtime.GetContainerLogs(ctx, status.ContainerID, tail)
}

// StopPolling stops all container polling goroutines
func (c *ContainerHandler) StopPolling() {
	// Close stop channels to signal goroutines to exit
	// Safe to call multiple times - we set channels to nil after closing
	if c.stopStatusPoll != nil {
		close(c.stopStatusPoll)
		c.stopStatusPoll = nil
	}
	if c.stopStatsPoll != nil {
		close(c.stopStatsPoll)
		c.stopStatsPoll = nil
	}
}
