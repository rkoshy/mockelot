package server

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"mockelot/models"
)

// CORSProcessor handles CORS header evaluation with JavaScript support
type CORSProcessor struct {
	config      *models.CORSConfig
	configMutex sync.RWMutex
}

// NewCORSProcessor creates a new CORS processor
func NewCORSProcessor(config *models.CORSConfig) *CORSProcessor {
	return &CORSProcessor{
		config: config,
	}
}

// UpdateConfig updates the CORS configuration
func (cp *CORSProcessor) UpdateConfig(config *models.CORSConfig) {
	cp.configMutex.Lock()
	defer cp.configMutex.Unlock()
	cp.config = config
}

// ProcessCORS evaluates CORS configuration and returns headers to set
func (cp *CORSProcessor) ProcessCORS(r *http.Request) map[string]string {
	cp.configMutex.RLock()
	config := cp.config
	cp.configMutex.RUnlock()

	if config == nil || !config.Enabled {
		return nil
	}

	headers := make(map[string]string)

	// Build request context for scripts
	reqContext := cp.buildRequestContext(r)

	// Determine mode (default to headers if not specified)
	mode := config.Mode
	if mode == "" {
		mode = models.CORSModeHeaders
	}

	switch mode {
	case models.CORSModeHeaders:
		// Evaluate each header expression
		for _, headerExpr := range config.HeaderExpressions {
			value, err := cp.evaluateHeaderExpression(headerExpr.Expression, reqContext)
			if err != nil {
				log.Printf("CORS header expression error for '%s': %v", headerExpr.Name, err)
				continue
			}
			if value != "" {
				headers[headerExpr.Name] = value
			}
		}

	case models.CORSModeScript:
		// Execute custom script
		scriptHeaders, err := cp.evaluateScript(config.Script, reqContext)
		if err != nil {
			log.Printf("CORS script execution error: %v", err)
			// Return empty headers on error
			return headers
		}
		headers = scriptHeaders
	}

	return headers
}

// buildRequestContext creates a request context object for CORS scripts
func (cp *CORSProcessor) buildRequestContext(r *http.Request) map[string]interface{} {
	return map[string]interface{}{
		"method":  r.Method,
		"path":    r.URL.Path,
		"origin":  r.Header.Get("Origin"),
		"headers": r.Header,
	}
}

// evaluateHeaderExpression evaluates a single header expression
func (cp *CORSProcessor) evaluateHeaderExpression(expression string, reqContext map[string]interface{}) (string, error) {
	// Create a new VM for this evaluation
	vm := goja.New()

	// Set request context
	vm.Set("request", reqContext)

	// Add helper functions
	cp.addHelperFunctions(vm, reqContext)

	// Execute expression with timeout
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("script panic: %v", r)
			}
		}()

		value, err := vm.RunString(expression)
		if err != nil {
			errChan <- err
			return
		}

		resultChan <- value.String()
	}()

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return "", err
	case <-time.After(1 * time.Second):
		return "", fmt.Errorf("header expression evaluation timeout")
	}
}

// evaluateScript evaluates a CORS script and returns the headers
func (cp *CORSProcessor) evaluateScript(script string, reqContext map[string]interface{}) (map[string]string, error) {
	// Create a new VM for execution
	vm := goja.New()

	// Set request context
	vm.Set("request", reqContext)

	// Add helper functions
	cp.addHelperFunctions(vm, reqContext)

	// Create headers object that script can populate
	headersObj := vm.NewObject()
	vm.Set("headers", headersObj)

	// Execute script with timeout
	resultChan := make(chan map[string]string, 1)
	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("script panic: %v", r)
			}
		}()

		_, err := vm.RunString(script)
		if err != nil {
			errChan <- err
			return
		}

		// Extract headers from the headers object
		headers := make(map[string]string)
		headersValue := vm.Get("headers")
		if headersValue != nil && !goja.IsUndefined(headersValue) && !goja.IsNull(headersValue) {
			obj := headersValue.ToObject(vm)
			for _, key := range obj.Keys() {
				value := obj.Get(key)
				if value != nil && !goja.IsUndefined(value) {
					headers[key] = value.String()
				}
			}
		}

		resultChan <- headers
	}()

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(2 * time.Second):
		return nil, fmt.Errorf("CORS script execution timeout")
	}
}

// addHelperFunctions adds helper functions to the VM
func (cp *CORSProcessor) addHelperFunctions(vm *goja.Runtime, reqContext map[string]interface{}) {
	// matchOrigin(pattern) - Check if origin matches pattern (supports wildcards)
	vm.Set("matchOrigin", func(pattern string) bool {
		origin, ok := reqContext["origin"].(string)
		if !ok || origin == "" {
			return false
		}

		// Simple wildcard matching
		if pattern == "*" {
			return true
		}

		// Exact match
		if pattern == origin {
			return true
		}

		// Wildcard prefix match (e.g., "https://*.example.com")
		if strings.Contains(pattern, "*") {
			// Convert wildcard pattern to simple matching
			parts := strings.Split(pattern, "*")
			if len(parts) == 2 {
				prefix := parts[0]
				suffix := parts[1]
				return strings.HasPrefix(origin, prefix) && strings.HasSuffix(origin, suffix)
			}
		}

		return false
	})

	// allowOrigins([...origins]) - Check if origin is in allowed list
	vm.Set("allowOrigins", func(call goja.FunctionCall) goja.Value {
		origin, ok := reqContext["origin"].(string)
		if !ok || origin == "" {
			return vm.ToValue(false)
		}

		// Check each allowed origin
		for _, arg := range call.Arguments {
			allowedOrigin := arg.String()
			if allowedOrigin == origin {
				return vm.ToValue(true)
			}
		}

		return vm.ToValue(false)
	})

	// getOrigin() - Get the request origin
	vm.Set("getOrigin", func() string {
		origin, ok := reqContext["origin"].(string)
		if ok {
			return origin
		}
		return ""
	})

	// getHeader(name) - Get a request header
	vm.Set("getHeader", func(name string) string {
		headers, ok := reqContext["headers"].(http.Header)
		if ok {
			return headers.Get(name)
		}
		return ""
	})
}

// ValidateScript validates a CORS script for syntax errors
func ValidateCORSScript(script string) error {
	vm := goja.New()
	// Try to compile the script by running it in a safe context
	_, err := vm.RunString(fmt.Sprintf("(function() { %s })", script))
	if err != nil {
		return fmt.Errorf("syntax error: %w", err)
	}
	return nil
}

// ValidateHeaderExpression validates a header expression for syntax errors
func ValidateHeaderExpression(expression string) error {
	vm := goja.New()

	// Provide mock request context for validation
	mockRequest := map[string]interface{}{
		"method":  "GET",
		"path":    "/",
		"origin":  "http://localhost:3000",
		"headers": http.Header{},
	}
	vm.Set("request", mockRequest)

	// Add helper functions
	processor := &CORSProcessor{}
	processor.addHelperFunctions(vm, mockRequest)

	// Try to compile and evaluate the expression
	_, err := vm.RunString(fmt.Sprintf("(function() { return %s; })()", expression))
	if err != nil {
		return fmt.Errorf("syntax error: %w", err)
	}
	return nil
}
