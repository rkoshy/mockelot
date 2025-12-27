# OpenAPI/Swagger Import Feature

## Overview

Mockelot now supports importing OpenAPI 3.x and Swagger specifications to automatically generate mock HTTP responses. This feature converts API specifications into fully functional mock endpoints with realistic data generation, request validation, and security responses.

## Features

### 1. Automatic Mock Data Generation
- **Schema-based generation**: Converts OpenAPI schemas to JavaScript mock data generators
- **Embedded Faker utilities**: Includes realistic data generation for common formats (email, UUID, dates, etc.)
- **Type-aware generation**: Generates appropriate data for strings, numbers, booleans, arrays, and objects
- **Example support**: Uses examples from the spec when available

### 2. Request Validation
- **Query parameter validation**: Validates required params, types (number, boolean), and enum values
- **Request body validation**: Validates required fields, type checking, and enum constraints
- **Combined validation**: Merges query and body validation scripts seamlessly

### 3. Security Responses
- **401 Unauthorized**: Generated for endpoints with security requirements
- **403 Forbidden**: Generated for access control scenarios
- **Authentication validation**: Detects and validates Bearer tokens, API keys, Basic auth
- **Disabled by default**: Security responses are created but disabled to avoid interfering with normal testing

### 4. Response Grouping
- **Path-based grouping**: All HTTP methods for the same path are grouped together
- **Clean organization**: Each path becomes a ResponseGroup containing all its operations
- **Status code handling**: Generates separate responses for each status code defined in the spec

### 5. Import Modes
- **Append mode**: Add imported endpoints to existing configuration
- **Replace mode**: Clear existing configuration and import fresh
- **User choice**: Dialog prompts user to choose mode during import

## Usage

### From UI
1. Click the "Import OpenAPI" button in the header
2. Choose "Append to existing" or "Replace all" in the dialog
3. Select your OpenAPI/Swagger file (.yaml, .yml, or .json)
4. The imported endpoints will appear in the Responses panel

## Step-by-Step UI Walkthrough

### 1. Import Your Specification

1. **Click "Import OpenAPI"** button in the Mockelot header bar
2. **Choose import mode** in the dialog:
   - **Append**: Add imported endpoints to existing configuration
   - **Replace**: Clear all endpoints and import fresh
   - **Cancel**: Close dialog without importing

3. **Select your file**:
   - Supported formats: `.yaml`, `.yml`, `.json`
   - File browser opens to select local OpenAPI specification
   - Large files (>5MB) may take a few seconds to process

### 2. Review Imported Endpoints

After import, you'll see:
- **Grouped by path**: All HTTP methods for same path grouped together
- **Status codes**: Separate response for each status code (200, 400, 404, etc.)
- **Default states**:
  - Success responses (2xx) are **enabled**
  - Error responses (4xx, 5xx) are **disabled**
  - Security responses (401, 403) are **disabled**

### 3. Understanding Generated Content

#### Static Responses

Created when OpenAPI spec includes `example` or `examples`:

```yaml
responses:
  '200':
    description: User object
    content:
      application/json:
        example:
          id: 123
          name: "John Doe"
```

→ Mockelot creates static JSON response

#### Script Responses

Created when OpenAPI spec includes `schema` but no example:

```yaml
responses:
  '200':
    description: User list
    content:
      application/json:
        schema:
          type: array
          items:
            type: object
            properties:
              id: {type: integer}
              name: {type: string}
```

→ Mockelot generates JavaScript using Faker.js:

```javascript
response.body = JSON.stringify([
  {id: faker.number.int(), name: faker.person.fullName()},
  {id: faker.number.int(), name: faker.person.fullName()}
]);
```

#### Request Validation

Created from OpenAPI query parameters and request body schemas:
- **Query parameter validation**: Type checking, enum validation
- **Request body validation**: Required fields, type checking
- **Validation mode**: Script-based for flexibility

### 4. Customizing After Import

#### Enable/Disable Responses

- Click response to expand
- Toggle "Enabled" checkbox
- Useful for testing specific error conditions

#### Modify Generated Scripts

- Edit script in "Script" tab
- Add custom logic
- Test with "Test Script" button

#### Adjust Delays

- Add response delays to simulate slow networks
- Useful for testing loading states

### Example: E-Commerce API

**OpenAPI Spec:**

```yaml
paths:
  /products:
    get:
      summary: List products
      responses:
        '200':
          description: Product list
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    id: {type: integer}
                    name: {type: string}
                    price: {type: number}
  /products/{id}:
    get:
      summary: Get product
      parameters:
        - name: id
          in: path
          schema: {type: integer}
      responses:
        '200':
          description: Product details
        '404':
          description: Not found
```

**After Import:**

- Group: "GET /products" with 200 response (enabled)
- Group: "GET /products/{id}" with 200 response (enabled) and 404 response (disabled)
- Both responses use Faker-generated data
- Path parameter `:id` automatically extracted
- Validation scripts ensure `id` is numeric

**Result:**

- `GET /products` → Returns array of fake products
- `GET /products/123` → Returns single fake product with id=123
- `GET /products/abc` → Returns 404 (validation fails)

### Supported Features

#### Schema Types
- ✅ Objects with properties
- ✅ Arrays with items
- ✅ Primitives (string, number, integer, boolean)
- ✅ Enums
- ✅ Composition (allOf, oneOf, anyOf)
- ✅ Required fields
- ✅ Format constraints (date-time, email, uuid, etc.)

#### Request Validation
- ✅ Query parameters (required, type, enum)
- ✅ Request body (required fields, types, enums)
- ✅ Path parameters (converted to :param format)

#### Security Schemes
- ✅ Bearer/JWT tokens
- ✅ API keys
- ✅ Basic authentication
- ✅ Custom header-based auth

#### Response Generation
- ✅ Static responses (when examples are provided)
- ✅ Script-based generation (when schemas are provided)
- ✅ Multiple status codes per operation
- ✅ Response headers

## Example

### Input OpenAPI Spec
```yaml
paths:
  /users:
    get:
      parameters:
        - name: limit
          in: query
          required: false
          schema:
            type: integer
            minimum: 1
            maximum: 100
      responses:
        '200':
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/User'
```

### Generated Output
- **Response Group**: "Users"
- **Path Pattern**: "/users"
- **Method**: GET
- **Status Code**: 200
- **Response Mode**: Script
- **Script**: Includes Faker utilities and generates array of User objects
- **Query Validation**: Validates `limit` is a number between 1-100

## Architecture

### Backend Components

#### `openapi/parser.go`
- Parses OpenAPI 3.x specifications using `kin-openapi` library
- Extracts operations, parameters, and schemas
- Handles both YAML and JSON formats

#### `openapi/converter.go`
- Converts OpenAPI operations to Mockelot ResponseItems
- Groups operations by path
- Generates response configurations with validation scripts
- Creates security responses for authenticated endpoints

#### `openapi/schema_generator.go`
- Generates JavaScript mock data from OpenAPI schemas
- Embeds Faker utilities for realistic data
- Handles composition (allOf, oneOf, anyOf)
- Supports all primitive types and formats

#### `openapi/faker.go`
- Embedded Faker.js-like utilities
- Provides realistic data generation functions
- Supports dates, emails, UUIDs, names, addresses, etc.

#### `openapi/importer.go`
- Main entry point for import functionality
- Coordinates parsing and conversion
- Returns ready-to-use ResponseItems

### Frontend Integration

#### `HeaderBar.vue`
- Added "Import OpenAPI" button
- Calls `ImportOpenAPISpec()` backend method
- Refreshes items after successful import

#### `app.go`
- `ImportOpenAPISpec()`: Shows import mode dialog
- `importOpenAPISpecWithMode()`: Handles file selection and import
- Emits events to update frontend

## Testing

A comprehensive test spec is provided in `test-api.yaml` with:
- GET /users (with query params: limit, role)
- POST /users (with request body validation)
- GET /users/{userId} (with Bearer auth security)
- Component schemas with examples, enums, formats

Run the test:
```bash
go run test_import.go
```

Expected output:
- 2 response groups (one per path)
- 6 total responses (including success, error, and security responses)
- Script-mode responses with embedded Faker utilities
- Validation scripts for query params and request bodies
- Security validation scripts for authenticated endpoints

## Limitations

### Not Yet Supported
- ❌ OAuth2/OpenID Connect flows (basic validation only)
- ❌ Webhook definitions
- ❌ Server variables substitution
- ❌ Link objects
- ❌ Callback definitions
- ❌ Discriminator handling for polymorphism

### Known Issues
- Circular references are limited to 3 levels depth to prevent infinite loops
- Very large schemas may generate verbose JavaScript code
- Pattern/regex validation is simplified (uses generic string generation)

## Future Enhancements

Potential improvements:
1. Support for response examples from the spec
2. More sophisticated pattern matching for regex-constrained strings
3. OAuth2 flow simulation
4. Import from URLs (not just local files)
5. Batch import of multiple specs
6. Export imported config to OpenAPI spec

## Technical Details

### Response Generation Priority
1. **Example**: If provided in the spec, use it directly
2. **Schema with example**: Use schema.example if available
3. **Schema**: Generate mock data script using Faker utilities
4. **No schema**: Return empty response

### Validation Script Structure
All validation scripts follow this pattern:
```javascript
(function() {
  // Validation checks here
  if (!condition) {
    return {valid: false, error: 'Error message'};
  }
  return {valid: true};
})()
```

### Mock Data Script Structure
```javascript
// Faker utilities embedded here

// Generated mock data based on OpenAPI schema
(function() {
    const generateData = () => {
        return {
            // Schema-based generation
        };
    };

    // Set response
    response.headers['Content-Type'] = 'application/json';
    response.body = JSON.stringify(generateData(), null, 2);
})();
```

## Files Modified/Created

### Created
- `openapi/parser.go` - OpenAPI spec parsing
- `openapi/extractor.go` - Operation extraction
- `openapi/converter.go` - Conversion to ResponseItems
- `openapi/schema_generator.go` - Mock data generation
- `openapi/faker.go` - Embedded Faker utilities
- `openapi/importer.go` - Import entry point
- `openapi/path_utils.go` - Path pattern conversion
- `openapi/group_naming.go` - Group name generation
- `test-api.yaml` - Test OpenAPI specification
- `test_import.go` - Import test program
- `OPENAPI_IMPORT.md` - This documentation

### Modified
- `app.go` - Added ImportOpenAPISpec methods
- `frontend/src/components/layout/HeaderBar.vue` - Added Import OpenAPI button

## Conclusion

The OpenAPI import feature is fully functional and tested. It provides a powerful way to quickly create mock endpoints from API specifications, complete with realistic data generation and request validation. The feature integrates seamlessly with Mockelot's existing response management system.

---

**Related Documentation:**
- [MOCK-GUIDE.md](MOCK-GUIDE.md) - Customize imported endpoints with static, template, and script responses
- [PROXY-GUIDE.md](PROXY-GUIDE.md) - Reverse proxy endpoints for existing APIs
- [CONTAINER-GUIDE.md](CONTAINER-GUIDE.md) - Docker/Podman container endpoints
- [SETUP.md](SETUP.md) - HTTPS configuration and deployment
