package server

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja"
	"mockelot/models"
)

// ValidationResult contains the result of request validation
type ValidationResult struct {
	Valid bool                   `json:"valid"`           // Whether validation passed
	Vars  map[string]interface{} `json:"vars,omitempty"`  // Extracted variables
	Error string                 `json:"error,omitempty"` // Error message if validation failed
}

// ValidateRequest validates the request body based on the validation config
// and extracts variables that can be used in the response
func ValidateRequest(validation *models.RequestValidation, body string, reqContext *RequestContext) *ValidationResult {
	// No validation configured - always valid
	if validation == nil || validation.Mode == "" || validation.Mode == models.ValidationModeNone {
		return &ValidationResult{Valid: true, Vars: make(map[string]interface{})}
	}

	switch validation.Mode {
	case models.ValidationModeStatic:
		return validateStatic(validation, body)
	case models.ValidationModeRegex:
		return validateRegex(validation, body)
	case models.ValidationModeScript:
		return validateScript(validation, body, reqContext)
	default:
		return &ValidationResult{Valid: true, Vars: make(map[string]interface{})}
	}
}

// validateStatic performs static text matching (exact or contains)
func validateStatic(validation *models.RequestValidation, body string) *ValidationResult {
	pattern := validation.Pattern
	matchType := validation.MatchType
	if matchType == "" {
		matchType = models.ValidationMatchContains // Default to contains
	}

	var valid bool
	switch matchType {
	case models.ValidationMatchExact:
		valid = body == pattern
	case models.ValidationMatchContains:
		valid = strings.Contains(body, pattern)
	default:
		valid = strings.Contains(body, pattern)
	}

	if !valid {
		return &ValidationResult{
			Valid: false,
			Error: fmt.Sprintf("body does not %s pattern", matchType),
		}
	}

	return &ValidationResult{Valid: true, Vars: make(map[string]interface{})}
}

// validateRegex performs regex matching with named group extraction
func validateRegex(validation *models.RequestValidation, body string) *ValidationResult {
	pattern := validation.Pattern
	if pattern == "" {
		return &ValidationResult{Valid: true, Vars: make(map[string]interface{})}
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Error: fmt.Sprintf("invalid regex pattern: %v", err),
		}
	}

	// Find match
	match := re.FindStringSubmatch(body)
	if match == nil {
		return &ValidationResult{
			Valid: false,
			Error: "body does not match regex pattern",
		}
	}

	// Extract named groups into vars
	vars := make(map[string]interface{})
	groupNames := re.SubexpNames()
	for i, name := range groupNames {
		if i > 0 && name != "" && i < len(match) {
			vars[name] = match[i]
		}
	}

	// Also add numbered groups for convenience
	for i, val := range match {
		if i > 0 {
			vars[fmt.Sprintf("$%d", i)] = val
		}
	}

	return &ValidationResult{Valid: true, Vars: vars}
}

// validateScript runs a JavaScript validation script that can extract variables
func validateScript(validation *models.RequestValidation, body string, reqContext *RequestContext) *ValidationResult {
	script := validation.Script
	if script == "" {
		return &ValidationResult{Valid: true, Vars: make(map[string]interface{})}
	}

	// Create a new JavaScript runtime
	vm := goja.New()

	// Set up timeout context (5 second limit)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Channel to receive result or error
	resultChan := make(chan *ValidationResult, 1)
	errChan := make(chan error, 1)

	go func() {
		result, err := runValidationScript(vm, script, body, reqContext)
		if err != nil {
			errChan <- err
		} else {
			resultChan <- result
		}
	}()

	// Wait for result or timeout
	select {
	case result := <-resultChan:
		return result
	case err := <-errChan:
		return &ValidationResult{
			Valid: false,
			Error: fmt.Sprintf("script error: %v", err),
		}
	case <-ctx.Done():
		vm.Interrupt("validation script timeout")
		return &ValidationResult{
			Valid: false,
			Error: "validation script timeout (5s limit)",
		}
	}
}

func runValidationScript(vm *goja.Runtime, script string, body string, reqContext *RequestContext) (*ValidationResult, error) {
	// Set up request object (same as response scripts)
	requestObj := reqContext.ToMap()
	if err := vm.Set("request", requestObj); err != nil {
		return nil, fmt.Errorf("failed to set request object: %v", err)
	}

	// Set up body directly for convenience
	if err := vm.Set("body", body); err != nil {
		return nil, fmt.Errorf("failed to set body: %v", err)
	}

	// Initialize result object
	result := map[string]interface{}{
		"valid": true,
		"vars":  make(map[string]interface{}),
		"error": "",
	}
	if err := vm.Set("result", result); err != nil {
		return nil, fmt.Errorf("failed to set result object: %v", err)
	}

	// Add console.log for debugging
	console := map[string]interface{}{
		"log":   func(args ...interface{}) {},
		"error": func(args ...interface{}) {},
		"warn":  func(args ...interface{}) {},
	}
	if err := vm.Set("console", console); err != nil {
		return nil, fmt.Errorf("failed to set console object: %v", err)
	}

	// Add JSON utility
	jsonUtil := map[string]interface{}{
		"stringify": func(v interface{}, args ...interface{}) string {
			b, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return string(b)
		},
		"parse": func(s string) interface{} {
			var v interface{}
			if err := json.Unmarshal([]byte(s), &v); err != nil {
				return nil
			}
			return v
		},
	}
	if err := vm.Set("JSON", jsonUtil); err != nil {
		return nil, fmt.Errorf("failed to set JSON object: %v", err)
	}

	// Execute the script
	_, err := vm.RunString(script)
	if err != nil {
		if jsErr, ok := err.(*goja.Exception); ok {
			return nil, fmt.Errorf(jsErr.String())
		}
		return nil, err
	}

	// Extract result from VM
	resultVal := vm.Get("result")
	if resultVal != nil && !goja.IsUndefined(resultVal) && !goja.IsNull(resultVal) {
		resultObj := resultVal.Export()
		if respMap, ok := resultObj.(map[string]interface{}); ok {
			validationResult := &ValidationResult{
				Valid: true,
				Vars:  make(map[string]interface{}),
			}

			// Extract valid
			if valid, ok := respMap["valid"].(bool); ok {
				validationResult.Valid = valid
			}

			// Extract vars
			if vars, ok := respMap["vars"].(map[string]interface{}); ok {
				validationResult.Vars = vars
			}

			// Extract error message
			if errMsg, ok := respMap["error"].(string); ok {
				validationResult.Error = errMsg
			}

			return validationResult, nil
		}
	}

	// Default to valid if result wasn't modified
	return &ValidationResult{Valid: true, Vars: make(map[string]interface{})}, nil
}
