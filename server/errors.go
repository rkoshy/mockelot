package server

import (
	"fmt"
	"net/http"
)

// AppVersion is set once at startup by the main package via SetAppVersion().
// It is embedded in all Mockelot-generated error response bodies.
var AppVersion = "dev"

// SetAppVersion configures the version string included in error responses.
func SetAppVersion(v string) {
	AppVersion = v
}

// mockelotError writes a structured plain-text error response that clearly
// identifies Mockelot as the source of the error (not the upstream backend).
//
// Format:
//
//	Mockelot v1.2.3
//	<message>
//	Incoming request: METHOD https://host/path
func mockelotError(w http.ResponseWriter, r *http.Request, message string, code int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	fullURL := fmt.Sprintf("%s://%s%s", scheme, host, r.RequestURI)

	body := fmt.Sprintf("Mockelot v%s\n%s\nIncoming request: %s %s\n",
		AppVersion, message, r.Method, fullURL)
	fmt.Fprint(w, body)
}
