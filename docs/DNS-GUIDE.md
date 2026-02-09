# DNS Resolution and Override Guide

## Overview

Mockelot provides comprehensive DNS resolution control with support for:
- **DNS Overrides**: Pattern-based domain resolution overrides
- **Custom Upstream Servers**: Choose from presets or specify custom DNS servers
- **SOCKS5h Remote DNS**: DNS resolution through the proxy (privacy-preserving)
- **UDP ASSOCIATE**: True DNS-over-SOCKS5 for specialized tools

DNS configuration is independent of domain takeover - you can override DNS resolution without intercepting traffic for mocking.

## Features

### DNS Override Rules
- **Pattern Matching**: Full regex support for domain matching
- **Static IP**: Redirect domains to specific IP addresses
- **CNAME**: Alias domains to other domains
- **Priority System**: Lower numbers are checked first
- **Per-Rule Enable/Disable**: Toggle individual rules without deleting

### Upstream DNS Servers
- **System Default**: Automatically uses system DNS from `/etc/resolv.conf`
- **Provider Presets**: Quick selection of popular DNS providers:
  - Google DNS (8.8.8.8, 8.8.4.4)
  - Cloudflare (1.1.1.1, 1.0.0.1)
  - Cloudflare Family (with content filtering)
  - Quad9 (security-focused)
  - OpenDNS
  - AdGuard DNS (ad blocking)
  - NextDNS
- **Custom Servers**: Manually specify DNS server IP addresses
- **Failover Support**: Tries multiple upstream servers if one fails

## Configuration

### Accessing DNS Settings

1. Open Mockelot
2. Go to Settings (top navigation)
3. Expand "DNS Resolution" section
4. Configure upstream servers and add override rules

### Setting Upstream DNS Servers

1. **Select DNS Provider**: Choose from dropdown:
   - System Default (uses `/etc/resolv.conf`)
   - Google, Cloudflare, etc. (presets)
   - Custom DNS Servers

2. **For Custom Servers**: Enter IP addresses, one per line:
   ```
   8.8.8.8
   8.8.4.4
   1.1.1.1
   ```

### Creating DNS Override Rules

1. Click "+ Add Rule" in DNS Override Rules section
2. Configure the rule:
   - **Pattern**: Regex pattern for domain matching
   - **Type**: Static IP or CNAME
   - **Target**: IP address or domain
   - **Priority**: Lower numbers checked first
   - **Enabled**: Toggle rule on/off

#### Pattern Examples
- `^api\.example\.com$` - Exact match
- `.*\.example\.com` - All subdomains
- `^(www\.)?example\.com$` - With optional www
- `^test-.*\.dev$` - test-anything.dev

## How DNS Resolution Works

### Resolution Flow
1. Request arrives with domain name
2. Check DNS override rules (by priority)
3. If override matches → Return configured response
4. If no match → Query upstream DNS servers
5. Cache result for 5 minutes

### Integration Points

#### SOCKS5h (Remote DNS)
When browsers use SOCKS5 with remote DNS enabled:
```
Browser → SOCKS5 CONNECT → api.example.com:443
         (sends hostname, not IP)
Mockelot → DNS Resolution (with overrides) → Get IP
Mockelot → Connect to resolved IP
```

#### UDP ASSOCIATE (DNS over SOCKS5)
For tools that send DNS queries through SOCKS5:
```
Client → SOCKS5 UDP ASSOCIATE → Get UDP relay port
Client → UDP DNS Query → Mockelot UDP Relay
Mockelot → Apply DNS overrides → Return response
```

## Browser Configuration

### Firefox (SOCKS5h)
```
Settings → Network Settings:
- Manual proxy configuration
- SOCKS Host: localhost
- SOCKS Port: 1080
- SOCKS v5: ✓
- Proxy DNS when using SOCKS v5: ✓  ← CRITICAL for DNS overrides

Alternative (about:config):
- network.proxy.socks_remote_dns = true
```

### Chrome/Chromium
```bash
# Launch with remote DNS enabled
google-chrome --proxy-server="socks5://localhost:1080" \
              --host-resolver-rules="MAP * ~NOTFOUND, EXCLUDE localhost"
```

### cURL
```bash
# Use --socks5-hostname (not --socks5) for remote DNS
curl --socks5-hostname localhost:1080 https://api.example.com
```

## Use Cases

### Local Development
Override production domains to localhost:
- Pattern: `^api\.production\.com$`
- Type: Static IP
- Target: `127.0.0.1`

### Environment Switching
Point domains to different environments:
- Pattern: `^app\.example\.com$`
- Type: Static IP
- Target: `10.0.1.50` (staging server)

### Service Aliasing
Create domain aliases:
- Pattern: `^oldapi\.example\.com$`
- Type: CNAME
- Target: `newapi.example.com`

### Wildcard Subdomains
Redirect all subdomains:
- Pattern: `.*\.dev\.company\.com$`
- Type: Static IP
- Target: `192.168.1.100`

## DNS vs Domain Takeover

### DNS Overrides (This Feature)
- **Purpose**: Control how domain names resolve to IP addresses
- **Scope**: Affects ALL DNS resolution in Mockelot
- **Use When**: You want to redirect domains to different IPs
- **Example**: Make `api.prod.com` resolve to `127.0.0.1`

### Domain Takeover (Separate Feature)
- **Purpose**: Intercept and mock HTTP/HTTPS traffic
- **Scope**: Only affects domains in the takeover list
- **Use When**: You want to mock API responses
- **Example**: Return mock data for `api.example.com/users`

### Independent Operation
- **DNS without Takeover**: Override DNS but pass traffic through normally
- **Takeover without DNS**: Mock responses using normal DNS resolution
- **Both**: Complete control over resolution and responses

## Testing DNS Overrides

### Test Setup
1. Configure DNS override:
   - Pattern: `^test\.example\.com$`
   - Type: Static IP
   - Target: `127.0.0.1`

2. Enable SOCKS5 proxy in Mockelot

3. Configure browser for SOCKS5h (remote DNS)

4. Navigate to `http://test.example.com`

### Expected Results
- Browser connects through SOCKS5
- Mockelot receives hostname (not IP)
- DNS override applied: `test.example.com` → `127.0.0.1`
- Connection goes to localhost

### Verification
Check Mockelot logs:
```
[DNS] Override matched for test.example.com: ^test\.example\.com$ -> 127.0.0.1
SOCKS5 pass-through: Resolved test.example.com to 127.0.0.1
```

## Troubleshooting

### DNS Overrides Not Applied
- **Check pattern syntax**: Must be valid regex
- **Verify rule is enabled**: Check the enabled checkbox
- **Check priority order**: Lower numbers are processed first
- **Look for match in logs**: `[DNS] Override matched...`

### Browser Not Using Remote DNS
- **Firefox**: Ensure "Proxy DNS when using SOCKS v5" is checked
- **Chrome**: Use `--host-resolver-rules` flag
- **Test**: Mockelot logs should show hostnames, not IPs

### Custom DNS Servers Not Working
- **Verify IP addresses**: Must be valid IPv4 addresses
- **Check connectivity**: Ensure servers are reachable
- **Test with dig/nslookup**: `dig @8.8.8.8 example.com`

## Technical Details

### Performance
- DNS results cached for 5 minutes
- Regex patterns compiled and cached
- Failover adds ~2s timeout per failed server
- UDP relay sessions expire after 3 minutes of inactivity

### Security Considerations
- All DNS queries routed through Mockelot (privacy)
- No DNS leaks when using SOCKS5h properly
- DNS cache prevents amplification attacks
- Session management prevents hijacking

### Limitations
- IPv6 DNS (AAAA records) not yet supported
- DNSSEC validation not implemented
- DNS-over-HTTPS/TLS not supported for upstream
- Maximum UDP packet size: 512 bytes (standard DNS)

## Advanced Configuration

### Multiple Override Rules
Rules are processed in priority order (lowest first):
1. Priority 0: `^api\.example\.com$` → `127.0.0.1`
2. Priority 1: `.*\.example\.com` → `10.0.0.1`
3. Priority 2: `.*` → `192.168.1.1` (catch-all)

### Conditional Overrides
Use regex groups for complex patterns:
- `^(dev|test|staging)\.example\.com$` - Multiple environments
- `^api-v[0-9]+\.example\.com$` - Versioned APIs

### CNAME Chains
CNAME targets are resolved recursively:
- `old.example.com` → CNAME → `new.example.com`
- `new.example.com` → CNAME → `final.example.com`
- `final.example.com` → A → `1.2.3.4`

## Related Documentation

- [SOCKS5 Proxy Guide](SOCKS5-GUIDE.md) - Setting up the SOCKS5 proxy
- [Mock Endpoint Guide](MOCK-GUIDE.md) - Creating mock responses
- [Domain Takeover](SOCKS5-GUIDE.md#domain-takeover) - Intercepting domains for mocking