<template>
  <div class="space-y-6">
    <!-- Upstream DNS Servers -->
    <div>
      <h4 class="text-sm font-medium text-gray-300 mb-3">Upstream DNS Servers</h4>
      <p class="text-xs text-gray-400 mb-3">
        Configure DNS servers for resolving domains. Applies to SOCKS5h (remote DNS) and UDP ASSOCIATE.
      </p>

      <!-- DNS Provider Selection -->
      <div class="space-y-3">
        <div>
          <label class="block text-xs font-medium text-gray-400 mb-2">
            DNS Provider
          </label>
          <div class="relative">
            <button
              @click.stop="dnsProviderDropdownOpen = !dnsProviderDropdownOpen"
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm text-left flex items-center justify-between hover:bg-gray-650 transition-colors"
            >
              <span>{{ selectedProviderName }}</span>
              <svg class="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
              </svg>
            </button>

            <!-- Custom Dropdown Menu -->
            <div v-if="dnsProviderDropdownOpen"
                 class="absolute z-10 w-full mt-1 bg-gray-800 border border-gray-600 rounded shadow-lg max-h-60 overflow-y-auto">
              <div v-for="provider in providersArray"
                   :key="provider.id"
                   @click="selectDNSProvider(provider.id)"
                   class="px-3 py-2 hover:bg-gray-700 cursor-pointer transition-colors">
                <div class="text-sm text-white">{{ provider.name }}</div>
                <div class="text-xs text-gray-400 mt-0.5">{{ provider.description }}</div>
              </div>
            </div>
          </div>
        </div>

        <!-- Show server list for selected provider or custom input -->
        <div v-if="selectedDNSProvider !== 'system'">
          <label class="block text-xs font-medium text-gray-400 mb-2">
            DNS Servers
          </label>

          <div v-if="selectedDNSProvider === 'custom'">
            <textarea
              v-model="customDNSServers"
              @input="updateCustomServers"
              placeholder="Enter DNS server IPs (one per line)&#10;8.8.8.8&#10;8.8.4.4"
              rows="3"
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p class="mt-1 text-xs text-gray-500">
              Enter one IP address per line (IPv4 only)
            </p>
          </div>

          <div v-else class="p-3 bg-gray-700/50 rounded">
            <div v-for="(server, index) in currentServers" :key="index"
                 class="font-mono text-sm text-blue-300">
              {{ server }}
            </div>
          </div>
        </div>

        <!-- System DNS Info -->
        <div v-if="selectedDNSProvider === 'system'" class="p-3 bg-gray-700/50 rounded">
          <p class="text-xs text-gray-300">
            Using system DNS servers from /etc/resolv.conf
          </p>
          <div v-if="config.upstreamServers.length > 0" class="mt-2">
            <div v-for="server in config.upstreamServers" :key="server"
                 class="font-mono text-sm text-blue-300">
              {{ server }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- DNS Override Rules -->
    <div class="border-t border-gray-700 pt-6">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h4 class="text-sm font-medium text-gray-300">DNS Override Rules</h4>
          <p class="text-xs text-gray-400 mt-1">
            Override DNS resolution for specific domains (regex patterns)
          </p>
        </div>
        <button
          @click="addOverride"
          class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-xs font-medium rounded transition-colors"
        >
          + Add Rule
        </button>
      </div>

      <!-- Rules Table -->
      <div v-if="config.overrides.length > 0" class="bg-gray-800 rounded border border-gray-700 overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="border-b border-gray-700">
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-400">Pattern</th>
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-400">Type</th>
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-400">Target</th>
              <th class="px-3 py-2 text-center text-xs font-medium text-gray-400">Priority</th>
              <th class="px-3 py-2 text-center text-xs font-medium text-gray-400">Enabled</th>
              <th class="px-3 py-2 text-center text-xs font-medium text-gray-400">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(override, index) in config.overrides" :key="override.id"
                class="border-b border-gray-700/50 hover:bg-gray-750">
              <td class="px-3 py-2">
                <input v-model="override.pattern"
                       @input="emitChange"
                       class="w-full bg-transparent text-sm font-mono text-white focus:outline-none focus:ring-1 focus:ring-blue-500 rounded px-1"
                       placeholder="^api\.example\.com$">
              </td>
              <td class="px-3 py-2">
                <div class="relative">
                  <button
                    @click.stop="toggleTypeDropdown(override.id)"
                    class="w-full px-2 py-1 bg-gray-700 rounded text-sm text-white text-left hover:bg-gray-650 transition-colors"
                  >
                    {{ override.type === 'static' ? 'Static IP' : 'CNAME' }}
                  </button>
                  <div v-if="typeDropdownOpen === override.id"
                       class="absolute z-10 mt-1 w-32 bg-gray-800 border border-gray-600 rounded shadow-lg">
                    <div @click="setOverrideType(override, 'static')"
                         class="px-3 py-2 hover:bg-gray-700 cursor-pointer text-sm text-white">
                      Static IP
                    </div>
                    <div @click="setOverrideType(override, 'cname')"
                         class="px-3 py-2 hover:bg-gray-700 cursor-pointer text-sm text-white">
                      CNAME
                    </div>
                  </div>
                </div>
              </td>
              <td class="px-3 py-2">
                <input v-model="override.target"
                       @input="emitChange"
                       class="w-full bg-transparent text-sm font-mono text-white focus:outline-none focus:ring-1 focus:ring-blue-500 rounded px-1"
                       :placeholder="override.type === 'static' ? '127.0.0.1' : 'localhost'">
              </td>
              <td class="px-3 py-2 text-center">
                <input v-model.number="override.priority"
                       @input="emitChange"
                       type="number"
                       min="0"
                       class="w-16 bg-transparent text-sm text-white text-center focus:outline-none focus:ring-1 focus:ring-blue-500 rounded px-1">
              </td>
              <td class="px-3 py-2 text-center">
                <input type="checkbox"
                       v-model="override.enabled"
                       @change="emitChange"
                       class="rounded text-blue-600">
              </td>
              <td class="px-3 py-2 text-center">
                <button @click="deleteOverride(index)"
                        class="text-red-400 hover:text-red-300 text-sm">
                  Delete
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-else class="p-8 bg-gray-800/50 rounded border border-gray-700 text-center">
        <p class="text-sm text-gray-500">No DNS override rules configured</p>
        <p class="text-xs text-gray-600 mt-2">Click "Add Rule" to create your first override</p>
      </div>

      <!-- Pattern Examples -->
      <div class="mt-4 p-3 bg-gray-800 rounded border border-gray-700">
        <h5 class="text-xs font-medium text-gray-400 mb-2">Pattern Examples</h5>
        <div class="space-y-1 text-xs font-mono text-gray-500">
          <div><span class="text-gray-400">^api\.example\.com$</span> - Exact match</div>
          <div><span class="text-gray-400">.*\.example\.com</span> - All subdomains</div>
          <div><span class="text-gray-400">^(www\.)?example\.com$</span> - With optional www</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { v4 as uuidv4 } from 'uuid'

// Props and emits
const props = defineProps<{
  config: {
    enabled: boolean
    overrides: Array<{
      id: string
      pattern: string
      type: 'static' | 'cname'
      target: string
      priority: number
      enabled: boolean
    }>
    upstreamServers: string[]
    useSystemDNS: boolean
  }
  providers: Record<string, any>
}>()

const emit = defineEmits<{
  'update:config': [value: typeof props.config]
  'change': []
}>()

// Local state
const selectedDNSProvider = ref<string>('system')
const customDNSServers = ref<string>('')
const dnsProviderDropdownOpen = ref(false)
const typeDropdownOpen = ref<string | null>(null)

// Convert providers object to array for dropdown
const providersArray = computed(() => {
  const arr = Object.entries(props.providers).map(([id, provider]) => ({
    id,
    ...provider
  }))
  // Add custom option
  arr.push({
    id: 'custom',
    name: 'Custom DNS Servers',
    description: 'Manually specify DNS server IPs'
  })
  return arr
})

// Get selected provider name
const selectedProviderName = computed(() => {
  if (selectedDNSProvider.value === 'custom') {
    return 'Custom DNS Servers'
  }
  return props.providers[selectedDNSProvider.value]?.name || 'System Default'
})

// Get current servers based on selection
const currentServers = computed(() => {
  if (selectedDNSProvider.value === 'custom') {
    return customDNSServers.value.split('\n').filter(s => s.trim())
  }
  return props.providers[selectedDNSProvider.value]?.servers || []
})

// Initialize selected provider based on config
watch(() => props.config, (newConfig) => {
  if (!newConfig) return

  if (newConfig.useSystemDNS) {
    selectedDNSProvider.value = 'system'
    customDNSServers.value = ''
  } else if (newConfig.upstreamServers && newConfig.upstreamServers.length > 0) {
    // Check if these match a known provider
    const serversStr = newConfig.upstreamServers.join(',')
    let foundProvider = false

    for (const [key, provider] of Object.entries(props.providers)) {
      if (provider.servers && provider.servers.join(',') === serversStr) {
        selectedDNSProvider.value = key
        foundProvider = true
        break
      }
    }

    if (!foundProvider) {
      selectedDNSProvider.value = 'custom'
      customDNSServers.value = newConfig.upstreamServers.join('\n')
    }
  }
}, { immediate: true })

function selectDNSProvider(providerId: string) {
  selectedDNSProvider.value = providerId
  dnsProviderDropdownOpen.value = false

  let newConfig = { ...props.config }

  if (providerId === 'system') {
    newConfig.useSystemDNS = true
    newConfig.upstreamServers = []
  } else if (providerId === 'custom') {
    newConfig.useSystemDNS = false
    // Parse custom servers
    newConfig.upstreamServers = customDNSServers.value.split('\n')
      .map(s => s.trim())
      .filter(s => s.length > 0)
  } else {
    newConfig.useSystemDNS = false
    newConfig.upstreamServers = props.providers[providerId]?.servers || []
  }

  emit('update:config', newConfig)
  emitChange()
}

function updateCustomServers() {
  if (selectedDNSProvider.value === 'custom') {
    const newConfig = { ...props.config }
    newConfig.useSystemDNS = false
    newConfig.upstreamServers = customDNSServers.value.split('\n')
      .map(s => s.trim())
      .filter(s => s.length > 0)
    emit('update:config', newConfig)
    emitChange()
  }
}

function addOverride() {
  const newConfig = { ...props.config }
  const newPriority = newConfig.overrides.length > 0
    ? Math.max(...newConfig.overrides.map(o => o.priority)) + 1
    : 0

  newConfig.overrides.push({
    id: uuidv4(),
    pattern: '',
    type: 'static',
    target: '',
    priority: newPriority,
    enabled: true
  })

  emit('update:config', newConfig)
  emitChange()
}

function deleteOverride(index: number) {
  const newConfig = { ...props.config }
  newConfig.overrides.splice(index, 1)

  // Reorder priorities
  newConfig.overrides.forEach((o, i) => {
    o.priority = i
  })

  emit('update:config', newConfig)
  emitChange()
}

function toggleTypeDropdown(id: string) {
  if (typeDropdownOpen.value === id) {
    typeDropdownOpen.value = null
  } else {
    typeDropdownOpen.value = id
  }
}

function setOverrideType(override: any, type: 'static' | 'cname') {
  override.type = type
  typeDropdownOpen.value = null
  emitChange()
}

function emitChange() {
  emit('change')
}

// Click outside handler
function handleClickOutside() {
  dnsProviderDropdownOpen.value = false
  typeDropdownOpen.value = null
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>