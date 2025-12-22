package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"mockelot/models"
)

// OverlayHandler handles overlay mode - proxying requests to real servers
// when a domain is taken over but no endpoint matches the path
type OverlayHandler struct {
	dnsCache      map[string]*dnsCacheEntry
	cacheMutex    sync.RWMutex
	cacheExpiry   time.Duration
	proxyHandler  *ProxyHandler
}

// dnsCacheEntry represents a cached DNS lookup result
type dnsCacheEntry struct {
	ip        string
	timestamp time.Time
}

// NewOverlayHandler creates a new overlay mode handler
func NewOverlayHandler(proxyHandler *ProxyHandler) *OverlayHandler {
	return &OverlayHandler{
		dnsCache:     make(map[string]*dnsCacheEntry),
		cacheExpiry:  5 * time.Minute, // 5 minute cache expiry
		proxyHandler: proxyHandler,
	}
}

// shouldUseOverlay checks if overlay mode should be used for the given domain
// Returns true if domain is in takeover list with overlay mode enabled
func (h *OverlayHandler) shouldUseOverlay(domain string, domainTakeover *models.DomainTakeoverConfig) bool {
	if domainTakeover == nil {
		return false
	}

	for _, domainConfig := range domainTakeover.Domains {
		if !domainConfig.Enabled || !domainConfig.OverlayMode {
			continue
		}

		// Check if domain matches the pattern (already validated by domain matching)
		// For simplicity, we'll do a direct string comparison here
		// In a more robust implementation, we'd use regex matching
		if domain == domainConfig.Pattern {
			return true
		}
	}

	return false
}

// handleOverlay proxies the request to the real server
// Resolves the real IP for the domain and forwards the request
func (h *OverlayHandler) handleOverlay(w http.ResponseWriter, r *http.Request, domain string) error {
	// 1. Resolve real IP for domain (with caching)
	realIP, err := h.resolveRealIP(domain)
	if err != nil {
		return fmt.Errorf("failed to resolve domain %s: %w", domain, err)
	}

	// 2. Build backend URL with real IP
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	// Use the original request URI (includes path and query string)
	backendURL := fmt.Sprintf("%s://%s%s", scheme, realIP, r.URL.RequestURI())

	// 3. Create a synthetic proxy endpoint for this request
	// This allows us to reuse the existing ProxyHandler logic
	proxyEndpoint := &models.Endpoint{
		ID:   "overlay-" + domain,
		Name: "Overlay: " + domain,
		Type: models.EndpointTypeProxy,
		ProxyConfig: &models.ProxyConfig{
			BackendURL:        backendURL,
			TimeoutSeconds:    30,
			StatusPassthrough: true,
			// Header manipulation: preserve Host header
			InboundHeaders: []models.HeaderManipulation{
				{
					Name:       "Host",
					Mode:       models.HeaderModeReplace,
					Value:      domain, // Preserve original domain in Host header
				},
			},
		},
	}

	// 4. Use existing ProxyHandler to execute the request
	// The ProxyHandler already handles:
	// - Header manipulation
	// - Body transformation
	// - Status code translation
	// - Logging

	// Execute the proxy request using the existing handler
	if h.proxyHandler != nil {
		// Use the proxy handler's internal logic
		// Note: We'll need to expose or use the proxy execution logic here
		// For now, let's create a simple direct proxy implementation
		h.executeProxyRequest(w, r, proxyEndpoint, domain)
	} else {
		return fmt.Errorf("proxy handler not available")
	}

	return nil
}

// resolveRealIP resolves the real IP address for a domain (with caching)
func (h *OverlayHandler) resolveRealIP(domain string) (string, error) {
	// Check cache first (read lock)
	h.cacheMutex.RLock()
	if entry, exists := h.dnsCache[domain]; exists {
		// Check if cache entry is still valid
		if time.Since(entry.timestamp) < h.cacheExpiry {
			h.cacheMutex.RUnlock()
			return entry.ip, nil
		}
	}
	h.cacheMutex.RUnlock()

	// Perform DNS lookup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupHost(ctx, domain)
	if err != nil {
		return "", fmt.Errorf("DNS lookup failed: %w", err)
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses found for domain %s", domain)
	}

	// Use first IP address
	ip := ips[0]

	// Store in cache (write lock)
	h.cacheMutex.Lock()
	h.dnsCache[domain] = &dnsCacheEntry{
		ip:        ip,
		timestamp: time.Now(),
	}
	h.cacheMutex.Unlock()

	log.Printf("Resolved %s to %s (cached for %v)", domain, ip, h.cacheExpiry)
	return ip, nil
}

// executeProxyRequest executes a proxy request to the backend server
// This is a simplified version that directly proxies the request
func (h *OverlayHandler) executeProxyRequest(w http.ResponseWriter, r *http.Request, endpoint *models.Endpoint, originalDomain string) {
	// Create backend request
	backendURL := endpoint.ProxyConfig.BackendURL

	// Create new request to backend
	backendReq, err := http.NewRequest(r.Method, backendURL, r.Body)
	if err != nil {
		log.Printf("Failed to create backend request: %v", err)
		http.Error(w, "Failed to create backend request", http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	backendReq.Header = r.Header.Clone()

	// Set Host header to original domain (important for virtual hosting)
	backendReq.Header.Set("Host", originalDomain)

	// Set X-Forwarded-* headers
	backendReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
	backendReq.Header.Set("X-Forwarded-Host", originalDomain)
	if r.TLS != nil {
		backendReq.Header.Set("X-Forwarded-Proto", "https")
	} else {
		backendReq.Header.Set("X-Forwarded-Proto", "http")
	}

	// Create HTTP client with timeout
	timeout := 30 * time.Second
	if endpoint.ProxyConfig.TimeoutSeconds > 0 {
		timeout = time.Duration(endpoint.ProxyConfig.TimeoutSeconds) * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
		// Don't follow redirects - pass them through to client
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Execute backend request
	resp, err := client.Do(backendReq)
	if err != nil {
		log.Printf("Backend request failed: %v", err)
		http.Error(w, "Backend request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write response status
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	if resp.Body != nil {
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Printf("Failed to copy response body: %v", err)
		}
	}

	log.Printf("Overlay mode: proxied %s %s to %s (status: %d)", r.Method, r.URL.Path, backendURL, resp.StatusCode)
}

// ClearDNSCache clears the DNS resolution cache
func (h *OverlayHandler) ClearDNSCache() {
	h.cacheMutex.Lock()
	h.dnsCache = make(map[string]*dnsCacheEntry)
	h.cacheMutex.Unlock()
	log.Println("DNS cache cleared")
}
