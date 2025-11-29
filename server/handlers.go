package server

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"mockelot/models"
)

type RequestLogger interface {
	LogRequest(log models.RequestLog)
}

type ResponseHandler struct {
	config        *models.AppConfig
	configMutex   sync.RWMutex
	requestLogger RequestLogger
}

func NewResponseHandler(config *models.AppConfig, logger RequestLogger) *ResponseHandler {
	return &ResponseHandler{
		config:        config,
		requestLogger: logger,
	}
}

func (h *ResponseHandler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	// Read request body
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Find matching response configuration and extract path parameters
	h.configMutex.RLock()
	var matchedResponse *models.MethodResponse
	var pathParams map[string]string
	var extractedVars map[string]interface{}
	allResponses := h.config.GetAllResponses()
	for i := range allResponses {
		resp := &allResponses[i]
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

		// Check if path matches and extract path parameters
		if methodMatches {
			matchResult := matchPathPatternWithParams(resp.PathPattern, r.URL.Path)
			if matchResult.Matches {
				// Build initial context for validation (without vars yet)
				tempContext := BuildRequestContext(r, bodyBytes, matchResult.PathParams)

				// Run request body validation if configured
				validationResult := ValidateRequest(resp.RequestValidation, string(bodyBytes), tempContext)
				if !validationResult.Valid {
					// Validation failed - skip this response and try next
					log.Printf("Validation failed for %s %s: %s", r.Method, r.URL.Path, validationResult.Error)
					continue
				}

				// Validation passed - use this response
				matchedResponse = resp
				pathParams = matchResult.PathParams
				extractedVars = validationResult.Vars
				break
			}
		}
	}
	h.configMutex.RUnlock()

	// Determine status code for logging
	statusCode := http.StatusNotFound
	if matchedResponse != nil {
		statusCode = matchedResponse.StatusCode
	}

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

	// Log the request with the status code we'll return
	requestLog := models.RequestLog{
		ID:          uuid.New().String(),
		Timestamp:   time.Now(),
		Method:      r.Method,
		Path:        r.URL.Path,
		StatusCode:  statusCode,
		SourceIP:    r.RemoteAddr,
		Headers:     headersCopy,
		Body:        string(bodyBytes),
		QueryParams: queryParamsCopy,
		Protocol:    r.Proto,
		UserAgent:   r.UserAgent(),
	}

	// Send log to logger
	h.requestLogger.LogRequest(requestLog)

	if matchedResponse == nil {
		http.Error(w, "No matching response configuration", http.StatusNotFound)
		return
	}

	// Process response based on mode
	finalBody, finalHeaders, finalStatus, finalDelay := h.processResponse(
		matchedResponse, r, bodyBytes, pathParams, extractedVars,
	)

	// Implement response delay
	if finalDelay > 0 {
		time.Sleep(time.Duration(finalDelay) * time.Millisecond)
	}

	// Set headers
	for name, value := range finalHeaders {
		w.Header().Set(name, value)
	}

	// Set status code
	w.WriteHeader(finalStatus)

	// Write response body
	w.Write([]byte(finalBody))
}

// processResponse processes the response based on the response mode
func (h *ResponseHandler) processResponse(
	resp *models.MethodResponse,
	r *http.Request,
	bodyBytes []byte,
	pathParams map[string]string,
	extractedVars map[string]interface{},
) (body string, headers map[string]string, status int, delay int) {
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
		processedBody, err := ProcessTemplate(resp.Body, reqContext)
		if err != nil {
			log.Printf("Template processing error: %v", err)
			// Fall back to static body on error
		} else {
			body = processedBody
		}

		// Also process headers as templates
		processedHeaders, err := ProcessTemplateHeaders(resp.Headers, reqContext)
		if err != nil {
			log.Printf("Template header processing error: %v", err)
		} else {
			headers = processedHeaders
		}

	case models.ResponseModeScript:
		// Build request context with extracted vars
		reqContext := BuildRequestContext(r, bodyBytes, pathParams)
		reqContext.Vars = extractedVars

		// Execute script
		scriptResp, err := ProcessScript(resp.ScriptBody, reqContext, resp)
		if err != nil {
			log.Printf("Script execution error: %v", err)
			// Fall back to static response on error
		} else {
			body = scriptResp.Body
			headers = scriptResp.Headers
			status = scriptResp.Status
			delay = scriptResp.Delay
		}

	default:
		// Static mode - use values as-is (already set above)
	}

	return
}