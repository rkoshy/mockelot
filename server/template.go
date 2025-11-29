package server

import (
	"bytes"
	"encoding/json"
	"strings"
	"text/template"
	"time"
)

// templateFuncs provides custom functions for templates
var templateFuncs = template.FuncMap{
	// JSON functions
	"json": func(v interface{}) string {
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	},
	"jsonPretty": func(v interface{}) string {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return ""
		}
		return string(b)
	},

	// String functions
	"upper":    strings.ToUpper,
	"lower":    strings.ToLower,
	"trim":     strings.TrimSpace,
	"contains": strings.Contains,
	"replace":  strings.ReplaceAll,
	"split":    strings.Split,
	"join":     strings.Join,

	// Time functions
	"now": func() string {
		return time.Now().Format(time.RFC3339)
	},
	"timestamp": func() int64 {
		return time.Now().Unix()
	},
	"timestampMs": func() int64 {
		return time.Now().UnixMilli()
	},

	// Default value function
	"default": func(defaultVal, val interface{}) interface{} {
		if val == nil || val == "" {
			return defaultVal
		}
		return val
	},

	// Coalesce - return first non-empty value
	"coalesce": func(values ...interface{}) interface{} {
		for _, v := range values {
			if v != nil && v != "" {
				return v
			}
		}
		return nil
	},
}

// ProcessTemplate processes a template string with the request context
func ProcessTemplate(templateBody string, context *RequestContext) (string, error) {
	tmpl, err := template.New("response").Funcs(templateFuncs).Parse(templateBody)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, context)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ProcessTemplateHeaders processes template strings in headers
func ProcessTemplateHeaders(headers map[string]string, context *RequestContext) (map[string]string, error) {
	result := make(map[string]string)

	for key, value := range headers {
		// Check if value contains template syntax
		if strings.Contains(value, "{{") {
			processed, err := ProcessTemplate(value, context)
			if err != nil {
				// On error, use original value
				result[key] = value
			} else {
				result[key] = processed
			}
		} else {
			result[key] = value
		}
	}

	return result, nil
}
