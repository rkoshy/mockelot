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

	"mockelot/models"
)

type HTTPServer struct {
	httpServer    *http.Server
	httpsServer   *http.Server
	config        *models.AppConfig
	configMutex   sync.RWMutex
	requestLogger RequestLogger
	httpStopChan  chan struct{}
	httpsStopChan chan struct{}
	certManager   *CertificateManager
}

func NewHTTPServer(config *models.AppConfig, requestLogger RequestLogger) *HTTPServer {
	certManager, err := NewCertificateManager()
	if err != nil {
		log.Printf("Warning: Failed to initialize certificate manager: %v", err)
	}

	return &HTTPServer{
		config:        config,
		requestLogger: requestLogger,
		httpStopChan:  make(chan struct{}),
		httpsStopChan: make(chan struct{}),
		certManager:   certManager,
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
		responseHandler := NewResponseHandler(s.config, s.requestLogger)
		handler = http.HandlerFunc(responseHandler.HandleRequest)
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
	responseHandler := NewResponseHandler(s.config, s.requestLogger)

	// Create HTTPS server
	s.httpsServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", httpsPort),
		Handler:      http.HandlerFunc(responseHandler.HandleRequest),
		TLSConfig:    tlsConfig,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
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
	s.configMutex.RUnlock()

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

// Stop stops both HTTP and HTTPS servers
func (s *HTTPServer) Stop() error {
	var httpErr, httpsErr error

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