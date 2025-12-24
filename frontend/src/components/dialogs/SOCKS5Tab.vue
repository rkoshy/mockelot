<script lang="ts" setup>
import { ref, watch, computed, onMounted } from 'vue'
import { GetSOCKS5Config } from '../../../wailsjs/go/main/App'

const emit = defineEmits<{
  validationChange: [valid: boolean]
}>()

// SOCKS5 Configuration
const socks5Enabled = ref(false)
const socks5Port = ref(1080)
const socks5Auth = ref(false)
const socks5Username = ref('')
const socks5Password = ref('')
const trackRequests = ref(false)

// Domain Takeover Configuration
const domains = ref<Array<{
  id: string
  pattern: string
  overlayMode: boolean
  enabled: boolean
}>>([])

// Validation state
const portValid = computed(() => {
  return socks5Port.value >= 1 && socks5Port.value <= 65535
})

const authValid = computed(() => {
  if (!socks5Auth.value) return true
  return socks5Username.value.trim() !== '' && socks5Password.value.trim() !== ''
})

const domainsValid = computed(() => {
  // Check that all enabled domains have valid patterns
  return domains.value.every(d => {
    if (!d.enabled) return true
    return d.pattern.trim() !== ''
  })
})

const isValid = computed(() => {
  if (!socks5Enabled.value) return true
  return portValid.value && authValid.value && domainsValid.value
})

watch(isValid, (valid) => {
  emit('validationChange', valid)
})

// Load SOCKS5 config
async function loadSOCKS5Config() {
  try {
    const config = await GetSOCKS5Config()
    if (config.socks5_config) {
      socks5Enabled.value = config.socks5_config.enabled || false
      socks5Port.value = config.socks5_config.port || 1080
      socks5Auth.value = config.socks5_config.authentication || false
      socks5Username.value = config.socks5_config.username || ''
      socks5Password.value = config.socks5_config.password || ''
      trackRequests.value = config.socks5_config.track_requests || false
    }
    if (config.domain_takeover && config.domain_takeover.domains) {
      domains.value = config.domain_takeover.domains.map((d: any) => ({
        id: d.id,
        pattern: d.pattern,
        overlayMode: d.overlay_mode,
        enabled: d.enabled
      }))
    }
  } catch (error) {
    console.error('Failed to load SOCKS5 config:', error)
  }
}

onMounted(() => {
  loadSOCKS5Config()
})

// Domain management
function addDomain() {
  const newId = 'domain-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9)
  domains.value.push({
    id: newId,
    pattern: '',
    overlayMode: true,  // Default to ON
    enabled: true
  })
}

function removeDomain(id: string) {
  const index = domains.value.findIndex(d => d.id === id)
  if (index !== -1) {
    domains.value.splice(index, 1)
  }
}

// Reset port to default
function resetSOCKS5Port() {
  socks5Port.value = 1080
}

// Hosts file helper
const hostsFileEntries = computed(() => {
  return domains.value
    .filter(d => d.enabled && d.pattern.trim() !== '')
    .map(d => `127.0.0.1 ${d.pattern}`)
    .join('\n')
})

async function copyHostsEntries() {
  try {
    await navigator.clipboard.writeText(hostsFileEntries.value)
    // Could show a toast notification here
    console.log('Hosts file entries copied to clipboard')
  } catch (error) {
    console.error('Failed to copy to clipboard:', error)
  }
}

// Expose config for parent
defineExpose({
  getConfig: () => ({
    socks5_config: {
      enabled: socks5Enabled.value,
      port: socks5Port.value,
      authentication: socks5Auth.value,
      username: socks5Username.value,
      password: socks5Password.value,
      track_requests: trackRequests.value,
    },
    domain_takeover: {
      domains: domains.value.map(d => ({
        id: d.id,
        pattern: d.pattern,
        overlay_mode: d.overlayMode,
        enabled: d.enabled
      }))
    }
  })
})
</script>

<template>
  <div class="space-y-6">
    <!-- Enable SOCKS5 -->
    <div>
      <label class="flex items-center gap-2 cursor-pointer">
        <input
          v-model="socks5Enabled"
          type="checkbox"
          class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
        />
        <span class="text-sm font-medium text-white">Enable SOCKS5 Proxy</span>
      </label>
      <p class="mt-1 text-xs text-gray-400 ml-6">
        Allow browsers and apps to proxy HTTP/HTTPS requests through Mockelot
      </p>
    </div>

    <div v-if="socks5Enabled" class="space-y-6">
      <!-- Port Configuration -->
      <div>
        <label class="block text-sm font-medium text-gray-300 mb-2">
          Port
        </label>
        <div class="flex gap-3 items-start">
          <div class="flex-1">
            <input
              v-model.number="socks5Port"
              type="number"
              min="1"
              max="65535"
              :class="[
                'w-full px-3 py-2 bg-gray-700 border rounded text-white',
                portValid ? 'border-gray-600' : 'border-red-500'
              ]"
            />
            <p v-if="!portValid" class="mt-1 text-xs text-red-400">
              Port must be between 1 and 65535
            </p>
          </div>
          <button
            @click="resetSOCKS5Port"
            class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors whitespace-nowrap"
          >
            Reset to Default (1080)
          </button>
        </div>
      </div>

      <!-- Authentication -->
      <div>
        <label class="flex items-center gap-2 cursor-pointer">
          <input
            v-model="socks5Auth"
            type="checkbox"
            class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
          />
          <span class="text-sm font-medium text-white">Require Authentication</span>
        </label>
        <p class="mt-1 text-xs text-gray-400 ml-6">
          Require username/password for SOCKS5 connections
        </p>

        <!-- Credentials (shown only if auth enabled) -->
        <div v-if="socks5Auth" class="mt-3 ml-6 space-y-3">
          <div>
            <label class="block text-sm font-medium text-gray-300 mb-2">
              Username
            </label>
            <input
              v-model="socks5Username"
              type="text"
              placeholder="Enter username"
              :class="[
                'w-full px-3 py-2 bg-gray-700 border rounded text-white',
                authValid || !socks5Auth ? 'border-gray-600' : 'border-red-500'
              ]"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-300 mb-2">
              Password
            </label>
            <input
              v-model="socks5Password"
              type="password"
              placeholder="Enter password"
              :class="[
                'w-full px-3 py-2 bg-gray-700 border rounded text-white',
                authValid || !socks5Auth ? 'border-gray-600' : 'border-red-500'
              ]"
            />
          </div>
          <p v-if="!authValid" class="text-xs text-red-400">
            Username and password are required when authentication is enabled
          </p>
        </div>
      </div>

      <!-- Track Requests -->
      <div>
        <label class="flex items-center gap-2 cursor-pointer">
          <input
            v-model="trackRequests"
            type="checkbox"
            class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
          />
          <span class="text-sm font-medium text-white">Track SOCKS5 Requests</span>
        </label>
        <p class="mt-1 text-xs text-gray-400 ml-6">
          Log all SOCKS5 traffic in the Traffic Log panel (visible in SOCKS5 Proxy endpoint)
        </p>
      </div>

      <!-- Domain Takeover List -->
      <div class="border-t border-gray-700 pt-6">
        <h4 class="text-sm font-semibold text-white mb-3">Intercepted Domains</h4>
        <p class="text-xs text-gray-400 mb-4">
          Configure which domains should be intercepted when using SOCKS5 proxy
        </p>

        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="text-xs uppercase text-gray-400 bg-gray-700/50">
              <tr>
                <th class="px-3 py-2 text-left">Domain Pattern (regex)</th>
                <th class="px-3 py-2 text-center w-32">Overlay Mode</th>
                <th class="px-3 py-2 text-center w-24">Enabled</th>
                <th class="px-3 py-2 text-center w-24">Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="domain in domains"
                :key="domain.id"
                class="border-t border-gray-700 hover:bg-gray-700/30"
              >
                <td class="px-3 py-2">
                  <input
                    v-model="domain.pattern"
                    type="text"
                    placeholder="e.g., api\.example\.com"
                    class="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-sm"
                  />
                </td>
                <td class="px-3 py-2 text-center">
                  <input
                    v-model="domain.overlayMode"
                    type="checkbox"
                    class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
                  />
                </td>
                <td class="px-3 py-2 text-center">
                  <input
                    v-model="domain.enabled"
                    type="checkbox"
                    class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
                  />
                </td>
                <td class="px-3 py-2 text-center">
                  <button
                    @click="removeDomain(domain.id)"
                    class="px-2 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-xs transition-colors"
                  >
                    Delete
                  </button>
                </td>
              </tr>
              <tr v-if="domains.length === 0" class="border-t border-gray-700">
                <td colspan="4" class="px-3 py-4 text-center text-gray-500 text-sm">
                  No domains configured. Click "Add Domain" to get started.
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <button
          @click="addDomain"
          class="mt-3 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors text-sm"
        >
          Add Domain
        </button>
        <p class="mt-2 text-xs text-gray-400">
          New domains default to overlay mode ON (pass through to real server if no endpoint matches)
        </p>
      </div>

      <!-- Hosts File Helper -->
      <div class="border-t border-gray-700 pt-6">
        <h4 class="text-sm font-semibold text-white mb-3">Hosts File Helper</h4>
        <p class="text-xs text-gray-400 mb-3">
          For apps that don't support SOCKS5, add these entries to your hosts file:
        </p>

        <textarea
          readonly
          :value="hostsFileEntries"
          rows="5"
          class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono"
          placeholder="No enabled domains configured"
        />

        <button
          @click="copyHostsEntries"
          :disabled="hostsFileEntries === ''"
          :class="[
            'mt-2 px-4 py-2 rounded transition-colors text-sm',
            hostsFileEntries !== ''
              ? 'bg-gray-700 hover:bg-gray-600 text-gray-300'
              : 'bg-gray-800 text-gray-600 cursor-not-allowed'
          ]"
        >
          Copy to Clipboard
        </button>

        <div class="mt-4 p-3 bg-gray-700/50 rounded text-xs text-gray-300 space-y-1">
          <p><strong class="text-white">Windows:</strong> C:\Windows\System32\drivers\etc\hosts</p>
          <p><strong class="text-white">Linux/macOS:</strong> /etc/hosts</p>
          <p class="text-gray-400 mt-2">Note: Editing hosts file requires administrator/root privileges</p>
        </div>
      </div>

      <!-- Browser Configuration Instructions -->
      <div class="border-t border-gray-700 pt-6">
        <h4 class="text-sm font-semibold text-white mb-3">Browser Setup</h4>
        <p class="text-xs text-gray-400 mb-3">
          Configure your browser's SOCKS5 proxy:
        </p>

        <div class="p-3 bg-gray-700/50 rounded">
          <code class="text-sm text-blue-300">Host: localhost, Port: {{ socks5Port }}</code>
        </div>

        <details class="mt-3">
          <summary class="cursor-pointer text-sm text-blue-400 hover:text-blue-300">
            Browser-specific instructions
          </summary>
          <div class="mt-3 space-y-3 text-xs text-gray-300">
            <div>
              <strong class="text-white">Firefox:</strong>
              <p class="ml-4 mt-1">Settings → Network Settings → Manual proxy configuration</p>
              <p class="ml-4">Set "SOCKS Host" to "localhost" and Port to "{{ socks5Port }}"</p>
              <p class="ml-4">Select "SOCKS v5" and enable "Proxy DNS when using SOCKS v5"</p>
            </div>
            <div>
              <strong class="text-white">Chrome/Edge:</strong>
              <p class="ml-4 mt-1">Use a browser extension like "Proxy SwitchyOmega" or</p>
              <p class="ml-4">Configure system proxy settings (OS-level)</p>
            </div>
            <div>
              <strong class="text-white">cURL:</strong>
              <p class="ml-4 mt-1">
                <code class="text-blue-300">curl --socks5 localhost:{{ socks5Port }} https://api.example.com</code>
              </p>
            </div>
          </div>
        </details>
      </div>

      <!-- Info -->
      <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
        <p class="text-sm font-medium text-blue-300 mb-2">How it Works</p>
        <ul class="text-xs text-blue-300 space-y-1 list-disc list-inside">
          <li>Browser connects to Mockelot via SOCKS5</li>
          <li>Requests to intercepted domains are routed through your endpoints</li>
          <li>Overlay mode passes unmatched requests to real servers</li>
          <li>Non-intercepted domains pass through transparently</li>
        </ul>
      </div>
    </div>
  </div>
</template>
