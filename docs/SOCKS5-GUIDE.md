# SOCKS5 Proxy Guide for Mockelot

## Table of Contents
1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Configuration](#configuration)
4. [Domain Takeover](#domain-takeover)
5. [Domain Filtering](#domain-filtering)
6. [Overlay Mode](#overlay-mode)
7. [Browser Configuration](#browser-configuration)
8. [Testing](#testing)
9. [Common Use Cases](#common-use-cases)
10. [Troubleshooting](#troubleshooting)

## Overview

The SOCKS5 proxy feature allows you to route browser traffic through Mockelot without modifying DNS settings or using browser redirects. This makes it easy to test multiple domains and intercept specific requests while allowing others to pass through to real servers.

### How It Works

1. **Configure SOCKS5** - Enable SOCKS5 proxy in Mockelot settings (default port 1080)
2. **Add Domains** - Specify which domains to intercept (e.g., `api.example.com`)
3. **Configure Browser** - Set browser to use SOCKS5 proxy `localhost:1080`
4. **Route Traffic** - Browser requests go through Mockelot
5. **Smart Routing** - Mockelot checks if domain is intercepted
   - If intercepted: Match against configured endpoints (by domain + path)
   - If endpoint matches: Return mock/proxy/container response
   - If no endpoint matches: Use overlay mode (optional passthrough to real server)
   - If not intercepted: Pass through directly to real server

### Key Benefits

- **No DNS Changes** - No need to modify `/etc/hosts` or DNS settings
- **Domain Isolation** - Only intercept specific domains you're testing
- **Selective Mocking** - Mix mocked endpoints with real backend calls
- **Multi-Domain Testing** - Test multiple domains simultaneously
- **Easy Switching** - Toggle proxy on/off in browser without config changes

## Quick Start

### 1. Enable SOCKS5

1. Open Mockelot
2. Click **Settings** (gear icon)
3. Navigate to **SOCKS5** tab
4. Check **Enable SOCKS5 Proxy**
5. Leave port as **1080** (default)
6. Leave **Require Authentication** unchecked for testing
7. Click **Apply**

### 2. Add a Domain to Intercept

1. In the **Intercepted Domains** section, click **Add Domain**
2. Enter domain pattern (regex): `api\.test\.local`
3. Check **Overlay Mode** (recommended for starting)
4. Check **Enabled**
5. Click **Apply**

### 3. Configure Your Browser

**Firefox:**
1. Settings → General → Network Settings
2. Click **Settings** button
3. Select **Manual proxy configuration**
4. SOCKS Host: `localhost`, Port: `1080`
5. Select **SOCKS v5**
6. Check **Proxy DNS when using SOCKS v5**
7. Click **OK**

**Chrome/Edge:**
Use FoxyProxy or system proxy settings

### 4. Add Hosts Entry

Add to `/etc/hosts` (Linux/Mac) or `C:\Windows\System32\drivers\etc\hosts` (Windows):
```
127.0.0.1 api.test.local
```

### 5. Test

Create an endpoint in Mockelot:
- **Path:** `/api/users`
- **Domain Filter:** Specific → Select `api.test.local`
- **Response:** Static JSON with user data

Navigate to `http://api.test.local:8080/api/users` in your browser.

## Configuration

### SOCKS5 Settings

**Enable SOCKS5 Proxy**
- Toggle to enable/disable the SOCKS5 server
- Requires server restart when changed

**Port**
- Default: `1080` (standard SOCKS5 port)
- Range: 1-65535
- Click **Reset to Default** to restore port 1080
- Avoid ports already in use

**Require Authentication**
- When enabled, clients must provide username/password
- Useful for restricting access in shared environments
- Leave disabled for local development

**Username / Password**
- Only shown when authentication is enabled
- Credentials are stored in plain text in config
- Use strong passwords if exposing to network

### Domain Takeover Configuration

**Intercepted Domains Table**

| Field | Description |
|-------|-------------|
| **Domain Pattern** | Regex pattern matching domain (e.g., `api\.example\.com`) |
| **Overlay Mode** | When checked, requests that don't match any endpoint are proxied to the real server |
| **Enabled** | When checked, domain is actively intercepted |
| **Actions** | Delete button to remove domain |

**Add Domain Button**
- Creates new domain entry with defaults:
  - Pattern: empty (you must fill in)
  - Overlay Mode: ON (recommended)
  - Enabled: ON

**Domain Pattern Syntax**

Patterns are **regular expressions**:
- `api\.example\.com` - Match exact domain (escape dots)
- `.*\.example\.com` - Match all subdomains of example.com
- `(api|www)\.example\.com` - Match api.example.com or www.example.com

**Overlay Mode**

When enabled for a domain:
- If request matches an endpoint → Use endpoint response
- If request doesn't match any endpoint → Proxy to real server

When disabled:
- If request matches an endpoint → Use endpoint response
- If request doesn't match any endpoint → Return 404

Use overlay mode when you want to mock specific endpoints while allowing other requests to the same domain to reach the real server.

## Domain Filtering

Endpoints can filter which domains they respond to using the **Domain Filter** setting in the endpoint configuration.

### Domain Filter Modes

**Any Domain (`any`)**
- Endpoint responds to requests from **all domains**
- Use for endpoints that should work regardless of domain
- Example: Health check endpoint `/health`

**All Intercepted Domains (`all`)**
- Endpoint responds to **all domains in the SOCKS5 takeover list**
- Use for endpoints that apply to all your test domains
- Example: Common `/api/status` endpoint

**Specific Domains (`specific`)**
- Endpoint responds **only to selected domains**
- Select from list of enabled intercepted domains
- Use for domain-specific endpoints
- Example: `/api/users` only for `api.example.com`

### Configuring Domain Filters

1. Open endpoint settings (click endpoint in list)
2. Scroll to **Domain Filter (SOCKS5 Proxy)** section
3. Select filter mode from dropdown
4. If **Specific Domains**, check domains to match
5. Click **Save**

### Domain + Path Matching

Both domain AND path must match for endpoint to handle request:

```
Request: GET http://api.test.local:8080/api/users

Endpoint Check:
  1. Domain Filter: Does api.test.local match? → YES
  2. Path Pattern: Does /api/users match ^/api/users? → YES
  3. Method: Does GET match GET? → YES
  → Endpoint handles request
```

If domain doesn't match, endpoint is skipped regardless of path.

## Overlay Mode

Overlay mode provides **selective passthrough** - mock some endpoints while allowing others to reach the real server.

### How Overlay Mode Works

```
Browser Request → SOCKS5 Proxy → Mockelot

1. Is domain in takeover list?
   NO  → Pass through to real server (transparent proxy)
   YES → Continue to step 2

2. Does any endpoint match domain + path?
   YES → Use endpoint response (mock/proxy/container)
   NO  → Continue to step 3

3. Is overlay mode enabled for this domain?
   YES → Proxy to real server with DNS resolution
   NO  → Return 404 Not Found
```

### Use Cases for Overlay Mode

**Testing New API Integration**
- Mock authentication endpoints (`/auth/login`)
- Let other endpoints pass through to staging server
- Gradually add more mocked endpoints as needed

**Frontend Development**
- Mock slow or flaky endpoints
- Let working endpoints pass through
- Avoid maintaining full mock for entire API

**Debugging Specific Flows**
- Intercept problematic endpoint to test fixes
- Allow rest of application to work normally
- Add logging or modify responses for that endpoint only

### DNS Resolution and Caching

When overlay mode proxies to a real server:
1. **Resolve Domain** - Look up real IP address via DNS
2. **Cache Result** - Store IP for 5 minutes
3. **Build Request** - Create proxy request with real IP
4. **Preserve Host** - Keep original `Host` header for virtual hosting
5. **Execute Request** - Forward to real server
6. **Return Response** - Send real server response to browser

DNS caching reduces latency on subsequent requests to the same domain.

## Browser Configuration

### Firefox (Recommended)

Firefox has native SOCKS5 support with DNS proxying:

1. Open **Settings**
2. Scroll to **Network Settings**
3. Click **Settings** button
4. Select **Manual proxy configuration**
5. In **SOCKS Host**, enter: `localhost`
6. In **Port**, enter: `1080`
7. Select **SOCKS v5** radio button
8. **Important:** Check **Proxy DNS when using SOCKS v5**
9. Click **OK**

**Testing:** Navigate to `http://any.domain.com/health` (if you have an endpoint configured)

**Disable Proxy:** Open Network Settings, select **No proxy**

### Chrome / Edge

Chrome and Edge don't have built-in SOCKS5 configuration. Use one of these methods:

**Option 1: FoxyProxy Extension (Recommended)**
1. Install FoxyProxy extension
2. Add new proxy:
   - Type: SOCKS5
   - Hostname: localhost
   - Port: 1080
3. Enable proxy

**Option 2: Command Line**
```bash
# Linux/Mac
google-chrome --proxy-server="socks5://localhost:1080"

# Windows
chrome.exe --proxy-server="socks5://localhost:1080"
```

**Option 3: System Proxy**
Configure OS-level SOCKS proxy (affects all applications)

### cURL Command Line

Test with cURL:

```bash
# Basic SOCKS5 request
curl --socks5 localhost:1080 http://api.test.local:8080/api/users

# With authentication
curl --socks5 localhost:1080 \
  --proxy-user username:password \
  http://api.test.local:8080/api/users

# HTTPS through SOCKS5
curl --socks5 localhost:1080 \
  -k \
  https://api.test.local:8443/api/users

# Verbose output for debugging
curl --socks5 localhost:1080 -v http://api.test.local:8080/health
```

## Testing

### Automated Test Script

Mockelot includes a comprehensive test script: `test-socks5.sh`

```bash
# Make script executable
chmod +x test-socks5.sh

# Run tests
./test-socks5.sh
```

The script tests:
1. Basic SOCKS5 connectivity
2. Domain-specific matching
3. All intercepted domains matching
4. Overlay mode passthrough
5. HTTPS through SOCKS5
6. Non-intercepted domain passthrough

### Test Configuration

Load `test-socks5-config.json` for pre-configured test endpoints:
- Endpoint with `any` domain filter at `/health`
- Endpoint with `specific` domain filter at `/api/users` for `api.test.local`
- Endpoint with `all` domain filter at `/test`
- Three test domains: `api.test.local`, `app.test.local`, `passthrough.test.local`

### Manual Testing Steps

**Test 1: Basic Connectivity**
```bash
curl --socks5 localhost:1080 http://any.domain.com/health
```
Expected: `{"status": "healthy"}` (if endpoint configured)

**Test 2: Domain Matching**
```bash
curl --socks5 localhost:1080 http://api.test.local:8080/api/users
```
Expected: User data from configured endpoint

**Test 3: Overlay Mode**
```bash
curl --socks5 localhost:1080 http://passthrough.test.local:8080/unknown
```
Expected: Proxied response from real server (or DNS error if domain doesn't exist)

**Test 4: Authentication**

Enable authentication in settings, then:
```bash
curl --socks5 localhost:1080 \
  --proxy-user testuser:testpass \
  http://api.test.local:8080/api/users
```

## Common Use Cases

### Use Case 1: Frontend Development Against Multiple Services

**Scenario:** You're developing a frontend that calls multiple backend services (`auth.example.com`, `api.example.com`, `cdn.example.com`). You want to mock auth and API but use real CDN.

**Configuration:**
1. Enable SOCKS5 on port 1080
2. Add intercepted domains:
   - `auth\.example\.com` (overlay mode: OFF)
   - `api\.example\.com` (overlay mode: ON)
   - `cdn\.example\.com` (overlay mode: ON)
3. Create endpoints:
   - `/auth/login` with domain filter: `auth.example.com` → Mock response
   - `/api/users` with domain filter: `api.example.com` → Mock response
4. Configure browser to use SOCKS5 proxy
5. Result:
   - Auth requests → Mocked
   - `/api/users` requests → Mocked
   - Other API requests → Proxied to real server (overlay mode)
   - CDN requests → Proxied to real server (no endpoints configured)

### Use Case 2: Testing Microservices

**Scenario:** Testing a microservice that calls other services. Want to mock external dependencies.

**Configuration:**
1. Enable SOCKS5 on port 1080
2. Add intercepted domains for external services:
   - `payment-service\.internal` (overlay mode: OFF)
   - `notification-service\.internal` (overlay mode: OFF)
3. Create mock endpoints for external API calls
4. Run your microservice with SOCKS5 proxy environment:
   ```bash
   export http_proxy=socks5://localhost:1080
   export https_proxy=socks5://localhost:1080
   ./your-service
   ```

### Use Case 3: API Testing with Partial Mocking

**Scenario:** Testing API client library against staging server, but one endpoint is broken.

**Configuration:**
1. Enable SOCKS5
2. Add domain: `api-staging.example.com` (overlay mode: ON)
3. Create endpoint for broken path: `/api/broken-endpoint` → Mock working response
4. All other requests pass through to real staging server
5. Run tests with proxy configured

### Use Case 4: Simulating Multi-Tenant Environments

**Scenario:** Testing SaaS application with different tenant subdomains.

**Configuration:**
1. Enable SOCKS5
2. Add intercepted domain pattern: `.*\.example\.com` (matches all subdomains)
3. Create endpoints with domain filters:
   - `/api/config` → Domain filter: `tenant1\.example\.com` → Tenant 1 config
   - `/api/config` → Domain filter: `tenant2\.example\.com` → Tenant 2 config
4. Browser accesses different subdomains, gets tenant-specific responses

## Troubleshooting

### SOCKS5 Server Won't Start

**Symptom:** Error starting server, or SOCKS5 proxy doesn't respond

**Causes:**
1. Port 1080 already in use
2. Permission denied (port < 1024 on Linux without sudo)
3. Firewall blocking port

**Solutions:**
```bash
# Check if port is in use
netstat -an | grep 1080
# or
ss -tuln | grep 1080

# Use different port (e.g., 8081)
# Change in SOCKS5 settings, click Apply

# Check firewall (Linux)
sudo ufw status
sudo ufw allow 1080
```

### Browser Can't Connect Through Proxy

**Symptom:** Browser shows connection errors when proxy is enabled

**Checks:**
1. Verify Mockelot is running
2. Verify SOCKS5 is enabled in settings
3. Check browser proxy configuration:
   - Hostname: `localhost` (not 127.0.0.1, some browsers differ)
   - Port: `1080` (match Mockelot settings)
   - Type: SOCKS v5 (not SOCKS v4)
4. Test with cURL:
   ```bash
   curl --socks5 localhost:1080 http://example.com
   ```

### Domain Not Being Intercepted

**Symptom:** Requests go to real server instead of Mockelot endpoints

**Causes:**
1. Domain not in intercepted domains list
2. Domain pattern doesn't match (regex error)
3. Domain is disabled in list
4. Endpoint domain filter doesn't match

**Debug Steps:**
1. Check Mockelot logs for incoming requests
2. Verify domain pattern matches with regex tester
3. Ensure domain is **Enabled** in table
4. Check endpoint **Domain Filter** setting
5. Test with wildcard endpoint (domain filter: any, path: `.*`)

### Authentication Failing

**Symptom:** Browser prompts for credentials repeatedly, or cURL returns authentication error

**Solutions:**
1. Verify username/password in Mockelot settings
2. Check browser proxy username/password configuration
3. For cURL, use `--proxy-user username:password`
4. Try disabling authentication to isolate issue

### Overlay Mode Not Working

**Symptom:** Get 404 instead of proxied response

**Checks:**
1. Verify overlay mode is **checked** for domain
2. Ensure domain resolves:
   ```bash
   nslookup example.com
   ```
3. Check Mockelot logs for DNS resolution errors
4. Verify no firewall blocking outbound connections

### HTTPS/TLS Errors

**Symptom:** Certificate errors when accessing HTTPS through SOCKS5

**Solutions:**
1. Install Mockelot CA certificate in browser/OS
2. See `docs/SETUP.md` for CA installation instructions
3. For cURL, use `-k` to skip verification (testing only):
   ```bash
   curl --socks5 localhost:1080 -k https://api.test.local:8443/api/users
   ```
4. Ensure domain is in **Cert Names** in HTTPS settings

### DNS Resolution Takes Long Time

**Symptom:** First request to domain is slow, subsequent requests are fast

**Explanation:** This is normal - first request performs DNS lookup, result is cached for 5 minutes.

**Optimization:** Pre-warm cache by accessing domain once before testing.

### Browser Using Proxy for All Domains

**Symptom:** All browser traffic goes through Mockelot, even non-intercepted domains

**Explanation:** This is correct behavior for SOCKS5 proxy. Mockelot passes through non-intercepted domains transparently.

**If you want to avoid:** Configure proxy only for specific domains using PAC file or browser extension like FoxyProxy.

## Advanced Configuration

### SOCKS5 with Authentication

For production or shared environments:
1. Enable **Require Authentication**
2. Set strong username and password
3. Note: Credentials stored in plain text in config file
4. Browser will prompt for credentials once per session

### Hosts File Helper

Instead of SOCKS5 (or in addition to), use hosts file entries:

1. In SOCKS5 settings, see **Hosts File Helper** section
2. Copy generated entries
3. Paste into hosts file:
   - **Linux/Mac:** `/etc/hosts` (requires sudo)
   - **Windows:** `C:\Windows\System32\drivers\etc\hosts` (requires admin)
4. Save file
5. Test: `ping api.test.local` should resolve to 127.0.0.1

Hosts file approach works for applications that don't support SOCKS5 proxy.

### Combining SOCKS5 with Endpoint Features

SOCKS5 domain filtering works with **all endpoint types**:

- **Mock Endpoints:** Serve static/template/script responses per domain
- **Proxy Endpoints:** Proxy to different backends based on domain
- **Container Endpoints:** Route to containerized services by domain

Example: Route `api.test.local` to mock, `db.test.local` to PostgreSQL container, `backend.test.local` to real proxy backend.

## Best Practices

1. **Start with Overlay Mode ON** - Prevents breaking other requests while you build endpoints
2. **Use Descriptive Domain Patterns** - Comment your regex patterns
3. **Enable HTTPS** - Install CA certificate for realistic browser testing
4. **Test Incrementally** - Start with one domain, one endpoint
5. **Use Domain Filter: Specific** - Explicitly list domains per endpoint for clarity
6. **Monitor Logs** - Watch Traffic Log panel to verify requests being intercepted
7. **Disable Proxy When Done** - Remember to disable browser proxy after testing
8. **Version Control** - Save configurations for different test scenarios

## See Also

- [Setup Guide](SETUP.md) - Installation and HTTPS setup
- [Mock Endpoint Guide](MOCK-GUIDE.md) - Creating mock responses
- [Proxy Endpoint Guide](PROXY-GUIDE.md) - Configuring proxy endpoints
- [Container Endpoint Guide](CONTAINER-GUIDE.md) - Using container endpoints
