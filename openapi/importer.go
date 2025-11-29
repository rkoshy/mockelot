package openapi

import (
	"fmt"
	"mockelot/models"
)

// ImportSpec imports an OpenAPI/Swagger specification file and converts it to ResponseItems
// This is the main entry point for the OpenAPI import functionality
func ImportSpec(filePath string) ([]models.ResponseItem, error) {
	// Step 1: Parse the OpenAPI spec
	spec, err := ParseSpec(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Step 2: Convert to ResponseItems
	items, err := ConvertToResponseItems(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to convert OpenAPI spec: %w", err)
	}

	return items, nil
}
