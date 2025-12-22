package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"mockelot/models"
)

// SOCKS5 Protocol Constants
const (
	socks5Version = 0x05

	// Authentication methods
	authMethodNoAuth       = 0x00
	authMethodUserPassword = 0x02
	authMethodNoAcceptable = 0xFF

	// Commands
	cmdConnect = 0x01

	// Address types
	atypIPv4   = 0x01
	atypDomain = 0x03
	atypIPv6   = 0x04

	// Reply codes
	replySuccess              = 0x00
	replyGeneralFailure       = 0x01
	replyConnectionNotAllowed = 0x02
	replyNetworkUnreachable   = 0x03
	replyHostUnreachable      = 0x04
	replyConnectionRefused    = 0x05
	replyTTLExpired           = 0x06
	replyCommandNotSupported  = 0x07
	replyAddressNotSupported  = 0x08
)

// SOCKS5Server handles SOCKS5 proxy connections
type SOCKS5Server struct {
	config         *models.SOCKS5Config
	listener       net.Listener
	responseHandler *ResponseHandler
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	running        bool
	mu             sync.Mutex
}

// NewSOCKS5Server creates a new SOCKS5 server instance
func NewSOCKS5Server(config *models.SOCKS5Config, handler *ResponseHandler) *SOCKS5Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &SOCKS5Server{
		config:          config,
		responseHandler: handler,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start begins listening for SOCKS5 connections
func (s *SOCKS5Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("SOCKS5 server already running")
	}

	addr := fmt.Sprintf(":%d", s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to start SOCKS5 server: %w", err)
	}

	s.listener = listener
	s.running = true
	s.mu.Unlock()

	log.Printf("SOCKS5 server listening on %s", addr)

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				log.Printf("SOCKS5 accept error: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn)
		}()
	}
}

// Stop gracefully shuts down the SOCKS5 server
func (s *SOCKS5Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	log.Println("Stopping SOCKS5 server...")
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for all connections to finish (with timeout)
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("SOCKS5 server stopped")
	case <-time.After(5 * time.Second):
		log.Println("SOCKS5 server stopped (timeout)")
	}

	return nil
}

// handleConnection processes a single SOCKS5 connection
func (s *SOCKS5Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set read deadline for handshake
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// 1. Version identification/method selection
	authMethod, err := s.handleHandshake(conn)
	if err != nil {
		log.Printf("SOCKS5 handshake failed: %v", err)
		return
	}

	// 2. Authentication (if required)
	if authMethod == authMethodUserPassword {
		if err := s.handleAuthentication(conn); err != nil {
			log.Printf("SOCKS5 authentication failed: %v", err)
			return
		}
	}

	// 3. Request (CONNECT command)
	targetAddr, targetPort, err := s.handleRequest(conn)
	if err != nil {
		log.Printf("SOCKS5 request failed: %v", err)
		return
	}

	// Reset read deadline after handshake
	conn.SetReadDeadline(time.Time{})

	log.Printf("SOCKS5 connection established to %s:%d", targetAddr, targetPort)

	// 4. Tunnel HTTP traffic
	s.handleTunnel(conn, targetAddr, targetPort)
}

// handleHandshake performs SOCKS5 version identification and method selection
func (s *SOCKS5Server) handleHandshake(conn net.Conn) (byte, error) {
	// Read version identifier/method selection message
	// +----+----------+----------+
	// |VER | NMETHODS | METHODS  |
	// +----+----------+----------+
	// | 1  |    1     | 1 to 255 |
	// +----+----------+----------+

	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return 0, fmt.Errorf("read version: %w", err)
	}

	version := buf[0]
	nMethods := buf[1]

	if version != socks5Version {
		return 0, fmt.Errorf("unsupported SOCKS version: %d", version)
	}

	// Read methods
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return 0, fmt.Errorf("read methods: %w", err)
	}

	// Select authentication method
	selectedMethod := byte(authMethodNoAcceptable)
	if s.config.Authentication {
		// Check if client supports username/password
		for _, method := range methods {
			if method == authMethodUserPassword {
				selectedMethod = authMethodUserPassword
				break
			}
		}
	} else {
		// Check if client supports no authentication
		for _, method := range methods {
			if method == authMethodNoAuth {
				selectedMethod = authMethodNoAuth
				break
			}
		}
	}

	// Send method selection message
	// +----+--------+
	// |VER | METHOD |
	// +----+--------+
	// | 1  |   1    |
	// +----+--------+
	if _, err := conn.Write([]byte{socks5Version, selectedMethod}); err != nil {
		return 0, fmt.Errorf("write method selection: %w", err)
	}

	if selectedMethod == authMethodNoAcceptable {
		return 0, fmt.Errorf("no acceptable authentication method")
	}

	return selectedMethod, nil
}

// handleAuthentication performs username/password authentication
func (s *SOCKS5Server) handleAuthentication(conn net.Conn) error {
	// Read authentication request
	// +----+------+----------+------+----------+
	// |VER | ULEN |  UNAME   | PLEN |  PASSWD  |
	// +----+------+----------+------+----------+
	// | 1  |  1   | 1 to 255 |  1   | 1 to 255 |
	// +----+------+----------+------+----------+

	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("read auth version: %w", err)
	}

	version := buf[0]
	if version != 0x01 {
		return fmt.Errorf("unsupported auth version: %d", version)
	}

	// Read username
	uLen := buf[1]
	username := make([]byte, uLen)
	if _, err := io.ReadFull(conn, username); err != nil {
		return fmt.Errorf("read username: %w", err)
	}

	// Read password length
	if _, err := io.ReadFull(conn, buf[:1]); err != nil {
		return fmt.Errorf("read password length: %w", err)
	}
	pLen := buf[0]

	// Read password
	password := make([]byte, pLen)
	if _, err := io.ReadFull(conn, password); err != nil {
		return fmt.Errorf("read password: %w", err)
	}

	// Verify credentials
	success := string(username) == s.config.Username && string(password) == s.config.Password

	// Send authentication response
	// +----+--------+
	// |VER | STATUS |
	// +----+--------+
	// | 1  |   1    |
	// +----+--------+
	status := byte(0x00)
	if !success {
		status = 0x01
	}

	if _, err := conn.Write([]byte{0x01, status}); err != nil {
		return fmt.Errorf("write auth response: %w", err)
	}

	if !success {
		return fmt.Errorf("authentication failed")
	}

	return nil
}

// handleRequest processes the SOCKS5 request (CONNECT command)
func (s *SOCKS5Server) handleRequest(conn net.Conn) (string, uint16, error) {
	// Read request
	// +----+-----+-------+------+----------+----------+
	// |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
	// +----+-----+-------+------+----------+----------+
	// | 1  |  1  | X'00' |  1   | Variable |    2     |
	// +----+-----+-------+------+----------+----------+

	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		s.sendReply(conn, replyGeneralFailure)
		return "", 0, fmt.Errorf("read request header: %w", err)
	}

	version := buf[0]
	cmd := buf[1]
	atyp := buf[3]

	if version != socks5Version {
		s.sendReply(conn, replyGeneralFailure)
		return "", 0, fmt.Errorf("invalid version: %d", version)
	}

	if cmd != cmdConnect {
		s.sendReply(conn, replyCommandNotSupported)
		return "", 0, fmt.Errorf("unsupported command: %d", cmd)
	}

	// Read destination address
	var dstAddr string
	switch atyp {
	case atypIPv4:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			s.sendReply(conn, replyGeneralFailure)
			return "", 0, fmt.Errorf("read IPv4 address: %w", err)
		}
		dstAddr = net.IP(addr).String()

	case atypDomain:
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			s.sendReply(conn, replyGeneralFailure)
			return "", 0, fmt.Errorf("read domain length: %w", err)
		}
		domainLen := buf[0]
		domain := make([]byte, domainLen)
		if _, err := io.ReadFull(conn, domain); err != nil {
			s.sendReply(conn, replyGeneralFailure)
			return "", 0, fmt.Errorf("read domain: %w", err)
		}
		dstAddr = string(domain)

	case atypIPv6:
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			s.sendReply(conn, replyGeneralFailure)
			return "", 0, fmt.Errorf("read IPv6 address: %w", err)
		}
		dstAddr = net.IP(addr).String()

	default:
		s.sendReply(conn, replyAddressNotSupported)
		return "", 0, fmt.Errorf("unsupported address type: %d", atyp)
	}

	// Read destination port
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		s.sendReply(conn, replyGeneralFailure)
		return "", 0, fmt.Errorf("read port: %w", err)
	}
	dstPort := binary.BigEndian.Uint16(portBuf)

	// Send success reply
	if err := s.sendReply(conn, replySuccess); err != nil {
		return "", 0, fmt.Errorf("send reply: %w", err)
	}

	return dstAddr, dstPort, nil
}

// sendReply sends a SOCKS5 reply message
func (s *SOCKS5Server) sendReply(conn net.Conn, rep byte) error {
	// +----+-----+-------+------+----------+----------+
	// |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
	// +----+-----+-------+------+----------+----------+
	// | 1  |  1  | X'00' |  1   | Variable |    2     |
	// +----+-----+-------+------+----------+----------+

	// Use IPv4 0.0.0.0:0 for bind address
	reply := []byte{
		socks5Version,
		rep,
		0x00,        // Reserved
		atypIPv4,    // Address type
		0, 0, 0, 0,  // Bind address (0.0.0.0)
		0, 0,        // Bind port (0)
	}

	_, err := conn.Write(reply)
	return err
}

// handleTunnel processes HTTP requests through the SOCKS5 tunnel
func (s *SOCKS5Server) handleTunnel(conn net.Conn, targetAddr string, targetPort uint16) {
	// Read HTTP request from tunnel
	reader := bufio.NewReader(conn)

	for {
		// Read HTTP request
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				log.Printf("SOCKS5 read request error: %v", err)
			}
			return
		}

		// Set request URL scheme and host
		if targetPort == 443 {
			req.URL.Scheme = "https"
		} else {
			req.URL.Scheme = "http"
		}
		req.URL.Host = fmt.Sprintf("%s:%d", targetAddr, targetPort)

		// Ensure Host header is set
		if req.Host == "" {
			req.Host = targetAddr
		}

		// Create a response recorder to capture the response
		rec := newResponseRecorder()

		// Pass request to ResponseHandler
		s.responseHandler.HandleRequest(rec, req)

		// Write response back through tunnel
		if err := s.writeResponse(conn, rec); err != nil {
			log.Printf("SOCKS5 write response error: %v", err)
			return
		}

		// Check if connection should be closed
		if req.Header.Get("Connection") == "close" || rec.Header().Get("Connection") == "close" {
			return
		}
	}
}

// writeResponse writes an HTTP response to the connection
func (s *SOCKS5Server) writeResponse(conn net.Conn, rec *responseRecorder) error {
	var buf bytes.Buffer

	// Write status line
	statusCode := rec.statusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	statusText := http.StatusText(statusCode)
	fmt.Fprintf(&buf, "HTTP/1.1 %d %s\r\n", statusCode, statusText)

	// Write headers
	for key, values := range rec.Header() {
		for _, value := range values {
			fmt.Fprintf(&buf, "%s: %s\r\n", key, value)
		}
	}

	// Write body
	buf.WriteString("\r\n")
	buf.Write(rec.body.Bytes())

	// Write to connection
	_, err := conn.Write(buf.Bytes())
	return err
}

// responseRecorder captures HTTP responses for SOCKS5 tunneling
type responseRecorder struct {
	statusCode int
	header     http.Header
	body       *bytes.Buffer
}

// newResponseRecorder creates a new response recorder
func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		header: make(http.Header),
		body:   &bytes.Buffer{},
	}
}

// Header returns the response headers
func (r *responseRecorder) Header() http.Header {
	return r.header
}

// Write writes data to the response body
func (r *responseRecorder) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

// WriteHeader sets the response status code
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}
