# Response Modes Guide

Mockelot (MAT) supports three response modes that give you flexibility in how responses are generated: **Static**, **Template**, and **Script**.

## Static Mode (Default)

Static mode returns the configured response body exactly as entered. No processing or variable substitution is performed.

**Use when:**
- You need simple, fixed responses
- Response content doesn't depend on request data
- Maximum performance is required

**Example:**
```json
{
  "status": "ok",
  "message": "Hello, World!"
}
```

---

## Template Mode

Template mode uses Go's [text/template](https://pkg.go.dev/text/template) engine to dynamically generate responses based on request data.

### Available Variables

| Variable | Description | Example Value |
|----------|-------------|---------------|
| `{{.Method}}` | HTTP method | `GET`, `POST` |
| `{{.Path}}` | Request path | `/api/users/123` |
| `{{.PathParams.name}}` | Path parameter value | `123` (from `/users/:id`) |
| `{{.QueryParams.key}}` | Query parameters (array) | `["value1", "value2"]` |
| `{{.Headers}}` | Request headers map | `{"Content-Type": ["application/json"]}` |
| `{{.Body.Raw}}` | Raw request body string | `{"name": "John"}` |
| `{{.Body.Json}}` | Parsed JSON body | Accessible as map |
| `{{.Body.Form}}` | Parsed form data | `{"field": ["value"]}` |

### Helper Functions

| Function | Description | Example |
|----------|-------------|---------|
| `{{.GetQueryParam "key"}}` | Get first query param value | `{{.GetQueryParam "page"}}` → `1` |
| `{{.GetHeader "name"}}` | Get first header value | `{{.GetHeader "Authorization"}}` |
| `{{json .Value}}` | JSON encode a value | `{{json .PathParams}}` |
| `{{jsonPretty .Value}}` | Pretty-print JSON | `{{jsonPretty .Body.Json}}` |
| `{{now}}` | Current timestamp (RFC3339) | `2024-01-15T10:30:00Z` |
| `{{timestamp}}` | Unix timestamp (seconds) | `1705315800` |
| `{{timestampMs}}` | Unix timestamp (milliseconds) | `1705315800000` |
| `{{upper .Value}}` | Uppercase string | `{{upper .Method}}` → `GET` |
| `{{lower .Value}}` | Lowercase string | `{{lower .Method}}` → `get` |
| `{{default "fallback" .Value}}` | Default if empty | `{{default "guest" .PathParams.user}}` |

### Path Parameters

Path parameters are extracted from URL patterns. Two syntaxes are supported:

**Colon syntax:** `/users/:id/posts/:postId`
**Brace syntax:** `/users/{id}/posts/{postId}`

Both extract parameters accessible via `{{.PathParams.id}}` and `{{.PathParams.postId}}`.

### Template Examples

**Echo user ID from path:**
```
Path Pattern: /users/:id
Response Body:
{
  "userId": "{{.PathParams.id}}",
  "requestedAt": "{{now}}"
}
```

**Dynamic response with query params:**
```
Path Pattern: /search
Response Body:
{
  "query": "{{.GetQueryParam "q"}}",
  "page": {{default 1 (.GetQueryParam "page")}},
  "results": []
}
```

**Echo request details:**
```json
{
  "method": "{{.Method}}",
  "path": "{{.Path}}",
  "params": {{json .PathParams}},
  "receivedBody": {{json .Body.Json}}
}
```

---

## Script Mode

Script mode executes JavaScript code using the [goja](https://github.com/dop251/goja) engine, providing full programmatic control over the response.

### Available Objects

#### `request` (read-only)

| Property | Type | Description |
|----------|------|-------------|
| `request.method` | string | HTTP method (`GET`, `POST`, etc.) |
| `request.path` | string | Request path |
| `request.pathParams` | object | Path parameters `{id: "123"}` |
| `request.queryParams` | object | Query params `{page: ["1"], sort: ["asc"]}` |
| `request.headers` | object | Headers `{"Content-Type": ["application/json"]}` |
| `request.body.raw` | string | Raw body string |
| `request.body.json` | object | Parsed JSON body (or null) |
| `request.body.form` | object | Parsed form data (or null) |

#### `response` (writable)

| Property | Type | Description |
|----------|------|-------------|
| `response.status` | number | HTTP status code (default: from config) |
| `response.headers` | object | Response headers `{"Content-Type": "..."}` |
| `response.body` | string | Response body |
| `response.delay` | number | Response delay in milliseconds |

### Available Functions

- `JSON.stringify(obj)` - Convert object to JSON string
- `JSON.stringify(obj, null, 2)` - Pretty-print JSON
- `JSON.parse(str)` - Parse JSON string
- `console.log(...)` - Debug logging (output not visible)

### Script Examples

**Echo request body:**
```javascript
response.body = request.body.raw;
```

**JSON response with path params:**
```javascript
const userId = request.pathParams.id;
response.headers["Content-Type"] = "application/json";
response.body = JSON.stringify({
  userId: userId,
  found: true,
  timestamp: Date.now()
});
```

**Conditional response based on method:**
```javascript
if (request.method === "POST") {
  response.status = 201;
  response.body = JSON.stringify({ created: true });
} else if (request.method === "DELETE") {
  response.status = 204;
  response.body = "";
} else {
  response.status = 200;
  response.body = JSON.stringify({ data: [] });
}
```

**Use query parameters:**
```javascript
const page = request.queryParams.page
  ? parseInt(request.queryParams.page[0])
  : 1;
const limit = request.queryParams.limit
  ? parseInt(request.queryParams.limit[0])
  : 10;

response.body = JSON.stringify({
  page: page,
  limit: limit,
  offset: (page - 1) * limit
});
```

**Parse and transform JSON body:**
```javascript
const body = request.body.json || {};
const name = body.name || "Unknown";

response.status = 200;
response.headers["Content-Type"] = "application/json";
response.body = JSON.stringify({
  greeting: "Hello, " + name + "!",
  received: body
});
```

**Dynamic delay:**
```javascript
// Add artificial latency based on query param
const delay = request.queryParams.delay
  ? parseInt(request.queryParams.delay[0])
  : 0;
response.delay = Math.min(delay, 5000); // Cap at 5 seconds
response.body = JSON.stringify({ delayed: delay + "ms" });
```

### Script Limitations

- **5-second timeout:** Scripts that run longer are terminated
- **No external access:** Cannot make HTTP requests or access filesystem
- **Single-threaded:** No async/await or setTimeout

---

## Tips

1. **Start with Static mode** for simple mocks, then upgrade to Template or Script when you need dynamic behavior.

2. **Use Template mode** when you just need to inject request values into a response structure.

3. **Use Script mode** when you need conditional logic, data transformation, or complex response generation.

4. **Path patterns** support both simple wildcards and regex:
   - Simple: `/api/*` matches any path under `/api/`
   - Regex: `^/api/v[0-9]+/` matches versioned API paths
   - Parameters: `/users/:id` or `/users/{id}` extract path segments

5. **Headers in templates** are also processed, so you can do:
   ```
   X-Request-Id: {{.GetHeader "X-Request-Id"}}
   ```

6. **Test your templates** by making requests and checking the response. Errors fall back to the raw template text.
