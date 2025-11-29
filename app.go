package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
	"mockelot/models"
	"mockelot/openapi"
	"mockelot/server"
)

// ServerStatus represents the current state of the HTTP server
type ServerStatus struct {
	Running bool   `json:"running"`
	Port    int    `json:"port"`
	Error   string `json:"error,omitempty"`
}

// App struct
type App struct {
	ctx         context.Context
	server      *server.HTTPServer
	config      *models.AppConfig
	requestLogs []models.RequestLog
	logMutex    sync.RWMutex
	status      ServerStatus
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
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
		requestLogs: make([]models.RequestLog, 0),
		status: ServerStatus{
			Running: false,
			Port:    8080,
		},
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.server != nil {
		a.server.Stop()
	}
}

// StartServer starts the HTTP mock server on the specified port
func (a *App) StartServer(port int) error {
	if a.server != nil && a.status.Running {
		return fmt.Errorf("server is already running")
	}

	// Update config with the port
	a.config.Port = port

	a.server = server.NewHTTPServer(a.config, a)

	err := a.server.Start()
	if err != nil {
		a.status = ServerStatus{Running: false, Port: port, Error: err.Error()}
		runtime.EventsEmit(a.ctx, "server:status", a.status)
		return err
	}

	a.status = ServerStatus{Running: true, Port: port}
	runtime.EventsEmit(a.ctx, "server:status", a.status)
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
		runtime.EventsEmit(a.ctx, "server:status", a.status)
		return err
	}

	a.status = ServerStatus{Running: false, Port: a.status.Port}
	a.server = nil
	runtime.EventsEmit(a.ctx, "server:status", a.status)
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
	return a.config.Items
}

// SetItems replaces all response items
func (a *App) SetItems(items []models.ResponseItem) error {
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

	a.config.Items = items
	a.config.Responses = nil // Clear legacy responses when using items

	// If server is running, update it
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "items:updated", items)

	return nil
}

// AddGroup adds a new group
func (a *App) AddGroup(name string) (models.ResponseGroup, error) {
	group := models.ResponseGroup{
		ID:        uuid.New().String(),
		Name:      name,
		Responses: []models.MethodResponse{},
	}

	item := models.ResponseItem{
		Type:  "group",
		Group: &group,
	}

	a.config.Items = append(a.config.Items, item)
	a.config.Responses = nil // Clear legacy responses when using items

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

// SaveConfig saves the configuration to a YAML file
func (a *App) SaveConfig() error {
	// Open save dialog
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Configuration",
		DefaultFilename: "http-tester-config.yaml",
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

	// Save to YAML file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not create file: %v", err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(a.config)
}

// LoadConfig loads configuration from a YAML file
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

	var cfg models.AppConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("could not decode config: %v", err)
	}

	// Ensure all responses have IDs
	for i := range cfg.Responses {
		if cfg.Responses[i].ID == "" {
			cfg.Responses[i].ID = uuid.New().String()
		}
	}

	a.config = &cfg

	// Update server if running
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "responses:updated", cfg.Responses)

	return &cfg, nil
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

	if appendMode {
		// Append to existing items
		a.config.Items = append(a.config.Items, items...)
	} else {
		// Replace existing items
		a.config.Items = items
	}

	// Update server if running
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "items:updated", a.config.Items)

	return a.config, nil
}

// GetRequestLogs returns all request logs
func (a *App) GetRequestLogs() []models.RequestLog {
	a.logMutex.RLock()
	defer a.logMutex.RUnlock()

	// Return a copy
	logs := make([]models.RequestLog, len(a.requestLogs))
	copy(logs, a.requestLogs)
	return logs
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

// LogRequest implements the server.RequestLogger interface
func (a *App) LogRequest(log models.RequestLog) {
	a.logMutex.Lock()
	a.requestLogs = append(a.requestLogs, log)
	a.logMutex.Unlock()

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "request:received", log)
}
