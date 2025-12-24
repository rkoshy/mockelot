package server

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"log"
	"sync"
	"time"
)

// CertCache provides thread-safe caching of dynamically generated TLS certificates
// for SOCKS5 TLS interception. Certificates are generated on-demand for each domain
// and cached to improve performance.
type CertCache struct {
	mu          sync.RWMutex
	certs       map[string]*cachedCert
	maxEntries  int
	certManager *CertificateManager
	caCert      *x509.Certificate
	caKey       *rsa.PrivateKey
}

// cachedCert holds a cached TLS certificate with metadata
type cachedCert struct {
	cert      *tls.Certificate
	createdAt time.Time
}

// NewCertCache creates a new certificate cache
// Parameters:
//   - certManager: The certificate manager for generating new certs
//   - caCert: The CA certificate to sign new domain certs
//   - caKey: The CA private key for signing
//   - maxEntries: Maximum number of certificates to cache (LRU eviction when exceeded)
func NewCertCache(certManager *CertificateManager, caCert *x509.Certificate, caKey *rsa.PrivateKey, maxEntries int) *CertCache {
	return &CertCache{
		certs:       make(map[string]*cachedCert),
		maxEntries:  maxEntries,
		certManager: certManager,
		caCert:      caCert,
		caKey:       caKey,
	}
}

// GetOrCreate returns a cached certificate or generates a new one for the domain
// Thread-safe: uses read lock for cache hits, write lock for cache misses
func (c *CertCache) GetOrCreate(domain string) (*tls.Certificate, error) {
	// Check cache first (read lock)
	c.mu.RLock()
	if cached, ok := c.certs[domain]; ok {
		c.mu.RUnlock()
		return cached.cert, nil
	}
	c.mu.RUnlock()

	// Generate new certificate (write lock)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have created it)
	if cached, ok := c.certs[domain]; ok {
		return cached.cert, nil
	}

	// Evict oldest if at capacity
	if len(c.certs) >= c.maxEntries {
		c.evictOldest()
	}

	// Generate certificate for domain
	// The domain is used as the DNS SAN (Subject Alternative Name)
	certPEM, keyPEM, err := c.certManager.GenerateServerCert(
		c.caCert,
		c.caKey,
		[]string{domain},
		nil, // No IP addresses needed for domain certs
	)
	if err != nil {
		return nil, err
	}

	// Parse into tls.Certificate
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	// Cache it
	c.certs[domain] = &cachedCert{
		cert:      &tlsCert,
		createdAt: time.Now(),
	}

	log.Printf("CertCache: Generated certificate for domain: %s (cache size: %d)", domain, len(c.certs))

	return &tlsCert, nil
}

// evictOldest removes the oldest cached certificate (LRU eviction)
// Must be called with write lock held
func (c *CertCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, cached := range c.certs {
		if oldestKey == "" || cached.createdAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = cached.createdAt
		}
	}

	if oldestKey != "" {
		delete(c.certs, oldestKey)
		log.Printf("CertCache: Evicted oldest certificate for domain: %s", oldestKey)
	}
}

// Clear empties the cache (useful for cleanup or when CA cert changes)
func (c *CertCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.certs = make(map[string]*cachedCert)
	log.Printf("CertCache: Cleared all cached certificates")
}

// Size returns the current number of cached certificates
func (c *CertCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.certs)
}
