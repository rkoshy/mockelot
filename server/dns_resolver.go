package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"sort"
	"sync"
	"time"

	"mockelot/models"
)

// DNSResolver handles DNS overrides and caching
type DNSResolver struct {
	config      *models.DNSOverrideConfig
	cache       map[string]*dnsEntry
	cacheMutex  sync.RWMutex
	regexCache  map[string]*regexp.Regexp
	regexMutex  sync.RWMutex
}

type dnsEntry struct {
	ips       []string
	timestamp time.Time
}

// NewDNSResolver creates a new DNS resolver with overrides
func NewDNSResolver(config *models.DNSOverrideConfig) *DNSResolver {
	return &DNSResolver{
		config:     config,
		cache:      make(map[string]*dnsEntry),
		regexCache: make(map[string]*regexp.Regexp),
	}
}

// Resolve resolves a domain to IP addresses with override support
func (r *DNSResolver) Resolve(ctx context.Context, domain string) ([]string, error) {
	// Check if DNS overrides are enabled
	if r.config != nil && r.config.Enabled {
		// Sort overrides by priority (lower number = higher priority)
		sortedOverrides := make([]models.DNSOverride, len(r.config.Overrides))
		copy(sortedOverrides, r.config.Overrides)
		sort.Slice(sortedOverrides, func(i, j int) bool {
			return sortedOverrides[i].Priority < sortedOverrides[j].Priority
		})

		for _, override := range sortedOverrides {
			if !override.Enabled {
				continue
			}

			// Get or compile regex
			re, err := r.getRegex(override.Pattern)
			if err != nil {
				log.Printf("[DNS] Failed to compile pattern %s: %v", override.Pattern, err)
				continue
			}

			if re.MatchString(domain) {
				log.Printf("[DNS] Override matched for %s: %s -> %s (%s)",
					domain, override.Pattern, override.Target, override.Type)

				switch override.Type {
				case "static":
					// Return static IP
					return []string{override.Target}, nil
				case "cname":
					// Resolve CNAME target
					log.Printf("[DNS] Resolving CNAME target: %s", override.Target)
					return r.resolveStandard(ctx, override.Target)
				default:
					log.Printf("[DNS] Unknown override type: %s", override.Type)
				}
			}
		}
	}

	// No override matched, use standard resolution
	return r.resolveStandard(ctx, domain)
}

func (r *DNSResolver) resolveStandard(ctx context.Context, domain string) ([]string, error) {
	// Check cache
	r.cacheMutex.RLock()
	if entry, ok := r.cache[domain]; ok {
		if time.Since(entry.timestamp) < 5*time.Minute {
			r.cacheMutex.RUnlock()
			log.Printf("[DNS] Cache hit for %s: %v", domain, entry.ips)
			return entry.ips, nil
		}
	}
	r.cacheMutex.RUnlock()

	// Resolve using system resolver
	log.Printf("[DNS] Resolving %s via system resolver", domain)
	ips, err := net.DefaultResolver.LookupHost(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", domain, err)
	}

	log.Printf("[DNS] Resolved %s to %v", domain, ips)

	// Update cache
	r.cacheMutex.Lock()
	r.cache[domain] = &dnsEntry{
		ips:       ips,
		timestamp: time.Now(),
	}
	r.cacheMutex.Unlock()

	return ips, nil
}

func (r *DNSResolver) getRegex(pattern string) (*regexp.Regexp, error) {
	r.regexMutex.RLock()
	if re, ok := r.regexCache[pattern]; ok {
		r.regexMutex.RUnlock()
		return re, nil
	}
	r.regexMutex.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	r.regexMutex.Lock()
	r.regexCache[pattern] = re
	r.regexMutex.Unlock()

	return re, nil
}

// UpdateConfig updates the DNS override configuration
func (r *DNSResolver) UpdateConfig(config *models.DNSOverrideConfig) {
	r.config = config
	// Clear regex cache when config changes
	r.regexMutex.Lock()
	r.regexCache = make(map[string]*regexp.Regexp)
	r.regexMutex.Unlock()

	// Clear DNS cache to ensure new overrides take effect immediately
	r.cacheMutex.Lock()
	r.cache = make(map[string]*dnsEntry)
	r.cacheMutex.Unlock()

	log.Printf("[DNS] Configuration updated with %d overrides", len(config.Overrides))
}

// ClearCache clears the DNS cache
func (r *DNSResolver) ClearCache() {
	r.cacheMutex.Lock()
	r.cache = make(map[string]*dnsEntry)
	r.cacheMutex.Unlock()
	log.Printf("[DNS] Cache cleared")
}