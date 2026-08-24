# File Server Endpoint Guide

File server endpoints serve files from a local directory. They're ideal for mocking a CDN, serving static frontend assets during development, or simulating any HTTP file server — with optional Server-Side Includes (SSI) and full header/status manipulation.

## Table of Contents

- [Overview](#overview)
- [Creating a File Server Endpoint](#creating-a-file-server-endpoint)
- [Configuration Reference](#configuration-reference)
- [Path Translation](#path-translation)
- [Server-Side Includes (SSI)](#server-side-includes-ssi)
- [Header & Status Manipulation](#header--status-manipulation)
- [Overlay Mode (Fallback)](#overlay-mode-fallback)
- [Common Use Cases](#common-use-cases)
- [Best Practices](#best-practices)

## Overview

A file server endpoint maps an incoming URL path to a file on the local filesystem. When a request arrives:

1. The URL path is optionally translated (strip prefix, regex rewrite, or multi-rule)
2. The translated path is joined onto the configured `base_path`
3. The file is read from disk (a security check ensures the resolved path stays inside `base_path`)
4. If `enable_ssi` is on, SSI `<!--#include virtual="...">` directives are resolved before the response is sent
5. If a `proxy_config` is set, outbound header manipulation and status translation are applied
6. If the file is not found, the request falls through to overlay mode (if enabled) or returns 404

Content-Type is determined automatically from the file extension. `.shtml` files are always served as `text/html; charset=utf-8`.

## Creating a File Server Endpoint

**In the UI:**
1. Click **"New Endpoint"** → **"File Server"**
2. Set a name for the endpoint
3. Set the **Base Path** — the local directory to serve from (e.g. `/home/user/my-site`)
4. Configure the **Listen Path** — the URL prefix this endpoint handles (e.g. `/static/`)
5. Optionally enable SSI or configure path translation

**In YAML configuration:**
```yaml
endpoints:
  - type: file_server
    name: "Static Site"
    path: "/static/"
    enabled: true
    file_server_config:
      base_path: "/home/user/my-site"
      enable_ssi: false
```

## Configuration Reference

| Field | Type | Description |
|-------|------|-------------|
| `base_path` | string | Local filesystem directory to serve files from |
| `enable_ssi` | bool | Process SSI `<!--#include virtual="...">` directives in `.shtml` files |
| `proxy_config` | object | Optional — header manipulation and status-code translation (same as proxy endpoints) |

### Base Path

The directory from which files are served. The resolved file path must stay inside this directory — path traversal attempts (e.g. `../../etc/passwd`) are rejected with 400.

```yaml
file_server_config:
  base_path: "/home/user/my-project/dist"
```

If the incoming path is `/static/css/main.css` and `base_path` is `/home/user/my-project/dist`, Mockelot will serve `/home/user/my-project/dist/css/main.css` (after any path translation strips the `/static/` prefix).

## Path Translation

File server endpoints share the same path translation pipeline as proxy endpoints. Use this to strip a URL prefix before mapping to the filesystem.

### Strip Prefix (most common)

```yaml
endpoints:
  - type: file_server
    name: "Static Assets"
    path: "/assets/"
    translation_mode: "strip"
    translate_pattern: "/assets"
    file_server_config:
      base_path: "/home/user/project/dist"
```

Request for `/assets/js/app.js` → strips `/assets` → serves `/home/user/project/dist/js/app.js`.

### Regex Rewrite

```yaml
translation_mode: "regex"
translate_pattern: "^/v[0-9]+/(.*)"
translate_replace: "/$1"
```

Request for `/v2/images/logo.png` → rewrites to `/images/logo.png` → serves from `base_path`.

### No Translation (serve at root)

```yaml
translation_mode: "none"
file_server_config:
  base_path: "/home/user/site"
```

Request for `/index.html` → serves `/home/user/site/index.html` directly.

## Server-Side Includes (SSI)

When `enable_ssi: true`, Mockelot processes SSI `<!--#include virtual="...">` directives in `.shtml` files before sending the response.

```html
<!-- index.shtml -->
<html>
<body>
  <header>
    <!--#include virtual="/partials/nav.shtml" -->
  </header>
  <main>
    <!--#include virtual="/partials/content.shtml" -->
  </main>
</body>
</html>
```

**How it works:**
- The virtual path in each directive is dispatched as an internal sub-request through the full endpoint-matching pipeline — the same way a real browser request would be handled
- SSI includes can nest (up to 10 levels deep)
- Each included path is resolved relative to the endpoint's domain filter and path matching rules — not just the filesystem
- Non-`.shtml` files are served as-is even when SSI is enabled

**Configuration:**
```yaml
file_server_config:
  base_path: "/home/user/my-site"
  enable_ssi: true
```

**Use case:** Serving a legacy SSI-based site locally without an Apache/Nginx setup, or building page-fragment-based prototypes where shared headers/footers are included from separate files.

## Header & Status Manipulation

Attach a `proxy_config` to apply the same header manipulation and status-code translation available on proxy endpoints.

```yaml
file_server_config:
  base_path: "/home/user/dist"
  proxy_config:
    outbound_headers:
      - name: "Cache-Control"
        value: "public, max-age=3600"
      - name: "X-Served-By"
        value: "Mockelot"
    status_translations:
      - from: 404
        to: 200
        body: '{"error": "not found"}'
```

This is useful for:
- Adding CORS headers to all served files
- Overriding cache headers
- Translating a filesystem 404 into a custom API error

## Overlay Mode (Fallback)

If a requested file doesn't exist locally, the endpoint can fall through to a real server via overlay mode. This is configured at the SOCKS5 domain-takeover level (see [SOCKS5-GUIDE.md](SOCKS5-GUIDE.md)).

**Typical pattern:**
- Mock specific paths with a file server
- Unknown paths pass through to the real CDN/origin

```
Request: /images/logo.png
  ↓ File exists in base_path?
  YES → Serve local file
  NO  → Overlay mode → proxy to real origin
```

## Common Use Cases

### 1. Frontend Development Against a Real API

Serve your built frontend locally while routing API calls to a real backend:

```yaml
endpoints:
  # Serve the built frontend
  - type: file_server
    name: "Frontend"
    path: "/"
    translation_mode: "none"
    file_server_config:
      base_path: "/home/user/my-app/dist"

  # Proxy API calls to real backend
  - type: proxy
    name: "API"
    path: "/api/"
    proxy_config:
      target_url: "https://api.example.com"
```

### 2. Mock a CDN

Replace a production CDN domain with local files when testing via SOCKS5:

```yaml
endpoints:
  - type: file_server
    name: "Mock CDN"
    path: "/"
    file_server_config:
      base_path: "/home/user/cdn-assets"
      proxy_config:
        outbound_headers:
          - name: "Cache-Control"
            value: "no-cache"
```

Configure SOCKS5 to intercept `cdn.example.com`, and all asset requests go to your local directory instead.

### 3. Multi-Version Static Site

Serve different directory versions at different URL prefixes:

```yaml
endpoints:
  - type: file_server
    name: "Docs v2"
    path: "/docs/v2/"
    translation_mode: "strip"
    translate_pattern: "/docs/v2"
    file_server_config:
      base_path: "/home/user/docs/v2/dist"

  - type: file_server
    name: "Docs v1 (legacy)"
    path: "/docs/v1/"
    translation_mode: "strip"
    translate_pattern: "/docs/v1"
    file_server_config:
      base_path: "/home/user/docs/v1/dist"
```

### 4. Legacy SSI Site

Run an SSI-based site without a web server:

```yaml
endpoints:
  - type: file_server
    name: "Legacy Site"
    path: "/"
    file_server_config:
      base_path: "/home/user/legacy-site"
      enable_ssi: true
```

Create shared partials as `.shtml` files and include them with `<!--#include virtual="/partials/header.shtml" -->`.

## Best Practices

**1. Always use path translation when serving under a prefix**
Strip the URL prefix before mapping to the filesystem so directory structure doesn't need to mirror the URL.

**2. Point `base_path` to your build output**
For frontend projects, point to the `dist/` or `build/` directory rather than source files so you serve exactly what a real server would.

**3. Enable SSI only when needed**
SSI adds processing overhead on every `.shtml` request. Leave it off unless you're working with SSI-based templates.

**4. Use overlay mode for partial mocking**
When you only want to override some assets, enable overlay mode on the domain so unmatched requests fall through to the real server.

**5. Combine with mock endpoints for API + static**
Use a file server endpoint for static assets and a mock endpoint for API routes on the same listen address — the endpoint matching order determines priority.

---

**Related Documentation:**
- [MOCK-GUIDE.md](MOCK-GUIDE.md) - Script-generated and static API responses
- [PROXY-GUIDE.md](PROXY-GUIDE.md) - Reverse proxy endpoints
- [SOCKS5-GUIDE.md](SOCKS5-GUIDE.md) - Domain-level interception and overlay mode
- [SETUP.md](SETUP.md) - HTTPS and deployment
