#!/bin/bash

# SOCKS5 Testing Script for Mockelot
# This script tests the SOCKS5 proxy functionality

set -e

SOCKS5_HOST="localhost"
SOCKS5_PORT="1080"
HTTP_PORT="8080"
HTTPS_PORT="8443"

echo "========================================="
echo "Mockelot SOCKS5 Proxy Test Suite"
echo "========================================="
echo ""

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print test results
print_result() {
    local test_name="$1"
    local result="$2"
    if [ "$result" -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $test_name"
    else
        echo -e "${RED}✗${NC} $test_name"
    fi
}

# Check if Mockelot is running
echo "Checking if Mockelot is running..."
if ! curl -s http://localhost:${HTTP_PORT}/health > /dev/null 2>&1; then
    echo -e "${YELLOW}Warning: Mockelot doesn't seem to be running on port ${HTTP_PORT}${NC}"
    echo "Please start Mockelot with the test-socks5-config.json configuration"
    echo "You can load it from the UI or use it as a test configuration"
    echo ""
fi

echo ""
echo "========================================="
echo "Test 1: SOCKS5 Connectivity"
echo "========================================="

# Test 1a: Basic SOCKS5 connection to /health endpoint (any domain)
echo -n "Test 1a: Basic SOCKS5 connection (any domain)... "
if curl -s --socks5 ${SOCKS5_HOST}:${SOCKS5_PORT} \
    http://any.domain.com/health > /tmp/socks5-test-1a.txt 2>&1; then
    if grep -q "healthy" /tmp/socks5-test-1a.txt; then
        print_result "SOCKS5 connectivity with 'any' domain filter" 0
        cat /tmp/socks5-test-1a.txt
    else
        print_result "SOCKS5 connectivity with 'any' domain filter" 1
        echo "Response didn't contain expected 'healthy' status"
        cat /tmp/socks5-test-1a.txt
    fi
else
    print_result "SOCKS5 connectivity with 'any' domain filter" 1
    echo "Failed to connect via SOCKS5"
    cat /tmp/socks5-test-1a.txt
fi
echo ""

echo "========================================="
echo "Test 2: Domain-Specific Matching"
echo "========================================="

# Test 2a: Match specific domain (api.test.local)
echo -n "Test 2a: Specific domain match (api.test.local /api/users)... "
if curl -s --socks5 ${SOCKS5_HOST}:${SOCKS5_PORT} \
    -H "Host: api.test.local" \
    http://api.test.local/api/users > /tmp/socks5-test-2a.txt 2>&1; then
    if grep -q "Alice" /tmp/socks5-test-2a.txt; then
        print_result "Specific domain endpoint matching" 0
        cat /tmp/socks5-test-2a.txt
    else
        print_result "Specific domain endpoint matching" 1
        echo "Response didn't contain expected user data"
        cat /tmp/socks5-test-2a.txt
    fi
else
    print_result "Specific domain endpoint matching" 1
    cat /tmp/socks5-test-2a.txt
fi
echo ""

# Test 2b: Match all intercepted domains (app.test.local /test)
echo -n "Test 2b: All intercepted domains match (app.test.local /test)... "
if curl -s --socks5 ${SOCKS5_HOST}:${SOCKS5_PORT} \
    -H "Host: app.test.local" \
    http://app.test.local/test > /tmp/socks5-test-2b.txt 2>&1; then
    if grep -q "intercepted domains" /tmp/socks5-test-2b.txt; then
        print_result "All intercepted domains endpoint matching" 0
        cat /tmp/socks5-test-2b.txt
    else
        print_result "All intercepted domains endpoint matching" 1
        echo "Response didn't contain expected text"
        cat /tmp/socks5-test-2b.txt
    fi
else
    print_result "All intercepted domains endpoint matching" 1
    cat /tmp/socks5-test-2b.txt
fi
echo ""

echo "========================================="
echo "Test 3: Overlay Mode (Passthrough)"
echo "========================================="

# Test 3a: Overlay mode to real server (passthrough.test.local)
echo "Test 3a: Overlay mode passthrough (passthrough.test.local)... "
echo "Note: This will try to resolve passthrough.test.local and proxy to real server"
echo "If the domain doesn't exist, it should fail gracefully"
if curl -s --socks5 ${SOCKS5_HOST}:${SOCKS5_PORT} \
    --connect-timeout 5 \
    -H "Host: passthrough.test.local" \
    http://passthrough.test.local/ > /tmp/socks5-test-3a.txt 2>&1; then
    print_result "Overlay mode passthrough" 0
    echo "Response (first 200 chars):"
    head -c 200 /tmp/socks5-test-3a.txt
    echo ""
else
    echo -e "${YELLOW}⚠${NC} Overlay mode test - DNS resolution expected to fail for test domain"
    echo "This is normal if passthrough.test.local doesn't resolve to a real server"
fi
echo ""

echo "========================================="
echo "Test 4: HTTPS through SOCKS5"
echo "========================================="

# Test 4a: HTTPS connection via SOCKS5
echo -n "Test 4a: HTTPS via SOCKS5 (api.test.local)... "
if curl -s --socks5 ${SOCKS5_HOST}:${SOCKS5_PORT} \
    -k \
    -H "Host: api.test.local" \
    https://api.test.local:${HTTPS_PORT}/api/users > /tmp/socks5-test-4a.txt 2>&1; then
    if grep -q "Alice" /tmp/socks5-test-4a.txt; then
        print_result "HTTPS through SOCKS5" 0
        cat /tmp/socks5-test-4a.txt
    else
        print_result "HTTPS through SOCKS5" 1
        cat /tmp/socks5-test-4a.txt
    fi
else
    print_result "HTTPS through SOCKS5" 1
    cat /tmp/socks5-test-4a.txt
fi
echo ""

echo "========================================="
echo "Test 5: Non-Intercepted Domain"
echo "========================================="

# Test 5a: Request to non-intercepted domain (should pass through)
echo "Test 5a: Non-intercepted domain passthrough... "
echo "Note: This tests passthrough for domains NOT in the takeover list"
if curl -s --socks5 ${SOCKS5_HOST}:${SOCKS5_PORT} \
    --connect-timeout 5 \
    http://example.com/ > /tmp/socks5-test-5a.txt 2>&1; then
    print_result "Non-intercepted domain passthrough" 0
    echo "Successfully connected to example.com via SOCKS5 passthrough"
    echo "Response (first 100 chars):"
    head -c 100 /tmp/socks5-test-5a.txt
    echo ""
else
    echo -e "${YELLOW}⚠${NC} Passthrough test - may fail if network/DNS unavailable"
fi
echo ""

echo "========================================="
echo "Test Summary"
echo "========================================="
echo ""
echo "Core SOCKS5 tests completed."
echo ""
echo "To test with authentication, update the configuration to enable"
echo "authentication and run this script again."
echo ""
echo "To test with a browser:"
echo "  1. Configure Firefox SOCKS5 proxy: localhost:1080"
echo "  2. Add entries to /etc/hosts:"
echo "     127.0.0.1 api.test.local"
echo "     127.0.0.1 app.test.local"
echo "  3. Navigate to http://api.test.local:8080/api/users"
echo ""
echo "Cleanup: removing test output files..."
rm -f /tmp/socks5-test-*.txt
echo "Done!"
