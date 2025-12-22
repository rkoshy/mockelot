# Mockelot

**A powerful, developer-friendly HTTP mock server with a beautiful desktop UI.**

Stop wrestling with clunky mock servers. Mockelot gives you instant, configurable HTTP responses with zero friction - complete with regex matching, Go templates, JavaScript scripting, and request validation that actually works.

![Mockelot Screenshot](docs/screenshot.png)

## Why Mockelot?

Ever needed to:
- Test how your app handles a 500 error from an API?
- Simulate slow responses to check your timeout handling?
- Mock complex API responses without spinning up a whole backend?
- Validate that incoming webhooks have the right payload?

**Mockelot does all of this with a clean UI and powerful features you'll actually use.**

## Features

### Flexible Request Matching

Match requests your way:

```
/api/users              # Exact match
/api/*                  # Wildcard
^/api/v[0-9]+/users$    # Regex
/users/{id}             # Path parameters
/users/:id/posts/:postId # Multiple parameters
```

### Three Response Modes

**Static** - Simple, predictable responses
```json
{"status": "ok", "message": "Hello, World!"}
```

**Template** - Dynamic responses using Go templates
```json
{
  "userId": "{{.PathParams.id}}",
  "query": "{{.GetQueryParam \"search\"}}",
  "timestamp": "{{now}}"
}
```

**Script** - Full JavaScript for complex logic
```javascript
const userId = request.pathParams.id;
const items = [
  { id: 1, name: "Item 1" },
  { id: 2, name: "Item 2" }
];

response.status = 200;
response.headers["X-Custom-Header"] = "dynamic-value";
response.body = JSON.stringify({
  userId,
  items: items.filter(i => i.id <= parseInt(userId))
});
```

### Request Validation with Variable Extraction

Don't just mock - validate incoming requests and extract data for your responses:

**Static Matching**
```
# Check if body contains specific text
Contains: "action": "subscribe"
```

**Regex with Named Groups**
```regex
"userId":\s*"(?P<userId>\d+)".*"action":\s*"(?P<action>\w+)"
```
Variables `userId` and `action` are now available in your response templates!

**Script Validation**
```javascript
const json = JSON.parse(body);
result.valid = json.userId !== undefined && json.action === "subscribe";
result.vars.userId = json.userId;
result.vars.plan = json.plan || "free";
```

### Organize with Groups

Group related responses together. Enable/disable entire groups with one click. Perfect for:
- Different API versions
- Feature flags
- Test scenarios
- Environment-specific mocks

### Response Delay Simulation

Test your timeout handling and loading states by adding configurable delays (in milliseconds) to any response.

### Real-time Request Logging

See every request that hits your server:
- Method, path, and status code
- Headers and query parameters
- Request body
- Timestamp and source IP

Export logs as JSON or CSV for analysis.

### SOCKS5 Proxy for Multi-Domain Testing

Route browser traffic through Mockelot without modifying DNS settings:

```
Browser → SOCKS5 Proxy (localhost:1080) → Mockelot → Smart Routing
```

**Key Features:**
- **Domain-Based Routing** - Intercept specific domains (e.g., `api.example.com`)
- **Selective Mocking** - Mock some endpoints, pass others through to real server
- **Overlay Mode** - Automatic passthrough when no endpoint matches
- **No DNS Changes** - Configure browser proxy instead of `/etc/hosts`
- **Multi-Domain Support** - Test multiple domains simultaneously

**Perfect for:**
- Frontend development against multiple APIs
- Testing microservices with mocked dependencies
- Partial mocking (some endpoints mocked, others real)
- Multi-tenant application testing

See the **[SOCKS5 Guide](docs/SOCKS5-GUIDE.md)** for complete setup and usage.

## Installation

### Download Binary

Download the latest release for your platform from the [Releases](https://github.com/rkoshy/mockelot/releases) page.

### Build from Source

Requirements:
- Go 1.21+
- Node.js 18+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

```bash
# Clone the repo
git clone https://github.com/rkoshy/mockelot.git
cd mockelot

# Build
wails build

# Or run in development mode
wails dev
```

## Quick Start

1. **Launch Mockelot** and set your server port (default: 8080)

2. **Add a response rule:**
   - Click "Add Response"
   - Set path pattern: `/api/hello`
   - Select methods: `GET`
   - Set status: `200 OK`
   - Add body: `{"message": "Hello!"}`

3. **Start the server** and test it:
   ```bash
   curl http://localhost:8080/api/hello
   ```

4. **Get creative** - try templates, scripts, and validation!

## Template Reference

Available in Template mode:

| Variable | Description |
|----------|-------------|
| `{{.Method}}` | HTTP method (GET, POST, etc.) |
| `{{.Path}}` | Request path |
| `{{.PathParams.name}}` | Path parameter value |
| `{{.GetQueryParam "key"}}` | Query parameter value |
| `{{.GetHeader "X-Header"}}` | Request header value |
| `{{.Body.Raw}}` | Raw request body |
| `{{.Body.JSON.field}}` | Parsed JSON field |
| `{{.Vars.name}}` | Extracted validation variable |

**Template Functions:**
- `{{now}}` - Current timestamp
- `{{timestamp}}` - Unix timestamp
- `{{json .Body.JSON}}` - JSON encode
- `{{jsonPretty .Body.JSON}}` - Pretty JSON
- `{{upper "text"}}` - Uppercase
- `{{lower "TEXT"}}` - Lowercase
- `{{uuid}}` - Generate UUID

## Script Reference

Available in Script mode:

```javascript
// Request context (read-only)
request.method          // "POST"
request.path            // "/api/users/123"
request.pathParams.id   // "123"
request.queryParams.q   // ["search term"]
request.headers["Content-Type"]  // ["application/json"]
request.body.raw        // Raw body string
request.body.json       // Parsed JSON object
request.vars.userId     // Extracted from validation

// Response (modify these)
response.status = 201;
response.headers["X-Custom"] = "value";
response.body = JSON.stringify({...});
response.delay = 1000;  // Add 1 second delay

// Utilities
console.log("Debug message");
JSON.parse(str);
JSON.stringify(obj);
```

## Documentation

Comprehensive guides for all features:

- **[Setup Guide](docs/SETUP.md)** - Complete setup instructions including HTTPS configuration and certificate installation
- **[Mock Endpoint Guide](docs/MOCK-GUIDE.md)** - Deep dive into mock endpoints, response modes, and validation
- **[Proxy Endpoint Guide](docs/PROXY-GUIDE.md)** - Reverse proxy configuration, header manipulation, and body transformation
- **[Container Endpoint Guide](docs/CONTAINER-GUIDE.md)** - Docker/Podman container management and configuration
- **[SOCKS5 Proxy Guide](docs/SOCKS5-GUIDE.md)** - SOCKS5 proxy setup, domain-based routing, and overlay mode
- **[OpenAPI Import Guide](docs/OPENAPI_IMPORT.md)** - Import OpenAPI/Swagger specifications to generate mock endpoints

## Configuration

Mockelot saves configurations as YAML files. You can:
- **Save** your current configuration for later
- **Load** saved configurations
- **Share** configuration files with your team

Example configuration:
```yaml
port: 8080
items:
  - type: response
    response:
      id: "abc123"
      enabled: true
      path_pattern: "/api/users/{id}"
      methods: ["GET"]
      status_code: 200
      headers:
        Content-Type: "application/json"
      response_mode: "template"
      body: '{"id": "{{.PathParams.id}}", "name": "Test User"}'
  - type: group
    group:
      name: "Error Responses"
      enabled: true
      responses:
        - path_pattern: "/api/error"
          methods: ["GET"]
          status_code: 500
          body: '{"error": "Internal Server Error"}'
```

## Use Cases

### API Development
Mock your backend while building the frontend. No more waiting for backend teams.

### Integration Testing
Create predictable responses for your test suite. Test error handling, edge cases, and timeouts.

### Webhook Development
Test your webhook handlers by simulating incoming requests with specific payloads.

### Demo & Presentations
Create a fake API for demos without exposing real data or services.

### Learning & Experimentation
Understand how APIs work by creating your own mock endpoints.

## Development

### Live Development

Run in live development mode with hot reload:

```bash
wails dev
```

This starts a Vite development server for fast frontend changes. Access the dev server at http://localhost:34115 to call Go methods from browser devtools.

### Building

Build a redistributable package:

```bash
wails build
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Credits

Built with:
- [Wails](https://wails.io/) - Go + Web Technologies
- [Vue 3](https://vuejs.org/) - Frontend framework
- [Tailwind CSS](https://tailwindcss.com/) - Styling
- [Goja](https://github.com/dop251/goja) - JavaScript runtime for Go

---

**Made with care for developers who are tired of bad tooling.**

*By [Renny Koshy](https://github.com/rkoshy)*
