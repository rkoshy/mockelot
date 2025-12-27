# Mockelot Documentation

## Overview

Mockelot is an HTTP mock server and proxy for API development, testing, and debugging.

## Quick Start

1. [Setup & Installation](SETUP.md) - Get started in 5 minutes
2. Create your first mock endpoint
3. Configure HTTPS certificates

## Core Features

- **[Mock Endpoints](MOCK-GUIDE.md)** - Static, template, and script-based responses
- **[Proxy Endpoints](PROXY-GUIDE.md)** - Reverse proxy with transformation
- **[Container Endpoints](CONTAINER-GUIDE.md)** - Docker/Podman integration
- **[SOCKS5 Proxy](SOCKS5-GUIDE.md)** - Multi-domain browser testing
- **[OpenAPI Import](OPENAPI_IMPORT.md)** - Import from Swagger/OpenAPI specs

## Common Scenarios

- REST API mocking during frontend development
- Testing error conditions and edge cases
- Proxying to local services with header manipulation
- Running containerized dependencies (PostgreSQL, Redis, etc.)
- Multi-domain testing without DNS changes

## Configuration

- [HTTPS & SSL Certificates](SETUP.md#enabling-https)
- [CORS Configuration](MOCK-GUIDE.md#cors-configuration)
- [Domain Takeover](SOCKS5-GUIDE.md#step-4-configure-domain-takeover)

## Support

- GitHub Issues: [Report bugs or request features](https://github.com/rkoshy/mockelot/issues)
- Documentation: You're reading it!
