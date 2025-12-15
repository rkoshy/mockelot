# Proxy Endpoint Guide

Proxy endpoints act as a reverse proxy, forwarding requests to a backend server while providing powerful manipulation capabilities. They're ideal for testing, debugging, and augmenting existing APIs.

## Table of Contents

- [Overview](#overview)
- [Creating Proxy Endpoints](#creating-proxy-endpoints)
- [Path Translation](#path-translation)
- [Header Manipulation](#header-manipulation)
- [Status Code Translation](#status-code-translation)
- [Body Transformation](#body-transformation)
- [Health Checks](#health-checks)
- [WebSocket Support](#websocket-support)
- [Common Use Cases](#common-use-cases)
- [Best Practices](#best-practices)

## Overview

Proxy endpoints forward HTTP requests to a backend URL while allowing you to:
- **Translate paths** - Modify request paths before forwarding
- **Manipulate headers** - Add, remove, or modify headers in both directions
- **Transform bodies** - Modify response bodies using JavaScript
- **Translate status codes** - Change backend status codes
- **Monitor health** - Automatic backend health checking
- **Proxy WebSockets** - Full WebSocket support

**Request Flow:**
```
Client → Mockelot Proxy → Backend Server
         ↓ (manipulate)
Client ← Mockelot Proxy ← Backend Server
```

## Creating Proxy Endpoints

### Via UI

1. Click **"Add Endpoint"**
2. Select **"Proxy"** as endpoint type
3. Configure:
   - Name (e.g., "Production API")
   - Path prefix (e.g., "/api")
   - Backend URL (e.g., "https://api.example.com")
4. Optionally configure path translation, headers, etc.

### Via YAML Configuration

```yaml
endpoints:
  - id: "proxy-1"
    name: "Production API"
    path_prefix: "/api"
    type: "proxy"
    enabled: true
    translation_mode: "strip"
    proxy_config:
      backend_url: "https://api.example.com"
      timeout_seconds: 30
```

## Path Translation

Path translation determines how the request path is modified before forwarding to the backend.

### Translation Modes

#### 1. None (No Translation)

Forward the path exactly as received.

```yaml
path_prefix: "/api"
translation_mode: "none"
proxy_config:
  backend_url: "https://backend.com"

# Client requests: /api/users
# Backend receives: https://backend.com/api/users
```

**When to use:**
- Backend expects the same path structure
- Simple passthrough proxy

#### 2. Strip Prefix

Remove the path prefix before forwarding.

```yaml
path_prefix: "/api"
translation_mode: "strip"
proxy_config:
  backend_url: "https://backend.com"

# Client requests: /api/users
# Backend receives: https://backend.com/users
```

**When to use:**
- Backend doesn't use the `/api` prefix
- Proxying to root-level backend endpoints
- Most common mode for API proxying

#### 3. Regex Translation

Use regex pattern matching and replacement.

```yaml
path_prefix: "/api"
translation_mode: "translate"
translate_pattern: "^/api/v([0-9]+)/(.*)$"
translate_replace: "/v$1/api/$2"
proxy_config:
  backend_url: "https://backend.com"

# Client requests: /api/v1/users
# Backend receives: https://backend.com/v1/api/users
```

**When to use:**
- Complex path restructuring
- Version routing
- Path component reordering

**Capture Groups:**

You can use capture groups in the backend URL:

```yaml
translate_pattern: "^/proxy/([^/]+)/(.*)$"
proxy_config:
  backend_url: "https://$1.example.com/$2"

# Client requests: /proxy/api/users
# Backend receives: https://api.example.com/users
```

### Path Translation Examples

**Example 1: API Gateway**
```yaml
# Route /external to external service
path_prefix: "/external"
translation_mode: "strip"
proxy_config:
  backend_url: "https://external-api.com"

# /external/data → https://external-api.com/data
```

**Example 2: Version Routing**
```yaml
# Route versioned APIs to different backends
path_prefix: "/api/v1"
translation_mode: "strip"
proxy_config:
  backend_url: "https://v1.api.example.com"

# /api/v1/users → https://v1.api.example.com/users
```

**Example 3: Service Mesh**
```yaml
# Dynamic routing to services
path_prefix: "/services"
translation_mode: "translate"
translate_pattern: "^/services/([^/]+)/(.*)$"
proxy_config:
  backend_url: "http://$1.svc.cluster.local/$2"

# /services/auth/login → http://auth.svc.cluster.local/login
```

## Header Manipulation

Headers can be manipulated in both directions (inbound to backend, outbound to client).

### Header Modes

#### 1. Drop

Remove a header entirely.

```yaml
inbound_headers:
  - name: "X-Custom-Header"
    mode: "drop"
```

#### 2. Replace

Set a header to a static value.

```yaml
inbound_headers:
  - name: "Authorization"
    mode: "replace"
    value: "Bearer static-token-for-testing"
```

#### 3. Expression

Use JavaScript to compute header value dynamically.

```yaml
inbound_headers:
  - name: "X-Forwarded-For"
    mode: "expression"
    expression: "request.remoteAddr"

  - name: "X-Request-ID"
    mode: "expression"
    expression: "Math.random().toString(36).substring(7)"
```

### Available Context in Expressions

```javascript
request.method          // HTTP method
request.path            // Request path
request.headers         // Request headers object
request.host            // Request host
request.remoteAddr      // Client IP
request.scheme          // "http" or "https"
request.tls             // Boolean - true if HTTPS
```

### Inbound Headers (Client → Backend)

Modify headers before forwarding to backend.

```yaml
proxy_config:
  inbound_headers:
    # Add authentication
    - name: "Authorization"
      mode: "replace"
      value: "Bearer secret-token"

    # Add client IP
    - name: "X-Real-IP"
      mode: "expression"
      expression: "request.remoteAddr"

    # Remove problematic headers
    - name: "Cookie"
      mode: "drop"

    # Add custom tracking
    - name: "X-Proxy-Time"
      mode: "expression"
      expression: "new Date().toISOString()"
```

**Automatic Inbound Headers:**

Mockelot automatically handles these headers:
- Strips hop-by-hop headers (`Connection`, `Keep-Alive`, etc.)
- Sets `Host` header to backend host
- Adds `X-Forwarded-For` with client IP
- Adds `X-Forwarded-Proto` with original scheme

### Outbound Headers (Backend → Client)

Modify headers before returning to client.

```yaml
proxy_config:
  outbound_headers:
    # Remove backend internal headers
    - name: "X-Backend-Server"
      mode: "drop"

    # Add CORS headers
    - name: "Access-Control-Allow-Origin"
      mode: "replace"
      value: "*"

    # Add cache control
    - name: "Cache-Control"
      mode: "replace"
      value: "no-cache, no-store, must-revalidate"

    # Add custom response header
    - name: "X-Served-By"
      mode: "replace"
      value: "Mockelot Proxy"
```

### Header Manipulation Examples

**Example 1: API Key Injection**
```yaml
# Add API key for backend authentication
inbound_headers:
  - name: "X-API-Key"
    mode: "replace"
    value: "your-backend-api-key"
```

**Example 2: CORS Enablement**
```yaml
# Enable CORS for frontend testing
outbound_headers:
  - name: "Access-Control-Allow-Origin"
    mode: "replace"
    value: "*"
  - name: "Access-Control-Allow-Methods"
    mode: "replace"
    value: "GET, POST, PUT, DELETE, OPTIONS"
  - name: "Access-Control-Allow-Headers"
    mode: "replace"
    value: "Content-Type, Authorization"
```

**Example 3: Request Tracking**
```yaml
# Add unique request ID
inbound_headers:
  - name: "X-Request-ID"
    mode: "expression"
    expression: "Math.random().toString(36).substring(2, 15)"
```

**Example 4: Conditional Headers**
```yaml
# Add header based on request path
inbound_headers:
  - name: "X-Service"
    mode: "expression"
    expression: |
      request.path.startsWith('/admin') ? 'admin-service' : 'api-service'
```

## Status Code Translation

Translate backend status codes to different values.

### Configuration

```yaml
proxy_config:
  status_passthrough: false  # Enable translation
  status_translation:
    - from_pattern: "404"
      to_code: 200
    - from_pattern: "5xx"
      to_code: 503
    - from_pattern: "2xx"
      to_code: 200
```

### Pattern Matching

**Exact Match:**
```yaml
- from_pattern: "404"
  to_code: 200
# Translates: 404 → 200
```

**Wildcard Match:**
```yaml
- from_pattern: "5xx"
  to_code: 503
# Translates: 500, 501, 502, etc. → 503

- from_pattern: "2xx"
  to_code: 200
# Translates: 201, 204, etc. → 200
```

### Translation Examples

**Example 1: Hide Backend Errors**
```yaml
# Convert all 5xx to 503 Service Unavailable
status_passthrough: false
status_translation:
  - from_pattern: "5xx"
    to_code: 503
```

**Example 2: Normalize Success Codes**
```yaml
# Convert all 2xx to 200 OK
status_passthrough: false
status_translation:
  - from_pattern: "2xx"
    to_code: 200
```

**Example 3: Custom Error Mapping**
```yaml
# Convert backend 401/403 to 404 (security through obscurity)
status_passthrough: false
status_translation:
  - from_pattern: "401"
    to_code: 404
  - from_pattern: "403"
    to_code: 404
```

## Body Transformation

Transform response bodies using JavaScript before returning to client.

### Configuration

```yaml
proxy_config:
  body_transform: |
    // Parse JSON response
    const data = JSON.parse(body);

    // Modify data
    data.transformed = true;
    data.timestamp = new Date().toISOString();

    // Return modified JSON
    return JSON.stringify(data, null, 2);
```

### Available Context

```javascript
body            // Response body as string
contentType     // Content-Type header value

JSON.parse()    // Parse JSON string
JSON.stringify()// Convert to JSON string
```

### Transformation Examples

**Example 1: Add Metadata**
```javascript
const data = JSON.parse(body);
data._metadata = {
  proxied: true,
  timestamp: new Date().toISOString(),
  source: "mockelot"
};
return JSON.stringify(data);
```

**Example 2: Filter Sensitive Data**
```javascript
const data = JSON.parse(body);

// Remove sensitive fields
delete data.password;
delete data.ssn;
delete data.creditCard;

return JSON.stringify(data);
```

**Example 3: Wrap Response**
```javascript
const data = JSON.parse(body);

return JSON.stringify({
  success: true,
  data: data,
  timestamp: new Date().toISOString()
});
```

**Example 4: Extract Nested Data**
```javascript
// Backend returns: {"result": {"data": {"users": [...]}}}
// Transform to: {"users": [...]}

const response = JSON.parse(body);
return JSON.stringify(response.result.data);
```

**Example 5: Format XML to JSON**
```javascript
// For XML responses, you'd need to parse XML first
// This is a simplified example for JSON

const data = JSON.parse(body);

// Convert flat structure to nested
return JSON.stringify({
  users: data.items.map(item => ({
    id: item.id,
    name: item.name,
    profile: {
      email: item.email,
      age: item.age
    }
  }))
});
```

## Health Checks

Automatic backend health monitoring with configurable intervals.

### Configuration

```yaml
proxy_config:
  health_check_enabled: true
  health_check_interval: 30  # seconds
  health_check_path: "/health"
```

### How It Works

1. Mockelot periodically sends GET requests to `backend_url + health_check_path`
2. Status codes 200-499 are considered healthy
3. Status codes ≥500 or network errors are unhealthy
4. Health status is logged and can be monitored

### Health Check Examples

**Example 1: Basic Health Check**
```yaml
proxy_config:
  backend_url: "https://api.example.com"
  health_check_enabled: true
  health_check_interval: 60  # Check every minute
  health_check_path: "/health"
```

**Example 2: Custom Health Endpoint**
```yaml
proxy_config:
  backend_url: "https://api.example.com"
  health_check_enabled: true
  health_check_interval: 10  # Check every 10 seconds
  health_check_path: "/api/status"
```

**Example 3: Root Path Check**
```yaml
proxy_config:
  backend_url: "https://api.example.com"
  health_check_enabled: true
  health_check_interval: 30
  health_check_path: "/"  # Check root
```

## WebSocket Support

Proxy endpoints automatically detect and proxy WebSocket connections.

### How It Works

1. Client sends WebSocket upgrade request
2. Mockelot detects `Upgrade: websocket` header
3. Establishes WebSocket connection to backend
4. Bidirectional message forwarding (Client ↔ Backend)

### Configuration

No special configuration needed - WebSocket support is automatic:

```yaml
# Regular proxy endpoint automatically supports WebSockets
path_prefix: "/ws"
type: "proxy"
translation_mode: "strip"
proxy_config:
  backend_url: "ws://backend.com"  # or wss:// for secure
```

### WebSocket Examples

**Example 1: WebSocket Gateway**
```yaml
# Proxy WebSocket connections
path_prefix: "/socket"
translation_mode: "strip"
proxy_config:
  backend_url: "ws://websocket-server.com"

# Client connects to: ws://localhost:8080/socket
# Backend connection: ws://websocket-server.com/
```

**Example 2: Secure WebSocket**
```yaml
# Proxy to secure WebSocket backend
path_prefix: "/chat"
translation_mode: "none"
proxy_config:
  backend_url: "wss://chat.example.com"

# Client connects to: ws://localhost:8080/chat
# Backend connection: wss://chat.example.com/chat
```

## Common Use Cases

### 1. Development Proxy

Proxy to production API during development:

```yaml
endpoints:
  - name: "Production API"
    path_prefix: "/api"
    type: "proxy"
    translation_mode: "strip"
    proxy_config:
      backend_url: "https://api.production.com"
      timeout_seconds: 30

      # Add authentication
      inbound_headers:
        - name: "Authorization"
          mode: "replace"
          value: "Bearer dev-token"

      # Enable CORS for local dev
      outbound_headers:
        - name: "Access-Control-Allow-Origin"
          mode: "replace"
          value: "http://localhost:3000"
```

### 2. API Debugging

Debug API issues with logging and transformation:

```yaml
endpoints:
  - name: "Debug Proxy"
    path_prefix: "/debug"
    type: "proxy"
    translation_mode: "strip"
    proxy_config:
      backend_url: "https://api.example.com"

      # Add request tracking
      inbound_headers:
        - name: "X-Request-ID"
          mode: "expression"
          expression: "Math.random().toString(36).substr(2, 9)"

      # Log response metadata
      body_transform: |
        const data = JSON.parse(body);
        data._debug = {
          timestamp: new Date().toISOString(),
          proxied: true
        };
        return JSON.stringify(data, null, 2);
```

### 3. Service Mesh / API Gateway

Route to multiple backend services:

```yaml
endpoints:
  # Auth service
  - name: "Auth Service"
    path_prefix: "/auth"
    type: "proxy"
    translation_mode: "strip"
    proxy_config:
      backend_url: "http://auth-service:8080"

  # User service
  - name: "User Service"
    path_prefix: "/users"
    type: "proxy"
    translation_mode: "strip"
    proxy_config:
      backend_url: "http://user-service:8080"

  # Order service
  - name: "Order Service"
    path_prefix: "/orders"
    type: "proxy"
    translation_mode: "strip"
    proxy_config:
      backend_url: "http://order-service:8080"
```

### 4. Legacy API Modernization

Transform legacy API responses:

```yaml
endpoints:
  - name: "Legacy API Wrapper"
    path_prefix: "/api/v2"
    type: "proxy"
    translation_mode: "translate"
    translate_pattern: "^/api/v2/(.*)$"
    translate_replace: "/legacy/$1"
    proxy_config:
      backend_url: "http://legacy-system.com"

      # Transform old response format to new
      body_transform: |
        const legacy = JSON.parse(body);
        return JSON.stringify({
          version: "2.0",
          data: legacy.result,
          metadata: {
            timestamp: new Date().toISOString(),
            source: "legacy"
          }
        });
```

### 5. Testing Error Scenarios

Simulate backend failures:

```yaml
endpoints:
  - name: "Unreliable Backend"
    path_prefix: "/flaky"
    type: "proxy"
    translation_mode: "strip"
    proxy_config:
      backend_url: "https://api.example.com"

      # Convert random 2xx to 5xx for testing
      status_passthrough: false
      status_translation:
        - from_pattern: "200"
          to_code: 500  # Simulate backend errors
```

### 6. Response Caching Headers

Add cache control to backend responses:

```yaml
endpoints:
  - name: "Cached API"
    path_prefix: "/cached"
    type: "proxy"
    translation_mode: "strip"
    proxy_config:
      backend_url: "https://api.example.com"

      outbound_headers:
        - name: "Cache-Control"
          mode: "replace"
          value: "public, max-age=3600"
        - name: "ETag"
          mode: "expression"
          expression: "Math.random().toString(36).substring(7)"
```

## Best Practices

### 1. Path Translation Strategy

- **Use "strip" mode** for most cases (removes prefix)
- **Use "none" mode** when backend expects same path
- **Use "translate" mode** only for complex routing
- **Test path translation** thoroughly - check logs to verify correct backend URLs

### 2. Header Security

**Do:**
- Strip internal headers before forwarding to client
- Add authentication headers for backend
- Use expressions for dynamic values
- Document all header modifications

**Don't:**
- Expose backend server information
- Forward sensitive headers to untrusted clients
- Hardcode secrets in configuration (use environment variables)

### 3. Body Transformation

**Do:**
- Keep transformations simple and fast
- Handle parse errors gracefully
- Test with real backend responses
- Document transformation logic

**Don't:**
- Transform large responses (memory intensive)
- Use complex logic (5-second timeout applies)
- Modify binary data (only works with text)

### 4. Error Handling

Always consider error scenarios:
- Backend unavailable (connection refused)
- Backend timeout (slow response)
- Backend error (5xx status)
- Invalid response body (parse errors)

```yaml
# Use status translation to normalize errors
status_passthrough: false
status_translation:
  - from_pattern: "5xx"
    to_code: 503
```

### 5. Health Checks

**Best practices:**
- Enable health checks for critical backends
- Use dedicated health endpoints (not production endpoints)
- Set appropriate intervals (30-60 seconds typical)
- Monitor health check logs

### 6. Performance

**Optimize for performance:**
- Minimize header manipulations
- Avoid complex body transformations
- Use appropriate timeouts (default 30s)
- Consider backend response times

### 7. Logging and Monitoring

**Enable traffic logging:**
- All proxy requests are logged in Traffic Log panel
- Logs include both client and backend timing
- Use Request Inspector to view full request/response details
- Monitor RTT (round-trip time) for performance issues

### 8. Testing Workflow

1. **Start simple** - Basic proxy with no manipulation
2. **Add path translation** - Verify correct backend URLs
3. **Add headers** - Test one at a time
4. **Add transformations** - Test with real responses
5. **Enable health checks** - Monitor backend status

### 9. WebSocket Considerations

- WebSocket connections are long-lived (not HTTP request/response)
- No header manipulation applied to WebSocket messages
- No body transformation applied to WebSocket frames
- Connection established once, then bidirectional forwarding

### 10. HTTPS Backend Support

Proxy supports both HTTP and HTTPS backends:

```yaml
# HTTPS backend
proxy_config:
  backend_url: "https://api.example.com"

# HTTP backend
proxy_config:
  backend_url: "http://internal-service:8080"
```

Certificate verification is enabled by default. For self-signed certificates, you may need to configure system trust stores.

---

**Related Documentation:**
- [MOCK-GUIDE.md](MOCK-GUIDE.md) - Mock endpoints with static, template, and script responses
- [CONTAINER-GUIDE.md](CONTAINER-GUIDE.md) - Docker/Podman container endpoints
- [OPENAPI_IMPORT.md](OPENAPI_IMPORT.md) - Generate endpoints from OpenAPI specifications
- [SETUP.md](SETUP.md) - HTTPS configuration and deployment
