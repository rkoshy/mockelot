package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
)

// TLSInterceptor handles TLS interception for SOCKS5 HTTPS connections.
// It performs a TLS handshake with the client using a dynamically-generated
// certificate for the target domain, signed by the Mockelot CA.
type TLSInterceptor struct {
	certCache *CertCache
}

// NewTLSInterceptor creates a new TLS interceptor
// Parameters:
//   - certCache: Certificate cache for generating/caching domain-specific certs
func NewTLSInterceptor(certCache *CertCache) *TLSInterceptor {
	return &TLSInterceptor{
		certCache: certCache,
	}
}

// Intercept performs TLS handshake with the client using a dynamic certificate.
// Returns a wrapped connection that reads/writes decrypted data.
// Parameters:
//   - conn: The underlying TCP connection from the SOCKS5 tunnel
//   - targetDomain: The domain the client is connecting to (used for certificate generation)
//
// Returns:
//   - net.Conn: A TLS-wrapped connection for reading/writing decrypted HTTP data
//   - error: Any error during handshake or certificate generation
func (t *TLSInterceptor) Intercept(conn net.Conn, targetDomain string) (net.Conn, error) {
	if t.certCache == nil {
		return nil, fmt.Errorf("TLS interception not available: no certificate cache configured")
	}

	// Get or generate certificate for target domain
	cert, err := t.certCache.GetOrCreate(targetDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate for domain %s: %w", targetDomain, err)
	}

	// Create TLS config with the domain-specific certificate
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
		// Server name is not strictly required since we're acting as the server,
		// but we set it for logging/debugging purposes
		ServerName: targetDomain,
	}

	// Wrap connection with TLS server
	tlsConn := tls.Server(conn, tlsConfig)

	// Perform handshake with timeout (uses connection's deadline if set)
	if err := tlsConn.Handshake(); err != nil {
		// Close the TLS connection on handshake failure
		tlsConn.Close()
		return nil, fmt.Errorf("TLS handshake failed for domain %s: %w", targetDomain, err)
	}

	// Log successful handshake
	state := tlsConn.ConnectionState()
	log.Printf("TLS interception established for %s (TLS %s, cipher: %s)",
		targetDomain,
		tlsVersionString(state.Version),
		tls.CipherSuiteName(state.CipherSuite))

	return tlsConn, nil
}

// tlsVersionString converts a TLS version number to a human-readable string
func tlsVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "1.0"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS13:
		return "1.3"
	default:
		return fmt.Sprintf("0x%04x", version)
	}
}
