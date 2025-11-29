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
}

// IsEnabled returns whether this response rule is enabled (defaults to true if not set)
func (r *MethodResponse) IsEnabled() bool {
	return r.Enabled == nil || *r.Enabled
}

// ResponseGroup represents a named group of response rules
type ResponseGroup struct {
	ID        string           `json:"id,omitempty" yaml:"id,omitempty"`               // Unique identifier for this group
	Name      string           `json:"name" yaml:"name"`                               // Display name for the group
	Expanded  *bool            `json:"expanded,omitempty" yaml:"expanded,omitempty"`   // Whether group is expanded in UI (default: true)
	Enabled   *bool            `json:"enabled,omitempty" yaml:"enabled,omitempty"`     // Whether all responses in group are enabled (default: true)
	Responses []MethodResponse `json:"responses,omitempty" yaml:"responses,omitempty"` // Responses within this group
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

// AppConfig stores the application's configuration
type AppConfig struct {
	Port         int              `json:"port" yaml:"port"`                                       // Server port
	Responses    []MethodResponse `json:"responses,omitempty" yaml:"responses,omitempty"`         // Legacy: flat response list (for backward compatibility)
	Items        []ResponseItem   `json:"items,omitempty" yaml:"items,omitempty"`                 // New: mixed list of responses and groups
	LastModified time.Time        `json:"last_modified,omitempty" yaml:"last_modified,omitempty"` // Last time configuration was modified
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

// RequestLog represents a detailed log of an incoming HTTP request
type RequestLog struct {
	ID            string              // Unique request identifier
	Timestamp     time.Time           // Time request was received
	Method        string              // HTTP method (GET, POST, etc.)
	Path          string              // Request path
	StatusCode    int                 // Response status code sent
	SourceIP      string              // Client IP address
	Headers       map[string][]string // Request headers
	Body          string              // Request body
	QueryParams   map[string][]string // Query parameters
	Protocol      string              // HTTP protocol version
	UserAgent     string              // Client user agent
}