package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
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
	cmdConnect      = 0x01
	cmdBind         = 0x02
	cmdUDPAssociate = 0x03

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
	config          *models.SOCKS5Config
	listener        net.Listener
	responseHandler *ResponseHandler
	tlsInterceptor  *TLSInterceptor             // TLS interception for HTTPS connections
	domainTakeover  *models.DomainTakeoverConfig // Domain takeover config for intercept decisions
	requestLogger   RequestLogger                // For logging SOCKS5 requests (observational)
	udpRelay        *UDPRelay                    // UDP relay for DNS over SOCKS5
	dnsResolver     *DNSResolver                 // DNS resolver for overrides
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	running         bool
	mu              sync.Mutex
}

// NewSOCKS5Server creates a new SOCKS5 server instance
// Parameters:
//   - config: SOCKS5 server configuration (port, auth, etc.)
//   - handler: ResponseHandler for processing intercepted requests
//   - certCache: Certificate cache for TLS interception (nil disables TLS interception)
//   - domainTakeover: Domain takeover config to determine which domains to intercept
//   - dnsResolver: DNS resolver for UDP ASSOCIATE DNS queries
//   - logger: RequestLogger for logging SOCKS5 requests (observational only)
func NewSOCKS5Server(config *models.SOCKS5Config, handler *ResponseHandler, certCache *CertCache, domainTakeover *models.DomainTakeoverConfig, dnsResolver *DNSResolver, logger RequestLogger) *SOCKS5Server {
	ctx, cancel := context.WithCancel(context.Background())

	var tlsInterceptor *TLSInterceptor
	if certCache != nil {
		tlsInterceptor = NewTLSInterceptor(certCache)
		log.Println("SOCKS5 TLS interception enabled")
	}

	// Initialize UDP relay for DNS over SOCKS5
	var udpRelay *UDPRelay
	if dnsResolver != nil {
		relay, err := NewUDPRelay(dnsResolver)
		if err != nil {
			log.Printf("Warning: Failed to create UDP relay: %v", err)
		} else {
			udpRelay = relay
			log.Printf("SOCKS5 UDP ASSOCIATE enabled for DNS on port %d", relay.GetAddress().Port)
		}
	} else {
		log.Printf("Warning: DNS resolver is nil, UDP ASSOCIATE will not be available")
	}

	return &SOCKS5Server{
		config:          config,
		responseHandler: handler,
		tlsInterceptor:  tlsInterceptor,
		domainTakeover:  domainTakeover,
		requestLogger:   logger,
		udpRelay:        udpRelay,
		dnsResolver:     dnsResolver,
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

	// Start UDP relay if available
	if s.udpRelay != nil {
		s.udpRelay.Start()
	}

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

	// Stop UDP relay if running
	if s.udpRelay != nil {
		s.udpRelay.Stop()
	}

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

// GetUDPRelay returns the UDP relay instance
func (s *SOCKS5Server) GetUDPRelay() *UDPRelay {
	return s.udpRelay
}

// UpdateDomainTakeover updates the domain takeover configuration
func (s *SOCKS5Server) UpdateDomainTakeover(domainTakeover *models.DomainTakeoverConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.domainTakeover = domainTakeover
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

	// Check if this is a UDP ASSOCIATE request
	if targetAddr == "udp-associate" {
		// Keep the TCP connection alive as a control channel
		// It will be closed when the client disconnects
		log.Printf("SOCKS5 UDP ASSOCIATE control channel established (relay port: %d)", targetPort)

		// Read from connection until closed (control channel)
		buf := make([]byte, 1)
		for {
			if _, err := conn.Read(buf); err != nil {
				log.Printf("SOCKS5 UDP ASSOCIATE control channel closed")
				break
			}
		}
		return
	}

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

	// Handle UDP ASSOCIATE command
	if cmd == cmdUDPAssociate {
		return s.handleUDPAssociate(conn, atyp, buf[3:])
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

// handleUDPAssociate handles UDP ASSOCIATE requests for DNS over SOCKS5
func (s *SOCKS5Server) handleUDPAssociate(conn net.Conn, atyp byte, header []byte) (string, uint16, error) {
	// UDP ASSOCIATE is used for DNS queries through SOCKS5
	// We need to provide a UDP relay address for the client to send UDP packets to

	if s.udpRelay == nil {
		s.sendReply(conn, replyCommandNotSupported)
		return "", 0, fmt.Errorf("UDP ASSOCIATE not supported (UDP relay not initialized)")
	}

	// The client sends the address it will use as source for UDP packets
	// We don't really need to parse it for our use case
	// We just need to send back our UDP relay address

	// Get UDP relay address
	relayAddr := s.udpRelay.GetAddress()

	// Send successful reply with UDP relay address
	reply := []byte{
		socks5Version,
		replySuccess,
		0x00, // RSV
		atypIPv4,
	}

	// Add IP address (use 0.0.0.0 to indicate same host)
	reply = append(reply, 0, 0, 0, 0)

	// Add port
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(relayAddr.Port))
	reply = append(reply, portBytes...)

	if _, err := conn.Write(reply); err != nil {
		return "", 0, fmt.Errorf("failed to send UDP ASSOCIATE reply: %w", err)
	}

	log.Printf("SOCKS5 UDP ASSOCIATE established - relay on port %d", relayAddr.Port)

	// Keep the TCP connection open (it's used as a control channel)
	// The connection will be closed when the client disconnects
	// We'll handle this in handleConnection

	// Return special marker to indicate UDP ASSOCIATE mode
	return "udp-associate", uint16(relayAddr.Port), nil
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

// shouldIntercept checks if a domain should be intercepted based on domain takeover config
// Returns true if the domain matches any enabled domain in the takeover list
func (s *SOCKS5Server) shouldIntercept(domain string) bool {
	if s.domainTakeover == nil {
		return false
	}

	for _, domainConfig := range s.domainTakeover.Domains {
		if !domainConfig.Enabled {
			continue
		}

		// Check if domain matches the pattern (exact match for now)
		// TODO: Add wildcard/regex matching if needed
		if domain == domainConfig.Pattern {
			return true
		}
	}

	return false
}

// extractWSCloseInfo extracts the WebSocket close code and reason from a relay error.
// Returns (0, "") for non-close errors (TCP reset, timeout, etc.).
func extractWSCloseInfo(err error) (code int, reason string) {
	if err == nil {
		return websocket.CloseNormalClosure, ""
	}
	if ce, ok := err.(*websocket.CloseError); ok {
		return ce.Code, ce.Text
	}
	return 0, ""
}

// writeSimResponse writes a plain HTTP error response to conn based on the
// overlay simulation config.  Used when a WebSocket upgrade is intercepted
// before HandleRequest runs (which is where the normal sim-mode check lives).
func writeSimResponse(conn net.Conn, cfg models.OverlaySimConfig) {
	switch cfg.Mode {
	case OverlaySimTimeout:
		secs := cfg.TimeoutSecs
		if secs <= 0 {
			secs = 30
		}
		time.Sleep(time.Duration(secs) * time.Second)
		fmt.Fprintf(conn, "HTTP/1.1 504 Gateway Timeout\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
	case OverlaySimDNSError:
		fmt.Fprintf(conn, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
	case OverlaySimOther:
		code := cfg.StatusCode
		if code < 100 || code > 599 {
			code = http.StatusBadGateway
		}
		fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
			code, http.StatusText(code))
	}
}

// isWebSocketUpgrade returns true when r carries a WebSocket upgrade request.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Upgrade")) == "websocket" &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// resolveTarget resolves a domain to an IP using the server's DNS resolver.
// Returns the original value if it is already an IP or resolution fails.
func (s *SOCKS5Server) resolveTarget(targetAddr string) string {
	if net.ParseIP(targetAddr) != nil {
		return targetAddr
	}
	if s.dnsResolver != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ips, err := s.dnsResolver.Resolve(ctx, targetAddr)
		if err != nil || len(ips) == 0 {
			return targetAddr
		}
		return ips[0]
	}
	return targetAddr
}

// prefixConn wraps a net.Conn, prepending bytes that were already buffered in a
// bufio.Reader before the connection was handed to the WebSocket upgrader.
// All net.Conn methods (Write, Close, SetDeadline, …) delegate to the inner Conn.
type prefixConn struct {
	net.Conn
	prefix *bytes.Reader // drained-ahead bytes; nil once exhausted
}

func (c *prefixConn) Read(p []byte) (int, error) {
	if c.prefix != nil && c.prefix.Len() > 0 {
		n, _ := c.prefix.Read(p)
		if n > 0 {
			return n, nil // serve prefix bytes; fall through to Conn once exhausted
		}
	}
	c.prefix = nil
	return c.Conn.Read(p)
}

// socks5ResponseHijacker wraps a raw net.Conn + bufio.Reader so that the gorilla
// WebSocket upgrader can perform the HTTP→WS upgrade inside a SOCKS5 tunnel where
// there is no real http.ResponseWriter.
type socks5ResponseHijacker struct {
	conn   net.Conn
	reader *bufio.Reader
	header http.Header
}

func (h *socks5ResponseHijacker) Header() http.Header { return h.header }
func (h *socks5ResponseHijacker) Write(b []byte) (int, error) { return h.conn.Write(b) }
func (h *socks5ResponseHijacker) WriteHeader(_ int)           {}
func (h *socks5ResponseHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	// gorilla's Upgrade() does two things that interact with what we return here:
	//
	//   1. Checks brw.Reader.Buffered() > 0 → closes conn if true.
	//   2. Calls bufioReaderSize(netConn, brw.Reader) which internally calls
	//      brw.Reader.Reset(netConn).  This replaces whatever the bufio.Reader was
	//      wrapping with netConn (the first return value).
	//
	// A previous fix drained h.reader's buffered bytes into an io.MultiReader and
	// returned h.conn as netConn.  That was still broken: step 2 resets brw.Reader
	// to h.conn, discarding the MultiReader and losing the drained bytes.
	//
	// Correct fix: if there are buffered bytes, return a *prefixConn as netConn.
	// Then step 2 resets brw.Reader to the *prefixConn — whose Read() still serves
	// the drained bytes first before falling through to the real connection.
	// Writes and all other net.Conn operations delegate to the inner conn unchanged.
	netConn := net.Conn(h.conn)
	if n := h.reader.Buffered(); n > 0 {
		buf := make([]byte, n)
		h.reader.Read(buf) // reads exactly n in-memory bytes — no new I/O
		netConn = &prefixConn{Conn: h.conn, prefix: bytes.NewReader(buf)}
	}
	brw := bufio.NewReadWriter(bufio.NewReaderSize(netConn, 4096), bufio.NewWriter(h.conn))
	return netConn, brw, nil
}

// forwardWSHeaders builds the header set to send with the backend WebSocket dial.
// We forward headers that backends commonly need for auth and subprotocol selection
// while excluding WebSocket handshake headers that gorilla generates itself.
func forwardWSHeaders(req *http.Request) http.Header {
	skip := map[string]bool{
		"Upgrade":                  true,
		"Connection":               true,
		"Sec-Websocket-Key":        true,
		"Sec-Websocket-Version":    true,
		"Sec-Websocket-Extensions": true,
	}
	h := http.Header{}
	for k, vals := range req.Header {
		if skip[http.CanonicalHeaderKey(k)] {
			continue
		}
		h[http.CanonicalHeaderKey(k)] = vals
	}
	return h
}

// forwardBackendWSResponseHeaders extracts backend WebSocket upgrade response
// headers that must be forwarded to the client. This is critical for protocol
// negotiation (e.g. Sec-WebSocket-Protocol: sip) — without it clients like SIP
// phones close immediately because the subprotocol handshake is incomplete.
func forwardBackendWSResponseHeaders(resp *http.Response) http.Header {
	if resp == nil {
		return nil
	}
	skip := map[string]bool{
		"Upgrade":              true,
		"Connection":           true,
		"Sec-Websocket-Accept": true, // gorilla computes this itself from the client key
	}
	h := http.Header{}
	for k, vals := range resp.Header {
		if skip[http.CanonicalHeaderKey(k)] {
			continue
		}
		h[http.CanonicalHeaderKey(k)] = vals
	}
	return h
}

// makeWSEvent creates a WebSocketEvent from a gorilla message type + payload.
// captureBytes controls how much data is stored in DataPreview (0 = use default 1024).
func makeWSEvent(msgType int, data []byte, direction string, connStart time.Time, captureBytes int) models.WebSocketEvent {
	if captureBytes <= 0 {
		captureBytes = 1024
	}

	opcode := models.WSOpcodeText
	switch msgType {
	case websocket.TextMessage:
		opcode = models.WSOpcodeText
	case websocket.BinaryMessage:
		opcode = models.WSOpcodeBinary
	case websocket.PingMessage:
		opcode = models.WSOpcodePing
	case websocket.PongMessage:
		opcode = models.WSOpcodePong
	case websocket.CloseMessage:
		opcode = models.WSOpcodeClose
	}

	event := models.WebSocketEvent{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().Format(time.RFC3339Nano),
		OffsetMs:  time.Since(connStart).Milliseconds(),
		Direction: direction,
		Opcode:    opcode,
		DataSize:  len(data),
	}

	switch msgType {
	case websocket.TextMessage:
		if len(data) > 0 {
			captured := data
			truncated := len(data) > captureBytes
			if truncated {
				captured = data[:captureBytes]
			}
			event.DataPreview = string(captured)
			if truncated {
				event.DataPreview += fmt.Sprintf("\n… [%d bytes total, showing first %d]", len(data), captureBytes)
			}
		}
	case websocket.BinaryMessage:
		if len(data) > 0 {
			captured := data
			truncated := len(data) > captureBytes
			if truncated {
				captured = data[:captureBytes]
			}
			event.DataPreview = hex.EncodeToString(captured)
			if truncated {
				event.DataPreview += fmt.Sprintf("\n… [%d bytes total, showing first %d as hex]", len(data), captureBytes)
			}
		}
	case websocket.CloseMessage:
		if len(data) >= 2 {
			event.CloseCode = int(data[0])<<8 | int(data[1])
			if len(data) > 2 {
				event.CloseText = string(data[2:])
			}
		}
	}

	return event
}

// handleInterceptedWebSocket relays a WebSocket connection that was detected inside
// an already-TLS-terminated SOCKS5 tunnel (port 443 / intercepted domain).
func (s *SOCKS5Server) handleInterceptedWebSocket(clientConn net.Conn, clientReader *bufio.Reader, req *http.Request, targetAddr string, targetPort uint16) {
	connID := fmt.Sprintf("ws-%d", time.Now().UnixNano())
	startTime := time.Now()
	status101 := http.StatusSwitchingProtocols

	endpointID := s.responseHandler.FindEndpointID(req)

	// ── Step 1: dial the backend FIRST ───────────────────────────────────────
	// Dialling before upgrading the client means a backend failure sends HTTP 502
	// rather than a WebSocket close frame (client never sees onopen+onclose).
	backendURL := fmt.Sprintf("wss://%s:%d%s", targetAddr, targetPort, req.URL.Path)
	if req.URL.RawQuery != "" {
		backendURL += "?" + req.URL.RawQuery
	}
	backendHeaders := forwardWSHeaders(req)
	log.Printf("SOCKS5 WSS [%s] dialling backend %s (subprotocol: %s)",
		targetAddr, backendURL, req.Header.Get("Sec-Websocket-Protocol"))
	backendWS, backendResp, err := websocket.DefaultDialer.Dial(backendURL, backendHeaders)
	if err != nil {
		log.Printf("SOCKS5 WSS [%s] backend dial FAILED: %v", targetAddr, err)
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"))
		return
	}
	defer backendWS.Close()
	log.Printf("SOCKS5 WSS [%s] backend connected (subprotocol: %s)",
		targetAddr, backendResp.Header.Get("Sec-Websocket-Protocol"))

	// ── Step 2: upgrade the client, forwarding backend response headers ──────
	// Critical: forward Sec-WebSocket-Protocol and any other backend response
	// headers to the client. Without this, SIP phones (and other protocol-
	// specific WebSocket clients) close immediately because the subprotocol
	// negotiation response is missing.
	clientRespHeaders := forwardBackendWSResponseHeaders(backendResp)
	log.Printf("SOCKS5 WSS [%s] upgrading client (buffered=%d, forwarding headers: %v)",
		targetAddr, clientReader.Buffered(), clientRespHeaders)
	hijacker := &socks5ResponseHijacker{conn: clientConn, reader: clientReader, header: make(http.Header)}
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clientWS, err := upgrader.Upgrade(hijacker, req, clientRespHeaders)
	if err != nil {
		log.Printf("SOCKS5 WSS [%s] client upgrade FAILED: %v", targetAddr, err)
		return
	}
	defer clientWS.Close()
	log.Printf("SOCKS5 WSS [%s] tunnel established", targetAddr)

	// ── Step 3: log the completed handshake with full backend details ─────────
	if s.requestLogger != nil {
		wsLog := models.RequestLog{
			ID:          connID,
			Timestamp:   startTime.Format(time.RFC3339),
			EndpointID:  endpointID,
			IsWebSocket: true,
			SOCKS5Info: &models.SOCKS5RequestInfo{
				TargetHost:    targetAddr,
				TargetPort:    int(targetPort),
				Protocol:      "HTTPS",
				IsIntercepted: true,
			},
		}
		wsLog.ClientRequest.Method = req.Method
		wsLog.ClientRequest.FullURL = fmt.Sprintf("wss://%s:%d%s", targetAddr, targetPort, req.URL.Path)
		wsLog.ClientRequest.Path = req.URL.Path
		wsLog.ClientRequest.Headers = map[string][]string(req.Header)

		wsLog.ClientResponse.StatusCode = &status101
		wsLog.ClientResponse.StatusText = "Switching Protocols"
		wsLog.ClientResponse.Headers = map[string][]string(clientRespHeaders)

		backendReqEntry := struct {
			Method      string              `json:"method"`
			FullURL     string              `json:"full_url"`
			Path        string              `json:"path"`
			QueryParams map[string][]string `json:"query_params,omitempty"`
			Headers     map[string][]string `json:"headers,omitempty"`
			Body        string              `json:"body,omitempty"`
		}{
			Method:  req.Method,
			FullURL: backendURL,
			Path:    req.URL.Path,
			Headers: map[string][]string(backendHeaders),
		}
		wsLog.BackendRequest = &backendReqEntry

		backendStatus := backendResp.StatusCode
		backendRespEntry := struct {
			StatusCode *int                `json:"status_code,omitempty"`
			StatusText string              `json:"status_text,omitempty"`
			Headers    map[string][]string `json:"headers,omitempty"`
			Body       string              `json:"body,omitempty"`
			DelayMs    *int64              `json:"delay_ms,omitempty"`
			RTTMs      *int64              `json:"rtt_ms,omitempty"`
		}{
			StatusCode: &backendStatus,
			StatusText: backendResp.Status,
			Headers:    map[string][]string(backendResp.Header),
		}
		wsLog.BackendResponse = &backendRespEntry

		s.requestLogger.LogRequest(wsLog)
	}

	// ── Step 4: relay frames bidirectionally ─────────────────────────────────
	captureBytes := 1024
	if s.requestLogger != nil {
		captureBytes = s.requestLogger.GetWSCaptureBytes()
	}

	var blocked atomic.Bool // false = relay frames; true = read-and-drop
	if s.requestLogger != nil {
		s.requestLogger.RegisterWSConnection(connID,
			func() { clientWS.Close(); backendWS.Close() },
			func(b bool) { blocked.Store(b) },
		)
	}

	errChan := make(chan error, 2)

	// Client → Backend
	go func() {
		for {
			msgType, msg, err := clientWS.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if !blocked.Load() {
				if err := backendWS.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
			if s.requestLogger != nil {
				s.requestLogger.AppendWebSocketEvent(connID, makeWSEvent(msgType, msg, models.WSDirectionSend, startTime, captureBytes))
			}
		}
	}()

	// Backend → Client
	go func() {
		for {
			msgType, msg, err := backendWS.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if !blocked.Load() {
				if err := clientWS.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
			if s.requestLogger != nil {
				s.requestLogger.AppendWebSocketEvent(connID, makeWSEvent(msgType, msg, models.WSDirectionRecv, startTime, captureBytes))
			}
		}
	}()

	relayErr := <-errChan
	clientWS.Close()
	backendWS.Close()
	<-errChan

	closeCode, closeReason := extractWSCloseInfo(relayErr)
	log.Printf("SOCKS5 WSS [%s] tunnel closed after %.1fs: %v (code=%d)", targetAddr, time.Since(startTime).Seconds(), relayErr, closeCode)
	if s.requestLogger != nil {
		s.requestLogger.CloseWebSocketConnection(connID, closeCode, closeReason, relayErr)
	}
}

// handlePlainWebSocket relays a WebSocket connection detected inside a plain-HTTP SOCKS5 tunnel.
func (s *SOCKS5Server) handlePlainWebSocket(clientConn net.Conn, clientReader *bufio.Reader, req *http.Request, targetAddr string, targetPort uint16) {
	connID := fmt.Sprintf("ws-%d", time.Now().UnixNano())
	startTime := time.Now()
	status101 := http.StatusSwitchingProtocols

	endpointID := s.responseHandler.FindEndpointID(req)

	// ── Step 1: dial backend first ────────────────────────────────────────────
	backendURL := fmt.Sprintf("ws://%s:%d%s", targetAddr, targetPort, req.URL.Path)
	if req.URL.RawQuery != "" {
		backendURL += "?" + req.URL.RawQuery
	}
	backendHeaders := forwardWSHeaders(req)
	log.Printf("SOCKS5 WS [%s] dialling backend %s (subprotocol: %s)",
		targetAddr, backendURL, req.Header.Get("Sec-Websocket-Protocol"))
	backendWS, backendResp, err := websocket.DefaultDialer.Dial(backendURL, backendHeaders)
	if err != nil {
		log.Printf("SOCKS5 WS [%s] backend dial FAILED: %v", targetAddr, err)
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"))
		return
	}
	defer backendWS.Close()
	log.Printf("SOCKS5 WS [%s] backend connected (subprotocol: %s)",
		targetAddr, backendResp.Header.Get("Sec-Websocket-Protocol"))

	// ── Step 2: upgrade client, forwarding backend response headers ───────────
	clientRespHeaders := forwardBackendWSResponseHeaders(backendResp)
	log.Printf("SOCKS5 WS [%s] upgrading client (buffered=%d, forwarding headers: %v)",
		targetAddr, clientReader.Buffered(), clientRespHeaders)
	hijacker := &socks5ResponseHijacker{conn: clientConn, reader: clientReader, header: make(http.Header)}
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clientWS, err := upgrader.Upgrade(hijacker, req, clientRespHeaders)
	if err != nil {
		log.Printf("SOCKS5 WS [%s] client upgrade FAILED: %v", targetAddr, err)
		return
	}
	defer clientWS.Close()
	log.Printf("SOCKS5 WS [%s] tunnel established", targetAddr)

	// ── Step 3: log completed handshake ──────────────────────────────────────
	if s.requestLogger != nil {
		wsLog := models.RequestLog{
			ID:          connID,
			Timestamp:   startTime.Format(time.RFC3339),
			EndpointID:  endpointID,
			IsWebSocket: true,
			SOCKS5Info: &models.SOCKS5RequestInfo{
				TargetHost: targetAddr,
				TargetPort: int(targetPort),
				Protocol:   "HTTP",
			},
		}
		wsLog.ClientRequest.Method = req.Method
		wsLog.ClientRequest.FullURL = fmt.Sprintf("ws://%s:%d%s", targetAddr, targetPort, req.URL.Path)
		wsLog.ClientRequest.Path = req.URL.Path
		wsLog.ClientRequest.Headers = map[string][]string(req.Header)
		wsLog.ClientResponse.StatusCode = &status101
		wsLog.ClientResponse.StatusText = "Switching Protocols"
		wsLog.ClientResponse.Headers = map[string][]string(clientRespHeaders)

		backendReqEntry := struct {
			Method      string              `json:"method"`
			FullURL     string              `json:"full_url"`
			Path        string              `json:"path"`
			QueryParams map[string][]string `json:"query_params,omitempty"`
			Headers     map[string][]string `json:"headers,omitempty"`
			Body        string              `json:"body,omitempty"`
		}{
			Method:  req.Method,
			FullURL: backendURL,
			Path:    req.URL.Path,
			Headers: map[string][]string(backendHeaders),
		}
		wsLog.BackendRequest = &backendReqEntry

		backendStatus := backendResp.StatusCode
		backendRespEntry := struct {
			StatusCode *int                `json:"status_code,omitempty"`
			StatusText string              `json:"status_text,omitempty"`
			Headers    map[string][]string `json:"headers,omitempty"`
			Body       string              `json:"body,omitempty"`
			DelayMs    *int64              `json:"delay_ms,omitempty"`
			RTTMs      *int64              `json:"rtt_ms,omitempty"`
		}{
			StatusCode: &backendStatus,
			StatusText: backendResp.Status,
			Headers:    map[string][]string(backendResp.Header),
		}
		wsLog.BackendResponse = &backendRespEntry

		s.requestLogger.LogRequest(wsLog)
	}

	// ── Step 4: relay frames ─────────────────────────────────────────────────
	captureBytes := 1024
	if s.requestLogger != nil {
		captureBytes = s.requestLogger.GetWSCaptureBytes()
	}

	var blocked atomic.Bool
	if s.requestLogger != nil {
		s.requestLogger.RegisterWSConnection(connID,
			func() { clientWS.Close(); backendWS.Close() },
			func(b bool) { blocked.Store(b) },
		)
	}

	errChan := make(chan error, 2)

	go func() {
		for {
			msgType, msg, err := clientWS.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if !blocked.Load() {
				if err := backendWS.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
			if s.requestLogger != nil {
				s.requestLogger.AppendWebSocketEvent(connID, makeWSEvent(msgType, msg, models.WSDirectionSend, startTime, captureBytes))
			}
		}
	}()

	go func() {
		for {
			msgType, msg, err := backendWS.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if !blocked.Load() {
				if err := clientWS.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
			if s.requestLogger != nil {
				s.requestLogger.AppendWebSocketEvent(connID, makeWSEvent(msgType, msg, models.WSDirectionRecv, startTime, captureBytes))
			}
		}
	}()

	relayErr := <-errChan
	clientWS.Close()
	backendWS.Close()
	<-errChan

	closeCode, closeReason := extractWSCloseInfo(relayErr)
	log.Printf("SOCKS5 WS [%s] tunnel closed after %.1fs: %v (code=%d)", targetAddr, time.Since(startTime).Seconds(), relayErr, closeCode)
	if s.requestLogger != nil {
		s.requestLogger.CloseWebSocketConnection(connID, closeCode, closeReason, relayErr)
	}
}

// handleTunnel processes HTTP/HTTPS requests through the SOCKS5 tunnel
// For HTTPS (port 443):
//   - If domain is in takeover list: TLS intercept → ResponseHandler
//   - If domain NOT in takeover list: Pass-through to real server
func (s *SOCKS5Server) handleTunnel(conn net.Conn, targetAddr string, targetPort uint16) {
	isHTTPS := targetPort == 443

	// For HTTPS connections, decide: intercept or pass-through
	if isHTTPS {
		if s.shouldIntercept(targetAddr) && s.tlsInterceptor != nil {
			// Domain is in takeover list - TLS intercept and handle with ResponseHandler
			s.handleInterceptedHTTPS(conn, targetAddr, targetPort)
		} else {
			// Domain NOT in takeover list - pass-through to real server
			s.handlePassthrough(conn, targetAddr, targetPort)
		}
		return
	}

	// For HTTP connections, handle directly with ResponseHandler
	s.handleHTTP(conn, targetAddr, targetPort)
}

// handleInterceptedHTTPS performs TLS interception for domains in the takeover list
// Performs TLS handshake with client, then reads decrypted HTTP requests
func (s *SOCKS5Server) handleInterceptedHTTPS(conn net.Conn, targetAddr string, targetPort uint16) {
	// Perform TLS handshake with the client
	tlsConn, err := s.tlsInterceptor.Intercept(conn, targetAddr)
	if err != nil {
		log.Printf("SOCKS5 TLS interception failed for %s: %v", targetAddr, err)
		// Fall back to pass-through on TLS error
		// Note: Connection may be in bad state, so this might fail
		return
	}
	defer tlsConn.Close()

	log.Printf("SOCKS5 TLS intercepted: %s:%d", targetAddr, targetPort)

	// Log intercepted HTTPS connection (connection-level only)
	// Individual HTTP requests are logged by the overlay endpoint handler
	if s.requestLogger != nil {
		requestLog := models.RequestLog{
			ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
			Timestamp:  time.Now().Format(time.RFC3339),
			EndpointID: "system-socks5-proxy",
			SOCKS5Info: &models.SOCKS5RequestInfo{
				TargetHost:    targetAddr,
				TargetPort:    int(targetPort),
				Protocol:      "HTTPS",
				IsIntercepted: true,
			},
		}
		requestLog.ClientRequest.Method = "CONNECT"
		requestLog.ClientRequest.FullURL = fmt.Sprintf("https://%s:%d", targetAddr, targetPort)
		requestLog.ClientRequest.Path = fmt.Sprintf("%s:%d", targetAddr, targetPort)
		s.requestLogger.LogRequest(requestLog)
	}

	// Read HTTP requests from the TLS-wrapped connection
	reader := bufio.NewReader(tlsConn)

	for {
		// Read HTTP request (now decrypted)
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				log.Printf("SOCKS5 read request error (intercepted): %v", err)
			}
			return
		}

		// Set request URL scheme and host
		req.URL.Scheme = "https"
		req.URL.Host = fmt.Sprintf("%s:%d", targetAddr, targetPort)

		// Ensure Host header is set
		if req.Host == "" {
			req.Host = targetAddr
		}

		// WebSocket upgrade: check simulation mode first, then hand off.
		// HandleRequest is bypassed for WS connections, so the sim check must happen here.
		if isWebSocketUpgrade(req) {
			if cfg, ok := s.responseHandler.CheckOverlaySimMode(req); ok {
				log.Printf("SOCKS5 WSS [%s] simulation mode %q applied to WS upgrade", targetAddr, cfg.Mode)
				writeSimResponse(tlsConn, cfg)
				return
			}
			s.handleInterceptedWebSocket(tlsConn, reader, req, targetAddr, targetPort)
			return
		}

		// Create a response recorder to capture the response
		rec := newResponseRecorder()

		// Pass request to ResponseHandler
		s.responseHandler.HandleRequest(rec, req)

		// Write response back through TLS tunnel
		if err := s.writeResponse(tlsConn, rec); err != nil {
			log.Printf("SOCKS5 write response error (intercepted): %v", err)
			return
		}

		// Check if connection should be closed
		if req.Header.Get("Connection") == "close" || rec.Header().Get("Connection") == "close" {
			return
		}
	}
}

// handlePassthrough connects to the real server and forwards raw bytes
// Used for domains NOT in the takeover list (Option A - pass-through mode)
func (s *SOCKS5Server) handlePassthrough(conn net.Conn, targetAddr string, targetPort uint16) {
	// Resolve domain if needed (using our DNS resolver for override support)
	var resolvedAddr string
	if net.ParseIP(targetAddr) != nil {
		// Already an IP address
		resolvedAddr = targetAddr
	} else {
		// It's a domain, resolve it
		if s.dnsResolver != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ips, err := s.dnsResolver.Resolve(ctx, targetAddr)
			if err != nil {
				log.Printf("SOCKS5 pass-through: DNS resolution failed for %s: %v", targetAddr, err)
				return
			}
			if len(ips) == 0 {
				log.Printf("SOCKS5 pass-through: No IPs found for %s", targetAddr)
				return
			}
			resolvedAddr = ips[0]
			if resolvedAddr != targetAddr {
				log.Printf("SOCKS5 pass-through: Resolved %s to %s", targetAddr, resolvedAddr)
			}
		} else {
			// Fallback to system resolver
			resolvedAddr = targetAddr
		}
	}

	// Connect to the real destination
	destAddr := fmt.Sprintf("%s:%d", resolvedAddr, targetPort)
	destConn, err := net.DialTimeout("tcp", destAddr, 30*time.Second)
	if err != nil {
		log.Printf("SOCKS5 pass-through: failed to connect to %s: %v", destAddr, err)
		return
	}
	defer destConn.Close()

	log.Printf("SOCKS5 pass-through: %s (not in takeover list)", destAddr)

	// Log pass-through connection (metadata only, no bodies)
	if s.requestLogger != nil {
		requestLog := models.RequestLog{
			ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
			Timestamp:  time.Now().Format(time.RFC3339),
			EndpointID: "system-socks5-proxy",
			SOCKS5Info: &models.SOCKS5RequestInfo{
				TargetHost:    targetAddr,
				TargetPort:    int(targetPort),
				Protocol:      "PASS-THROUGH",
				IsIntercepted: false,
			},
		}
		requestLog.ClientRequest.Method = "CONNECT"
		requestLog.ClientRequest.FullURL = fmt.Sprintf("%s:%d", targetAddr, targetPort)
		requestLog.ClientRequest.Path = fmt.Sprintf("%s:%d", targetAddr, targetPort)
		s.requestLogger.LogRequest(requestLog)
	}

	// Set up bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	// Client → Destination
	go func() {
		defer wg.Done()
		io.Copy(destConn, conn)
		// Signal EOF to destination
		if tcpConn, ok := destConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// Destination → Client
	go func() {
		defer wg.Done()
		io.Copy(conn, destConn)
		// Signal EOF to client
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	wg.Wait()
}

// handleHTTP processes HTTP (non-HTTPS) requests through the SOCKS5 tunnel
func (s *SOCKS5Server) handleHTTP(conn net.Conn, targetAddr string, targetPort uint16) {
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
		req.URL.Scheme = "http"
		req.URL.Host = fmt.Sprintf("%s:%d", targetAddr, targetPort)

		// Ensure Host header is set
		if req.Host == "" {
			req.Host = targetAddr
		}

		// WebSocket upgrade: check simulation mode first, then hand off.
		if isWebSocketUpgrade(req) {
			if cfg, ok := s.responseHandler.CheckOverlaySimMode(req); ok {
				log.Printf("SOCKS5 WS [%s] simulation mode %q applied to WS upgrade", targetAddr, cfg.Mode)
				writeSimResponse(conn, cfg)
				return
			}
			s.handlePlainWebSocket(conn, reader, req, targetAddr, targetPort)
			return
		}

		// Create a response recorder to capture the response
		rec := newResponseRecorder()

		// Pass request to ResponseHandler
		s.responseHandler.HandleRequest(rec, req)

		// Log HTTP request (plain HTTP through SOCKS5)
		if s.requestLogger != nil {
			requestLog := models.RequestLog{
				ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
				Timestamp:  time.Now().Format(time.RFC3339),
				EndpointID: "system-socks5-proxy",
				SOCKS5Info: &models.SOCKS5RequestInfo{
					TargetHost:    targetAddr,
					TargetPort:    int(targetPort),
					Protocol:      "HTTP",
					IsIntercepted: false,
				},
			}
			requestLog.ClientRequest.Method = req.Method
			requestLog.ClientRequest.FullURL = req.URL.String()
			requestLog.ClientRequest.Path = req.URL.Path
			s.requestLogger.LogRequest(requestLog)
		}

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

	// Get body bytes
	bodyBytes := rec.body.Bytes()

	// Write headers, but fix Transfer-Encoding and Content-Length issues
	// The backend may have sent chunked encoding, but we've already read the full body,
	// so we need to send Content-Length instead
	hasContentLength := false
	for key, values := range rec.Header() {
		// Skip Transfer-Encoding since we're sending the full body
		if strings.EqualFold(key, "Transfer-Encoding") {
			continue
		}
		// Track if Content-Length is already set
		if strings.EqualFold(key, "Content-Length") {
			hasContentLength = true
			// Update to actual body length (may differ due to transformations)
			fmt.Fprintf(&buf, "Content-Length: %d\r\n", len(bodyBytes))
			continue
		}
		for _, value := range values {
			fmt.Fprintf(&buf, "%s: %s\r\n", key, value)
		}
	}

	// Add Content-Length if not already present
	if !hasContentLength && len(bodyBytes) > 0 {
		fmt.Fprintf(&buf, "Content-Length: %d\r\n", len(bodyBytes))
	}

	// Write body
	buf.WriteString("\r\n")
	buf.Write(bodyBytes)

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
