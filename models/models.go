package models

import (
	"time"
)

// ResponseMode constants
const (
	ResponseModeStatic   = "static"   // Simple static response (default)
	ResponseModeTemplate = "template" // Go text/template with request context
	ResponseModeScript   = "script"   // JavaScript (goja) for complex logic
)

// ValidationMode constants
const (
	ValidationModeNone   = "none"   // No validation (default) - always match
	ValidationModeStatic = "static" // Static text match (exact or contains)
	ValidationModeRegex  = "regex"  // Regex match with named group extraction
	ValidationModeScript = "script" // JavaScript validation with variable extraction
)

// ValidationMatchType constants for static validation
const (
	ValidationMatchExact    = "exact"    // Body must exactly match pattern
	ValidationMatchContains = "contains" // Body must contain pattern
)

// CertMode constants for HTTPS certificate modes
const (
	CertModeAuto         = "auto"          // Auto-generate CA and server certs
	CertModeCAProvided   = "ca-provided"   // User provides CA cert + key
	CertModeCertProvided = "cert-provided" // User provides server cert + key + bundle
)

// CORSMode constants for CORS configuration modes
const (
	CORSModeHeaders = "headers" // Use header list with JavaScript expressions
	CORSModeScript  = "script"  // Use custom JavaScript script
)

// PathTranslationMode constants for endpoint path translation
const (
	TranslationModeNone      = "none"      // No translation - use path as-is
	TranslationModeStrip     = "strip"     // Strip prefix before matching
	TranslationModeTranslate = "translate" // Regex match/replace
)

// EndpointType constants for different endpoint behaviors
const (
	EndpointTypeMock      = "mock"      // Script-based mock responses
	EndpointTypeProxy     = "proxy"     // Reverse proxy with translation
	EndpointTypeContainer = "container" // Docker container management
)

// HeaderManipulation mode constants for proxy endpoints
const (
	HeaderModeDrop       = "drop"       // Drop the header
	HeaderModeReplace    = "replace"    // Replace with static value
	HeaderModeExpression = "expression" // JS expression for dynamic value
)

// RequestValidation defines how to validate and extract data from request body
type RequestValidation struct {
	Mode      string `json:"mode,omitempty" yaml:"mode,omitempty"`             // "none", "static", "regex", "script"
	Pattern   string `json:"pattern,omitempty" yaml:"pattern,omitempty"`       // Static text or regex pattern
	MatchType string `json:"match_type,omitempty" yaml:"match_type,omitempty"` // For static: "exact" or "contains"
	Script    string `json:"script,omitempty" yaml:"script,omitempty"`         // JavaScript validation script
}

// MethodResponse represents the configuration for a specific HTTP method's response
type MethodResponse struct {
	ID            string            `json:"id,omitempty" yaml:"id,omitempty"`                         // Unique identifier for this response rule
	Enabled       *bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`               // Whether this response is enabled (default: true)
	PathPattern   string            `json:"path_pattern" yaml:"path_pattern"`                         // Glob pattern like /api/*, regex like ^/api/v[0-9]+, or exact match
	Methods       []string          `json:"methods" yaml:"methods"`                                   // HTTP methods this response applies to (GET, POST, etc.)
	StatusCode    int               `json:"status_code" yaml:"status_code"`                           // HTTP response status code
	StatusText    string            `json:"status_text,omitempty" yaml:"status_text,omitempty"`       // Status text description
	Headers       map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`               // Response headers
	Body          string            `json:"body,omitempty" yaml:"body,omitempty"`                     // Response body (used for static and template modes)
	ResponseDelay int               `json:"response_delay,omitempty" yaml:"response_delay,omitempty"` // Delay in milliseconds before sending response
	ResponseMode       string             `json:"response_mode,omitempty" yaml:"response_mode,omitempty"`       // Response mode: "static", "template", or "script"
	ScriptBody         string             `json:"script_body,omitempty" yaml:"script_body,omitempty"`           // JavaScript code for script mode
	RequestValidation  *RequestValidation `json:"request_validation,omitempty" yaml:"request_validation,omitempty"` // Request body validation config
	UseGlobalCORS      *bool              `json:"use_global_cors,omitempty" yaml:"use_global_cors,omitempty"`   // Whether to use global CORS (nil=use group setting, true=use, false=disable)
}

// IsEnabled returns whether this response rule is enabled (defaults to true if not set)
func (r *MethodResponse) IsEnabled() bool {
	return r.Enabled == nil || *r.Enabled
}

// ResponseGroup represents a named group of response rules
type ResponseGroup struct {
	ID            string           `json:"id,omitempty" yaml:"id,omitempty"`                               // Unique identifier for this group
	Name          string           `json:"name" yaml:"name"`                                               // Display name for the group
	Expanded      *bool            `json:"expanded,omitempty" yaml:"expanded,omitempty"`                   // Whether group is expanded in UI (default: true)
	Enabled       *bool            `json:"enabled,omitempty" yaml:"enabled,omitempty"`                     // Whether all responses in group are enabled (default: true)
	UseGlobalCORS *bool            `json:"use_global_cors,omitempty" yaml:"use_global_cors,omitempty"`     // Whether to use global CORS (nil=enabled, true=use, false=disable)
	Responses     []MethodResponse `json:"responses,omitempty" yaml:"responses,omitempty"`                 // Responses within this group
}

// IsExpanded returns whether this group is expanded (defaults to true if not set)
func (g *ResponseGroup) IsExpanded() bool {
	return g.Expanded == nil || *g.Expanded
}

// IsEnabled returns whether this group is enabled (defaults to true if not set)
func (g *ResponseGroup) IsEnabled() bool {
	return g.Enabled == nil || *g.Enabled
}

// ResponseItem represents either a single response or a group of responses
// This allows mixing groups and individual responses in the same list
type ResponseItem struct {
	Type     string          `json:"type" yaml:"type"`                             // "response" or "group"
	Response *MethodResponse `json:"response,omitempty" yaml:"response,omitempty"` // Set if Type is "response"
	Group    *ResponseGroup  `json:"group,omitempty" yaml:"group,omitempty"`       // Set if Type is "group"
}

// HeaderManipulation defines how to modify a header (for proxy endpoints)
type HeaderManipulation struct {
	Name       string `json:"name" yaml:"name"`                                 // Header name
	Mode       string `json:"mode" yaml:"mode"`                                 // "drop", "replace", "expression"
	Value      string `json:"value,omitempty" yaml:"value,omitempty"`           // For replace mode
	Expression string `json:"expression,omitempty" yaml:"expression,omitempty"` // For expression mode (JS)
}

// StatusTranslation defines status code mapping (for proxy endpoints)
type StatusTranslation struct {
	FromPattern string `json:"from_pattern" yaml:"from_pattern"` // e.g., "5xx", "404", "2xx"
	ToCode      int    `json:"to_code" yaml:"to_code"`           // e.g., 403
}

// ProxyConfig contains reverse proxy configuration
type ProxyConfig struct {
	BackendURL       string                `json:"backend_url" yaml:"backend_url"`
	TimeoutSeconds   int                   `json:"timeout_seconds" yaml:"timeout_seconds"` // Default: 30

	// Path translation uses endpoint's TranslationMode, TranslatePattern, TranslateReplace

	// Header manipulation
	InboundHeaders  []HeaderManipulation `json:"inbound_headers,omitempty" yaml:"inbound_headers,omitempty"`
	OutboundHeaders []HeaderManipulation `json:"outbound_headers,omitempty" yaml:"outbound_headers,omitempty"`

	// Status code translation
	StatusPassthrough bool                `json:"status_passthrough" yaml:"status_passthrough"` // Default: true
	StatusTranslation []StatusTranslation `json:"status_translation,omitempty" yaml:"status_translation,omitempty"`

	// Body transformation
	BodyTransform string `json:"body_transform,omitempty" yaml:"body_transform,omitempty"` // JS script

	// Health check
	HealthCheckEnabled  bool   `json:"health_check_enabled" yaml:"health_check_enabled"`
	HealthCheckInterval int    `json:"health_check_interval" yaml:"health_check_interval"`         // Seconds, default: 30
	HealthCheckPath     string `json:"health_check_path,omitempty" yaml:"health_check_path,omitempty"` // Default: "/"
}

// DefaultContainerInboundHeaders returns the default inbound header manipulation rules for container endpoints.
// These rules ensure proper proxying to containers by:
// - Dropping hop-by-hop headers that should not be forwarded
// - Setting Host header to the container's dynamic port (127.0.0.1:PORT)
// - Adding X-Forwarded-* headers for proxy transparency
func DefaultContainerInboundHeaders() []HeaderManipulation {
	return []HeaderManipulation{
		// Drop hop-by-hop headers (RFC 7230 section 6.1)
		{Name: "Connection", Mode: HeaderModeDrop},
		{Name: "Keep-Alive", Mode: HeaderModeDrop},
		{Name: "Proxy-Authenticate", Mode: HeaderModeDrop},
		{Name: "Proxy-Authorization", Mode: HeaderModeDrop},
		{Name: "Te", Mode: HeaderModeDrop},
		{Name: "Trailers", Mode: HeaderModeDrop},
		{Name: "Transfer-Encoding", Mode: HeaderModeDrop},
		{Name: "Upgrade", Mode: HeaderModeDrop},

		// Set Host to container backend (127.0.0.1:DYNAMIC_PORT)
		// The hostPort variable is provided by ContainerHandler via customContext
		{Name: "Host", Mode: HeaderModeExpression, Expression: `"127.0.0.1:" + request.hostPort`},

		// Add X-Forwarded-* headers for proxy transparency
		{Name: "X-Forwarded-For", Mode: HeaderModeExpression, Expression: `request.remoteAddr`},
		{Name: "X-Forwarded-Host", Mode: HeaderModeExpression, Expression: `request.host`},
		{Name: "X-Forwarded-Proto", Mode: HeaderModeExpression, Expression: `request.scheme`},
	}
}

// VolumeMapping defines a volume mount (for container endpoints)
type VolumeMapping struct {
	HostPath      string `json:"host_path" yaml:"host_path"`           // Host directory or volume name
	ContainerPath string `json:"container_path" yaml:"container_path"` // Container mount point
	ReadOnly      bool   `json:"read_only" yaml:"read_only"`           // Default: false
}

// EnvironmentVar defines an environment variable (for container endpoints)
type EnvironmentVar struct {
	Name       string `json:"name" yaml:"name"`
	Value      string `json:"value,omitempty" yaml:"value,omitempty"`           // Static value
	Expression string `json:"expression,omitempty" yaml:"expression,omitempty"` // JS expression for dynamic value
}

// ContainerConfig contains Docker container configuration
// A container is a special type of proxy where the backend is a dynamically-started container.
// The ProxyConfig handles HTTP proxying (headers, status codes, health checks, etc.)
// Container-specific fields handle Docker/Podman management (image, volumes, environment, etc.)
type ContainerConfig struct {
	// Proxy configuration - handles HTTP proxying to the container
	// Note: BackendURL is not used for containers (dynamically set to http://127.0.0.1:{hostPort})
	// Note: Health check fields in ProxyConfig apply to the container's HTTP endpoint
	ProxyConfig ProxyConfig `json:"proxy_config" yaml:"proxy_config"`

	// Container image and startup
	ImageName     string   `json:"image_name" yaml:"image_name"`
	ContainerPort int      `json:"container_port" yaml:"container_port"`
	ExposedPorts  []string `json:"exposed_ports,omitempty" yaml:"exposed_ports,omitempty"` // Ports detected from image inspection (e.g., ["80/tcp", "443/tcp"])
	PullOnStartup bool     `json:"pull_on_startup" yaml:"pull_on_startup"`                 // Default: true
	RestartPolicy string   `json:"restart_policy,omitempty" yaml:"restart_policy,omitempty"` // "no", "always", "unless-stopped", "on-failure"

	// Port mapping (Mockelot forwards to container on this port)
	// The endpoint's PathPrefix determines routing, container receives on ContainerPort

	// Volume mappings
	Volumes []VolumeMapping `json:"volumes,omitempty" yaml:"volumes,omitempty"`

	// Environment variables
	Environment []EnvironmentVar `json:"environment,omitempty" yaml:"environment,omitempty"`

	// Special permissions
	HostNetworking     bool `json:"host_networking,omitempty" yaml:"host_networking,omitempty"`         // Use host network stack
	DockerSocketAccess bool `json:"docker_socket_access,omitempty" yaml:"docker_socket_access,omitempty"` // Mount Docker socket into container

	// Startup behavior
	RestartOnServerStart bool `json:"restart_on_server_start,omitempty" yaml:"restart_on_server_start,omitempty"` // Restart container if already running when server starts

	// Runtime state (not persisted)
	ContainerID string `json:"-" yaml:"-"` // Set when container is running
}

// HealthStatus represents health check state
type HealthStatus struct {
	EndpointID   string `json:"endpoint_id"`
	Healthy      bool   `json:"healthy"`
	LastCheck    string `json:"last_check"` // ISO8601/RFC3339 formatted timestamp
	ErrorMessage string `json:"error_message,omitempty"`
}

// ContainerStatus represents the runtime state of a container (separate from health checks)
type ContainerStatus struct {
	EndpointID  string `json:"endpoint_id"`
	ContainerID string `json:"container_id"` // Docker/Podman container ID
	Running     bool   `json:"running"`
	Status      string `json:"status"` // "running", "exited", "dead", "not started", "gone"
	Gone        bool   `json:"gone"`   // true if container doesn't exist (not found)
	LastCheck   string `json:"last_check"` // ISO8601/RFC3339 formatted timestamp
}

// ContainerStartProgress represents a startup progress event
type ContainerStartProgress struct {
	EndpointID string `json:"endpoint_id"`
	Stage      string `json:"stage"`    // "pulling", "creating", "starting", "ready", "error"
	Message    string `json:"message"`
	Progress   int    `json:"progress"` // 0-100 percentage
}

// ContainerStats represents real-time container resource usage metrics
type ContainerStats struct {
	EndpointID      string  `json:"endpoint_id"`
	CPUPercent      float64 `json:"cpu_percent"`       // CPU usage percentage (0-100+)
	MemoryUsageMB   float64 `json:"memory_usage_mb"`   // Memory usage in MB
	MemoryLimitMB   float64 `json:"memory_limit_mb"`   // Memory limit in MB (0 if unlimited)
	MemoryPercent   float64 `json:"memory_percent"`    // Memory usage percentage
	NetworkRxBytes  uint64  `json:"network_rx_bytes"`  // Network bytes received
	NetworkTxBytes  uint64  `json:"network_tx_bytes"`  // Network bytes transmitted
	BlockReadBytes  uint64  `json:"block_read_bytes"`  // Block I/O bytes read
	BlockWriteBytes uint64  `json:"block_write_bytes"` // Block I/O bytes written
	PIDs            uint64  `json:"pids"`              // Number of processes
	LastCheck       string  `json:"last_check"`        // ISO8601/RFC3339 formatted timestamp
}

// Endpoint represents a top-level container for response rules with path prefix and translation
type Endpoint struct {
	ID               string         `json:"id" yaml:"id"`                                                   // Unique identifier
	Name             string         `json:"name" yaml:"name"`                                               // Display name
	PathPrefix       string         `json:"path_prefix" yaml:"path_prefix"`                                 // Path prefix to match (e.g., "/api/v1")
	TranslationMode  string         `json:"translation_mode" yaml:"translation_mode"`                       // Translation mode: "none", "strip", "translate"
	TranslatePattern string         `json:"translate_pattern,omitempty" yaml:"translate_pattern,omitempty"` // Regex pattern for translate mode
	TranslateReplace string         `json:"translate_replace,omitempty" yaml:"translate_replace,omitempty"` // Replacement for translate mode
	Enabled          *bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`                     // Whether endpoint is enabled (default: true)

	// Endpoint type and type-specific configurations
	Type            string           `json:"type" yaml:"type"`                                         // "mock", "proxy", "container"
	Items           []ResponseItem   `json:"items,omitempty" yaml:"items,omitempty"`                   // For mock type only
	ProxyConfig     *ProxyConfig     `json:"proxy_config,omitempty" yaml:"proxy_config,omitempty"`     // For proxy type
	ContainerConfig *ContainerConfig `json:"container_config,omitempty" yaml:"container_config,omitempty"` // For container type
}

// IsEnabled returns whether this endpoint is enabled (defaults to true if not set)
func (e *Endpoint) IsEnabled() bool {
	return e.Enabled == nil || *e.Enabled
}

// CORSHeader represents a single CORS header with JavaScript expression
type CORSHeader struct {
	Name       string `json:"name" yaml:"name"`               // Header name (e.g., "Access-Control-Allow-Origin")
	Expression string `json:"expression" yaml:"expression"`   // JavaScript expression to evaluate
}

// CORSConfig stores global CORS configuration
type CORSConfig struct {
	Enabled              bool         `json:"enabled" yaml:"enabled"`                                             // Whether global CORS is enabled
	Mode                 string       `json:"mode,omitempty" yaml:"mode,omitempty"`                               // "headers" or "script"
	HeaderExpressions    []CORSHeader `json:"header_expressions,omitempty" yaml:"header_expressions,omitempty"`   // Header list mode: headers with JS expressions
	Script               string       `json:"script,omitempty" yaml:"script,omitempty"`                           // Script mode: custom JavaScript
	OptionsDefaultStatus int          `json:"options_default_status,omitempty" yaml:"options_default_status,omitempty"` // Default status for OPTIONS (200 or 204)
}

// CACertInfo contains information about the CA certificate
type CACertInfo struct {
	Exists    bool   `json:"exists"`              // Whether CA cert exists
	Generated string `json:"generated,omitempty"` // When CA was generated (ISO8601/RFC3339 format)
}

// CertPaths contains file paths for user-provided certificates
type CertPaths struct {
	CACertPath       string `json:"ca_cert_path,omitempty"`
	CAKeyPath        string `json:"ca_key_path,omitempty"`
	ServerCertPath   string `json:"server_cert_path,omitempty"`
	ServerKeyPath    string `json:"server_key_path,omitempty"`
	ServerBundlePath string `json:"server_bundle_path,omitempty"`
}

// ServerConfig stores server-level settings (auto-saved to ~/.mockelot/server-config.yaml)
type ServerConfig struct {
	// HTTP Server
	Port int `json:"port" yaml:"port"` // HTTP server port

	// HTTP/2 Support
	HTTP2Enabled bool `json:"http2_enabled,omitempty" yaml:"http2_enabled,omitempty"` // Whether HTTP/2 is enabled for both HTTP and HTTPS servers

	// HTTPS Configuration
	HTTPSEnabled        bool      `json:"https_enabled,omitempty" yaml:"https_enabled,omitempty"`                       // Whether HTTPS is enabled
	HTTPSPort           int       `json:"https_port,omitempty" yaml:"https_port,omitempty"`                             // HTTPS server port
	HTTPToHTTPSRedirect bool      `json:"http_to_https_redirect,omitempty" yaml:"http_to_https_redirect,omitempty"`     // Whether to redirect HTTP to HTTPS
	CertMode            string    `json:"cert_mode,omitempty" yaml:"cert_mode,omitempty"`                               // Certificate mode: "auto", "ca-provided", "cert-provided"
	CertPaths           CertPaths `json:"cert_paths,omitempty" yaml:"cert_paths,omitempty"`                             // Paths to user-provided certificates
	CertNames           []string  `json:"cert_names,omitempty" yaml:"cert_names,omitempty"`                             // Custom DNS names and IP addresses for certificate (CN/SAN)

	// CORS Configuration
	CORS CORSConfig `json:"cors,omitempty" yaml:"cors,omitempty"` // Global CORS configuration

	// UI State
	SelectedEndpointId string `json:"selected_endpoint_id,omitempty" yaml:"selected_endpoint_id,omitempty"` // Currently selected endpoint in UI

	LastModified time.Time `json:"last_modified,omitempty" yaml:"last_modified,omitempty"` // Last time configuration was modified
}

// UserConfig stores user-defined request processing rules (manual save/load)
type UserConfig struct {
	Responses    []MethodResponse `json:"responses,omitempty" yaml:"responses,omitempty"` // Legacy: flat response list (for backward compatibility)
	Items        []ResponseItem   `json:"items,omitempty" yaml:"items,omitempty"`         // New: mixed list of responses and groups (legacy app-level)
	Endpoints    []Endpoint       `json:"endpoints,omitempty" yaml:"endpoints,omitempty"` // Current: all endpoints (mock, proxy, container)
	CORS         CORSConfig       `json:"cors,omitempty" yaml:"cors,omitempty"`           // Global CORS configuration
	LastModified time.Time        `json:"last_modified,omitempty" yaml:"last_modified,omitempty"` // Last time configuration was modified
}

// GetAllResponses returns all enabled responses in priority order (flattened from items and legacy responses)
func (c *UserConfig) GetAllResponses() []MethodResponse {
	var result []MethodResponse

	// First, process items (new format)
	for _, item := range c.Items {
		switch item.Type {
		case "response":
			if item.Response != nil {
				result = append(result, *item.Response)
			}
		case "group":
			if item.Group != nil && item.Group.IsEnabled() {
				result = append(result, item.Group.Responses...)
			}
		}
	}

	// Then, process legacy responses (if no items exist)
	if len(c.Items) == 0 {
		result = append(result, c.Responses...)
	}

	return result
}

// AppConfig stores the application's configuration (DEPRECATED - split into ServerConfig and UserConfig)
// Kept for backward compatibility with existing code
type AppConfig struct {
	// HTTP Server
	Port         int              `json:"port" yaml:"port"`                                       // HTTP server port
	Responses    []MethodResponse `json:"responses,omitempty" yaml:"responses,omitempty"`         // Legacy: flat response list (for backward compatibility)
	Items        []ResponseItem   `json:"items,omitempty" yaml:"items,omitempty"`                 // Legacy: mixed list of responses and groups (pre-endpoint)
	Endpoints    []Endpoint       `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`         // New: endpoint-based organization
	LastModified time.Time        `json:"last_modified,omitempty" yaml:"last_modified,omitempty"` // Last time configuration was modified

	// HTTP/2 Support
	HTTP2Enabled bool `json:"http2_enabled,omitempty" yaml:"http2_enabled,omitempty"` // Whether HTTP/2 is enabled for both HTTP and HTTPS servers

	// HTTPS Configuration
	HTTPSEnabled        bool      `json:"https_enabled,omitempty" yaml:"https_enabled,omitempty"`                       // Whether HTTPS is enabled
	HTTPSPort           int       `json:"https_port,omitempty" yaml:"https_port,omitempty"`                             // HTTPS server port
	HTTPToHTTPSRedirect bool      `json:"http_to_https_redirect,omitempty" yaml:"http_to_https_redirect,omitempty"`     // Whether to redirect HTTP to HTTPS
	CertMode            string    `json:"cert_mode,omitempty" yaml:"cert_mode,omitempty"`                               // Certificate mode: "auto", "ca-provided", "cert-provided"
	CertPaths           CertPaths `json:"cert_paths,omitempty" yaml:"cert_paths,omitempty"`                             // Paths to user-provided certificates
	CertNames           []string  `json:"cert_names,omitempty" yaml:"cert_names,omitempty"`                             // Custom DNS names and IP addresses for certificate (CN/SAN)

	// CORS Configuration
	CORS CORSConfig `json:"cors,omitempty" yaml:"cors,omitempty"` // Global CORS configuration

	// Container Configuration
	ContainerLogLineLimit int `json:"container_log_line_limit,omitempty" yaml:"container_log_line_limit,omitempty"` // Max number of log lines to retrieve (default 5000)
}

// GetAllResponses returns all enabled responses in priority order (flattened from items and legacy responses)
func (c *AppConfig) GetAllResponses() []MethodResponse {
	var result []MethodResponse

	// First, process items (new format)
	for _, item := range c.Items {
		switch item.Type {
		case "response":
			if item.Response != nil {
				result = append(result, *item.Response)
			}
		case "group":
			if item.Group != nil && item.Group.IsEnabled() {
				result = append(result, item.Group.Responses...)
			}
		}
	}

	// Then, process legacy responses (if no items exist)
	if len(c.Items) == 0 {
		result = append(result, c.Responses...)
	}

	return result
}

// RequestLogSummary represents a lightweight summary of a request for efficient UI display
// Full details can be fetched on-demand using GetRequestLogDetails(id)
type RequestLogSummary struct {
	ID             string `json:"id"`                    // Unique request identifier
	Timestamp      string `json:"timestamp"`             // Time request was received (ISO8601/RFC3339 format)
	EndpointID     string `json:"endpoint_id,omitempty"` // ID of endpoint that handled this request
	Method         string `json:"method"`                // HTTP method
	Path           string `json:"path"`                  // Request path
	SourceIP       string `json:"source_ip"`             // Client IP address
	ClientStatus   int    `json:"client_status"`         // Client response status code
	BackendStatus  int    `json:"backend_status"`        // Backend response status code (0 if no backend)
	ClientRTT      *int64 `json:"client_rtt,omitempty"`  // Client round-trip time (ms), nil if not measured
	BackendRTT     *int64 `json:"backend_rtt,omitempty"` // Backend round-trip time (ms), nil if no backend
	HasBackend     bool   `json:"has_backend"`           // Whether this request involved a backend call
	ClientBodySize int    `json:"client_body_size"`      // Size of client request body in bytes
	Pending        bool   `json:"pending"`               // Whether this request is still in progress (no response yet)
}

// RequestLog represents a detailed log of an incoming HTTP request and response
// with dual-sided tracking for proxy/container endpoints (client↔server and server↔backend)
type RequestLog struct {
	ID         string `json:"id"`                    // Unique request identifier
	Timestamp  string `json:"timestamp"`             // Time request was received (ISO8601/RFC3339 format)
	EndpointID string `json:"endpoint_id,omitempty"` // ID of endpoint that handled this request

	// Client side: Client → Server
	ClientRequest struct {
		Method      string              `json:"method"`                 // HTTP method (GET, POST, etc.)
		FullURL     string              `json:"full_url"`               // Full URL as seen by client (e.g., http://localhost:8080/api/users?page=1)
		Path        string              `json:"path"`                   // Request path
		QueryParams map[string][]string `json:"query_params,omitempty"` // Query parameters
		Headers     map[string][]string `json:"headers,omitempty"`      // Request headers
		Body        string              `json:"body,omitempty"`         // Request body
		Protocol    string              `json:"protocol,omitempty"`     // HTTP protocol version (HTTP/1.1, HTTP/2)
		SourceIP    string              `json:"source_ip"`              // Client IP address
		UserAgent   string              `json:"user_agent,omitempty"`   // Client user agent
	} `json:"client_request"`

	// Client side: Server → Client
	ClientResponse struct {
		StatusCode int                 `json:"status_code"`              // Response status code sent to client
		StatusText string              `json:"status_text,omitempty"`    // Status text (e.g., "OK", "Not Found")
		Headers    map[string][]string `json:"headers,omitempty"`        // Response headers sent to client
		Body       string              `json:"body,omitempty"`           // Response body sent to client
		DelayMs    *int64              `json:"delay_ms,omitempty"`       // Time from request to first byte of response (ms), nil if not measured
		RTTMs      *int64              `json:"rtt_ms,omitempty"`         // Total round-trip time including body streaming (ms), nil if not measured
	} `json:"client_response"`

	// Backend side: Server → Backend (only for proxy/container endpoints)
	BackendRequest *struct {
		Method      string              `json:"method"`                 // HTTP method sent to backend
		FullURL     string              `json:"full_url"`               // Full backend URL (e.g., https://api.example.com/v1/users?page=1)
		Path        string              `json:"path"`                   // Backend request path
		QueryParams map[string][]string `json:"query_params,omitempty"` // Backend query parameters
		Headers     map[string][]string `json:"headers,omitempty"`      // Headers sent to backend
		Body        string              `json:"body,omitempty"`         // Body sent to backend
	} `json:"backend_request,omitempty"`

	// Backend side: Backend → Server (only for proxy/container endpoints)
	BackendResponse *struct {
		StatusCode int                 `json:"status_code"`           // Backend response status code
		StatusText string              `json:"status_text,omitempty"` // Backend status text
		Headers    map[string][]string `json:"headers,omitempty"`     // Headers received from backend
		Body       string              `json:"body,omitempty"`        // Body received from backend
		DelayMs    *int64              `json:"delay_ms,omitempty"`    // Time from backend request to first byte (ms), nil if not measured
		RTTMs      *int64              `json:"rtt_ms,omitempty"`      // Backend round-trip time (ms), nil if not measured
	} `json:"backend_response,omitempty"`
}

// DockerImageInfo contains metadata extracted from Docker image inspection
type DockerImageInfo struct {
	ImageName    string            `json:"image_name"`              // Full image name with tag
	ExposedPorts []string          `json:"exposed_ports"`           // Exposed ports from image (e.g., ["80/tcp", "443/tcp"])
	Volumes      []string          `json:"volumes"`                 // Volume mount points defined in image (e.g., ["/data", "/config"])
	Environment  map[string]string `json:"environment"`             // Environment variables from image (ENV directives)
	WorkingDir   string            `json:"working_dir,omitempty"`   // Working directory (WORKDIR)
	Entrypoint   []string          `json:"entrypoint,omitempty"`    // Entrypoint command
	Cmd          []string          `json:"cmd,omitempty"`           // Default command
	Labels       map[string]string `json:"labels,omitempty"`        // Image labels
	SuggestedHealthCheckPath string `json:"suggested_health_check_path,omitempty"` // Auto-detected health check path
	IsHTTPService bool             `json:"is_http_service"`         // Whether this appears to be an HTTP service
}

// RecentFile represents a recently opened/saved configuration file
type RecentFile struct {
	Path         string    `json:"path"`           // Absolute path to the file
	LastAccessed time.Time `json:"last_accessed"`  // Last time file was opened or saved
	Exists       bool      `json:"exists"`         // Whether file currently exists on disk
}

// RecentFiles contains the list of recent configuration files
type RecentFiles struct {
	Files []RecentFile `json:"files"`
}