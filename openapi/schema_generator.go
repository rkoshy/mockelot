package openapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// GenerateMockScript generates a complete JavaScript script for mock data generation
// Includes Faker utilities and schema-based generation logic
func GenerateMockScript(schema *openapi3.SchemaRef, op OperationInfo) string {
	if schema == nil || schema.Value == nil {
		return generateEmptyScript()
	}

	ctx := &SchemaContext{
		visited:  make(map[string]int),
		maxDepth: 3,
		spec:     nil, // We don't have access to the full spec here
	}

	dataGenCode := generateSchemaCode(schema.Value, ctx, 0)

	return fmt.Sprintf(`
%s

// Generated mock data based on OpenAPI schema
(function() {
    const generateData = () => {
        return %s;
    };

    // Set response
    response.headers['Content-Type'] = 'application/json';
    response.body = JSON.stringify(generateData(), null, 2);
})();
`, FakerJS, dataGenCode)
}

// generateEmptyScript generates a simple empty response script
func generateEmptyScript() string {
	return `
response.headers['Content-Type'] = 'application/json';
response.body = JSON.stringify({status: "ok"}, null, 2);
`
}

// generateSchemaCode generates JavaScript code for a schema
func generateSchemaCode(schema *openapi3.Schema, ctx *SchemaContext, depth int) string {
	if depth > ctx.maxDepth {
		return "null /* max depth reached */"
	}

	// Priority 1: Use example if available
	if schema.Example != nil {
		return convertExampleToJS(schema.Example)
	}

	// Priority 2: Use enum if available
	if len(schema.Enum) > 0 {
		return generateEnumCode(schema.Enum)
	}

	// Priority 3: Handle composition (allOf, oneOf, anyOf)
	if len(schema.AllOf) > 0 || len(schema.OneOf) > 0 || len(schema.AnyOf) > 0 {
		return generateCompositionCode(schema, ctx, depth)
	}

	// Priority 4: Generate based on type
	types := schema.Type.Slice()
	if len(types) == 0 {
		// No type specified - try to infer
		if len(schema.Properties) > 0 {
			return generateObjectCode(schema, ctx, depth)
		}
		return "null"
	}

	typ := types[0]

	switch typ {
	case "object":
		return generateObjectCode(schema, ctx, depth)
	case "array":
		return generateArrayCode(schema, ctx, depth)
	case "string":
		return generateStringCode(schema)
	case "integer", "number":
		return generateNumberCode(schema, typ)
	case "boolean":
		return "faker.random.boolean()"
	case "null":
		return "null"
	default:
		return "null /* unknown type */"
	}
}

// generateObjectCode generates JavaScript for an object schema
func generateObjectCode(schema *openapi3.Schema, ctx *SchemaContext, depth int) string {
	if len(schema.Properties) == 0 {
		return "{}"
	}

	var parts []string

	// Always include required fields
	for _, fieldName := range schema.Required {
		propRef, exists := schema.Properties[fieldName]
		if !exists || propRef.Value == nil {
			continue
		}

		fieldValue := generateSchemaCode(propRef.Value, ctx, depth+1)
		parts = append(parts, fmt.Sprintf(`    "%s": %s`, fieldName, fieldValue))
	}

	// Optional fields: 50% chance to include
	for fieldName, propRef := range schema.Properties {
		if contains(schema.Required, fieldName) {
			continue // Already added
		}

		if propRef.Value == nil {
			continue
		}

		fieldValue := generateSchemaCode(propRef.Value, ctx, depth+1)
		parts = append(parts, fmt.Sprintf(`    ...(Math.random() > 0.5 ? {"%s": %s} : {})`, fieldName, fieldValue))
	}

	if len(parts) == 0 {
		return "{}"
	}

	return "{\n" + strings.Join(parts, ",\n") + "\n  }"
}

// generateArrayCode generates JavaScript for an array schema
func generateArrayCode(schema *openapi3.Schema, ctx *SchemaContext, depth int) string {
	if schema.Items == nil || schema.Items.Value == nil {
		return "[]"
	}

	itemCode := generateSchemaCode(schema.Items.Value, ctx, depth+1)

	// Generate 2-3 items by default
	minItems := 2
	maxItems := 3

	if schema.MinItems > 0 {
		minItems = int(schema.MinItems)
	}
	if schema.MaxItems != nil && *schema.MaxItems > 0 {
		maxItems = int(*schema.MaxItems)
	}

	// Cap at reasonable limits
	if minItems > 5 {
		minItems = 5
	}
	if maxItems > 5 {
		maxItems = 5
	}

	return fmt.Sprintf(`Array(%d).fill(0).map(() => %s)`,
		(minItems+maxItems)/2, itemCode)
}

// generateStringCode generates JavaScript for a string schema
func generateStringCode(schema *openapi3.Schema) string {
	// Check for format
	if schema.Format != "" {
		return generateFormattedString(schema.Format)
	}

	// Check for pattern (simple patterns only)
	if schema.Pattern != "" {
		return `faker.lorem.word()` // Placeholder for regex patterns
	}

	// Min/max length constraints
	if schema.MinLength > 0 || schema.MaxLength != nil {
		length := 10
		if schema.MinLength > 0 {
			length = int(schema.MinLength)
		}
		if schema.MaxLength != nil && *schema.MaxLength < uint64(length) {
			length = int(*schema.MaxLength)
		}
		return fmt.Sprintf(`faker.lorem.words(%d)`, (length/5)+1)
	}

	return `faker.lorem.word()`
}

// generateFormattedString generates code for formatted strings
func generateFormattedString(format string) string {
	switch format {
	case "date-time":
		return "faker.date.now()"
	case "date":
		return "faker.date.today()"
	case "time":
		return `new Date().toISOString().split('T')[1]`
	case "email":
		return "faker.internet.email()"
	case "uuid":
		return "faker.random.uuid()"
	case "uri", "url":
		return "faker.internet.url()"
	case "hostname":
		return "faker.internet.domainName()"
	case "ipv4":
		return "faker.internet.ipv4()"
	case "ipv6":
		return "faker.internet.ipv6()"
	case "byte":
		return `btoa(faker.lorem.word())` // Base64 encoded
	case "binary":
		return `faker.lorem.word()`
	case "password":
		return `faker.random.uuid()` // Random UUID as password
	default:
		return `faker.lorem.word()`
	}
}

// generateNumberCode generates JavaScript for number/integer schemas
func generateNumberCode(schema *openapi3.Schema, typ string) string {
	min := 0.0
	max := 100.0

	if schema.Min != nil {
		min = *schema.Min
	}
	if schema.Max != nil {
		max = *schema.Max
	}

	if typ == "integer" {
		return fmt.Sprintf(`faker.random.number(%d, %d)`, int(min), int(max))
	}

	return fmt.Sprintf(`faker.random.float(%g, %g, 2)`, min, max)
}

// generateEnumCode generates JavaScript for enum values
func generateEnumCode(enums []interface{}) string {
	if len(enums) == 0 {
		return "null"
	}

	// Build array of enum values
	var values []string
	for _, e := range enums {
		switch v := e.(type) {
		case string:
			values = append(values, fmt.Sprintf(`"%s"`, escapeString(v)))
		case int, int64, float64:
			values = append(values, fmt.Sprintf(`%v`, v))
		case bool:
			values = append(values, fmt.Sprintf(`%v`, v))
		default:
			values = append(values, `null`)
		}
	}

	return fmt.Sprintf(`[%s][Math.floor(Math.random() * %d)]`,
		strings.Join(values, ", "), len(values))
}

// generateCompositionCode handles allOf/oneOf/anyOf composition
func generateCompositionCode(schema *openapi3.Schema, ctx *SchemaContext, depth int) string {
	if len(schema.AllOf) > 0 {
		// allOf: Merge all schemas using spread operator
		var parts []string
		for _, schemaRef := range schema.AllOf {
			if schemaRef.Value != nil {
				code := generateSchemaCode(schemaRef.Value, ctx, depth)
				parts = append(parts, code)
			}
		}
		if len(parts) == 0 {
			return "{}"
		}
		if len(parts) == 1 {
			return parts[0]
		}
		// Merge multiple objects using spread syntax
		return "Object.assign({}, " + strings.Join(parts, ", ") + ")"
	}

	if len(schema.OneOf) > 0 {
		// oneOf: Randomly pick one variant
		var variants []string
		for _, schemaRef := range schema.OneOf {
			if schemaRef.Value != nil {
				code := generateSchemaCode(schemaRef.Value, ctx, depth)
				variants = append(variants, code)
			}
		}
		if len(variants) == 0 {
			return "{}"
		}
		if len(variants) == 1 {
			return variants[0]
		}
		return fmt.Sprintf("[%s][Math.floor(Math.random() * %d)]",
			strings.Join(variants, ", "), len(variants))
	}

	if len(schema.AnyOf) > 0 {
		// anyOf: Pick first variant (could randomize like oneOf)
		if len(schema.AnyOf) > 0 && schema.AnyOf[0].Value != nil {
			return generateSchemaCode(schema.AnyOf[0].Value, ctx, depth)
		}
	}

	return "{}"
}

// convertExampleToJS converts an example value to JavaScript code
func convertExampleToJS(example interface{}) string {
	// Convert to JSON first
	jsonBytes, err := json.Marshal(example)
	if err != nil {
		return "null"
	}
	return string(jsonBytes)
}

// escapeString escapes special characters in strings for JavaScript
func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
