package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

// DNS packet structure for parsing and building DNS messages
// Based on RFC 1035

// DNSHeader represents the DNS message header
type DNSHeader struct {
	ID      uint16
	Flags   uint16
	QDCount uint16 // Number of questions
	ANCount uint16 // Number of answers
	NSCount uint16 // Number of authority records
	ARCount uint16 // Number of additional records
}

// DNSQuestion represents a DNS question
type DNSQuestion struct {
	Name  string
	Type  uint16
	Class uint16
}

// DNSResourceRecord represents a DNS resource record (answer, authority, or additional)
type DNSResourceRecord struct {
	Name     string
	Type     uint16
	Class    uint16
	TTL      uint32
	RDLength uint16
	RData    []byte
}

// DNSPacket represents a complete DNS message
type DNSPacket struct {
	Header    DNSHeader
	Questions []DNSQuestion
	Answers   []DNSResourceRecord
	Authority []DNSResourceRecord
	Additional []DNSResourceRecord
}

// DNS constants
const (
	DNSTypeA     = 1  // IPv4 address
	DNSTypeAAAA  = 28 // IPv6 address
	DNSTypeCNAME = 5  // Canonical name
	DNSTypePTR   = 12 // Pointer record

	DNSClassIN = 1 // Internet class

	// DNS flags
	DNSFlagResponse = 1 << 15 // Query/Response flag (1 = response)
	DNSFlagAA       = 1 << 10 // Authoritative answer
	DNSFlagRD       = 1 << 8  // Recursion desired
	DNSFlagRA       = 1 << 7  // Recursion available
)

// ParseDNSPacket parses a DNS packet from bytes
func ParseDNSPacket(data []byte) (*DNSPacket, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("DNS packet too short")
	}

	packet := &DNSPacket{}
	reader := bytes.NewReader(data)

	// Parse header
	if err := binary.Read(reader, binary.BigEndian, &packet.Header); err != nil {
		return nil, fmt.Errorf("failed to read DNS header: %w", err)
	}

	// Parse questions
	for i := 0; i < int(packet.Header.QDCount); i++ {
		question, err := parseDNSQuestion(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to parse question %d: %w", i, err)
		}
		packet.Questions = append(packet.Questions, *question)
	}

	// Parse answers
	for i := 0; i < int(packet.Header.ANCount); i++ {
		answer, err := parseDNSResourceRecord(reader, data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse answer %d: %w", i, err)
		}
		packet.Answers = append(packet.Answers, *answer)
	}

	// Skip authority and additional records for now (not needed for basic DNS proxy)

	return packet, nil
}

// BuildDNSResponse builds a DNS response packet
func BuildDNSResponse(query *DNSPacket, answers []DNSResourceRecord) []byte {
	var buf bytes.Buffer

	// Build header
	header := DNSHeader{
		ID:      query.Header.ID,
		Flags:   DNSFlagResponse | DNSFlagRD | DNSFlagRA, // Response with recursion
		QDCount: query.Header.QDCount,
		ANCount: uint16(len(answers)),
		NSCount: 0,
		ARCount: 0,
	}

	binary.Write(&buf, binary.BigEndian, header)

	// Write questions (echo from query)
	for _, q := range query.Questions {
		writeDNSName(&buf, q.Name)
		binary.Write(&buf, binary.BigEndian, q.Type)
		binary.Write(&buf, binary.BigEndian, q.Class)
	}

	// Write answers
	for _, a := range answers {
		writeDNSName(&buf, a.Name)
		binary.Write(&buf, binary.BigEndian, a.Type)
		binary.Write(&buf, binary.BigEndian, a.Class)
		binary.Write(&buf, binary.BigEndian, a.TTL)
		binary.Write(&buf, binary.BigEndian, a.RDLength)
		buf.Write(a.RData)
	}

	return buf.Bytes()
}

// BuildDNSAnswer builds a DNS A record answer for an IP address
func BuildDNSAnswer(name string, ip net.IP) DNSResourceRecord {
	ipv4 := ip.To4()
	if ipv4 == nil {
		// IPv6 not supported yet
		return DNSResourceRecord{}
	}

	return DNSResourceRecord{
		Name:     name,
		Type:     DNSTypeA,
		Class:    DNSClassIN,
		TTL:      300, // 5 minutes
		RDLength: 4,
		RData:    ipv4,
	}
}

// BuildCNAMEAnswer builds a DNS CNAME record answer
func BuildCNAMEAnswer(name string, cname string) DNSResourceRecord {
	var rdata bytes.Buffer
	writeDNSName(&rdata, cname)

	return DNSResourceRecord{
		Name:     name,
		Type:     DNSTypeCNAME,
		Class:    DNSClassIN,
		TTL:      300, // 5 minutes
		RDLength: uint16(rdata.Len()),
		RData:    rdata.Bytes(),
	}
}

// parseDNSQuestion parses a DNS question from the reader
func parseDNSQuestion(reader *bytes.Reader) (*DNSQuestion, error) {
	name, err := readDNSName(reader)
	if err != nil {
		return nil, err
	}

	var qtype, qclass uint16
	if err := binary.Read(reader, binary.BigEndian, &qtype); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.BigEndian, &qclass); err != nil {
		return nil, err
	}

	return &DNSQuestion{
		Name:  name,
		Type:  qtype,
		Class: qclass,
	}, nil
}

// parseDNSResourceRecord parses a DNS resource record
func parseDNSResourceRecord(reader *bytes.Reader, packet []byte) (*DNSResourceRecord, error) {
	name, err := readDNSName(reader)
	if err != nil {
		return nil, err
	}

	var rr DNSResourceRecord
	rr.Name = name

	if err := binary.Read(reader, binary.BigEndian, &rr.Type); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.BigEndian, &rr.Class); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.BigEndian, &rr.TTL); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.BigEndian, &rr.RDLength); err != nil {
		return nil, err
	}

	rr.RData = make([]byte, rr.RDLength)
	if _, err := reader.Read(rr.RData); err != nil {
		return nil, err
	}

	return &rr, nil
}

// readDNSName reads a DNS name from the reader (handles compression)
func readDNSName(reader *bytes.Reader) (string, error) {
	var parts []string

	for {
		var length byte
		if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
			return "", err
		}

		if length == 0 {
			// End of name
			break
		}

		if length&0xC0 == 0xC0 {
			// Compression pointer (not fully implemented for simplicity)
			// Skip the second byte
			reader.ReadByte()
			break
		}

		// Read label
		label := make([]byte, length)
		if _, err := reader.Read(label); err != nil {
			return "", err
		}
		parts = append(parts, string(label))
	}

	return strings.Join(parts, "."), nil
}

// writeDNSName writes a DNS name to the buffer
func writeDNSName(buf *bytes.Buffer, name string) {
	if name == "" || name == "." {
		buf.WriteByte(0)
		return
	}

	// Remove trailing dot if present
	name = strings.TrimSuffix(name, ".")

	parts := strings.Split(name, ".")
	for _, part := range parts {
		if len(part) > 63 {
			part = part[:63] // Truncate to max label length
		}
		buf.WriteByte(byte(len(part)))
		buf.WriteString(part)
	}
	buf.WriteByte(0) // End of name
}