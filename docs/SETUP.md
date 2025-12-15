# Mockelot Setup Guide

This guide walks you through setting up Mockelot from building to running with HTTPS enabled and trusted certificates.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Building the Application](#building-the-application)
- [Starting the Application](#starting-the-application)
- [Enabling HTTPS](#enabling-https)
- [Installing the CA Certificate](#installing-the-ca-certificate)
  - [Step 1: Export the CA Certificate (via UI)](#step-1-export-the-ca-certificate-via-ui)
  - [Step 2: Install on Your Operating System](#step-2-install-on-your-operating-system)
    - [Linux (Ubuntu/Debian)](#linux-ubuntudebian)
    - [Linux (Fedora/RHEL/CentOS)](#linux-fedorарhelcentos)
    - [macOS](#macos)
    - [Windows](#windows)
  - [Step 3: Browser-Specific Import (Alternative Method)](#step-3-browser-specific-import-alternative-method)
    - [Chrome (Windows/macOS)](#chrome-windowsmacos)
    - [Chrome/Chromium/Brave (Linux)](#chromechrommiumbrave-linux)
    - [Brave (Windows/macOS)](#brave-windowsmacos)
    - [Firefox (All Platforms)](#firefox-all-platforms)
- [Verifying the Setup](#verifying-the-setup)
- [Troubleshooting](#troubleshooting)

## Prerequisites

Before you begin, ensure you have the following installed:

- **Go 1.21 or later** - [Download Go](https://golang.org/dl/)
- **Node.js 16 or later** - [Download Node.js](https://nodejs.org/)
- **npm** (comes with Node.js)
- **Wails CLI v2** - Install with:
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```
- **Build dependencies** (Linux only):
  ```bash
  # Ubuntu/Debian
  sudo apt install build-essential libgtk-3-dev libwebkit2gtk-4.0-dev

  # Fedora/RHEL
  sudo dnf install gtk3-devel webkit2gtk3-devel
  ```

## Building the Application

1. **Clone the repository** (if you haven't already):
   ```bash
   git clone <repository-url>
   cd mockelot
   ```

2. **Build the application**:
   ```bash
   ~/go/bin/wails build
   ```

   This will:
   - Generate TypeScript bindings from Go models
   - Install frontend dependencies
   - Build the Vue frontend
   - Compile the Go backend
   - Package the application

3. **Locate the binary**:

   The built application will be at:
   ```
   build/bin/mockelot
   ```

## Starting the Application

### Option 1: Run the Built Binary

```bash
./build/bin/mockelot
```

### Option 2: Development Mode (Hot Reload)

For development with hot reload:

```bash
~/go/bin/wails dev
```

**Note**: In development mode, the app runs on a random port. Use production mode for HTTPS setup.

### First Launch

On first launch:
1. The application window will open
2. Default HTTP server runs on port **8080**
3. HTTPS is **disabled** by default
4. Certificate directory `~/.mockelot/certs/` is created but empty

## Enabling HTTPS

### Step 1: Access HTTPS Settings

1. Click the **Settings** icon (gear) in the top-right corner of the application
2. Navigate to the **HTTPS** tab

### Step 2: Configure HTTPS

**Basic Setup (Auto-Generated Certificates - Recommended)**:

1. Check **Enable HTTPS**
2. Set **HTTPS Port** (default: 8443)
3. Set **Certificate Mode** to **Auto** (default)
4. Optionally check **Redirect HTTP to HTTPS** to automatically redirect HTTP requests
5. Click **Save** at the bottom

**Advanced Setup (Custom Certificates)**:

See [HTTPS Configuration](#https-configuration-modes) section below.

### Step 3: Restart the Server

After saving HTTPS settings:

1. Click **Stop Server** (if running)
2. Click **Start Server**
3. Verify the server started successfully:
   - HTTP server on port 8080 (if HTTP is enabled)
   - HTTPS server on port 8443
   - CA certificate automatically generated (first HTTPS start only)

### Step 4: Test HTTPS (Will Show Warning)

Open your browser to:
```
https://localhost:8443
```

**Expected**: You'll see a security warning because the CA certificate is not trusted yet.
- Chrome: "Your connection is not private" (NET::ERR_CERT_AUTHORITY_INVALID)
- Firefox: "Warning: Potential Security Risk Ahead"
- Safari: "This Connection Is Not Private"

**Do not proceed yet** - instead, install the CA certificate to avoid warnings.

## Installing the CA Certificate

To make browsers trust Mockelot's HTTPS server, you need to install the auto-generated CA certificate as a trusted root certificate.

**Two Installation Methods Available:**

1. **System-Wide Installation (Recommended)** - Install the certificate at the operating system level. This makes the certificate trusted by all applications and browsers (except Firefox on all platforms, and Chrome/Chromium/Brave on Linux).
   - ✅ **Pros**: Works for all apps, single installation
   - ❌ **Cons**: Requires admin/sudo access

2. **Browser-Specific Import** - Install the certificate directly in individual browsers.
   - ✅ **Pros**: No admin access needed, browser-only scope
   - ❌ **Cons**: Must repeat for each browser

**Quick Guide:**
- **Most users**: Use **Step 2** (system-wide installation)
- **Linux Chrome/Brave users**: Use **Step 3** (browser-specific) - these browsers don't use system certificates on Linux
- **Firefox users (all platforms)**: Use **Step 3** (browser-specific) - Firefox always uses its own certificate store
- **Users without admin access**: Use **Step 3** (browser-specific)

### Step 1: Export the CA Certificate (via UI)

**IMPORTANT**: Use Mockelot's built-in export feature, don't manually copy files from the configuration directory.

1. Open Mockelot application
2. Click the **Settings** icon (gear) in the top-right corner
3. Navigate to the **HTTPS** tab
4. Click the **Export CA Certificate** button
5. Choose a location to save the file (e.g., Desktop, Downloads)
6. Save the file as `mockelot-ca.crt` (or any name you prefer)

**Why use the Export button?**
- Ensures correct file is exported
- Handles file permissions automatically
- Works across all platforms
- Provides user-friendly file dialog

**Advanced users only**: The CA certificate is stored at `~/.mockelot/certs/ca.crt` but you should always use the Export button instead of copying this file manually.

### Step 2: Install on Your Operating System

After exporting the CA certificate using the steps above, follow the instructions for your operating system below.

**Note**: In all commands below, `mockelot-ca.crt` refers to the file you exported in Step 1.

#### Linux (Ubuntu/Debian)

```bash
# Copy the exported CA certificate to the system trust store
sudo cp mockelot-ca.crt /usr/local/share/ca-certificates/mockelot.crt

# Update the system CA certificates
sudo update-ca-certificates

# Verify it was added
sudo update-ca-certificates --verbose | grep mockelot
```

**Restart your browser** for changes to take effect.

#### Linux (Fedora/RHEL/CentOS)

```bash
# Copy the exported CA certificate to the system trust store
sudo cp mockelot-ca.crt /etc/pki/ca-trust/source/anchors/mockelot.crt

# Update the system CA certificates
sudo update-ca-trust

# Verify it was added
trust list | grep -i mockelot
```

**Restart your browser** for changes to take effect.

#### macOS

**Method 1: Using Keychain Access (GUI - Recommended)**

1. Locate the exported `mockelot-ca.crt` file (from Step 1)
2. Double-click the `mockelot-ca.crt` file
3. **Keychain Access** will open
4. Select **System** keychain (or **Login** for current user only)
5. Click **Add**
6. Find the "Mockelot Development CA" certificate in the list
7. Double-click it to open details
8. Expand **Trust** section
9. Set **When using this certificate** to **Always Trust**
10. Close the window (you'll be prompted for your password)

**Method 2: Using Command Line**

```bash
# Add the exported certificate to system keychain
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain \
  mockelot-ca.crt

# Verify it was added
security find-certificate -c "Mockelot Development CA" \
  -a /Library/Keychains/System.keychain
```

**Restart your browser** for changes to take effect.

#### Windows

**Method 1: Using Certificate Manager (GUI - Recommended)**

1. Locate the exported `mockelot-ca.crt` file (from Step 1)
2. Right-click the `mockelot-ca.crt` file
3. Select **Install Certificate**
4. Choose **Local Machine** (requires admin) or **Current User**
5. Click **Next**
6. Select **Place all certificates in the following store**
7. Click **Browse**
8. Select **Trusted Root Certification Authorities**
9. Click **OK**, then **Next**, then **Finish**
10. Click **Yes** to confirm the security warning

**Method 2: Using Command Line (PowerShell as Administrator)**

```powershell
# Import the exported certificate to Trusted Root store
Import-Certificate -FilePath "mockelot-ca.crt" `
  -CertStoreLocation Cert:\LocalMachine\Root

# Verify it was added
Get-ChildItem -Path Cert:\LocalMachine\Root | Where-Object { $_.Subject -like "*Mockelot*" }
```

**Restart your browser** for changes to take effect.

### Step 3: Browser-Specific Import (Alternative Method)

If you prefer to import the certificate directly into your browser instead of system-wide installation, or if you don't have admin/sudo access, use these browser-specific methods.

**Note**: After exporting the CA certificate using Step 1, choose either system-wide installation (Step 2) OR browser-specific import (Step 3) - you don't need both.

#### Chrome (Windows/macOS)

**Note**: On Windows and macOS, Chrome uses the system certificate store. The system-level installation in Step 2 is recommended. However, you can also import directly via Chrome settings.

1. Open Chrome
2. Go to **Settings** → **Privacy and security** → **Security**
3. Scroll down and click **Manage certificates**
4. **Windows**: This opens the Windows Certificate Manager
   - Click **Import** in the "Trusted Root Certification Authorities" tab
   - Click **Next**
   - Browse and select the exported `mockelot-ca.crt` file
   - Click **Next**, then **Finish**
5. **macOS**: This opens Keychain Access
   - Follow the macOS system instructions from Step 2 above
6. Restart Chrome

#### Chrome/Chromium/Brave (Linux)

**Note**: Chrome, Chromium, and Brave on Linux use the NSS certificate database, not the system certificate store.

**Method 1: Using certutil (Recommended)**

```bash
# Install NSS tools if not already installed
sudo apt install libnss3-tools  # Ubuntu/Debian
sudo dnf install nss-tools      # Fedora/RHEL

# Import certificate to Chrome/Chromium/Brave NSS database
certutil -d sql:$HOME/.pki/nssdb -A -t "C,," \
  -n "Mockelot Development CA" -i mockelot-ca.crt

# Verify import
certutil -d sql:$HOME/.pki/nssdb -L | grep Mockelot
```

**Expected output:**
```
Mockelot Development CA                                      C,,
```

**Method 2: Using Chrome Settings (GUI)**

1. Open Chrome/Chromium/Brave
2. Go to `chrome://settings/certificates` (or `brave://settings/certificates`)
3. Click the **Authorities** tab
4. Click **Import**
5. Select the exported `mockelot-ca.crt` file
6. Check **Trust this certificate for identifying websites**
7. Click **OK**

**Restart your browser** for changes to take effect.

#### Brave (Windows/macOS)

Brave uses the same certificate store as Chrome:
- **Windows/macOS**: Follow the **Chrome (Windows/macOS)** instructions above, but open Brave instead
- **Linux**: Follow the **Chrome/Chromium/Brave (Linux)** instructions above

Alternatively, you can import directly via Brave settings:
1. Open Brave
2. Go to `brave://settings/certificates`
3. Follow the same steps as Chrome settings method

#### Firefox (All Platforms)

**Note**: Firefox uses its own certificate store and doesn't use the system certificates. You need to import the CA certificate directly into Firefox.

1. Locate the exported `mockelot-ca.crt` file (from Step 1)
2. Open Firefox
3. Go to **Settings** (or **Preferences**)
4. Search for **certificates**
5. Click **View Certificates**
6. Go to the **Authorities** tab
7. Click **Import**
8. Select the exported `mockelot-ca.crt` file
9. Check **Trust this CA to identify websites**
10. Click **OK**

**No restart needed** - changes take effect immediately.

## Verifying the Setup

### 1. Check Certificate Installation

**Linux**:
```bash
# Ubuntu/Debian
awk -v cmd='openssl x509 -noout -subject' '/BEGIN/{close(cmd)};{print | cmd}' \
  < /etc/ssl/certs/ca-certificates.crt | grep Mockelot

# Fedora/RHEL
trust list | grep -i mockelot
```

**macOS**:
```bash
security find-certificate -c "Mockelot Development CA" \
  -a /Library/Keychains/System.keychain
```

**Windows** (PowerShell):
```powershell
Get-ChildItem -Path Cert:\LocalMachine\Root | Where-Object { $_.Subject -like "*Mockelot*" }
```

### 2. Test HTTPS Connection

Open your browser to:
```
https://localhost:8443
```

**Expected**:
- ✅ No security warning
- ✅ Green padlock icon in address bar
- ✅ Certificate shows "Mockelot Development CA" as issuer
- ✅ Page loads normally

**If you still see warnings**, see [Troubleshooting](#troubleshooting).

### 3. Test with curl

```bash
# Should work without warnings
curl https://localhost:8443

# Verify certificate details
curl -v https://localhost:8443 2>&1 | grep "SSL certificate"
```

### 4. Inspect Certificate in Browser

1. Click the padlock icon in the address bar
2. Click **Certificate** or **Connection is secure** → **Certificate is valid**
3. Verify:
   - **Issued to**: localhost (or your custom names)
   - **Issued by**: Mockelot Development CA
   - **Valid from/to**: 1 year validity
   - **Subject Alternative Names**: localhost, 127.0.0.1, ::1 (plus any custom names)

### 5. Verify Browser-Specific Installation

If you used browser-specific import instead of system-wide installation:

**Chrome/Chromium/Brave**:
- Go to `chrome://settings/certificates` (or `brave://settings/certificates`)
- Click **Authorities** tab
- Search for "Mockelot"
- Verify "Mockelot Development CA" appears in the list

**Firefox**:
- Go to Settings → Privacy & Security → Certificates → **View Certificates**
- Click **Authorities** tab
- Search for "Mockelot"
- Verify "Mockelot Development CA" appears in the list

## HTTPS Configuration Modes

Mockelot supports three certificate modes:

### 1. Auto Mode (Recommended)

- **Automatic CA generation** - Mockelot creates a CA certificate on first HTTPS start
- **Automatic server certificate generation** - Server certificate is generated from the CA
- **Custom names supported** - Add custom DNS names and IP addresses in HTTPS settings
- **One-year validity** - Certificates expire after 1 year and must be regenerated

**Best for**: Development, testing, demos

### 2. CA-Provided Mode

- **You provide the CA certificate and key**
- **Mockelot generates server certificates** from your CA
- **Custom names supported**

**Best for**: Corporate environments with existing CA infrastructure

### 3. Certificate-Provided Mode

- **You provide everything**: server certificate, server key, and CA bundle
- **No certificate generation** by Mockelot
- **You manage renewals**

**Best for**: Production deployments with external certificate management

## HTTP/2 Support

Mockelot supports HTTP/2 for both HTTP and HTTPS servers:

1. Go to **Settings** → **HTTP** tab
2. Check **Enable HTTP/2**
3. Click **Save**
4. Restart the server

**Note**: HTTP/2 over HTTPS (h2) provides better performance than HTTP/1.1.

## Troubleshooting

### "Connection is not private" or "NET::ERR_CERT_AUTHORITY_INVALID"

**Problem**: Browser doesn't trust the CA certificate.

**Solutions**:
1. Verify CA certificate was installed correctly (see [Verifying the Setup](#verifying-the-setup))
2. Restart your browser after installing the certificate
3. Clear browser cache and reload the page
4. Re-export and re-install the CA certificate
5. For Chrome/Edge on Linux, ensure NSS tools are installed:
   ```bash
   sudo apt install libnss3-tools  # Ubuntu/Debian
   sudo dnf install nss-tools      # Fedora/RHEL
   ```

### Certificate Shows Wrong Domain

**Problem**: Certificate is for "localhost" but you're accessing via IP or hostname.

**Solution**:
1. Go to **Settings** → **HTTPS** tab
2. Scroll to **Custom Certificate Names**
3. Add your IP addresses or hostnames (one per line):
   ```
   192.168.1.100
   myserver.local
   dev.example.com
   ```
4. Click **Save**
5. Restart the server (this regenerates the server certificate)

### "Certificate has expired"

**Problem**: Auto-generated certificates expire after 1 year.

**Solutions**:
1. In Mockelot, go to **Settings** → **HTTPS** tab
2. Change **Certificate Mode** to a different mode (e.g., from "Auto" to "CA-Provided" and back to "Auto")
   - This forces regeneration of certificates
3. Click **Save**
4. Restart the server
5. Re-export the CA certificate using the **Export CA Certificate** button
6. Re-install the CA certificate using the instructions for your OS above

**Advanced**: You can manually delete the certificate directory at `~/.mockelot/certs/` and restart Mockelot to regenerate, but using the UI method above is recommended.

### Firefox Still Shows Warning (Other Browsers Work)

**Problem**: Firefox uses its own certificate store.

**Solution**: Import the CA certificate directly into Firefox (see [Firefox section](#firefox-all-platforms))

### "Failed to start HTTPS server"

**Problem**: Port already in use or permission denied.

**Solutions**:
1. **Port in use**: Change HTTPS port in Settings → HTTPS tab
2. **Permission denied**: Ports below 1024 require root privileges
   - Use port ≥1024 (e.g., 8443)
   - Or run with sudo: `sudo ./build/bin/mockelot` (not recommended)

### Cannot Access from Other Machines

**Problem**: Accessing https://192.168.1.100:8443 from another computer shows warning.

**Solution**:
1. Add the IP address to **Custom Certificate Names** (see above)
2. **Export and install** the CA certificate on the other machine
3. Restart the server to regenerate certificates with new names

### Linux: System-wide Certificate Not Working in Chrome/Brave

**Problem**: System certificates don't affect Chrome, Chromium, or Brave on Linux distributions.

**Solution**: Chrome, Chromium, and Brave on Linux use the NSS certificate database instead of the system certificate store. Follow the **[Chrome/Chromium/Brave (Linux)](#chromechrommiumbrave-linux)** instructions in the browser-specific import section above.

Quick summary:
```bash
# Install NSS tools
sudo apt install libnss3-tools  # Ubuntu/Debian
sudo dnf install nss-tools      # Fedora/RHEL

# Import certificate
certutil -d sql:$HOME/.pki/nssdb -A -t "C,," \
  -n "Mockelot Development CA" -i mockelot-ca.crt
```

Restart your browser for changes to take effect.

## Security Notes

### Development vs Production

The auto-generated certificates are intended for **development and testing only**:

- ✅ **Safe for local development**
- ✅ **Safe for internal testing environments**
- ❌ **NOT for production internet-facing services**
- ❌ **NOT for sensitive data in production**

For production:
- Use certificates from a trusted CA (Let's Encrypt, DigiCert, etc.)
- Use the **Certificate-Provided** mode
- Follow your organization's certificate management policies

### Protecting Your CA Private Key

The CA private key (stored internally by Mockelot) can sign any certificate:

- **Keep it private** - don't share the exported CA certificate's private key
- **Delete after testing** if you don't need it anymore
- **Regenerate if compromised**:
  1. Go to **Settings** → **HTTPS** tab
  2. Toggle **Certificate Mode** to force regeneration
  3. Click **Save** and restart the server

**Advanced users**: The CA private key is stored at `~/.mockelot/certs/ca.key`. You can secure it with `chmod 600 ~/.mockelot/certs/ca.key` if needed.

### Uninstalling the CA Certificate

When you're done with Mockelot, remove the CA certificate from your system trust store:

**Linux (Ubuntu/Debian)**:
```bash
sudo rm /usr/local/share/ca-certificates/mockelot.crt
sudo update-ca-certificates --fresh
```

**Linux (Fedora/RHEL)**:
```bash
sudo rm /etc/pki/ca-trust/source/anchors/mockelot.crt
sudo update-ca-trust
```

**macOS**:
```bash
sudo security delete-certificate -c "Mockelot Development CA" \
  /Library/Keychains/System.keychain
```

**Windows** (PowerShell as Administrator):
```powershell
Get-ChildItem -Path Cert:\LocalMachine\Root |
  Where-Object { $_.Subject -like "*Mockelot*" } |
  Remove-Item
```

**Firefox**: Settings → Privacy & Security → Certificates → View Certificates → Authorities → Select "Mockelot Development CA" → Delete

## Additional Resources

### Documentation
- **Main Documentation**: See `README.md` for feature overview
- **Mock Endpoints**: See [MOCK-GUIDE.md](MOCK-GUIDE.md) for mock endpoint configuration
- **Proxy Endpoints**: See [PROXY-GUIDE.md](PROXY-GUIDE.md) for reverse proxy configuration
- **Container Endpoints**: See [CONTAINER-GUIDE.md](CONTAINER-GUIDE.md) for Docker/Podman containers
- **OpenAPI Import**: See [OPENAPI_IMPORT.md](OPENAPI_IMPORT.md) for OpenAPI spec import

### Development
- **Development Notes**: See `CLAUDE.md` for architecture and development details
- **Wails Documentation**: https://wails.io/

## Getting Help

If you encounter issues not covered in this guide:

1. Check the application logs in the console
2. Verify all prerequisites are installed
3. Try deleting `~/.mockelot/` and starting fresh
4. Open an issue on GitHub with:
   - Your operating system and version
   - Steps to reproduce the problem
   - Application logs
   - Browser console errors (if browser-related)
