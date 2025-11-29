# Mockelot Configuration File Format

Mockelot (MAT) uses YAML configuration files to save and load server settings and response rules.

## File Extension

Configuration files use the `.yaml` or `.yml` extension.

## Structure Overview

MAT supports two configuration formats:

### Modern Format (with Groups)

```yaml
port: 8080
items:
  - type: response
    response:
      path_pattern: "/api/health"
      methods: [GET]
      status_code: 200
      body: '{"status": "ok"}'

  - type: group
    group:
      name: "User API"
      enabled: true
      responses:
        - path_pattern: "/api/users"
          methods: [GET]
          status_code: 200
          body: '[]'
```

### Legacy Format (flat list)

```yaml
port: 8080
responses:
  - path_pattern: "/*"
    methods: [GET, POST]
    status_code: 200
    body: '{"message": "Hello!"}'
```

Both formats are supported. The modern `items` format is recommended for new configurations.

---

## Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `port` | integer | Yes | Server port (1-65535) |
| `items` | array | No | List of response items (responses and groups) |
| `responses` | array | No | Legacy: flat list of responses |

---

## Response Item Structure

Each item in `items` has a `type` field:

| Type | Description |
|------|-------------|
| `response` | A single response rule |
| `group` | A named group of responses |

### Response Item

```yaml
- type: response
  response:
    # ... response fields
```

### Group Item

```yaml
- type: group
  group:
    name: "Group Name"
    enabled: true
    responses:
      - # ... response fields
```

---

## Response Rule Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `id` | string | No | auto | Unique identifier |
| `enabled` | boolean | No | true | Whether this response is active |
| `path_pattern` | string | Yes | - | URL path pattern (see Path Matching) |
| `methods` | array | Yes | - | HTTP methods: GET, POST, PUT, DELETE, PATCH, OPTIONS |
| `status_code` | integer | Yes | - | HTTP status code (200, 404, 500, etc.) |
| `status_text` | string | No | "" | Status text (e.g., "OK", "Not Found") |
| `headers` | object | No | {} | Response headers |
| `body` | string | No | "" | Response body (for static/template modes) |
| `response_delay` | integer | No | 0 | Delay in milliseconds |
| `response_mode` | string | No | "static" | Response mode: `static`, `template`, or `script` |
| `script_body` | string | No | "" | JavaScript code (for script mode) |
| `request_validation` | object | No | null | Request body validation config |

---

## Response Modes

### Static Mode (default)

Returns the body exactly as configured:

```yaml
response_mode: static
body: '{"message": "Hello, World!"}'
```

### Template Mode

Uses Go templates with request context:

```yaml
response_mode: template
body: |
  {
    "userId": "{{.PathParams.id}}",
    "method": "{{.Method}}",
    "timestamp": "{{now}}"
  }
```

**Available template variables:**
- `{{.Method}}` - HTTP method
- `{{.Path}}` - Request path
- `{{.PathParams.name}}` - Path parameter
- `{{.GetQueryParam "key"}}` - Query parameter
- `{{.GetHeader "name"}}` - Request header
- `{{.Body.Raw}}` - Raw request body
- `{{.Body.JSON.field}}` - Parsed JSON field
- `{{.Vars.name}}` - Variables extracted from validation

**Template functions:**
- `{{now}}` - Current timestamp (RFC3339)
- `{{timestamp}}` - Unix timestamp
- `{{uuid}}` - Generate UUID
- `{{upper "text"}}` / `{{lower "text"}}` - Case conversion
- `{{json .Value}}` / `{{jsonPretty .Value}}` - JSON encoding

### Script Mode

Full JavaScript control over the response:

```yaml
response_mode: script
script_body: |
  const userId = request.pathParams.id;

  response.status = 200;
  response.headers["Content-Type"] = "application/json";
  response.body = JSON.stringify({
    userId: userId,
    timestamp: Date.now()
  });
```

**Available objects:**
- `request.method`, `request.path`, `request.pathParams`, `request.queryParams`
- `request.headers`, `request.body.raw`, `request.body.json`
- `request.vars` - Variables from validation
- `response.status`, `response.headers`, `response.body`, `response.delay`

---

## Request Validation

Validate incoming request bodies and extract variables for use in responses.

### Validation Fields

| Field | Type | Description |
|-------|------|-------------|
| `mode` | string | `none`, `static`, `regex`, or `script` |
| `pattern` | string | Match pattern (for static/regex modes) |
| `match_type` | string | For static: `exact` or `contains` |
| `script` | string | JavaScript validation code |

### No Validation (default)

```yaml
request_validation:
  mode: none
```

### Static Validation

Check if body contains or matches text:

```yaml
request_validation:
  mode: static
  match_type: contains  # or "exact"
  pattern: '"action": "subscribe"'
```

### Regex Validation

Match patterns and extract named groups as variables:

```yaml
request_validation:
  mode: regex
  pattern: '"userId":\s*"(?P<userId>\d+)".*"action":\s*"(?P<action>\w+)"'
```

Extracted variables (`userId`, `action`) are available in templates as `{{.Vars.userId}}` or in scripts as `request.vars.userId`.

### Script Validation

Full JavaScript validation with variable extraction:

```yaml
request_validation:
  mode: script
  script: |
    const json = JSON.parse(body);

    result.valid = json.userId !== undefined;
    result.vars.userId = json.userId;
    result.vars.plan = json.plan || "free";
```

---

## Response Groups

Organize related responses and enable/disable them together:

```yaml
items:
  - type: group
    group:
      id: "error-responses"
      name: "Error Responses"
      enabled: true
      responses:
        - path_pattern: "/api/error"
          methods: [GET]
          status_code: 500
          body: '{"error": "Server error"}'

        - path_pattern: "/api/forbidden"
          methods: [GET]
          status_code: 403
          body: '{"error": "Forbidden"}'
```

### Group Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `id` | string | auto | Unique identifier |
| `name` | string | required | Display name |
| `enabled` | boolean | true | Enable/disable all responses in group |
| `expanded` | boolean | true | UI state (expanded/collapsed) |
| `responses` | array | [] | List of response rules |

---

## Path Matching

### Path Parameters

Extract values from URL paths:

```yaml
# Colon syntax
path_pattern: "/users/:id/posts/:postId"

# Brace syntax
path_pattern: "/users/{id}/posts/{postId}"
```

Access in templates: `{{.PathParams.id}}`, `{{.PathParams.postId}}`
Access in scripts: `request.pathParams.id`

### Wildcard Patterns

```yaml
path_pattern: "/api/*"        # Matches /api/anything
path_pattern: "/*"            # Catch-all
```

### Regex Patterns

Prefix with `^` for regex matching:

```yaml
path_pattern: "^/api/v[0-9]+/users"   # /api/v1/users, /api/v2/users
path_pattern: "^/items/[0-9]+$"        # /items/123
```

### Exact Match

```yaml
path_pattern: "/health"       # Only matches /health exactly
```

---

## Complete Example

```yaml
port: 8080
items:
  # Individual response with template
  - type: response
    response:
      path_pattern: "/api/users/:id"
      methods: [GET]
      status_code: 200
      response_mode: template
      headers:
        Content-Type: application/json
      body: |
        {
          "id": "{{.PathParams.id}}",
          "name": "User {{.PathParams.id}}",
          "requestedAt": "{{now}}"
        }

  # Response with validation
  - type: response
    response:
      path_pattern: "/api/webhook"
      methods: [POST]
      status_code: 200
      request_validation:
        mode: regex
        pattern: '"event":\s*"(?P<event>\w+)"'
      response_mode: template
      body: '{"received": "{{.Vars.event}}"}'

  # Scripted response
  - type: response
    response:
      path_pattern: "/api/calculate"
      methods: [POST]
      status_code: 200
      response_mode: script
      script_body: |
        const body = request.body.json || {};
        const a = body.a || 0;
        const b = body.b || 0;

        response.headers["Content-Type"] = "application/json";
        response.body = JSON.stringify({
          a: a,
          b: b,
          sum: a + b,
          product: a * b
        });

  # Group of error responses
  - type: group
    group:
      name: "Error Simulations"
      enabled: true
      responses:
        - path_pattern: "/api/error/500"
          methods: [GET, POST]
          status_code: 500
          response_delay: 100
          body: '{"error": "Internal Server Error"}'

        - path_pattern: "/api/error/timeout"
          methods: [GET]
          status_code: 504
          response_delay: 30000
          body: '{"error": "Gateway Timeout"}'

  # Catch-all (should be last)
  - type: response
    response:
      path_pattern: "/*"
      methods: [GET, POST, PUT, DELETE, PATCH, OPTIONS]
      status_code: 404
      body: '{"error": "Not found"}'
```

---

## Rule Priority

Rules are evaluated **in order from top to bottom**. The first matching rule wins.

1. Put specific rules first
2. Put general/catch-all rules last
3. Groups are evaluated in their position within the items list
4. Within a group, responses are evaluated in order

---

## Loading and Saving

- **Save Config**: Click "Save Config" in the header to export
- **Load Config**: Click "Load Config" to import

When loading:
- Port setting is updated
- All items/responses are replaced
- Changes take effect immediately if server is running
