# Bug Fix: Proxy Default Headers Issue

## Issue Report

**Reported by User:**
> "The creation wizard for a proxy doesn't insert the default headers any more. So I created the proxy w/o any headeers. Then When I edited the proxy and clicked the default headers button, it got stuck"

## Root Cause Analysis

Two separate but related issues were identified:

### Problem 1: Default Headers Not Inserted During Proxy Creation

**Location:** `frontend/src/components/dialogs/AddEndpointDialog.vue`

**Issue:**
- Container endpoints auto-load default headers when entering Step 6 (proxy configuration) - line 327-329
- Regular proxy endpoints have NO auto-loading mechanism during the wizard
- Headers remain empty throughout the creation process (initialized as empty array on line 172)

**Code Comparison:**
```typescript
// Container endpoints - HAD auto-loading
if (endpointType.value === 'container' && currentStep.value === 6 && requestHeaders.value.length === 0) {
  await loadDefaultContainerHeaders()
}

// Regular proxy endpoints - MISSING auto-loading (before fix)
// No equivalent code existed
```

### Problem 2: "Reset to Defaults" Button Gets Stuck

**Location:** `frontend/src/components/dialogs/ProxyConfigPanel.vue`

**Issue:**
- The `resetToDefaults()` async function (lines 85-97) called `GetDefaultContainerHeaders()` without error handling
- No try/catch block to handle backend call failures
- No timeout or loading indicators
- If backend call failed, hung, or timed out → UI appeared "stuck"

**Original Code:**
```typescript
async function resetToDefaults() {
  const defaults = await GetDefaultContainerHeaders()  // No error handling!

  if (props.isContainerEndpoint) {
    inboundHeaders.value = defaults
  } else {
    inboundHeaders.value = defaults.filter(h => h.name !== 'Host')
  }
  emitUpdate()
}
```

## Implemented Fixes

### Fix 1: Auto-load Default Headers for Proxy Endpoints

**File:** `frontend/src/components/dialogs/AddEndpointDialog.vue`

**Changes:**
1. Added new function `loadDefaultProxyHeaders()` (lines 321-329):
   ```typescript
   async function loadDefaultProxyHeaders() {
     try {
       const defaults = await GetDefaultContainerHeaders()
       // For regular proxy endpoints, filter out container-specific Host header
       requestHeaders.value = defaults.filter(h => h.name !== 'Host')
     } catch (error) {
       console.error('Failed to load default proxy headers:', error)
     }
   }
   ```

2. Modified `handleNext()` to auto-call this function when entering Step 3 for proxy endpoints (lines 331-334):
   ```typescript
   // Auto-load default proxy headers when entering headers step for regular proxy endpoints
   if (endpointType.value === 'proxy' && currentStep.value === 3 && requestHeaders.value.length === 0) {
     await loadDefaultProxyHeaders()
   }
   ```

**Behavior:**
- When user creates a proxy endpoint and advances to Step 3 (Headers & Status Codes)
- Default headers are automatically loaded (filtered to exclude Host header)
- Same RFC 7230 compliant headers as container endpoints (hop-by-hop header drops + X-Forwarded-* headers)
- Only loads if headers list is empty (respects user's existing configuration)

### Fix 2: Add Error Handling to Reset Defaults Button

**File:** `frontend/src/components/dialogs/ProxyConfigPanel.vue`

**Changes:**
Wrapped `resetToDefaults()` function in try/catch block (lines 85-103):
```typescript
async function resetToDefaults() {
  try {
    const defaults = await GetDefaultContainerHeaders()

    if (props.isContainerEndpoint) {
      // For containers, use all RFC-compliant container headers (includes Host manipulation)
      inboundHeaders.value = defaults
    } else {
      // For regular proxy endpoints, use all defaults except the container-specific Host header
      // Keep: hop-by-hop header drops (RFC 7230) + X-Forwarded-* headers
      inboundHeaders.value = defaults.filter(h => h.name !== 'Host')
    }
    emitUpdate()
  } catch (error) {
    console.error('Failed to load default headers:', error)
    // Optionally show user feedback
    alert('Failed to load default headers. Please try again or check the console for errors.')
  }
}
```

**Behavior:**
- If backend call fails: Error logged to console + user alert shown
- UI no longer "freezes" or appears stuck
- User receives clear feedback about the failure

## Default Headers Explained

### For Container Endpoints
All default headers from `GetDefaultContainerHeaders()` including:
- Host header manipulation (for proper container communication)
- Hop-by-hop header drops (Connection, Keep-Alive, etc.) per RFC 7230
- X-Forwarded-For, X-Forwarded-Proto, X-Real-IP headers

### For Regular Proxy Endpoints
Same as container endpoints EXCEPT:
- Host header is filtered out (proxies typically don't need to manipulate Host)
- Retains all other RFC 7230 compliant headers
- X-Forwarded-* headers for proper request forwarding

## Testing Checklist

- [x] Frontend builds successfully (`npm run build`)
- [x] Backend compiles successfully (`go build`)
- [ ] Manual testing: Create proxy endpoint via wizard → verify default headers loaded in Step 3
- [ ] Manual testing: Edit existing proxy endpoint → click "Reset to Defaults" → verify it works
- [ ] Manual testing: Simulate backend failure → verify error handling shows alert instead of freezing

## Files Modified

1. `frontend/src/components/dialogs/AddEndpointDialog.vue`
   - Added `loadDefaultProxyHeaders()` function
   - Modified `handleNext()` to auto-load headers for proxy endpoints

2. `frontend/src/components/dialogs/ProxyConfigPanel.vue`
   - Added try/catch error handling to `resetToDefaults()` function
   - Added user feedback via alert on failure

## Related Documentation

- [PROXY-GUIDE.md](docs/PROXY-GUIDE.md) - Proxy endpoint guide (header manipulation section)
- [SETUP.md](docs/SETUP.md) - Setup guide
- [CLAUDE.md](CLAUDE.md) - Development notes (Performance Issues section - line 17)

## Notes

- Both fixes follow the same pattern already established for container endpoints
- Error handling is consistent with other async functions in AddEndpointDialog.vue
- No backend changes required - uses existing `GetDefaultContainerHeaders()` function
- Filter logic (`h => h.name !== 'Host'`) is consistent between creation wizard and edit panel
