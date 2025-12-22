# Mockelot Architecture

## Container ↔ Proxy Unification

### Overview

Containers in Mockelot are **first-class proxy endpoints**. They share all header manipulation, status translation, body transformation, and health check logic with standard proxy endpoints. The only difference is that containers have a **dynamic backend URL** (`http://127.0.0.1:{hostPort}`) instead of a static one.

### Core Architectural Principle

```
Client Request
  ↓
ProxyConfig (header manipulation, health checks, status codes, etc.)
  ↓
Backend (either static proxy URL OR dynamic container at 127.0.0.1:PORT)
  ↓
Backend Response
  ↓
ProxyConfig (outbound header manipulation, body transformation, etc.)
  ↓
Client Response
```

**Containers are proxies with dynamic backends.**

---

## Data Model

### ContainerConfig Embedding

`ContainerConfig` **embeds** `ProxyConfig`, inheriting all proxy features:

```go
type ContainerConfig struct {
    // Proxy configuration - handles HTTP proxying to the container
    // Note: BackendURL is not used for containers (dynamically set to http://127.0.0.1:{hostPort})
    ProxyConfig ProxyConfig `json:"proxy_config" yaml:"proxy_config"`

    // Container-specific fields (image, volumes, environment, etc.)
    ImageName     string
    ContainerPort int
    Volumes       []VolumeMapping
    Environment   []EnvironmentVar
    // ...
}
```

This means containers get:
- Header manipulation (inbound/outbound)
- Status code translation
- Body transformation (JS scripts)
- Health checks (HTTP endpoint polling)
- Timeouts

---

## Shared ProxyHandler

### Single Handler for All Proxying

Both standard proxies and containers use the **same `ProxyHandler`** instance:

```go
type App struct {
    proxyHandler     *ProxyHandler     // Shared between HTTPServer and ContainerHandler
    containerHandler *ContainerHandler
    server           *HTTPServer
}
```

### Initialization Order

1. **App.NewApp()**: Creates shared `ProxyHandler`
2. **App.NewApp()**: Creates `ContainerHandler`, passes shared `ProxyHandler`
3. **App.StartServer()**: Creates `HTTPServer`, passes shared `ProxyHandler`

This ensures consistency: all proxy logic flows through a single handler.

---

## Header Manipulation

### Context-Aware Header Processing

The `ProxyHandler` provides two methods:

```go
// For standard proxies (no custom context)
func (p *ProxyHandler) applyHeaderManipulation(
    headers http.Header,
    manipulations []HeaderManipulation,
    originalReq *http.Request,
)

// For containers (with custom context for dynamic port)
func (p *ProxyHandler) applyHeaderManipulationWithContext(
    headers http.Header,
    manipulations []HeaderManipulation,
    originalReq *http.Request,
    customContext map[string]interface{},
)
```

### JavaScript Expression Context

Header expressions run in a Goja VM with this context:

```javascript
request = {
    method:     "GET",
    path:       "/api/users",
    headers:    {...},
    host:       "example.com",
    remoteAddr: "192.168.1.100:54321",
    scheme:     "https",  // or "http"
    tls:        true,     // or false

    // Container-specific (via customContext)
    hostPort:   "32768"   // Dynamic container port
}
```

### Container Custom Context

`ContainerHandler.ServeHTTP` injects `hostPort` into the expression context:

```go
customContext := map[string]interface{}{
    "hostPort": hostPort,  // e.g., "32768"
}

c.proxyHandler.applyHeaderManipulationWithContext(
    backendReq.Header,
    cfg.ProxyConfig.InboundHeaders,
    r,
    customContext,
)
```

---

## Default Container Headers

### Why Defaults Are Needed

Without proper header handling, containers produce **malformed redirects** because:
- Client sends `Host: example.com`
- Container receives `Host: example.com`
- Container generates redirect: `http://example.com/login`
- **Wrong!** Should be `http://127.0.0.1:PORT/login`

### Default Rules

All container endpoints get these **default inbound headers**:

```go
func DefaultContainerInboundHeaders() []HeaderManipulation {
    return []HeaderManipulation{
        // 1. Drop hop-by-hop headers (RFC 7230 section 6.1)
        {Name: "Connection", Mode: HeaderModeDrop},
        {Name: "Keep-Alive", Mode: HeaderModeDrop},
        {Name: "Proxy-Authenticate", Mode: HeaderModeDrop},
        {Name: "Proxy-Authorization", Mode: HeaderModeDrop},
        {Name: "Te", Mode: HeaderModeDrop},
        {Name: "Trailers", Mode: HeaderModeDrop},
        {Name: "Transfer-Encoding", Mode: HeaderModeDrop},
        {Name: "Upgrade", Mode: HeaderModeDrop},

        // 2. Set Host to container backend (dynamic port)
        {
            Name:       "Host",
            Mode:       HeaderModeExpression,
            Expression: `"127.0.0.1:" + request.hostPort`,
        },

        // 3. Add X-Forwarded-* headers (proxy transparency)
        {
            Name:       "X-Forwarded-For",
            Mode:       HeaderModeExpression,
            Expression: `request.remoteAddr`,
        },
        {
            Name:       "X-Forwarded-Host",
            Mode:       HeaderModeExpression,
            Expression: `request.host`,
        },
        {
            Name:       "X-Forwarded-Proto",
            Mode:       HeaderModeExpression,
            Expression: `request.scheme`,
        },
    }
}
```

### Application Points

Defaults are applied automatically when creating container endpoints:

1. **`AddEndpoint()`** (line 591 in app.go):
   ```go
   ProxyConfig: models.ProxyConfig{
       InboundHeaders: models.DefaultContainerInboundHeaders(),
       // ...
   }
   ```

2. **`AddEndpointWithConfig()`** (line 691 in app.go):
   ```go
   ProxyConfig: models.ProxyConfig{
       InboundHeaders: models.DefaultContainerInboundHeaders(),
       // ...
   }
   ```

Users can **override** these defaults via the UI or configuration files.

---

## Request Flow

### Container Request Lifecycle

```
1. Client → HTTPServer
   └─ Match endpoint by PathPrefix

2. HTTPServer → ContainerHandler.ServeHTTP()
   ├─ Inspect container to get dynamic port
   ├─ Build backend URL: http://127.0.0.1:{hostPort}{translatedPath}
   ├─ Create backend request
   └─ Apply inbound headers via ProxyHandler:
       customContext = {hostPort: "32768"}
       applyHeaderManipulationWithContext(backendReq.Header, cfg.ProxyConfig.InboundHeaders, r, customContext)

3. ContainerHandler → Container (127.0.0.1:32768)
   └─ Backend request with:
       - Host: 127.0.0.1:32768
       - X-Forwarded-For: <client IP>
       - X-Forwarded-Host: <original Host>
       - X-Forwarded-Proto: http/https

4. Container → ContainerHandler
   └─ Backend response

5. ContainerHandler → Client
   └─ Apply outbound headers (same ProxyHandler logic)
```

---

## Code Locations

### Core Files

| File | Lines | Responsibility |
|------|-------|----------------|
| `models/models.go` | 163-184 | `DefaultContainerInboundHeaders()` function |
| `models/models.go` | 176-206 | `ContainerConfig` struct (embeds ProxyConfig) |
| `server/proxy.go` | 248-296 | `applyHeaderManipulationWithContext()` |
| `server/container.go` | 29-43 | `ContainerHandler` struct (holds shared ProxyHandler) |
| `server/container.go` | 437-442 | `ServeHTTP()` calls ProxyHandler with custom context |
| `app.go` | 91-96 | ProxyHandler initialization and sharing |
| `app.go` | 585-598 | Container endpoint creation with defaults |
| `app.go` | 684-743 | Container endpoint creation from wizard |

### Key Commits

- Initial refactor: Embed ProxyConfig into ContainerConfig
- ProxyHandler sharing: Single handler for proxies and containers
- Custom context: `applyHeaderManipulationWithContext()` method
- Default headers: `DefaultContainerInboundHeaders()` function
- Final integration: Apply defaults at container creation

---

## Testing & Validation

### Edge Cases Handled

1. **Host Networking**: When container uses host network, port mapping doesn't exist
   - **Solution**: Health checks fail gracefully, ServeHTTP returns 503

2. **TLS/HTTPS**: X-Forwarded-Proto correctly reflects client protocol
   - **Solution**: Expression context includes `request.scheme` and `request.tls`

3. **IPv6**: RemoteAddr may be IPv6 format
   - **Solution**: No special handling needed, passed through as-is

4. **Port Collision**: Multiple containers on same dynamic port
   - **Solution**: Docker/Podman handles port allocation automatically

5. **Custom Headers Override**: User configures custom inbound headers
   - **Solution**: Wizard parsing in `AddEndpointWithConfig()` line 707-713

### Manual Testing Checklist

- [ ] Create container endpoint via UI
- [ ] Verify default headers appear in ProxyConfig
- [ ] Start container, verify dynamic port is detected
- [ ] Send request, verify Host header is `127.0.0.1:PORT`
- [ ] Verify X-Forwarded-* headers are added
- [ ] Test container redirect (e.g., HTTP 302)
- [ ] Verify redirect URL uses 127.0.0.1:PORT
- [ ] Test with HTTPS client connection
- [ ] Verify X-Forwarded-Proto is "https"
- [ ] Test mixed proxy + container endpoints

---

## Design Rationale

### Why Embed ProxyConfig?

**Alternative 1**: Duplicate fields in ContainerConfig
- ❌ Code duplication
- ❌ Maintenance burden
- ❌ Feature parity issues

**Alternative 2**: Use interface for proxy/container
- ❌ Go doesn't support inheritance
- ❌ Complex abstraction for simple problem

**Chosen Approach**: Embed ProxyConfig
- ✅ Containers **are** proxies semantically
- ✅ Zero code duplication
- ✅ Natural composition

### Why Shared ProxyHandler?

**Alternative**: Duplicate header logic in ContainerHandler
- ❌ Copy-paste error risk
- ❌ Behavior divergence over time

**Chosen Approach**: Single shared handler
- ✅ **DRY** (Don't Repeat Yourself)
- ✅ Guaranteed consistency
- ✅ Easier testing/debugging

### Why Default Headers?

**Alternative**: Require users to configure manually
- ❌ Poor UX (confusing errors)
- ❌ Every container needs same boilerplate

**Chosen Approach**: Sensible defaults, user can override
- ✅ **Convention over configuration**
- ✅ Works out-of-the-box
- ✅ Power users can customize

---

## Future Enhancements

### Potential Improvements

1. **UI for Proxy Config on Containers**
   - Add "Proxy" tab in container settings panel
   - Allow editing InboundHeaders/OutboundHeaders
   - Show defaults with ability to reset

2. **Health Check Metrics**
   - Track health check history
   - Alert on consecutive failures
   - Auto-restart unhealthy containers

3. **Advanced Header Expressions**
   - Support for request body introspection
   - Regex replacement in header values
   - Conditional header rules

4. **Performance Optimizations**
   - Cache compiled JS expressions
   - Reuse Goja VM instances (pool)
   - Lazy header evaluation

5. **Logging & Observability**
   - Log header transformations in request log
   - Show before/after headers in UI
   - Debug mode for header manipulation

---

## Summary

**Mental Model**: Containers are proxies with dynamic backends.

**Key Components**:
- `ProxyConfig` embedded in `ContainerConfig`
- Shared `ProxyHandler` for all proxying
- `applyHeaderManipulationWithContext()` for custom contexts
- `DefaultContainerInboundHeaders()` for sensible defaults

**Critical Insight**: By treating containers as proxy endpoints, we eliminated code duplication, ensured behavioral consistency, and fixed the redirect issue elegantly.

**Architecture Win**: The refactor required **zero changes** to proxy logic, only additions to support custom context. This proves the abstraction was correct.

---

**Last Updated**: 2025-12-13
**Related Documents**: `CLAUDE.md`, `OPENAPI_IMPORT.md`
