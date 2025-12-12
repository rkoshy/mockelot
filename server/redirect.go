package server

import (
	"fmt"
	"net/http"
	"strings"
)

// HTTPSRedirectHandler creates an HTTP handler that redirects all requests to HTTPS
func HTTPSRedirectHandler(httpsPort int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract hostname from Host header (without port)
		host := strings.Split(r.Host, ":")[0]

		// Build HTTPS URL with the HTTPS port
		var target string
		if httpsPort == 443 {
			// Standard HTTPS port - don't include in URL
			target = fmt.Sprintf("https://%s%s", host, r.RequestURI)
		} else {
			// Custom HTTPS port - include in URL
			target = fmt.Sprintf("https://%s:%d%s", host, httpsPort, r.RequestURI)
		}

		// Send 302 redirect
		http.Redirect(w, r, target, http.StatusFound)
	})
}
