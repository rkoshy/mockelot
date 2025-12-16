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

// ValidateRequest validates the request body and headers based on the validation config
// and extracts variables that can be used in the response
func ValidateRequest(validation *models.RequestValidation, body string, reqContext *RequestContext) *ValidationResult {
	// No validation configured - always valid
	if validation == nil {
		return &ValidationResult{Valid: true, Vars: make(map[string]interface{})}
	}

	// Validate body first
	bodyResult := &ValidationResult{Valid: true, Vars: make(map[string]interface{})}
	if validation.Mode != "" && validation.Mode != models.ValidationModeNone {
		switch validation.Mode {
		case models.ValidationModeStatic:
			bodyResult = validateStatic(validation, body)
		case models.ValidationModeRegex:
			bodyResult = validateRegex(validation, body)
		case models.ValidationModeScript:
			bodyResult = validateScript(validation, body, reqContext)
		default:
			bodyResult = &ValidationResult{Valid: true, Vars: make(map[string]interface{})}
		}

		// If body validation failed, return immediately
		if !bodyResult.Valid {
			return bodyResult
		}
	}

	// Validate headers (AND logic with body validation)
	if len(validation.Headers) > 0 {
		headerResult := validateHeaders(validation.Headers, reqContext)
		if !headerResult.Valid {
			return headerResult
		}

		// Merge variables from body and header validation
		for k, v := range headerResult.Vars {
			bodyResult.Vars[k] = v
		}
	}

	return bodyResult
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

// validateHeaders validates request headers based on header validation rules
func validateHeaders(headers []models.HeaderValidation, reqContext *RequestContext) *ValidationResult {
	if len(headers) == 0 {
		return &ValidationResult{Valid: true, Vars: make(map[string]interface{})}
	}

	vars := make(map[string]interface{})

	// Validate each header (AND logic - all must pass)
	for _, headerVal := range headers {
		// Get header value from request (case-insensitive)
		headerValue := ""
		for key, values := range reqContext.Headers {
			if strings.EqualFold(key, headerVal.Name) {
				if len(values) > 0 {
					headerValue = values[0]
				}
				break
			}
		}

		// Check if header is required but missing
		if headerVal.Required && headerValue == "" {
			return &ValidationResult{
				Valid: false,
				Error: fmt.Sprintf("required header '%s' is missing", headerVal.Name),
			}
		}

		// If header is not required and missing, skip validation
		if headerValue == "" {
			continue
		}

		// Validate based on mode
		mode := headerVal.Mode
		if mode == "" || mode == models.HeaderValidationModeNone {
			continue // No validation for this header
		}

		switch mode {
		case models.HeaderValidationModeExact:
			if headerValue != headerVal.Value {
				return &ValidationResult{
					Valid: false,
					Error: fmt.Sprintf("header '%s' value '%s' does not exactly match expected value '%s'",
						headerVal.Name, headerValue, headerVal.Value),
				}
			}

		case models.HeaderValidationModeContains:
			if !strings.Contains(headerValue, headerVal.Value) {
				return &ValidationResult{
					Valid: false,
					Error: fmt.Sprintf("header '%s' value '%s' does not contain expected substring '%s'",
						headerVal.Name, headerValue, headerVal.Value),
				}
			}

		case models.HeaderValidationModeRegex:
			if headerVal.Pattern == "" {
				continue
			}
			re, err := regexp.Compile(headerVal.Pattern)
			if err != nil {
				return &ValidationResult{
					Valid: false,
					Error: fmt.Sprintf("invalid regex pattern for header '%s': %v", headerVal.Name, err),
				}
			}

			match := re.FindStringSubmatch(headerValue)
			if match == nil {
				return &ValidationResult{
					Valid: false,
					Error: fmt.Sprintf("header '%s' value '%s' does not match regex pattern '%s'",
						headerVal.Name, headerValue, headerVal.Pattern),
				}
			}

			// Extract named groups into vars (prefixed with header name to avoid conflicts)
			groupNames := re.SubexpNames()
			for i, name := range groupNames {
				if i > 0 && name != "" && i < len(match) {
					varName := fmt.Sprintf("%s_%s", headerVal.Name, name)
					vars[varName] = match[i]
				}
			}

		case models.HeaderValidationModeScript:
			if headerVal.Expression == "" {
				continue
			}

			// Create a new JavaScript runtime for header validation
			vm := goja.New()

			// Set up timeout context (5 second limit)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Channel to receive result or error
			resultChan := make(chan bool, 1)
			errChan := make(chan error, 1)

			go func() {
				valid, err := runHeaderValidationScript(vm, headerVal.Expression, headerValue, headerVal.Name, reqContext)
				if err != nil {
					errChan <- err
				} else {
					resultChan <- valid
				}
			}()

			// Wait for result or timeout
			select {
			case valid := <-resultChan:
				if !valid {
					return &ValidationResult{
						Valid: false,
						Error: fmt.Sprintf("header '%s' failed script validation", headerVal.Name),
					}
				}
			case err := <-errChan:
				return &ValidationResult{
					Valid: false,
					Error: fmt.Sprintf("header '%s' script error: %v", headerVal.Name, err),
				}
			case <-ctx.Done():
				vm.Interrupt("header validation script timeout")
				return &ValidationResult{
					Valid: false,
					Error: fmt.Sprintf("header '%s' validation script timeout (5s limit)", headerVal.Name),
				}
			}
		}
	}

	return &ValidationResult{Valid: true, Vars: vars}
}

// runHeaderValidationScript executes a JavaScript expression to validate a header value
func runHeaderValidationScript(vm *goja.Runtime, expression string, headerValue string, headerName string, reqContext *RequestContext) (bool, error) {
	// Set up request object
	requestObj := reqContext.ToMap()
	if err := vm.Set("request", requestObj); err != nil {
		return false, fmt.Errorf("failed to set request object: %v", err)
	}

	// Set header value for convenience
	if err := vm.Set("headerValue", headerValue); err != nil {
		return false, fmt.Errorf("failed to set headerValue: %v", err)
	}

	// Set header name for context
	if err := vm.Set("headerName", headerName); err != nil {
		return false, fmt.Errorf("failed to set headerName: %v", err)
	}

	// Add console.log for debugging
	console := map[string]interface{}{
		"log":   func(args ...interface{}) {},
		"error": func(args ...interface{}) {},
		"warn":  func(args ...interface{}) {},
	}
	if err := vm.Set("console", console); err != nil {
		return false, fmt.Errorf("failed to set console object: %v", err)
	}

	// Execute the expression (should return a boolean)
	result, err := vm.RunString(expression)
	if err != nil {
		if jsErr, ok := err.(*goja.Exception); ok {
			return false, fmt.Errorf(jsErr.String())
		}
		return false, err
	}

	// Extract boolean result
	if result != nil && !goja.IsUndefined(result) && !goja.IsNull(result) {
		if valid, ok := result.Export().(bool); ok {
			return valid, nil
		}
	}

	// Default to false if expression didn't return a boolean
	return false, fmt.Errorf("expression did not return a boolean value")
}
