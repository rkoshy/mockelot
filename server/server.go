package server

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"mockelot/models"
)

type HTTPServer struct {
	httpServer       *http.Server
	httpsServer      *http.Server
	config           *models.AppConfig
	configMutex      sync.RWMutex
	requestLogger    RequestLogger
	httpStopChan     chan struct{}
	httpsStopChan    chan struct{}
	certManager      *CertificateManager
	proxyHandler     *ProxyHandler
	containerHandler *ContainerHandler
	startupCtx       context.Context    // Context for container startup
	startupCancel    context.CancelFunc // Cancel function for startup
}

func NewHTTPServer(config *models.AppConfig, requestLogger RequestLogger, eventSender EventSender, containerHandler *ContainerHandler, proxyHandler *ProxyHandler) *HTTPServer {
	certManager, err := NewCertificateManager()
	if err != nil {
		log.Printf("Warning: Failed to initialize certificate manager: %v", err)
	}

	// Proxy handler is passed in (shared with container handler)

	return &HTTPServer{
		config:           config,
		requestLogger:    requestLogger,
		httpStopChan:     make(chan struct{}),
		httpsStopChan:    make(chan struct{}),
		certManager:      certManager,
		proxyHandler:     proxyHandler,
		containerHandler: containerHandler,
	}
}

// StartHTTP starts the HTTP server
func (s *HTTPServer) StartHTTP() error {
	// Thread-safe config access
	s.configMutex.RLock()
	port := s.config.Port
	httpToHTTPSRedirect := s.config.HTTPToHTTPSRedirect
	httpsEnabled := s.config.HTTPSEnabled
	httpsPort := s.config.HTTPSPort
	s.configMutex.RUnlock()

	var handler http.Handler

	// If HTTP to HTTPS redirect is enabled and HTTPS is enabled, use redirect handler
	if httpToHTTPSRedirect && httpsEnabled {
		handler = HTTPSRedirectHandler(httpsPort)
	} else {
		// Use normal response handler
		responseHandler := NewResponseHandler(s.config, s.requestLogger, s.proxyHandler, s.containerHandler)
		handler = http.HandlerFunc(responseHandler.HandleRequest)
	}

	// Wrap with h2c if HTTP/2 is enabled (for cleartext HTTP/2)
	s.configMutex.RLock()
	http2Enabled := s.config.HTTP2Enabled
	s.configMutex.RUnlock()

	if http2Enabled {
		h2s := &http2.Server{}
		handler = h2c.NewHandler(handler, h2s)
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on port %d", port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
		s.httpStopChan <- struct{}{}
	}()

	return nil
}

// StartHTTPS starts the HTTPS server with TLS configuration
func (s *HTTPServer) StartHTTPS() error {
	if s.certManager == nil {
		return fmt.Errorf("certificate manager not initialized")
	}

	// Thread-safe config access
	s.configMutex.RLock()
	httpsPort := s.config.HTTPSPort
	certMode := s.config.CertMode
	certPaths := s.config.CertPaths
	certNames := s.config.CertNames
	s.configMutex.RUnlock()

	// Default to auto mode if not specified
	if certMode == "" {
		certMode = models.CertModeAuto
	}

	// Always start with defaults
	dnsNames, ipAddresses := GetDefaultCertNames()

	// If custom names provided, append them to the defaults
	if len(certNames) > 0 {
		customDNS, customIPs := ParseCertNames(certNames)
		dnsNames = append(dnsNames, customDNS...)
		ipAddresses = append(ipAddresses, customIPs...)
	}

	var certPEM, keyPEM []byte
	var err error

	switch certMode {
	case models.CertModeAuto:
		// Auto-generate certificates
		var caCert *x509.Certificate
		var caPrivKey *rsa.PrivateKey

		// Check if CA exists, otherwise generate it
		if s.certManager.CAExists() {
			caCert, caPrivKey, err = s.certManager.LoadCA()
			if err != nil {
				log.Printf("Failed to load existing CA, generating new one: %v", err)
				caCert, caPrivKey, err = s.certManager.GenerateCA()
				if err != nil {
					return fmt.Errorf("failed to generate CA: %w", err)
				}
			}
		} else {
			caCert, caPrivKey, err = s.certManager.GenerateCA()
			if err != nil {
				return fmt.Errorf("failed to generate CA: %w", err)
			}
		}

		// Generate server certificate with custom or default names
		certPEM, keyPEM, err = s.certManager.GenerateServerCert(caCert, caPrivKey, dnsNames, ipAddresses)
		if err != nil {
			return fmt.Errorf("failed to generate server certificate: %w", err)
		}

	case models.CertModeCAProvided:
		// User provides CA cert + key, we generate server cert
		if certPaths.CACertPath == "" || certPaths.CAKeyPath == "" {
			return fmt.Errorf("CA certificate and key paths are required for ca-provided mode")
		}

		caCert, caPrivKey, err := LoadUserCACert(certPaths.CACertPath, certPaths.CAKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load user CA certificate: %w", err)
		}

		// Generate server certificate using user's CA with custom or default names
		certPEM, keyPEM, err = s.certManager.GenerateServerCert(caCert, caPrivKey, dnsNames, ipAddresses)
		if err != nil {
			return fmt.Errorf("failed to generate server certificate with user CA: %w", err)
		}

	case models.CertModeCertProvided:
		// User provides server cert + key + optional bundle
		if certPaths.ServerCertPath == "" || certPaths.ServerKeyPath == "" {
			return fmt.Errorf("server certificate and key paths are required for cert-provided mode")
		}

		certPEM, keyPEM, err = LoadUserServerCert(certPaths.ServerCertPath, certPaths.ServerKeyPath, certPaths.ServerBundlePath)
		if err != nil {
			return fmt.Errorf("failed to load user server certificate: %w", err)
		}

	default:
		return fmt.Errorf("unknown certificate mode: %s", certMode)
	}

	// Create TLS config from PEM-encoded cert and key
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Create response handler
	responseHandler := NewResponseHandler(s.config, s.requestLogger, s.proxyHandler, s.containerHandler)

	// Create HTTPS server
	s.httpsServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", httpsPort),
		Handler:      http.HandlerFunc(responseHandler.HandleRequest),
		TLSConfig:    tlsConfig,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Configure HTTP/2 support
	s.configMutex.RLock()
	http2Enabled := s.config.HTTP2Enabled
	s.configMutex.RUnlock()

	if http2Enabled {
		// Enable HTTP/2 (default behavior, but explicit for clarity)
		http2.ConfigureServer(s.httpsServer, &http2.Server{})
	} else {
		// Disable HTTP/2 by setting TLSNextProto to empty map
		s.httpsServer.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting HTTPS server on port %d", httpsPort)
		// Use ListenAndServeTLS with empty strings since we provided TLSConfig
		if err := s.httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTPS server error: %v", err)
		}
		s.httpsStopChan <- struct{}{}
	}()

	return nil
}

// Start starts both HTTP and HTTPS servers based on configuration
func (s *HTTPServer) Start() error {
	s.configMutex.RLock()
	httpsEnabled := s.config.HTTPSEnabled
	endpoints := s.config.Endpoints
	s.configMutex.RUnlock()

	// Create cancellable context for container startup (will be used when frontend calls StartContainers)
	s.startupCtx, s.startupCancel = context.WithCancel(context.Background())

	log.Printf("Server started. Waiting for frontend to signal readiness before starting containers...")

	// Note: Containers will be started by explicit call to StartContainers() from frontend
	// This prevents race condition where backend emits progress events before frontend is ready

	// Start health checks for proxy endpoints
	if s.proxyHandler != nil {
		var proxyEndpoints []*models.Endpoint
		for i := range endpoints {
			if endpoints[i].Type == models.EndpointTypeProxy {
				proxyEndpoints = append(proxyEndpoints, &endpoints[i])
			}
		}
		s.proxyHandler.StartHealthChecks(proxyEndpoints)
	}

	// Always start HTTP server
	if err := s.StartHTTP(); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start HTTPS server if enabled
	if httpsEnabled {
		if err := s.StartHTTPS(); err != nil {
			log.Printf("Failed to start HTTPS server: %v", err)
			// Don't fail completely if HTTPS fails, HTTP is still running
		}
	}

	// Start monitoring for any container endpoints in config
	// This will detect and track any containers already running from previous sessions
	s.EnsureContainerMonitoring()

	return nil
}

// StopHTTP stops the HTTP server
func (s *HTTPServer) StopHTTP() error {
	if s.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
		return err
	}

	<-s.httpStopChan
	log.Println("HTTP server stopped")
	return nil
}

// StopHTTPS stops the HTTPS server
func (s *HTTPServer) StopHTTPS() error {
	if s.httpsServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpsServer.Shutdown(ctx); err != nil {
		log.Printf("HTTPS server shutdown error: %v", err)
		return err
	}

	<-s.httpsStopChan
	log.Println("HTTPS server stopped")
	return nil
}

// EnsureContainerMonitoring starts status/stats polling for all container endpoints in config
// This is called when server starts or config is loaded to monitor any already-running containers
func (s *HTTPServer) EnsureContainerMonitoring() {
	if s.containerHandler == nil {
		log.Printf("WARNING: Container handler is nil")
		return
	}

	s.configMutex.RLock()

	// Build list of container endpoints - use pointers to ACTUAL config endpoints, not copies
	var containerEndpoints []*models.Endpoint
	for i := range s.config.Endpoints {
		endpoint := &s.config.Endpoints[i]
		if endpoint.Type == models.EndpointTypeContainer {
			containerEndpoints = append(containerEndpoints, endpoint)
		}
	}
	s.configMutex.RUnlock()

	if len(containerEndpoints) > 0 {
		// Stop any existing polling to avoid duplicates
		s.containerHandler.StopPolling()

		// Start fresh polling for all container endpoints
		s.containerHandler.StartContainerStatusPolling(containerEndpoints)
		s.containerHandler.StartContainerStatsPolling(containerEndpoints)
	}
}

// StartContainers starts all enabled container endpoints
// This should be called by the frontend after it's ready to receive progress events
func (s *HTTPServer) StartContainers() error {
	s.configMutex.RLock()
	endpoints := s.config.Endpoints
	s.configMutex.RUnlock()

	if s.containerHandler == nil {
		log.Printf("WARNING: Container handler not available")
		return nil
	}

	var containerEndpoints []*models.Endpoint
	for i := range endpoints {
		endpoint := &endpoints[i]
		if endpoint.Type == models.EndpointTypeContainer && endpoint.IsEnabled() {
			// Check if container is already running
			status := s.containerHandler.GetContainerStatus(endpoint.ID)

			if status != nil && status.Running {
				// Container is already running
				if endpoint.ContainerConfig != nil && endpoint.ContainerConfig.RestartOnServerStart {
					// Restart the container
					// Stop first
					if err := s.containerHandler.StopContainer(s.startupCtx, endpoint); err != nil {
						log.Printf("Failed to stop container for endpoint %s: %v", endpoint.Name, err)
						// Check if cancelled
						if s.startupCtx.Err() != nil {
							return fmt.Errorf("startup cancelled: %w", s.startupCtx.Err())
						}
						// Continue with other containers even if one fails
						continue
					}

					// Then start
					if err := s.containerHandler.StartContainer(s.startupCtx, endpoint); err != nil {
						log.Printf("Failed to start container for endpoint %s: %v", endpoint.Name, err)
						// Check if cancelled
						if s.startupCtx.Err() != nil {
							return fmt.Errorf("startup cancelled: %w", s.startupCtx.Err())
						}
						// Continue with other containers even if one fails
					}
				}
				// Skip starting - container is already running and RestartOnServerStart is false
			} else {
				// Container is not running, start it normally
				if err := s.containerHandler.StartContainer(s.startupCtx, endpoint); err != nil {
					log.Printf("Failed to start container for endpoint %s: %v", endpoint.Name, err)
					// Check if cancelled
					if s.startupCtx.Err() != nil {
						return fmt.Errorf("startup cancelled: %w", s.startupCtx.Err())
					}
					// Continue with other containers even if one fails
				}
			}

			containerEndpoints = append(containerEndpoints, endpoint)
		}
	}

	// Start container status and stats polling (status: 10s, stats: 5s)
	if len(containerEndpoints) > 0 {
		s.containerHandler.StartContainerStatusPolling(containerEndpoints)
		s.containerHandler.StartContainerStatsPolling(containerEndpoints)
	}

	return nil
}

// Stop stops both HTTP and HTTPS servers
func (s *HTTPServer) Stop() error {
	var httpErr, httpsErr error

	// Stop containers before stopping servers
	if s.containerHandler != nil {
		// Stop polling goroutines first
		s.containerHandler.StopPolling()

		s.configMutex.RLock()
		endpoints := s.config.Endpoints
		s.configMutex.RUnlock()

		for i := range endpoints {
			endpoint := &endpoints[i]
			if endpoint.Type == models.EndpointTypeContainer {
				if err := s.containerHandler.StopContainer(context.Background(), endpoint); err != nil {
					log.Printf("Error stopping container for endpoint %s: %v", endpoint.Name, err)
				}
			}
		}
	}

	// Stop HTTP server if running
	if s.httpServer != nil {
		httpErr = s.StopHTTP()
	}

	// Stop HTTPS server if running
	if s.httpsServer != nil {
		httpsErr = s.StopHTTPS()
	}

	// Return first error encountered
	if httpErr != nil {
		return httpErr
	}
	if httpsErr != nil {
		return httpsErr
	}

	log.Println("All servers stopped")
	return nil
}

// RestartHTTPS restarts the HTTPS server (used after CA regeneration)
func (s *HTTPServer) RestartHTTPS() error {
	// Stop HTTPS server if running
	if s.httpsServer != nil {
		if err := s.StopHTTPS(); err != nil {
			log.Printf("Error stopping HTTPS server: %v", err)
		}
		// Reset the stop channel
		s.httpsStopChan = make(chan struct{})
	}

	// Start HTTPS server
	return s.StartHTTPS()
}

func (s *HTTPServer) UpdateConfig(newConfig *models.AppConfig) {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()
	s.config = newConfig
}

// GetProxyHealthStatus returns the health status for a proxy endpoint
func (s *HTTPServer) GetProxyHealthStatus(endpointID string) *models.HealthStatus {
	if s.proxyHandler == nil {
		return nil
	}
	return s.proxyHandler.GetHealthStatus(endpointID)
}

// GetContainerHealthStatus returns the health status for a container endpoint
func (s *HTTPServer) GetContainerHealthStatus(endpointID string) *models.HealthStatus {
	if s.containerHandler == nil {
		return nil
	}
	return s.containerHandler.GetHealthStatus(endpointID)
}

// GetContainerStatus returns the runtime status for a container endpoint
func (s *HTTPServer) GetContainerStatus(endpointID string) *models.ContainerStatus {
	if s.containerHandler == nil {
		return nil
	}
	return s.containerHandler.GetContainerStatus(endpointID)
}

// GetContainerStats returns the resource usage stats for a container endpoint
func (s *HTTPServer) GetContainerStats(endpointID string) *models.ContainerStats {
	if s.containerHandler == nil {
		return nil
	}
	return s.containerHandler.GetContainerStats(endpointID)
}

// GetContainerLogs retrieves container stdout/stderr logs
func (s *HTTPServer) GetContainerLogs(ctx context.Context, endpointID string, tail int) (string, error) {
	if s.containerHandler == nil {
		return "", fmt.Errorf("container handler not available")
	}
	return s.containerHandler.GetContainerLogs(ctx, endpointID, tail)
}

// StartSingleContainer starts a single container endpoint
func (s *HTTPServer) StartSingleContainer(ctx context.Context, endpoint *models.Endpoint) error {
	if s.containerHandler == nil {
		return fmt.Errorf("container handler not available")
	}

	if err := s.containerHandler.StartContainer(ctx, endpoint); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

// StopSingleContainer stops (and removes) a single container endpoint
func (s *HTTPServer) StopSingleContainer(ctx context.Context, endpoint *models.Endpoint) error {
	if s.containerHandler == nil {
		return fmt.Errorf("container handler not available")
	}

	if err := s.containerHandler.StopContainer(ctx, endpoint); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return nil
}

// RestartContainer restarts a container endpoint
func (s *HTTPServer) RestartContainer(ctx context.Context, endpoint *models.Endpoint) error {
	if s.containerHandler == nil {
		return fmt.Errorf("container handler not available")
	}

	if err := s.containerHandler.StopContainer(ctx, endpoint); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	if err := s.containerHandler.StartContainer(ctx, endpoint); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}