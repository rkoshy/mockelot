package openapi

import (
	"encoding/json"
	"fmt"
	"mockelot/models"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
)

// ConvertToResponseItems converts an OpenAPI spec to MockAgainTool ResponseItems
// Groups responses by path, with all HTTP methods for each path in the same group
func ConvertToResponseItems(spec *openapi3.T) ([]models.ResponseItem, error) {
	operations := ExtractOperations(spec)

	// Group operations by path
	pathGroups := groupOperationsByPath(operations)

	// Convert each path group to a ResponseItem
	items := make([]models.ResponseItem, 0, len(pathGroups))
	for _, group := range pathGroups {
		items = append(items, models.ResponseItem{
			Type:  "group",
			Group: group,
		})
	}

	return items, nil
}

// groupOperationsByPath groups all operations by their path
// Each unique path becomes a ResponseGroup containing all HTTP methods for that path
func groupOperationsByPath(operations []OperationInfo) map[string]*models.ResponseGroup {
	groups := make(map[string]*models.ResponseGroup)

	for _, op := range operations {
		// Get or create group for this path
		group, exists := groups[op.Path]
		if !exists {
			enabled := true
			expanded := true
			group = &models.ResponseGroup{
				ID:        uuid.New().String(),
				Name:      GenerateGroupName(op.Path),
				Enabled:   &enabled,
				Expanded:  &expanded,
				Responses: []models.MethodResponse{},
			}
			groups[op.Path] = group
		}

		// Convert this operation to response(s)
		responses := convertOperation(op)
		group.Responses = append(group.Responses, responses...)
	}

	return groups
}

// convertOperation converts a single OpenAPI operation to one or more MethodResponses
// Creates one response per status code defined in the operation
func convertOperation(op OperationInfo) []models.MethodResponse {
	responses := make([]models.MethodResponse, 0)

	// Convert path to MockAgainTool format
	pathPattern := ConvertOpenAPIPath(op.Path)

	// Convert each response status code
	for statusStr, responseRef := range op.Operation.Responses.Map() {
		if responseRef == nil || responseRef.Value == nil {
			continue
		}

		statusCode := parseStatusCode(statusStr)
		if statusCode == 0 {
			continue // Skip invalid status codes
		}

		response := responseRef.Value

		// Determine if this response should be enabled (only 2xx by default)
		enabled := statusCode >= 200 && statusCode < 300

		// Extract headers
		headers := extractResponseHeaders(response)

		// Generate response body/script
		body, responseMode, scriptBody := generateResponseBody(op, response)

		// Extract status text (dereference pointer)
		statusText := ""
		if response.Description != nil {
			statusText = *response.Description
		}

		// Create the MethodResponse
		methodResponse := models.MethodResponse{
			ID:           uuid.New().String(),
			Enabled:      &enabled,
			PathPattern:  pathPattern,
			Methods:      []string{op.Method},
			StatusCode:   statusCode,
			StatusText:   statusText,
			Headers:      headers,
			Body:         body,
			ResponseMode: responseMode,
			ScriptBody:   scriptBody,
		}

		// Add request validation for POST/PUT/PATCH methods
		if shouldValidateRequest(op.Method) && op.Operation.RequestBody != nil {
			methodResponse.RequestValidation = generateRequestValidation(op)
		}

		// Add query parameter validation if there are query params
		if len(op.Parameters) > 0 {
			queryValidation := generateQueryParamValidation(op.Parameters)
			if queryValidation != "" {
				if methodResponse.RequestValidation == nil {
					methodResponse.RequestValidation = &models.RequestValidation{
						Mode:   models.ValidationModeScript,
						Script: queryValidation,
					}
				} else {
					// Combine with existing validation
					methodResponse.RequestValidation.Script = combineValidationScripts(
						methodResponse.RequestValidation.Script,
						queryValidation,
					)
				}
			}
		}

		responses = append(responses, methodResponse)
	}

	// Add security responses if operation has security requirements
	if op.Operation.Security != nil && len(*op.Operation.Security) > 0 {
		securityResponses := generateSecurityResponses(op)
		responses = append(responses, securityResponses...)
	}

	return responses
}

// parseStatusCode converts OpenAPI status code string to int
func parseStatusCode(statusStr string) int {
	// Handle "default" or wildcard patterns
	if statusStr == "default" {
		return 200 // Default to 200 for "default" responses
	}

	var code int
	fmt.Sscanf(statusStr, "%d", &code)
	return code
}

// extractResponseHeaders extracts headers from OpenAPI response
func extractResponseHeaders(response *openapi3.Response) map[string]string {
	headers := make(map[string]string)

	// Add Content-Type if not present
	hasContentType := false

	for name, headerRef := range response.Headers {
		if headerRef == nil || headerRef.Value == nil {
			continue
		}

		header := headerRef.Value

		if strings.ToLower(name) == "content-type" {
			hasContentType = true
		}

		// Use example value if available
		if header.Example != nil {
			headers[name] = fmt.Sprintf("%v", header.Example)
		} else if header.Schema != nil && header.Schema.Value != nil {
			// Generate from schema
			headers[name] = generateHeaderValue(header.Schema.Value)
		}
	}

	// Default Content-Type to application/json if not specified
	if !hasContentType {
		headers["Content-Type"] = "application/json"
	}

	return headers
}

// generateHeaderValue generates a header value from a schema
func generateHeaderValue(schema *openapi3.Schema) string {
	if schema.Example != nil {
		return fmt.Sprintf("%v", schema.Example)
	}

	switch schema.Type.Slice()[0] {
	case "string":
		if len(schema.Enum) > 0 {
			return fmt.Sprintf("%v", schema.Enum[0])
		}
		return "example-value"
	case "integer", "number":
		return "0"
	case "boolean":
		return "true"
	default:
		return "value"
	}
}

// shouldValidateRequest determines if request validation should be added
func shouldValidateRequest(method string) bool {
	method = strings.ToUpper(method)
	return method == "POST" || method == "PUT" || method == "PATCH"
}

// combineValidationScripts combines two validation scripts into one
func combineValidationScripts(script1, script2 string) string {
	if script1 == "" {
		return script2
	}
	if script2 == "" {
		return script1
	}

	// Extract validation logic from both scripts and combine
	return fmt.Sprintf(`(function() {
  // Request body validation
  var result1 = %s;
  if (!result1.valid) { return result1; }

  // Query parameter validation
  var result2 = %s;
  if (!result2.valid) { return result2; }

  return {valid: true};
})()`, script1, script2)
}

// generateQueryParamValidation generates JavaScript validation for query parameters
func generateQueryParamValidation(params openapi3.Parameters) string {
	if len(params) == 0 {
		return ""
	}

	var validations []string

	for _, paramRef := range params {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}

		param := paramRef.Value

		// Only handle query parameters
		if param.In != "query" {
			continue
		}

		paramName := param.Name
		isRequired := param.Required

		if isRequired {
			validations = append(validations, fmt.Sprintf(
				`  if (!request.queryParams['%s']) { return {valid: false, error: 'Missing required query parameter: %s'}; }`,
				paramName, paramName))
		}

		// Add type/format validation if schema is available
		if param.Schema != nil && param.Schema.Value != nil {
			schema := param.Schema.Value
			types := schema.Type.Slice()
			if len(types) > 0 {
				typ := types[0]

				switch typ {
				case "integer", "number":
					validations = append(validations, fmt.Sprintf(
						`  if (request.queryParams['%s'] && isNaN(request.queryParams['%s'])) { return {valid: false, error: 'Query parameter %s must be a number'}; }`,
						paramName, paramName, paramName))
				case "boolean":
					validations = append(validations, fmt.Sprintf(
						`  if (request.queryParams['%s'] && !['true', 'false'].includes(request.queryParams['%s'].toLowerCase())) { return {valid: false, error: 'Query parameter %s must be true or false'}; }`,
						paramName, paramName, paramName))
				}

				// Enum validation
				if len(schema.Enum) > 0 {
					var enumVals []string
					for _, e := range schema.Enum {
						enumVals = append(enumVals, fmt.Sprintf("'%v'", e))
					}
					validations = append(validations, fmt.Sprintf(
						`  if (request.queryParams['%s'] && ![%s].includes(request.queryParams['%s'])) { return {valid: false, error: 'Query parameter %s must be one of: %s'}; }`,
						paramName, strings.Join(enumVals, ", "), paramName, paramName, strings.Join(enumVals, ", ")))
				}
			}
		}
	}

	if len(validations) == 0 {
		return ""
	}

	return fmt.Sprintf(`(function() {
%s
  return {valid: true};
})()`, strings.Join(validations, "\n"))
}

// generateRequestValidation generates request validation config for request body
func generateRequestValidation(op OperationInfo) *models.RequestValidation {
	if op.Operation.RequestBody == nil || op.Operation.RequestBody.Value == nil {
		return nil
	}

	requestBody := op.Operation.RequestBody.Value

	// Look for application/json content
	var mediaType *openapi3.MediaType
	for contentType, mt := range requestBody.Content {
		if strings.Contains(contentType, "application/json") {
			mediaType = mt
			break
		}
	}

	if mediaType == nil || mediaType.Schema == nil || mediaType.Schema.Value == nil {
		return nil
	}

	schema := mediaType.Schema.Value

	// Generate validation script based on schema
	validationScript := generateRequestBodyValidationScript(schema)
	if validationScript == "" {
		return nil
	}

	return &models.RequestValidation{
		Mode:   models.ValidationModeScript,
		Script: validationScript,
	}
}

// generateRequestBodyValidationScript generates JavaScript validation for request body
func generateRequestBodyValidationScript(schema *openapi3.Schema) string {
	var validations []string

	// Required properties
	if len(schema.Required) > 0 {
		for _, fieldName := range schema.Required {
			validations = append(validations, fmt.Sprintf(
				`  if (!body || body['%s'] === undefined) { return {valid: false, error: 'Missing required field: %s'}; }`,
				fieldName, fieldName))
		}
	}

	// Type validation for properties
	if len(schema.Properties) > 0 {
		for fieldName, propRef := range schema.Properties {
			if propRef.Value == nil {
				continue
			}

			prop := propRef.Value
			types := prop.Type.Slice()
			if len(types) == 0 {
				continue
			}

			typ := types[0]

			// Only validate if field exists (optional fields)
			switch typ {
			case "string":
				validations = append(validations, fmt.Sprintf(
					`  if (body && body['%s'] !== undefined && typeof body['%s'] !== 'string') { return {valid: false, error: 'Field %s must be a string'}; }`,
					fieldName, fieldName, fieldName))
			case "integer", "number":
				validations = append(validations, fmt.Sprintf(
					`  if (body && body['%s'] !== undefined && typeof body['%s'] !== 'number') { return {valid: false, error: 'Field %s must be a number'}; }`,
					fieldName, fieldName, fieldName))
			case "boolean":
				validations = append(validations, fmt.Sprintf(
					`  if (body && body['%s'] !== undefined && typeof body['%s'] !== 'boolean') { return {valid: false, error: 'Field %s must be a boolean'}; }`,
					fieldName, fieldName, fieldName))
			case "array":
				validations = append(validations, fmt.Sprintf(
					`  if (body && body['%s'] !== undefined && !Array.isArray(body['%s'])) { return {valid: false, error: 'Field %s must be an array'}; }`,
					fieldName, fieldName, fieldName))
			case "object":
				validations = append(validations, fmt.Sprintf(
					`  if (body && body['%s'] !== undefined && (typeof body['%s'] !== 'object' || Array.isArray(body['%s']))) { return {valid: false, error: 'Field %s must be an object'}; }`,
					fieldName, fieldName, fieldName, fieldName))
			}

			// Enum validation
			if len(prop.Enum) > 0 {
				var enumVals []string
				for _, e := range prop.Enum {
					switch v := e.(type) {
					case string:
						enumVals = append(enumVals, fmt.Sprintf("'%s'", v))
					default:
						enumVals = append(enumVals, fmt.Sprintf("%v", v))
					}
				}
				validations = append(validations, fmt.Sprintf(
					`  if (body && body['%s'] !== undefined && ![%s].includes(body['%s'])) { return {valid: false, error: 'Field %s must be one of: %s'}; }`,
					fieldName, strings.Join(enumVals, ", "), fieldName, fieldName, strings.Join(enumVals, ", ")))
			}
		}
	}

	if len(validations) == 0 {
		return ""
	}

	return fmt.Sprintf(`(function() {
  var body;
  try {
    body = JSON.parse(request.body);
  } catch (e) {
    return {valid: false, error: 'Invalid JSON in request body'};
  }

%s
  return {valid: true};
})()`, strings.Join(validations, "\n"))
}

// generateSecurityResponses generates 401/403 responses for secured operations
func generateSecurityResponses(op OperationInfo) []models.MethodResponse {
	if op.Operation.Security == nil || len(*op.Operation.Security) == 0 {
		return nil
	}

	pathPattern := ConvertOpenAPIPath(op.Path)
	responses := make([]models.MethodResponse, 0)

	// Generate 401 Unauthorized response
	enabled401 := false
	response401 := models.MethodResponse{
		ID:           uuid.New().String(),
		Enabled:      &enabled401,
		PathPattern:  pathPattern,
		Methods:      []string{op.Method},
		StatusCode:   401,
		StatusText:   "Unauthorized - Missing or invalid authentication",
		Headers:      map[string]string{"Content-Type": "application/json"},
		Body:         `{"error": "Unauthorized", "message": "Authentication required"}`,
		ResponseMode: models.ResponseModeStatic,
		RequestValidation: &models.RequestValidation{
			Mode:   models.ValidationModeScript,
			Script: generateAuthValidationScript(op),
		},
	}
	responses = append(responses, response401)

	// Generate 403 Forbidden response
	enabled403 := false
	response403 := models.MethodResponse{
		ID:          uuid.New().String(),
		Enabled:     &enabled403,
		PathPattern: pathPattern,
		Methods:     []string{op.Method},
		StatusCode:  403,
		StatusText:  "Forbidden - Insufficient permissions",
		Headers:     map[string]string{"Content-Type": "application/json"},
		Body:        `{"error": "Forbidden", "message": "Insufficient permissions"}`,
		ResponseMode: models.ResponseModeStatic,
	}
	responses = append(responses, response403)

	return responses
}

// generateAuthValidationScript generates authentication validation script
func generateAuthValidationScript(op OperationInfo) string {
	if op.Operation.Security == nil || len(*op.Operation.Security) == 0 {
		return ""
	}

	// Analyze security requirements
	var authChecks []string

	for _, secReq := range *op.Operation.Security {
		for schemeName := range secReq {
			// Generate validation based on common scheme names
			lowerName := strings.ToLower(schemeName)

			if strings.Contains(lowerName, "bearer") || strings.Contains(lowerName, "jwt") {
				authChecks = append(authChecks, `
  // Check for Bearer token
  var authHeader = request.headers['authorization'] || request.headers['Authorization'];
  if (!authHeader || !authHeader.startsWith('Bearer ')) {
    return {valid: false, error: 'Missing or invalid Authorization header'};
  }`)
			} else if strings.Contains(lowerName, "api") && strings.Contains(lowerName, "key") {
				authChecks = append(authChecks, `
  // Check for API Key
  var apiKey = request.headers['x-api-key'] || request.headers['X-API-Key'];
  if (!apiKey) {
    return {valid: false, error: 'Missing API key'};
  }`)
			} else if strings.Contains(lowerName, "basic") {
				authChecks = append(authChecks, `
  // Check for Basic auth
  var authHeader = request.headers['authorization'] || request.headers['Authorization'];
  if (!authHeader || !authHeader.startsWith('Basic ')) {
    return {valid: false, error: 'Missing or invalid Basic authentication'};
  }`)
			} else {
				// Generic auth header check
				authChecks = append(authChecks, `
  // Check for authentication header
  var authHeader = request.headers['authorization'] || request.headers['Authorization'];
  if (!authHeader) {
    return {valid: false, error: 'Missing authentication'};
  }`)
			}
			break // Only generate one check per operation
		}
	}

	if len(authChecks) == 0 {
		return ""
	}

	return fmt.Sprintf(`(function() {
%s
  return {valid: true};
})()`, strings.Join(authChecks, "\n"))
}

// generateResponseBody generates the response body, mode, and script
// Returns: (body, responseMode, scriptBody)
func generateResponseBody(op OperationInfo, response *openapi3.Response) (string, string, string) {
	// Check if there's content defined
	if response.Content == nil || len(response.Content) == 0 {
		// No content - empty body
		return "", models.ResponseModeStatic, ""
	}

	// Try to find application/json content first
	var mediaType *openapi3.MediaType
	for contentType, mt := range response.Content {
		if strings.Contains(contentType, "application/json") {
			mediaType = mt
			break
		}
	}

	// If no JSON, use the first available content type
	if mediaType == nil {
		for _, mt := range response.Content {
			mediaType = mt
			break
		}
	}

	if mediaType == nil {
		return "", models.ResponseModeStatic, ""
	}

	// Check for example first (priority)
	if mediaType.Example != nil {
		// Use the example directly
		exampleJSON, _ := convertToJSON(mediaType.Example)
		return exampleJSON, models.ResponseModeStatic, ""
	}

	// Check schema.example
	if mediaType.Schema != nil && mediaType.Schema.Value != nil && mediaType.Schema.Value.Example != nil {
		exampleJSON, _ := convertToJSON(mediaType.Schema.Value.Example)
		return exampleJSON, models.ResponseModeStatic, ""
	}

	// No example - generate script from schema
	if mediaType.Schema != nil && mediaType.Schema.Value != nil {
		script := GenerateMockScript(mediaType.Schema, op)
		return "", models.ResponseModeScript, script
	}

	// No schema either - return empty response
	return "", models.ResponseModeStatic, ""
}

// convertToJSON converts an interface{} to JSON string
func convertToJSON(v interface{}) (string, error) {
	// Use encoding/json to properly marshal
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}
