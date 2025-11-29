package openapi

import (
	"mockelot/models"

	"github.com/getkin/kin-openapi/openapi3"
)

// ImportContext holds state during OpenAPI import
type ImportContext struct {
	spec     *openapi3.T
	groups   map[string]*models.ResponseGroup
	visited  map[string]int // Track schema depth for circular reference detection
	maxDepth int            // Maximum depth for schema resolution
}

// NewImportContext creates a new import context
func NewImportContext(spec *openapi3.T) *ImportContext {
	return &ImportContext{
		spec:     spec,
		groups:   make(map[string]*models.ResponseGroup),
		visited:  make(map[string]int),
		maxDepth: 3, // Max 3 levels deep for nested schemas
	}
}

// SchemaContext holds state for schema generation
type SchemaContext struct {
	visited  map[string]int
	maxDepth int
	spec     *openapi3.T
}

// NewSchemaContext creates a new schema context
func NewSchemaContext(spec *openapi3.T) *SchemaContext {
	return &SchemaContext{
		visited:  make(map[string]int),
		maxDepth: 3,
		spec:     spec,
	}
}

// OperationInfo holds extracted operation information
type OperationInfo struct {
	Method      string
	Path        string
	Operation   *openapi3.Operation
	PathItem    *openapi3.PathItem
	Parameters  openapi3.Parameters
}
