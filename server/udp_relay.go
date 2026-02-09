package server

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"mockelot/models"
	"net"
	"sync"
	"time"
)

// UDPRelay handles UDP traffic for SOCKS5 UDP ASSOCIATE
type UDPRelay struct {
	conn            *net.UDPConn
	clientAddr      *net.UDPAddr
	dnsResolver     *DNSResolver
	upstreamServers []string // Upstream DNS servers
	sessions        map[string]*udpSession
	sessionsMu      sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
}

// udpSession tracks a UDP "connection"
type udpSession struct {
	clientAddr   *net.UDPAddr
	lastActivity time.Time
}

// NewUDPRelay creates a new UDP relay handler
func NewUDPRelay(dnsResolver *DNSResolver) (*UDPRelay, error) {
	// Bind to any available port
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP socket: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Get initial upstream servers
	upstreamServers := []string{"8.8.8.8", "8.8.4.4"} // Default to Google DNS
	if dnsResolver != nil && dnsResolver.config != nil {
		upstreamServers = getUpstreamServers(dnsResolver.config)
	}

	relay := &UDPRelay{
		conn:            conn,
		dnsResolver:     dnsResolver,
		upstreamServers: upstreamServers,
		sessions:        make(map[string]*udpSession),
		ctx:             ctx,
		cancel:          cancel,
	}

	return relay, nil
}

// UpdateUpstreamServers updates the list of upstream DNS servers
func (r *UDPRelay) UpdateUpstreamServers(servers []string) {
	if len(servers) == 0 {
		// Fall back to defaults if no servers provided
		servers = GetSystemDNSServers()
	}
	r.upstreamServers = servers
	log.Printf("[UDP Relay] Updated upstream DNS servers: %v", servers)
}

// getUpstreamServers returns the configured upstream servers
func getUpstreamServers(config *models.DNSOverrideConfig) []string {
	if config == nil {
		return GetSystemDNSServers()
	}

	// Use configured upstream servers if available
	if len(config.UpstreamServers) > 0 {
		return config.UpstreamServers
	}

	// Use system DNS if requested
	if config.UseSystemDNS {
		return GetSystemDNSServers()
	}

	// Default to Google DNS
	return []string{"8.8.8.8", "8.8.4.4"}
}

// Start begins processing UDP packets
func (r *UDPRelay) Start() {
	go r.readLoop()
	go r.cleanupLoop()
	log.Printf("[UDP Relay] Started on %s (listening for DNS queries)", r.conn.LocalAddr())
}

// Stop stops the UDP relay
func (r *UDPRelay) Stop() {
	r.cancel()
	r.conn.Close()
	log.Println("[UDP Relay] Stopped")
}

// GetAddress returns the local address of the UDP relay
func (r *UDPRelay) GetAddress() *net.UDPAddr {
	return r.conn.LocalAddr().(*net.UDPAddr)
}

// readLoop reads incoming UDP packets
func (r *UDPRelay) readLoop() {
	buffer := make([]byte, 65535)

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			// Set read deadline to allow periodic context checks
			r.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, clientAddr, err := r.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Timeout is expected, check context and continue
				}
				if r.ctx.Err() == nil {
					log.Printf("[UDP Relay] Read error: %v", err)
				}
				continue
			}

			log.Printf("[UDP Relay] Received %d bytes from %s", n, clientAddr)

			// Process the packet in a goroutine
			packet := make([]byte, n)
			copy(packet, buffer[:n])
			go r.handlePacket(packet, clientAddr)
		}
	}
}

// handlePacket processes a UDP packet from client
func (r *UDPRelay) handlePacket(data []byte, clientAddr *net.UDPAddr) {
	// SOCKS5 UDP request header:
	// +----+------+------+----------+----------+----------+
	// |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
	// +----+------+------+----------+----------+----------+
	// | 2  |  1   |  1   | Variable |    2     | Variable |
	// +----+------+------+----------+----------+----------+

	if len(data) < 10 {
		log.Printf("[UDP Relay] Packet too short from %s", clientAddr)
		return
	}

	// Skip RSV (2 bytes) and FRAG (1 byte)
	if data[2] != 0x00 {
		log.Printf("[UDP Relay] Fragmentation not supported")
		return
	}

	atyp := data[3]
	offset := 4

	var dstHost string
	var dstPort uint16

	switch atyp {
	case atypIPv4:
		if len(data) < offset+4+2 {
			log.Printf("[UDP Relay] Invalid IPv4 packet")
			return
		}
		ip := net.IP(data[offset : offset+4])
		dstHost = ip.String()
		offset += 4

	case atypDomain:
		domainLen := int(data[offset])
		offset++
		if len(data) < offset+domainLen+2 {
			log.Printf("[UDP Relay] Invalid domain packet")
			return
		}
		dstHost = string(data[offset : offset+domainLen])
		offset += domainLen

	case atypIPv6:
		if len(data) < offset+16+2 {
			log.Printf("[UDP Relay] Invalid IPv6 packet")
			return
		}
		ip := net.IP(data[offset : offset+16])
		dstHost = ip.String()
		offset += 16

	default:
		log.Printf("[UDP Relay] Unsupported address type: %d", atyp)
		return
	}

	// Read port
	dstPort = binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Extract the actual data (DNS packet if port 53)
	payload := data[offset:]

	// Update session
	r.updateSession(clientAddr)

	// Check if this is a DNS query (port 53)
	if dstPort == 53 {
		r.handleDNSPacket(payload, dstHost, clientAddr)
	} else {
		// For non-DNS UDP, just forward it (future enhancement)
		log.Printf("[UDP Relay] Non-DNS UDP to %s:%d not implemented", dstHost, dstPort)
	}
}

// handleDNSPacket processes DNS queries and applies overrides
func (r *UDPRelay) handleDNSPacket(data []byte, dnsServer string, clientAddr *net.UDPAddr) {
	// Parse DNS query
	query, err := ParseDNSPacket(data)
	if err != nil {
		log.Printf("[UDP Relay] Failed to parse DNS query: %v", err)
		return
	}

	// Check if it's a query (not a response)
	if query.Header.Flags&DNSFlagResponse != 0 {
		log.Printf("[UDP Relay] Ignoring DNS response packet")
		return
	}

	// Process each question
	for _, question := range query.Questions {
		if question.Type != DNSTypeA {
			// For now, only handle A records
			log.Printf("[UDP Relay] DNS query for %s (type %d) - forwarding to upstream", question.Name, question.Type)
			r.forwardDNSQuery(data, dnsServer, clientAddr)
			return
		}

		log.Printf("[UDP Relay] DNS query for %s from %s", question.Name, clientAddr)

		// Check for override
		if r.dnsResolver != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			ips, err := r.dnsResolver.Resolve(ctx, question.Name)
			if err == nil && len(ips) > 0 {
				// Build DNS response with override
				log.Printf("[UDP Relay] DNS override for %s -> %s", question.Name, ips[0])
				r.sendDNSResponse(query, question.Name, ips[0], clientAddr)
				return
			}
		}

		// No override, forward to upstream DNS
		r.forwardDNSQuery(data, dnsServer, clientAddr)
	}
}

// sendDNSResponse sends a DNS response with the overridden IP
func (r *UDPRelay) sendDNSResponse(query *DNSPacket, domain string, ipStr string, clientAddr *net.UDPAddr) {
	// Parse IP address
	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.Printf("[UDP Relay] Invalid IP address: %s", ipStr)
		return
	}

	// Build answer
	answer := BuildDNSAnswer(domain, ip)
	if answer.RData == nil {
		log.Printf("[UDP Relay] Failed to build DNS answer for %s", domain)
		return
	}

	// Build response packet
	responseData := BuildDNSResponse(query, []DNSResourceRecord{answer})

	// Wrap in SOCKS5 UDP reply format
	r.sendUDPReply(responseData, clientAddr, "0.0.0.0", 53)
}

// forwardDNSQuery forwards DNS query to upstream server
func (r *UDPRelay) forwardDNSQuery(data []byte, dnsServer string, clientAddr *net.UDPAddr) {
	// Get list of upstream servers to try
	var upstreamServers []string

	// If a specific DNS server was requested (from the packet), use it first
	if dnsServer != "" && dnsServer != "0.0.0.0" {
		upstreamServers = []string{dnsServer}
	} else {
		// Use configured upstream servers
		upstreamServers = r.upstreamServers
		if len(upstreamServers) == 0 {
			// Fallback to defaults if no upstream configured
			upstreamServers = []string{"8.8.8.8", "8.8.4.4"}
		}
	}

	// Try each upstream server until one succeeds
	for _, upstream := range upstreamServers {
		// Resolve DNS server address
		serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:53", upstream))
		if err != nil {
			log.Printf("[UDP Relay] Failed to resolve DNS server %s: %v", upstream, err)
			continue
		}

		// Create temporary connection to DNS server
		conn, err := net.DialUDP("udp", nil, serverAddr)
		if err != nil {
			log.Printf("[UDP Relay] Failed to connect to DNS server %s: %v", upstream, err)
			continue
		}
		defer conn.Close()

		// Send query
		if _, err := conn.Write(data); err != nil {
			log.Printf("[UDP Relay] Failed to send DNS query to %s: %v", upstream, err)
			continue
		}

		// Read response
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		response := make([]byte, 512) // Standard DNS response size
		n, err := conn.Read(response)
		if err != nil {
			log.Printf("[UDP Relay] Failed to read DNS response from %s: %v", upstream, err)
			continue
		}

		// Successfully got response, send it back to client
		r.sendUDPReply(response[:n], clientAddr, serverAddr.IP.String(), 53)
		return
	}

	// All upstream servers failed
	log.Printf("[UDP Relay] All upstream DNS servers failed for query from %s", clientAddr)
}

// sendUDPReply sends a UDP reply to the client in SOCKS5 format
func (r *UDPRelay) sendUDPReply(data []byte, clientAddr *net.UDPAddr, fromHost string, fromPort uint16) {
	// Build SOCKS5 UDP reply header
	var reply []byte
	reply = append(reply, 0x00, 0x00) // RSV
	reply = append(reply, 0x00)       // FRAG

	// Add source address
	ip := net.ParseIP(fromHost)
	if ip4 := ip.To4(); ip4 != nil {
		reply = append(reply, atypIPv4)
		reply = append(reply, ip4...)
	} else if ip6 := ip.To16(); ip6 != nil {
		reply = append(reply, atypIPv6)
		reply = append(reply, ip6...)
	} else {
		// Domain name
		reply = append(reply, atypDomain)
		reply = append(reply, byte(len(fromHost)))
		reply = append(reply, []byte(fromHost)...)
	}

	// Add port
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, fromPort)
	reply = append(reply, portBytes...)

	// Add data
	reply = append(reply, data...)

	// Send to client
	if _, err := r.conn.WriteToUDP(reply, clientAddr); err != nil {
		log.Printf("[UDP Relay] Failed to send reply to %s: %v", clientAddr, err)
	}
}

// updateSession updates or creates a session for the client
func (r *UDPRelay) updateSession(clientAddr *net.UDPAddr) {
	r.sessionsMu.Lock()
	defer r.sessionsMu.Unlock()

	key := clientAddr.String()
	r.sessions[key] = &udpSession{
		clientAddr:   clientAddr,
		lastActivity: time.Now(),
	}
}

// cleanupLoop removes inactive sessions
func (r *UDPRelay) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.cleanupSessions()
		}
	}
}

// cleanupSessions removes sessions inactive for more than 3 minutes
func (r *UDPRelay) cleanupSessions() {
	r.sessionsMu.Lock()
	defer r.sessionsMu.Unlock()

	now := time.Now()
	for key, session := range r.sessions {
		if now.Sub(session.lastActivity) > 3*time.Minute {
			delete(r.sessions, key)
			log.Printf("[UDP Relay] Cleaned up session for %s", key)
		}
	}
}