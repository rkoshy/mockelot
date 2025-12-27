# Mock Endpoint Guide

Mock endpoints provide static, template-based, or script-generated HTTP responses without requiring a backend service. They're ideal for testing, development, and prototyping.

## Table of Contents

- [Overview](#overview)
- [Creating Mock Endpoints](#creating-mock-endpoints)
- [Response Modes](#response-modes)
  - [Static Mode](#static-mode)
  - [Template Mode](#template-mode)
  - [Script Mode](#script-mode)
- [Path Patterns](#path-patterns)
- [Domain Filtering (SOCKS5 Integration)](#domain-filtering-socks5-integration)
- [Request Validation](#request-validation)
- [Response Configuration](#response-configuration)
- [Organizing with Groups](#organizing-with-groups)
- [Common Use Cases](#common-use-cases)
- [Best Practices](#best-practices)

## Overview

Mock endpoints are the simplest endpoint type in Mockelot. They return pre-defined responses based on:
- **HTTP method** (GET, POST, PUT, DELETE, etc.)
- **Path pattern** (exact match, wildcard, path parameters, or regex)
- **Request validation** (optional body matching)

Mock endpoints support three response modes with increasing complexity:
1. **Static** - Fixed response (fastest)
2. **Template** - Go templates with request context (dynamic)
3. **Script** - JavaScript for complex logic (most flexible)

## Creating Mock Endpoints

Mock endpoints are created implicitly when you add response rules to the root-level responses configuration (not under a specific endpoint).

**In the UI:**
1. Click **"Add Response"** or **"Add Group"** in the Responses panel
2. Configure your response rule (path pattern, method, status code, body)
3. The server will route matching requests to your mock response

**In YAML configuration:**
```yaml
port: 8080
items:
  - type: response
    response:
      path_pattern: "/api/users"
      methods: ["GET"]
      status_code: 200
      headers:
        Content-Type: "application/json"
      body: '{"users": []}'
```

## Response Modes

### Static Mode

Static mode returns a fixed response body with no processing. This is the fastest mode and suitable for most cases.

**Configuration:**
```yaml
response:
  path_pattern: "/api/status"
  methods: ["GET"]
  status_code: 200
  response_mode: "static"  # Default if not specified
  headers:
    Content-Type: "application/json"
  body: '{"status": "ok", "version": "1.0.0"}'
```

**When to use:**
- Fixed API responses
- Static JSON/XML data
- Health check endpoints
- Simple error responses

**Limitations:**
- Cannot access request data
- Cannot generate dynamic values
- Response is identical every time

### Template Mode

Template mode uses Go's `text/template` syntax to generate dynamic responses based on the incoming request.

**Available Context Variables:**
```go
.Method          // HTTP method (GET, POST, etc.)
.Path            // Request path
.Query           // Query parameters (map[string][]string)
.Headers         // Request headers (map[string][]string)
.Body            // Request body (string)
.PathParams      // Extracted path parameters (map[string]string)
.RemoteAddr      // Client IP address
```

**Template Functions:**
```go
json           // Marshal value to JSON
base64Encode   // Base64 encode string
base64Decode   // Base64 decode string
upper          // Uppercase string
lower          // Lowercase string
title          // Title case string
trim           // Trim whitespace
now            // Current timestamp (RFC3339)
uuid           // Generate UUID
randomInt      // Random integer (min, max)
randomString   // Random alphanumeric string (length)
```

**Example - Echo Request Data:**
```yaml
response:
  path_pattern: "/api/echo"
  methods: ["POST"]
  status_code: 200
  response_mode: "template"
  headers:
    Content-Type: "application/json"
  body: |
    {
      "received": {
        "method": "{{.Method}}",
        "path": "{{.Path}}",
        "body": {{.Body | json}},
        "timestamp": "{{now}}",
        "client": "{{.RemoteAddr}}"
      }
    }
```

**Example - Query Parameter Access:**
```yaml
response:
  path_pattern: "/api/search"
  methods: ["GET"]
  status_code: 200
  response_mode: "template"
  body: |
    {
      "query": "{{index .Query "q" 0}}",
      "limit": {{index .Query "limit" 0 | default "10"}},
      "results": []
    }
```

**Example - Path Parameters:**
```yaml
response:
  path_pattern: "/api/users/:id"
  methods: ["GET"]
  status_code: 200
  response_mode: "template"
  body: |
    {
      "id": "{{.PathParams.id}}",
      "name": "User {{.PathParams.id}}",
      "email": "user{{.PathParams.id}}@example.com"
    }
```

**When to use:**
- Echo/mirror endpoints
- Dynamic values based on request
- Simple data extraction
- Timestamp generation
- UUID generation

**Limitations:**
- Limited logic (no conditionals beyond template syntax)
- Cannot modify response object dynamically
- No external data access

### Script Mode

Script mode uses JavaScript (via Goja runtime) for maximum flexibility. Scripts have full access to request data and can implement complex logic.

**Available Objects:**
```javascript
request.method          // HTTP method
request.path            // Request path
request.query           // Query params (object)
request.headers         // Request headers (object)
request.body            // Request body (string)
request.pathParams      // Extracted path params (object)
request.remoteAddr      // Client IP address

response.status         // Set status code
response.statusText     // Set status text
response.headers        // Set headers (object)
response.body           // Set response body (string)

JSON.parse(str)         // Parse JSON string
JSON.stringify(obj)     // Convert object to JSON

// Utility functions
uuid()                  // Generate UUID
now()                   // Current timestamp (RFC3339)
randomInt(min, max)     // Random integer
randomString(length)    // Random alphanumeric string
base64Encode(str)       // Base64 encode
base64Decode(str)       // Base64 decode
```

**Example - Conditional Response:**
```javascript
// Parse request body
const data = JSON.parse(request.body);

// Validate input
if (!data.email || !data.password) {
    response.status = 400;
    response.headers['Content-Type'] = 'application/json';
    response.body = JSON.stringify({
        error: "Missing required fields",
        required: ["email", "password"]
    });
} else {
    response.status = 200;
    response.headers['Content-Type'] = 'application/json';
    response.body = JSON.stringify({
        id: uuid(),
        email: data.email,
        created: now()
    });
}
```

**Example - Mock Database:**
```javascript
// Simulate a simple user database
const users = {
    "1": { id: "1", name: "Alice", email: "alice@example.com" },
    "2": { id: "2", name: "Bob", email: "bob@example.com" }
};

const userId = request.pathParams.id;

if (users[userId]) {
    response.status = 200;
    response.headers['Content-Type'] = 'application/json';
    response.body = JSON.stringify(users[userId]);
} else {
    response.status = 404;
    response.headers['Content-Type'] = 'application/json';
    response.body = JSON.stringify({
        error: "User not found",
        id: userId
    });
}
```

**Example - Request Counting:**
```javascript
// Note: Each request gets a fresh script execution
// For persistent state, use external storage

response.status = 200;
response.headers['Content-Type'] = 'application/json';
response.headers['X-Request-ID'] = uuid();

response.body = JSON.stringify({
    timestamp: now(),
    request: {
        method: request.method,
        path: request.path,
        ip: request.remoteAddr
    }
});
```

**When to use:**
- Complex validation logic
- Conditional responses
- Data transformation
- Simulating stateful behavior (with limitations)
- Custom error handling

**Limitations:**
- 5-second execution timeout
- No persistent state between requests
- No external API calls
- No file system access
- No network access

## Path Patterns

Mock endpoints support multiple path pattern types with priority-based matching.

### Pattern Types (in order of priority)

1. **Exact Match** - Highest priority
   ```yaml
   path_pattern: "/api/users"
   # Matches: /api/users only
   ```

2. **Wildcard Match**
   ```yaml
   path_pattern: "/api/*"
   # Matches: /api/foo, /api/bar, /api/baz
   # Does NOT match: /api/foo/bar (no slash in wildcard)
   ```

3. **Path Parameters**
   ```yaml
   path_pattern: "/api/users/:id"
   # Matches: /api/users/123, /api/users/abc
   # Extracts: .PathParams.id = "123"
   ```

   Multiple parameters:
   ```yaml
   path_pattern: "/api/:resource/:id"
   # Matches: /api/users/123
   # Extracts: .PathParams.resource = "users", .PathParams.id = "123"
   ```

4. **Regex Match** - Lowest priority
   ```yaml
   path_pattern: "^/api/v[0-9]+/users$"
   # Matches: /api/v1/users, /api/v2/users
   # Does NOT match: /api/users, /api/v1/users/123
   ```

### Multiple Methods

A single response can handle multiple HTTP methods:

```yaml
response:
  path_pattern: "/api/resource"
  methods: ["GET", "POST", "PUT", "DELETE"]
  status_code: 200
```

Or use separate responses for different methods:

```yaml
- type: response
  response:
    path_pattern: "/api/users"
    methods: ["GET"]
    status_code: 200
    body: '{"users": []}'

- type: response
  response:
    path_pattern: "/api/users"
    methods: ["POST"]
    status_code: 201
    body: '{"id": "new-user"}'
```

## Domain Filtering (SOCKS5 Integration)

When using Mockelot as a SOCKS5 proxy, you can configure mock endpoints to respond only to specific domains.

### Filter Modes

#### Any Domain (Default)

Mock responds to all requests regardless of domain:

```json
{
  "domain_filter": {
    "mode": "any"
  }
}
```

This is the default mode when no domain filter is configured.

#### All SOCKS5 Domains

Mock responds only to domains configured in SOCKS5 domain takeover:

```json
{
  "domain_filter": {
    "mode": "all"
  }
}
```

The endpoint will match requests from any domain listed in your SOCKS5 configuration's domain takeover list.

#### Specific Domains

Mock responds only to specified domain patterns:

```json
{
  "domain_filter": {
    "mode": "specific",
    "patterns": ["api.example.com", "*.staging.example.com"]
  }
}
```

Domain patterns support:
- **Exact match**: `api.example.com`
- **Wildcard subdomain**: `*.example.com` (matches any subdomain)
- **Multiple patterns**: List multiple domains/patterns in the array

### Use Cases

#### Multi-Tenant API Testing

Different mock responses for different subdomains:

```yaml
# Tenant 1 endpoint
- type: response
  response:
    path_pattern: "/api/users"
    domain_filter:
      mode: "specific"
      patterns: ["api.tenant1.example.com"]
    body: '{"tenant": "tenant1", "users": [...]}'

# Tenant 2 endpoint
- type: response
  response:
    path_pattern: "/api/users"
    domain_filter:
      mode: "specific"
      patterns: ["api.tenant2.example.com"]
    body: '{"tenant": "tenant2", "users": [...]}'
```

Results:
- `api.tenant1.example.com/api/users` → Tenant 1 data
- `api.tenant2.example.com/api/users` → Tenant 2 data

#### Service-Specific Mocking

Mock only authentication service while proxying others:

```yaml
# Mock auth service
- type: response
  response:
    path_pattern: "/login"
    domain_filter:
      mode: "specific"
      patterns: ["auth.example.com"]
    body: '{"token": "mock-jwt-token"}'

# Other services use overlay mode (pass through to real backend)
```

Results:
- `auth.example.com/login` → Mock response
- `api.example.com/*` → Pass through to real API (overlay mode)

#### Progressive Migration

Mock legacy endpoints while using new ones:

```yaml
# Mock old API version
- type: response
  response:
    path_pattern: "/v1/*"
    domain_filter:
      mode: "specific"
      patterns: ["api.example.com"]
    body: '{"version": "v1", "deprecated": true}'

# New API version passes through to real backend
```

Results:
- `api.example.com/v1/users` → Mock old API
- `api.example.com/v2/users` → Proxy to new API (overlay mode)

### Configuration in UI

1. Open endpoint settings dialog
2. Expand **"Domain Filter"** section
3. Select filter mode:
   - **Any** - Match all domains (default)
   - **All SOCKS5** - Match all intercepted domains
   - **Specific** - Match selected domains
4. For "Specific" mode, add domain patterns (one per line or comma-separated)
5. Click **Save** to apply changes

### Domain + Path Matching

Both domain filter AND path pattern must match for the endpoint to handle the request:

```
Request: api.example.com/api/users
         ↓
Domain Filter Check: Does domain match?
         ↓ YES
Path Pattern Check: Does /api/users match?
         ↓ YES
Return Mock Response
```

If either check fails, the request continues to the next endpoint or overlay mode.

See [SOCKS5 Guide](SOCKS5-GUIDE.md) for complete SOCKS5 proxy setup and domain takeover configuration.

## Request Validation

Request validation allows you to conditionally match requests based on their body content. This enables different responses based on request data.

### Validation Modes

#### None (Default)
No validation - all requests matching path and method are accepted.

```yaml
request_validation:
  mode: "none"
```

#### Static Match
Match request body against a static pattern.

**Exact Match:**
```yaml
request_validation:
  mode: "static"
  match_type: "exact"
  pattern: '{"action": "login"}'
```

**Contains Match:**
```yaml
request_validation:
  mode: "static"
  match_type: "contains"
  pattern: '"action":"login"'
```

#### Regex Match
Use regular expressions with named capture groups.

```yaml
request_validation:
  mode: "regex"
  pattern: '"email":\s*"(?P<email>[^"]+)"'
# Extracts email and makes it available in templates/scripts
```

#### Script Validation
Use JavaScript for complex validation logic.

```javascript
(function() {
    try {
        const data = JSON.parse(request.body);

        // Validate required fields
        if (!data.email || !data.password) {
            return {valid: false, error: "Missing credentials"};
        }

        // Validate email format
        if (!data.email.includes('@')) {
            return {valid: false, error: "Invalid email"};
        }

        return {valid: true};
    } catch (e) {
        return {valid: false, error: "Invalid JSON"};
    }
})()
```

## Response Configuration

### Status Codes

Set any HTTP status code:

```yaml
status_code: 200   # OK
status_code: 201   # Created
status_code: 400   # Bad Request
status_code: 404   # Not Found
status_code: 500   # Internal Server Error
```

### Headers

Set custom response headers:

```yaml
headers:
  Content-Type: "application/json"
  X-Custom-Header: "custom-value"
  Cache-Control: "no-cache"
  Access-Control-Allow-Origin: "*"
```

### Response Delay

Simulate network latency or slow endpoints:

```yaml
response_delay: 500  # milliseconds
```

This delays the response by 500ms before sending.

### Enable/Disable

Temporarily disable a response without deleting it:

```yaml
enabled: false
```

## Organizing with Groups

Groups help organize related responses and apply common settings.

### Creating a Group

```yaml
- type: group
  group:
    name: "User API"
    enabled: true
    expanded: true
    responses:
      - path_pattern: "/api/users"
        methods: ["GET"]
        status_code: 200
        body: '{"users": []}'

      - path_pattern: "/api/users/:id"
        methods: ["GET"]
        status_code: 200
        response_mode: "template"
        body: '{"id": "{{.PathParams.id}}"}'
```

### Group Benefits

1. **Organization** - Logical grouping of related endpoints
2. **Bulk Enable/Disable** - Toggle entire API sections
3. **Visual Hierarchy** - Collapsible groups in UI
4. **CORS Settings** - Apply CORS to all responses in group

### Group-Level Enable/Disable

Disable an entire group:

```yaml
group:
  name: "Admin API"
  enabled: false  # Disables all responses in group
  responses: [...]
```

## Common Use Cases

### 1. REST API Mock

```yaml
- type: group
  group:
    name: "Products API"
    responses:
      # List products
      - path_pattern: "/api/products"
        methods: ["GET"]
        status_code: 200
        response_mode: "script"
        script_body: |
          response.headers['Content-Type'] = 'application/json';
          response.body = JSON.stringify({
            products: [
              { id: "1", name: "Widget", price: 9.99 },
              { id: "2", name: "Gadget", price: 19.99 }
            ]
          });

      # Get product by ID
      - path_pattern: "/api/products/:id"
        methods: ["GET"]
        status_code: 200
        response_mode: "template"
        body: |
          {
            "id": "{{.PathParams.id}}",
            "name": "Product {{.PathParams.id}}",
            "price": {{randomInt 10 100}}.99
          }

      # Create product
      - path_pattern: "/api/products"
        methods: ["POST"]
        status_code: 201
        response_mode: "script"
        script_body: |
          const data = JSON.parse(request.body);
          response.status = 201;
          response.headers['Content-Type'] = 'application/json';
          response.body = JSON.stringify({
            id: uuid(),
            ...data,
            created: now()
          });
```

### 2. Error Simulation

```yaml
# 50% success, 50% error
- path_pattern: "/api/flaky"
  methods: ["GET"]
  status_code: 200
  response_mode: "script"
  script_body: |
    if (randomInt(0, 100) < 50) {
      response.status = 200;
      response.body = JSON.stringify({success: true});
    } else {
      response.status = 500;
      response.body = JSON.stringify({error: "Random failure"});
    }
```

### 3. Delayed Response

```yaml
# Simulate slow endpoint
- path_pattern: "/api/slow"
  methods: ["GET"]
  status_code: 200
  response_delay: 3000  # 3 seconds
  body: '{"data": "This took 3 seconds"}'
```

### 4. Authentication Mock

```yaml
# Login endpoint
- path_pattern: "/api/login"
  methods: ["POST"]
  status_code: 200
  response_mode: "script"
  request_validation:
    mode: "script"
    script: |
      (function() {
        try {
          const data = JSON.parse(request.body);
          if (!data.email || !data.password) {
            return {valid: false, error: "Missing credentials"};
          }
          return {valid: true};
        } catch (e) {
          return {valid: false, error: "Invalid JSON"};
        }
      })()
  script_body: |
    const creds = JSON.parse(request.body);
    response.status = 200;
    response.headers['Content-Type'] = 'application/json';
    response.body = JSON.stringify({
      token: base64Encode(creds.email + ':' + now()),
      expires: now()
    });

# Failed login (different validation)
- path_pattern: "/api/login"
  methods: ["POST"]
  status_code: 401
  request_validation:
    mode: "static"
    match_type: "contains"
    pattern: '"email":"bad@example.com"'
  body: '{"error": "Invalid credentials"}'
```

### 5. OpenAPI Import Enhancement

After importing an OpenAPI spec, you can customize generated responses:

```yaml
# Original generated response (from OpenAPI)
- path_pattern: "/api/users/:id"
  methods: ["GET"]
  status_code: 200
  response_mode: "script"
  script_body: |
    // Faker-generated mock data
    response.body = JSON.stringify({...});

# Add custom response for specific ID
- path_pattern: "/api/users/admin"
  methods: ["GET"]
  status_code: 200
  body: |
    {
      "id": "admin",
      "name": "Admin User",
      "role": "administrator"
    }
```

## Best Practices

### 1. Start Simple, Add Complexity as Needed

**Progression:**
1. Static mode for fixed responses
2. Template mode when you need request data
3. Script mode only for complex logic

### 2. Use Groups for Organization

Group related endpoints together:

```yaml
- type: group
  group:
    name: "Auth Endpoints"
    responses: [...]

- type: group
  group:
    name: "User Management"
    responses: [...]
```

### 3. Path Pattern Specificity

Order patterns from most specific to least specific:

```yaml
# Most specific first
- path_pattern: "/api/users/admin"      # Exact
- path_pattern: "/api/users/:id"        # Path param
- path_pattern: "/api/*"                # Wildcard
- path_pattern: "^/api/v[0-9]+/.*$"     # Regex
```

### 4. Script Performance

- Keep scripts simple and fast
- Avoid complex loops or recursion
- Remember: 5-second timeout applies
- Each request gets a fresh script execution (no state)

### 5. Error Handling in Scripts

Always handle errors gracefully:

```javascript
try {
    const data = JSON.parse(request.body);
    // ... process data
} catch (e) {
    response.status = 400;
    response.body = JSON.stringify({
        error: "Invalid request",
        details: e.toString()
    });
}
```

### 6. Content-Type Headers

Always set appropriate Content-Type:

```javascript
// JSON
response.headers['Content-Type'] = 'application/json';

// XML
response.headers['Content-Type'] = 'application/xml';

// Plain text
response.headers['Content-Type'] = 'text/plain';
```

### 7. Testing Patterns

1. **Create test group** - Group all test endpoints
2. **Use enable/disable** - Toggle groups without deleting
3. **Version your mocks** - Use path patterns like `/api/v1/`, `/api/v2/`
4. **Document scripts** - Add comments explaining complex logic

### 8. OpenAPI Integration

When using OpenAPI import:
- Generated responses use Script mode with Faker utilities
- Customize by adding your own static/template responses
- Exact path matches take precedence over generated patterns
- Use groups to separate generated vs custom endpoints

---

**Related Documentation:**
- [PROXY-GUIDE.md](PROXY-GUIDE.md) - Reverse proxy endpoints with header manipulation and body transformation
- [CONTAINER-GUIDE.md](CONTAINER-GUIDE.md) - Docker/Podman container endpoints
- [OPENAPI_IMPORT.md](OPENAPI_IMPORT.md) - Generate mock endpoints from OpenAPI specifications
- [SETUP.md](SETUP.md) - HTTPS configuration and deployment
