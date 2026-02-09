<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <div class="px-4 py-3 border-b border-gray-700">
      <h3 class="text-sm font-semibold text-gray-300">Domains Passed Through</h3>
      <p class="text-xs text-gray-500 mt-1">
        {{ filteredDomains.length }}
        {{ filterPattern ? 'filtered' : 'unique' }}
        domain{{ filteredDomains.length !== 1 ? 's' : '' }}
        <span v-if="filterPattern && domains.length > filteredDomains.length">
          ({{ domains.length }} total)
        </span>
      </p>
    </div>

    <!-- Filter Box -->
    <div class="px-4 py-2 border-b border-gray-700">
      <div class="relative">
        <input
          v-model="filterPattern"
          type="text"
          placeholder="Filter domains (regex or substring)..."
          class="w-full px-3 py-1.5 pr-8 bg-gray-800 border border-gray-600 rounded text-sm text-white
                 placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500"
        />
        <button
          v-if="filterPattern"
          @click="filterPattern = ''"
          class="absolute right-2 top-1/2 transform -translate-y-1/2 text-gray-400 hover:text-gray-300"
          title="Clear filter"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <p v-if="filterError" class="text-xs text-red-400 mt-1">{{ filterError }}</p>
    </div>

    <!-- Domain List -->
    <div class="flex-1 overflow-y-auto">
      <div v-if="domains.length === 0" class="px-4 py-8 text-center text-gray-500">
        <p class="mb-2">No domains accessed yet through SOCKS5</p>
        <p class="text-xs">Configure your browser to use SOCKS5 proxy at localhost:{{ socks5Port }}</p>
      </div>

      <div v-else-if="filteredDomains.length === 0" class="px-4 py-8 text-center text-gray-500">
        <p>No domains match the filter "{{ filterPattern }}"</p>
      </div>

      <div v-for="domain in filteredDomains" :key="domain.domain"
           class="px-4 py-3 border-b border-gray-800 hover:bg-gray-800/50 transition-colors">
        <div class="flex items-start justify-between">
          <div class="flex-1 min-w-0">
            <!-- Domain Name with highlighting if filtered -->
            <div class="font-mono text-sm text-white truncate" :title="domain.domain">
              <span v-if="filterPattern && !isRegexPattern" v-html="highlightMatch(domain.domain)"></span>
              <span v-else>{{ domain.domain }}</span>
            </div>

            <!-- Metadata -->
            <div class="flex items-center gap-3 mt-1 text-xs text-gray-500">
              <span>{{ domain.request_count }} request{{ domain.request_count !== 1 ? 's' : '' }}</span>
              <span>•</span>
              <span :title="'First: ' + formatFullTimestamp(domain.first_seen)">
                Last: {{ formatTimestamp(domain.last_seen) }}
              </span>
              <span v-if="domain.is_intercepted" class="text-green-500">• Intercepted</span>
            </div>
          </div>

          <!-- Add Button -->
          <button v-if="!domain.is_configured"
                  @click="addDomain(domain.domain)"
                  :disabled="adding[domain.domain]"
                  class="ml-2 px-3 py-1 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-700 disabled:cursor-not-allowed
                         text-white text-xs font-medium rounded transition-colors"
                  title="Add domain to SOCKS5 takeover list">
            {{ adding[domain.domain] ? 'Adding...' : 'ADD' }}
          </button>

          <!-- Already Configured Badge -->
          <span v-else
                class="ml-2 px-2 py-1 bg-gray-800 text-gray-400 text-xs rounded"
                title="Domain is already in SOCKS5 takeover list">
            Configured
          </span>
        </div>
      </div>
    </div>

    <!-- Footer with Refresh -->
    <div class="px-4 py-2 border-t border-gray-700 flex justify-between items-center">
      <span class="text-xs text-gray-500">
        Auto-refreshes with traffic logs
      </span>
      <button @click="refreshDomains"
              :disabled="loading"
              class="px-3 py-1 text-xs text-blue-400 hover:text-blue-300 disabled:text-gray-500 transition-colors"
              title="Manually refresh domain list">
        {{ loading ? 'Loading...' : 'Refresh' }}
      </button>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { GetSOCKS5Domains, AddDomainToSOCKS5Takeover, GetSOCKS5Config, FrontendLog } from '../../../wailsjs/go/main/App'
import { useServerStore } from '../../stores/server'

// Define the domain info type
interface SOCKS5DomainInfo {
  domain: string
  request_count: number
  first_seen: string
  last_seen: string
  is_configured: boolean
  is_intercepted: boolean
}

const serverStore = useServerStore()
const domains = ref<SOCKS5DomainInfo[]>([])
const loading = ref(false)
const adding = ref<Record<string, boolean>>({})
const refreshInterval = ref<number | null>(null)
const socks5Port = ref(1080)
const filterPattern = ref('')
const filterError = ref('')

// Check if pattern has regex special characters (excluding ^ and $ which we handle specially)
const regexSpecialChars = /[.*+?{}()|[\]\\]/

// Check if the filter pattern is a regex or substring
const isRegexPattern = computed(() => {
  if (!filterPattern.value) return false
  // Check for anchors or special regex chars
  return filterPattern.value.startsWith('^') ||
         filterPattern.value.endsWith('$') ||
         regexSpecialChars.test(filterPattern.value)
})

// Filtered and sorted domains
const filteredDomains = computed(() => {
  let result = [...domains.value]

  // Apply filter if present
  if (filterPattern.value) {
    filterError.value = ''

    try {
      if (isRegexPattern.value) {
        // Use as regex
        const regex = new RegExp(filterPattern.value, 'i')
        result = result.filter(d => regex.test(d.domain))
      } else {
        // Use as case-insensitive substring search
        const searchTerm = filterPattern.value.toLowerCase()
        result = result.filter(d => d.domain.toLowerCase().includes(searchTerm))
      }
    } catch (e) {
      // Invalid regex
      filterError.value = `Invalid regex: ${e.message}`
      return []
    }
  }

  // Sort alphabetically by domain name
  result.sort((a, b) => a.domain.localeCompare(b.domain))

  return result
})

// Highlight matching substring in domain name (for substring search only)
function highlightMatch(domain: string): string {
  if (!filterPattern.value || isRegexPattern.value) return domain

  const searchTerm = filterPattern.value.toLowerCase()
  const index = domain.toLowerCase().indexOf(searchTerm)

  if (index === -1) return domain

  const before = domain.substring(0, index)
  const match = domain.substring(index, index + filterPattern.value.length)
  const after = domain.substring(index + filterPattern.value.length)

  return `${escapeHtml(before)}<span class="text-blue-400 font-semibold">${escapeHtml(match)}</span>${escapeHtml(after)}`
}

function escapeHtml(text: string): string {
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

async function refreshDomains() {
  loading.value = true
  try {
    domains.value = await GetSOCKS5Domains()
  } catch (error) {
    await FrontendLog(`[SOCKS5] Failed to get domains: ${error}`)
  } finally {
    loading.value = false
  }
}

async function loadSOCKS5Config() {
  try {
    const config = await GetSOCKS5Config()
    if (config.socks5_config?.port) {
      socks5Port.value = config.socks5_config.port
    }
  } catch (error) {
    await FrontendLog(`[SOCKS5] Failed to get config: ${error}`)
  }
}

async function addDomain(domain: string) {
  adding.value[domain] = true
  try {
    await AddDomainToSOCKS5Takeover(domain, true) // Enable overlay by default

    // Log success
    await FrontendLog(`[SOCKS5] Added domain to takeover list: ${domain}`)

    // Refresh the server store config so the Server tab updates
    await serverStore.refreshConfig()

    // Refresh to update the is_configured status
    await refreshDomains()
  } catch (error) {
    await FrontendLog(`[SOCKS5] Failed to add domain: ${error}`)
  } finally {
    adding.value[domain] = false
  }
}

function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const seconds = Math.floor(diff / 1000)

  if (seconds < 60) return 'Just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`
  return date.toLocaleDateString()
}

function formatFullTimestamp(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleString()
}

onMounted(() => {
  loadSOCKS5Config()
  refreshDomains()
  // Auto-refresh every 5 seconds
  refreshInterval.value = window.setInterval(refreshDomains, 5000)
})

onUnmounted(() => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
  }
})
</script>