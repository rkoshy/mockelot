# Mockelot - Development Notes

## Project Overview

Mockelot is a Wails v2 application that provides an HTTP mock server for testing and development. It features a Go backend with a Vue 3 + TypeScript frontend.

**Rating: 8/10** - Solid, production-ready tool with excellent architecture and some minor performance optimizations needed.

## Code Review Summary (2025-12-11)

### Architecture & Design Assessment

**Strengths:**
- Clean separation of concerns across packages (models, server, openapi, config, export)
- Elegant three-mode response system (Static, Template, Script)
- Proper request matching hierarchy (exact, wildcard, path params, regex)
- Thread-safe concurrent access patterns
- Graceful shutdown handling

**Package Structure:**
```
models/     - Data structures and business logic (131 lines)
server/     - HTTP handling, matching, validation, templating (973 lines)
  ├── context.go     - Request context building
  ├── handlers.go    - Main HTTP request handler
  ├── matcher.go     - Path pattern matching
  ├── script.go      - JavaScript execution via Goja
  ├── server.go      - HTTP server lifecycle
  ├── template.go    - Go template processing
  └── validation.go  - Request validation
openapi/    - OpenAPI import functionality (1,345 lines)
  ├── converter.go   - Convert operations to ResponseItems
  ├── faker.go       - Embedded Faker utilities
  ├── importer.go    - Import entry point
  ├── parser.go      - OpenAPI spec parsing
  ├── schema_generator.go - Mock data generation
  └── types.go       - OpenAPI-specific types
app.go      - Wails application backend (521 lines)
```

### Performance Issues Identified

#### CRITICAL: Regex Recompilation on Every Request
**Location:** `server/matcher.go:94`, `server/validation.go:77`

Every request matching a regex pattern recompiles it. For high-volume testing (1000s of requests), this is wasteful.

```go
// Current implementation (slow)
re, err := regexp.Compile(pattern)  // Recompiled every request!
```

**Fix Required:** Add regex cache to `ResponseHandler`:
```go
type ResponseHandler struct {
    config        *models.AppConfig
    configMutex   sync.RWMutex
    requestLogger RequestLogger
    regexCache    map[string]*regexp.Regexp  // Add this
    cacheMutex    sync.RWMutex                // And this
}
```

#### CRITICAL: Template Re-parsing on Every Request
**Location:** `server/template.go:70`

Templates are re-parsed on every request:
```go
tmpl, err := template.New("response").Funcs(templateFuncs).Parse(templateBody)
```

**Fix Required:** Cache parsed templates keyed by content hash.

#### HIGH: Unbounded Request Log Growth
**Location:** `app.go:516`

```go
a.requestLogs = append(a.requestLogs, log)  // No size limit!
```

For long-running sessions, this consumes unbounded memory.

**Fix Required:** Implement log rotation:
- Configurable max entries (default 10,000)
- Ring buffer or FIFO eviction
- Optional TTL for old logs

### Security Considerations

**Good Security Practices:**
- ✅ Script execution timeout (5 seconds) prevents infinite loops
- ✅ Context isolation - each script gets fresh VM
- ✅ Read-only request context in scripts
- ✅ Graceful error handling without stack trace leakage

**Security Concerns:**

1. **JavaScript sandbox resource limits** (`server/script.go:39-40`)
   - No memory limits per VM
   - Scripts could DoS by consuming memory
   - Consider adding resource limits to Goja VM

2. **Request logging stores sensitive data** (`models/models.go:129`)
   - Full request bodies logged, including potential passwords/tokens
   - No sensitive field redaction
   - No body size limits

3. **No rate limiting**
   - Mock server can be overwhelmed by clients
   - Consider adding configurable rate limiting

4. **File path validation** (`openapi/importer.go:10`)
   - No validation that file is actually an OpenAPI spec before parsing
   - Malicious YAML could exploit parser

### REST/HTTP Best Practices Assessment

**Endpoint Types:**
- ✅ Mock — static, template, and script-generated responses
- ✅ Proxy — reverse proxy with header manipulation and body transformation
- ✅ Container — Docker/Podman ephemeral container management
- ✅ File Server — serve a local directory with optional SSI and header manipulation
- ✅ SOCKS5 — domain-level interception (system endpoint, not user-created)

**Excellent REST Support:**
- ✅ Proper HTTP status code handling
- ✅ Header management throughout
- ✅ Content-Type handling in script generation
- ✅ Method-based routing with wildcard support
- ✅ Query parameter and path parameter extraction
- ✅ Request/response body handling

**Missing HTTP Features:**
- ❌ HTTP/2 support
- ❌ Multipart/form-data validation

### Code Quality Assessment

**Excellent Practices:**
- Consistent naming conventions throughout
- Clear comments on complex logic
- Proper use of constants for mode/validation types
- Correct pointer semantics (using `*bool` for tri-state)
- Descriptive error messages
- Proper mutex usage (RWMutex for config access)
- Context propagation in Wails lifecycle
- Graceful HTTP server shutdown with timeout

**Minor Issues:**

1. **Magic numbers** - Should be constants:
   - `server/script.go:42` - 5 seconds timeout
   - `server/validation.go:124` - 5 seconds timeout
   - `openapi/schema_generator.go:20` - Max depth 3

2. **Missing input validation:**
   - `app.go:73` - Port not validated (should be 1-65535)
   - No request body size limits

3. **Silent error handling:**
   - `server/template.go:92-94` - Template header errors ignored
   - `server/script.go:101-109` - console.log does nothing (consider collecting for debugging)

4. **No unit tests:**
   - No `*_test.go` files found
   - Critical logic (matcher, validation, converter) should have tests

## Prioritized Recommendations

### High Priority (Performance & Stability)

1. **Implement Regex Caching**
   - Cache compiled regexes in `ResponseHandler` and path matcher
   - Invalidate cache on config update
   - Expected improvement: 50-90% reduction in request latency for regex patterns

2. **Implement Template Caching**
   - Parse templates once, cache by content hash
   - Use `sync.Map` for thread-safe access
   - Expected improvement: 30-70% reduction in template response latency

3. **Add Request Log Rotation**
   - Configurable max entries (default 10,000)
   - Ring buffer implementation
   - Prevent memory exhaustion in long-running sessions

4. **Add CORS Support**
   - Simple enable/disable config
   - Configurable origins, methods, headers
   - Auto-respond to OPTIONS requests

### Medium Priority (Security & Robustness)

5. **Add Resource Limits for Scripts**
   - Memory limit per Goja VM
   - CPU time limits (already have timeout)
   - Prevent DoS via resource exhaustion

6. **Input Validation**
   - Port range validation (1-65535)
   - File path/type validation before parsing
   - Request body size limits (prevent memory attacks)

7. **Improve Script Debugging**
   - Collect console.log output from scripts
   - Expose in UI for debugging failed responses
   - Add script execution metrics

8. **Sensitive Data Handling**
   - Configurable field redaction in logs
   - Body size limits for logging
   - Optional log encryption

### Low Priority (Nice to Have)

9. **Add Comprehensive Tests**
   - Unit tests for matcher.go path matching logic
   - Unit tests for validation.go validation modes
   - Unit tests for openapi/converter.go conversion logic
   - Integration tests for server lifecycle

10. **HTTP/2 Support**
    - Not critical but nice for modern APIs

11. **Multipart Form-Data**
    - Validation and parsing support
    - Useful for file upload mocking

12. **Metrics & Analytics**
    - Request count per endpoint
    - Average response time
    - Script execution statistics
    - Export metrics for analysis

## Recent Work: OpenAPI/Swagger Import Feature

### Feature Summary

Implemented comprehensive OpenAPI 3.x/Swagger specification import functionality that automatically generates mock HTTP endpoints with:
- Realistic mock data generation using embedded Faker utilities
- Request validation (query params and body)
- Security responses (401/403 with authentication validation)
- Response grouping by path
- Dark-mode compatible UI

### Implementation Phases

#### Phase 1: Core Parsing Infrastructure
- Created `openapi/parser.go` - Parses OpenAPI specs using `kin-openapi` library
- Created `openapi/extractor.go` - Extracts operations and parameters
- Created `openapi/path_utils.go` - Converts OpenAPI paths to Mockelot format
- Created `openapi/group_naming.go` - Generates friendly group names

#### Phase 2: Conversion Logic
- Created `openapi/converter.go` - Converts operations to ResponseItems
- Implemented path-based grouping (all HTTP methods for same path grouped together)
- Added status code handling (separate response per status code)
- Implemented header extraction

#### Phase 3: Schema Generation
- Created `openapi/schema_generator.go` - Generates JavaScript mock data from schemas
- Created `openapi/faker.go` - Embedded Faker.js-like utilities for realistic data
- Implemented support for all OpenAPI types (object, array, string, number, boolean)
- Added format handling (date-time, email, uuid, etc.)

#### Phase 4: Advanced Features
- Implemented composition handling (allOf, oneOf, anyOf)
- Added query parameter validation with type checking and enum support
- Implemented circular reference protection (max depth 3)

#### Phase 5: Request Validation & Security
- Implemented request body validation (required fields, types, enums)
- Generated security responses (401/403) for secured endpoints
- Created authentication validation scripts (Bearer, API Key, Basic auth)
- Combined validation scripts for query params and body

#### Phase 6: UI Integration
- Added `ImportOpenAPISpecWithDialog()` backend method
- Created custom dark-mode dialog component (`ConfirmDialog.vue`)
- Added "Import OpenAPI" button to HeaderBar
- Implemented three-button dialog (Append, Replace, Cancel)

#### Phase 7: Testing
- Created `test-api.yaml` - Comprehensive test OpenAPI specification
- Created `test_import.go` - Test program to verify import
- Successfully imported and validated all features

### OpenAPI Implementation Assessment

**Excellent OpenAPI Support:**

1. **Comprehensive parsing** - Uses `kin-openapi` (industry standard)
2. **Smart schema generation** - Proper priority: Example → Enum → Composition → Type-based
3. **Composition handling** - Correctly handles allOf, oneOf, anyOf
4. **Circular reference protection** - Max depth prevents infinite recursion
5. **Path conversion** - OpenAPI `{id}` → Mockelot `:id` format maintained
6. **Response grouping** - Groups all methods for same path, one response per status code
7. **Smart defaults** - Enables 2xx responses, disables errors (good UX)

**OpenAPI Spec Compliance:**

Supported:
- ✅ Objects with properties
- ✅ Arrays with items
- ✅ Primitives (string, number, integer, boolean)
- ✅ Enums
- ✅ Composition (allOf, oneOf, anyOf)
- ✅ Required fields
- ✅ Format constraints (date-time, email, uuid, etc.)
- ✅ Query parameters (required, type, enum)
- ✅ Request body validation
- ✅ Path parameters
- ✅ Bearer/JWT, API keys, Basic auth
- ✅ Static responses (examples)
- ✅ Script-based generation (schemas)
- ✅ Multiple status codes per operation
- ✅ Response headers

Not Yet Supported:
- ❌ OAuth2/OpenID Connect flows (basic validation only)
- ❌ Webhook definitions
- ❌ Server variables substitution
- ❌ Link objects
- ❌ Callback definitions
- ❌ Discriminator handling for polymorphism

### Files Created

**Backend:**
- `openapi/parser.go` - OpenAPI spec parsing (169 lines)
- `openapi/extractor.go` - Operation extraction
- `openapi/converter.go` - Conversion to ResponseItems (608 lines)
- `openapi/schema_generator.go` - Mock data generation (353 lines)
- `openapi/faker.go` - Embedded Faker utilities (141 lines)
- `openapi/importer.go` - Import entry point (24 lines)
- `openapi/path_utils.go` - Path pattern conversion
- `openapi/group_naming.go` - Group name generation
- `openapi/types.go` - OpenAPI-specific types (50 lines)
- `test-api.yaml` - Test OpenAPI specification
- `test_import.go` - Import test program

**Frontend:**
- `frontend/src/components/dialogs/ConfirmDialog.vue` - Custom dark-mode dialog

**Documentation:**
- `docs/OPENAPI_IMPORT.md` - Feature documentation
- `CLAUDE.md` - This file

### Files Modified

**Backend:**
- `app.go` - Added `ImportOpenAPISpecWithDialog()` method

**Frontend:**
- `frontend/src/components/layout/HeaderBar.vue` - Added Import OpenAPI button and dialog
- `frontend/src/stores/server.ts` - Added `importOpenAPISpec()` method

### Technical Details

#### Response Generation Priority
1. **Example**: Use directly if provided in spec
2. **Schema with example**: Use schema.example
3. **Schema**: Generate mock data script with Faker
4. **No schema**: Return empty response

#### Mock Data Script Structure
```javascript
// Faker utilities (embedded)

// Generated mock data based on OpenAPI schema
(function() {
    const generateData = () => {
        return { /* schema-based generation */ };
    };

    response.headers['Content-Type'] = 'application/json';
    response.body = JSON.stringify(generateData(), null, 2);
})();
```

#### Validation Script Structure
```javascript
(function() {
  // Validation checks
  if (!condition) {
    return {valid: false, error: 'Error message'};
  }
  return {valid: true};
})()
```

#### Response Grouping
- All HTTP methods for the same path are grouped into a single `ResponseGroup`
- Each status code becomes a separate `MethodResponse` within the group
- Success responses (2xx) are enabled by default
- Error and security responses are disabled by default

### Build Issues Encountered

1. **SchemaContext initialization error** - Fixed by creating inline instead of calling NewSchemaContext
2. **MinItems type mismatch** - Fixed by removing pointer dereference (uint64 not *uint64)
3. **Nil pointer in Security check** - Fixed by adding `op.Operation.Security != nil` check
4. **White dialog in dark mode** - Fixed by creating custom Vue dialog component

### Usage

#### From UI
1. Click "Import OpenAPI" button in header
2. Choose action in dialog:
   - **Append** - Add to existing endpoints
   - **Replace** - Replace all endpoints
   - **Cancel** - Close without importing
3. Select OpenAPI file (.yaml, .yml, or .json)
4. Endpoints appear in Responses panel

#### From Test Program
```bash
go run test_import.go
```

### Dependencies

- `github.com/getkin/kin-openapi/openapi3` - OpenAPI parsing
- `github.com/google/uuid` - ID generation
- `github.com/dop251/goja` - JavaScript runtime for Go
- Wails v2 - Application framework
- Vue 3 + TypeScript - Frontend

### Future Enhancements

Potential improvements:
1. Support for response examples from spec
2. More sophisticated regex pattern matching
3. OAuth2 flow simulation
4. Import from URLs (not just local files)
5. Batch import of multiple specs
6. Export config to OpenAPI spec

## Recent Work: RTT Pointer Conversion (2025-12-15)

### Feature Summary

Converted all timing metrics (RTT, Delay) from `int64` to `*int64` pointer types to properly distinguish between:
- **nil** - Value not measured yet (pending requests)
- **0** - Actually measured as 0 milliseconds (very fast response)

This is especially important for container endpoints where RTT can be 0-2ms, and users need to distinguish between "pending" vs "extremely fast."

### Motivation

The original implementation used `0` for both "not measured" and "measured as 0ms". This caused confusion in the UI:
- Pending requests showed "0ms" instead of "-"
- Very fast container responses (0-2ms) were indistinguishable from unmeasured requests
- Chrome DevTools-style logging pattern (pending → complete) was unclear

With pointer types:
- Pending requests: `nil` → displays "-"
- Fast responses: `&0` → displays "0ms"
- Normal responses: `&15` → displays "15ms"

### Implementation Details

#### Backend Changes

**models/models.go** - Updated data structures:
```go
// RequestLogSummary (lines 455-456)
ClientRTT      *int64 `json:"client_rtt,omitempty"`  // nil if not measured
BackendRTT     *int64 `json:"backend_rtt,omitempty"` // nil if no backend

// RequestLog.ClientResponse (lines 488-489)
DelayMs    *int64 `json:"delay_ms,omitempty"`       // nil if not measured
RTTMs      *int64 `json:"rtt_ms,omitempty"`         // nil if not measured

// RequestLog.BackendResponse (lines 508-509)
DelayMs    *int64 `json:"delay_ms,omitempty"`    // nil if not measured
RTTMs      *int64 `json:"rtt_ms,omitempty"`      // nil if not measured
```

**server/handlers.go** - Mock endpoint logging:
- Updated `HandleRequest` (lines 462-468) to use `&delayMs` and `&rttMs`
- Updated `handleMockRequest` (lines 696-701) to use pointer values

**server/proxy.go** - Proxy endpoint logging:
- Updated `logProxyRequest` to use `&clientDelayMs`, `&clientRTTMs`, `&backendDelayMs`, `&backendRTTMs`
- Updated `logPendingRequest` to use `nil` for unmeasured values (lines 762-763)

**server/container.go** - Container endpoint logging:
- Updated `logRequest` to use pointer values (lines 756-757, 782-790)
- Updated `logPendingRequest` to use `nil` for pending state (lines 892-893)
- Updated `logErrorRequest` to use `&zero` for immediate errors (lines 857-859)

#### Frontend Changes

**frontend/wailsjs/go/models.ts** - Auto-generated TypeScript bindings:
```typescript
// Lines 705-706
client_rtt?: number;     // Optional property
backend_rtt?: number;    // Optional property
```

**frontend/src/components/traffic/TrafficLogPanel.vue** - UI display:
```typescript
// Lines 63-66
function formatRTT(rtt: number | undefined): string {
  if (rtt === undefined || rtt === null) return '-'
  return `${rtt}ms`
}

// Line 171 - Removed || 0 fallback
{{ formatRTT(log.client_rtt) }}  // Now handles undefined correctly
```

### Files Modified

1. `models/models.go` - Core data structures
2. `server/handlers.go` - Mock request handling
3. `server/proxy.go` - Proxy request handling
4. `server/container.go` - Container request handling
5. `frontend/src/components/traffic/TrafficLogPanel.vue` - UI display
6. `frontend/wailsjs/go/models.ts` - Auto-regenerated TypeScript bindings
7. `docs/SETUP.md` - Created comprehensive setup guide

### JSON Serialization Behavior

With `omitempty` tag and pointer types:
- `nil` pointer → field omitted from JSON entirely
- `&0` → `"client_rtt": 0` in JSON
- `&15` → `"client_rtt": 15` in JSON

TypeScript handles this as optional properties (`?:`), displaying undefined as intended.

### Build Verification

Application builds successfully with all changes:
```bash
~/go/bin/wails build -platform linux/amd64
# Build completed in 8.1s
```

### Technical Insights

1. **Pointer Semantics** - Using `*int64` instead of `int64` for nullable timing values
2. **JSON omitempty** - Nil pointers are omitted from JSON, not serialized as `null`
3. **TypeScript Optional Properties** - `?:` syntax correctly represents nullable fields
4. **Three-State Values** - Similar pattern to `*bool` for tri-state flags (nil/true/false)
5. **Two-Phase Logging** - Log immediately with nil values, update with measured values

### Testing

- ✅ Application compiles successfully
- ✅ TypeScript bindings regenerated correctly
- ✅ Frontend handles undefined values properly
- ✅ Displays "-" for pending, "Xms" for measured values

### Documentation Created

- `docs/SETUP.md` - Comprehensive setup guide covering:
  - Building and starting the application
  - Enabling HTTPS with auto-generated certificates
  - Installing CA certificates on all platforms (Linux, macOS, Windows, Firefox)
  - UI-focused workflow (Export CA Certificate button, Settings panel)
  - Verification and troubleshooting steps
  - Security best practices

- `docs/MOCK-GUIDE.md` - Complete guide for mock endpoints:
  - Three response modes (static, template, script)
  - Path patterns (exact, wildcard, path parameters, regex)
  - Request validation modes
  - Response configuration (status codes, headers, delays)
  - Groups for organization
  - Common use cases and best practices

- `docs/PROXY-GUIDE.md` - Complete guide for proxy endpoints:
  - Path translation modes (none, strip, regex)
  - Header manipulation (inbound/outbound)
  - Status code translation
  - Body transformation with JavaScript
  - Health checks and WebSocket support
  - Common use cases and best practices

- `docs/CONTAINER-GUIDE.md` - Complete guide for container endpoints:
  - Docker and Podman runtime support
  - Image configuration and lifecycle management
  - Environment variables (static and JavaScript expressions)
  - Volume mappings with WSL path translation
  - Resource monitoring and health checks
  - Container logs and troubleshooting
  - Common use cases (PostgreSQL, Redis, Nginx, etc.)

## Development Guidelines

### Code Style
- Use clear, descriptive variable names
- Add comments for complex logic
- Follow Go conventions (gofmt)
- Use TypeScript types consistently
- Extract magic numbers to constants
- Add unit tests for new features

### Testing Strategy
- Test with real-world OpenAPI specs
- Verify all response modes (static, template, script)
- Check validation scripts work correctly
- Test both append and replace modes
- Add unit tests for critical paths:
  - Path matching logic (`server/matcher.go`)
  - Validation logic (`server/validation.go`)
  - OpenAPI conversion (`openapi/converter.go`)
  - Schema generation (`openapi/schema_generator.go`)

### Building

**CRITICAL: Always use the scripts in `scripts/`. Never run builds manually or bypass the script system.**

**For Development (local builds only):**
```bash
# Development mode with hot reload
~/go/bin/wails dev

# Backend only (for quick Go compile check)
go build ./...

# Full local build (for quick testing before committing)
~/go/bin/wails build
```

**For Production Builds — ALWAYS use the scripts:**

All build logic lives in `scripts/`. Laminar CI jobs are thin wrappers that `exec` these scripts. **Never bypass the scripts.**

```bash
# ✅ CORRECT: Release via release script (does everything)
./scripts/release.sh v0.4.1

# ✅ CORRECT: Build all platforms (e.g. after a commit)
./scripts/build-all.sh --laminar

# ✅ CORRECT: Individual platform (debugging a platform-specific issue)
./scripts/build-linux.sh
CI_KEYCHAIN_PASSWORD=... ./scripts/build-macos.sh
./scripts/build-appimage.sh debian12
```

**Releasing:**
```bash
./scripts/release.sh v0.4.1
```

**How `release.sh` works (in order):**
1. Validates version format (`vX.Y.Z`)
2. Checks for uncommitted changes
3. Creates a **local git tag** with the release version (so `git describe` returns the right version inside Laminar CI jobs)
4. Runs `build-all.sh --laminar` — all 5 platform builds in parallel via Laminar CI
5. If any build fails → **deletes the local tag** and exits
6. Builds `.deb` packages from the AppImages
7. Generates `checksums.txt`
8. Shows release notes (auto-generated from commits since last tag)
9. Prompts for confirmation
10. Pushes the tag to origin
11. Creates the GitHub release with all artifacts via `gh release create`

**⚠ Tag ordering matters:** The release script tags locally BEFORE building (step 3) so Laminar CI jobs see the correct version via `git describe`. The tag is only pushed after successful build + user confirmation.

**⚠ Old artifacts in dist/:** Clean up old versioned artifacts before releasing to prevent stale files being included in the release. Run `./scripts/clean.sh` if needed.

**CI Build Infrastructure:**

| Script | Laminar Job | Platform | Output |
|--------|------------|----------|--------|
| `scripts/build-linux.sh` | `mockelot-linux-build` | Linux amd64 | `dist/linux/mockelot-linux-amd64.tar.gz` |
| `scripts/build-windows.sh` | `mockelot-windows-build` | Windows amd64 | `dist/windows/mockelot-windows-amd64.zip` |
| `scripts/build-macos.sh` | `mockelot-macos-build` | macOS Universal | `dist/macos/mockelot-darwin-universal.zip` |
| `scripts/build-appimage.sh` | `mockelot-appimage-d12/d13` | Linux AppImage | `dist/linux/mockelot-{version}-{distro}-x86_64.AppImage` |
| `scripts/build-all.sh` | `mockelot-build-all` | All platforms | All above |

**macOS build notes:**
- Requires SSH access to `10.100.102.102`
- macOS node_modules are cleared before build to pick up new npm dependencies
- Requires `CI_KEYCHAIN_PASSWORD` env var (injected by Laminar)
- The binary is code-signed with `Developer ID Application: Renny Koshy (YET2EWBPK9)`

**Prerequisites:**
- Laminar CI running at `http://localhost:9000`
- macOS builder at `10.100.102.102` accessible via SSH
- `gh` CLI authenticated as `rkoshy` (use `gh auth switch --user rkoshy`)
- Git remote `origin` configured pointing to `github.com/rkoshy/mockelot`

### Git Workflow
- Main branch: `main` (this project uses `main`, not `itd`/`prd`)
- Feature branches: `feature/<short-name>-<ticketid>`
- Always compile (`go build ./...`) before committing
- Always run `~/go/bin/wails build` to verify frontend compiles before committing
- No Claude/AI attributions in commit messages
- The post-commit hook triggers `mockelot-build-all` automatically on `main` — this is expected

## Project Structure

```
mockelot/
├── app.go                          # Main application backend (521 lines)
├── main.go                         # Entry point
├── models/                         # Data models
│   └── models.go                   # Core data structures (131 lines)
├── openapi/                        # OpenAPI import feature (1,345 lines)
│   ├── converter.go                # Convert operations to ResponseItems (608 lines)
│   ├── extractor.go                # Extract operations from spec
│   ├── faker.go                    # Embedded Faker utilities (141 lines)
│   ├── group_naming.go             # Generate group names
│   ├── importer.go                 # Main import entry point (24 lines)
│   ├── parser.go                   # Parse OpenAPI specs (169 lines)
│   ├── path_utils.go               # Path pattern conversion
│   ├── schema_generator.go         # Generate mock data scripts (353 lines)
│   └── types.go                    # OpenAPI-specific types (50 lines)
├── server/                         # HTTP mock server (973 lines)
│   ├── context.go                  # Request context building (109 lines)
│   ├── handlers.go                 # Main HTTP request handler (220 lines)
│   ├── matcher.go                  # Path pattern matching (121 lines)
│   ├── script.go                   # JavaScript execution (198 lines)
│   ├── server.go                   # HTTP server lifecycle (79 lines)
│   ├── template.go                 # Go template processing (104 lines)
│   └── validation.go               # Request validation (251 lines)
├── config/                         # Configuration handling
│   └── config.go
├── export/                         # Export functionality
│   └── export.go
├── frontend/                       # Vue 3 frontend
│   └── src/
│       ├── components/
│       │   ├── dialogs/
│       │   │   └── ConfirmDialog.vue    # Custom dialog
│       │   └── layout/
│       │       └── HeaderBar.vue        # Header with import button
│       └── stores/
│           └── server.ts                # Pinia store
├── docs/                           # Documentation
│   ├── SETUP.md                    # Setup and HTTPS guide
│   ├── MOCK-GUIDE.md              # Mock endpoint guide
│   ├── PROXY-GUIDE.md             # Proxy endpoint guide
│   ├── CONTAINER-GUIDE.md         # Container endpoint guide
│   └── OPENAPI_IMPORT.md          # OpenAPI import guide
├── scripts/                        # Build system (single source of truth)
│   ├── _common.sh                  # Shared library (version, logging, paths)
│   ├── build-linux.sh              # Linux amd64 build
│   ├── build-windows.sh            # Windows amd64 cross-compile
│   ├── build-macos.sh              # macOS universal via SSH
│   ├── build-appimage.sh           # AppImage (parameterized: debian12/debian13)
│   ├── build-appimage-container.sh # Runs inside Docker for AppImage
│   ├── build-deb.sh                # Convert AppImage to .deb
│   ├── build-all.sh                # Orchestrator (direct or --laminar)
│   ├── release.sh                  # Full release workflow
│   ├── clean.sh                    # Remove build artifacts
│   └── README.md                   # Build system documentation
├── test-api.yaml                   # Test OpenAPI spec
├── test_import.go                  # Import test program
└── README.md                       # User-facing documentation
```

## Key Learnings & Best Practices

### Architecture Patterns
1. **Clean separation of concerns** - Each package has single responsibility
2. **Thread-safe config updates** - RWMutex allows concurrent reads, exclusive writes
3. **Proper resource lifecycle** - Server shutdown, context cancellation, cleanup
4. **Three-mode progression** - Static → Template → Script provides natural complexity curve

### Technical Insights
1. **Wails MessageDialog Limitation** - Native dialogs don't respect app theme → use custom Vue components
2. **OpenAPI Schema Complexity** - Handling all composition types requires careful merging logic
3. **Validation Script Combination** - Merge validation scripts for query params and body carefully
4. **Security Response Strategy** - Generate but disable by default to avoid interfering with normal testing
5. **Response Grouping** - Grouping by path provides better organization than flat list
6. **Goja Timeout Pattern** - Use buffered channels + context timeout to prevent goroutine leaks
7. **Regex/Template Caching** - Critical for performance at scale

### Performance Considerations
- **Cache compiled regexes** - Don't recompile on every request
- **Cache parsed templates** - Parse once, reuse many times
- **Limit log storage** - Implement rotation to prevent unbounded growth
- **Use RWMutex** - Allow concurrent reads where possible
- **Deep copy carefully** - Only when necessary (e.g., headers to prevent races)

### Security Practices
- **Timeout all user scripts** - 5 second limit prevents infinite loops
- **Isolate script contexts** - Fresh VM per execution
- **Graceful error handling** - Don't leak stack traces
- **Consider resource limits** - Memory, CPU for script execution
- **Redact sensitive data** - Especially in logs

## Known Limitations & Issues

### Known Issues from OpenAPI Import
- Circular references limited to 3 levels depth
- Very large schemas may generate verbose JavaScript
- Pattern/regex validation is simplified

### Performance Limitations
- ❌ Regex recompilation on every request (CRITICAL - needs fix)
- ❌ Template re-parsing on every request (CRITICAL - needs fix)
- ❌ Unbounded request log growth (HIGH - needs fix)

### Feature Limitations
- No CORS support (common requirement for frontend development)
- No OPTIONS handling (needed for CORS preflight)
- No HTTP/2 support
- No multipart/form-data validation
- No rate limiting
- No metrics/analytics

### Security Limitations
- No memory limits on script execution
- No sensitive field redaction in logs
- No request body size limits
- Minimal file validation before parsing

## Testing Checklist

When implementing new features or fixes:

- [ ] Unit tests for core logic
- [ ] Integration tests for server behavior
- [ ] Test with real-world OpenAPI specs
- [ ] Verify all response modes work
- [ ] Test validation scripts
- [ ] Check memory usage under load
- [ ] Verify thread safety
- [ ] Test error handling paths
- [ ] Validate input sanitization
- [ ] Check performance impact

## Resources

- [Wails Documentation](https://wails.io/)
- [OpenAPI 3.0 Specification](https://swagger.io/specification/)
- [kin-openapi Library](https://github.com/getkin/kin-openapi)
- [Faker.js Documentation](https://fakerjs.dev/)
- [Goja (JavaScript Runtime)](https://github.com/dop251/goja)
- [Go text/template](https://pkg.go.dev/text/template)

## Next Steps for Development

### Immediate (Critical Path)
1. Implement regex caching in `server/matcher.go` and `server/validation.go`
2. Implement template caching in `server/template.go`
3. Add request log rotation with configurable size limit
4. Add basic CORS support

### Short Term
5. Add unit tests for matcher, validator, converter
6. Add resource limits to Goja VM
7. Implement input validation (port range, file types)
8. Add script debugging output collection

### Long Term
9. Add metrics and analytics
10. Implement sensitive data redaction
11. Add HTTP/2 support
12. Implement multipart/form-data support

## Recent Work: SOCKS5 Proxy Feature (2025-12-19)

### Feature Summary

Implemented comprehensive SOCKS5 proxy server functionality enabling browser-based multi-domain testing without DNS modification. This allows developers to route traffic through Mockelot and selectively mock endpoints across multiple domains while allowing unmocked requests to pass through to real servers.

### Motivation

Frontend developers often need to test against multiple backend domains simultaneously (e.g., `auth.example.com`, `api.example.com`, `cdn.example.com`). Previous solutions required:
- Modifying `/etc/hosts` for each domain
- Browser redirects
- DNS server setup

The SOCKS5 proxy solution provides:
- **One-time browser configuration** - Set proxy to `localhost:1080`
- **Selective domain interception** - Configure which domains to intercept
- **Overlay mode** - Mix mocked endpoints with real backend calls
- **No DNS changes** - SOCKS5 handles routing transparently

### Implementation Phases

#### Phase 1: Backend Models and SOCKS5 Server
- Created `models/models.go` extensions for SOCKS5 configuration
- Created `server/socks5.go` - Full SOCKS5 protocol implementation (~600 lines)
- Implemented SOCKS5 handshake with version negotiation
- Implemented authentication methods (no-auth and username/password)
- Implemented CONNECT command for TCP tunneling
- Created HTTP tunnel reading and response capture

#### Phase 2: Domain Matching
- Enhanced `server/handlers.go` with domain extraction and matching
- Implemented three domain filter modes:
  - `any` - Match all domains
  - `all` - Match all SOCKS5 intercepted domains
  - `specific` - Match selected domain patterns
- Added domain filter check before path matching (both must match)
- Integrated with existing endpoint matching hierarchy

#### Phase 3: Overlay Mode
- Created `server/overlay.go` - Overlay mode passthrough handler (~250 lines)
- Implemented DNS resolution with 5-minute caching
- Created fallback chain: domain match → path match → overlay → 404
- Preserved original Host header for virtual hosting support
- Integrated with existing ProxyHandler patterns

#### Phase 4: Frontend UI
- Created `frontend/src/components/dialogs/SOCKS5Tab.vue` - Complete SOCKS5 configuration UI (~450 lines)
- Added SOCKS5 tab to `ServerConfigDialog.vue`
- Added domain filter UI to `EndpointSettingsDialog.vue`
- Implemented domain takeover table with add/delete/enable/disable
- Added hosts file helper with copy-to-clipboard
- Added browser setup instructions (Firefox, Chrome, cURL)

#### Phase 5: Testing & Documentation
- Created `test-socks5-config.json` - Comprehensive test configuration
- Created `test-socks5.sh` - Automated test suite for SOCKS5 functionality
- Created `docs/SOCKS5-GUIDE.md` - Complete user guide (~600 lines)
- Updated `README.md` with SOCKS5 feature overview
- Successfully built full application with all features integrated

### Architecture Design

**Component 1: SOCKS5 Server (`server/socks5.go`)**
- Accepts SOCKS5 connections on configurable port (default 1080)
- Handles SOCKS5 handshake and authentication
- Reads CONNECT command to get target domain and port
- Establishes HTTP tunnel
- Passes requests to existing `ResponseHandler.HandleRequest()`
- Uses custom `responseRecorder` to capture responses for tunnel writing

**Component 2: Domain Configuration (`models/models.go`)**
```go
type SOCKS5Config struct {
    Enabled        bool   `json:"enabled"`
    Port           int    `json:"port"`
    Authentication bool   `json:"authentication"`
    Username       string `json:"username,omitempty"`
    Password       string `json:"password,omitempty"`
}

type DomainTakeoverConfig struct {
    Domains []DomainConfig `json:"domains"`
}

type DomainConfig struct {
    ID          string `json:"id"`
    Pattern     string `json:"pattern"`      // Regex pattern
    OverlayMode bool   `json:"overlay_mode"` // Passthrough if no match
    Enabled     bool   `json:"enabled"`
}

type DomainFilter struct {
    Mode     string   `json:"mode"`     // "any", "all", "specific"
    Patterns []string `json:"patterns"` // For "specific" mode
}
```

**Component 3: Domain-Aware Routing (`server/handlers.go`)**
- Extracts domain from Host header
- Checks domain filter before path matching
- Routes to overlay mode if domain has overlay enabled and no endpoint matches
- Falls back to 404 if no match and no overlay

**Component 4: Overlay Handler (`server/overlay.go`)**
- Resolves real IP addresses for domains (with caching)
- Creates synthetic proxy endpoint with real backend URL
- Preserves original Host header
- Executes proxy request using existing ProxyHandler

### Request Flow Examples

**Example 1: Mocked Endpoint**
```
Browser → SOCKS5 (localhost:1080) → Mockelot
Domain: api.example.com, Path: /api/users
↓
1. Extract domain: api.example.com
2. Check domain filter: ✅ matches endpoint
3. Check path: ✅ matches /api/users
4. Return mock response
```

**Example 2: Overlay Mode Passthrough**
```
Browser → SOCKS5 (localhost:1080) → Mockelot
Domain: api.example.com, Path: /unknown
↓
1. Extract domain: api.example.com
2. No endpoint matches /unknown
3. Check overlay mode: ✅ enabled
4. Resolve real IP: 93.184.216.34
5. Proxy to https://93.184.216.34/unknown (Host: api.example.com)
6. Return real server response
```

**Example 3: Non-Intercepted Domain**
```
Browser → SOCKS5 (localhost:1080) → Mockelot
Domain: google.com
↓
1. Domain not in takeover list
2. Pass through transparently
3. Act as dumb SOCKS5 proxy
```

### Files Created

**Backend:**
- `server/socks5.go` - SOCKS5 protocol implementation (~600 lines)
- `server/overlay.go` - Overlay mode handler (~250 lines)

**Frontend:**
- `frontend/src/components/server/tabs/ServerTab.vue` - Server settings including SOCKS5 configuration

**Testing:**
- `test-socks5-config.json` - Test configuration
- `test-socks5.sh` - Automated test script

**Documentation:**
- `docs/SOCKS5-GUIDE.md` - Complete SOCKS5 guide (~600 lines)

### Files Modified

**Backend:**
- `models/models.go` - Added SOCKS5Config, DomainTakeoverConfig, DomainConfig, DomainFilter
- `server/server.go` - Added SOCKS5 server startup in Start() method
- `server/handlers.go` - Added domain extraction, domain matching, overlay mode integration
- `app.go` - Added GetSOCKS5Config() and SetSOCKS5Config() methods

**Frontend:**
- `frontend/src/components/server/tabs/ServerTab.vue` - Contains all server settings including SOCKS5 in collapsible sections
- `frontend/src/components/dialogs/EndpointSettingsDialog.vue` - Added domain filter UI
- `frontend/wailsjs/go/models.ts` - Auto-regenerated TypeScript bindings

**Documentation:**
- `README.md` - Added SOCKS5 feature section
- `CLAUDE.md` - This section

### Technical Insights

1. **SOCKS5 Protocol Implementation** - Used low-level `net.Conn` for TCP connection handling
2. **HTTP Tunneling** - Read HTTP requests from SOCKS5 tunnel using `bufio.Reader`
3. **Response Capture** - Created custom `responseRecorder` implementing `http.ResponseWriter` to capture responses for tunnel
4. **Domain + Path Matching** - Both must match for endpoint to handle request
5. **DNS Caching** - 5-minute TTL reduces overlay mode latency
6. **Thread Safety** - Used `sync.RWMutex` for DNS cache access
7. **First Match Wins** - Existing endpoint matching strategy applies to domain+path combinations

### Build Verification

All phases completed successfully:
```bash
~/go/bin/wails build -platform linux/amd64
# Build completed in 9.88s
# Binary: 22MB at build/bin/mockelot
```

Application compiles without errors and all TypeScript bindings regenerate correctly.

### Testing Approach

**Automated Tests (test-socks5.sh):**
1. Basic SOCKS5 connectivity test
2. Domain-specific endpoint matching
3. All intercepted domains matching
4. Overlay mode passthrough
5. HTTPS through SOCKS5
6. Non-intercepted domain passthrough

**Manual Testing:**
- Configure browser to use SOCKS5 proxy
- Add test domains to intercepted list
- Create endpoints with domain filters
- Verify selective mocking with overlay mode
- Test authentication if enabled

### Known Limitations

- SOCKS5 CONNECT command only (no BIND or UDP ASSOCIATE)
- IPv4 only (IPv6 not implemented)
- Authentication credentials stored in plain text in config
- No PAC file support for automatic proxy configuration
- DNS caching fixed at 5 minutes (not configurable)

### Performance Considerations

- **DNS Caching** - Reduces overlay mode latency on repeated requests
- **Connection Pooling** - Could add connection pooling for overlay mode
- **Regex Caching** - Reuses existing regex compilation cache for domain patterns

### Use Cases

1. **Frontend Development** - Mock auth service, proxy to real API
2. **Microservices Testing** - Mock external dependencies
3. **Partial Mocking** - Mock broken endpoints, pass through working ones
4. **Multi-Tenant Testing** - Different mocks per subdomain

## Recent Work: Server Tab Architecture (2026-02-08)

### Architecture Update

The application has moved from a Settings dialog architecture to a unified Server Tab architecture:

1. **Old Architecture (REMOVED)**:
   - ServerConfigDialog.vue with separate tab components (HTTPTab, HTTPSTab, CORSTab, SOCKS5Tab)
   - Gear icon in header to open Settings dialog
   - Multiple dialog components for configuration

2. **Current Architecture**:
   - Server tab in main panel (`ServerConfigPanel.vue`)
   - All settings in collapsible sections within `ServerTab.vue`
   - Direct editing in main interface, no modal dialogs
   - Settings sections: HTTP, HTTPS, CORS, SOCKS5 Proxy

### Files Removed
- `frontend/src/components/dialogs/ServerConfigDialog.vue`
- `frontend/src/components/dialogs/HTTPTab.vue`
- `frontend/src/components/dialogs/HTTPSTab.vue`
- `frontend/src/components/dialogs/CORSTab.vue`
- `frontend/src/components/dialogs/SOCKS5Tab.vue`
- `frontend/src/components/dialogs/CORSDialog.vue`

### Current UI Navigation
1. Click "Server" tab in the left sidebar to access all server settings
2. Each section (HTTP, HTTPS, CORS, SOCKS5) is in a collapsible panel
3. Changes are applied immediately without needing a separate "Apply" action
4. SOCKS5 configuration is found under the "SOCKS5 Proxy" collapsible section

## Recent Work: SOCKS5 Endpoint Tab Enhancement (2026-02-08)

### Feature Summary

Enhanced the SOCKS5 Proxy endpoint tab to show a list of domains that have passed through the proxy with quick-add functionality.

### Implementation Details

1. **Backend Changes**:
   - Fixed `RequestLogSummary` to include `TargetHost` and `TargetPort` from `SOCKS5Info`
   - Added `SOCKS5DomainInfo` model for domain aggregation
   - Implemented `GetSOCKS5Domains()` method to aggregate unique domains
   - Implemented `AddDomainToSOCKS5Takeover()` for quick domain addition
   - Added `UpdateDomainTakeover()` to HTTPServer and SOCKS5Server

2. **Frontend Changes**:
   - Created `SOCKS5DomainsPanel.vue` component
   - Integrated into `ServerConfigPanel.vue` for system-socks5-proxy endpoint
   - Shows domain list with request counts and timestamps
   - ADD button for non-configured domains
   - Auto-refresh every 5 seconds
   - Uses `FrontendLog` for proper backend logging

### Files Created
- `frontend/src/components/socks5/SOCKS5DomainsPanel.vue`

### Files Modified
- `app.go` - Added domain aggregation methods
- `models/models.go` - Added SOCKS5DomainInfo struct
- `server/server.go` - Added UpdateDomainTakeover method
- `server/socks5.go` - Added UpdateDomainTakeover method
- `frontend/src/components/server/ServerConfigPanel.vue` - Integrated domains panel

### Testing the Feature
1. Run the application
2. Click "Server" tab → expand "SOCKS5 Proxy" section → enable SOCKS5
3. Configure browser to use `socks5h://localhost:1080`
4. Browse websites
5. Click "SOCKS5 Proxy" endpoint tab (not Server tab)
6. See domains list with ADD buttons for quick configuration

## Releasing to GitHub

### Overview

Releases are versioned `vX.Y.Z` and published to https://github.com/rkoshy/mockelot/releases. Each release attaches five artifacts:

| Artifact | Source path |
|----------|-------------|
| `mockelot-linux-amd64.tar.gz` | `dist/linux/` |
| `mockelot-windows-amd64.zip` | `dist/windows/` |
| `mockelot-darwin-universal.zip` | `dist/macos/` |
| `mockelot_X.Y.Z-native-debian13_amd64.deb` | `dist/linux/` |
| `checksums.txt` | `dist/linux/` |

### Path A — Fully Automated (normal flow)

```bash
./scripts/release.sh v0.5.1
```

`release.sh` does in order:
1. Validates version format and clean working tree
2. Creates a **local** annotated git tag (so `git describe` returns the right version inside Laminar CI)
3. Runs `build-all.sh --laminar` — all platform builds in parallel via Laminar CI
4. If any build fails → deletes local tag and exits
5. Builds native Debian 13 `.deb` locally via `build-deb-native.sh`
6. Moves `.deb` to `dist/linux/` and generates `dist/linux/checksums.txt`
7. Prints release notes (commits since last tag) and asks for confirmation
8. Pushes `main` + tag to `origin`
9. Runs `gh release create` with all five artifacts

**Prerequisites:**
- Laminar CI running at `http://localhost:9000`
- macOS builder at `10.100.102.102` accessible via SSH
- `CI_KEYCHAIN_PASSWORD` env var set (for macOS code signing)
- `gh` CLI authenticated as `rkoshy` (`gh auth switch --user rkoshy`)

### Path B — Manual (when artifacts are pre-built)

Use this when `release.sh` was partially run (tag exists locally, builds already completed) or when publishing artifacts that are already in `dist/`.

**Pre-flight checks:**
```bash
# Verify tag exists locally and NOT yet on GitHub
git describe --tags                            # should print vX.Y.Z
git ls-remote --tags origin | grep vX.Y.Z     # should print nothing

# Verify all five artifacts exist
ls dist/linux/mockelot-linux-amd64.tar.gz
ls dist/linux/mockelot_X.Y.Z-native-debian13_amd64.deb
ls dist/linux/checksums.txt
ls dist/windows/mockelot-windows-amd64.zip
ls dist/macos/mockelot-darwin-universal.zip

# Check existing GitHub releases
gh release list --repo rkoshy/mockelot | head -5
```

**Publish:**
```bash
git push origin main
git push origin vX.Y.Z

# Build release notes (commits since previous tag)
git log --pretty=format:"- %s" "$(git describe --tags --abbrev=0 HEAD^)..vX.Y.Z" | grep -v "^- Merge"

gh release create vX.Y.Z \
    "dist/linux/mockelot-linux-amd64.tar.gz" \
    "dist/windows/mockelot-windows-amd64.zip" \
    "dist/macos/mockelot-darwin-universal.zip" \
    "dist/linux/mockelot_X.Y.Z-native-debian13_amd64.deb" \
    "dist/linux/checksums.txt" \
    --title "vX.Y.Z" \
    --notes "## Changes

- feat: ..."
```

### Post-release cleanup

Stale `.deb` files accumulate in the project root as build intermediates — delete them after each release:
```bash
rm -f mockelot_*.deb
```

Run `./scripts/clean.sh` before the next release to prevent old `dist/` artifacts from being included.

### Troubleshooting

**"Tag already exists" error in `release.sh`:** A prior partial run created the tag. Check `dist/` to see if artifacts are complete; if so, use Path B.

**macOS build fails:** SSH to `10.100.102.102` and check Laminar logs. macOS node_modules are cleared before each build to pick up new npm dependencies.

**`gh release create` auth error:** Run `gh auth switch --user rkoshy` and retry.

**Wrong version embedded in binary:** `git describe` drives the version string. The tag must exist locally *before* building. `release.sh` handles this automatically; for manual builds, tag first.

## Contact & Support

For issues or questions:
- `README.md` for user-facing documentation
- `docs/SETUP.md` for setup and HTTPS configuration
- `docs/MOCK-GUIDE.md` for mock endpoint configuration
- `docs/PROXY-GUIDE.md` for proxy endpoint configuration
- `docs/CONTAINER-GUIDE.md` for container endpoint configuration
- `docs/SOCKS5-GUIDE.md` for SOCKS5 proxy configuration
- `docs/OPENAPI_IMPORT.md` for OpenAPI import feature details
- Test files (`test-api.yaml`, `test_import.go`, `test-socks5-config.json`, `test-socks5.sh`) for examples
- Code comments in packages for implementation details

---

## File Server Endpoint

Added in v0.5.0. Defined in `server/fileserver.go`.

**Config struct** (`models/models.go`):
```go
type FileServerConfig struct {
    BasePath    string       `json:"base_path"`                // Local directory to serve from
    EnableSSI   bool         `json:"enable_ssi"`               // Process <!--#include virtual="..."> in .shtml
    ProxyConfig *ProxyConfig `json:"proxy_config,omitempty"`   // Header/status manipulation (optional)
}
```

**Request flow:**
1. Translate path using the endpoint's `TranslationMode`/`TranslatePattern`/`TranslationRules` (shared with proxy endpoints)
2. Join translated path onto `BasePath`; security-check that result stays inside `BasePath`
3. `os.ReadFile` the file — if missing, fall through to overlay or return 404
4. If `EnableSSI` and extension is `.shtml`, replace `<!--#include virtual="...">` by re-dispatching each virtual path as an internal sub-request (max depth 10)
5. Detect Content-Type by extension (`mime.TypeByExtension`); `.shtml` → `text/html; charset=utf-8`
6. If `ProxyConfig` set, apply outbound header manipulation + status translation
7. Write response

**UI visual treatment for disabled endpoints/rules:**
- Endpoint in sidebar: name gets `line-through opacity-50`, type badge gets `opacity-50` (`EndpointNavigator.vue`)
- Disabled response rule card: `opacity-60` + darker bg/border (`ResponseRuleCard.vue`)
- Disabled response group card: `opacity-60` + darker bg/border (`ResponseGroupCard.vue`)

**Last Updated:** 2026-08-23
**Review Rating:** 8/10 - Production-ready with performance optimizations needed
**Total Lines of Code:** ~4,270 lines (Go backend only - includes SOCKS5 ~850 lines)
