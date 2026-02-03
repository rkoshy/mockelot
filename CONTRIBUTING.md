# Contributing to Mockelot

Thank you for your interest in contributing to Mockelot! We welcome contributions from the community to help make this tool better for everyone.

## Getting Started

1.  **Fork the Repository**: Click the "Fork" button on the top right of the GitHub page.
2.  **Clone your Fork**:
    ```bash
    git clone https://github.com/YOUR_USERNAME/mockelot.git
    cd mockelot
    ```
3.  **Install Dependencies**:
    - **Go**: Version 1.21 or higher.
    - **Node.js**: Version 18 or higher.
    - **Wails CLI**: Install with `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.
    - **Docker/Podman**: Optional, but required for container features.

## Development Workflow

### Live Development (Hot Reload)

Run the application in development mode. This will start a Vite server for the frontend and recompile the backend on changes.

```bash
wails dev
```

- **Frontend URL**: `http://localhost:34115` (useful for browser devtools).
- **Backend**: Changes to Go files trigger a rebuild automatically.

### Building for Production

Build the production binary for your current platform:

```bash
wails build
```

The binary will be created in `build/bin/`.

### Cross-Compilation

To build for other platforms (requires standard Go cross-compilation setup):

```bash
wails build -platform windows/amd64
wails build -platform darwin/universal
```

## Project Structure

- **`main.go` / `app.go`**: Entry point and Wails application lifecycle.
- **`frontend/`**: Vue 3 + TypeScript + Tailwind CSS application.
- **`server/`**: Core logic for HTTP mocking, proxying, SOCKS5, and container management.
- **`models/`**: Shared data structures.
- **`openapi/`**: OpenAPI import logic.

## Testing

### Running Tests

Run all Go unit tests:

```bash
go test ./...
```

### Integration Tests

There are shell scripts for testing specific features (e.g., SOCKS5):

```bash
# Start Mockelot first, then run:
./test-socks5.sh
```

## Submitting Changes

1.  **Create a Branch**: `git checkout -b feature/my-new-feature`
2.  **Commit Changes**: Keep commits atomic and messages descriptive.
    - Good: `feat: add regex support to domain filter`
    - Bad: `update code`
3.  **Verify**: Ensure `wails build` succeeds and tests pass.
4.  **Push**: `git push origin feature/my-new-feature`
5.  **Open Pull Request**: Describe your changes and link any related issues.

## Guidelines

- **Code Style**:
    - **Go**: Follow standard Go conventions (`gofmt`).
    - **TypeScript**: Follow the existing Vue/TS patterns.
- **Documentation**: If you add a feature, please update the relevant documentation in `docs/`.
- **Breaking Changes**: Please flag any breaking changes in the PR description.

Thank you for contributing!
