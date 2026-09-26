package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"mockelot/models"
)

const (
	devServerReadyTimeout = 120 * time.Second // max wait for process to become ready
	devServerRingBufSize  = 10_000            // max lines kept in memory per process
	devServerTCPPollInterval = 500 * time.Millisecond
	devServerOutputEvent  = "devsvr:output"
	devServerProgressEvent = "devsvr:progress"
)

// readyURLRegex matches the local URL printed by most JS dev servers when ready.
var readyURLRegex = regexp.MustCompile(`https?://(localhost|127\.0\.0\.1|0\.0\.0\.0|\[::\]):\d+`)

// devServerProcess holds the runtime state for a single running dev server.
type devServerProcess struct {
	cmd       *exec.Cmd
	port      int
	pid       int
	startedAt time.Time
	cancel    context.CancelFunc // cancels the process context
}

// ringBuffer is a fixed-size circular log buffer, safe for concurrent use.
type ringBuffer struct {
	mu    sync.Mutex
	lines []string
	cap   int
	head  int  // index of next write position
	count int  // number of valid entries
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		lines: make([]string, capacity),
		cap:   capacity,
	}
}

func (rb *ringBuffer) append(line string) {
	rb.mu.Lock()
	rb.lines[rb.head] = line
	rb.head = (rb.head + 1) % rb.cap
	if rb.count < rb.cap {
		rb.count++
	}
	rb.mu.Unlock()
}

// tail returns the last n lines (or fewer if fewer exist).
func (rb *ringBuffer) tail(n int) []string {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if n <= 0 || rb.count == 0 {
		return nil
	}
	if n > rb.count {
		n = rb.count
	}
	out := make([]string, n)
	// start at the oldest line we want
	start := (rb.head - n + rb.cap) % rb.cap
	for i := 0; i < n; i++ {
		out[i] = rb.lines[(start+i)%rb.cap]
	}
	return out
}

// DevServerHandler manages dev server child processes and proxies requests to them.
type DevServerHandler struct {
	eventSender  EventSender
	proxyHandler *ProxyHandler

	mu        sync.RWMutex
	processes map[string]*devServerProcess // endpointID → process
	buffers   map[string]*ringBuffer       // endpointID → output buffer
}

// NewDevServerHandler creates a new DevServerHandler.
func NewDevServerHandler(eventSender EventSender, proxyHandler *ProxyHandler) *DevServerHandler {
	return &DevServerHandler{
		eventSender:  eventSender,
		proxyHandler: proxyHandler,
		processes:    make(map[string]*devServerProcess),
		buffers:      make(map[string]*ringBuffer),
	}
}

// ---- Lifecycle ----

// StartDevServer starts the dev server process for the given endpoint.
// It blocks until the server is ready (or timeout/error).
func (h *DevServerHandler) StartDevServer(ctx context.Context, endpoint *models.Endpoint) error {
	cfg := endpoint.DevServerConfig
	if cfg == nil {
		return fmt.Errorf("dev server config is nil")
	}

	// Stop any existing process for this endpoint
	h.stopProcess(endpoint.ID)

	// Ensure output buffer exists
	h.mu.Lock()
	if h.buffers[endpoint.ID] == nil {
		h.buffers[endpoint.ID] = newRingBuffer(devServerRingBufSize)
	}
	buf := h.buffers[endpoint.ID]
	h.mu.Unlock()

	h.emitProgress(endpoint.ID, "starting", "Finding available port...", 10)

	// Find a free port
	port, err := findFreePort()
	if err != nil {
		h.emitProgress(endpoint.ID, "error", "Failed to find available port: "+err.Error(), 0)
		return fmt.Errorf("failed to find free port: %w", err)
	}
	portStr := strconv.Itoa(port)
	log.Printf("[DevServer:%s] Assigned port %d", endpoint.Name, port)

	// Optionally run install first
	if cfg.AutoInstall {
		if _, err := os.Stat(cfg.ProjectDir + "/node_modules"); os.IsNotExist(err) {
			h.emitProgress(endpoint.ID, "installing", "Running install...", 20)
			if err := h.runInstall(ctx, endpoint, cfg, buf); err != nil {
				h.emitProgress(endpoint.ID, "error", "Install failed: "+err.Error(), 0)
				return fmt.Errorf("install failed: %w", err)
			}
		}
	}

	h.emitProgress(endpoint.ID, "starting", fmt.Sprintf("Starting process on port %d...", port), 40)

	// Build command — may be wrapped in a shell if nvm/fnm or pre-run script is configured
	cmdStr := buildCommand(cfg.Command, portStr)
	shell, shellArgs := buildShellInvocation(cfg, cmdStr)

	procCtx, procCancel := context.WithCancel(context.Background()) // NOT the caller ctx — process lives beyond this call
	cmd := exec.CommandContext(procCtx, shell, shellArgs...)
	cmd.Dir = cfg.ProjectDir

	// Set process group so we can kill the entire tree
	setProcAttr(cmd)

	// Build environment
	cmd.Env = buildEnv(cfg, portStr)

	// Capture stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		procCancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		procCancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		procCancel()
		h.emitProgress(endpoint.ID, "error", "Failed to start process: "+err.Error(), 0)
		return fmt.Errorf("failed to start process: %w", err)
	}

	pid := cmd.Process.Pid
	log.Printf("[DevServer:%s] Process started PID=%d, port=%d", endpoint.Name, pid, port)

	proc := &devServerProcess{
		cmd:       cmd,
		port:      port,
		pid:       pid,
		startedAt: time.Now(),
		cancel:    procCancel,
	}

	h.mu.Lock()
	h.processes[endpoint.ID] = proc
	h.mu.Unlock()

	// Update runtime state on config so ServeHTTP can use the port
	cfg.Port = port
	cfg.ProcessID = pid

	// Channel to signal readiness from stdout scanner
	readyCh := make(chan struct{}, 1)

	// Stream stdout in background — scan for ready URL and buffer lines
	go h.streamOutput(endpoint, buf, stdoutPipe, false, readyCh, port)
	go h.streamOutput(endpoint, buf, stderrPipe, true, nil, port)

	// Watch process exit in background
	go func() {
		err := cmd.Wait()
		log.Printf("[DevServer:%s] Process exited: %v", endpoint.Name, err)
		h.mu.Lock()
		// Only clear if it's still our process (not replaced by a restart)
		if p, ok := h.processes[endpoint.ID]; ok && p.pid == pid {
			delete(h.processes, endpoint.ID)
			cfg.Port = 0
			cfg.ProcessID = 0
		}
		h.mu.Unlock()
		h.emitProgress(endpoint.ID, "stopped", "Process exited", 0)

		// Run cleanup script as a separate invocation (non-blocking on next start)
		if cfg.CleanupScript != "" {
			go func() {
				log.Printf("[DevServer:%s] Running cleanup script", endpoint.Name)
				sh, shArgs := loginShell()
				cleanupArgs := append(shArgs, cfg.CleanupScript)
				cleanupCmd := exec.Command(sh, cleanupArgs...)
				cleanupCmd.Dir = cfg.ProjectDir
				cleanupCmd.Env = buildEnv(cfg, portStr)
				if out, err := cleanupCmd.CombinedOutput(); err != nil {
					log.Printf("[DevServer:%s] Cleanup script error: %v\n%s", endpoint.Name, err, string(out))
				} else {
					log.Printf("[DevServer:%s] Cleanup script completed", endpoint.Name)
					if len(out) > 0 {
						// Write output to the ring buffer so it shows in the console
						for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
							buf.append("[cleanup] " + line)
						}
					}
				}
			}()
		}
	}()

	// Wait for ready: stdout URL detection OR TCP polling, whichever wins
	h.emitProgress(endpoint.ID, "starting", "Waiting for server to become ready...", 60)
	if err := h.waitForReady(ctx, endpoint.ID, endpoint.Name, port, readyCh); err != nil {
		// Kill the process since it never became ready
		h.stopProcess(endpoint.ID)
		h.emitProgress(endpoint.ID, "error", "Server did not become ready: "+err.Error(), 0)
		return fmt.Errorf("dev server did not become ready: %w", err)
	}

	h.emitProgress(endpoint.ID, "ready", fmt.Sprintf("Ready on port %d", port), 100)
	log.Printf("[DevServer:%s] Ready on port %d", endpoint.Name, port)
	return nil
}

// StopDevServer stops the dev server process for the given endpoint.
func (h *DevServerHandler) StopDevServer(endpoint *models.Endpoint) error {
	h.stopProcess(endpoint.ID)
	if endpoint.DevServerConfig != nil {
		endpoint.DevServerConfig.Port = 0
		endpoint.DevServerConfig.ProcessID = 0
	}
	h.emitProgress(endpoint.ID, "stopped", "Dev server stopped", 0)
	return nil
}

// StopAll stops all running dev server processes. Called during shutdown.
func (h *DevServerHandler) StopAll() {
	h.mu.RLock()
	ids := make([]string, 0, len(h.processes))
	for id := range h.processes {
		ids = append(ids, id)
	}
	h.mu.RUnlock()

	for _, id := range ids {
		h.stopProcess(id)
	}
}

// stopProcess terminates the process for an endpoint (if running).
func (h *DevServerHandler) stopProcess(endpointID string) {
	h.mu.Lock()
	proc, ok := h.processes[endpointID]
	if ok {
		delete(h.processes, endpointID)
	}
	h.mu.Unlock()

	if !ok || proc == nil {
		return
	}

	log.Printf("[DevServer] Stopping PID=%d", proc.pid)
	gracefulKill(proc)
	proc.cancel()
}

// GetDevServerStatus returns the current status of a dev server endpoint.
func (h *DevServerHandler) GetDevServerStatus(endpointID string) *models.DevServerStatus {
	h.mu.RLock()
	proc, ok := h.processes[endpointID]
	h.mu.RUnlock()

	if !ok {
		return &models.DevServerStatus{EndpointID: endpointID, Running: false}
	}

	uptime := time.Since(proc.startedAt).Milliseconds()
	return &models.DevServerStatus{
		EndpointID: endpointID,
		Running:    true,
		Ready:      true,
		Port:       proc.port,
		ProcessID:  proc.pid,
		StartedAt:  proc.startedAt.Format(time.RFC3339),
		UptimeMs:   uptime,
	}
}

// GetDevServerLogs returns the last `tail` lines of output for an endpoint.
func (h *DevServerHandler) GetDevServerLogs(endpointID string, tail int) string {
	h.mu.RLock()
	buf := h.buffers[endpointID]
	h.mu.RUnlock()

	if buf == nil {
		return ""
	}
	lines := buf.tail(tail)
	return strings.Join(lines, "\n")
}

// ---- HTTP Proxying ----

// ServeHTTP proxies the request to the running dev server process.
func (h *DevServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, endpoint *models.Endpoint, translatedPath string) {
	cfg := endpoint.DevServerConfig
	if cfg == nil || cfg.Port == 0 {
		mockelotError(w, r, "Dev server is not running", http.StatusServiceUnavailable)
		return
	}

	// Build target URL
	targetURL := fmt.Sprintf("http://127.0.0.1:%d%s", cfg.Port, translatedPath)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// We reuse the proxy handler's HTTP client for the actual request.
	// Build a synthetic ProxyConfig pointing at the dev server.
	proxyCfg := &models.ProxyConfig{
		BackendURL:        fmt.Sprintf("http://127.0.0.1:%d", cfg.Port),
		TimeoutSeconds:    30,
		StatusPassthrough: true,
	}
	if cfg.ProxyConfig != nil {
		// Merge user config (headers, status translations, CSP, etc.) over our synthetic one
		proxyCfg = cfg.ProxyConfig
		proxyCfg.BackendURL = fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	}

	// Create a synthetic endpoint with the proxy config so proxyHandler.ServeHTTP works
	syntheticEndpoint := &models.Endpoint{
		ID:              endpoint.ID,
		Name:            endpoint.Name,
		PathPrefix:      endpoint.PathPrefix,
		TranslationMode: endpoint.TranslationMode,
		Type:            models.EndpointTypeProxy,
		ProxyConfig:     proxyCfg,
	}

	h.proxyHandler.ServeHTTP(w, r, syntheticEndpoint, translatedPath, nil)
}

// ---- Internal helpers ----

// streamOutput reads lines from a pipe, appends to the ring buffer, emits events,
// and signals readyCh when the ready URL pattern is detected.
func (h *DevServerHandler) streamOutput(endpoint *models.Endpoint, buf *ringBuffer, pipe io.ReadCloser, isError bool, readyCh chan<- struct{}, port int) {
	defer pipe.Close()
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 64*1024), 64*1024) // generous buffer for long lines

	portStr := strconv.Itoa(port)
	signaled := false

	for scanner.Scan() {
		line := scanner.Text()
		buf.append(line)

		// Emit output event (non-blocking — if frontend is slow we drop, not block)
		if h.eventSender != nil {
			evt := models.DevServerOutputLine{
				EndpointID: endpoint.ID,
				Line:       line,
				IsError:    isError,
			}
			h.eventSender.SendEvent(devServerOutputEvent, evt)
		}

		// Detect ready URL on stdout only
		if !isError && !signaled && readyCh != nil {
			if readyURLRegex.MatchString(line) || strings.Contains(line, ":"+portStr) {
				log.Printf("[DevServer:%s] Ready signal detected from stdout: %s", endpoint.Name, line)
				signaled = true
				select {
				case readyCh <- struct{}{}:
				default:
				}
			}
		}
	}
}

// waitForReady waits for the dev server to become ready, using two strategies:
// 1. stdout URL detection (via readyCh)
// 2. TCP polling fallback
// Whichever fires first wins.
func (h *DevServerHandler) waitForReady(ctx context.Context, endpointID, name string, port int, readyCh <-chan struct{}) error {
	deadline := time.Now().Add(devServerReadyTimeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// TCP polling ticker
	ticker := time.NewTicker(devServerTCPPollInterval)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(devServerReadyTimeout)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeoutTimer.C:
			return fmt.Errorf("timed out after %v waiting for port %d", devServerReadyTimeout, port)

		case <-readyCh:
			// stdout signalled readiness — do one final TCP check to be sure
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err == nil {
				conn.Close()
				return nil
			}
			// Sometimes the process prints "ready" just before the socket opens;
			// give it a moment and fall through to TCP polling.
			log.Printf("[DevServer:%s] stdout ready but TCP not yet open, continuing to poll", name)

		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out after %v", devServerReadyTimeout)
			}
			conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
			if err == nil {
				conn.Close()
				log.Printf("[DevServer:%s] TCP port %d open — server ready", name, port)
				return nil
			}
		}
	}
}

// runInstall runs the package manager install command and streams output.
// If a version manager is configured it runs install inside the same shell wrapper
// so the correct Node version is active.
func (h *DevServerHandler) runInstall(ctx context.Context, endpoint *models.Endpoint, cfg *models.DevServerConfig, buf *ringBuffer) error {
	pm := detectPackageManager(cfg.ProjectDir)
	installCmd := strings.Join(InstallCommand(pm), " ")
	log.Printf("[DevServer:%s] Running: %s", endpoint.Name, installCmd)

	// Use shell wrapping if version manager is configured, else run directly
	var shell string
	var cmdArgs []string
	if cfg.NodeVersionManager != "" {
		// Build a minimal script: version manager setup + install (no pre-run)
		minCfg := &models.DevServerConfig{
			NodeVersionManager: cfg.NodeVersionManager,
			NodeVersion:        cfg.NodeVersion,
			// No PreRunScript for install step
		}
		shell, cmdArgs = buildShellInvocation(minCfg, installCmd)
	} else {
		parts := strings.Fields(installCmd)
		shell = parts[0]
		cmdArgs = parts[1:]
	}

	cmd := exec.CommandContext(ctx, shell, cmdArgs...)
	cmd.Dir = cfg.ProjectDir
	cmd.Env = buildEnv(cfg, "")

	outPipe, _ := cmd.StdoutPipe()
	errPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	// Stream output
	streamText := func(pipe io.ReadCloser, isErr bool) {
		defer pipe.Close()
		sc := bufio.NewScanner(pipe)
		for sc.Scan() {
			line := sc.Text()
			buf.append(line)
			if h.eventSender != nil {
				h.eventSender.SendEvent(devServerOutputEvent, models.DevServerOutputLine{
					EndpointID: endpoint.ID,
					Line:       line,
					IsError:    isErr,
				})
			}
		}
	}
	go streamText(outPipe, false)
	go streamText(errPipe, true)

	return cmd.Wait()
}

// emitProgress sends a DevServerStartProgress event.
func (h *DevServerHandler) emitProgress(endpointID, stage, message string, progress int) {
	if h.eventSender == nil {
		return
	}
	h.eventSender.SendEvent(devServerProgressEvent, models.DevServerStartProgress{
		EndpointID: endpointID,
		Stage:      stage,
		Message:    message,
		Progress:   progress,
	})
}

// ---- Port and command utilities ----

// findFreePort returns an unused TCP port on localhost.
func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// loginShell returns the user's preferred shell and the flag needed to run a
// command string non-interactively: (shell, []string{"-c"}).
// On Windows returns cmd.exe with /C.
func loginShell() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", []string{"/C"}
	}
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/bash"
	}
	return sh, []string{"-c"}
}

// buildShellInvocation returns the executable and arguments that run cmdStr
// (already port-substituted) inside a shell.
//
// When no version manager or pre-run script is configured the command is split
// into argv directly (fast path, no shell overhead).
//
// When nvm/fnm or a pre-run script is present the whole script is assembled and
// handed to the user's $SHELL -c "...".  On Windows nvm-windows is a regular
// binary so it does not need shell sourcing — the cmd.exe wrapper is used only
// for the pre-run script if present.
func buildShellInvocation(cfg *models.DevServerConfig, cmdStr string) (string, []string) {
	hasVersionMgr := cfg.NodeVersionManager == "nvm" || cfg.NodeVersionManager == "fnm"
	hasPreRun := strings.TrimSpace(cfg.PreRunScript) != ""

	// Fast path: no shell wrapping needed
	if !hasVersionMgr && !hasPreRun {
		parts := shellArgs(cmdStr)
		if len(parts) == 0 {
			return "echo", []string{"(empty command)"}
		}
		return parts[0], parts[1:]
	}

	// Build a multi-line shell script
	var sb strings.Builder

	// -- Version manager setup --
	switch cfg.NodeVersionManager {
	case "nvm":
		if runtime.GOOS == "windows" {
			// nvm-windows: plain binary, no sourcing needed
			v := strings.TrimSpace(cfg.NodeVersion)
			if v != "" {
				sb.WriteString("nvm use " + v + "\n")
			}
		} else {
			// Unix: explicitly source nvm.sh so it works in non-interactive shells
			// (bash -c does not source ~/.bashrc, and ~/.bashrc guards against non-interactive)
			nvmDir := os.Getenv("NVM_DIR")
			if nvmDir == "" {
				home, _ := os.UserHomeDir()
				nvmDir = filepath.Join(home, ".nvm")
			}
			sb.WriteString(`export NVM_DIR="` + nvmDir + `"` + "\n")
			sb.WriteString(`[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"` + "\n")
			v := strings.TrimSpace(cfg.NodeVersion)
			if v != "" {
				sb.WriteString("nvm use " + v + " || nvm install " + v + "\n")
			} else {
				// No version specified: use .nvmrc / .node-version if present
				sb.WriteString("[ -f .nvmrc ] || [ -f .node-version ] && nvm use || true\n")
			}
		}

	case "fnm":
		if runtime.GOOS == "windows" {
			// fnm on Windows: use PowerShell env init
			sb.WriteString(`fnm env --use-on-cd | Out-String | Invoke-Expression` + "\n")
		} else {
			sb.WriteString(`eval "$(fnm env)"` + "\n")
		}
		v := strings.TrimSpace(cfg.NodeVersion)
		if v != "" {
			sb.WriteString("fnm use " + v + " || fnm install " + v + "\n")
		} else {
			sb.WriteString("[ -f .nvmrc ] || [ -f .node-version ] && fnm use || true\n")
		}
	}

	// -- Pre-run script --
	if hasPreRun {
		sb.WriteString(strings.TrimRight(cfg.PreRunScript, "\n"))
		sb.WriteString("\n")
	}

	// -- The actual command --
	sb.WriteString(cmdStr)
	sb.WriteString("\n")

	script := sb.String()

	if runtime.GOOS == "windows" && cfg.NodeVersionManager == "fnm" {
		// fnm on Windows needs PowerShell
		return "powershell", []string{"-Command", script}
	}

	sh, flags := loginShell()
	return sh, append(flags, script)
}

// buildCommand returns the final command string with $PORT substituted.
// If $PORT is not present in the original command, "--port <port>" is appended.
// For npm/yarn/pnpm "run <script>" forms, args are appended after "--".
func buildCommand(commandTemplate, portStr string) string {
	if strings.Contains(commandTemplate, "$PORT") {
		return strings.ReplaceAll(commandTemplate, "$PORT", portStr)
	}
	// Append port flag — use "-- --port" for npm/yarn/pnpm/bun run commands
	if isNpmRunCmd(commandTemplate) {
		return commandTemplate + " -- --port " + portStr
	}
	return commandTemplate + " --port " + portStr
}

// isNpmRunCmd returns true if the command is of the form "<pm> run <script>".
func isNpmRunCmd(cmd string) bool {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return false
	}
	pm := parts[0]
	if pm != "npm" && pm != "yarn" && pm != "pnpm" && pm != "bun" {
		return false
	}
	// yarn/bun can run scripts without "run" keyword, but npm always needs it
	if len(parts) >= 2 && parts[1] == "run" {
		return true
	}
	// "yarn dev" or "bun dev" style — also needs "--" separator
	if (pm == "yarn" || pm == "bun") && len(parts) == 2 {
		return true
	}
	return false
}

// shellArgs splits a command string into argv, respecting quoted strings.
func shellArgs(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case inQuote && c == quoteChar:
			inQuote = false
		case !inQuote && (c == '"' || c == '\''):
			inQuote = true
			quoteChar = c
		case !inQuote && c == ' ':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// buildEnv constructs the child process environment.
func buildEnv(cfg *models.DevServerConfig, portStr string) []string {
	// Start with the current process environment
	env := os.Environ()

	// Inject standard dev-server env vars (override any existing values)
	overrides := map[string]string{
		"PORT":    portStr,
		"BROWSER": "none", // suppress CRA auto-open
	}

	// Apply user-defined env vars (overrides take precedence over user values
	// only for PORT — user can set BROWSER themselves if they want)
	for _, ev := range cfg.EnvVars {
		if ev.Name != "" && ev.Value != "" {
			overrides[ev.Name] = ev.Value
		}
	}

	// Build final env slice: filter out keys we're overriding, then append
	var filtered []string
	for _, kv := range env {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			filtered = append(filtered, kv)
			continue
		}
		key := kv[:idx]
		if _, found := overrides[key]; !found {
			filtered = append(filtered, kv)
		}
	}
	for k, v := range overrides {
		filtered = append(filtered, k+"="+v)
	}
	return filtered
}

// gracefulKill and setProcAttr are implemented in platform-specific files:
//   devserver_unix.go    (Linux, macOS)
//   devserver_windows.go (Windows)


