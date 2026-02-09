package server

import (
	"bufio"
	"fmt"
	"log"
	"mockelot/models"
	"net"
	"os"
	"runtime"
	"strings"
)

// GetSystemDNSServers returns the system's configured DNS servers
func GetSystemDNSServers() []string {
	switch runtime.GOOS {
	case "linux":
		return getLinuxDNSServers()
	case "darwin":
		return getMacDNSServers()
	case "windows":
		return getWindowsDNSServers()
	default:
		// Fallback to common public DNS
		return []string{"8.8.8.8", "8.8.4.4"}
	}
}

// getLinuxDNSServers reads DNS servers from /etc/resolv.conf
func getLinuxDNSServers() []string {
	var servers []string

	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		log.Printf("[DNS] Failed to read /etc/resolv.conf: %v", err)
		return []string{"8.8.8.8", "8.8.4.4"} // Fallback to Google DNS
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Look for nameserver lines
		if strings.HasPrefix(line, "nameserver") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ip := parts[1]
				// Validate IP
				if net.ParseIP(ip) != nil {
					// Skip localhost (would cause loops)
					if ip != "127.0.0.1" && ip != "::1" && !strings.HasPrefix(ip, "127.") {
						servers = append(servers, ip)
					}
				}
			}
		}
	}

	if len(servers) == 0 {
		// No valid servers found, use fallback
		return []string{"8.8.8.8", "8.8.4.4"}
	}

	log.Printf("[DNS] Found system DNS servers: %v", servers)
	return servers
}

// getMacDNSServers gets DNS servers on macOS using scutil
func getMacDNSServers() []string {
	// On macOS, we could parse scutil output, but for simplicity
	// we'll read from /etc/resolv.conf which macOS also maintains
	return getLinuxDNSServers()
}

// getWindowsDNSServers gets DNS servers on Windows
func getWindowsDNSServers() []string {
	// On Windows, this would require using Windows API or parsing netsh output
	// For now, return common defaults
	log.Printf("[DNS] Windows DNS detection not implemented, using defaults")
	return []string{"8.8.8.8", "8.8.4.4"}
}

// Common DNS Provider presets
var DNSProviders = map[string]models.DNSProvider{
	"system": {
		Name:        "System Default",
		Description: "Use system configured DNS servers",
		Servers:     []string{}, // Populated dynamically
		IsSystem:    true,
	},
	"google": {
		Name:        "Google Public DNS",
		Description: "Google's fast, secure DNS service",
		Servers:     []string{"8.8.8.8", "8.8.4.4"},
		IsSystem:    false,
	},
	"cloudflare": {
		Name:        "Cloudflare DNS",
		Description: "Privacy-focused DNS with 1.1.1.1",
		Servers:     []string{"1.1.1.1", "1.0.0.1"},
		IsSystem:    false,
	},
	"cloudflare-family": {
		Name:        "Cloudflare Family",
		Description: "Cloudflare DNS with malware and adult content blocking",
		Servers:     []string{"1.1.1.3", "1.0.0.3"},
		IsSystem:    false,
	},
	"quad9": {
		Name:        "Quad9",
		Description: "Security-focused DNS with threat blocking",
		Servers:     []string{"9.9.9.9", "149.112.112.112"},
		IsSystem:    false,
	},
	"opendns": {
		Name:        "OpenDNS",
		Description: "Cisco's OpenDNS with phishing protection",
		Servers:     []string{"208.67.222.222", "208.67.220.220"},
		IsSystem:    false,
	},
	"opendns-family": {
		Name:        "OpenDNS Family Shield",
		Description: "OpenDNS with adult content filtering",
		Servers:     []string{"208.67.222.123", "208.67.220.123"},
		IsSystem:    false,
	},
	"adguard": {
		Name:        "AdGuard DNS",
		Description: "DNS with ad and tracker blocking",
		Servers:     []string{"94.140.14.14", "94.140.15.15"},
		IsSystem:    false,
	},
	"adguard-family": {
		Name:        "AdGuard Family",
		Description: "AdGuard DNS with family protection",
		Servers:     []string{"94.140.14.15", "94.140.15.16"},
		IsSystem:    false,
	},
	"nextdns": {
		Name:        "NextDNS",
		Description: "Privacy-focused DNS with customizable filtering",
		Servers:     []string{"45.90.28.0", "45.90.30.0"},
		IsSystem:    false,
	},
}

// GetDNSProviders returns all available DNS provider presets
func GetDNSProviders() map[string]models.DNSProvider {
	// Update system provider with actual system DNS
	providers := make(map[string]models.DNSProvider)
	for k, v := range DNSProviders {
		if k == "system" {
			v.Servers = GetSystemDNSServers()
		}
		providers[k] = v
	}
	return providers
}

// ValidateDNSServer validates that a string is a valid IP address for DNS
func ValidateDNSServer(server string) error {
	ip := net.ParseIP(server)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", server)
	}

	// Check for loopback addresses that could cause issues
	if ip.IsLoopback() {
		return fmt.Errorf("loopback address not allowed: %s", server)
	}

	return nil
}