package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sync"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
	"mockelot/config"
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
	ctx               context.Context
	server            *server.HTTPServer
	config            *models.AppConfig
	serverConfigMgr   *config.ServerConfigManager
	requestLogs       []models.RequestLog
	logMutex          sync.RWMutex
	status            ServerStatus
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
		serverConfigMgr: config.NewServerConfigManager(""),
		requestLogs:     make([]models.RequestLog, 0),
		status: ServerStatus{
			Running: false,
			Port:    8080,
		},
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load server configuration
	serverCfg, err := a.serverConfigMgr.Load()
	if err != nil {
		// Log error but continue with defaults
		fmt.Printf("Failed to load server config, using defaults: %v\n", err)
	} else {
		// Apply server config to app config
		a.config.Port = serverCfg.Port
		a.config.HTTP2Enabled = serverCfg.HTTP2Enabled
		a.config.HTTPSEnabled = serverCfg.HTTPSEnabled
		a.config.HTTPSPort = serverCfg.HTTPSPort
		a.config.HTTPToHTTPSRedirect = serverCfg.HTTPToHTTPSRedirect
		a.config.CertMode = serverCfg.CertMode
		a.config.CertPaths = serverCfg.CertPaths
		a.config.CertNames = serverCfg.CertNames
		a.config.CORS = serverCfg.CORS
		a.status.Port = serverCfg.Port
	}
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

// SaveConfig saves the user configuration (request processing rules + CORS) to a YAML file
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

	// Create UserConfig with only request processing rules and CORS
	userConfig := &models.UserConfig{
		Responses: a.config.Responses,
		Items:     a.config.Items,
		CORS:      a.config.CORS,
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

	// Update only the request processing rules and CORS (preserve server settings)
	a.config.Responses = userCfg.Responses
	a.config.Items = userCfg.Items
	a.config.CORS = userCfg.CORS

	// Update server if running
	if a.server != nil {
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "responses:updated", userCfg.Responses)
	runtime.EventsEmit(a.ctx, "items:updated", userCfg.Items)

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
			info.Generated = timestamp
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

// SetHTTPSConfig updates HTTPS configuration
func (a *App) SetHTTPSConfig(enabled bool, port int, redirect bool) error {
	// Update config
	a.config.HTTPSEnabled = enabled
	a.config.HTTPSPort = port
	a.config.HTTPToHTTPSRedirect = redirect

	// Auto-save server config
	serverCfg := &models.ServerConfig{
		Port:                a.config.Port,
		HTTP2Enabled:        a.config.HTTP2Enabled,
		HTTPSEnabled:        a.config.HTTPSEnabled,
		HTTPSPort:           a.config.HTTPSPort,
		HTTPToHTTPSRedirect: a.config.HTTPToHTTPSRedirect,
		CertMode:            a.config.CertMode,
		CertPaths:           a.config.CertPaths,
		CertNames:           a.config.CertNames,
		CORS:                a.config.CORS,
	}
	if err := a.serverConfigMgr.Save(serverCfg); err != nil {
		fmt.Printf("Warning: failed to save server config: %v\n", err)
	}

	// If server is running, apply changes
	if a.server != nil && a.status.Running {
		// Update server config
		a.server.UpdateConfig(a.config)

		// If HTTPS is now enabled and wasn't before, start HTTPS server
		if enabled {
			err := a.server.StartHTTPS()
			if err != nil {
				return fmt.Errorf("failed to start HTTPS server: %w", err)
			}
		} else {
			// If HTTPS is now disabled, stop HTTPS server
			err := a.server.StopHTTPS()
			if err != nil {
				return fmt.Errorf("failed to stop HTTPS server: %w", err)
			}
		}

		// Restart HTTP server to apply redirect changes
		err := a.server.StopHTTP()
		if err != nil {
			return fmt.Errorf("failed to stop HTTP server: %w", err)
		}
		err = a.server.StartHTTP()
		if err != nil {
			return fmt.Errorf("failed to restart HTTP server: %w", err)
		}
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "https:config-updated", nil)

	return nil
}

// SetCertMode updates the certificate mode, paths, and custom cert names
func (a *App) SetCertMode(mode string, certPaths models.CertPaths, certNames []string) error {
	// Validate certificate mode
	if mode != models.CertModeAuto && mode != models.CertModeCAProvided && mode != models.CertModeCertProvided {
		return fmt.Errorf("invalid certificate mode: %s", mode)
	}

	// Update config
	a.config.CertMode = mode
	a.config.CertPaths = certPaths
	a.config.CertNames = certNames

	// Auto-save server config
	serverCfg := &models.ServerConfig{
		Port:                a.config.Port,
		HTTP2Enabled:        a.config.HTTP2Enabled,
		HTTPSEnabled:        a.config.HTTPSEnabled,
		HTTPSPort:           a.config.HTTPSPort,
		HTTPToHTTPSRedirect: a.config.HTTPToHTTPSRedirect,
		CertMode:            a.config.CertMode,
		CertPaths:           a.config.CertPaths,
		CertNames:           a.config.CertNames,
		CORS:                a.config.CORS,
	}
	if err := a.serverConfigMgr.Save(serverCfg); err != nil {
		fmt.Printf("Warning: failed to save server config: %v\n", err)
	}

	// If server is running and HTTPS is enabled, restart HTTPS server
	if a.server != nil && a.status.Running && a.config.HTTPSEnabled {
		err := a.server.RestartHTTPS()
		if err != nil {
			return fmt.Errorf("failed to restart HTTPS server: %w", err)
		}
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "cert:mode-updated", nil)

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

// SetCORSConfig updates the CORS configuration
func (a *App) SetCORSConfig(corsConfig models.CORSConfig) error {
	// Update config
	a.config.CORS = corsConfig

	// Auto-save server config
	serverCfg := &models.ServerConfig{
		Port:                a.config.Port,
		HTTP2Enabled:        a.config.HTTP2Enabled,
		HTTPSEnabled:        a.config.HTTPSEnabled,
		HTTPSPort:           a.config.HTTPSPort,
		HTTPToHTTPSRedirect: a.config.HTTPToHTTPSRedirect,
		CertMode:            a.config.CertMode,
		CertPaths:           a.config.CertPaths,
		CertNames:           a.config.CertNames,
		CORS:                a.config.CORS,
	}
	if err := a.serverConfigMgr.Save(serverCfg); err != nil {
		fmt.Printf("Warning: failed to save server config: %v\n", err)
	}

	// If server is running, update CORS processor
	if a.server != nil {
		// The server's response handler will use the updated config
		a.server.UpdateConfig(a.config)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "cors:config-updated", nil)

	return nil
}

// ValidateCORSScript validates a CORS script for syntax errors
func (a *App) ValidateCORSScript(script string) error {
	return server.ValidateCORSScript(script)
}

// SetHTTP2Enabled enables or disables HTTP/2 support for both HTTP and HTTPS servers
func (a *App) SetHTTP2Enabled(enabled bool) error {
	// Update config
	a.config.HTTP2Enabled = enabled

	// Auto-save server config
	serverCfg := &models.ServerConfig{
		Port:                a.config.Port,
		HTTP2Enabled:        a.config.HTTP2Enabled,
		HTTPSEnabled:        a.config.HTTPSEnabled,
		HTTPSPort:           a.config.HTTPSPort,
		HTTPToHTTPSRedirect: a.config.HTTPToHTTPSRedirect,
		CertMode:            a.config.CertMode,
		CertPaths:           a.config.CertPaths,
		CertNames:           a.config.CertNames,
		CORS:                a.config.CORS,
	}
	if err := a.serverConfigMgr.Save(serverCfg); err != nil {
		fmt.Printf("Warning: failed to save server config: %v\n", err)
	}

	// If server is running, restart both servers to apply HTTP/2 changes
	if a.server != nil {
		// Stop both servers
		if err := a.server.Stop(); err != nil {
			return fmt.Errorf("failed to stop servers: %w", err)
		}

		// Start both servers with new HTTP/2 setting
		if err := a.server.Start(); err != nil {
			return fmt.Errorf("failed to restart servers: %w", err)
		}
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "http2:config-updated", nil)

	return nil
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

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "request:received", log)
}
