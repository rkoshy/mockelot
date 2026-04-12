package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mockelot/models"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
)

// ProxyHandler handles reverse proxy requests with translation capabilities
type ProxyHandler struct {
	logger          RequestLogger
	healthStatus    map[string]*models.HealthStatus
	healthMutex     sync.RWMutex
	expressionCache map[string]*goja.Program // Cache for compiled JS expressions
	cacheMutex      sync.RWMutex             // Mutex for expression cache
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(logger RequestLogger) *ProxyHandler {
	return &ProxyHandler{
		logger:          logger,
		healthStatus:    make(map[string]*models.HealthStatus),
		expressionCache: make(map[string]*goja.Program),
	}
}

// ServeHTTP handles a proxy request
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, endpoint *models.Endpoint, translatedPath string, captureGroups []string) {
	cfg := endpoint.ProxyConfig
	if cfg == nil {
		http.Error(w, "Proxy configuration missing", http.StatusInternalServerError)
		return
	}

	// Check if this is a WebSocket upgrade request
	if p.isWebSocketUpgrade(r) {
		p.handleWebSocket(w, r, endpoint, translatedPath, captureGroups)
		return
	}

	// Build backend URL with capture group substitution.
	// resolveBackendURL handles scheme-relative URLs ("//host") used by overlay endpoints
	// so the correct scheme (http/https) is inferred from the incoming request.
	backendURLStr := resolveBackendURL(substituteCaptureGroups(cfg.BackendURL, captureGroups), r, false)
	backendURL, err := url.Parse(backendURLStr)
	if err != nil {
		http.Error(w, "Invalid backend URL", http.StatusInternalServerError)
		return
	}

	// Apply path to backend URL
	backendURL.Path = translatedPath
	backendURL.RawQuery = r.URL.RawQuery

	// Capture original request data for logging
	var requestBody string
	var bodyReader io.Reader
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		requestBody = string(bodyBytes)
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Capture original request headers
	requestHeaders := make(map[string][]string, len(r.Header))
	for name, values := range r.Header {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		requestHeaders[name] = valuesCopy
	}

	// Capture query parameters
	queryParams := make(map[string][]string)
	for key, values := range r.URL.Query() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		queryParams[key] = valuesCopy
	}

	// Generate request ID for tracking
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Build full client URL for logging
	clientScheme := "http"
	if r.TLS != nil {
		clientScheme = "https"
	}
	clientFullURL := clientScheme + "://" + r.Host + r.URL.RequestURI()

	// Log request immediately as pending (before waiting for response)
	p.logPendingRequest(requestID, endpoint, r, clientFullURL, requestHeaders, requestBody, queryParams)

	// Create proxy request
	proxyReq, err := http.NewRequest(r.Method, backendURL.String(), bodyReader)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Log the backend URL being proxied to
	log.Printf("Proxy request: %s %s", r.Method, backendURL.String())

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Apply inbound header manipulation
	p.applyHeaderManipulation(proxyReq.Header, cfg.InboundHeaders, r)

	// Capture backend request headers for logging
	backendReqHeaders := make(map[string][]string, len(proxyReq.Header))
	for name, values := range proxyReq.Header {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		backendReqHeaders[name] = valuesCopy
	}

	// Capture client request start time
	clientStartTime := time.Now()

	// Build full backend URL
	backendFullURL := backendURL.String()

	// Build backend query params from URL
	backendQueryParams := make(map[string][]string)
	for key, values := range backendURL.Query() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		backendQueryParams[key] = valuesCopy
	}

	// Connection (dial) timeout — how long to wait for TCP+TLS to complete.
	connectTimeout := time.Duration(cfg.ConnectTimeoutSeconds) * time.Second
	if connectTimeout <= 0 {
		connectTimeout = 5 * time.Second // safe default
	}

	// Total request timeout — used as ResponseHeaderTimeout (how long the backend must
	// begin responding) and as the first-event deadline for SSE streams.
	// After the first SSE event arrives the relay runs indefinitely (no overall cap),
	// matching WebSocket behaviour. 0 = unlimited.
	totalTimeout := time.Duration(cfg.TimeoutSeconds) * time.Second

	// Use the client-disconnect context without a deadline. We rely on
	// ResponseHeaderTimeout rather than a context deadline so that SSE body reads
	// are not killed after TimeoutSeconds.
	proxyReq = proxyReq.WithContext(r.Context())

	// Execute backend request and measure timing.
	// Note: Don't follow redirects — pass them through to the client.
	client := &http.Client{
		Timeout: 0, // no overall timeout; ResponseHeaderTimeout handles backend latency
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: connectTimeout,
			}).DialContext,
			ResponseHeaderTimeout: totalTimeout, // 0 = unlimited
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	backendStartTime := time.Now()
	resp, err := client.Do(proxyReq)
	backendFirstByteTime := time.Now() // Response headers received

	if err != nil {
		http.Error(w, "Backend request failed", http.StatusBadGateway)
		// Note: For error cases, we don't have complete timing data
		return
	}
	defer resp.Body.Close()

	// SSE: stream instead of buffering. totalTimeout is used as the first-event
	// deadline; after the first event arrives the relay runs indefinitely (no
	// overall cap), matching WebSocket behaviour.
	if isSSEResponse(resp) {
		log.Printf("[SSE] Streaming response (endpoint: %s, path: %s)", endpoint.Name, r.URL.Path)
		p.streamSSEResponse(w, r, resp, endpoint, requestID, totalTimeout,
			clientFullURL, requestHeaders, requestBody, queryParams,
			backendURL.String(), backendReqHeaders, backendStartTime)
		return
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusBadGateway)
		return
	}
	backendCompletionTime := time.Now() // Full response received

	// Calculate backend timing metrics
	backendDelayMs := backendFirstByteTime.Sub(backendStartTime).Milliseconds()
	backendRTTMs := backendCompletionTime.Sub(backendStartTime).Milliseconds()

	// Capture backend response headers for logging
	backendRespHeaders := make(map[string][]string, len(resp.Header))
	for name, values := range resp.Header {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		backendRespHeaders[name] = valuesCopy
	}

	// Save original backend response body before transformation
	originalBackendBody := string(bodyBytes)
	backendStatusCode := resp.StatusCode
	backendStatusText := http.StatusText(resp.StatusCode)

	// Apply body transformation
	if cfg.BodyTransform != "" {
		bodyBytes, err = p.transformBody(bodyBytes, resp.Header.Get("Content-Type"), cfg.BodyTransform)
		if err != nil {
			http.Error(w, "Body transformation failed", http.StatusInternalServerError)
			return
		}
	}

	// Apply status code translation
	statusCode := resp.StatusCode
	if !cfg.StatusPassthrough {
		statusCode = p.translateStatusCode(resp.StatusCode, cfg.StatusTranslation)
	}

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Rewrite redirect Location headers to route back through our proxy
	if statusCode >= 300 && statusCode < 400 {
		if location := resp.Header.Get("Location"); location != "" {
			rewrittenLocation := p.rewriteRedirectLocation(location, cfg.BackendURL, r.URL.Path, translatedPath, endpoint, r)
			if rewrittenLocation != location {
				w.Header().Set("Location", rewrittenLocation)
				log.Printf("Redirect rewrite: %s -> %s", location, rewrittenLocation)
			}
		}
	}

	// Apply outbound header manipulation
	p.applyHeaderManipulation(w.Header(), cfg.OutboundHeaders, r)

	// Capture final response headers for logging
	finalRespHeaders := make(map[string][]string, len(w.Header()))
	for name, values := range w.Header() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		finalRespHeaders[name] = valuesCopy
	}

	// Capture time before sending first byte to client
	clientFirstByteTime := time.Now()

	// Write response
	w.WriteHeader(statusCode)
	w.Write(bodyBytes)

	// Capture client completion time
	clientCompletionTime := time.Now()

	// Calculate client timing metrics
	clientDelayMs := clientFirstByteTime.Sub(clientStartTime).Milliseconds()
	clientRTTMs := clientCompletionTime.Sub(clientStartTime).Milliseconds()

	// Log request with full proxy details (both client and backend sides)
	// This updates the pending log entry created at the start of the request
	p.logProxyRequest(requestID, endpoint, r,
		clientFullURL, requestHeaders, requestBody, queryParams,
		statusCode, finalRespHeaders, string(bodyBytes), clientDelayMs, clientRTTMs,
		backendFullURL, r.Method, translatedPath, backendQueryParams, backendReqHeaders,
		backendStatusCode, backendStatusText, backendRespHeaders, originalBackendBody, backendDelayMs, backendRTTMs)
}

// compileExpression compiles a JS expression and caches it
func (p *ProxyHandler) compileExpression(expression string) (*goja.Program, error) {
	// Check cache first (read lock)
	p.cacheMutex.RLock()
	if program, exists := p.expressionCache[expression]; exists {
		p.cacheMutex.RUnlock()
		return program, nil
	}
	p.cacheMutex.RUnlock()

	// Compile expression (outside lock to avoid blocking readers)
	program, err := goja.Compile("", expression, false)
	if err != nil {
		return nil, err
	}

	// Store in cache (write lock)
	p.cacheMutex.Lock()
	p.expressionCache[expression] = program
	p.cacheMutex.Unlock()

	return program, nil
}

// InvalidateExpressionCache clears the expression cache (call when config changes)
func (p *ProxyHandler) InvalidateExpressionCache() {
	p.cacheMutex.Lock()
	p.expressionCache = make(map[string]*goja.Program)
	p.cacheMutex.Unlock()
}

// applyHeaderManipulation applies header manipulation rules
func (p *ProxyHandler) applyHeaderManipulation(headers http.Header, manipulations []models.HeaderManipulation, originalReq *http.Request) {
	p.applyHeaderManipulationWithContext(headers, manipulations, originalReq, nil)
}

// applyHeaderManipulationWithContext applies header manipulation rules with custom JS context
func (p *ProxyHandler) applyHeaderManipulationWithContext(headers http.Header, manipulations []models.HeaderManipulation, originalReq *http.Request, customContext map[string]interface{}) {
	if len(manipulations) == 0 {
		return
	}

	vm := goja.New() // JS engine for expressions

	// Set up JS context with request data
	requestContext := map[string]interface{}{
		"method":  originalReq.Method,
		"path":    originalReq.URL.Path,
		"headers": originalReq.Header,
		"host":    originalReq.Host,
		"remoteAddr": originalReq.RemoteAddr,
	}

	// Add TLS info
	if originalReq.TLS != nil {
		requestContext["scheme"] = "https"
		requestContext["tls"] = true
	} else {
		requestContext["scheme"] = "http"
		requestContext["tls"] = false
	}

	// Merge custom context if provided
	if customContext != nil {
		for key, value := range customContext {
			requestContext[key] = value
		}
	}

	vm.Set("request", requestContext)

	for _, manip := range manipulations {
		switch manip.Mode {
		case models.HeaderModeDrop:
			headers.Del(manip.Name)
		case models.HeaderModeReplace:
			headers.Set(manip.Name, manip.Value)
		case models.HeaderModeExpression:
			// Use cached compiled expression for performance
			program, err := p.compileExpression(manip.Expression)
			if err != nil {
				log.Printf("Failed to compile header expression for %s: %v", manip.Name, err)
				continue
			}
			result, err := vm.RunProgram(program)
			if err == nil {
				headers.Set(manip.Name, result.String())
			} else {
				log.Printf("Failed to evaluate header expression for %s: %v", manip.Name, err)
			}
		}
	}
}

// transformBody applies JavaScript transformation to response body
func (p *ProxyHandler) transformBody(bodyBytes []byte, contentType string, script string) ([]byte, error) {
	vm := goja.New()

	// Provide marshalling utilities
	vm.Set("JSON", map[string]interface{}{
		"parse": func(s string) (interface{}, error) {
			var result interface{}
			err := json.Unmarshal([]byte(s), &result)
			return result, err
		},
		"stringify": func(v interface{}) (string, error) {
			bytes, err := json.Marshal(v)
			return string(bytes), err
		},
	})

	// Set body
	bodyStr := string(bodyBytes)
	vm.Set("body", bodyStr)
	vm.Set("contentType", contentType)

	// Execute transformation script
	result, err := vm.RunString(script)
	if err != nil {
		return nil, err
	}

	return []byte(result.String()), nil
}

// translateStatusCode translates status codes based on patterns
func (p *ProxyHandler) translateStatusCode(originalCode int, translations []models.StatusTranslation) int {
	for _, trans := range translations {
		if p.matchesStatusPattern(originalCode, trans.FromPattern) {
			return trans.ToCode
		}
	}
	return originalCode
}

// matchesStatusPattern checks if a status code matches a pattern
func (p *ProxyHandler) matchesStatusPattern(code int, pattern string) bool {
	// Support exact match ("404") and wildcard ("5xx", "2xx")
	if pattern == fmt.Sprintf("%d", code) {
		return true
	}

	if strings.HasSuffix(pattern, "xx") {
		prefix := pattern[:1]
		codeStr := fmt.Sprintf("%d", code)
		if len(codeStr) >= 1 {
			return prefix == codeStr[:1]
		}
	}

	return false
}

// substituteCaptureGroups replaces $1, $2, etc. in the URL with capture group values
// substituteCaptureGroups replaces $1, $2, … placeholders in urlTemplate with
// the corresponding regex capture groups.  captureGroups[0] is the full match.
func substituteCaptureGroups(urlTemplate string, captureGroups []string) string {
	if len(captureGroups) == 0 {
		return urlTemplate
	}

	result := urlTemplate
	for i := 1; i < len(captureGroups); i++ {
		placeholder := fmt.Sprintf("$%d", i)
		result = strings.ReplaceAll(result, placeholder, captureGroups[i])
	}
	return result
}

// rewriteRedirectLocation rewrites redirect Location headers to route back through our proxy
func (p *ProxyHandler) rewriteRedirectLocation(locationHeader, backendBaseURL, originalPath, translatedPath string, endpoint *models.Endpoint, r *http.Request) string {
	// Parse the redirect location URL
	locationURL, err := url.Parse(locationHeader)
	if err != nil {
		// Can't parse, return as-is
		return locationHeader
	}

	// Parse the backend base URL
	backendURL, err := url.Parse(backendBaseURL)
	if err != nil {
		// Can't parse backend URL, return location as-is
		return locationHeader
	}

	// Check if redirect is to the backend (same scheme + host)
	// If it's an external redirect, don't rewrite
	if locationURL.Scheme != "" && locationURL.Host != "" {
		if locationURL.Scheme != backendURL.Scheme || locationURL.Host != backendURL.Host {
			// External redirect, leave as-is
			return locationHeader
		}
	}

	// Get the redirect path
	redirectPath := locationURL.Path

	// Strip backend base path if it exists
	if backendURL.Path != "" && backendURL.Path != "/" {
		if strings.HasPrefix(redirectPath, backendURL.Path) {
			redirectPath = strings.TrimPrefix(redirectPath, backendURL.Path)
			// Ensure it starts with /
			if !strings.HasPrefix(redirectPath, "/") {
				redirectPath = "/" + redirectPath
			}
		}
	}

	// Now reverse-translate the path
	var newPath string

	if strings.HasPrefix(redirectPath, translatedPath) {
		// Simple case: redirect path starts with what we sent to backend
		// Replace the translated prefix with the original prefix
		suffix := strings.TrimPrefix(redirectPath, translatedPath)
		newPath = originalPath + suffix
	} else {
		// Complex case: backend redirected to a different path
		switch endpoint.TranslationMode {
		case models.TranslationModeStrip:
			// We stripped the prefix, so prepend it back
			newPath = endpoint.PathPrefix + redirectPath
		case models.TranslationModeNone:
			// No translation, use as-is
			newPath = redirectPath
		default:
			// For regex/translate, we can't reverse-translate unknown paths
			// Best effort: if the redirect is relative, try to maintain it
			newPath = redirectPath
		}
	}

	// Build the new location URL
	// Preserve query string and fragment
	if locationURL.RawQuery != "" {
		newPath += "?" + locationURL.RawQuery
	}
	if locationURL.Fragment != "" {
		newPath += "#" + locationURL.Fragment
	}

	// If the original location was absolute, return absolute
	// Otherwise return relative
	if locationURL.Scheme != "" && locationURL.Host != "" {
		// Determine the scheme to use for the client redirect
		// Priority:
		// 1. If backend explicitly redirects to HTTPS, preserve that (security upgrade)
		// 2. If backend redirects to HTTP and client used HTTPS, preserve HTTPS
		// 3. Otherwise use the backend's redirect scheme
		scheme := locationURL.Scheme
		if r.TLS != nil && scheme == "http" {
			// Client used HTTPS, don't downgrade to HTTP
			scheme = "https"
		}
		// Note: If client used HTTP but backend redirects to HTTPS, we honor the upgrade
		return scheme + "://" + r.Host + newPath
	}

	// Return relative path
	return newPath
}

// handleWebSocket handles WebSocket connections
func (p *ProxyHandler) handleWebSocket(w http.ResponseWriter, r *http.Request, endpoint *models.Endpoint, translatedPath string, captureGroups []string) {
	// Upgrade client connection
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer clientConn.Close()

	// Connect to backend WebSocket with capture group substitution.
	// resolveBackendURL handles scheme-relative URLs; forWebSocket=true selects ws/wss.
	backendURL := resolveBackendURL(substituteCaptureGroups(endpoint.ProxyConfig.BackendURL, captureGroups), r, true)
	backendURL = strings.Replace(backendURL, "http://", "ws://", 1)
	backendURL = strings.Replace(backendURL, "https://", "wss://", 1)
	backendURL += translatedPath
	if r.URL.RawQuery != "" {
		backendURL += "?" + r.URL.RawQuery
	}

	backendConn, _, err := websocket.DefaultDialer.Dial(backendURL, nil)
	if err != nil {
		clientConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Backend connection failed"))
		return
	}
	defer backendConn.Close()

	// Log the WebSocket upgrade
	connID := fmt.Sprintf("ws-%d", time.Now().UnixNano())
	startTime := time.Now()
	status101 := http.StatusSwitchingProtocols
	if p.logger != nil {
		wsLog := models.RequestLog{
			ID:          connID,
			Timestamp:   startTime.Format(time.RFC3339),
			EndpointID:  endpoint.ID,
			IsWebSocket: true,
		}
		wsLog.ClientRequest.Method = r.Method
		wsLog.ClientRequest.FullURL = r.URL.String()
		wsLog.ClientRequest.Path = r.URL.Path
		wsLog.ClientRequest.SourceIP = r.RemoteAddr
		wsLog.ClientResponse.StatusCode = &status101
		wsLog.ClientResponse.StatusText = "Switching Protocols"
		p.logger.LogRequest(wsLog)
	}

	// Bidirectional forwarding with frame logging
	captureBytes := 1024
	if p.logger != nil {
		captureBytes = p.logger.GetWSCaptureBytes()
	}

	var blocked atomic.Bool
	if p.logger != nil {
		p.logger.RegisterWSConnection(connID,
			func() { clientConn.Close(); backendConn.Close() },
			func(b bool) { blocked.Store(b) },
		)
	}

	errChan := make(chan error, 2)

	// Client -> Backend
	go func() {
		for {
			msgType, msg, err := clientConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if !blocked.Load() {
				if err := backendConn.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
			if p.logger != nil {
				p.logger.AppendWebSocketEvent(connID, makeWSEvent(msgType, msg, models.WSDirectionSend, startTime, captureBytes))
			}
		}
	}()

	// Backend -> Client
	go func() {
		for {
			msgType, msg, err := backendConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if !blocked.Load() {
				if err := clientConn.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
			if p.logger != nil {
				p.logger.AppendWebSocketEvent(connID, makeWSEvent(msgType, msg, models.WSDirectionRecv, startTime, captureBytes))
			}
		}
	}()

	// Drain both goroutines before recording close so no AppendWebSocketEvent races CloseWebSocketConnection.
	relayErr := <-errChan
	clientConn.Close()
	backendConn.Close()
	<-errChan

	if p.logger != nil {
		closeCode, closeReason := extractWSCloseInfo(relayErr)
		p.logger.CloseWebSocketConnection(connID, closeCode, closeReason, relayErr)
	}
}

// resolveBackendURL resolves a possibly scheme-relative backend URL ("//host/path")
// to the correct absolute URL by inferring the scheme from the incoming request.
// This allows overlay endpoints to serve http, https, ws, and wss without requiring
// a separate BackendURL per protocol.
func resolveBackendURL(rawURL string, r *http.Request, forWebSocket bool) string {
	if !strings.HasPrefix(rawURL, "//") {
		return rawURL // already has an explicit scheme
	}
	// Determine scheme from the incoming request's TLS state and URL scheme
	isTLS := r.TLS != nil || r.URL.Scheme == "https"
	var scheme string
	switch {
	case forWebSocket && isTLS:
		scheme = "wss"
	case forWebSocket:
		scheme = "ws"
	case isTLS:
		scheme = "https"
	default:
		scheme = "http"
	}
	return scheme + ":" + rawURL
}

// isWebSocketUpgrade checks if the request is a WebSocket upgrade
func (p *ProxyHandler) isWebSocketUpgrade(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Upgrade")) == "websocket" &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// StartHealthChecks starts health check loops for all proxy endpoints
func (p *ProxyHandler) StartHealthChecks(endpoints []*models.Endpoint) {
	for _, endpoint := range endpoints {
		if endpoint.Type == models.EndpointTypeProxy && endpoint.ProxyConfig != nil && endpoint.ProxyConfig.HealthCheckEnabled {
			go p.healthCheckLoop(endpoint)
		}
	}
}

// healthCheckLoop runs periodic health checks for an endpoint
func (p *ProxyHandler) healthCheckLoop(endpoint *models.Endpoint) {
	cfg := endpoint.ProxyConfig
	interval := time.Duration(cfg.HealthCheckInterval) * time.Second
	if interval == 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		healthy, errMsg := p.performHealthCheck(endpoint)

		p.healthMutex.Lock()
		p.healthStatus[endpoint.ID] = &models.HealthStatus{
			EndpointID:   endpoint.ID,
			Healthy:      healthy,
			LastCheck:    time.Now().Format(time.RFC3339),
			ErrorMessage: errMsg,
		}
		p.healthMutex.Unlock()
	}
}

// performHealthCheck performs a single health check
func (p *ProxyHandler) performHealthCheck(endpoint *models.Endpoint) (bool, string) {
	cfg := endpoint.ProxyConfig
	healthPath := cfg.HealthCheckPath
	if healthPath == "" {
		healthPath = "/"
	}

	healthURL := cfg.BackendURL + healthPath

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()

	// Accept status codes 200-499 (client errors are not backend down)
	healthy := resp.StatusCode >= 200 && resp.StatusCode < 500
	if !healthy {
		return false, fmt.Sprintf("Status code %d", resp.StatusCode)
	}

	return true, ""
}

// GetHealthStatus returns the health status for an endpoint
func (p *ProxyHandler) GetHealthStatus(endpointID string) *models.HealthStatus {
	p.healthMutex.RLock()
	defer p.healthMutex.RUnlock()
	return p.healthStatus[endpointID]
}

// logProxyRequest logs a proxy request with full backend details using new nested structure
// This updates the existing pending log entry with complete response data
func (p *ProxyHandler) logProxyRequest(requestID string, endpoint *models.Endpoint, r *http.Request,
	clientFullURL string, clientReqHeaders map[string][]string, clientReqBody string, clientQueryParams map[string][]string,
	clientStatusCode int, clientRespHeaders map[string][]string, clientRespBody string, clientDelayMs int64, clientRTTMs int64,
	backendFullURL string, backendMethod string, backendPath string, backendQueryParams map[string][]string, backendReqHeaders map[string][]string,
	backendStatusCode int, backendStatusText string, backendRespHeaders map[string][]string, backendRespBody string, backendDelayMs int64, backendRTTMs int64) {
	if p.logger != nil {
		// Create RequestLog with new nested structure
		requestLog := models.RequestLog{
			ID:         requestID,
			Timestamp:  time.Now().Format(time.RFC3339),
			EndpointID: endpoint.ID,
		}

		// Populate client request
		requestLog.ClientRequest.Method = r.Method
		requestLog.ClientRequest.FullURL = clientFullURL
		requestLog.ClientRequest.Path = r.URL.Path
		requestLog.ClientRequest.QueryParams = clientQueryParams
		requestLog.ClientRequest.Headers = clientReqHeaders
		requestLog.ClientRequest.Body = clientReqBody
		requestLog.ClientRequest.Protocol = r.Proto
		requestLog.ClientRequest.SourceIP = r.RemoteAddr
		requestLog.ClientRequest.UserAgent = r.Header.Get("User-Agent")

		// Populate client response
		requestLog.ClientResponse.StatusCode = &clientStatusCode
		requestLog.ClientResponse.StatusText = http.StatusText(clientStatusCode)
		requestLog.ClientResponse.Headers = clientRespHeaders
		requestLog.ClientResponse.Body = clientRespBody
		requestLog.ClientResponse.DelayMs = &clientDelayMs
		requestLog.ClientResponse.RTTMs = &clientRTTMs

		// Populate backend request (pointer struct)
		requestLog.BackendRequest = &struct {
			Method      string              `json:"method"`
			FullURL     string              `json:"full_url"`
			Path        string              `json:"path"`
			QueryParams map[string][]string `json:"query_params,omitempty"`
			Headers     map[string][]string `json:"headers,omitempty"`
			Body        string              `json:"body,omitempty"`
		}{
			Method:      backendMethod,
			FullURL:     backendFullURL,
			Path:        backendPath,
			QueryParams: backendQueryParams,
			Headers:     backendReqHeaders,
			Body:        clientReqBody, // Same as client request body (proxied through)
		}

		// Populate backend response (pointer struct)
		requestLog.BackendResponse = &struct {
			StatusCode *int                `json:"status_code,omitempty"`
			StatusText string              `json:"status_text,omitempty"`
			Headers    map[string][]string `json:"headers,omitempty"`
			Body       string              `json:"body,omitempty"`
			DelayMs    *int64              `json:"delay_ms,omitempty"`
			RTTMs      *int64              `json:"rtt_ms,omitempty"`
		}{
			StatusCode: &backendStatusCode,
			StatusText: backendStatusText,
			Headers:    backendRespHeaders,
			Body:       backendRespBody,
			DelayMs:    &backendDelayMs,
			RTTMs:      &backendRTTMs,
		}

		p.logger.UpdateRequestLog(requestLog)
	}
}

// isSSEResponse reports whether the backend response carries a Server-Sent Events stream.
func isSSEResponse(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	base := strings.TrimSpace(strings.SplitN(ct, ";", 2)[0])
	return strings.EqualFold(base, "text/event-stream")
}

// streamSSEResponse relays a Server-Sent Events stream to the client.
//
// Like WebSocket connections there is no overall timeout — the relay runs until
// either side closes the connection. firstMsgTimeout controls how long we wait
// for the very first event from the backend; if no data arrives within that window
// we can still return a proper HTTP error (headers have not been sent yet).
// If firstMsgTimeout is 0 a 30-second default is used.
//
// Events are parsed line-by-line (blank-line delimited per the SSE spec) and
// emitted to the logger via AppendSSEEvent for live display in the UI.
func (p *ProxyHandler) streamSSEResponse(
	w http.ResponseWriter, r *http.Request, resp *http.Response,
	endpoint *models.Endpoint, requestID string, firstMsgTimeout time.Duration,
	clientFullURL string, clientReqHeaders map[string][]string,
	clientReqBody string, clientQueryParams map[string][]string,
	backendFullURL string, backendReqHeaders map[string][]string,
	backendStartTime time.Time,
) {
	cfg := endpoint.ProxyConfig
	if firstMsgTimeout <= 0 {
		firstMsgTimeout = 30 * time.Second
	}

	// Read the first chunk BEFORE writing headers to the client so we can still
	// return a proper HTTP error if the backend doesn't produce any data in time.
	type readResult struct {
		data []byte
		err  error
	}
	firstCh := make(chan readResult, 1)
	initialBuf := make([]byte, 4096)
	go func() {
		n, err := resp.Body.Read(initialBuf)
		res := readResult{err: err}
		if n > 0 {
			res.data = make([]byte, n)
			copy(res.data, initialBuf[:n])
		}
		firstCh <- res
	}()

	var first readResult
	select {
	case first = <-firstCh:
		// backend produced data (or an immediate error)
	case <-time.After(firstMsgTimeout):
		resp.Body.Close()
		<-firstCh // drain goroutine
		log.Printf("[SSE] First-event timeout after %v (endpoint: %s)", firstMsgTimeout, endpoint.Name)
		http.Error(w, "SSE: first event timeout", http.StatusGatewayTimeout)
		return
	}

	if first.err != nil && len(first.data) == 0 {
		log.Printf("[SSE] Backend closed before first event (endpoint: %s): %v", endpoint.Name, first.err)
		http.Error(w, "SSE: backend closed without data", http.StatusBadGateway)
		return
	}

	// We have the first chunk — write response headers to the client now.
	statusCode := resp.StatusCode
	if !cfg.StatusPassthrough {
		statusCode = p.translateStatusCode(resp.StatusCode, cfg.StatusTranslation)
	}
	for name, values := range resp.Header {
		// Go's http.Client already decoded chunked encoding; don't forward framing headers.
		if strings.EqualFold(name, "Transfer-Encoding") || strings.EqualFold(name, "Content-Length") {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	p.applyHeaderManipulation(w.Header(), cfg.OutboundHeaders, r)
	w.WriteHeader(statusCode)

	// Flush — for socks5StreamWriter this signals streaming mode and writes the
	// HTTP status line + headers to the underlying net.Conn immediately.
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Mark the log entry as an SSE stream now that we are committed to streaming.
	streamOpenedAt := time.Now()
	if p.logger != nil {
		p.logger.MarkSSEOpen(requestID, streamOpenedAt.Format(time.RFC3339))
	}

	// Write first chunk and begin SSE parsing.
	// We use io.MultiReader to prepend the first chunk to the remaining body so
	// the scanner sees a single contiguous stream.
	combinedReader := io.MultiReader(bytes.NewReader(first.data), resp.Body)

	// Pipe the stream to both the client writer and the SSE parser via a pipe.
	// The scanner reads from pipeReader; we write each line to both the client
	// and the pipe writer.  To keep it simple we write to both inline in the
	// scanner loop (no goroutine needed — we own both sides).
	//
	// We use bufio.Scanner for line-by-line parsing.
	scanner := bufio.NewScanner(combinedReader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // up to 1 MB per line

	// Accumulated SSE event fields
	var (
		eventType    string
		eventID      string
		dataLines    []string
		rawLines     []string
		retryMs      int
		eventCounter int
	)

	var streamErr string

	for scanner.Scan() {
		line := scanner.Text()

		// Forward the line to the client.
		if _, writeErr := fmt.Fprintf(w, "%s\n", line); writeErr != nil {
			log.Printf("[SSE] Client write error (endpoint: %s): %v", endpoint.Name, writeErr)
			streamErr = writeErr.Error()
			break
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Parse the SSE line.
		if line == "" {
			// Blank line — dispatch event if we have any data fields.
			if len(dataLines) > 0 || eventType != "" {
				data := strings.Join(dataLines, "\n")
				rawText := strings.Join(rawLines, "\n")
				et := eventType
				if et == "" {
					et = "message"
				}
				offsetMs := time.Since(streamOpenedAt).Milliseconds()
				eventCounter++
				sseEvent := models.SSEEvent{
					ID:        fmt.Sprintf("sse-%s-%d", requestID, eventCounter),
					Timestamp: time.Now().Format(time.RFC3339Nano),
					OffsetMs:  offsetMs,
					EventType: et,
					EventID:   eventID,
					Data:      data,
					DataSize:  len(data),
					RawText:   rawText,
					Retry:     retryMs,
				}
				if p.logger != nil {
					p.logger.AppendSSEEvent(requestID, sseEvent)
				}
			}
			// Reset accumulated fields for next event.
			eventType = ""
			eventID = ""
			dataLines = dataLines[:0]
			rawLines = rawLines[:0]
			retryMs = 0
			continue
		}

		rawLines = append(rawLines, line)

		if strings.HasPrefix(line, ":") {
			// Comment line — skip but forward (already written above).
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			// Field with no colon — treat as field name with empty value.
			continue
		}
		field := line[:colonIdx]
		value := line[colonIdx+1:]
		// Strip a single leading space if present.
		if strings.HasPrefix(value, " ") {
			value = value[1:]
		}

		switch field {
		case "data":
			dataLines = append(dataLines, value)
		case "event":
			eventType = value
		case "id":
			eventID = value
		case "retry":
			if n, err := strconv.Atoi(value); err == nil {
				retryMs = n
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		log.Printf("[SSE] Backend stream ended (endpoint: %s): %v", endpoint.Name, err)
		if streamErr == "" {
			streamErr = err.Error()
		}
	}

	// Dispatch any trailing event (stream closed without a final blank line).
	if len(dataLines) > 0 && p.logger != nil {
		data := strings.Join(dataLines, "\n")
		rawText := strings.Join(rawLines, "\n")
		et := eventType
		if et == "" {
			et = "message"
		}
		eventCounter++
		sseEvent := models.SSEEvent{
			ID:        fmt.Sprintf("sse-%s-%d", requestID, eventCounter),
			Timestamp: time.Now().Format(time.RFC3339Nano),
			OffsetMs:  time.Since(streamOpenedAt).Milliseconds(),
			EventType: et,
			EventID:   eventID,
			Data:      data,
			DataSize:  len(data),
			RawText:   rawText,
			Retry:     retryMs,
		}
		p.logger.AppendSSEEvent(requestID, sseEvent)
	}

	if p.logger != nil {
		p.logger.CloseSSEStream(requestID, streamErr)
	}

	p.logSSECompletion(requestID, endpoint, r, clientFullURL, clientReqHeaders, clientReqBody, clientQueryParams, statusCode, backendFullURL, backendReqHeaders)
}

// logSSECompletion updates the pending log entry once an SSE stream has ended.
func (p *ProxyHandler) logSSECompletion(
	requestID string, endpoint *models.Endpoint, r *http.Request,
	clientFullURL string, clientReqHeaders map[string][]string,
	clientReqBody string, clientQueryParams map[string][]string,
	statusCode int, backendFullURL string, backendReqHeaders map[string][]string,
) {
	if p.logger == nil {
		return
	}
	requestLog := models.RequestLog{
		ID:         requestID,
		Timestamp:  time.Now().Format(time.RFC3339),
		EndpointID: endpoint.ID,
	}
	requestLog.ClientRequest.Method = r.Method
	requestLog.ClientRequest.FullURL = clientFullURL
	requestLog.ClientRequest.Path = r.URL.Path
	requestLog.ClientRequest.QueryParams = clientQueryParams
	requestLog.ClientRequest.Headers = clientReqHeaders
	requestLog.ClientRequest.Body = clientReqBody
	requestLog.ClientRequest.Protocol = r.Proto
	requestLog.ClientRequest.SourceIP = r.RemoteAddr
	requestLog.ClientRequest.UserAgent = r.Header.Get("User-Agent")
	requestLog.ClientResponse.StatusCode = &statusCode
	requestLog.ClientResponse.StatusText = http.StatusText(statusCode)
	requestLog.ClientResponse.Body = "[SSE stream]"
	requestLog.BackendRequest = &struct {
		Method      string              `json:"method"`
		FullURL     string              `json:"full_url"`
		Path        string              `json:"path"`
		QueryParams map[string][]string `json:"query_params,omitempty"`
		Headers     map[string][]string `json:"headers,omitempty"`
		Body        string              `json:"body,omitempty"`
	}{
		Method:  r.Method,
		FullURL: backendFullURL,
		Path:    r.URL.Path,
		Headers: backendReqHeaders,
	}
	p.logger.UpdateRequestLog(requestLog)
}

// logPendingRequest logs a request immediately when received (before waiting for response)
func (p *ProxyHandler) logPendingRequest(requestID string, endpoint *models.Endpoint, r *http.Request,
	clientFullURL string, clientReqHeaders map[string][]string, clientReqBody string, clientQueryParams map[string][]string) {
	if p.logger != nil {
		// Create RequestLog with pending status
		requestLog := models.RequestLog{
			ID:         requestID,
			Timestamp:  time.Now().Format(time.RFC3339),
			EndpointID: endpoint.ID,
		}

		// Populate client request (we have this data immediately)
		requestLog.ClientRequest.Method = r.Method
		requestLog.ClientRequest.FullURL = clientFullURL
		requestLog.ClientRequest.Path = r.URL.Path
		requestLog.ClientRequest.QueryParams = clientQueryParams
		requestLog.ClientRequest.Headers = clientReqHeaders
		requestLog.ClientRequest.Body = clientReqBody
		requestLog.ClientRequest.Protocol = r.Proto
		requestLog.ClientRequest.SourceIP = r.RemoteAddr
		requestLog.ClientRequest.UserAgent = r.Header.Get("User-Agent")

		// Client response is empty (pending)
		requestLog.ClientResponse.StatusCode = nil
		requestLog.ClientResponse.StatusText = ""
		requestLog.ClientResponse.Headers = nil
		requestLog.ClientResponse.Body = ""
		requestLog.ClientResponse.DelayMs = nil
		requestLog.ClientResponse.RTTMs = nil

		// Backend data is nil (pending)
		requestLog.BackendRequest = nil
		requestLog.BackendResponse = nil

		p.logger.LogRequest(requestLog)
	}
}
