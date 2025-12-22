package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
	"mockelot/config"
	"mockelot/models"
	"mockelot/openapi"
	"mockelot/server"
	containerruntime "mockelot/server/runtime"
)

// ServerStatus represents the current state of the HTTP server
type ServerStatus struct {
	Running bool   `json:"running"`
	Port    int    `json:"port"`
	Error   string `json:"error,omitempty"`
}

// Event represents an event to be sent to the frontend
// Data MUST be a map to ensure proper JSON serialization by Wails
type Event struct {
	Source string                 `json:"source"` // Event source/type (e.g., "ctr:progress", "server:status")
	Data   map[string]interface{} `json:"data"`   // Event payload - MUST be a map for Wails serialization
}

// ScriptErrorLog represents a logged script execution error
type ScriptErrorLog struct {
	Timestamp  time.Time `json:"timestamp"`
	Error      string    `json:"error"`
	ResponseID string    `json:"response_id"`
	Path       string    `json:"path"`
	Method     string    `json:"method"`
}

// App struct
type App struct {
	ctx                    context.Context
	server                 *server.HTTPServer
	containerHandler       *server.ContainerHandler // Container handler for independent container operations
	proxyHandler           *server.ProxyHandler     // Proxy handler shared between HTTPServer and ContainerHandler
	config                 *models.AppConfig
	serverConfigMgr        *config.ServerConfigManager
	currentConfigPath      string                         // Path to the currently loaded/saved config file
	savedConfig            *models.AppConfig              // Last saved state for dirty tracking
	configMutex            sync.RWMutex                   // Protects config and savedConfig
	requestLogs            []models.RequestLog
	logMutex               sync.RWMutex
	requestLogSummaryQueue []models.RequestLogSummary // Queue of request log summaries for frontend polling
	requestLogQueueMutex   sync.Mutex                 // Mutex for thread-safe request log queue access
	status                 ServerStatus
	eventQueue             []Event    // Queue of events for frontend polling
	eventQueueMutex        sync.Mutex // Mutex for thread-safe event queue access
	containerStartContexts map[string]context.CancelFunc // Map of endpoint ID to cancel function for container startup
	containerStartMutex    sync.Mutex                    // Mutex for thread-safe access to containerStartContexts
	scriptErrors           map[string][]ScriptErrorLog   // Map of response ID to list of script errors
	scriptErrorsMutex      sync.RWMutex                  // Mutex for thread-safe access to scriptErrors
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{
		config: &models.AppConfig{
			Port: 8080,
			Responses: []models.MethodResponse{
				{
					ID:          uuid.New().String(),
					PathPattern: "/*",
					Methods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
					StatusCode:  200,
					StatusText:  "OK",
					Headers:     make(map[string]string),
					Body:        "",
				},
			},
		},
		serverConfigMgr:        config.NewServerConfigManager(""),
		requestLogs:            make([]models.RequestLog, 0),
		requestLogSummaryQueue: make([]models.RequestLogSummary, 0),
		status: ServerStatus{
			Running: false,
			Port:    8080,
		},
		eventQueue:             make([]Event, 0),                       // Event queue for frontend polling
		containerStartContexts: make(map[string]context.CancelFunc),
		scriptErrors:           make(map[string][]ScriptErrorLog), // Script error tracking
	}

	// Initialize proxy handler (shared between server and container handler)
	app.proxyHandler = server.NewProxyHandler(app)

	// Initialize container handler (independent of server)
	// App implements EventSender interface via SendEvent method
	app.containerHandler = server.NewContainerHandler(app, app, app.proxyHandler)

	// Ensure all endpoints have DisplayOrder set
	app.ensureDisplayOrder()

	// Ensure rejections endpoint exists
	app.ensureRejectionsEndpoint()

	return app
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Event polling architecture: Frontend polls PollEvents() periodically
	// No need for event sender goroutine
	log.Println("[App.startup] Using polling-based event delivery")

	// Load server configuration from old ~/.mockelot/server-config.yaml if it exists
	// This provides migration path for users upgrading from old version
	serverCfg, err := a.serverConfigMgr.Load()
	if err != nil {
		// Log error but continue with defaults
		fmt.Printf("Failed to load server config, using defaults: %v\n", err)
	} else {
		// Found old server-config.yaml, migrate to AppConfig
		log.Println("Migrating server settings from old server-config.yaml to AppConfig")
		log.Println("These settings will be marked as unsaved - please save to your main config file")

		// Apply server config to app config
		a.configMutex.Lock()
		a.config.Port = serverCfg.Port
		a.config.HTTP2Enabled = serverCfg.HTTP2Enabled
		a.config.HTTPSEnabled = serverCfg.HTTPSEnabled
		a.config.HTTPSPort = serverCfg.HTTPSPort
		a.config.HTTPToHTTPSRedirect = serverCfg.HTTPToHTTPSRedirect
		a.config.CertMode = serverCfg.CertMode
		a.config.CertPaths = serverCfg.CertPaths
		a.config.CertNames = serverCfg.CertNames
		a.config.CORS = serverCfg.CORS
		a.config.SOCKS5Config = serverCfg.SOCKS5Config
		a.config.DomainTakeover = serverCfg.DomainTakeover
		a.configMutex.Unlock()

		a.status.Port = serverCfg.Port
		// Note: SelectedEndpointId is loaded on-demand in GetSelectedEndpointId()

		// Mark as dirty to encourage user to save migrated settings
		// Don't set savedConfig - this keeps IsDirty() returning true
		runtime.EventsEmit(ctx, "config:dirty", true)
		runtime.EventsEmit(ctx, "config:migration-notice", "Server settings migrated from old server-config.yaml. Please save to preserve these settings.")
	}
}

// SendEvent queues an event for frontend polling
// This is non-blocking and thread-safe
// All data is converted to map[string]interface{} to ensure proper Wails serialization
func (a *App) SendEvent(source string, data interface{}) {
	// Convert all data to map[string]interface{} for Wails serialization
	var eventData map[string]interface{}

	switch v := data.(type) {
	case models.ContainerStartProgress:
		eventData = map[string]interface{}{
			"endpoint_id": v.EndpointID,
			"stage":       v.Stage,
			"message":     v.Message,
			"progress":    v.Progress,
		}

	case *models.ContainerStatus:
		eventData = map[string]interface{}{
			"endpoint_id": v.EndpointID,
			"running":     v.Running,
			"status":      v.Status,
			"gone":        v.Gone,
			"last_check":  v.LastCheck, // Already a string (RFC3339 format)
		}

	case *models.ContainerStats:
		eventData = map[string]interface{}{
			"endpoint_id":       v.EndpointID,
			"cpu_percent":       v.CPUPercent,
			"memory_usage_mb":   v.MemoryUsageMB,
			"memory_limit_mb":   v.MemoryLimitMB,
			"memory_percent":    v.MemoryPercent,
			"network_rx_bytes":  v.NetworkRxBytes,
			"network_tx_bytes":  v.NetworkTxBytes,
			"block_read_bytes":  v.BlockReadBytes,
			"block_write_bytes": v.BlockWriteBytes,
			"pids":              v.PIDs,
			"last_check":        v.LastCheck, // Already a string (RFC3339 format)
		}

	case ServerStatus:
		eventData = map[string]interface{}{
			"running": v.Running,
			"port":    v.Port,
			"error":   v.Error,
		}

	case map[string]interface{}:
		// Already a map, use as-is
		eventData = v

	default:
		// Unknown type - log warning and create empty map
		log.Printf("WARNING: Unknown event type %T for source %s", data, source)
		eventData = map[string]interface{}{
			"raw_value": fmt.Sprintf("%+v", data),
			"type":      fmt.Sprintf("%T", data),
		}
	}

	// Append to event queue (thread-safe)
	a.eventQueueMutex.Lock()
	a.eventQueue = append(a.eventQueue, Event{Source: source, Data: eventData})
	a.eventQueueMutex.Unlock()
}

// PollEvents returns all queued events and clears the queue
// This is called by the frontend at regular intervals (polling)
func (a *App) PollEvents() []Event {
	a.eventQueueMutex.Lock()
	defer a.eventQueueMutex.Unlock()

	// Get current events
	events := a.eventQueue

	// Clear the queue
	a.eventQueue = make([]Event, 0)

	return events
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.server != nil {
		a.server.Stop()
	}
}

// Emit implements the EventEmitter interface for Wails runtime events
func (a *App) Emit(eventName string, data interface{}) {
	runtime.EventsEmit(a.ctx, eventName, data)
}

// StartServer starts the HTTP mock server on the specified port
func (a *App) StartServer(port int) error {
	if a.server != nil && a.status.Running {
		return fmt.Errorf("server is already running")
	}

	// Check if port is changing from current config
	a.configMutex.Lock()
	originalPort := a.config.Port
	portChanged := (port != originalPort)

	// Update config with the port
	a.config.Port = port
	a.configMutex.Unlock()

	// If port changed, emit events to mark dirty
	if portChanged {
		runtime.EventsEmit(a.ctx, "config:port-changed", map[string]int{
			"http": port,
		})
		runtime.EventsEmit(a.ctx, "config:dirty", true)
	}

	a.server = server.NewHTTPServer(a.config, a, a, a, a.containerHandler, a.proxyHandler)

	err := a.server.Start()
	if err != nil {
		a.status = ServerStatus{Running: false, Port: port, Error: err.Error()}
		a.SendEvent("server:status", a.status)
		return err
	}

	a.status = ServerStatus{Running: true, Port: port}
	a.SendEvent("server:status", a.status)
	return nil
}

// StartContainers starts all container endpoints in the background
// Events are sent via the event channel to the frontend
func (a *App) StartContainers() error {
	if a.server == nil {
		return fmt.Errorf("server is not running")
	}

	log.Println("[StartContainers] Starting containers in background...")
	// Start containers in goroutine so this function returns immediately
	// Events will be sent via the event channel which is already listening
	go func() {
		if err := a.server.StartContainers(); err != nil {
			log.Printf("[StartContainers] Error starting containers: %v", err)
		}
	}()

	return nil
}

// StopServer stops the HTTP mock server
func (a *App) StopServer() error {
	if a.server == nil {
		return fmt.Errorf("server is not running")
	}

	err := a.server.Stop()
	if err != nil {
		a.status.Error = err.Error()
		a.SendEvent("server:status", a.status)
		return err
	}

	a.status = ServerStatus{Running: false, Port: a.status.Port}
	a.server = nil
	a.SendEvent("server:status", a.status)
	return nil
}

// GetServerStatus returns the current server status
func (a *App) GetServerStatus() ServerStatus {
	return a.status
}

// GetConfig returns the current configuration
func (a *App) GetConfig() *models.AppConfig {
	return a.config
}

// GetResponses returns all response rules (legacy - for backward compatibility)
func (a *App) GetResponses() []models.MethodResponse {
	return a.config.GetAllResponses()
}

// GetItems returns all response items (responses and groups)
func (a *App) GetItems() []models.ResponseItem {
	// Get items for the currently selected endpoint
	selectedId := a.GetSelectedEndpointId()
	if selectedId == "" {
		return []models.ResponseItem{}
	}

	// Find the selected endpoint
	for i := range a.config.Endpoints {
		if a.config.Endpoints[i].ID == selectedId {
			endpoint := &a.config.Endpoints[i]
			// Only return items for mock endpoints
			if endpoint.Type == models.EndpointTypeMock {
				return endpoint.Items
			}
			return []models.ResponseItem{}
		}
	}

	return []models.ResponseItem{}
}

// SetItems replaces all response items for the selected endpoint
func (a *App) SetItems(items []models.ResponseItem) error {
	// Get the selected endpoint ID
	selectedId := a.GetSelectedEndpointId()
	if selectedId == "" {
		return fmt.Errorf("no endpoint selected")
	}

	// Ensure all items have IDs
	for i := range items {
		if items[i].Type == "response" && items[i].Response != nil {
			if items[i].Response.ID == "" {
				items[i].Response.ID = uuid.New().String()
			}
		} else if items[i].Type == "group" && items[i].Group != nil {
			if items[i].Group.ID == "" {
				items[i].Group.ID = uuid.New().String()
			}
			for j := range items[i].Group.Responses {
				if items[i].Group.Responses[j].ID == "" {
					items[i].Group.Responses[j].ID = uuid.New().String()
				}
			}
		}
	}

	// Find the selected endpoint and update its items
	for i := range a.config.Endpoints {
		if a.config.Endpoints[i].ID == selectedId {
			endpoint := &a.config.Endpoints[i]
			// Only set items for mock endpoints
			if endpoint.Type == models.EndpointTypeMock {
				endpoint.Items = items
			} else {
				return fmt.Errorf("cannot set items for non-mock endpoint")
			}
			break
		}
	}

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "items:updated", items)

	return nil
}

// AddGroup adds a new group to the selected endpoint
func (a *App) AddGroup(name string) (models.ResponseGroup, error) {
	// Get the selected endpoint ID
	selectedId := a.GetSelectedEndpointId()
	if selectedId == "" {
		return models.ResponseGroup{}, fmt.Errorf("no endpoint selected")
	}

	group := models.ResponseGroup{
		ID:        uuid.New().String(),
		Name:      name,
		Responses: []models.MethodResponse{},
	}

	item := models.ResponseItem{
		Type:  "group",
		Group: &group,
	}

	// Find the selected endpoint and add the group to it
	found := false
	for i := range a.config.Endpoints {
		if a.config.Endpoints[i].ID == selectedId {
			endpoint := &a.config.Endpoints[i]
			// Only add groups to mock endpoints
			if endpoint.Type == models.EndpointTypeMock {
				endpoint.Items = append(endpoint.Items, item)
				found = true
			} else {
				return models.ResponseGroup{}, fmt.Errorf("cannot add group to non-mock endpoint")
			}
			break
		}
	}

	if !found {
		return models.ResponseGroup{}, fmt.Errorf("endpoint not found")
	}

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	return group, nil
}

// UpdateResponse updates a single response configuration (legacy - updates first response)
func (a *App) UpdateResponse(response models.MethodResponse) error {
	// Ensure ID is set
	if response.ID == "" {
		response.ID = uuid.New().String()
	}

	// Update the config
	a.config.Responses = []models.MethodResponse{response}

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	return nil
}

// SetResponses replaces all response rules with the provided list
func (a *App) SetResponses(responses []models.MethodResponse) error {
	// Ensure all responses have IDs
	for i := range responses {
		if responses[i].ID == "" {
			responses[i].ID = uuid.New().String()
		}
	}

	a.config.Responses = responses

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "responses:updated", responses)

	return nil
}

// AddResponse adds a new response rule
func (a *App) AddResponse(response models.MethodResponse) (models.MethodResponse, error) {
	// Generate ID if not provided
	if response.ID == "" {
		response.ID = uuid.New().String()
	}

	a.config.Responses = append(a.config.Responses, response)

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	return response, nil
}

// UpdateResponseByID updates a specific response rule by ID
func (a *App) UpdateResponseByID(response models.MethodResponse) error {
	for i, r := range a.config.Responses {
		if r.ID == response.ID {
			a.config.Responses[i] = response
			break
		}
	}

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	return nil
}

// DeleteResponse removes a response rule by ID
func (a *App) DeleteResponse(id string) error {
	for i, r := range a.config.Responses {
		if r.ID == id {
			a.config.Responses = append(a.config.Responses[:i], a.config.Responses[i+1:]...)
			break
		}
	}

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	return nil
}

// ReorderResponses reorders response rules based on the provided ID order
func (a *App) ReorderResponses(ids []string) error {
	// Create a map for quick lookup
	responseMap := make(map[string]models.MethodResponse)
	for _, r := range a.config.Responses {
		responseMap[r.ID] = r
	}

	// Reorder based on provided IDs
	newResponses := make([]models.MethodResponse, 0, len(ids))
	for _, id := range ids {
		if r, ok := responseMap[id]; ok {
			newResponses = append(newResponses, r)
		}
	}

	a.config.Responses = newResponses

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	return nil
}

// Endpoint Management Methods

// GetEndpoints returns all endpoints
func (a *App) GetEndpoints() []models.Endpoint {
	return a.config.Endpoints
}

// GetDefaultContainerHeaders returns the default inbound headers for container endpoints
func (a *App) GetDefaultContainerHeaders() []models.HeaderManipulation {
	return models.DefaultContainerInboundHeaders()
}

// AddEndpoint adds a new endpoint with specified type
func (a *App) AddEndpoint(name string, pathPrefix string, translationMode string, endpointType string) (models.Endpoint, error) {
	log.Printf("AddEndpoint called with: name=%s, pathPrefix=%s, translationMode=%s, endpointType=%s", name, pathPrefix, translationMode, endpointType)

	// Validate translation mode
	if translationMode != models.TranslationModeNone &&
		translationMode != models.TranslationModeStrip &&
		translationMode != models.TranslationModeTranslate {
		log.Printf("Invalid translation mode '%s', defaulting to 'none'", translationMode)
		translationMode = models.TranslationModeNone // Default to none if invalid
	}

	// Validate endpoint type
	if endpointType != models.EndpointTypeMock &&
		endpointType != models.EndpointTypeProxy &&
		endpointType != models.EndpointTypeContainer {
		log.Printf("Invalid endpoint type '%s', defaulting to 'mock'. Valid types: %s, %s, %s",
			endpointType, models.EndpointTypeMock, models.EndpointTypeProxy, models.EndpointTypeContainer)
		endpointType = models.EndpointTypeMock // Default to mock if invalid
	}

	endpoint := models.Endpoint{
		ID:              uuid.New().String(),
		Name:            name,
		PathPrefix:      pathPrefix,
		TranslationMode: translationMode,
		Type:            endpointType,
	}

	// Initialize type-specific configuration
	switch endpointType {
	case models.EndpointTypeMock:
		endpoint.Items = []models.ResponseItem{}
	case models.EndpointTypeProxy:
		// Initialize with basic proxy config
		endpoint.ProxyConfig = &models.ProxyConfig{
			BackendURL:        "",
			TimeoutSeconds:    30,
			StatusPassthrough: true,
		}
	case models.EndpointTypeContainer:
		// Initialize with basic container config
		endpoint.ContainerConfig = &models.ContainerConfig{
			ProxyConfig: models.ProxyConfig{
				TimeoutSeconds:    30,
				StatusPassthrough: true,
				InboundHeaders:    models.DefaultContainerInboundHeaders(), // Apply default header rules
			},
			ImageName:     "",
			ContainerPort: 80,
			PullOnStartup: true,
			Volumes:       []models.VolumeMapping{},
			Environment:   []models.EnvironmentVar{},
		}
	}

	// Insert endpoint before system endpoints (like Rejections)
	// Find the index of the first system endpoint
	insertIndex := len(a.config.Endpoints)
	for i, ep := range a.config.Endpoints {
		if ep.IsSystem {
			insertIndex = i
			break
		}
	}

	// Insert at the found index
	if insertIndex < len(a.config.Endpoints) {
		// Insert before system endpoints
		a.config.Endpoints = append(a.config.Endpoints[:insertIndex], append([]models.Endpoint{endpoint}, a.config.Endpoints[insertIndex:]...)...)
	} else {
		// No system endpoints, append at end
		a.config.Endpoints = append(a.config.Endpoints, endpoint)
	}

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "endpoints:updated", a.config.Endpoints)

	return endpoint, nil
}

// AddEndpointWithConfig adds a new endpoint with full configuration from wizard
func (a *App) AddEndpointWithConfig(config map[string]interface{}) (models.Endpoint, error) {

	// Extract basic fields
	name, _ := config["name"].(string)
	pathPrefix, _ := config["path_prefix"].(string)
	translationMode, _ := config["translation_mode"].(string)
	endpointType, _ := config["type"].(string)

	// Validate translation mode
	if translationMode != models.TranslationModeNone &&
		translationMode != models.TranslationModeStrip &&
		translationMode != models.TranslationModeTranslate {
		log.Printf("Invalid translation mode '%s', defaulting to 'none'", translationMode)
		translationMode = models.TranslationModeNone
	}

	// Validate endpoint type
	if endpointType != models.EndpointTypeMock &&
		endpointType != models.EndpointTypeProxy &&
		endpointType != models.EndpointTypeContainer {
		log.Printf("Invalid endpoint type '%s', defaulting to 'mock'", endpointType)
		endpointType = models.EndpointTypeMock
	}

	endpoint := models.Endpoint{
		ID:              uuid.New().String(),
		Name:            name,
		PathPrefix:      pathPrefix,
		TranslationMode: translationMode,
		Type:            endpointType,
	}

	// Initialize type-specific configuration from wizard data
	switch endpointType {
	case models.EndpointTypeMock:
		endpoint.Items = []models.ResponseItem{}

	case models.EndpointTypeProxy:
		proxyConfig, _ := config["proxy_config"].(map[string]interface{})
		if proxyConfig != nil {
			endpoint.ProxyConfig = &models.ProxyConfig{
				BackendURL:        getString(proxyConfig, "backend_url"),
				TimeoutSeconds:    getInt(proxyConfig, "timeout_seconds", 30),
				StatusPassthrough: getBool(proxyConfig, "status_passthrough", true),
			}

			// Parse status translations
			if statusTranslations, ok := proxyConfig["status_translation"].([]interface{}); ok {
				endpoint.ProxyConfig.StatusTranslation = parseStatusTranslations(statusTranslations)
			}

			// Parse inbound headers
			if inboundHeaders, ok := proxyConfig["inbound_headers"].([]interface{}); ok {
				endpoint.ProxyConfig.InboundHeaders = parseHeaderManipulations(inboundHeaders)
			}

			// Parse outbound headers
			if outboundHeaders, ok := proxyConfig["outbound_headers"].([]interface{}); ok {
				endpoint.ProxyConfig.OutboundHeaders = parseHeaderManipulations(outboundHeaders)
			}
		} else {
			// Initialize with defaults if no config provided
			endpoint.ProxyConfig = &models.ProxyConfig{
				BackendURL:        "",
				TimeoutSeconds:    30,
				StatusPassthrough: true,
			}
		}

	case models.EndpointTypeContainer:
		containerConfig, _ := config["container_config"].(map[string]interface{})
		if containerConfig != nil {
			endpoint.ContainerConfig = &models.ContainerConfig{
				ProxyConfig: models.ProxyConfig{
					TimeoutSeconds:      30,
					StatusPassthrough:   true,
					InboundHeaders:      models.DefaultContainerInboundHeaders(), // Apply default header rules
					HealthCheckEnabled:  getBool(containerConfig, "health_check_enabled", false),
					HealthCheckInterval: getInt(containerConfig, "health_check_interval", 30),
					HealthCheckPath:     getString(containerConfig, "health_check_path"),
				},
				ImageName:            getString(containerConfig, "image_name"),
				ContainerPort:        getInt(containerConfig, "container_port", 80),
				PullOnStartup:        getBool(containerConfig, "pull_on_startup", true),
				RestartOnServerStart: getBool(containerConfig, "restart_on_server_start", false),
				RestartPolicy:        getString(containerConfig, "restart_policy"),
				HostNetworking:       getBool(containerConfig, "host_networking", false),
				DockerSocketAccess:   getBool(containerConfig, "docker_socket_access", false),
			}

			// Parse inbound headers (if custom headers provided, they override defaults)
			if proxyConfig, ok := containerConfig["proxy_config"].(map[string]interface{}); ok {
				if inboundHeaders, ok := proxyConfig["inbound_headers"].([]interface{}); ok {
					endpoint.ContainerConfig.ProxyConfig.InboundHeaders = parseHeaderManipulations(inboundHeaders)
				}
				// Parse outbound headers
				if outboundHeaders, ok := proxyConfig["outbound_headers"].([]interface{}); ok {
					endpoint.ContainerConfig.ProxyConfig.OutboundHeaders = parseHeaderManipulations(outboundHeaders)
				}
			}

			// Parse volumes
			if volumes, ok := containerConfig["volumes"].([]interface{}); ok {
				endpoint.ContainerConfig.Volumes = parseVolumes(volumes)
			} else {
				endpoint.ContainerConfig.Volumes = []models.VolumeMapping{}
			}

			// Parse environment variables
			if environment, ok := containerConfig["environment"].([]interface{}); ok {
				endpoint.ContainerConfig.Environment = parseEnvironmentVars(environment)
			} else {
				endpoint.ContainerConfig.Environment = []models.EnvironmentVar{}
			}
		} else {
			// Initialize with defaults if no config provided
			endpoint.ContainerConfig = &models.ContainerConfig{
				ProxyConfig: models.ProxyConfig{
					TimeoutSeconds:    30,
					StatusPassthrough: true,
					InboundHeaders:    models.DefaultContainerInboundHeaders(), // Apply default header rules
				},
				ImageName:     "",
				ContainerPort: 80,
				PullOnStartup: true,
				Volumes:       []models.VolumeMapping{},
				Environment:   []models.EnvironmentVar{},
			}
		}
	}

	// Insert endpoint before system endpoints (like Rejections)
	// Find the index of the first system endpoint
	insertIndex := len(a.config.Endpoints)
	for i, ep := range a.config.Endpoints {
		if ep.IsSystem {
			insertIndex = i
			break
		}
	}

	// Insert at the found index
	if insertIndex < len(a.config.Endpoints) {
		// Insert before system endpoints
		a.config.Endpoints = append(a.config.Endpoints[:insertIndex], append([]models.Endpoint{endpoint}, a.config.Endpoints[insertIndex:]...)...)
	} else {
		// No system endpoints, append at end
		a.config.Endpoints = append(a.config.Endpoints, endpoint)
	}

	log.Printf("Created endpoint with full config: ID=%s, Name=%s, Type=%s", endpoint.ID, endpoint.Name, endpoint.Type)

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "endpoints:updated", a.config.Endpoints)

	return endpoint, nil
}

// Helper functions for parsing config data
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getInt(m map[string]interface{}, key string, defaultVal int) int {
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return defaultVal
}

func getBool(m map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return defaultVal
}

func parseStatusTranslations(data []interface{}) []models.StatusTranslation {
	result := []models.StatusTranslation{}
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, models.StatusTranslation{
				FromPattern: getString(m, "from_pattern"),
				ToCode:      getInt(m, "to_code", 0),
			})
		}
	}
	return result
}

func parseHeaderManipulations(data []interface{}) []models.HeaderManipulation {
	result := []models.HeaderManipulation{}
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, models.HeaderManipulation{
				Name:       getString(m, "name"),
				Mode:       getString(m, "mode"),
				Value:      getString(m, "value"),
				Expression: getString(m, "expression"),
			})
		}
	}
	return result
}

func parseVolumes(data []interface{}) []models.VolumeMapping {
	result := []models.VolumeMapping{}
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, models.VolumeMapping{
				HostPath:      getString(m, "host_path"),
				ContainerPath: getString(m, "container_path"),
				ReadOnly:      getBool(m, "read_only", false),
			})
		}
	}
	return result
}

func parseEnvironmentVars(data []interface{}) []models.EnvironmentVar {
	result := []models.EnvironmentVar{}
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, models.EnvironmentVar{
				Name:       getString(m, "name"),
				Value:      getString(m, "value"),
				Expression: getString(m, "expression"),
			})
		}
	}
	return result
}

// ensureRejectionsEndpoint creates the system "Rejections" endpoint if it doesn't exist
// This endpoint catches all requests that don't match any other endpoint
func (a *App) ensureRejectionsEndpoint() {
	const rejectionsID = "system-rejections"

	// Check if rejections endpoint already exists
	for i := range a.config.Endpoints {
		if a.config.Endpoints[i].ID == rejectionsID {
			// Endpoint exists - ensure it has correct system properties
			a.config.Endpoints[i].IsSystem = true
			a.config.Endpoints[i].DisplayOrder = 999999 // Always last
			return
		}
	}

	// Create rejections endpoint
	enabled := true
	rejectionsEndpoint := models.Endpoint{
		ID:           rejectionsID,
		Name:         "Rejections",
		PathPrefix:   "/",
		TranslationMode: models.TranslationModeNone,
		Enabled:      &enabled,
		IsSystem:     true,
		DisplayOrder: 999999, // Always last in matching order
		Type:         models.EndpointTypeMock,
		Items: []models.ResponseItem{
			{
				Type: "response",
				Response: &models.MethodResponse{
					ID:          "reject-request",
					Enabled:     &enabled,
					PathPattern: "/*",
					Methods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
					StatusCode:  404,
					StatusText:  "Not Found",
					Headers: map[string]string{
						"Content-Type": "text/plain",
					},
					Body: "No matching endpoint found",
				},
			},
		},
	}

	// Add to endpoints list
	a.config.Endpoints = append(a.config.Endpoints, rejectionsEndpoint)
}

// ensureDisplayOrder ensures all endpoints have DisplayOrder set
// Legacy configs may not have this field, so we set it based on array index
func (a *App) ensureDisplayOrder() {
	for i := range a.config.Endpoints {
		if a.config.Endpoints[i].DisplayOrder == 0 && !a.config.Endpoints[i].IsSystem {
			a.config.Endpoints[i].DisplayOrder = i
		}
	}
}

// UpdateEndpoint updates an existing endpoint
func (a *App) UpdateEndpoint(endpoint models.Endpoint) error {
	for i := range a.config.Endpoints {
		if a.config.Endpoints[i].ID == endpoint.ID {
			// Preserve Items array (not sent from settings dialog)
			existingItems := a.config.Endpoints[i].Items

			// Preserve runtime state for containers
			var existingContainerID string
			if a.config.Endpoints[i].ContainerConfig != nil {
				existingContainerID = a.config.Endpoints[i].ContainerConfig.ContainerID
			}

			// Update endpoint
			a.config.Endpoints[i] = endpoint

			// Restore preserved data
			a.config.Endpoints[i].Items = existingItems
			if a.config.Endpoints[i].ContainerConfig != nil && existingContainerID != "" {
				a.config.Endpoints[i].ContainerConfig.ContainerID = existingContainerID
			}

			break
		}
	}

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "endpoints:updated", a.config.Endpoints)

	return nil
}

// DeleteEndpoint removes an endpoint by ID
func (a *App) DeleteEndpoint(id string) error {
	for i, endpoint := range a.config.Endpoints {
		if endpoint.ID == id {
			// Prevent deletion of system endpoints
			if endpoint.IsSystem {
				return fmt.Errorf("cannot delete system endpoint")
			}
			a.config.Endpoints = append(a.config.Endpoints[:i], a.config.Endpoints[i+1:]...)
			break
		}
	}

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "endpoints:updated", a.config.Endpoints)

	return nil
}

// GetEndpointHealth returns health status for an endpoint
func (a *App) GetEndpointHealth(endpointID string) (*models.HealthStatus, error) {
	if a.server == nil {
		return nil, fmt.Errorf("server not running")
	}

	// Find endpoint
	var endpoint *models.Endpoint
	for i := range a.config.Endpoints {
		if a.config.Endpoints[i].ID == endpointID {
			endpoint = &a.config.Endpoints[i]
			break
		}
	}

	if endpoint == nil {
		return nil, fmt.Errorf("endpoint not found")
	}

	switch endpoint.Type {
	case models.EndpointTypeProxy:
		status := a.server.GetProxyHealthStatus(endpointID)
		if status == nil {
			return &models.HealthStatus{EndpointID: endpointID, Healthy: false}, nil
		}
		return status, nil
	case models.EndpointTypeContainer:
		status := a.server.GetContainerHealthStatus(endpointID)
		if status == nil {
			return &models.HealthStatus{EndpointID: endpointID, Healthy: false}, nil
		}
		return status, nil
	default:
		// Mock endpoints are always healthy
		return &models.HealthStatus{EndpointID: endpointID, Healthy: true}, nil
	}
}

// TestProxyConnection tests connectivity to a proxy backend
func (a *App) TestProxyConnection(backendURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(backendURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ValidateDockerImage checks if a Docker image is available
func (a *App) ValidateDockerImage(imageName string) error {
	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("Docker not available: %w", err)
	}
	defer dockerClient.Close()

	// Try to inspect the image
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _, err = dockerClient.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return fmt.Errorf("image not found locally: %s", imageName)
	}

	return nil
}

// PullDockerImage pulls a Docker image from the registry
func (a *App) PullDockerImage(imageName string) error {
	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("Docker not available: %w", err)
	}
	defer dockerClient.Close()

	// Pull the image
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // 5 minute timeout for pull
	defer cancel()

	reader, err := dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Wait for pull to complete by reading to the end
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("error during image pull: %w", err)
	}

	return nil
}

// ValidateAndInspectDockerImage inspects a Docker image and returns metadata
func (a *App) ValidateAndInspectDockerImage(imageName string) (*models.DockerImageInfo, error) {
	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("Docker not available: %w", err)
	}
	defer dockerClient.Close()

	// Inspect the image
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	inspectData, _, err := dockerClient.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return nil, fmt.Errorf("image not found locally: %s", imageName)
	}

	info := &models.DockerImageInfo{
		ImageName:   imageName,
		Environment: make(map[string]string),
		Labels:      make(map[string]string),
	}

	// Extract exposed ports
	if inspectData.Config != nil && inspectData.Config.ExposedPorts != nil {
		for port := range inspectData.Config.ExposedPorts {
			info.ExposedPorts = append(info.ExposedPorts, string(port))
		}
	}

	// Extract volumes
	if inspectData.Config != nil && inspectData.Config.Volumes != nil {
		for vol := range inspectData.Config.Volumes {
			info.Volumes = append(info.Volumes, vol)
		}
	}

	// Extract environment variables
	if inspectData.Config != nil && inspectData.Config.Env != nil {
		for _, envVar := range inspectData.Config.Env {
			parts := strings.SplitN(envVar, "=", 2)
			if len(parts) == 2 {
				info.Environment[parts[0]] = parts[1]
			}
		}
	}

	// Extract working directory
	if inspectData.Config != nil {
		info.WorkingDir = inspectData.Config.WorkingDir
	}

	// Extract entrypoint
	if inspectData.Config != nil && inspectData.Config.Entrypoint != nil {
		info.Entrypoint = inspectData.Config.Entrypoint
	}

	// Extract command
	if inspectData.Config != nil && inspectData.Config.Cmd != nil {
		info.Cmd = inspectData.Config.Cmd
	}

	// Extract labels
	if inspectData.Config != nil && inspectData.Config.Labels != nil {
		info.Labels = inspectData.Config.Labels
	}

	// Detect if this is an HTTP service and suggest health check path
	info.IsHTTPService = isHTTPService(imageName, info.ExposedPorts, info.Labels)
	info.SuggestedHealthCheckPath = detectHealthCheckPath(imageName, info.Labels, info.IsHTTPService)

	return info, nil
}

// isHTTPService determines if an image is likely an HTTP service
func isHTTPService(imageName string, exposedPorts []string, labels map[string]string) bool {
	// Check for common HTTP/HTTPS ports
	for _, port := range exposedPorts {
		portNum := strings.Split(port, "/")[0]
		switch portNum {
		case "80", "443", "8080", "8443", "3000", "5000", "8000", "9000":
			return true
		}
	}

	// Check labels for HTTP service indicators
	if labels != nil {
		if val, ok := labels["service.type"]; ok && strings.Contains(strings.ToLower(val), "http") {
			return true
		}
		if val, ok := labels["app.type"]; ok && strings.Contains(strings.ToLower(val), "web") {
			return true
		}
	}

	// Parse image name for service type detection
	imageLower := strings.ToLower(imageName)

	// Non-HTTP services (databases, message queues, caches)
	nonHTTPServices := []string{
		"postgres", "postgresql", "mysql", "mariadb", "mongodb", "mongo",
		"redis", "memcached", "cassandra", "elasticsearch", "rabbitmq",
		"kafka", "zookeeper", "etcd", "consul",
	}
	for _, service := range nonHTTPServices {
		if strings.Contains(imageLower, service) {
			return false
		}
	}

	// HTTP services patterns
	httpServicePatterns := []string{
		"nginx", "apache", "httpd", "tomcat", "jetty",
		"api", "web", "frontend", "backend", "service",
		"app", "server", "proxy", "gateway",
	}
	for _, pattern := range httpServicePatterns {
		if strings.Contains(imageLower, pattern) {
			return true
		}
	}

	// Default to true if we can't determine (safer for health checks)
	return true
}

// detectHealthCheckPath suggests a health check path based on image metadata
func detectHealthCheckPath(imageName string, labels map[string]string, isHTTPService bool) string {
	// If not an HTTP service, return empty (user should disable health checks)
	if !isHTTPService {
		return ""
	}

	// 1. Check labels for explicit health check configuration
	labelKeys := []string{
		"healthcheck.path",
		"health-check-path",
		"health.check.path",
		"app.healthcheck.path",
		"service.healthcheck.path",
	}
	for _, key := range labelKeys {
		if path, ok := labels[key]; ok && path != "" {
			return path
		}
	}

	// 2. Advanced pattern matching on image name
	// Extract service name from registry path: registry.company.com/team/service-api:v1.2.3 -> service-api
	imageLower := strings.ToLower(imageName)

	// Remove registry prefix (everything before first /)
	if idx := strings.Index(imageLower, "/"); idx != -1 {
		imageLower = imageLower[idx+1:]
	}

	// Remove tag (everything after :)
	if idx := strings.Index(imageLower, ":"); idx != -1 {
		imageLower = imageLower[:idx]
	}

	// Remove namespace/team prefix (everything before last /)
	if idx := strings.LastIndex(imageLower, "/"); idx != -1 {
		imageLower = imageLower[idx+1:]
	}

	// Service-specific patterns with regex matching
	type healthCheckPattern struct {
		pattern     string
		healthPath  string
	}

	patterns := []healthCheckPattern{
		// Framework-specific
		{`spring|springboot`, "/actuator/health"},
		{`express|nodejs|node`, "/health"},
		{`flask|django|fastapi`, "/health"},
		{`rails|ruby`, "/health"},
		{`gin|golang|go-`, "/health"},
		{`dotnet|aspnet`, "/health"},

		// Service-specific
		{`nginx|apache|httpd`, "/"},
		{`grafana`, "/api/health"},
		{`prometheus`, "/-/healthy"},
		{`kibana`, "/api/status"},
		{`jenkins`, "/login"},
		{`sonarqube`, "/api/system/status"},
		{`nexus`, "/service/rest/v1/status"},
		{`artifactory`, "/artifactory/api/system/ping"},

		// Generic API patterns
		{`api`, "/api/health"},
		{`service`, "/health"},
		{`web|app`, "/health"},
	}

	for _, p := range patterns {
		matched, _ := regexp.MatchString(p.pattern, imageLower)
		if matched {
			return p.healthPath
		}
	}

	// 3. Default fallback for HTTP services
	return "/health"
}

// GetContainerStatus returns the runtime status for a container endpoint
func (a *App) GetContainerStatus(endpointID string) (*models.ContainerStatus, error) {
	status := a.containerHandler.GetContainerStatus(endpointID)
	if status == nil {
		return &models.ContainerStatus{
			EndpointID: endpointID,
			Running:    false,
			Status:     "not started",
			LastCheck:  time.Now().Format(time.RFC3339),
		}, nil
	}

	return status, nil
}

// GetContainerStats returns the resource usage stats for a container endpoint
func (a *App) GetContainerStats(endpointID string) (*models.ContainerStats, error) {
	stats := a.containerHandler.GetContainerStats(endpointID)
	if stats == nil {
		return &models.ContainerStats{
			EndpointID: endpointID,
			LastCheck:  time.Now().Format(time.RFC3339),
		}, nil
	}

	return stats, nil
}

// StartContainer starts a single container endpoint
func (a *App) StartContainer(endpointID string) error {
	// Find endpoint
	for i := range a.config.Endpoints {
		if a.config.Endpoints[i].ID == endpointID {
			endpoint := &a.config.Endpoints[i]
			if endpoint.Type != models.EndpointTypeContainer {
				return fmt.Errorf("endpoint is not a container")
			}

			// Create cancellable context for this container startup
			ctx, cancel := context.WithCancel(context.Background())

			// Store cancel function in map (thread-safe)
			a.containerStartMutex.Lock()
			a.containerStartContexts[endpointID] = cancel
			a.containerStartMutex.Unlock()

			// Clean up cancel function after startup completes
			defer func() {
				a.containerStartMutex.Lock()
				delete(a.containerStartContexts, endpointID)
				a.containerStartMutex.Unlock()
			}()

			return a.containerHandler.StartContainer(ctx, endpoint)
		}
	}

	return fmt.Errorf("endpoint not found")
}

// CancelContainerStart cancels an ongoing container startup operation
func (a *App) CancelContainerStart(endpointID string) error {
	a.containerStartMutex.Lock()
	defer a.containerStartMutex.Unlock()

	// Get cancel function from map
	cancel, exists := a.containerStartContexts[endpointID]
	if !exists {
		return fmt.Errorf("no container startup in progress for endpoint %s", endpointID)
	}

	// Call the cancel function to stop the startup
	cancel()

	// Remove from map (cleanup will also happen in deferred function of StartContainer)
	delete(a.containerStartContexts, endpointID)

	log.Printf("Container startup cancelled for endpoint: %s", endpointID)
	return nil
}

// StopContainer stops (and removes) a single container endpoint
func (a *App) StopContainer(endpointID string) error {
	// Find endpoint
	for i := range a.config.Endpoints {
		if a.config.Endpoints[i].ID == endpointID {
			endpoint := &a.config.Endpoints[i]
			if endpoint.Type != models.EndpointTypeContainer {
				return fmt.Errorf("endpoint is not a container")
			}

			ctx := context.Background()
			return a.containerHandler.StopContainer(ctx, endpoint)
		}
	}

	return fmt.Errorf("endpoint not found")
}

// DeleteContainer is an alias for StopContainer (containers are removed when stopped)
func (a *App) DeleteContainer(endpointID string) error {
	return a.StopContainer(endpointID)
}

// RestartContainer restarts a container endpoint
func (a *App) RestartContainer(endpointID string) error {
	// Find endpoint
	for i := range a.config.Endpoints {
		if a.config.Endpoints[i].ID == endpointID {
			endpoint := &a.config.Endpoints[i]
			if endpoint.Type != models.EndpointTypeContainer {
				return fmt.Errorf("endpoint is not a container")
			}

			ctx := context.Background()
			if err := a.containerHandler.StopContainer(ctx, endpoint); err != nil {
				return fmt.Errorf("failed to stop container: %w", err)
			}

			if err := a.containerHandler.StartContainer(ctx, endpoint); err != nil {
				return fmt.Errorf("failed to start container: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("endpoint not found")
}

// GetContainerLogs retrieves container stdout/stderr logs
func (a *App) GetContainerLogs(endpointID string, tail int) (string, error) {
	// Use configured limit if not specified (tail <= 0)
	if tail <= 0 {
		tail = a.config.ContainerLogLineLimit
		if tail <= 0 {
			tail = 5000 // Default to 5000 lines
		}
	}

	ctx := context.Background()
	return a.containerHandler.GetContainerLogs(ctx, endpointID, tail)
}

// TestContainerConfig tests a container configuration by creating a temporary container
// This is called from the wizard before the endpoint is created
func (a *App) TestContainerConfig(config map[string]interface{}) error {
	// Parse configuration from frontend
	imageName := getString(config, "image_name")
	if imageName == "" {
		return fmt.Errorf("image_name is required")
	}

	containerPort := getInt(config, "container_port", 0)
	if containerPort <= 0 {
		return fmt.Errorf("container_port must be greater than 0")
	}

	// Parse volumes
	var volumes []models.VolumeMapping
	if volumesData, ok := config["volumes"].([]interface{}); ok {
		volumes = parseVolumes(volumesData)
	}

	// Parse environment variables
	var environment []models.EnvironmentVar
	if envData, ok := config["environment"].([]interface{}); ok {
		environment = parseEnvironmentVars(envData)
	}

	_ = getBool(config, "host_networking", false)           // Parsed but not used - not yet supported in runtime interface
	_ = getBool(config, "docker_socket_access", false)      // Parsed but not used - not yet supported in runtime interface
	healthCheckEnabled := getBool(config, "health_check_enabled", false)
	healthCheckPath := getString(config, "health_check_path")

	// Create temporary container runtime
	containerRuntime, err := containerruntime.DetectRuntime()
	if err != nil {
		return fmt.Errorf("Docker/Podman not available: %w", err)
	}

	// Generate unique test container name with timestamp
	testName := fmt.Sprintf("mockelot-test-%d", time.Now().Unix())

	var containerID string

	// Cleanup on error or completion
	defer func() {
		if containerID != "" {
			log.Printf("Cleaning up test container: %s", testName)
			cleanupCtx := context.Background()
			containerRuntime.StopContainer(cleanupCtx, containerID, 5)
			containerRuntime.RemoveContainer(cleanupCtx, containerID, true)
		}
	}()

	ctx := context.Background()

	// Check if image exists, pull if needed
	err = containerRuntime.ValidateImage(ctx, imageName)
	if err != nil {
		// Image not found, try to pull
		log.Printf("Pulling image for test: %s", imageName)
		reader, err := containerRuntime.PullImage(ctx, imageName)
		if err != nil {
			return fmt.Errorf("failed to pull image: %w", err)
		}
		defer reader.Close()

		// Wait for pull to complete
		_, err = io.Copy(io.Discard, reader)
		if err != nil {
			return fmt.Errorf("error during image pull: %w", err)
		}
	}

	// Prepare environment variables
	var env []string
	for _, envVar := range environment {
		value := envVar.Value
		// Expression evaluation is optional for test
		if envVar.Expression != "" {
			// For test, just use the static value if expression is set
			// Full expression evaluation would require goja VM
		}
		env = append(env, fmt.Sprintf("%s=%s", envVar.Name, value))
	}

	// Prepare volume mounts
	var mounts []containerruntime.Mount
	for _, vol := range volumes {
		hostPath := containerruntime.TranslatePath(vol.HostPath)
		mounts = append(mounts, containerruntime.Mount{
			Source:   hostPath,
			Target:   vol.ContainerPath,
			ReadOnly: vol.ReadOnly,
		})
	}

	// Create container
	// TODO: HostNetworking and DockerSocketAccess options not yet supported in runtime interface
	// These are validated in the wizard but not used for testing
	createConfig := &containerruntime.ContainerCreateConfig{
		Name:         testName,
		Image:        imageName,
		Env:          env,
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", containerPort)},
		PortBindings: map[string]string{
			fmt.Sprintf("%d/tcp", containerPort): "0", // Random host port
		},
		Mounts: mounts,
	}

	containerID, err = containerRuntime.CreateContainer(ctx, createConfig)
	if err != nil {
		return fmt.Errorf("failed to create test container: %w", err)
	}

	// Start container
	if err := containerRuntime.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("failed to start test container: %w", err)
	}

	// Wait a moment for container to initialize
	time.Sleep(2 * time.Second)

	// Check if container is still running
	info, err := containerRuntime.InspectContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to inspect test container: %w", err)
	}

	if !info.Running {
		return fmt.Errorf("container exited immediately (status: %s)", info.Status)
	}

	// Perform health check if enabled
	if healthCheckEnabled && healthCheckPath != "" {
		portKey := fmt.Sprintf("%d/tcp", containerPort)
		hostPort, ok := info.Ports[portKey]
		if !ok || hostPort == "" {
			return fmt.Errorf("container port %d not bound to host", containerPort)
		}

		healthURL := fmt.Sprintf("http://127.0.0.1:%s%s", hostPort, healthCheckPath)
		client := &http.Client{Timeout: 5 * time.Second}

		resp, err := client.Get(healthURL)
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return fmt.Errorf("health check returned status %d", resp.StatusCode)
		}
	}

	// Test succeeded - cleanup will happen in defer
	return nil
}

// GetSelectedEndpointId returns the currently selected endpoint ID from ServerConfig
func (a *App) GetSelectedEndpointId() string {
	// Load from server config
	serverCfg, err := a.serverConfigMgr.Load()
	if err != nil {
		fmt.Printf("Failed to load selected endpoint ID: %v\n", err)
		// Return first endpoint ID if available
		if len(a.config.Endpoints) > 0 {
			return a.config.Endpoints[0].ID
		}
		return ""
	}
	return serverCfg.SelectedEndpointId
}

// SetSelectedEndpointId sets the currently selected endpoint ID and saves to ServerConfig
func (a *App) SetSelectedEndpointId(endpointId string) error {
	a.configMutex.Lock()
	a.config.SelectedEndpointId = endpointId
	a.configMutex.Unlock()

	// Emit events to frontend
	runtime.EventsEmit(a.ctx, "endpoint:selected", endpointId)
	runtime.EventsEmit(a.ctx, "config:dirty", true)

	return nil
}

// SaveCurrentConfig saves to the current config file (overwrites)
func (a *App) SaveCurrentConfig() error {
	if a.currentConfigPath == "" {
		return fmt.Errorf("no file currently loaded - use Save As instead")
	}

	if err := a.saveConfigToPath(a.currentConfigPath); err != nil {
		return err
	}

	// Mark as clean after successful save
	runtime.EventsEmit(a.ctx, "config:dirty", false)
	runtime.EventsEmit(a.ctx, "config:path", a.currentConfigPath)

	return nil
}

// SaveConfig prompts user for a file with default name based on current file + timestamp
func (a *App) SaveConfig() error {
	// Generate default filename
	defaultFilename := "http-tester-config.yaml"
	if a.currentConfigPath != "" {
		// Extract filename without extension
		base := filepath.Base(a.currentConfigPath)
		ext := filepath.Ext(base)
		nameWithoutExt := strings.TrimSuffix(base, ext)

		// Add timestamp
		timestamp := time.Now().Format("060102-150405") // YYMMDD-HHMMSS
		defaultFilename = fmt.Sprintf("%s - %s%s", nameWithoutExt, timestamp, ext)
	}

	// Open save dialog
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Configuration As",
		DefaultFilename: defaultFilename,
		Filters: []runtime.FileFilter{
			{DisplayName: "YAML Files", Pattern: "*.yaml;*.yml"},
		},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return nil // User cancelled
	}

	// Save and update current path
	if err := a.saveConfigToPath(path); err != nil {
		return err
	}

	// Update path and mark as clean
	a.configMutex.Lock()
	a.currentConfigPath = path
	a.savedConfig = a.deepCopyConfig(a.config)
	a.configMutex.Unlock()

	// Emit events
	runtime.EventsEmit(a.ctx, "config:saved", path)
	runtime.EventsEmit(a.ctx, "config:dirty", false)
	runtime.EventsEmit(a.ctx, "config:path", path)

	a.AddRecentFile(path)
	return nil
}

// saveConfigToPath saves the configuration to the specified path
func (a *App) saveConfigToPath(path string) error {
	// Create UserConfig with all settings (server settings + user content)
	userConfig := &models.UserConfig{
		// User content
		Responses:      a.config.Responses,
		Items:          a.config.Items,
		Endpoints:      a.config.Endpoints,

		// Server settings (now included in UserConfig)
		Port:                   a.config.Port,
		HTTP2Enabled:           a.config.HTTP2Enabled,
		HTTPSEnabled:           a.config.HTTPSEnabled,
		HTTPSPort:              a.config.HTTPSPort,
		HTTPToHTTPSRedirect:    a.config.HTTPToHTTPSRedirect,
		CertMode:               a.config.CertMode,
		CertPaths:              a.config.CertPaths,
		CertNames:              a.config.CertNames,

		// Shared settings
		CORS:           a.config.CORS,
		SOCKS5Config:   a.config.SOCKS5Config,
		DomainTakeover: a.config.DomainTakeover,

		// UI state
		SelectedEndpointId: a.config.SelectedEndpointId,

		// Metadata
		LastModified:   time.Now(),
	}

	// Save to YAML file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not create file: %v", err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(userConfig)
}

// LoadConfig loads user configuration (request processing rules + CORS) from a YAML file
func (a *App) LoadConfig() (*models.AppConfig, error) {
	// Open file dialog
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Load Configuration",
		Filters: []runtime.FileFilter{
			{DisplayName: "YAML Files", Pattern: "*.yaml;*.yml"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // User cancelled
	}

	// Load from YAML file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	var userCfg models.UserConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&userCfg); err != nil {
		return nil, fmt.Errorf("could not decode config: %v", err)
	}

	// Ensure all responses have IDs
	for i := range userCfg.Responses {
		if userCfg.Responses[i].ID == "" {
			userCfg.Responses[i].ID = uuid.New().String()
		}
	}
	for i := range userCfg.Items {
		if userCfg.Items[i].Type == "response" && userCfg.Items[i].Response != nil {
			if userCfg.Items[i].Response.ID == "" {
				userCfg.Items[i].Response.ID = uuid.New().String()
			}
		} else if userCfg.Items[i].Type == "group" && userCfg.Items[i].Group != nil {
			if userCfg.Items[i].Group.ID == "" {
				userCfg.Items[i].Group.ID = uuid.New().String()
			}
			for j := range userCfg.Items[i].Group.Responses {
				if userCfg.Items[i].Group.Responses[j].ID == "" {
					userCfg.Items[i].Group.Responses[j].ID = uuid.New().String()
				}
			}
		}
	}

	// Convert UserConfig to AppConfig
	a.configMutex.Lock()
	a.config = userConfigToAppConfig(&userCfg, a.config)
	a.currentConfigPath = path

	// Mark as clean (just loaded)
	a.savedConfig = a.deepCopyConfig(a.config)
	a.configMutex.Unlock()

	// If there's no selected endpoint or the selected endpoint doesn't exist anymore,
	// select the first endpoint
	if len(a.config.Endpoints) > 0 {
		selectedId := a.GetSelectedEndpointId()
		validSelection := false
		for _, endpoint := range a.config.Endpoints {
			if endpoint.ID == selectedId {
				validSelection = true
				break
			}
		}

		if !validSelection {
			// Select first endpoint (don't call SetSelectedEndpointId as it marks dirty)
			a.config.SelectedEndpointId = a.config.Endpoints[0].ID
		}
	}

	// Ensure all endpoints have DisplayOrder set (for legacy configs)
	a.ensureDisplayOrder()

	// Ensure rejections endpoint exists
	a.ensureRejectionsEndpoint()

	// Update server if running
	if a.server != nil {
		a.server.UpdateConfig(a.config)
		// Start monitoring for any container endpoints in the loaded config
		// This will detect and track any containers already running from previous sessions
		a.server.EnsureContainerMonitoring()
	} else {
		// Even if server isn't running, we can still monitor containers
		if a.containerHandler != nil {
			var containerEndpoints []*models.Endpoint
			for i := range a.config.Endpoints {
				endpoint := &a.config.Endpoints[i]
				if endpoint.Type == models.EndpointTypeContainer {
					containerEndpoints = append(containerEndpoints, endpoint)
				}
			}

			if len(containerEndpoints) > 0 {
				a.containerHandler.StopPolling() // Stop any existing polling
				a.containerHandler.StartContainerStatusPolling(containerEndpoints)
				a.containerHandler.StartContainerStatsPolling(containerEndpoints)
			}
		}
	}

	// Emit events to frontend
	runtime.EventsEmit(a.ctx, "responses:updated", a.config.Responses)
	runtime.EventsEmit(a.ctx, "items:updated", a.config.Items)
	runtime.EventsEmit(a.ctx, "endpoints:updated", a.config.Endpoints)
	runtime.EventsEmit(a.ctx, "config:loaded", a.config)
	runtime.EventsEmit(a.ctx, "config:dirty", false)
	runtime.EventsEmit(a.ctx, "config:path", path)

	// Add to recent files
	a.AddRecentFile(path)

	return a.config, nil
}

// getRecentFilesPath returns the path to the recent files JSON file
func (a *App) getRecentFilesPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Failed to get home directory: %v", err)
		return ""
	}
	configDir := filepath.Join(homeDir, ".mockelot")
	return filepath.Join(configDir, "recent-files.json")
}

// GetRecentFiles returns the list of recent files with existence check
// Limited to 24 most recent files (3 columns × 8 rows)
func (a *App) GetRecentFiles() ([]models.RecentFile, error) {
	recentFilesPath := a.getRecentFilesPath()
	if recentFilesPath == "" {
		return []models.RecentFile{}, nil
	}

	// Read recent files JSON
	data, err := os.ReadFile(recentFilesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet - return empty list
			return []models.RecentFile{}, nil
		}
		return nil, fmt.Errorf("failed to read recent files: %v", err)
	}

	var recentFiles models.RecentFiles
	if err := json.Unmarshal(data, &recentFiles); err != nil {
		return nil, fmt.Errorf("failed to parse recent files: %v", err)
	}

	// Check existence of each file and update Exists field
	for i := range recentFiles.Files {
		_, err := os.Stat(recentFiles.Files[i].Path)
		recentFiles.Files[i].Exists = err == nil
	}

	// Limit to 24 most recent files (sorted by LastAccessed desc)
	if len(recentFiles.Files) > 24 {
		recentFiles.Files = recentFiles.Files[:24]
	}

	return recentFiles.Files, nil
}

// AddRecentFile adds or updates a file in the recent files list
func (a *App) AddRecentFile(path string) error {
	recentFilesPath := a.getRecentFilesPath()
	if recentFilesPath == "" {
		return fmt.Errorf("failed to get recent files path")
	}

	// Ensure directory exists
	configDir := filepath.Dir(recentFilesPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Read existing recent files
	var recentFiles models.RecentFiles
	data, err := os.ReadFile(recentFilesPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read recent files: %v", err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &recentFiles); err != nil {
			return fmt.Errorf("failed to parse recent files: %v", err)
		}
	}

	// Check if file already exists in list
	found := false
	for i := range recentFiles.Files {
		if recentFiles.Files[i].Path == path {
			// Update last accessed time
			recentFiles.Files[i].LastAccessed = time.Now()
			recentFiles.Files[i].Exists = true
			found = true
			break
		}
	}

	if !found {
		// Add new file to the beginning
		newFile := models.RecentFile{
			Path:         path,
			LastAccessed: time.Now(),
			Exists:       true,
		}
		recentFiles.Files = append([]models.RecentFile{newFile}, recentFiles.Files...)
	}

	// Sort by LastAccessed (most recent first)
	// Use bubble sort for simplicity
	for i := 0; i < len(recentFiles.Files); i++ {
		for j := i + 1; j < len(recentFiles.Files); j++ {
			if recentFiles.Files[j].LastAccessed.After(recentFiles.Files[i].LastAccessed) {
				recentFiles.Files[i], recentFiles.Files[j] = recentFiles.Files[j], recentFiles.Files[i]
			}
		}
	}

	// Limit to 24 files
	if len(recentFiles.Files) > 24 {
		recentFiles.Files = recentFiles.Files[:24]
	}

	// Save to JSON
	data, err = json.MarshalIndent(recentFiles, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal recent files: %v", err)
	}

	if err := os.WriteFile(recentFilesPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write recent files: %v", err)
	}

	return nil
}

// RemoveRecentFile removes a file from the recent files list
func (a *App) RemoveRecentFile(path string) error {
	recentFilesPath := a.getRecentFilesPath()
	if recentFilesPath == "" {
		return fmt.Errorf("failed to get recent files path")
	}

	// Read existing recent files
	data, err := os.ReadFile(recentFilesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - nothing to remove
			return nil
		}
		return fmt.Errorf("failed to read recent files: %v", err)
	}

	var recentFiles models.RecentFiles
	if err := json.Unmarshal(data, &recentFiles); err != nil {
		return fmt.Errorf("failed to parse recent files: %v", err)
	}

	// Remove the file
	var newFiles []models.RecentFile
	for _, f := range recentFiles.Files {
		if f.Path != path {
			newFiles = append(newFiles, f)
		}
	}
	recentFiles.Files = newFiles

	// Save to JSON
	data, err = json.MarshalIndent(recentFiles, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal recent files: %v", err)
	}

	if err := os.WriteFile(recentFilesPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write recent files: %v", err)
	}

	return nil
}

// LoadConfigFromPath loads configuration from a specific file path
func (a *App) LoadConfigFromPath(path string) (*models.AppConfig, error) {
	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to access file: %v", err)
	}

	// Load from YAML file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	var userCfg models.UserConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&userCfg); err != nil {
		return nil, fmt.Errorf("could not decode config: %v", err)
	}

	// Ensure all responses have IDs
	for i := range userCfg.Responses {
		if userCfg.Responses[i].ID == "" {
			userCfg.Responses[i].ID = uuid.New().String()
		}
	}
	for i := range userCfg.Items {
		if userCfg.Items[i].Type == "response" && userCfg.Items[i].Response != nil {
			if userCfg.Items[i].Response.ID == "" {
				userCfg.Items[i].Response.ID = uuid.New().String()
			}
		} else if userCfg.Items[i].Type == "group" && userCfg.Items[i].Group != nil {
			if userCfg.Items[i].Group.ID == "" {
				userCfg.Items[i].Group.ID = uuid.New().String()
			}
			for j := range userCfg.Items[i].Group.Responses {
				if userCfg.Items[i].Group.Responses[j].ID == "" {
					userCfg.Items[i].Group.Responses[j].ID = uuid.New().String()
				}
			}
		}
	}

	// Convert UserConfig to AppConfig
	a.configMutex.Lock()
	a.config = userConfigToAppConfig(&userCfg, a.config)
	a.currentConfigPath = path

	// Mark as clean (just loaded)
	a.savedConfig = a.deepCopyConfig(a.config)
	a.configMutex.Unlock()

	// If there's no selected endpoint or the selected endpoint doesn't exist anymore,
	// select the first endpoint
	if len(a.config.Endpoints) > 0 {
		selectedId := a.GetSelectedEndpointId()
		validSelection := false
		for _, endpoint := range a.config.Endpoints {
			if endpoint.ID == selectedId {
				validSelection = true
				break
			}
		}

		if !validSelection {
			// Select first endpoint (don't call SetSelectedEndpointId as it marks dirty)
			a.config.SelectedEndpointId = a.config.Endpoints[0].ID
		}
	}

	// Ensure all endpoints have DisplayOrder set (for legacy configs)
	a.ensureDisplayOrder()

	// Ensure rejections endpoint exists
	a.ensureRejectionsEndpoint()

	// Update server if running
	if a.server != nil {
		a.server.UpdateConfig(a.config)
		// Start monitoring for any container endpoints in the loaded config
		// This will detect and track any containers already running from previous sessions
		a.server.EnsureContainerMonitoring()
	} else {
		// Even if server isn't running, we can still monitor containers
		if a.containerHandler != nil {
			var containerEndpoints []*models.Endpoint
			for i := range a.config.Endpoints {
				endpoint := &a.config.Endpoints[i]
				if endpoint.Type == models.EndpointTypeContainer {
					containerEndpoints = append(containerEndpoints, endpoint)
				}
			}

			if len(containerEndpoints) > 0 {
				a.containerHandler.StopPolling() // Stop any existing polling
				a.containerHandler.StartContainerStatusPolling(containerEndpoints)
				a.containerHandler.StartContainerStatsPolling(containerEndpoints)
			}
		}
	}

	// Emit events to frontend
	runtime.EventsEmit(a.ctx, "responses:updated", a.config.Responses)
	runtime.EventsEmit(a.ctx, "items:updated", a.config.Items)
	runtime.EventsEmit(a.ctx, "endpoints:updated", a.config.Endpoints)
	runtime.EventsEmit(a.ctx, "config:loaded", a.config)
	runtime.EventsEmit(a.ctx, "config:dirty", false)
	runtime.EventsEmit(a.ctx, "config:path", path)

	// Add to recent files
	a.AddRecentFile(path)

	return a.config, nil
}

// ImportOpenAPISpecWithDialog imports an OpenAPI/Swagger specification file
// Shows a file dialog and imports with the specified append mode
func (a *App) ImportOpenAPISpecWithDialog(appendMode bool) (*models.AppConfig, error) {
	return a.importOpenAPISpecWithMode(appendMode)
}

// importOpenAPISpecWithMode imports an OpenAPI/Swagger specification file with the specified mode
func (a *App) importOpenAPISpecWithMode(appendMode bool) (*models.AppConfig, error) {
	// Open file dialog
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import OpenAPI Specification",
		Filters: []runtime.FileFilter{
			{DisplayName: "OpenAPI Files", Pattern: "*.yaml;*.yml;*.json"},
			{DisplayName: "YAML Files", Pattern: "*.yaml;*.yml"},
			{DisplayName: "JSON Files", Pattern: "*.json"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // User cancelled
	}

	// Import the spec
	items, err := openapi.ImportSpec(path)
	if err != nil {
		return nil, fmt.Errorf("failed to import OpenAPI spec: %v", err)
	}

	// Get selected endpoint ID
	selectedEndpointId := a.GetSelectedEndpointId()

	// Import into selected endpoint if endpoints are configured
	if len(a.config.Endpoints) > 0 {
		// Find the selected endpoint
		found := false
		for i := range a.config.Endpoints {
			if a.config.Endpoints[i].ID == selectedEndpointId {
				if appendMode {
					// Append to existing items
					a.config.Endpoints[i].Items = append(a.config.Endpoints[i].Items, items...)
				} else {
					// Replace existing items
					a.config.Endpoints[i].Items = items
				}
				found = true
				break
			}
		}

		if !found && len(a.config.Endpoints) > 0 {
			// If selected endpoint not found, use first endpoint
			if appendMode {
				a.config.Endpoints[0].Items = append(a.config.Endpoints[0].Items, items...)
			} else {
				a.config.Endpoints[0].Items = items
			}
		}
	} else {
		// Fallback to legacy Items for backward compatibility
		if appendMode {
			a.config.Items = append(a.config.Items, items...)
		} else {
			a.config.Items = items
		}
	}

	// Update server if running
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "items:updated", items)

	return a.config, nil
}

// GetRequestLogs returns all request log summaries
func (a *App) GetRequestLogs() []models.RequestLogSummary {
	a.logMutex.RLock()
	defer a.logMutex.RUnlock()

	// Create summaries from full logs
	summaries := make([]models.RequestLogSummary, len(a.requestLogs))
	for i, log := range a.requestLogs {
		summaries[i] = models.RequestLogSummary{
			ID:             log.ID,
			Timestamp:      log.Timestamp,
			EndpointID:     log.EndpointID,
			Method:         log.ClientRequest.Method,
			Path:           log.ClientRequest.Path,
			SourceIP:       log.ClientRequest.SourceIP,
			ClientStatus:   log.ClientResponse.StatusCode,
			ClientRTT:      log.ClientResponse.RTTMs,
			HasBackend:     log.BackendRequest != nil || log.BackendResponse != nil,
			ClientBodySize: len(log.ClientRequest.Body),
		}
		if log.BackendResponse != nil {
			summaries[i].BackendStatus = log.BackendResponse.StatusCode
			summaries[i].BackendRTT = log.BackendResponse.RTTMs
		}
	}
	return summaries
}

// GetRequestLogByID returns a specific request log by ID
func (a *App) GetRequestLogByID(id string) *models.RequestLog {
	a.logMutex.RLock()
	defer a.logMutex.RUnlock()

	for _, log := range a.requestLogs {
		if log.ID == id {
			return &log
		}
	}
	return nil
}

// ClearRequestLogs clears all request logs
func (a *App) ClearRequestLogs() {
	a.logMutex.Lock()
	defer a.logMutex.Unlock()

	a.requestLogs = make([]models.RequestLog, 0)
	runtime.EventsEmit(a.ctx, "logs:cleared", nil)
}

// ExportLogs exports logs in the specified format
func (a *App) ExportLogs(format string) error {
	a.logMutex.RLock()
	logs := make([]models.RequestLog, len(a.requestLogs))
	copy(logs, a.requestLogs)
	a.logMutex.RUnlock()

	var defaultName string
	var pattern string
	if format == "csv" {
		defaultName = "request-logs.csv"
		pattern = "*.csv"
	} else {
		defaultName = "request-logs.json"
		pattern = "*.json"
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export Logs",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: fmt.Sprintf("%s Files", format), Pattern: pattern},
		},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return nil // User cancelled
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not create file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(logs)
}

// HTTPS Certificate Management Methods

// GetCACertInfo returns information about the CA certificate
func (a *App) GetCACertInfo() (models.CACertInfo, error) {
	certManager, err := server.NewCertificateManager()
	if err != nil {
		return models.CACertInfo{}, fmt.Errorf("failed to initialize certificate manager: %w", err)
	}

	info := models.CACertInfo{
		Exists: certManager.CAExists(),
	}

	if info.Exists {
		timestamp, err := certManager.GetCATimestamp()
		if err == nil {
			info.Generated = timestamp.Format(time.RFC3339)
		}
	}

	return info, nil
}

// GetDefaultCertNames returns the default DNS names and IP addresses that will be used for certificates
// Returns a list of strings containing: localhost, machine hostname, and interface IP for default gateway
func (a *App) GetDefaultCertNames() ([]string, error) {
	dnsNames, ipAddresses := server.GetDefaultCertNames()

	// Combine into a single list of strings
	var result []string
	result = append(result, dnsNames...)
	for _, ip := range ipAddresses {
		result = append(result, ip.String())
	}

	return result, nil
}

// RegenerateCA regenerates the CA certificate and restarts the HTTPS server
func (a *App) RegenerateCA() error {
	certManager, err := server.NewCertificateManager()
	if err != nil {
		return fmt.Errorf("failed to initialize certificate manager: %w", err)
	}

	// Generate new CA certificate
	_, _, err = certManager.GenerateCA()
	if err != nil {
		return fmt.Errorf("failed to generate CA certificate: %w", err)
	}

	// Restart HTTPS server if it's running
	if a.server != nil && a.status.Running && a.config.HTTPSEnabled {
		err = a.server.RestartHTTPS()
		if err != nil {
			return fmt.Errorf("failed to restart HTTPS server: %w", err)
		}
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "ca:regenerated", nil)

	return nil
}

// DownloadCACert returns the CA certificate PEM for download
func (a *App) DownloadCACert() (string, error) {
	certManager, err := server.NewCertificateManager()
	if err != nil {
		return "", fmt.Errorf("failed to initialize certificate manager: %w", err)
	}

	if !certManager.CAExists() {
		return "", fmt.Errorf("CA certificate does not exist")
	}

	certPEM, err := certManager.GetCACertPEM()
	if err != nil {
		return "", fmt.Errorf("failed to read CA certificate: %w", err)
	}

	// Show save dialog
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Download CA Certificate",
		DefaultFilename: "mockelot-ca.crt",
		Filters: []runtime.FileFilter{
			{DisplayName: "Certificate Files", Pattern: "*.crt;*.pem"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // User cancelled
	}

	// Write certificate to file
	err = os.WriteFile(path, certPEM, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to save CA certificate: %w", err)
	}

	return path, nil
}

// InstallCACertSystem installs the CA certificate at the system level
// Requires administrator/root privileges
func (a *App) InstallCACertSystem() error {
	certManager, err := server.NewCertificateManager()
	if err != nil {
		return fmt.Errorf("failed to initialize certificate manager: %w", err)
	}

	if !certManager.CAExists() {
		return fmt.Errorf("CA certificate does not exist - please start HTTPS server first")
	}

	certPEM, err := certManager.GetCACertPEM()
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	// Save to temporary file
	tmpFile, err := os.CreateTemp("", "mockelot-ca-*.crt")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(certPEM); err != nil {
		return fmt.Errorf("failed to write temporary certificate: %w", err)
	}
	tmpFile.Close()

	// Detect OS and execute appropriate installation command
	switch goruntime.GOOS {
	case "linux":
		return a.installCACertLinux(tmpFile.Name())
	case "windows":
		return a.installCACertWindows(tmpFile.Name())
	case "darwin":
		return a.installCACertMacOS(tmpFile.Name())
	default:
		return fmt.Errorf("unsupported operating system: %s", goruntime.GOOS)
	}
}

// installCACertLinux installs CA certificate on Linux systems
func (a *App) installCACertLinux(certPath string) error {
	// Copy to system trust store
	targetDir := "/usr/local/share/ca-certificates"
	targetPath := filepath.Join(targetDir, "mockelot-ca.crt")

	// Check if we need sudo
	testFile := filepath.Join(targetDir, ".mockelot-test")
	needsSudo := true
	if f, err := os.Create(testFile); err == nil {
		f.Close()
		os.Remove(testFile)
		needsSudo = false
	}

	if needsSudo {
		// Use pkexec (polkit) for GUI sudo prompt, fallback to terminal
		cmd := exec.Command("pkexec", "sh", "-c",
			fmt.Sprintf("cp '%s' '%s' && update-ca-certificates", certPath, targetPath))

		output, err := cmd.CombinedOutput()
		if err != nil {
			// Try with terminal-based sudo as fallback
			cmd = exec.Command("sudo", "sh", "-c",
				fmt.Sprintf("cp '%s' '%s' && update-ca-certificates", certPath, targetPath))
			output, err = cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to install certificate: %w\nOutput: %s", err, string(output))
			}
		}
	} else {
		// Can write directly
		if err := copyFile(certPath, targetPath); err != nil {
			return fmt.Errorf("failed to copy certificate: %w", err)
		}
		cmd := exec.Command("update-ca-certificates")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to update certificates: %w\nOutput: %s", err, string(output))
		}
	}

	return nil
}

// installCACertWindows installs CA certificate on Windows systems
func (a *App) installCACertWindows(certPath string) error {
	// Use certutil to add to Trusted Root Certification Authorities
	cmd := exec.Command("certutil", "-addstore", "-f", "ROOT", certPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install certificate: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// installCACertMacOS installs CA certificate on macOS systems
func (a *App) installCACertMacOS(certPath string) error {
	// Use security command to add to system keychain
	cmd := exec.Command("sudo", "security", "add-trusted-cert", "-d", "-r", "trustRoot",
		"-k", "/Library/Keychains/System.keychain", certPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install certificate: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// UpdateServerSettings updates server configuration fields
// Does NOT save to disk - only updates in-memory config and emits events
// Frontend should call MarkDirty() after this to mark config as dirty
func (a *App) UpdateServerSettings(settings models.ServerSettings) error {
	a.configMutex.Lock()
	defer a.configMutex.Unlock()

	// Update AppConfig fields (only those provided - nil means don't update)
	if settings.Port != nil {
		a.config.Port = *settings.Port
	}
	if settings.HTTP2Enabled != nil {
		a.config.HTTP2Enabled = *settings.HTTP2Enabled
	}
	if settings.HTTPSEnabled != nil {
		a.config.HTTPSEnabled = *settings.HTTPSEnabled
	}
	if settings.HTTPSPort != nil {
		a.config.HTTPSPort = *settings.HTTPSPort
	}
	if settings.HTTPToHTTPSRedirect != nil {
		a.config.HTTPToHTTPSRedirect = *settings.HTTPToHTTPSRedirect
	}
	if settings.CertMode != nil {
		a.config.CertMode = *settings.CertMode
	}
	if settings.CertPaths != nil {
		a.config.CertPaths = *settings.CertPaths
	}
	if settings.CertNames != nil {
		a.config.CertNames = settings.CertNames
	}
	if settings.CORS != nil {
		a.config.CORS = *settings.CORS
	}
	if settings.SOCKS5Config != nil {
		a.config.SOCKS5Config = settings.SOCKS5Config
	}
	if settings.DomainTakeover != nil {
		a.config.DomainTakeover = settings.DomainTakeover
	}

	// Emit config updated event
	runtime.EventsEmit(a.ctx, "config:updated", a.config)

	return nil
}

// SelectCertFile shows a file picker for certificate files
func (a *App) SelectCertFile(title string) (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: title,
		Filters: []runtime.FileFilter{
			{DisplayName: "Certificate Files", Pattern: "*.pem;*.crt;*.key"},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// CORS Configuration Methods

// GetCORSConfig returns the current CORS configuration
func (a *App) GetCORSConfig() *models.CORSConfig {
	return &a.config.CORS
}

// ValidateCORSScript validates a CORS script for syntax errors
func (a *App) ValidateCORSScript(script string) error {
	return server.ValidateCORSScript(script)
}

// SOCKS5ConfigResponse represents the combined SOCKS5 and domain takeover configuration
type SOCKS5ConfigResponse struct {
	SOCKS5Config    *models.SOCKS5Config           `json:"socks5_config"`
	DomainTakeover *models.DomainTakeoverConfig `json:"domain_takeover"`
}

// GetSOCKS5Config returns the current SOCKS5 and domain takeover configuration
func (a *App) GetSOCKS5Config() SOCKS5ConfigResponse {
	return SOCKS5ConfigResponse{
		SOCKS5Config:    a.config.SOCKS5Config,
		DomainTakeover: a.config.DomainTakeover,
	}
}

// ValidateCORSHeaderExpression validates a CORS header expression for syntax errors
func (a *App) ValidateCORSHeaderExpression(expression string) error {
	return server.ValidateHeaderExpression(expression)
}

// LogRequest implements the server.RequestLogger interface
func (a *App) LogRequest(log models.RequestLog) {
	a.logMutex.Lock()
	a.requestLogs = append(a.requestLogs, log)
	a.logMutex.Unlock()

	// Create lightweight summary for frontend
	summary := models.RequestLogSummary{
		ID:         log.ID,
		Timestamp:  log.Timestamp,
		EndpointID: log.EndpointID,
		Method:     log.ClientRequest.Method,
		Path:       log.ClientRequest.Path,
		SourceIP:   log.ClientRequest.SourceIP,
		ClientStatus: log.ClientResponse.StatusCode,
		ClientRTT:  log.ClientResponse.RTTMs,
		HasBackend: log.BackendRequest != nil || log.BackendResponse != nil,
		ClientBodySize: len(log.ClientRequest.Body),
		ValidationFailed: log.ValidationFailed,
		ResponseFailed:   log.ResponseFailed,
	}

	// Add backend info if present
	if log.BackendResponse != nil {
		summary.BackendStatus = log.BackendResponse.StatusCode
		summary.BackendRTT = log.BackendResponse.RTTMs
	}

	// Set pending status
	summary.Pending = false // By default, logs are complete

	// Queue summary for frontend polling (more efficient than individual events during high traffic)
	a.requestLogQueueMutex.Lock()
	a.requestLogSummaryQueue = append(a.requestLogSummaryQueue, summary)
	a.requestLogQueueMutex.Unlock()
}

// UpdateRequestLog updates an existing request log (used for two-phase logging)
// This allows showing pending requests immediately, then updating them when complete
func (a *App) UpdateRequestLog(log models.RequestLog) {
	a.logMutex.Lock()

	// Find and update the existing log
	found := false
	for i := range a.requestLogs {
		if a.requestLogs[i].ID == log.ID {
			a.requestLogs[i] = log
			found = true
			break
		}
	}

	// If not found, just append it (fallback behavior)
	if !found {
		a.requestLogs = append(a.requestLogs, log)
	}

	a.logMutex.Unlock()

	// Create updated summary for frontend
	summary := models.RequestLogSummary{
		ID:         log.ID,
		Timestamp:  log.Timestamp,
		EndpointID: log.EndpointID,
		Method:     log.ClientRequest.Method,
		Path:       log.ClientRequest.Path,
		SourceIP:   log.ClientRequest.SourceIP,
		ClientStatus: log.ClientResponse.StatusCode,
		ClientRTT:  log.ClientResponse.RTTMs,
		HasBackend: log.BackendRequest != nil || log.BackendResponse != nil,
		ClientBodySize: len(log.ClientRequest.Body),
		Pending:    false, // Update means request is complete
		ValidationFailed: log.ValidationFailed,
		ResponseFailed:   log.ResponseFailed,
	}

	// Add backend info if present
	if log.BackendResponse != nil {
		summary.BackendStatus = log.BackendResponse.StatusCode
		summary.BackendRTT = log.BackendResponse.RTTMs
	}

	// Queue updated summary
	a.requestLogQueueMutex.Lock()
	a.requestLogSummaryQueue = append(a.requestLogSummaryQueue, summary)
	a.requestLogQueueMutex.Unlock()
}

// GetRequestLogDetails returns the full RequestLog details for a given ID
func (a *App) GetRequestLogDetails(id string) (*models.RequestLog, error) {
	a.logMutex.RLock()
	defer a.logMutex.RUnlock()

	for i := range a.requestLogs {
		if a.requestLogs[i].ID == id {
			return &a.requestLogs[i], nil
		}
	}

	return nil, fmt.Errorf("request log with ID %s not found", id)
}

// PollRequestLogs returns all queued request log summaries and clears the queue
// This is called by the frontend at regular intervals (polling) for efficient batching
// during high-volume traffic
func (a *App) PollRequestLogs() []models.RequestLogSummary {
	a.requestLogQueueMutex.Lock()
	defer a.requestLogQueueMutex.Unlock()

	// Get current summaries
	summaries := a.requestLogSummaryQueue

	// Clear the queue
	a.requestLogSummaryQueue = make([]models.RequestLogSummary, 0)

	return summaries
}

// ========== Script Error Management ==========

// LogScriptError logs a script execution error and emits an event to the frontend
func (a *App) LogScriptError(responseID, path, method, errorMsg string) {
	a.scriptErrorsMutex.Lock()
	defer a.scriptErrorsMutex.Unlock()

	log.Printf("LogScriptError called: responseID=%s, path=%s, method=%s, error=%s", responseID, path, method, errorMsg)

	errorLog := ScriptErrorLog{
		Timestamp:  time.Now(),
		Error:      errorMsg,
		ResponseID: responseID,
		Path:       path,
		Method:     method,
	}

	// Append to error log for this response (keep last 100 errors per response)
	if _, exists := a.scriptErrors[responseID]; !exists {
		a.scriptErrors[responseID] = make([]ScriptErrorLog, 0)
	}
	a.scriptErrors[responseID] = append(a.scriptErrors[responseID], errorLog)

	// Keep only last 100 errors per response
	if len(a.scriptErrors[responseID]) > 100 {
		a.scriptErrors[responseID] = a.scriptErrors[responseID][len(a.scriptErrors[responseID])-100:]
	}

	// Emit event to frontend via Wails runtime (not polling queue)
	eventData := map[string]interface{}{
		"response_id": responseID,
		"path":        path,
		"method":      method,
		"error":       errorMsg,
		"timestamp":   errorLog.Timestamp.Format(time.RFC3339),
	}
	log.Printf("Emitting script:error event with data: %+v", eventData)
	runtime.EventsEmit(a.ctx, "script:error", eventData)
}

// GetScriptErrors returns all script errors for a given response ID
func (a *App) GetScriptErrors(responseID string) []ScriptErrorLog {
	a.scriptErrorsMutex.RLock()
	defer a.scriptErrorsMutex.RUnlock()

	if errors, exists := a.scriptErrors[responseID]; exists {
		// Return a copy to avoid race conditions
		result := make([]ScriptErrorLog, len(errors))
		copy(result, errors)
		return result
	}
	return []ScriptErrorLog{}
}

// ClearScriptErrors clears all script errors for a given response ID
func (a *App) ClearScriptErrors(responseID string) {
	a.scriptErrorsMutex.Lock()
	defer a.scriptErrorsMutex.Unlock()

	delete(a.scriptErrors, responseID)

	// Emit event to frontend via Wails runtime (not polling queue)
	runtime.EventsEmit(a.ctx, "script:error:cleared", map[string]interface{}{
		"response_id": responseID,
	})
}

// GetAllResponseIDsWithErrors returns a list of all response IDs that have script errors
func (a *App) GetAllResponseIDsWithErrors() []string {
	a.scriptErrorsMutex.RLock()
	defer a.scriptErrorsMutex.RUnlock()

	ids := make([]string, 0, len(a.scriptErrors))
	for id := range a.scriptErrors {
		ids = append(ids, id)
	}
	return ids
}

// ================================================================================
// Dirty State Tracking Methods
// ================================================================================

// IsDirty returns true if current config differs from saved config
func (a *App) IsDirty() bool {
	a.configMutex.RLock()
	defer a.configMutex.RUnlock()

	if a.savedConfig == nil {
		return false // No saved state yet
	}

	// Deep comparison of configs (excluding LastModified)
	return !a.configsEqual(a.config, a.savedConfig)
}

// configsEqual performs deep comparison of two AppConfigs
func (a *App) configsEqual(c1, c2 *models.AppConfig) bool {
	if c1 == nil || c2 == nil {
		return c1 == c2
	}

	// Compare server settings
	if c1.Port != c2.Port ||
		c1.HTTP2Enabled != c2.HTTP2Enabled ||
		c1.HTTPSEnabled != c2.HTTPSEnabled ||
		c1.HTTPSPort != c2.HTTPSPort ||
		c1.HTTPToHTTPSRedirect != c2.HTTPToHTTPSRedirect ||
		c1.CertMode != c2.CertMode {
		return false
	}

	// Compare cert paths/names
	if !certPathsEqual(c1.CertPaths, c2.CertPaths) ||
		!stringSlicesEqual(c1.CertNames, c2.CertNames) {
		return false
	}

	// Compare CORS
	if !corsConfigEqual(&c1.CORS, &c2.CORS) {
		return false
	}

	// Compare SOCKS5
	if !socks5ConfigEqual(c1.SOCKS5Config, c2.SOCKS5Config) {
		return false
	}

	// Compare DomainTakeover
	if !domainTakeoverEqual(c1.DomainTakeover, c2.DomainTakeover) {
		return false
	}

	// Compare SelectedEndpointId
	if c1.SelectedEndpointId != c2.SelectedEndpointId {
		return false
	}

	// Compare user content (endpoints, responses, items)
	if !endpointsEqual(c1.Endpoints, c2.Endpoints) ||
		!responsesEqual(c1.Responses, c2.Responses) ||
		!itemsEqual(c1.Items, c2.Items) {
		return false
	}

	return true
}

// certPathsEqual compares two CertPaths structs for equality
func certPathsEqual(c1, c2 models.CertPaths) bool {
	return c1.CACertPath == c2.CACertPath &&
		c1.CAKeyPath == c2.CAKeyPath &&
		c1.ServerCertPath == c2.ServerCertPath &&
		c1.ServerKeyPath == c2.ServerKeyPath &&
		c1.ServerBundlePath == c2.ServerBundlePath
}

// stringSlicesEqual compares two string slices for equality
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// corsConfigEqual compares two CORS configs for equality
func corsConfigEqual(c1, c2 *models.CORSConfig) bool {
	if c1 == nil || c2 == nil {
		return c1 == c2
	}
	return c1.Enabled == c2.Enabled &&
		c1.Mode == c2.Mode &&
		headersEqual(c1.HeaderExpressions, c2.HeaderExpressions) &&
		c1.OptionsDefaultStatus == c2.OptionsDefaultStatus
}

// headersEqual compares two slices of CORSHeader for equality
func headersEqual(h1, h2 []models.CORSHeader) bool {
	if len(h1) != len(h2) {
		return false
	}
	for i := range h1 {
		if h1[i].Name != h2[i].Name || h1[i].Expression != h2[i].Expression {
			return false
		}
	}
	return true
}

// socks5ConfigEqual compares two SOCKS5 configs for equality
func socks5ConfigEqual(s1, s2 *models.SOCKS5Config) bool {
	if s1 == nil || s2 == nil {
		return s1 == s2
	}
	return s1.Enabled == s2.Enabled &&
		s1.Port == s2.Port &&
		s1.Authentication == s2.Authentication &&
		s1.Username == s2.Username &&
		s1.Password == s2.Password
}

// domainTakeoverEqual compares two DomainTakeover configs for equality
func domainTakeoverEqual(d1, d2 *models.DomainTakeoverConfig) bool {
	if d1 == nil || d2 == nil {
		return d1 == d2
	}
	// Compare domains using JSON deep equality
	return jsonEqual(d1.Domains, d2.Domains)
}

// endpointsEqual compares two endpoint slices for equality
func endpointsEqual(e1, e2 []models.Endpoint) bool {
	if len(e1) != len(e2) {
		return false
	}
	return jsonEqual(e1, e2)
}

// responsesEqual compares two response slices for equality
func responsesEqual(r1, r2 []models.MethodResponse) bool {
	if len(r1) != len(r2) {
		return false
	}
	return jsonEqual(r1, r2)
}

// itemsEqual compares two item slices for equality
func itemsEqual(i1, i2 []models.ResponseItem) bool {
	if len(i1) != len(i2) {
		return false
	}
	return jsonEqual(i1, i2)
}

// jsonEqual uses JSON marshaling for deep comparison
func jsonEqual(a, b interface{}) bool {
	aJSON, err1 := json.Marshal(a)
	bJSON, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

// MarkDirty marks the config as dirty (without updating savedConfig)
// Called when user makes changes in Server tab
func (a *App) MarkDirty() {
	a.configMutex.Lock()
	defer a.configMutex.Unlock()

	// Don't update savedConfig - this makes IsDirty() return true
	// savedConfig remains at last saved state

	// Emit event to update UI
	runtime.EventsEmit(a.ctx, "config:dirty", true)
}

// MarkClean updates savedConfig to current state
// Called after successful save
func (a *App) MarkClean() {
	a.configMutex.Lock()
	defer a.configMutex.Unlock()

	// Deep copy current config to savedConfig
	a.savedConfig = a.deepCopyConfig(a.config)

	// Emit event to update UI
	runtime.EventsEmit(a.ctx, "config:dirty", false)
}

// deepCopyConfig creates a deep copy of AppConfig
func (a *App) deepCopyConfig(config *models.AppConfig) *models.AppConfig {
	if config == nil {
		return nil
	}

	// Use JSON marshaling for deep copy
	data, err := json.Marshal(config)
	if err != nil {
		log.Printf("Error marshaling config for deep copy: %v", err)
		return nil
	}

	var copy models.AppConfig
	if err := json.Unmarshal(data, &copy); err != nil {
		log.Printf("Error unmarshaling config for deep copy: %v", err)
		return nil
	}

	return &copy
}

// GetCurrentConfigPath returns the current config file path
func (a *App) GetCurrentConfigPath() string {
	return a.currentConfigPath
}

// userConfigToAppConfig converts UserConfig to AppConfig
// serverCfg is the current AppConfig - we preserve server settings from it
func userConfigToAppConfig(userCfg *models.UserConfig, serverCfg *models.AppConfig) *models.AppConfig {
	// Start with defaults for server settings
	appCfg := &models.AppConfig{
		Port:                8080,
		HTTPSPort:           8443,
		HTTP2Enabled:        false,
		HTTPSEnabled:        false,
		HTTPToHTTPSRedirect: false,
		CertMode:            models.CertModeAuto,
		CertPaths:           models.CertPaths{},
		CertNames:           []string{},

		// Copy user content from UserConfig
		Responses:           userCfg.Responses,
		Items:               userCfg.Items,
		Endpoints:           userCfg.Endpoints,
		CORS:                userCfg.CORS,
		SOCKS5Config:        userCfg.SOCKS5Config,
		DomainTakeover:      userCfg.DomainTakeover,
		SelectedEndpointId:  userCfg.SelectedEndpointId,
	}

	// Server settings now come from UserConfig (unified format)
	// Use values from UserConfig if present (non-zero), otherwise keep defaults
	if userCfg.Port != 0 {
		appCfg.Port = userCfg.Port
	}
	if userCfg.HTTPSPort != 0 {
		appCfg.HTTPSPort = userCfg.HTTPSPort
	}
	appCfg.HTTP2Enabled = userCfg.HTTP2Enabled
	appCfg.HTTPSEnabled = userCfg.HTTPSEnabled
	appCfg.HTTPToHTTPSRedirect = userCfg.HTTPToHTTPSRedirect
	if userCfg.CertMode != "" {
		appCfg.CertMode = userCfg.CertMode
	}
	if len(userCfg.CertPaths.CACertPath) > 0 || len(userCfg.CertPaths.ServerCertPath) > 0 {
		appCfg.CertPaths = userCfg.CertPaths
	}
	if len(userCfg.CertNames) > 0 {
		appCfg.CertNames = userCfg.CertNames
	}

	// If we have an existing server config (for migration), preserve settings that aren't in the file
	if serverCfg != nil {
		// Only override if UserConfig has zero/empty values (backward compat)
		if userCfg.Port == 0 {
			appCfg.Port = serverCfg.Port
		}
		if userCfg.HTTPSPort == 0 {
			appCfg.HTTPSPort = serverCfg.HTTPSPort
		}
	}

	return appCfg
}
