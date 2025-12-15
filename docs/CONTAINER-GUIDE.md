# Container Endpoint Guide

Container endpoints run Docker or Podman containers and proxy HTTP requests to them. They're ideal for testing with real services, databases, and complex applications.

## Table of Contents

- [Overview](#overview)
- [Container Runtime Support](#container-runtime-support)
- [Creating Container Endpoints](#creating-container-endpoints)
- [Image Configuration](#image-configuration)
- [Environment Variables](#environment-variables)
- [Volume Mappings](#volume-mappings)
- [Port Configuration](#port-configuration)
- [Proxy Configuration](#proxy-configuration)
- [Health Checks](#health-checks)
- [Container Lifecycle](#container-lifecycle)
- [Resource Monitoring](#resource-monitoring)
- [Container Logs](#container-logs)
- [Common Use Cases](#common-use-cases)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

Container endpoints manage Docker or Podman containers as part of your mock server setup. They provide:
- **Automatic container lifecycle** - Start, stop, and restart containers
- **HTTP proxying** - Route requests to containerized services
- **Health monitoring** - Container state and HTTP health checks
- **Resource monitoring** - CPU, memory, network, and disk usage
- **Log access** - View container stdout/stderr
- **Volume mounting** - Mount host directories into containers
- **Environment configuration** - Static and dynamic environment variables
- **Path translation** - Same as proxy endpoints

**Request Flow:**
```
Client → Mockelot → Container (localhost:random-port)
         ↓ (manipulate)
Client ← Mockelot ← Container
```

## Container Runtime Support

Mockelot supports both Docker and Podman with automatic detection.

### Runtime Detection

**Automatic (Recommended):**
```bash
# Mockelot auto-detects Docker, then Podman
./mockelot
```

**Manual Override:**
```bash
# Force Docker
CONTAINER_RUNTIME=docker ./mockelot

# Force Podman
CONTAINER_RUNTIME=podman ./mockelot
```

### Docker Support

**Requirements:**
- Docker Engine 20.10+ or Docker Desktop
- Docker daemon running
- User has Docker permissions

**Linux:**
```bash
# Check Docker availability
docker ps

# Add user to docker group (if needed)
sudo usermod -aG docker $USER
newgrp docker
```

**macOS:**
```bash
# Install Docker Desktop
brew install --cask docker

# Start Docker Desktop
open -a Docker
```

**Windows:**
```bash
# Install Docker Desktop
winget install Docker.DockerDesktop
```

### Podman Support

**Requirements:**
- Podman 3.0+
- Podman socket enabled (for API access)

**Linux:**
```bash
# Install Podman
sudo apt install podman  # Ubuntu/Debian
sudo dnf install podman  # Fedora/RHEL

# Enable Podman socket
systemctl --user enable --now podman.socket

# Verify
podman ps
```

**macOS:**
```bash
# Install Podman
brew install podman

# Initialize Podman machine
podman machine init
podman machine start
```

**Windows WSL:**
```bash
# Install in WSL
sudo apt install podman
```

### Runtime Features

Both Docker and Podman provide:
- ✅ Image pulling and management
- ✅ Container lifecycle (create, start, stop, remove)
- ✅ Port binding and networking
- ✅ Volume mounting
- ✅ Environment variables
- ✅ Resource statistics
- ✅ Log streaming
- ✅ Health checks

## Creating Container Endpoints

### Via UI

1. Click **"Add Endpoint"**
2. Select **"Container"** as endpoint type
3. Configure:
   - Name (e.g., "PostgreSQL")
   - Path prefix (e.g., "/db")
   - Image name (e.g., "postgres:15")
   - Container port (e.g., 5432)
4. Optionally configure environment, volumes, health checks
5. Click **"Start Container"**

### Via YAML Configuration

```yaml
endpoints:
  - id: "container-1"
    name: "PostgreSQL Database"
    path_prefix: "/db"
    type: "container"
    enabled: true
    translation_mode: "strip"
    container_config:
      image_name: "postgres:15"
      container_port: 5432
      pull_on_startup: true
      restart_on_server_start: true
      environment:
        - name: "POSTGRES_PASSWORD"
          value: "password"
        - name: "POSTGRES_DB"
          value: "testdb"
      proxy_config:
        backend_url: ""  # Auto-filled with container URL
        timeout_seconds: 30
        health_check_enabled: true
        health_check_interval: 30
        health_check_path: "/"
```

## Image Configuration

### Image Name

Specify the Docker Hub image or full registry path.

```yaml
# Docker Hub official image
image_name: "postgres:15"

# Docker Hub user image
image_name: "username/myapp:latest"

# Google Container Registry
image_name: "gcr.io/project/image:tag"

# GitHub Container Registry
image_name: "ghcr.io/owner/repo:tag"

# Custom registry
image_name: "registry.example.com:5000/app:v1.0"
```

### Pull on Startup

Control when images are pulled.

```yaml
pull_on_startup: true   # Always pull latest (recommended for :latest tags)
pull_on_startup: false  # Use cached image (faster startup)
```

**When to pull:**
- ✅ Using `:latest` tag - always pull to get updates
- ✅ During development - pull to get new builds
- ❌ Using specific version tag (e.g., `:15.2`) - no need to pull repeatedly
- ❌ Large images - skip pulling to save time

### Restart on Server Start

Automatically start container when Mockelot server starts.

```yaml
restart_on_server_start: true   # Auto-start with server
restart_on_server_start: false  # Manual start required
```

**When to use:**
- ✅ Essential services (databases, caches)
- ✅ Long-running services
- ❌ Resource-intensive containers
- ❌ Temporary test services

### Restart Policy

Control container restart behavior.

```yaml
restart_policy: "no"           # Never restart (default)
restart_policy: "always"       # Always restart on failure
restart_policy: "unless-stopped"  # Restart unless explicitly stopped
restart_policy: "on-failure"   # Restart only on non-zero exit
```

## Environment Variables

Environment variables can be static values or JavaScript expressions.

### Static Values

Simple key-value pairs.

```yaml
environment:
  - name: "POSTGRES_PASSWORD"
    value: "mysecretpassword"

  - name: "POSTGRES_DB"
    value: "testdb"

  - name: "POSTGRES_USER"
    value: "admin"
```

### JavaScript Expressions

Dynamic values evaluated at container startup.

```yaml
environment:
  # Generate random password
  - name: "API_KEY"
    expression: "Math.random().toString(36).substring(2, 15)"

  # Current timestamp
  - name: "START_TIME"
    expression: "new Date().toISOString()"

  # Conditional value
  - name: "LOG_LEVEL"
    expression: "process.env.NODE_ENV === 'production' ? 'error' : 'debug'"

  # Computed value
  - name: "MAX_CONNECTIONS"
    expression: "(100 * 2).toString()"
```

**Available in expressions:**
```javascript
Math.random()         // Random numbers
new Date()            // Current date/time
.toString()           // Convert to string
```

### Environment Examples

**Example 1: PostgreSQL Database**
```yaml
environment:
  - name: "POSTGRES_PASSWORD"
    value: "postgres"
  - name: "POSTGRES_DB"
    value: "myapp_dev"
  - name: "POSTGRES_USER"
    value: "developer"
```

**Example 2: Redis Cache**
```yaml
environment:
  - name: "REDIS_PASSWORD"
    expression: "Math.random().toString(36).substring(2, 15)"
  - name: "REDIS_MAXMEMORY"
    value: "256mb"
  - name: "REDIS_MAXMEMORY_POLICY"
    value: "allkeys-lru"
```

**Example 3: Node.js App**
```yaml
environment:
  - name: "NODE_ENV"
    value: "development"
  - name: "PORT"
    value: "3000"
  - name: "DATABASE_URL"
    value: "postgres://user:pass@db:5432/mydb"
  - name: "SESSION_SECRET"
    expression: "Math.random().toString(36).substring(2, 50)"
```

## Volume Mappings

Mount host directories into containers for persistent data or configuration.

### Volume Configuration

```yaml
volumes:
  - host_path: "/home/user/data"
    container_path: "/var/lib/postgresql/data"
    read_only: false

  - host_path: "/home/user/config"
    container_path: "/etc/app/config"
    read_only: true
```

### Path Translation (WSL Support)

Mockelot automatically translates paths for WSL environments:

**Linux/macOS:**
```yaml
host_path: "/home/user/data"
# Used as-is: /home/user/data
```

**Windows (WSL):**
```yaml
host_path: "/home/user/data"
# Translated to: \\wsl$\Ubuntu\home\user\data
```

**Windows (Native):**
```yaml
host_path: "C:\\Users\\user\\data"
# Used as-is: C:\Users\user\data
```

### Read-Only Volumes

Prevent containers from modifying host files.

```yaml
volumes:
  # Read-write (default)
  - host_path: "/data"
    container_path: "/app/data"
    read_only: false

  # Read-only
  - host_path: "/config"
    container_path: "/app/config"
    read_only: true
```

### Volume Examples

**Example 1: PostgreSQL Persistent Data**
```yaml
volumes:
  - host_path: "/home/user/postgres-data"
    container_path: "/var/lib/postgresql/data"
    read_only: false
```

**Example 2: Application Configuration**
```yaml
volumes:
  - host_path: "/home/user/app-config"
    container_path: "/etc/myapp"
    read_only: true

  - host_path: "/home/user/app-logs"
    container_path: "/var/log/myapp"
    read_only: false
```

**Example 3: Nginx Static Files**
```yaml
volumes:
  - host_path: "/home/user/website"
    container_path: "/usr/share/nginx/html"
    read_only: true

  - host_path: "/home/user/nginx-config"
    container_path: "/etc/nginx/conf.d"
    read_only: true
```

## Port Configuration

Containers expose ports that are bound to random host ports.

### Container Port

The port your application listens on inside the container.

```yaml
container_port: 8080   # HTTP service
container_port: 5432   # PostgreSQL
container_port: 6379   # Redis
container_port: 3306   # MySQL
```

### Port Binding

Mockelot automatically binds the container port to a random host port to avoid conflicts.

```yaml
container_port: 8080
# Automatically bound to random host port (e.g., 32768)
# Accessible at: http://127.0.0.1:32768
```

**How it works:**
1. Container exposes port (e.g., 8080)
2. Runtime assigns random host port (e.g., 32768)
3. Mockelot proxies requests to `http://127.0.0.1:32768`
4. Path translation applied based on endpoint settings

### Exposed Ports (Advanced)

Expose additional ports from the container.

```yaml
exposed_ports:
  - "8080/tcp"   # Main HTTP port
  - "8443/tcp"   # HTTPS port
  - "9090/tcp"   # Metrics port
```

## Proxy Configuration

Container endpoints use the same proxy configuration as regular proxy endpoints.

### Basic Proxy Settings

```yaml
container_config:
  proxy_config:
    backend_url: ""  # Auto-filled with container URL
    timeout_seconds: 30
```

### Header Manipulation

Add, remove, or modify headers (same as [PROXY-GUIDE.md](PROXY-GUIDE.md)).

```yaml
proxy_config:
  inbound_headers:
    - name: "X-Forwarded-For"
      mode: "expression"
      expression: "request.remoteAddr"

    - name: "Authorization"
      mode: "replace"
      value: "Bearer test-token"
```

### Status Code Translation

Translate container response status codes.

```yaml
proxy_config:
  status_passthrough: false
  status_translation:
    - from_pattern: "5xx"
      to_code: 503

    - from_pattern: "404"
      to_code: 200
```

### Body Transformation

Transform container responses using JavaScript.

```yaml
proxy_config:
  body_transform: |
    const data = JSON.parse(body);
    data.proxied_through = "mockelot";
    return JSON.stringify(data, null, 2);
```

See [PROXY-GUIDE.md](PROXY-GUIDE.md) for detailed proxy configuration options.

## Health Checks

Monitor container health with state checks and HTTP health endpoints.

### Container State Check

Always enabled - monitors if container is running.

```yaml
# No configuration needed - automatic
```

**Health states:**
- ✅ Healthy: Container running
- ❌ Unhealthy: Container stopped, crashed, or removed
- ⏳ Starting: Container starting up

### HTTP Health Check

Optional HTTP endpoint check.

```yaml
container_config:
  proxy_config:
    health_check_enabled: true
    health_check_interval: 30  # seconds
    health_check_path: "/health"
```

**How it works:**
1. Mockelot sends GET request to `http://127.0.0.1:<port>/health`
2. Status codes 200-499: Healthy
3. Status codes ≥500 or timeout: Unhealthy
4. Container state also checked

### Health Check Examples

**Example 1: Basic Health Endpoint**
```yaml
proxy_config:
  health_check_enabled: true
  health_check_interval: 60  # Check every minute
  health_check_path: "/health"
```

**Example 2: Custom Health Path**
```yaml
proxy_config:
  health_check_enabled: true
  health_check_interval: 10  # Check every 10 seconds
  health_check_path: "/api/status"
```

**Example 3: Root Path Check**
```yaml
proxy_config:
  health_check_enabled: true
  health_check_interval: 30
  health_check_path: "/"
```

### Viewing Health Status

Health status is displayed in the UI and available via events.

**UI Display:**
- 🟢 Green: Healthy
- 🔴 Red: Unhealthy
- 🟡 Yellow: Starting

## Container Lifecycle

### Starting Containers

**Via UI:**
1. Configure container endpoint
2. Click **"Start Container"**
3. Progress displayed:
   - Pulling image (if enabled)
   - Creating container
   - Starting container
   - Ready

**Via API:**
```javascript
// Wails backend call
StartContainer(endpointId)
```

**Startup Process:**
1. Pull image (if `pull_on_startup: true`)
2. Remove existing container with same name
3. Create new container
4. Start container
5. Begin health checks

### Stopping Containers

**Via UI:**
1. Click **"Stop Container"**
2. Container stopped gracefully (10 second timeout)
3. Container removed

**Via API:**
```javascript
// Wails backend call
StopContainer(endpointId)
```

**Shutdown Process:**
1. Send stop signal to container
2. Wait up to 10 seconds
3. Force kill if not stopped
4. Remove container

### Container Naming

Containers are named automatically:
- Pattern: `mockelot-<endpoint-name>`
- Sanitized (lowercase, alphanumeric, hyphens only)

**Examples:**
- Endpoint "PostgreSQL" → `mockelot-postgresql`
- Endpoint "My API Server" → `mockelot-my-api-server`
- Endpoint "redis_cache" → `mockelot-redis-cache`

### Restart Behavior

**Manual Restart:**
1. Stop container
2. Start container
3. New container created (fresh state)

**Auto-Restart on Server Start:**
```yaml
restart_on_server_start: true
```

**Restart Policy:**
```yaml
restart_policy: "always"  # Restart on failure
```

## Resource Monitoring

Mockelot monitors container resource usage in real-time.

### Monitored Metrics

- **CPU Usage**: Percentage (0-100+)
- **Memory Usage**: MB used, MB limit, percentage
- **Network I/O**: Bytes received/transmitted
- **Block I/O**: Bytes read/written
- **Process Count**: Number of PIDs

### Viewing Stats

**UI Display:**
Container stats shown in endpoint panel:
- CPU: 2.5%
- Memory: 128 MB / 2048 MB (6.25%)
- Network RX: 1.2 MB
- Network TX: 512 KB
- Disk Read: 4 MB
- Disk Write: 2 MB
- PIDs: 12

**Update Frequency:**
- First 60 seconds: Every 1 second (fast polling)
- After 60 seconds: Every 5 seconds (steady-state)

### Stats API

Stats available via WebSocket events:
```javascript
// Event: ctr:stats
{
  endpoint_id: "container-1",
  cpu_percent: 2.5,
  memory_usage_mb: 128.5,
  memory_limit_mb: 2048,
  memory_percent: 6.28,
  network_rx_bytes: 1258291,
  network_tx_bytes: 524288,
  block_read_bytes: 4194304,
  block_write_bytes: 2097152,
  pids: 12,
  last_check: "2025-12-15T10:30:45Z"
}
```

## Container Logs

Access container stdout and stderr logs.

### Viewing Logs

**Via UI:**
1. Click **"View Logs"** on container endpoint
2. Console dialog shows recent logs
3. Auto-updates as new logs arrive

**Log Display:**
- Last 100 lines shown by default
- Configurable line limit in settings
- ANSI color codes preserved
- Auto-scroll to bottom

### Log Configuration

```yaml
# Global setting
container_log_line_limit: 100  # Default: 100 lines
```

### Log Examples

**Viewing PostgreSQL Logs:**
```
PostgreSQL Database init process complete; ready for start up.
LOG:  database system was shut down at 2025-12-15 10:25:03 UTC
LOG:  database system is ready to accept connections
```

**Viewing Node.js App Logs:**
```
Server listening on port 3000
Connected to database
API ready at http://localhost:3000/api
```

## Common Use Cases

### 1. PostgreSQL Database

Run a PostgreSQL database for testing.

```yaml
endpoints:
  - name: "PostgreSQL"
    path_prefix: "/db"
    type: "container"
    translation_mode: "strip"
    container_config:
      image_name: "postgres:15"
      container_port: 5432
      pull_on_startup: false
      restart_on_server_start: true
      environment:
        - name: "POSTGRES_PASSWORD"
          value: "postgres"
        - name: "POSTGRES_DB"
          value: "testdb"
      volumes:
        - host_path: "/home/user/pgdata"
          container_path: "/var/lib/postgresql/data"
          read_only: false
      proxy_config:
        health_check_enabled: true
        health_check_interval: 30
        health_check_path: "/"
```

**Usage:**
```bash
# Connect from host
psql -h localhost -p <mapped-port> -U postgres -d testdb

# From application
DATABASE_URL=postgresql://postgres:postgres@localhost:<port>/testdb
```

### 2. Redis Cache

Run Redis for caching and session storage.

```yaml
endpoints:
  - name: "Redis"
    path_prefix: "/redis"
    type: "container"
    container_config:
      image_name: "redis:7-alpine"
      container_port: 6379
      pull_on_startup: false
      restart_on_server_start: true
      environment:
        - name: "REDIS_MAXMEMORY"
          value: "256mb"
        - name: "REDIS_MAXMEMORY_POLICY"
          value: "allkeys-lru"
      volumes:
        - host_path: "/home/user/redis-data"
          container_path: "/data"
          read_only: false
      proxy_config:
        health_check_enabled: true
        health_check_interval: 10
        health_check_path: "/"
```

### 3. Nginx Static Server

Serve static files with Nginx.

```yaml
endpoints:
  - name: "Static Server"
    path_prefix: "/static"
    type: "container"
    translation_mode: "strip"
    container_config:
      image_name: "nginx:alpine"
      container_port: 80
      pull_on_startup: false
      volumes:
        - host_path: "/home/user/website"
          container_path: "/usr/share/nginx/html"
          read_only: true
      proxy_config:
        health_check_enabled: true
        health_check_interval: 60
        health_check_path: "/"
```

**Usage:**
```
# Client requests: /static/index.html
# Container serves: /usr/share/nginx/html/index.html
```

### 4. Custom API Service

Run a custom containerized API.

```yaml
endpoints:
  - name: "My API"
    path_prefix: "/api"
    type: "container"
    translation_mode: "strip"
    container_config:
      image_name: "myusername/myapi:latest"
      container_port: 3000
      pull_on_startup: true  # Always pull latest
      restart_on_server_start: true
      environment:
        - name: "NODE_ENV"
          value: "development"
        - name: "API_KEY"
          expression: "Math.random().toString(36).substring(2, 15)"
        - name: "DATABASE_URL"
          value: "postgres://user:pass@db:5432/mydb"
      volumes:
        - host_path: "/home/user/api-logs"
          container_path: "/app/logs"
          read_only: false
      proxy_config:
        health_check_enabled: true
        health_check_interval: 30
        health_check_path: "/health"

        # Add CORS headers
        outbound_headers:
          - name: "Access-Control-Allow-Origin"
            mode: "replace"
            value: "*"
```

### 5. MySQL Database

Run MySQL for testing.

```yaml
endpoints:
  - name: "MySQL"
    path_prefix: "/mysql"
    type: "container"
    container_config:
      image_name: "mysql:8.0"
      container_port: 3306
      pull_on_startup: false
      restart_on_server_start: true
      environment:
        - name: "MYSQL_ROOT_PASSWORD"
          value: "rootpassword"
        - name: "MYSQL_DATABASE"
          value: "testdb"
        - name: "MYSQL_USER"
          value: "testuser"
        - name: "MYSQL_PASSWORD"
          value: "testpass"
      volumes:
        - host_path: "/home/user/mysql-data"
          container_path: "/var/lib/mysql"
          read_only: false
      proxy_config:
        health_check_enabled: true
        health_check_interval: 30
        health_check_path: "/"
```

### 6. MongoDB

Run MongoDB for document storage testing.

```yaml
endpoints:
  - name: "MongoDB"
    path_prefix: "/mongo"
    type: "container"
    container_config:
      image_name: "mongo:7"
      container_port: 27017
      pull_on_startup: false
      restart_on_server_start: true
      environment:
        - name: "MONGO_INITDB_ROOT_USERNAME"
          value: "admin"
        - name: "MONGO_INITDB_ROOT_PASSWORD"
          value: "password"
      volumes:
        - host_path: "/home/user/mongo-data"
          container_path: "/data/db"
          read_only: false
      proxy_config:
        health_check_enabled: true
        health_check_interval: 30
```

## Best Practices

### 1. Image Management

**Best practices:**
- Use specific version tags (e.g., `postgres:15.2`) instead of `:latest`
- Pull images manually before first use to verify availability
- Enable `pull_on_startup` only for `:latest` tags or during development
- Use official images from Docker Hub when possible
- Scan images for vulnerabilities before use

### 2. Resource Limits

**Consider:**
- Set memory limits for resource-intensive containers
- Monitor CPU usage via stats panel
- Use Alpine-based images for smaller footprint
- Limit number of concurrent containers

**Example (Docker Compose style):**
```yaml
# Note: Mockelot doesn't expose resource limits in UI yet
# Set via docker run --memory, --cpus flags if running manually
```

### 3. Data Persistence

**Best practices:**
- Always use volumes for persistent data (databases, logs)
- Use absolute host paths
- Ensure host directories exist before starting container
- Set appropriate read-only flags
- Back up volume data regularly

### 4. Security

**Best practices:**
- Don't expose sensitive ports publicly
- Use strong passwords in environment variables
- Consider using read-only volumes for config files
- Limit container capabilities (not exposed in Mockelot yet)
- Scan images for vulnerabilities
- Don't run containers as root (use image-specific users)

### 5. Health Checks

**Best practices:**
- Enable health checks for critical services
- Use lightweight health endpoints (avoid expensive operations)
- Set appropriate intervals (10-60 seconds typical)
- Monitor health status in UI
- Don't use health checks for simple containers (Alpine, Nginx static)

### 6. Container Lifecycle

**Best practices:**
- Enable `restart_on_server_start` for essential services
- Use `pull_on_startup: false` for stable, versioned images
- Stop containers when not in use to free resources
- Clean up volumes when removing endpoints
- Use unique endpoint names to avoid container name conflicts

### 7. Networking

**Best practices:**
- Containers use random host ports (avoid conflicts)
- Use path translation to route requests correctly
- Test container accessibility before proxying
- Monitor network stats for performance issues
- Consider Docker networking for multi-container setups

### 8. Logging

**Best practices:**
- Monitor logs during container startup
- Increase log line limit for verbose applications
- Clear logs periodically to save memory
- Use structured logging in containerized apps
- Check logs when health checks fail

### 9. Development Workflow

**Recommended workflow:**
1. Pull image manually first (`docker pull <image>`)
2. Test container manually (`docker run`)
3. Configure in Mockelot with `restart_on_server_start: false`
4. Test startup and health checks
5. Enable auto-restart when stable
6. Configure volumes for persistent data
7. Add health checks for production-like testing

### 10. Performance

**Optimize for:**
- Use Alpine-based images for faster pulls
- Disable `pull_on_startup` for faster starts
- Set appropriate health check intervals (not too frequent)
- Monitor resource usage via stats panel
- Stop unused containers to free resources
- Use `restart_policy: "no"` for development

## Troubleshooting

### Container Fails to Start

**Problem**: Container shows "error" status after start attempt.

**Solutions:**
1. **Check container logs**:
   - Click "View Logs" to see error messages
   - Common issues: port conflicts, missing environment variables

2. **Verify image exists**:
   ```bash
   docker pull <image-name>
   # or
   podman pull <image-name>
   ```

3. **Check runtime availability**:
   ```bash
   docker ps  # or podman ps
   ```

4. **Verify environment variables**:
   - Check for required env vars in image documentation
   - Validate JavaScript expressions don't have errors

5. **Check volume paths**:
   - Ensure host directories exist
   - Verify permissions (readable/writable)

### Health Check Fails

**Problem**: Container running but health check shows unhealthy.

**Solutions:**
1. **Verify health endpoint**:
   ```bash
   # Get container port mapping
   docker ps

   # Test health endpoint
   curl http://localhost:<port>/health
   ```

2. **Check health path**:
   - Ensure `health_check_path` exists in container
   - Try `/` or `/health` or image-specific path

3. **Increase health interval**:
   - Some services need time to start
   - Set `health_check_interval: 60` for slower services

4. **Disable HTTP health check**:
   - Set `health_check_enabled: false`
   - Rely on container state check only

### Container Not Accessible

**Problem**: Requests to container endpoint return 503.

**Solutions:**
1. **Check container status**:
   - Verify container is running (green status in UI)
   - Click "View Logs" for errors

2. **Verify port binding**:
   ```bash
   docker ps
   # or
   podman ps
   ```
   - Check that container port is bound to host port

3. **Test direct access**:
   ```bash
   # Get mapped port from docker ps
   curl http://localhost:<mapped-port>/
   ```

4. **Check path translation**:
   - Verify `translation_mode` is correct
   - Try different modes (none, strip, translate)

5. **Increase timeout**:
   - Set `timeout_seconds: 60` for slow containers

### Volume Mount Issues

**Problem**: Container can't access mounted volumes.

**Solutions:**
1. **Verify host path exists**:
   ```bash
   ls -la /path/to/host/directory
   ```

2. **Check permissions**:
   ```bash
   # Make directory readable/writable
   chmod 755 /path/to/host/directory
   ```

3. **WSL path translation**:
   - On WSL, use Linux paths: `/home/user/data`
   - Mockelot auto-translates to `\\wsl$\Ubuntu\home\user\data`

4. **Verify mount in container**:
   ```bash
   docker exec <container-id> ls -la /container/path
   # or
   podman exec <container-id> ls -la /container/path
   ```

### High Resource Usage

**Problem**: Container consuming too much CPU/memory.

**Solutions:**
1. **Monitor stats panel**:
   - Check CPU and memory usage
   - Identify resource-intensive containers

2. **Use lighter images**:
   - Switch to Alpine-based variants
   - Example: `nginx:alpine` instead of `nginx:latest`

3. **Stop unused containers**:
   - Stop containers not currently needed
   - Click "Stop Container" in UI

4. **Restart containers**:
   - Stop and restart to clear accumulated state
   - Fresh container often uses less resources

### Runtime Not Detected

**Problem**: "Container runtime not available" error.

**Solutions:**
1. **Install Docker or Podman**:
   ```bash
   # Ubuntu/Debian
   sudo apt install docker.io
   # or
   sudo apt install podman
   ```

2. **Start Docker daemon**:
   ```bash
   sudo systemctl start docker
   sudo systemctl enable docker
   ```

3. **Enable Podman socket**:
   ```bash
   systemctl --user enable --now podman.socket
   ```

4. **Add user to docker group**:
   ```bash
   sudo usermod -aG docker $USER
   newgrp docker
   ```

5. **Override runtime detection**:
   ```bash
   CONTAINER_RUNTIME=docker ./mockelot
   ```

### Pull Progress Stuck

**Problem**: Image pull appears stuck at certain percentage.

**Solutions:**
1. **Wait patiently**:
   - Large images take time to download
   - Pull progress may pause during extraction

2. **Check network connectivity**:
   ```bash
   docker pull <image-name>
   # or
   podman pull <image-name>
   ```

3. **Cancel and retry**:
   - Click "Cancel" in progress dialog
   - Try starting container again

4. **Pull manually first**:
   ```bash
   docker pull <image-name>
   ```
   - Then start container with `pull_on_startup: false`

---

**Related Documentation:**
- [MOCK-GUIDE.md](MOCK-GUIDE.md) - Mock endpoints with static, template, and script responses
- [PROXY-GUIDE.md](PROXY-GUIDE.md) - Reverse proxy endpoints with header manipulation
- [OPENAPI_IMPORT.md](OPENAPI_IMPORT.md) - Generate endpoints from OpenAPI specifications
- [SETUP.md](SETUP.md) - HTTPS configuration and deployment
