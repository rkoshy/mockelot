package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// RequestContext represents the data available to templates and scripts
type RequestContext struct {
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	PathParams  map[string]string      `json:"pathParams"`
	QueryParams map[string][]string    `json:"queryParams"`
	Headers     map[string][]string    `json:"headers"`
	Body        RequestBody            `json:"body"`
	Vars        map[string]interface{} `json:"vars"` // Extracted variables from request validation
}

// RequestBody contains parsed body data in various formats
type RequestBody struct {
	Raw  string                 `json:"raw"`
	JSON interface{}            `json:"json,omitempty"`
	Form map[string][]string    `json:"form,omitempty"`
}

// BuildRequestContext creates a RequestContext from an HTTP request
func BuildRequestContext(r *http.Request, bodyBytes []byte, pathParams map[string]string) *RequestContext {
	ctx := &RequestContext{
		Method:      r.Method,
		Path:        r.URL.Path,
		PathParams:  pathParams,
		QueryParams: r.URL.Query(),
		Headers:     r.Header,
		Body: RequestBody{
			Raw: string(bodyBytes),
		},
	}

	// Ensure PathParams is not nil
	if ctx.PathParams == nil {
		ctx.PathParams = make(map[string]string)
	}

	// Try to parse body as JSON
	if len(bodyBytes) > 0 {
		var jsonData interface{}
		if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
			ctx.Body.JSON = jsonData
		}
	}

	// Try to parse as form data if Content-Type indicates it
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		if form, err := url.ParseQuery(string(bodyBytes)); err == nil {
			ctx.Body.Form = form
		}
	}

	// Also try to parse multipart form data
	if strings.Contains(contentType, "multipart/form-data") {
		// For multipart, the form was already parsed by the request
		// We'll just use the URL query form format
		if r.Form != nil {
			ctx.Body.Form = r.Form
		}
	}

	return ctx
}

// ToMap converts RequestContext to a map for template/script use
func (ctx *RequestContext) ToMap() map[string]interface{} {
	vars := ctx.Vars
	if vars == nil {
		vars = make(map[string]interface{})
	}
	return map[string]interface{}{
		"method":      ctx.Method,
		"path":        ctx.Path,
		"pathParams":  ctx.PathParams,
		"queryParams": ctx.QueryParams,
		"headers":     ctx.Headers,
		"vars":        vars,
		"body": map[string]interface{}{
			"raw":  ctx.Body.Raw,
			"json": ctx.Body.JSON,
			"form": ctx.Body.Form,
		},
	}
}

// GetQueryParam returns a single query parameter value (first value if multiple)
func (ctx *RequestContext) GetQueryParam(key string) string {
	if values, ok := ctx.QueryParams[key]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}

// GetHeader returns a single header value (first value if multiple)
func (ctx *RequestContext) GetHeader(key string) string {
	if values, ok := ctx.Headers[key]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}
