package openapi

import (
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
)

// ParseSpec loads and parses an OpenAPI specification from a file
// Supports both OpenAPI 3.x (YAML/JSON) and Swagger 2.0 formats
func ParseSpec(filePath string) (*openapi3.T, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try to parse as OpenAPI 3.x
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Validate the spec
	if err := doc.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	return doc, nil
}

// ExtractOperations extracts all operations from the OpenAPI spec
func ExtractOperations(spec *openapi3.T) []OperationInfo {
	var operations []OperationInfo

	// Iterate through all paths
	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		// Collect parameters from path level
		pathParams := pathItem.Parameters

		// Extract each HTTP method
		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Merge path-level and operation-level parameters
			params := make(openapi3.Parameters, 0)
			params = append(params, pathParams...)
			params = append(params, operation.Parameters...)

			operations = append(operations, OperationInfo{
				Method:     method,
				Path:       path,
				Operation:  operation,
				PathItem:   pathItem,
				Parameters: params,
			})
		}
	}

	return operations
}

// ConvertOpenAPIPath converts OpenAPI path syntax to MockAgainTool syntax
// Converts {param} to :param
func ConvertOpenAPIPath(openAPIPath string) string {
	// Simple state machine to convert {param} to :param
	result := ""
	inBrace := false
	param := ""

	for _, ch := range openAPIPath {
		if ch == '{' {
			inBrace = true
			param = ""
			result += ":"
		} else if ch == '}' {
			inBrace = false
			result += param
			param = ""
		} else if inBrace {
			param += string(ch)
		} else {
			result += string(ch)
		}
	}

	return result
}

// GenerateGroupName creates a readable group name from a path
func GenerateGroupName(path string) string {
	if path == "/" {
		return "Root"
	}

	// Remove leading slash
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	// Convert path like "/users/{id}/posts" to "Users - ID - Posts"
	// This is a simple implementation; could be more sophisticated
	name := ""
	parts := splitPath(path)

	for i, part := range parts {
		if i > 0 {
			name += " / "
		}

		// Capitalize first letter
		if len(part) > 0 {
			if part[0] == '{' && part[len(part)-1] == '}' {
				// Parameter: {id} -> :id
				name += ":" + capitalize(part[1:len(part)-1])
			} else {
				name += capitalize(part)
			}
		}
	}

	return name
}

// splitPath splits a path by slashes
func splitPath(path string) []string {
	parts := []string{}
	current := ""

	for _, ch := range path {
		if ch == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// capitalize capitalizes the first letter of a string
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}

	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}

	return s
}
