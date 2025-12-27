# HOWTO: Using Mockelot as a SOCKS5 Proxy

This guide shows you how to use Mockelot as a SOCKS5 proxy to selectively intercept and mock specific endpoints while allowing other traffic to pass through to real servers.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Step 1: Enable HTTPS in Mockelot](#step-1-enable-https-in-mockelot)
- [Step 2: Install SSL Certificates](#step-2-install-ssl-certificates)
- [Step 3: Enable SOCKS5 Proxy](#step-3-enable-socks5-proxy)
- [Step 4: Configure Domain Takeover](#step-4-configure-domain-takeover)
- [Step 5: Configure Your Browser](#step-5-configure-your-browser)
- [Example Scenarios](#example-scenarios)
- [Troubleshooting](#troubleshooting)

---

## Overview

**Why use SOCKS5 proxy mode?**

SOCKS5 proxy mode allows you to:
- Test against multiple domains simultaneously (e.g., `api.company.com`, `auth.company.com`, `cdn.company.com`)
- Selectively mock specific endpoints while allowing others to pass through to real servers
- Test frontend applications locally while using production/staging backends
- Override broken or slow endpoints during development
- No need to modify `/etc/hosts` or DNS settings

**How it works:**

1. Configure your browser to use Mockelot as a SOCKS5 proxy (e.g., `localhost:1080`)
2. Configure which domains Mockelot should intercept (e.g., `api.company.com`)
3. Create mock or proxy endpoints for the paths you want to override
4. All other requests pass through transparently to real servers

---

## Prerequisites

- Mockelot installed and running
- A web browser (Firefox, Chrome, Edge, Safari, or Brave)
- Administrator/root access to install SSL certificates system-wide (optional)

---

## Step 1: Enable HTTPS in Mockelot

SOCKS5 proxy mode requires HTTPS to work with modern web applications.

1. **Open Mockelot**
2. **Click the Settings icon** in the header bar
3. **In the HTTPS section:**
   - Check **"Enable HTTPS"**
   - Leave the port as `8443` (or choose your preferred port)
   - Click **"Generate Certificates"** if you don't have certificates yet
4. **Click "Save"**

Mockelot will now accept HTTPS connections on `https://localhost:8443`.

---

## Step 2: Install SSL Certificates

To avoid browser security warnings, you need to install Mockelot's CA certificate.

### Option A: Export and Install Manually

#### Step 2A.1: Export the CA Certificate

1. **In Mockelot Settings**, scroll to the **HTTPS section**
2. **Click "Export CA Certificate"**
3. **Save the file** as `mockelot-ca.crt` to your Downloads folder

#### Step 2A.2: Install in Firefox

1. **Open Firefox** → **Settings** (or Preferences)
2. **Search for "certificates"** in the search bar
3. **Click "View Certificates"**
4. **Go to the "Authorities" tab**
5. **Click "Import..."**
6. **Select** `mockelot-ca.crt` from your Downloads folder
7. **Check "Trust this CA to identify websites"**
8. **Click "OK"**

#### Step 2A.3: Install in Chrome / Edge / Brave (Windows)

1. **Open Chrome/Edge/Brave** → **Settings**
2. **Search for "certificates"** or navigate to **Privacy and Security → Security → Manage Certificates**
3. **Go to the "Trusted Root Certification Authorities" tab**
4. **Click "Import..."**
5. **Follow the wizard:**
   - Click "Next"
   - Browse and select `mockelot-ca.crt`
   - Click "Next"
   - Select "Place all certificates in the following store: **Trusted Root Certification Authorities**"
   - Click "Next" → "Finish"
6. **Click "Yes"** on the security warning
7. **Restart your browser**

#### Step 2A.4: Install in Chrome / Brave (macOS)

1. **Open Keychain Access** (Applications → Utilities → Keychain Access)
2. **Select "System" keychain** in the left sidebar
3. **Drag and drop** `mockelot-ca.crt` into the Keychain Access window
4. **Right-click** the imported certificate (named "Mockelot CA")
5. **Select "Get Info"**
6. **Expand "Trust"**
7. **Set "When using this certificate" to "Always Trust"**
8. **Close the window** and enter your password when prompted
9. **Restart Chrome/Brave**

#### Step 2A.5: Install in Safari (macOS)

Safari uses the macOS system keychain, so follow the Chrome/Brave (macOS) instructions above. The certificate will automatically be trusted by Safari.

#### Step 2A.6: Install in Chrome / Brave (Linux)

Chrome and Brave on Linux use the NSS certificate database.

```bash
# Install certutil if not already installed
sudo apt-get install libnss3-tools  # Debian/Ubuntu
# OR
sudo yum install nss-tools          # RHEL/CentOS

# Import the certificate
certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n "Mockelot CA" -i ~/Downloads/mockelot-ca.crt

# Restart your browser
```

### Option B: Install System-Wide (Recommended)

Installing the certificate system-wide makes it trusted by all applications on your computer.

#### Linux (System-Wide)

```bash
# Copy the certificate to the system trust store
sudo cp ~/Downloads/mockelot-ca.crt /usr/local/share/ca-certificates/mockelot-ca.crt

# Update the CA certificates
sudo update-ca-certificates

# Restart your browser
```

#### macOS (System-Wide)

```bash
# Import into the System keychain
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ~/Downloads/mockelot-ca.crt

# Restart your browser
```

#### Windows (System-Wide)

Use the Chrome/Edge/Brave instructions above (Step 2A.3) - installing to "Trusted Root Certification Authorities" makes it system-wide.

Alternatively, use PowerShell (as Administrator):

```powershell
# Import the certificate
Import-Certificate -FilePath "$env:USERPROFILE\Downloads\mockelot-ca.crt" -CertStoreLocation Cert:\LocalMachine\Root

# Restart your browser
```

---

## Step 3: Enable SOCKS5 Proxy

1. **Open Mockelot Settings** (click the Settings icon in the header)
2. **Go to the "SOCKS5 Proxy" tab**
3. **Enable SOCKS5 Proxy:**
   - Check **"Enable SOCKS5 Proxy"**
   - Leave the port as `1080` (or choose your preferred port)
4. **Authentication (optional):**
   - If you want to require authentication, check **"Require Authentication"**
   - Set a username and password
5. **Click "Save"**

Mockelot is now listening for SOCKS5 connections on `localhost:1080`.

---

## Step 4: Configure Domain Takeover

Domain takeover tells Mockelot which domains to intercept.

1. **In the SOCKS5 Proxy tab**, scroll to **"Domain Takeover Configuration"**
2. **Click "Add Domain"**
3. **Configure the domain:**
   - **Pattern:** Enter the domain pattern (e.g., `api.company.com` or `*.company.com` for wildcards)
   - **Overlay Mode:** Check this to allow unmatched requests to pass through to the real server
   - **Enabled:** Check this to activate the domain interception
4. **Click "Add"**
5. **Repeat** for each domain you want to intercept
6. **Click "Save"**

**Example configurations:**

| Pattern | Overlay Mode | Purpose |
|---------|--------------|---------|
| `api.company.com` | ✅ Enabled | Intercept API requests, pass through unmatched |
| `auth.company.com` | ❌ Disabled | Block all requests to auth server (full mock) |
| `*.staging.company.com` | ✅ Enabled | Intercept all staging subdomains |

---

## Step 5: Configure Your Browser

Configure your browser to use Mockelot as a SOCKS5 proxy.

### Firefox

1. **Open Firefox** → **Settings**
2. **Scroll down to "Network Settings"**
3. **Click "Settings..."**
4. **Select "Manual proxy configuration"**
5. **Configure:**
   - **SOCKS Host:** `localhost`
   - **Port:** `1080`
   - **Select "SOCKS v5"**
   - **Check "Proxy DNS when using SOCKS v5"** (important!)
6. **Click "OK"**

### Chrome / Edge / Brave (Command Line)

These browsers use system proxy settings, but you can launch them with a SOCKS5 proxy via command line:

**Windows:**
```cmd
chrome.exe --proxy-server="socks5://localhost:1080"
```

**macOS:**
```bash
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --proxy-server="socks5://localhost:1080"
```

**Linux:**
```bash
google-chrome --proxy-server="socks5://localhost:1080"
```

### Chrome / Edge / Brave (Extension)

Use a proxy extension like **Proxy SwitchyOmega**:

1. **Install Proxy SwitchyOmega** from the Chrome Web Store
2. **Click the extension icon** → **Options**
3. **Create a new profile** (e.g., "Mockelot")
4. **Select "Proxy Profile"**
5. **Configure:**
   - **Protocol:** `SOCKS5`
   - **Server:** `localhost`
   - **Port:** `1080`
6. **Click "Apply changes"**
7. **Click the extension icon** and select your "Mockelot" profile

### Safari (macOS)

1. **Open System Preferences** → **Network**
2. **Select your active network** (Wi-Fi or Ethernet)
3. **Click "Advanced..."**
4. **Go to the "Proxies" tab**
5. **Check "SOCKS Proxy"**
6. **Configure:**
   - **SOCKS Proxy Server:** `localhost:1080`
7. **Click "OK"** → **Apply**

**Note:** This changes system-wide proxy settings on macOS.

---

## Example Scenarios

### Scenario 1: Override One REST Service, Pass Through Others

**Use Case:** You're developing a new version of the `/users` service locally while using the production API for everything else.

**Setup:**

1. **Configure Domain Takeover:**
   - **Pattern:** `api.company.com`
   - **Overlay Mode:** ✅ **Enabled** (pass through unmatched requests)

2. **Create a Proxy Endpoint** for the local service:
   - **Type:** Proxy
   - **Domain Filter:**
     - **Mode:** Specific
     - **Patterns:** `api.company.com`
   - **Path Pattern:** `/users/*`
   - **Target URL:** `http://localhost:3000/users`
   - **Path Translation:** Strip (`/users/123` → `http://localhost:3000/users/123`)

**Result:**
- `https://api.company.com/users/123` → Your local service at `http://localhost:3000/users/123`
- `https://api.company.com/products/456` → Production server (passes through)
- `https://api.company.com/orders/789` → Production server (passes through)

---

### Scenario 2: Override Multiple Paths to Different Services

**Use Case:** You're testing two microservices locally (`users` and `orders`) while using production for everything else.

**Setup:**

1. **Configure Domain Takeover:**
   - **Pattern:** `api.company.com`
   - **Overlay Mode:** ✅ **Enabled**

2. **Create Proxy Endpoint for Users Service:**
   - **Domain Filter:** Specific → `api.company.com`
   - **Path Pattern:** `/api/v1/users/*`
   - **Target URL:** `http://localhost:3000`
   - **Path Translation:** None

3. **Create Proxy Endpoint for Orders Service:**
   - **Domain Filter:** Specific → `api.company.com`
   - **Path Pattern:** `/api/v1/orders/*`
   - **Target URL:** `http://localhost:4000`
   - **Path Translation:** None

**Result:**
- `https://api.company.com/api/v1/users/123` → `http://localhost:3000/api/v1/users/123`
- `https://api.company.com/api/v1/orders/456` → `http://localhost:4000/api/v1/orders/456`
- `https://api.company.com/api/v1/products/789` → Production (passes through)

---

### Scenario 3: Test Frontend Module Locally, Proxy Static Assets

**Use Case:** You're developing a portal module locally while using the production CDN for other assets.

**Setup:**

1. **Configure Domain Takeover:**
   - **Pattern:** `app.company.com`
   - **Overlay Mode:** ✅ **Enabled**

2. **Create Proxy Endpoint for Local Portal:**
   - **Domain Filter:** Specific → `app.company.com`
   - **Path Pattern:** `/portal/admin/*`
   - **Target URL:** `http://localhost:8080/portal/admin`
   - **Path Translation:** None

3. **Create Proxy Endpoint for Local Assets (optional):**
   - **Path Pattern:** `/assets/admin/*`
   - **Target URL:** `http://localhost:8080/assets/admin`

**Result:**
- `https://app.company.com/portal/admin` → Your local dev server at `http://localhost:8080/portal/admin`
- `https://app.company.com/portal/dashboard` → Production (passes through)
- `https://app.company.com/assets/admin/bundle.js` → Your local dev server
- `https://app.company.com/assets/shared/common.css` → Production CDN (passes through)

---

### Scenario 4: Mock Authentication, Proxy Everything Else

**Use Case:** You want to bypass authentication during testing but use real backend services.

**Setup:**

1. **Configure Domain Takeover:**
   - **Pattern:** `auth.company.com`
   - **Overlay Mode:** ❌ **Disabled** (block all, force mock)

2. **Create Mock Endpoint for Login:**
   - **Type:** Mock
   - **Domain Filter:** Specific → `auth.company.com`
   - **Path Pattern:** `/api/login`
   - **Method:** POST
   - **Response Mode:** Script
   - **Status Code:** 200
   - **Script:**
     ```javascript
     response.headers['Content-Type'] = 'application/json';
     response.body = JSON.stringify({
       token: 'mock-jwt-token-12345',
       user: {
         id: 1,
         username: 'testuser',
         email: 'test@example.com',
         roles: ['admin']
       }
     });
     ```

3. **Configure other domains normally:**
   - **Pattern:** `api.company.com`
   - **Overlay Mode:** ✅ **Enabled** (real API calls)

**Result:**
- `https://auth.company.com/api/login` → Returns mock JWT token
- `https://api.company.com/users` → Real API (uses mock token)

---

### Scenario 5: Add Delay to Test Timeout Handling

**Use Case:** You want to test how your frontend handles slow API responses.

**Setup:**

1. **Configure Domain Takeover:**
   - **Pattern:** `api.company.com`
   - **Overlay Mode:** ✅ **Enabled**

2. **Create Mock Endpoint with Delay:**
   - **Domain Filter:** Specific → `api.company.com`
   - **Path Pattern:** `/api/slow-endpoint`
   - **Response Mode:** Static
   - **Status Code:** 200
   - **Body:** `{"status": "ok"}`
   - **Delay (ms):** `5000` (5 second delay)

**Result:**
- `https://api.company.com/api/slow-endpoint` → Returns after 5 seconds
- All other requests → Normal speed (passes through)

---

### Scenario 6: Override Broken Production Endpoint Temporarily

**Use Case:** A production endpoint is returning errors. You want to mock it temporarily while waiting for the fix.

**Setup:**

1. **Configure Domain Takeover:**
   - **Pattern:** `api.company.com`
   - **Overlay Mode:** ✅ **Enabled**

2. **Create Mock Endpoint for Broken Service:**
   - **Domain Filter:** Specific → `api.company.com`
   - **Path Pattern:** `/api/broken-service/*`
   - **Response Mode:** Static
   - **Status Code:** 200
   - **Headers:** `Content-Type: application/json`
   - **Body:**
     ```json
     {
       "status": "temporary_mock",
       "data": {
         "id": 1,
         "name": "Placeholder Data"
       }
     }
     ```

**Result:**
- `https://api.company.com/api/broken-service/123` → Returns mock data
- Everything else → Production (passes through)

---

### Scenario 7: Test Against Staging API with Mock Auth

**Use Case:** You want to test your frontend against a staging API but mock the authentication service.

**Setup:**

1. **Configure Domain Takeover (Auth):**
   - **Pattern:** `auth.staging.company.com`
   - **Overlay Mode:** ❌ **Disabled**

2. **Configure Domain Takeover (API):**
   - **Pattern:** `api.staging.company.com`
   - **Overlay Mode:** ✅ **Enabled**

3. **Create Mock Auth Endpoint:**
   - **Domain Filter:** Specific → `auth.staging.company.com`
   - **Path Pattern:** `/oauth/token`
   - **Response Mode:** Script
   - **Script:**
     ```javascript
     response.headers['Content-Type'] = 'application/json';
     response.body = JSON.stringify({
       access_token: 'mock-oauth-token',
       token_type: 'Bearer',
       expires_in: 3600
     });
     ```

**Result:**
- `https://auth.staging.company.com/oauth/token` → Returns mock OAuth token
- `https://api.staging.company.com/*` → Real staging API

---

## Troubleshooting

### Browser Shows "Proxy Server Refusing Connections"

**Cause:** Mockelot's SOCKS5 proxy is not running or wrong port configured.

**Solution:**
1. Verify SOCKS5 is enabled in Mockelot Settings → SOCKS5 Proxy tab
2. Check the port matches your browser configuration (default: `1080`)
3. Restart Mockelot

### SSL/TLS Certificate Errors

**Cause:** CA certificate not installed or not trusted.

**Solution:**
1. Re-export the CA certificate from Mockelot
2. Follow the installation instructions for your browser (Step 2)
3. Restart your browser after installing the certificate
4. Verify the certificate is installed:
   - **Firefox:** Settings → Privacy & Security → View Certificates → Authorities → Search for "Mockelot"
   - **Chrome/Edge:** Settings → Security → Manage Certificates → Trusted Root → Look for "Mockelot CA"

### Requests Not Being Intercepted

**Cause:** Domain not configured in Domain Takeover or overlay mode disabled without matching endpoint.

**Solution:**
1. Verify the domain is in the Domain Takeover list (Settings → SOCKS5 Proxy → Domain Takeover)
2. Ensure the domain is **Enabled**
3. Check your endpoint's Domain Filter matches the intercepted domain
4. Enable **Overlay Mode** if you want unmatched requests to pass through

### All Requests Returning 404

**Cause:** Overlay mode is disabled and no matching endpoint exists.

**Solution:**
1. Enable **Overlay Mode** for the domain (Settings → SOCKS5 Proxy → Domain Takeover)
2. Or create endpoints to handle all requests to that domain

### Browser Proxy Settings Don't Persist

**Cause:** Some browsers reset proxy settings on restart.

**Solution:**
- Use a browser extension like **Proxy SwitchyOmega** for persistent settings
- Or use command-line flags to launch the browser with proxy enabled

### DNS Not Resolving Through Proxy

**Cause:** Browser not configured to proxy DNS queries.

**Solution:**
- **Firefox:** Enable "Proxy DNS when using SOCKS v5" in Network Settings
- **Chrome/Others:** Ensure you're using `socks5://` (not `socks4://`) in proxy configuration

### Performance Issues / Slow Responses

**Cause:** Overlay mode performs DNS lookups and creates proxy connections on demand.

**Solution:**
1. DNS results are cached for 5 minutes, so subsequent requests will be faster
2. For frequently accessed domains, consider creating explicit proxy endpoints instead of relying on overlay mode
3. Check your endpoint delay settings (Mock endpoints → Delay field)

### Authentication Required Popup Keeps Appearing

**Cause:** SOCKS5 authentication is enabled in Mockelot but browser credentials not saved.

**Solution:**
1. Enter the username/password from Mockelot Settings → SOCKS5 Proxy
2. Check "Remember password" in the browser prompt
3. Or disable authentication if not needed (Settings → SOCKS5 Proxy → uncheck "Require Authentication")

---

## Advanced Tips

### Using cURL with SOCKS5 Proxy

```bash
# Basic request
curl --proxy socks5://localhost:1080 https://api.company.com/users

# With authentication
curl --proxy socks5://username:password@localhost:1080 https://api.company.com/users

# With CA certificate
curl --proxy socks5://localhost:1080 --cacert mockelot-ca.crt https://api.company.com/users
```

### Combining Mock and Proxy Endpoints

You can mix mock and proxy endpoints for the same domain:

- **Mock** `/api/login` (authentication)
- **Proxy** `/api/users/*` to `http://localhost:3000` (local service)
- **Overlay** everything else to production

### Using Environment Variables for Different Configurations

Save different Domain Takeover configurations for different testing scenarios (dev, staging, production mix) and switch between them easily.

### Debugging Intercepted Traffic

1. Open the **Traffic Log** panel in Mockelot
2. Filter by domain to see which requests are being intercepted
3. Check the **Backend RTT** column to see if requests are being proxied (non-zero RTT) or mocked (zero RTT)

---

## Summary

SOCKS5 proxy mode in Mockelot is powerful for:
- Multi-domain testing without DNS changes
- Selective endpoint mocking with pass-through for unmatched requests
- Testing local services alongside production/staging backends
- Bypassing broken or slow endpoints during development

The key is the **Overlay Mode** feature, which allows fine-grained control over which requests to intercept and which to pass through to real servers.
