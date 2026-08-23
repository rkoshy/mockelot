package server

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"mockelot/models"
)

type RequestLogger interface {
	LogRequest(log models.RequestLog)
	UpdateRequestLog(log models.RequestLog)
	AppendWebSocketEvent(connectionID string, event models.WebSocketEvent)
	CloseWebSocketConnection(connectionID string, code int, reason string, relayErr error)
	GetWSCaptureBytes() int
	// RegisterWSConnection stores control callbacks so the frontend can terminate
	// or block a live WebSocket relay without touching the relay goroutines directly.
	RegisterWSConnection(connectionID string, terminate func(), setBlocked func(bool))
	// AppendSSEEvent appends a parsed SSE event to the stream log, updating running counters.
	AppendSSEEvent(connectionID string, event models.SSEEvent)
	// CloseSSEStream records the end of an SSE stream, optionally with an error message.
	CloseSSEStream(connectionID string, closeError string)
	// MarkSSEOpen stamps the SSE stream as open (IsSSE=true, SSEOpenedAt, SSEIsOpen=true).
	MarkSSEOpen(connectionID string, openedAt string)
}

type ScriptErrorLogger interface {
	LogScriptError(responseID, path, method, errorMsg string)
}

// CORSPreflightMatch contains information about a matched CORS preflight request
// This allows us to track which response triggered the CORS handling for logging purposes
type CORSPreflightMatch struct {
	ShouldHandle bool                    // Whether global CORS should handle this preflight
	ResponseID   string                  // ID of the response that triggered CORS handling
	Response     *models.MethodResponse  // The matching response (for UseGlobalCORS check)
	Group        *models.ResponseGroup   // The group containing the response (if any)
}

// OverlaySimMode constants for fault injection on overlay endpoints.
const (
	OverlaySimNormal   = "normal"    // pass through (default)
	OverlaySimTimeout  = "timeout"   // simulate connection timeout → 504
	OverlaySimDNSError = "dns_error" // simulate DNS resolution failure → 502
	OverlaySimOther    = "other"     // return a custom HTTP status code
)

type ResponseHandler struct {
	config              *models.AppConfig
	configMutex         sync.RWMutex
	requestLogger       RequestLogger
	scriptErrorLogger   ScriptErrorLogger
	corsProcessor       *CORSProcessor
	proxyHandler        *ProxyHandler
	containerHandler    *ContainerHandler
	fileServerHandler   *FileServerHandler
	overlayHandler      *OverlayHandler
	regexCache          map[string]*regexp.Regexp // Cache for compiled regexes
	regexCacheMutex     sync.RWMutex              // Mutex for regex cache
	logRequestMatching  bool                      // Enable verbose request matching logs
	overlaySimModes     *sync.Map                 // endpointID → OverlaySimMode constant; shared with HTTPServer
	proxySimModes       *sync.Map                 // endpointID → OverlaySimConfig; shared with HTTPServer
}

func NewResponseHandler(config *models.AppConfig, logger RequestLogger, scriptErrorLogger ScriptErrorLogger, proxyHandler *ProxyHandler, containerHandler *ContainerHandler, logRequestMatching bool, dnsResolver *DNSResolver, overlaySimModes *sync.Map, proxySimModes *sync.Map) *ResponseHandler {
	overlayHandler := NewOverlayHandler(proxyHandler, dnsResolver)
	return &ResponseHandler{
		config:             config,
		requestLogger:      logger,
		scriptErrorLogger:  scriptErrorLogger,
		corsProcessor:      NewCORSProcessor(&config.CORS),
		proxyHandler:       proxyHandler,
		containerHandler:   containerHandler,
		fileServerHandler:  NewFileServerHandler(proxyHandler),
		overlayHandler:     overlayHandler,
		regexCache:         make(map[string]*regexp.Regexp),
		logRequestMatching: logRequestMatching,
		overlaySimModes:    overlaySimModes,
		proxySimModes:      proxySimModes,
	}
}

// applyTranslation computes the translated path for an endpoint in "translate" mode.
// If the endpoint has TranslationRules, they are tried in order and the first matching
// rule's replacement is returned (nginx `rewrite ... break` semantics).
// Falls back to the legacy single TranslatePattern/TranslateReplace pair if no rules defined.
// Returns the original requestPath unchanged if nothing matches.
func (h *ResponseHandler) applyTranslation(endpoint *models.Endpoint, requestPath string) string {
	if endpoint == nil {
		return requestPath
	}
	// Multi-rule path: first matching rule wins
	if len(endpoint.TranslationRules) > 0 {
		for _, rule := range endpoint.TranslationRules {
			if rule.Pattern == "" {
				continue
			}
			re, err := h.compileRegex(rule.Pattern)
			if err != nil {
				log.Printf("Invalid translation rule pattern %q in endpoint %s: %v", rule.Pattern, endpoint.Name, err)
				continue
			}
			if re.MatchString(requestPath) {
				return re.ReplaceAllString(requestPath, rule.Replace)
			}
		}
		// No rule matched — return path unchanged
		return requestPath
	}

	// Legacy single pattern/replace
	if endpoint.TranslatePattern != "" {
		re, err := h.compileRegex(endpoint.TranslatePattern)
		if err != nil {
			log.Printf("Invalid regex pattern in endpoint %s: %v", endpoint.Name, err)
			return requestPath
		}
		return re.ReplaceAllString(requestPath, endpoint.TranslateReplace)
	}

	return requestPath
}

// compileRegex compiles a regex pattern and caches it
func (h *ResponseHandler) compileRegex(pattern string) (*regexp.Regexp, error) {
	// Check cache first (read lock)
	h.regexCacheMutex.RLock()
	if re, exists := h.regexCache[pattern]; exists {
		h.regexCacheMutex.RUnlock()
		return re, nil
	}
	h.regexCacheMutex.RUnlock()

	// Compile regex (outside lock to avoid blocking readers)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	// Store in cache (write lock)
	h.regexCacheMutex.Lock()
	h.regexCache[pattern] = re
	h.regexCacheMutex.Unlock()

	return re, nil
}

// InvalidateRegexCache clears the regex cache (call when config changes)
func (h *ResponseHandler) InvalidateRegexCache() {
	h.regexCacheMutex.Lock()
	h.regexCache = make(map[string]*regexp.Regexp)
	h.regexCacheMutex.Unlock()
}

// WarmRegexCache pre-compiles all regexes from the current endpoint configuration
// so that the first incoming request does not trigger regex compilation (and the
// associated memory allocation / GC pressure that can cause signal-handler races
// on Linux with WebKit2GTK's JSC).
func (h *ResponseHandler) WarmRegexCache() {
	h.configMutex.RLock()
	endpoints := h.config.Endpoints
	h.configMutex.RUnlock()

	compiled := 0
	for i := range endpoints {
		ep := &endpoints[i]

		// Path prefix (may be a regex)
		if strings.HasPrefix(ep.PathPrefix, "^") {
			if _, err := h.compileRegex(ep.PathPrefix); err == nil {
				compiled++
			}
		}

		// Legacy single translate pattern
		if ep.TranslatePattern != "" {
			if _, err := h.compileRegex(ep.TranslatePattern); err == nil {
				compiled++
			}
		}

		// Multi-rule translation patterns
		for _, rule := range ep.TranslationRules {
			if rule.Pattern != "" {
				if _, err := h.compileRegex(rule.Pattern); err == nil {
					compiled++
				}
			}
		}

		// Domain filter patterns (used in matchesDomain)
		if ep.DomainFilter != nil {
			for _, pat := range ep.DomainFilter.Patterns {
				if pat != "" {
					if _, err := h.compileRegex(pat); err == nil {
						compiled++
					}
				}
			}
		}
	}

	// Also pre-compile domain takeover patterns (used in matchesDomain "all" mode)
	h.configMutex.RLock()
	dt := h.config.DomainTakeover
	h.configMutex.RUnlock()
	if dt != nil {
		for _, d := range dt.Domains {
			if d.Pattern != "" {
				if _, err := h.compileRegex(d.Pattern); err == nil {
					compiled++
				}
			}
		}
	}

	log.Printf("[Server] Regex cache warmed: %d patterns pre-compiled", compiled)
}

// canMockEndpointHandleRequest checks if a mock endpoint has a response that can handle the request
// This checks both path pattern and method, but not validation (validation happens later)
func (h *ResponseHandler) canMockEndpointHandleRequest(endpoint *models.Endpoint, translatedPath string, method string) bool {
	if h.logRequestMatching {
		log.Printf("[MATCH] canMockEndpointHandleRequest: endpoint=%s path=%s method=%s", endpoint.Name, translatedPath, method)
	}

	// For OPTIONS requests, check if we should handle as CORS preflight
	if method == "OPTIONS" {
		corsMatch := h.canHandleCORSPreflightForEndpoint(endpoint, translatedPath)
		if h.logRequestMatching {
			log.Printf("[CORS] CORS preflight check for OPTIONS: endpoint=%s path=%s shouldHandle=%v responseID=%s",
				endpoint.Name, translatedPath, corsMatch.ShouldHandle, corsMatch.ResponseID)
		}
		if corsMatch.ShouldHandle {
			return true
		}
	}

	for _, item := range endpoint.Items {
		if item.Type == "response" && item.Response != nil {
			resp := item.Response
			if !resp.IsEnabled() {
				continue
			}

			// Check if method matches
			methodMatches := false
			for _, m := range resp.Methods {
				if m == method {
					methodMatches = true
					break
				}
			}
			if !methodMatches {
				continue
			}

			// Check if path matches
			matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
			if matchResult.Matches {
				return true
			}
		} else if item.Type == "group" && item.Group != nil {
			group := item.Group
			if !group.IsEnabled() {
				continue
			}

			for i := range group.Responses {
				resp := &group.Responses[i]
				if !resp.IsEnabled() {
					continue
				}

				// Check if method matches
				methodMatches := false
				for _, m := range resp.Methods {
					if m == method {
						methodMatches = true
						break
					}
				}
				if !methodMatches {
					continue
				}

				// Check if path matches
				matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
				if matchResult.Matches {
					return true
				}
			}
		}
	}
	return false
}

// canHandleCORSPreflightForEndpoint checks if a mock endpoint should handle
// a CORS preflight (OPTIONS) request for the given path.
// Returns CORSPreflightMatch with ShouldHandle=true if:
// 1. Global CORS is enabled
// 2. There is at least one response entry matching the path (any method)
// 3. That response has UseGlobalCORS enabled (or nil = default enabled)
// 4. There is no explicit OPTIONS handler (explicit handlers take precedence)
func (h *ResponseHandler) canHandleCORSPreflightForEndpoint(endpoint *models.Endpoint, translatedPath string) CORSPreflightMatch {
	noMatch := CORSPreflightMatch{ShouldHandle: false}

	// Check if global CORS is enabled
	if !h.config.CORS.Enabled {
		if h.logRequestMatching {
			log.Printf("[CORS] Global CORS is disabled")
		}
		return noMatch
	}

	hasExplicitOptionsHandler := false
	var matchingResponse *models.MethodResponse
	var matchingGroup *models.ResponseGroup

	for _, item := range endpoint.Items {
		switch item.Type {
		case "response":
			if item.Response == nil {
				continue
			}
			resp := item.Response

			// Skip disabled responses
			if !resp.IsEnabled() {
				continue
			}

			// Check if path matches
			matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
			if !matchResult.Matches {
				continue
			}

			if h.logRequestMatching {
				log.Printf("[CORS] Path matches response: id=%s pattern=%s", resp.ID, resp.PathPattern)
			}

			// Check for explicit OPTIONS handler
			for _, m := range resp.Methods {
				if m == "OPTIONS" {
					hasExplicitOptionsHandler = true
					if h.logRequestMatching {
						log.Printf("[CORS] Found explicit OPTIONS handler: id=%s", resp.ID)
					}
					break
				}
			}

			// Check UseGlobalCORS (nil defaults to true)
			if resp.UseGlobalCORS == nil || *resp.UseGlobalCORS {
				if matchingResponse == nil {
					matchingResponse = resp
					matchingGroup = nil // No group for standalone responses
					if h.logRequestMatching {
						log.Printf("[CORS] Found CORS-enabled response: id=%s useGlobalCORS=%v", resp.ID, resp.UseGlobalCORS)
					}
				}
			}

		case "group":
			if item.Group == nil {
				continue
			}
			group := item.Group

			// Skip disabled groups
			if !group.IsEnabled() {
				continue
			}

			for i := range group.Responses {
				resp := &group.Responses[i]

				// Skip disabled responses
				if !resp.IsEnabled() {
					continue
				}

				// Check if path matches
				matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
				if !matchResult.Matches {
					continue
				}

				if h.logRequestMatching {
					log.Printf("[CORS] Path matches group response: group=%s id=%s pattern=%s", group.Name, resp.ID, resp.PathPattern)
				}

				// Check for explicit OPTIONS handler
				for _, m := range resp.Methods {
					if m == "OPTIONS" {
						hasExplicitOptionsHandler = true
						if h.logRequestMatching {
							log.Printf("[CORS] Found explicit OPTIONS handler in group: group=%s id=%s", group.Name, resp.ID)
						}
						break
					}
				}

				// Response overrides group for UseGlobalCORS
				useGlobalCORS := true
				if resp.UseGlobalCORS != nil {
					useGlobalCORS = *resp.UseGlobalCORS
				} else if group.UseGlobalCORS != nil {
					useGlobalCORS = *group.UseGlobalCORS
				}

				if useGlobalCORS && matchingResponse == nil {
					matchingResponse = resp
					matchingGroup = group
					if h.logRequestMatching {
						log.Printf("[CORS] Found CORS-enabled group response: group=%s id=%s", group.Name, resp.ID)
					}
				}
			}
		}
	}

	// Handle preflight if path matches with CORS and no explicit OPTIONS handler
	shouldHandle := matchingResponse != nil && !hasExplicitOptionsHandler

	if h.logRequestMatching {
		log.Printf("[CORS] Final decision: shouldHandle=%v hasMatch=%v hasExplicitOptions=%v",
			shouldHandle, matchingResponse != nil, hasExplicitOptionsHandler)
	}

	if !shouldHandle {
		return noMatch
	}

	return CORSPreflightMatch{
		ShouldHandle: true,
		ResponseID:   matchingResponse.ID,
		Response:     matchingResponse,
		Group:        matchingGroup,
	}
}

// CheckOverlaySimMode returns the active OverlaySimConfig for the request's matched
// overlay endpoint (and true) when a non-normal simulation is configured.
// This is called from the SOCKS5 WebSocket handlers, which bypass HandleRequest
// entirely and therefore would otherwise skip the simulation-mode check.
func (h *ResponseHandler) CheckOverlaySimMode(r *http.Request) (models.OverlaySimConfig, bool) {
	endpointID := h.FindEndpointID(r)
	if !strings.HasPrefix(endpointID, "system-overlay-") || h.overlaySimModes == nil {
		return models.OverlaySimConfig{}, false
	}
	raw, ok := h.overlaySimModes.Load(endpointID)
	if !ok {
		return models.OverlaySimConfig{}, false
	}
	cfg := raw.(models.OverlaySimConfig)
	if cfg.Mode == "" || cfg.Mode == OverlaySimNormal {
		return models.OverlaySimConfig{}, false
	}
	return cfg, true
}

// EndpointMatch contains the result of matching a request to an endpoint,
// including the translated path and any regex capture groups.
// Used by the SOCKS5 WebSocket handler to route connections through proxy endpoints.
type EndpointMatch struct {
	Endpoint      *models.Endpoint
	TranslatedPath string
	CaptureGroups []string
}

// FindEndpointMatch returns the first enabled endpoint that matches r's domain
// and path prefix, along with the translated path and capture groups.
// Returns nil if no endpoint matches.
// Used by the SOCKS5 WebSocket handler to route and log connections correctly.
func (h *ResponseHandler) FindEndpointMatch(r *http.Request) *EndpointMatch {
	h.configMutex.RLock()
	defer h.configMutex.RUnlock()

	requestPath := r.URL.Path
	requestDomain := extractDomain(r)

	for i := range h.config.Endpoints {
		endpoint := &h.config.Endpoints[i]
		if !endpoint.IsEnabled() {
			continue
		}
		if !h.matchesDomain(endpoint, requestDomain) {
			continue
		}

		var prefixMatches bool
		var captureGroups []string
		if strings.HasPrefix(endpoint.PathPrefix, "^") {
			re, err := h.compileRegex(endpoint.PathPrefix)
			if err == nil {
				matches := re.FindStringSubmatch(requestPath)
				if matches != nil {
					prefixMatches = true
					captureGroups = matches
				}
			}
		} else if endpoint.PathPrefix == "/" {
			prefixMatches = true
		} else {
			prefixMatches = requestPath == endpoint.PathPrefix ||
				strings.HasPrefix(requestPath, endpoint.PathPrefix+"/")
		}

		if !prefixMatches {
			continue
		}

		// Compute translated path (same logic as HandleRequest)
		var translatedPath string
		switch endpoint.TranslationMode {
		case models.TranslationModeNone:
			translatedPath = requestPath
		case models.TranslationModeStrip:
			if strings.HasPrefix(endpoint.PathPrefix, "^") {
				re, err := h.compileRegex(endpoint.PathPrefix)
				if err != nil {
					translatedPath = requestPath
				} else {
					matched := re.FindString(requestPath)
					if matched != "" {
						translatedPath = strings.TrimPrefix(requestPath, matched)
					} else {
						translatedPath = requestPath
					}
				}
			} else {
				translatedPath = strings.TrimPrefix(requestPath, endpoint.PathPrefix)
			}
			if !strings.HasPrefix(translatedPath, "/") {
				translatedPath = "/" + translatedPath
			}
		case models.TranslationModeTranslate:
			translatedPath = h.applyTranslation(endpoint, requestPath)
		default:
			translatedPath = requestPath
		}

		return &EndpointMatch{
			Endpoint:       endpoint,
			TranslatedPath: translatedPath,
			CaptureGroups:  captureGroups,
		}
	}
	return nil
}

// FindEndpointID returns the ID of the first enabled endpoint that matches r's
// domain and path prefix, or "system-socks5-proxy" if no endpoint matches.
// Used by the SOCKS5 WebSocket handler to log frames under the correct endpoint tab.
func (h *ResponseHandler) FindEndpointID(r *http.Request) string {
	if match := h.FindEndpointMatch(r); match != nil {
		return match.Endpoint.ID
	}
	return "system-socks5-proxy"
}

func (h *ResponseHandler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[HANDLER] Panic handling %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}()

	// Read request body
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	h.configMutex.RLock()
	requestPath := r.URL.Path
	requestDomain := extractDomain(r) // Extract domain from Host header

	// Debug logging for request matching
	if h.logRequestMatching {
		log.Printf("[MATCH] === New Request ===")
		log.Printf("[MATCH] Method: %s, Path: %s, Domain: %s", r.Method, requestPath, requestDomain)
	}

	// Step 1: Find matching endpoint by prefix and apply path translation
	var matchedEndpoint *models.Endpoint
	var translatedPath string
	var items []models.ResponseItem
	var captureGroups []string // For regex capture groups (used by proxy endpoints)

	// Try to match an endpoint
	if len(h.config.Endpoints) > 0 {
		for i := range h.config.Endpoints {
			endpoint := &h.config.Endpoints[i]
			if !endpoint.IsEnabled() {
				if h.logRequestMatching {
					log.Printf("[MATCH] Skipping disabled endpoint: %s", endpoint.Name)
				}
				continue
			}

			if h.logRequestMatching {
				log.Printf("[MATCH] Checking endpoint: %s (prefix: %s, type: %s)", endpoint.Name, endpoint.PathPrefix, endpoint.Type)
			}

			// Check domain filter first (before path matching)
			if !h.matchesDomain(endpoint, requestDomain) {
				if h.logRequestMatching {
					log.Printf("[MATCH] Domain filter rejected: endpoint=%s domain=%s", endpoint.Name, requestDomain)
				}
				continue
			}

			// Check if PathPrefix is a regex (starts with ^) or plain prefix
			var prefixMatches bool
			var currentCaptureGroups []string
			if strings.HasPrefix(endpoint.PathPrefix, "^") {
				// Regex matching with capture groups
				re, err := h.compileRegex(endpoint.PathPrefix)
				if err != nil {
					log.Printf("Invalid regex pattern: %s (%v)", endpoint.PathPrefix, err)
					prefixMatches = false
				} else {
					matches := re.FindStringSubmatch(requestPath)
					if matches != nil {
						prefixMatches = true
						currentCaptureGroups = matches // Store all capture groups (matches[0] is full match, matches[1]... are groups)
					} else {
						prefixMatches = false
					}
				}
			} else {
				// Exact or prefix matching (with trailing slash)
				// This prevents /test2 from matching prefix /test
				// Special case: if PathPrefix is "/", match all paths
				if endpoint.PathPrefix == "/" {
					prefixMatches = strings.HasPrefix(requestPath, "/")
				} else {
					prefixMatches = requestPath == endpoint.PathPrefix || strings.HasPrefix(requestPath, endpoint.PathPrefix+"/")
				}
			}

			if !prefixMatches {
				continue
			}

			// Compute translated path for this endpoint
			var currentTranslatedPath string
			switch endpoint.TranslationMode {
			case models.TranslationModeNone:
				currentTranslatedPath = requestPath
			case models.TranslationModeStrip:
				// Check if PathPrefix is a regex pattern
				if strings.HasPrefix(endpoint.PathPrefix, "^") {
					// Regex strip: find what matched and remove it
					re, err := h.compileRegex(endpoint.PathPrefix)
					if err != nil {
						log.Printf("Invalid regex pattern for strip: %s (%v)", endpoint.PathPrefix, err)
						currentTranslatedPath = requestPath
					} else {
						matched := re.FindString(requestPath)
						if matched != "" {
							currentTranslatedPath = strings.TrimPrefix(requestPath, matched)
						} else {
							currentTranslatedPath = requestPath
						}
					}
				} else {
					// Plain string strip
					currentTranslatedPath = strings.TrimPrefix(requestPath, endpoint.PathPrefix)
				}
				// Ensure path starts with /
				if !strings.HasPrefix(currentTranslatedPath, "/") {
					currentTranslatedPath = "/" + currentTranslatedPath
				}
			case models.TranslationModeTranslate:
				currentTranslatedPath = h.applyTranslation(endpoint, requestPath)
			default:
				currentTranslatedPath = requestPath
			}

			// For mock endpoints, verify there's a response that can handle this request (path + method)
			// If not, continue to the next endpoint instead of committing to this one
			if endpoint.Type == models.EndpointTypeMock {
				if !h.canMockEndpointHandleRequest(endpoint, currentTranslatedPath, r.Method) {
					// This mock endpoint can't handle the request - try next endpoint
					if h.logRequestMatching {
						log.Printf("[MATCH] Mock endpoint can't handle request: endpoint=%s path=%s method=%s", endpoint.Name, currentTranslatedPath, r.Method)
					}
					continue
				}
				if h.logRequestMatching {
					log.Printf("[MATCH] Mock endpoint CAN handle request: endpoint=%s path=%s method=%s", endpoint.Name, currentTranslatedPath, r.Method)
				}
			}

			// This endpoint can handle the request
			matchedEndpoint = endpoint
			translatedPath = currentTranslatedPath
			captureGroups = currentCaptureGroups
			items = endpoint.Items
			if h.logRequestMatching {
				log.Printf("[MATCH] ✓ Matched endpoint: %s (type=%s, translated_path=%s)", endpoint.Name, endpoint.Type, translatedPath)
			}
			break // First match wins
		}

		// If no endpoint matched, check for overlay mode before returning 404
		if matchedEndpoint == nil {
			if h.logRequestMatching {
				log.Printf("[MATCH] No endpoint matched, checking overlay mode for domain: %s", requestDomain)
			}
			// Check if overlay mode should be used for this domain
			domainTakeover := h.config.DomainTakeover
			h.configMutex.RUnlock()

			if h.overlayHandler != nil && h.overlayHandler.shouldUseOverlay(requestDomain, domainTakeover) {
				// Use overlay mode - proxy to real server
				if h.logRequestMatching {
					log.Printf("[MATCH] Using overlay mode for domain: %s", requestDomain)
				}
				if err := h.overlayHandler.handleOverlay(w, r, requestDomain); err != nil {
					log.Printf("Overlay mode error: %v", err)
					http.Error(w, "Overlay mode failed", http.StatusBadGateway)
				}
				return
			}

			// No endpoint and no overlay - return 404
			if h.logRequestMatching {
				log.Printf("[MATCH] ✗ No match found, returning 404")
			}
			http.Error(w, "No endpoint configured for this path", http.StatusNotFound)
			return
		}

		// Dispatch based on endpoint type
		h.configMutex.RUnlock()

		// Fault injection for overlay endpoints.
		if strings.HasPrefix(matchedEndpoint.ID, "system-overlay-") && h.overlaySimModes != nil {
			if raw, ok := h.overlaySimModes.Load(matchedEndpoint.ID); ok {
				cfg := raw.(models.OverlaySimConfig)
				switch cfg.Mode {
				case OverlaySimTimeout:
					secs := cfg.TimeoutSecs
					if secs <= 0 {
						secs = 30
					}
					time.Sleep(time.Duration(secs) * time.Second)
					http.Error(w, "504 Gateway Timeout — simulated by Mockelot", http.StatusGatewayTimeout)
					return
				case OverlaySimDNSError:
					http.Error(w, "502 Bad Gateway — DNS resolution failed (simulated by Mockelot)", http.StatusBadGateway)
					return
				case OverlaySimOther:
					code := cfg.StatusCode
					if code < 100 || code > 599 {
						code = http.StatusBadGateway
					}
					http.Error(w, http.StatusText(code)+" — simulated by Mockelot", code)
					return
				}
			}
		}

		// Fault injection for proxy endpoints.
		if matchedEndpoint.Type == models.EndpointTypeProxy && h.proxySimModes != nil {
			if raw, ok := h.proxySimModes.Load(matchedEndpoint.ID); ok {
				cfg := raw.(models.OverlaySimConfig)
				switch cfg.Mode {
				case OverlaySimTimeout:
					secs := cfg.TimeoutSecs
					if secs <= 0 {
						secs = 30
					}
					time.Sleep(time.Duration(secs) * time.Second)
					http.Error(w, "504 Gateway Timeout — simulated by Mockelot", http.StatusGatewayTimeout)
					return
				case OverlaySimDNSError:
					http.Error(w, "502 Bad Gateway — DNS resolution failed (simulated by Mockelot)", http.StatusBadGateway)
					return
				case OverlaySimOther:
					code := cfg.StatusCode
					if code < 100 || code > 599 {
						code = http.StatusBadGateway
					}
					http.Error(w, http.StatusText(code)+" — simulated by Mockelot", code)
					return
				}
			}
		}

		switch matchedEndpoint.Type {
		case models.EndpointTypeMock:
			h.handleMockRequest(w, r, matchedEndpoint, translatedPath, bodyBytes)
		case models.EndpointTypeProxy:
			h.handleProxyRequest(w, r, matchedEndpoint, translatedPath, captureGroups)
		case models.EndpointTypeContainer:
			h.handleContainerRequest(w, r, matchedEndpoint, translatedPath)
		case models.EndpointTypeFileServer:
			h.handleFileServerRequest(w, r, matchedEndpoint, translatedPath)
		default:
			http.Error(w, "Unknown endpoint type", http.StatusInternalServerError)
		}
		return
	} else {
		// Fallback: No endpoints configured, use legacy Items
		translatedPath = requestPath
		items = h.config.Items
	}

	// Determine endpoint ID for logging (empty string if legacy fallback)
	endpointID := ""
	if matchedEndpoint != nil {
		endpointID = matchedEndpoint.ID
	}

	// Check if this is a CORS preflight that should be handled globally
	if r.Method == "OPTIONS" {
		corsMatch := h.shouldHandleCORSPreflightForItems(r, translatedPath, items)
		if corsMatch.ShouldHandle {
			h.configMutex.RUnlock()
			h.handleCORSPreflightWithLogging(w, r, endpointID, corsMatch.ResponseID, bodyBytes)
			return
		}
	}

	// Step 2: Find matching response within the endpoint's items using translated path
	var matchedResponse *models.MethodResponse
	var matchedGroup *models.ResponseGroup
	var pathParams map[string]string
	var extractedVars map[string]interface{}

	// Iterate through items to preserve group information
	for _, item := range items {
		if item.Type == "response" && item.Response != nil {
			resp := item.Response

			// Skip disabled responses
			if !resp.IsEnabled() {
				continue
			}

			// Check if method matches
			methodMatches := false
			for _, method := range resp.Methods {
				if method == r.Method {
					methodMatches = true
					break
				}
			}

			// Check if path matches and extract path parameters (using translated path)
			if methodMatches {
				matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
				if matchResult.Matches {
					// Build initial context for validation (without vars yet)
					tempContext := BuildRequestContext(r, bodyBytes, matchResult.PathParams)

					// Run request body validation if configured
					validationResult := ValidateRequest(resp.RequestValidation, string(bodyBytes), tempContext)
					if !validationResult.Valid {
						// Validation failed - log and continue to next response
						log.Printf("Validation failed for %s %s (translated: %s): %s", r.Method, r.URL.Path, translatedPath, validationResult.Error)

						// Log validation failure (no HTTP response sent)
						requestLog := buildRequestLog(r, bodyBytes, endpointID)
						requestLog.ResponseID = resp.ID // Track which response's validation failed
						requestLog.ValidationFailed = true
						requestLog.ClientResponse.StatusCode = nil // No HTTP response
						requestLog.ClientResponse.Body = validationResult.Error
						h.requestLogger.LogRequest(requestLog)

						continue
					}

					// Validation passed - use this response
					matchedResponse = resp
					matchedGroup = nil // No group for standalone responses
					pathParams = matchResult.PathParams
					extractedVars = validationResult.Vars
					break
				}
			}
		} else if item.Type == "group" && item.Group != nil {
			group := item.Group
			// Skip disabled groups
			if !group.IsEnabled() {
				continue
			}

			// Check responses within the group
			for i := range group.Responses {
				resp := &group.Responses[i]
				// Skip disabled responses
				if !resp.IsEnabled() {
					continue
				}

				// Check if method matches
				methodMatches := false
				for _, method := range resp.Methods {
					if method == r.Method {
						methodMatches = true
						break
					}
				}

				// Check if path matches and extract path parameters (using translated path)
				if methodMatches {
					matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
					if matchResult.Matches {
						// Build initial context for validation (without vars yet)
						tempContext := BuildRequestContext(r, bodyBytes, matchResult.PathParams)

						// Run request body validation if configured
						validationResult := ValidateRequest(resp.RequestValidation, string(bodyBytes), tempContext)
						if !validationResult.Valid {
							// Validation failed - log and continue to next response
							log.Printf("Validation failed for %s %s (translated: %s): %s", r.Method, r.URL.Path, translatedPath, validationResult.Error)

							// Log validation failure (no HTTP response sent)
							requestLog := buildRequestLog(r, bodyBytes, endpointID)
							requestLog.ResponseID = resp.ID // Track which response's validation failed
							requestLog.ValidationFailed = true
							requestLog.ClientResponse.StatusCode = nil // No HTTP response
							requestLog.ClientResponse.Body = validationResult.Error
							h.requestLogger.LogRequest(requestLog)

							continue
						}

						// Validation passed - use this response
						matchedResponse = resp
						matchedGroup = group
						pathParams = matchResult.PathParams
						extractedVars = validationResult.Vars
						break
					}
				}
			}

			if matchedResponse != nil {
				break
			}
		}

		if matchedResponse != nil {
			break
		}
	}

	// Fallback to legacy responses if no items matched and no endpoints configured
	if matchedResponse == nil && len(items) == 0 && len(h.config.Endpoints) == 0 {
		for i := range h.config.Responses {
			resp := &h.config.Responses[i]
			// Skip disabled responses
			if !resp.IsEnabled() {
				continue
			}

			// Check if method matches
			methodMatches := false
			for _, method := range resp.Methods {
				if method == r.Method {
					methodMatches = true
					break
				}
			}

			// Check if path matches and extract path parameters (using translated path)
			if methodMatches {
				matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
				if matchResult.Matches {
					// Build initial context for validation (without vars yet)
					tempContext := BuildRequestContext(r, bodyBytes, matchResult.PathParams)

					// Run request body validation if configured
					validationResult := ValidateRequest(resp.RequestValidation, string(bodyBytes), tempContext)
					if !validationResult.Valid {
						// Validation failed - log and continue to next response
						log.Printf("Validation failed for %s %s (translated: %s): %s", r.Method, r.URL.Path, translatedPath, validationResult.Error)

						// Log validation failure (no HTTP response sent)
						requestLog := buildRequestLog(r, bodyBytes, endpointID)
						requestLog.ResponseID = resp.ID // Track which response's validation failed
						requestLog.ValidationFailed = true
						requestLog.ClientResponse.StatusCode = nil // No HTTP response
						requestLog.ClientResponse.Body = validationResult.Error
						h.requestLogger.LogRequest(requestLog)

						continue
					}

					// Validation passed - use this response
					matchedResponse = resp
					matchedGroup = nil
					pathParams = matchResult.PathParams
					extractedVars = validationResult.Vars
					break
				}
			}
		}
	}
	h.configMutex.RUnlock()

	// Deep copy headers to avoid reference issues
	headersCopy := make(map[string][]string, len(r.Header))
	for key, values := range r.Header {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		headersCopy[key] = valuesCopy
	}

	// Deep copy query params to avoid reference issues
	queryParamsCopy := make(map[string][]string, len(r.URL.Query()))
	for key, values := range r.URL.Query() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		queryParamsCopy[key] = valuesCopy
	}

	if matchedResponse == nil {
		http.Error(w, "No matching response configuration", http.StatusNotFound)
		return
	}

	// Apply CORS headers if needed
	if h.shouldApplyCORS(matchedResponse, matchedGroup, r) {
		corsHeaders := h.corsProcessor.ProcessCORS(r)
		for name, value := range corsHeaders {
			w.Header().Set(name, value)
		}
	}

	// Capture request start time
	startTime := time.Now()

	// Process response based on mode
	finalBody, finalHeaders, finalStatus, finalDelay, responseErr := h.processResponse(
		matchedResponse, r, bodyBytes, pathParams, extractedVars,
	)

	// Check for response generation error
	if responseErr != nil {
		// Log response failure (no HTTP response sent)
		requestLog := buildRequestLog(r, bodyBytes, endpointID)
		requestLog.ResponseID = matchedResponse.ID // Track which response's generation failed
		requestLog.ResponseFailed = true
		requestLog.ClientResponse.StatusCode = nil // No HTTP response
		requestLog.ClientResponse.Body = responseErr.Error()
		h.requestLogger.LogRequest(requestLog)

		// TODO: Jump to Rejections endpoint (future implementation)
		http.Error(w, "Response generation failed", http.StatusInternalServerError)
		return
	}

	// Implement response delay
	if finalDelay > 0 {
		time.Sleep(time.Duration(finalDelay) * time.Millisecond)
	}

	// Set headers
	for name, value := range finalHeaders {
		w.Header().Set(name, value)
	}

	// Apply Content-Security-Policy if configured on the matched response
	applyCSP(w.Header(), matchedResponse.CSP)

	// Capture time before first byte (right before WriteHeader)
	firstByteTime := time.Now()

	// Set status code
	w.WriteHeader(finalStatus)

	// Write response body
	w.Write([]byte(finalBody))

	// Capture completion time
	completionTime := time.Now()

	// Calculate timing metrics
	delayMs := firstByteTime.Sub(startTime).Milliseconds()
	rttMs := completionTime.Sub(startTime).Milliseconds()

	// Capture final response headers for logging
	finalRespHeaders := make(map[string][]string, len(w.Header()))
	for name, values := range w.Header() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		finalRespHeaders[name] = valuesCopy
	}

	// Build full client URL (scheme://host:port/path?query)
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	fullURL := scheme + "://" + r.Host + r.URL.RequestURI()

	// Get status text
	statusText := http.StatusText(finalStatus)

	// Log the request with full response details using new nested structure
	requestLog := models.RequestLog{
		ID:         uuid.New().String(),
		Timestamp:  time.Now().Format(time.RFC3339),
		EndpointID: endpointID,
		ResponseID: matchedResponse.ID,
	}

	// Populate client request
	requestLog.ClientRequest.Method = r.Method
	requestLog.ClientRequest.FullURL = fullURL
	requestLog.ClientRequest.Path = r.URL.Path
	requestLog.ClientRequest.QueryParams = queryParamsCopy
	requestLog.ClientRequest.Headers = headersCopy
	requestLog.ClientRequest.Body = string(bodyBytes)
	requestLog.ClientRequest.Protocol = r.Proto
	requestLog.ClientRequest.SourceIP = r.RemoteAddr
	requestLog.ClientRequest.UserAgent = r.UserAgent()

	// Populate client response
	requestLog.ClientResponse.StatusCode = &finalStatus
	requestLog.ClientResponse.StatusText = statusText
	requestLog.ClientResponse.Headers = finalRespHeaders
	requestLog.ClientResponse.Body = finalBody
	requestLog.ClientResponse.DelayMs = &delayMs
	requestLog.ClientResponse.RTTMs = &rttMs

	// Backend fields are nil for mock endpoints (no backend proxy)

	// Send log to logger
	h.requestLogger.LogRequest(requestLog)
}

// handleMockRequest handles mock endpoint requests with script-based responses
func (h *ResponseHandler) handleMockRequest(w http.ResponseWriter, r *http.Request, endpoint *models.Endpoint, translatedPath string, bodyBytes []byte) {
	h.configMutex.RLock()
	items := endpoint.Items

	// Check if this is a CORS preflight that should be handled globally
	if r.Method == "OPTIONS" {
		corsMatch := h.shouldHandleCORSPreflightForItems(r, translatedPath, items)
		if corsMatch.ShouldHandle {
			h.configMutex.RUnlock()
			h.handleCORSPreflightWithLogging(w, r, endpoint.ID, corsMatch.ResponseID, bodyBytes)
			return
		}
	}

	// Find matching response within the endpoint's items using translated path
	var matchedResponse *models.MethodResponse
	var matchedGroup *models.ResponseGroup
	var pathParams map[string]string
	var extractedVars map[string]interface{}

	// Iterate through items to preserve group information
	for _, item := range items {
		if item.Type == "response" && item.Response != nil {
			resp := item.Response

			// Skip disabled responses
			if !resp.IsEnabled() {
				continue
			}

			// Check if method matches
			methodMatches := false
			for _, method := range resp.Methods {
				if method == r.Method {
					methodMatches = true
					break
				}
			}

			// Check if path matches and extract path parameters (using translated path)
			if methodMatches {
				matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
				if matchResult.Matches {
					// Build initial context for validation (without vars yet)
					tempContext := BuildRequestContext(r, bodyBytes, matchResult.PathParams)

					// Run request body validation if configured
					validationResult := ValidateRequest(resp.RequestValidation, string(bodyBytes), tempContext)
					if !validationResult.Valid {
						// Validation failed - log and continue to next response
						log.Printf("Validation failed for %s %s (translated: %s): %s", r.Method, r.URL.Path, translatedPath, validationResult.Error)

						// Log validation failure (no HTTP response sent)
						requestLog := buildRequestLog(r, bodyBytes, endpoint.ID)
						requestLog.ResponseID = resp.ID // Track which response's validation failed
						requestLog.ValidationFailed = true
						requestLog.ClientResponse.StatusCode = nil // No HTTP response
						requestLog.ClientResponse.Body = validationResult.Error
						h.requestLogger.LogRequest(requestLog)

						continue
					}

					// Validation passed - use this response
					matchedResponse = resp
					matchedGroup = nil // No group for standalone responses
					pathParams = matchResult.PathParams
					extractedVars = validationResult.Vars
					break
				}
			}
		} else if item.Type == "group" && item.Group != nil {
			group := item.Group
			// Skip disabled groups
			if !group.IsEnabled() {
				continue
			}

			// Check responses within the group
			for i := range group.Responses {
				resp := &group.Responses[i]
				// Skip disabled responses
				if !resp.IsEnabled() {
					continue
				}

				// Check if method matches
				methodMatches := false
				for _, method := range resp.Methods {
					if method == r.Method {
						methodMatches = true
						break
					}
				}

				// Check if path matches and extract path parameters (using translated path)
				if methodMatches {
					matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
					if matchResult.Matches {
						// Build initial context for validation (without vars yet)
						tempContext := BuildRequestContext(r, bodyBytes, matchResult.PathParams)

						// Run request body validation if configured
						validationResult := ValidateRequest(resp.RequestValidation, string(bodyBytes), tempContext)
						if !validationResult.Valid {
							// Validation failed - log and continue to next response
							log.Printf("Validation failed for %s %s (translated: %s): %s", r.Method, r.URL.Path, translatedPath, validationResult.Error)

							// Log validation failure (no HTTP response sent)
							requestLog := buildRequestLog(r, bodyBytes, endpoint.ID)
							requestLog.ResponseID = resp.ID // Track which response's validation failed
							requestLog.ValidationFailed = true
							requestLog.ClientResponse.StatusCode = nil // No HTTP response
							requestLog.ClientResponse.Body = validationResult.Error
							h.requestLogger.LogRequest(requestLog)

							continue
						}

						// Validation passed - use this response
						matchedResponse = resp
						matchedGroup = group
						pathParams = matchResult.PathParams
						extractedVars = validationResult.Vars
						break
					}
				}
			}

			if matchedResponse != nil {
				break
			}
		}

		if matchedResponse != nil {
			break
		}
	}
	h.configMutex.RUnlock()

	// Deep copy headers to avoid reference issues
	headersCopy := make(map[string][]string, len(r.Header))
	for key, values := range r.Header {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		headersCopy[key] = valuesCopy
	}

	// Deep copy query params to avoid reference issues
	queryParamsCopy := make(map[string][]string, len(r.URL.Query()))
	for key, values := range r.URL.Query() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		queryParamsCopy[key] = valuesCopy
	}

	if matchedResponse == nil {
		http.Error(w, "No matching response configuration", http.StatusNotFound)
		return
	}

	// Apply CORS headers if needed
	if h.shouldApplyCORS(matchedResponse, matchedGroup, r) {
		corsHeaders := h.corsProcessor.ProcessCORS(r)
		for name, value := range corsHeaders {
			w.Header().Set(name, value)
		}
	}

	// Capture request start time
	startTime := time.Now()

	// Process response based on mode
	finalBody, finalHeaders, finalStatus, finalDelay, responseErr := h.processResponse(
		matchedResponse, r, bodyBytes, pathParams, extractedVars,
	)

	// Check for response generation error
	if responseErr != nil {
		// Log response failure (no HTTP response sent)
		requestLog := buildRequestLog(r, bodyBytes, endpoint.ID)
		requestLog.ResponseID = matchedResponse.ID // Track which response's generation failed
		requestLog.ResponseFailed = true
		requestLog.ClientResponse.StatusCode = nil // No HTTP response
		requestLog.ClientResponse.Body = responseErr.Error()
		h.requestLogger.LogRequest(requestLog)

		// TODO: Jump to Rejections endpoint (future implementation)
		http.Error(w, "Response generation failed", http.StatusInternalServerError)
		return
	}

	// Implement response delay
	if finalDelay > 0 {
		time.Sleep(time.Duration(finalDelay) * time.Millisecond)
	}

	// Set headers
	for name, value := range finalHeaders {
		w.Header().Set(name, value)
	}

	// Apply Content-Security-Policy if configured on the matched response
	applyCSP(w.Header(), matchedResponse.CSP)

	// Capture time before first byte (right before WriteHeader)
	firstByteTime := time.Now()

	// Set status code
	w.WriteHeader(finalStatus)

	// Write response body
	w.Write([]byte(finalBody))

	// Capture completion time
	completionTime := time.Now()

	// Calculate timing metrics
	delayMs := firstByteTime.Sub(startTime).Milliseconds()
	rttMs := completionTime.Sub(startTime).Milliseconds()

	// Capture final response headers for logging
	finalRespHeaders := make(map[string][]string, len(w.Header()))
	for name, values := range w.Header() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		finalRespHeaders[name] = valuesCopy
	}

	// Build full client URL (scheme://host:port/path?query)
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	fullURL := scheme + "://" + r.Host + r.URL.RequestURI()

	// Get status text
	statusText := http.StatusText(finalStatus)

	// Log the request with full response details using new nested structure
	requestLog := models.RequestLog{
		ID:         uuid.New().String(),
		Timestamp:  time.Now().Format(time.RFC3339),
		EndpointID: endpoint.ID,
		ResponseID: matchedResponse.ID,
	}

	// Populate client request
	requestLog.ClientRequest.Method = r.Method
	requestLog.ClientRequest.FullURL = fullURL
	requestLog.ClientRequest.Path = r.URL.Path
	requestLog.ClientRequest.QueryParams = queryParamsCopy
	requestLog.ClientRequest.Headers = headersCopy
	requestLog.ClientRequest.Body = string(bodyBytes)
	requestLog.ClientRequest.Protocol = r.Proto
	requestLog.ClientRequest.SourceIP = r.RemoteAddr
	requestLog.ClientRequest.UserAgent = r.UserAgent()

	// Populate client response
	requestLog.ClientResponse.StatusCode = &finalStatus
	requestLog.ClientResponse.StatusText = statusText
	requestLog.ClientResponse.Headers = finalRespHeaders
	requestLog.ClientResponse.Body = finalBody
	requestLog.ClientResponse.DelayMs = &delayMs
	requestLog.ClientResponse.RTTMs = &rttMs

	// Backend fields are nil for mock endpoints (no backend proxy)

	// Send log to logger
	h.requestLogger.LogRequest(requestLog)
}

// handleProxyRequest handles proxy endpoint requests
func (h *ResponseHandler) handleProxyRequest(w http.ResponseWriter, r *http.Request, endpoint *models.Endpoint, translatedPath string, captureGroups []string) {
	if h.proxyHandler == nil || endpoint.ProxyConfig == nil {
		http.Error(w, "Proxy configuration missing", http.StatusInternalServerError)
		return
	}

	// Delegate to proxy handler
	h.proxyHandler.ServeHTTP(w, r, endpoint, translatedPath, captureGroups)
}

// handleFileServerRequest handles file server endpoint requests
func (h *ResponseHandler) handleFileServerRequest(w http.ResponseWriter, r *http.Request, endpoint *models.Endpoint, translatedPath string) {
	if h.fileServerHandler == nil || endpoint.FileServerConfig == nil {
		http.Error(w, "File server configuration missing", http.StatusInternalServerError)
		return
	}
	h.fileServerHandler.ServeHTTP(w, r, endpoint, translatedPath, h)
}

// handleContainerRequest handles container endpoint requests
func (h *ResponseHandler) handleContainerRequest(w http.ResponseWriter, r *http.Request, endpoint *models.Endpoint, translatedPath string) {
	if h.containerHandler == nil || endpoint.ContainerConfig == nil {
		http.Error(w, "Container configuration missing", http.StatusInternalServerError)
		return
	}

	if endpoint.ContainerConfig.ContainerID == "" {
		http.Error(w, "Container not running", http.StatusServiceUnavailable)
		return
	}

	// Delegate to container handler
	h.containerHandler.ServeHTTP(w, r, endpoint, translatedPath)
}

// buildRequestLog creates a RequestLog with common fields populated
func buildRequestLog(r *http.Request, bodyBytes []byte, endpointID string) models.RequestLog {
	// Deep copy headers
	headersCopy := make(map[string][]string, len(r.Header))
	for key, values := range r.Header {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		headersCopy[key] = valuesCopy
	}

	// Deep copy query params
	queryParamsCopy := make(map[string][]string, len(r.URL.Query()))
	for key, values := range r.URL.Query() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		queryParamsCopy[key] = valuesCopy
	}

	// Build full client URL (scheme://host:port/path?query)
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	fullURL := scheme + "://" + r.Host + r.URL.RequestURI()

	// Create base log
	requestLog := models.RequestLog{
		ID:         uuid.New().String(),
		Timestamp:  time.Now().Format(time.RFC3339),
		EndpointID: endpointID,
	}

	// Populate client request
	requestLog.ClientRequest.Method = r.Method
	requestLog.ClientRequest.FullURL = fullURL
	requestLog.ClientRequest.Path = r.URL.Path
	requestLog.ClientRequest.QueryParams = queryParamsCopy
	requestLog.ClientRequest.Headers = headersCopy
	requestLog.ClientRequest.Body = string(bodyBytes)
	requestLog.ClientRequest.Protocol = r.Proto
	requestLog.ClientRequest.SourceIP = r.RemoteAddr
	requestLog.ClientRequest.UserAgent = r.UserAgent()

	return requestLog
}

// processResponse processes the response based on the response mode
func (h *ResponseHandler) processResponse(
	resp *models.MethodResponse,
	r *http.Request,
	bodyBytes []byte,
	pathParams map[string]string,
	extractedVars map[string]interface{},
) (body string, headers map[string]string, status int, delay int, err error) {
	// Default values from the response configuration
	body = resp.Body
	headers = resp.Headers
	status = resp.StatusCode
	delay = resp.ResponseDelay

	// Ensure headers is not nil
	if headers == nil {
		headers = make(map[string]string)
	}

	// Determine response mode (default to static)
	responseMode := resp.ResponseMode
	if responseMode == "" {
		responseMode = models.ResponseModeStatic
	}

	switch responseMode {
	case models.ResponseModeTemplate:
		// Build request context with extracted vars
		reqContext := BuildRequestContext(r, bodyBytes, pathParams)
		reqContext.Vars = extractedVars

		// Process body as template
		processedBody, templateErr := ProcessTemplate(resp.Body, reqContext)
		if templateErr != nil {
			log.Printf("Template processing error: %v", templateErr)
			// Return error for response failure tracking
			err = templateErr
			return
		}
		body = processedBody

		// Also process headers as templates
		processedHeaders, headerErr := ProcessTemplateHeaders(resp.Headers, reqContext)
		if headerErr != nil {
			log.Printf("Template header processing error: %v", headerErr)
			// Return error for response failure tracking
			err = headerErr
			return
		}
		headers = processedHeaders

	case models.ResponseModeScript:
		// Build request context with extracted vars
		reqContext := BuildRequestContext(r, bodyBytes, pathParams)
		reqContext.Vars = extractedVars

		// Execute script
		scriptResp, scriptErr := ProcessScript(resp.ScriptBody, reqContext, resp)
		if scriptErr != nil {
			log.Printf("Script execution error: %v", scriptErr)
			// Log error to frontend
			if h.scriptErrorLogger != nil && resp.ID != "" {
				h.scriptErrorLogger.LogScriptError(resp.ID, r.URL.Path, r.Method, scriptErr.Error())
			}
			// Return error for response failure tracking
			err = scriptErr
			return
		}
		body = scriptResp.Body
		headers = scriptResp.Headers
		status = scriptResp.Status
		delay = scriptResp.Delay

	default:
		// Static mode - use values as-is (already set above)
	}

	return
}

// shouldHandleCORSPreflight checks if global CORS should handle an OPTIONS request (legacy, for backward compatibility)
func (h *ResponseHandler) shouldHandleCORSPreflight(r *http.Request) bool {
	// Check if global CORS is enabled
	if !h.config.CORS.Enabled {
		return false
	}

	// Check if there's an explicit OPTIONS handler for this path
	allResponses := h.config.GetAllResponses()
	for i := range allResponses {
		resp := &allResponses[i]
		if !resp.IsEnabled() {
			continue
		}

		// Check if this response handles OPTIONS
		for _, method := range resp.Methods {
			if method == "OPTIONS" {
				// Check if path matches
				matchResult := matchPathPatternWithParams(resp.PathPattern, r.URL.Path)
				if matchResult.Matches {
					// There's an explicit OPTIONS handler, don't use global CORS
					return false
				}
			}
		}
	}

	// No explicit OPTIONS handler, use global CORS
	return true
}

// shouldHandleCORSPreflightForItems checks if global CORS should handle an OPTIONS request for specific items
// Returns CORSPreflightMatch with info about the matching response for logging
func (h *ResponseHandler) shouldHandleCORSPreflightForItems(r *http.Request, translatedPath string, items []models.ResponseItem) CORSPreflightMatch {
	noMatch := CORSPreflightMatch{ShouldHandle: false}

	// Check if global CORS is enabled
	if !h.config.CORS.Enabled {
		if h.logRequestMatching {
			log.Printf("[CORS] Global CORS is disabled (shouldHandleCORSPreflightForItems)")
		}
		return noMatch
	}

	var matchingResponse *models.MethodResponse
	var matchingGroup *models.ResponseGroup
	hasExplicitOptionsHandler := false

	// Check items for explicit OPTIONS handlers and find matching CORS-enabled responses
	for _, item := range items {
		if item.Type == "response" && item.Response != nil {
			resp := item.Response
			if !resp.IsEnabled() {
				continue
			}

			// Check if path matches (using translated path)
			matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
			if !matchResult.Matches {
				continue
			}

			// Check if this response handles OPTIONS explicitly
			for _, method := range resp.Methods {
				if method == "OPTIONS" {
					// There's an explicit OPTIONS handler, don't use global CORS
					if h.logRequestMatching {
						log.Printf("[CORS] Found explicit OPTIONS handler: id=%s pattern=%s", resp.ID, resp.PathPattern)
					}
					hasExplicitOptionsHandler = true
					break
				}
			}

			// Track first matching CORS-enabled response
			if matchingResponse == nil && (resp.UseGlobalCORS == nil || *resp.UseGlobalCORS) {
				matchingResponse = resp
				matchingGroup = nil
				if h.logRequestMatching {
					log.Printf("[CORS] Found CORS-enabled response: id=%s pattern=%s", resp.ID, resp.PathPattern)
				}
			}
		} else if item.Type == "group" && item.Group != nil {
			group := item.Group
			if !group.IsEnabled() {
				continue
			}

			// Check responses within the group
			for i := range group.Responses {
				resp := &group.Responses[i]
				if !resp.IsEnabled() {
					continue
				}

				// Check if path matches (using translated path)
				matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
				if !matchResult.Matches {
					continue
				}

				// Check if this response handles OPTIONS explicitly
				for _, method := range resp.Methods {
					if method == "OPTIONS" {
						if h.logRequestMatching {
							log.Printf("[CORS] Found explicit OPTIONS handler in group: group=%s id=%s", group.Name, resp.ID)
						}
						hasExplicitOptionsHandler = true
						break
					}
				}

				// Track first matching CORS-enabled response
				// Response overrides group for UseGlobalCORS
				useGlobalCORS := true
				if resp.UseGlobalCORS != nil {
					useGlobalCORS = *resp.UseGlobalCORS
				} else if group.UseGlobalCORS != nil {
					useGlobalCORS = *group.UseGlobalCORS
				}

				if matchingResponse == nil && useGlobalCORS {
					matchingResponse = resp
					matchingGroup = group
					if h.logRequestMatching {
						log.Printf("[CORS] Found CORS-enabled group response: group=%s id=%s", group.Name, resp.ID)
					}
				}
			}
		}
	}

	// Handle preflight if there's a matching CORS-enabled response and no explicit OPTIONS handler
	shouldHandle := matchingResponse != nil && !hasExplicitOptionsHandler

	if h.logRequestMatching {
		log.Printf("[CORS] shouldHandleCORSPreflightForItems result: shouldHandle=%v hasMatch=%v hasExplicitOptions=%v",
			shouldHandle, matchingResponse != nil, hasExplicitOptionsHandler)
	}

	if !shouldHandle {
		return noMatch
	}

	return CORSPreflightMatch{
		ShouldHandle: true,
		ResponseID:   matchingResponse.ID,
		Response:     matchingResponse,
		Group:        matchingGroup,
	}
}

// handleCORSPreflight handles a CORS preflight request (legacy, no logging)
func (h *ResponseHandler) handleCORSPreflight(w http.ResponseWriter, r *http.Request) {
	// Process CORS headers (includes Content-Length: 0 from default config)
	corsHeaders := h.corsProcessor.ProcessCORS(r)
	for name, value := range corsHeaders {
		w.Header().Set(name, value)
	}

	// Compatibility: if Content-Length not set by CORS config, add it
	// This ensures HTTP/1.1 clients know the response is complete
	if w.Header().Get("Content-Length") == "" {
		w.Header().Set("Content-Length", "0")
	}

	// Set status code (default to 204 if not specified)
	status := h.config.CORS.OptionsDefaultStatus
	if status == 0 {
		status = http.StatusNoContent // 204
	}

	w.WriteHeader(status)
}

// handleCORSPreflightWithLogging handles a CORS preflight request and logs it to the traffic log
func (h *ResponseHandler) handleCORSPreflightWithLogging(w http.ResponseWriter, r *http.Request, endpointID string, responseID string, bodyBytes []byte) {
	startTime := time.Now()

	// Process CORS headers
	h.configMutex.RLock()
	corsHeaders := h.corsProcessor.ProcessCORS(r)
	status := h.config.CORS.OptionsDefaultStatus
	if status == 0 {
		status = http.StatusNoContent // 204
	}
	h.configMutex.RUnlock()

	// Set headers (includes Content-Length: 0 from default config)
	for name, value := range corsHeaders {
		w.Header().Set(name, value)
	}

	// Compatibility: if Content-Length not set by CORS config, add it
	// This ensures HTTP/1.1 clients know the response is complete
	if w.Header().Get("Content-Length") == "" {
		w.Header().Set("Content-Length", "0")
	}

	// Capture time before first byte
	firstByteTime := time.Now()

	// Write response
	w.WriteHeader(status)

	// Capture completion time
	completionTime := time.Now()

	// Calculate timing
	delayMs := firstByteTime.Sub(startTime).Milliseconds()
	rttMs := completionTime.Sub(startTime).Milliseconds()

	// Capture response headers for logging
	respHeaders := make(map[string][]string, len(w.Header()))
	for name, values := range w.Header() {
		valuesCopy := make([]string, len(values))
		copy(valuesCopy, values)
		respHeaders[name] = valuesCopy
	}

	// Build and log request
	requestLog := buildRequestLog(r, bodyBytes, endpointID)
	requestLog.ResponseID = responseID
	requestLog.ClientResponse.StatusCode = &status
	requestLog.ClientResponse.StatusText = http.StatusText(status)
	requestLog.ClientResponse.Headers = respHeaders
	requestLog.ClientResponse.Body = ""
	requestLog.ClientResponse.DelayMs = &delayMs
	requestLog.ClientResponse.RTTMs = &rttMs

	h.requestLogger.LogRequest(requestLog)

	if h.logRequestMatching {
		log.Printf("[CORS] Logged CORS preflight: endpoint=%s response=%s status=%d", endpointID, responseID, status)
	}
}

// shouldApplyCORS determines if CORS headers should be applied to a response
func (h *ResponseHandler) shouldApplyCORS(response *models.MethodResponse, group *models.ResponseGroup, r *http.Request) bool {
	// If global CORS is not enabled, return false
	if !h.config.CORS.Enabled {
		return false
	}

	// If response explicitly handles OPTIONS, don't apply global CORS
	if response != nil {
		for _, method := range response.Methods {
			if method == "OPTIONS" {
				return false
			}
		}
	}

	// Check per-entry override
	if response != nil && response.UseGlobalCORS != nil {
		return *response.UseGlobalCORS
	}

	// Check per-group override
	if group != nil && group.UseGlobalCORS != nil {
		return *group.UseGlobalCORS
	}

	// Default: use global CORS
	return true
}

// findGroupForResponse finds the group that contains the given response
func (h *ResponseHandler) findGroupForResponse(response *models.MethodResponse) *models.ResponseGroup {
	if response == nil {
		return nil
	}

	// Search through items to find the group containing this response
	for _, item := range h.config.Items {
		if item.Type == "group" && item.Group != nil {
			for _, groupResp := range item.Group.Responses {
				if groupResp.ID == response.ID {
					return item.Group
				}
			}
		}
	}

	return nil
}

// extractDomain extracts the domain name from the request's Host header
// Removes port if present (e.g., "example.com:8080" -> "example.com")
func extractDomain(r *http.Request) string {
	host := r.Host
	// Remove port if present
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}
	return host
}

// matchesDomain checks if the request domain matches the endpoint's domain filter
func (h *ResponseHandler) matchesDomain(endpoint *models.Endpoint, domain string) bool {
	// If no domain filter, match any domain
	if endpoint.DomainFilter == nil {
		return true
	}

	// Get domain takeover configuration from config
	h.configMutex.RLock()
	domainTakeover := h.config.DomainTakeover
	h.configMutex.RUnlock()

	switch endpoint.DomainFilter.Mode {
	case models.DomainFilterModeAny:
		// Match any domain
		return true

	case models.DomainFilterModeAll:
		// Match if domain is in any enabled takeover pattern
		if domainTakeover == nil {
			return false
		}
		for _, domainConfig := range domainTakeover.Domains {
			if !domainConfig.Enabled {
				continue
			}
			// Compile and check regex pattern
			re, err := h.compileRegex(domainConfig.Pattern)
			if err != nil {
				log.Printf("Invalid domain pattern %s: %v", domainConfig.Pattern, err)
				continue
			}
			if re.MatchString(domain) {
				return true
			}
		}
		return false

	case models.DomainFilterModeSpecific:
		// Match if domain matches any selected pattern
		for _, pattern := range endpoint.DomainFilter.Patterns {
			re, err := h.compileRegex(pattern)
			if err != nil {
				log.Printf("Invalid domain pattern %s: %v", pattern, err)
				continue
			}
			if re.MatchString(domain) {
				return true
			}
		}
		return false

	default:
		// Unknown mode, default to match
		return true
	}
}