package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	certDirName = ".mockelot"
	certSubDir  = "certs"

	caCertFile      = "ca.crt"
	caKeyFile       = "ca.key"
	caTimestampFile = "ca.timestamp"
	serverCertFile  = "server.crt"
	serverKeyFile   = "server.key"
)

// GetDefaultCertNames returns default DNS names and IP addresses for server certificates
// Includes machine hostname (as CN), localhost, and interface IP of default gateway
func GetDefaultCertNames() (dnsNames []string, ipAddresses []net.IP) {
	// Start with empty array
	dnsNames = []string{}
	ipAddresses = []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("::1"),
	}

	// Add machine hostname first (will be used as CN)
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		dnsNames = append(dnsNames, hostname)
	}

	// Add localhost (will be in SANs)
	dnsNames = append(dnsNames, "localhost")

	// Get interface IP that routes to default gateway
	if gatewayIP := getDefaultGatewayIP(); gatewayIP != nil {
		ipAddresses = append(ipAddresses, gatewayIP)
	}

	return dnsNames, ipAddresses
}

// ParseCertNames parses a list of strings (DNS names and/or IP addresses)
// into separate DNS names and IP addresses arrays
func ParseCertNames(certNames []string) (dnsNames []string, ipAddresses []net.IP) {
	for _, name := range certNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// Try to parse as IP address
		if ip := net.ParseIP(name); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			// It's a DNS name
			dnsNames = append(dnsNames, name)
		}
	}

	return dnsNames, ipAddresses
}

// getDefaultGatewayIP returns the IP address of the interface that routes to the default gateway
func getDefaultGatewayIP() net.IP {
	// Connect to a well-known public address to determine which interface is used
	// This doesn't actually send data, just establishes routing
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}

// CertificateManager handles certificate generation and loading
type CertificateManager struct {
	certDir string
}

// NewCertificateManager creates a new certificate manager
func NewCertificateManager() (*CertificateManager, error) {
	certDir, err := GetCertDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get cert directory: %w", err)
	}

	return &CertificateManager{
		certDir: certDir,
	}, nil
}

// GetCertDir returns the certificate storage directory, creating it if needed
func GetCertDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	certDir := filepath.Join(homeDir, certDirName, certSubDir)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create cert directory: %w", err)
	}

	return certDir, nil
}

// GenerateCA generates a new CA certificate and private key
// Returns the certificate and private key in memory, and saves to disk
func (cm *CertificateManager) GenerateCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	// Generate private key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA private key: %w", err)
	}

	// Create CA certificate template
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Mockelot CA",
			Organization: []string{"Mockelot"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	// Create self-signed CA certificate
	caCertBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	// Parse the certificate
	caCert, err := x509.ParseCertificate(caCertBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Save CA cert to disk (PEM format)
	caCertPath := filepath.Join(cm.certDir, caCertFile)
	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertBytes,
	})
	if err := os.WriteFile(caCertPath, caCertPEM, 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to write CA certificate: %w", err)
	}

	// Save CA private key to disk (PEM format)
	caKeyPath := filepath.Join(cm.certDir, caKeyFile)
	caKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	if err := os.WriteFile(caKeyPath, caKeyPEM, 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to write CA private key: %w", err)
	}

	// Save timestamp
	timestampPath := filepath.Join(cm.certDir, caTimestampFile)
	timestamp := time.Now().Format(time.RFC3339)
	if err := os.WriteFile(timestampPath, []byte(timestamp), 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to write CA timestamp: %w", err)
	}

	return caCert, caPrivKey, nil
}

// LoadCA loads the CA certificate and private key from disk
func (cm *CertificateManager) LoadCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	// Load CA certificate
	caCertPath := filepath.Join(cm.certDir, caCertFile)
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
	}

	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Load CA private key
	caKeyPath := filepath.Join(cm.certDir, caKeyFile)
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA private key: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA private key PEM")
	}

	caPrivKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA private key: %w", err)
	}

	return caCert, caPrivKey, nil
}

// CAExists checks if a CA certificate exists on disk
func (cm *CertificateManager) CAExists() bool {
	caCertPath := filepath.Join(cm.certDir, caCertFile)
	caKeyPath := filepath.Join(cm.certDir, caKeyFile)

	_, certErr := os.Stat(caCertPath)
	_, keyErr := os.Stat(caKeyPath)

	return certErr == nil && keyErr == nil
}

// GetCATimestamp returns the timestamp when the CA was generated
func (cm *CertificateManager) GetCATimestamp() (time.Time, error) {
	timestampPath := filepath.Join(cm.certDir, caTimestampFile)
	timestampBytes, err := os.ReadFile(timestampPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read CA timestamp: %w", err)
	}

	timestamp, err := time.Parse(time.RFC3339, string(timestampBytes))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse CA timestamp: %w", err)
	}

	return timestamp, nil
}

// GetCACertPEM returns the CA certificate in PEM format for download
func (cm *CertificateManager) GetCACertPEM() ([]byte, error) {
	caCertPath := filepath.Join(cm.certDir, caCertFile)
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	return caCertPEM, nil
}

// GenerateServerCert generates a new server certificate signed by the CA
// Returns PEM-encoded certificate and private key
// If dnsNames or ipAddresses are empty, defaults will be used
func (cm *CertificateManager) GenerateServerCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, dnsNames []string, ipAddresses []net.IP) ([]byte, []byte, error) {
	// Generate server private key
	serverPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate server private key: %w", err)
	}

	// Use defaults if not provided
	if len(dnsNames) == 0 || len(ipAddresses) == 0 {
		defaultDNS, defaultIPs := GetDefaultCertNames()
		if len(dnsNames) == 0 {
			dnsNames = defaultDNS
		}
		if len(ipAddresses) == 0 {
			ipAddresses = defaultIPs
		}
	}

	// Use first DNS name as CN, fallback to "localhost"
	cn := "localhost"
	if len(dnsNames) > 0 {
		cn = dnsNames[0]
	}

	// Create server certificate template
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{"Mockelot"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0), // 1 year
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
	}

	// Create server certificate signed by CA
	serverCertBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create server certificate: %w", err)
	}

	// Encode to PEM
	serverCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertBytes,
	})

	serverKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(serverPrivKey),
	})

	// Save to disk for reference
	serverCertPath := filepath.Join(cm.certDir, serverCertFile)
	if err := os.WriteFile(serverCertPath, serverCertPEM, 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to write server certificate: %w", err)
	}

	serverKeyPath := filepath.Join(cm.certDir, serverKeyFile)
	if err := os.WriteFile(serverKeyPath, serverKeyPEM, 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to write server private key: %w", err)
	}

	return serverCertPEM, serverKeyPEM, nil
}

// LoadUserCACert loads a user-provided CA certificate and key from paths
func LoadUserCACert(certPath, keyPath string) (*x509.Certificate, *rsa.PrivateKey, error) {
	// Load certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Load private key
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read key file: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode key PEM")
	}

	// Try parsing as PKCS1 first
	privKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		// Try PKCS8 format
		privKeyInterface, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		var ok bool
		privKey, ok = privKeyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("private key is not RSA")
		}
	}

	return cert, privKey, nil
}

// LoadUserServerCert loads a user-provided server certificate and key from paths
// Returns PEM-encoded certificate and key bytes
func LoadUserServerCert(certPath, keyPath, bundlePath string) ([]byte, []byte, error) {
	// Load certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	// Load private key
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Validate PEM format
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("invalid certificate PEM format")
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("invalid key PEM format")
	}

	// If bundle provided, append it to the certificate
	finalCertPEM := certPEM
	if bundlePath != "" {
		bundlePEM, err := os.ReadFile(bundlePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read bundle file: %w", err)
		}

		// Validate bundle PEM format
		bundleBlock, _ := pem.Decode(bundlePEM)
		if bundleBlock == nil {
			return nil, nil, fmt.Errorf("invalid bundle PEM format")
		}

		// Append bundle to certificate
		finalCertPEM = append(certPEM, bundlePEM...)
	}

	return finalCertPEM, keyPEM, nil
}
