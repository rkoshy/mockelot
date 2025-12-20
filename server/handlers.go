package server

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"mockelot/models"
)

type RequestLogger interface {
	LogRequest(log models.RequestLog)
	UpdateRequestLog(log models.RequestLog)
}

type ScriptErrorLogger interface {
	LogScriptError(responseID, path, method, errorMsg string)
}

type ResponseHandler struct {
	config            *models.AppConfig
	configMutex       sync.RWMutex
	requestLogger     RequestLogger
	scriptErrorLogger ScriptErrorLogger
	corsProcessor     *CORSProcessor
	proxyHandler      *ProxyHandler
	containerHandler  *ContainerHandler
	overlayHandler    *OverlayHandler
	regexCache        map[string]*regexp.Regexp // Cache for compiled regexes
	regexCacheMutex   sync.RWMutex              // Mutex for regex cache
}

func NewResponseHandler(config *models.AppConfig, logger RequestLogger, scriptErrorLogger ScriptErrorLogger, proxyHandler *ProxyHandler, containerHandler *ContainerHandler) *ResponseHandler {
	overlayHandler := NewOverlayHandler(proxyHandler)
	return &ResponseHandler{
		config:            config,
		requestLogger:     logger,
		scriptErrorLogger: scriptErrorLogger,
		corsProcessor:     NewCORSProcessor(&config.CORS),
		proxyHandler:      proxyHandler,
		containerHandler:  containerHandler,
		overlayHandler:    overlayHandler,
		regexCache:        make(map[string]*regexp.Regexp),
	}
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

func (h *ResponseHandler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	// Read request body
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	h.configMutex.RLock()
	requestPath := r.URL.Path
	requestDomain := extractDomain(r) // Extract domain from Host header

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
				continue
			}

			// Check domain filter first (before path matching)
			if !h.matchesDomain(endpoint, requestDomain) {
				continue
			}

			// Check if PathPrefix is a regex (starts with ^) or plain prefix
			var prefixMatches bool
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
						captureGroups = matches // Store all capture groups (matches[0] is full match, matches[1]... are groups)
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

			if prefixMatches {
				matchedEndpoint = endpoint

				// Apply path translation based on endpoint mode
				switch endpoint.TranslationMode {
				case models.TranslationModeNone:
					translatedPath = requestPath
				case models.TranslationModeStrip:
					// Check if PathPrefix is a regex pattern
					if strings.HasPrefix(endpoint.PathPrefix, "^") {
						// Regex strip: find what matched and remove it
						re, err := h.compileRegex(endpoint.PathPrefix)
						if err != nil {
							log.Printf("Invalid regex pattern for strip: %s (%v)", endpoint.PathPrefix, err)
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
						// Plain string strip
						translatedPath = strings.TrimPrefix(requestPath, endpoint.PathPrefix)
					}
					// Ensure path starts with /
					if !strings.HasPrefix(translatedPath, "/") {
						translatedPath = "/" + translatedPath
					}
				case models.TranslationModeTranslate:
					if endpoint.TranslatePattern != "" {
						re, err := h.compileRegex(endpoint.TranslatePattern)
						if err != nil {
							log.Printf("Invalid regex pattern in endpoint %s: %v", endpoint.Name, err)
							translatedPath = requestPath
						} else {
							translatedPath = re.ReplaceAllString(requestPath, endpoint.TranslateReplace)
						}
					} else {
						translatedPath = requestPath
					}
				default:
					translatedPath = requestPath
				}

				items = endpoint.Items
				break // First match wins
			}
		}

		// If no endpoint matched, check for overlay mode before returning 404
		if matchedEndpoint == nil {
			// Check if overlay mode should be used for this domain
			domainTakeover := h.config.DomainTakeover
			h.configMutex.RUnlock()

			if h.overlayHandler != nil && h.overlayHandler.shouldUseOverlay(requestDomain, domainTakeover) {
				// Use overlay mode - proxy to real server
				if err := h.overlayHandler.handleOverlay(w, r, requestDomain); err != nil {
					log.Printf("Overlay mode error: %v", err)
					http.Error(w, "Overlay mode failed", http.StatusBadGateway)
				}
				return
			}

			// No endpoint and no overlay - return 404
			http.Error(w, "No endpoint configured for this path", http.StatusNotFound)
			return
		}

		// Dispatch based on endpoint type
		h.configMutex.RUnlock()
		switch matchedEndpoint.Type {
		case models.EndpointTypeMock:
			h.handleMockRequest(w, r, matchedEndpoint, translatedPath, bodyBytes)
		case models.EndpointTypeProxy:
			h.handleProxyRequest(w, r, matchedEndpoint, translatedPath, captureGroups)
		case models.EndpointTypeContainer:
			h.handleContainerRequest(w, r, matchedEndpoint, translatedPath)
		default:
			http.Error(w, "Unknown endpoint type", http.StatusInternalServerError)
		}
		return
	} else {
		// Fallback: No endpoints configured, use legacy Items
		translatedPath = requestPath
		items = h.config.Items
	}

	// Check if this is a CORS preflight that should be handled globally
	if r.Method == "OPTIONS" && h.shouldHandleCORSPreflightForItems(r, translatedPath, items) {
		h.configMutex.RUnlock()
		h.handleCORSPreflight(w, r)
		return
	}

	// Determine endpoint ID for logging (empty string if legacy fallback)
	endpointID := ""
	if matchedEndpoint != nil {
		endpointID = matchedEndpoint.ID
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
	if r.Method == "OPTIONS" && h.shouldHandleCORSPreflightForItems(r, translatedPath, items) {
		h.configMutex.RUnlock()
		h.handleCORSPreflight(w, r)
		return
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
func (h *ResponseHandler) shouldHandleCORSPreflightForItems(r *http.Request, translatedPath string, items []models.ResponseItem) bool {
	// Check if global CORS is enabled
	if !h.config.CORS.Enabled {
		return false
	}

	// Check if there's an explicit OPTIONS handler in the items for this translated path
	for _, item := range items {
		if item.Type == "response" && item.Response != nil {
			resp := item.Response
			if !resp.IsEnabled() {
				continue
			}

			// Check if this response handles OPTIONS
			for _, method := range resp.Methods {
				if method == "OPTIONS" {
					// Check if path matches (using translated path)
					matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
					if matchResult.Matches {
						// There's an explicit OPTIONS handler, don't use global CORS
						return false
					}
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

				// Check if this response handles OPTIONS
				for _, method := range resp.Methods {
					if method == "OPTIONS" {
						// Check if path matches (using translated path)
						matchResult := matchPathPatternWithParams(resp.PathPattern, translatedPath)
						if matchResult.Matches {
							// There's an explicit OPTIONS handler, don't use global CORS
							return false
						}
					}
				}
			}
		}
	}

	// No explicit OPTIONS handler, use global CORS
	return true
}

// handleCORSPreflight handles a CORS preflight request
func (h *ResponseHandler) handleCORSPreflight(w http.ResponseWriter, r *http.Request) {
	// Process CORS headers
	corsHeaders := h.corsProcessor.ProcessCORS(r)
	for name, value := range corsHeaders {
		w.Header().Set(name, value)
	}

	// Set status code (default to 204 if not specified)
	status := h.config.CORS.OptionsDefaultStatus
	if status == 0 {
		status = http.StatusNoContent // 204
	}

	w.WriteHeader(status)
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