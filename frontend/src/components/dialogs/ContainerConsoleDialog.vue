<script lang="ts" setup>
import { ref, watch, onUnmounted } from 'vue'
import { GetContainerLogs } from '../../../wailsjs/go/main/App'

const props = defineProps<{
  show: boolean
  endpointId: string
  endpointName: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const logs = ref<string>('')
const loading = ref(false)
const error = ref<string>('')
const tail = ref(5000) // Default log line limit

// Auto-refresh controls
const autoRefreshEnabled = ref(true) // Enabled by default
const autoRefreshInterval = ref(5) // Default 5 seconds
let refreshIntervalId: number | null = null

// Load logs when dialog is shown
watch(() => props.show, async (newValue) => {
  if (newValue && props.endpointId) {
    await loadLogs()
    // Start auto-refresh if enabled
    if (autoRefreshEnabled.value) {
      startAutoRefresh()
    }
  } else {
    // Stop auto-refresh when dialog is hidden
    stopAutoRefresh()
  }
})

// Watch auto-refresh enabled state
watch(autoRefreshEnabled, (newValue) => {
  if (newValue && props.show) {
    startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
})

// Watch auto-refresh interval changes
watch(autoRefreshInterval, () => {
  if (autoRefreshEnabled.value && props.show) {
    // Restart auto-refresh with new interval
    stopAutoRefresh()
    startAutoRefresh()
  }
})

// Start auto-refresh timer
function startAutoRefresh() {
  stopAutoRefresh() // Clear any existing interval
  if (autoRefreshInterval.value > 0) {
    refreshIntervalId = window.setInterval(() => {
      loadLogs()
    }, autoRefreshInterval.value * 1000)
  }
}

// Stop auto-refresh timer
function stopAutoRefresh() {
  if (refreshIntervalId !== null) {
    clearInterval(refreshIntervalId)
    refreshIntervalId = null
  }
}

// Clean up on component unmount
onUnmounted(() => {
  stopAutoRefresh()
})

async function loadLogs() {
  loading.value = true
  error.value = ''
  logs.value = ''

  try {
    logs.value = await GetContainerLogs(props.endpointId, tail.value)
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
  }
}

async function handleRefresh() {
  await loadLogs()
}

function handleClose() {
  stopAutoRefresh()
  emit('close')
}
</script>

<template>
  <div
    v-if="show"
    class="fixed inset-0 bg-black/80 flex items-center justify-center p-4 z-50"
    @click.self="handleClose"
  >
    <div class="bg-gray-800 rounded-lg border border-gray-700 w-[80vw] flex flex-col max-h-[90vh]">
      <!-- Header -->
      <div class="px-4 py-3 border-b border-gray-700 flex items-center justify-between">
        <h3 class="text-lg font-semibold text-white">
          Container Console: {{ endpointName }}
        </h3>
        <div class="flex items-center gap-3">
          <!-- Auto-refresh controls -->
          <div class="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              id="auto-refresh"
              v-model="autoRefreshEnabled"
              class="w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600 focus:ring-blue-500"
            />
            <label for="auto-refresh" class="text-gray-300 cursor-pointer">
              Auto-refresh
            </label>
            <input
              v-model.number="autoRefreshInterval"
              type="number"
              min="1"
              max="60"
              :disabled="!autoRefreshEnabled"
              class="w-16 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-sm disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <span class="text-gray-400 text-xs">sec</span>
          </div>

          <!-- Manual refresh button -->
          <button
            @click="handleRefresh"
            :disabled="loading"
            class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded text-sm font-medium transition-colors"
          >
            {{ loading ? 'Loading...' : 'Refresh' }}
          </button>

          <!-- Close button -->
          <button
            @click="handleClose"
            class="p-1 hover:bg-gray-700 rounded transition-colors text-gray-400 hover:text-white"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Console Content -->
      <div class="flex-1 overflow-y-auto p-4 bg-black font-mono text-sm">
        <div v-if="loading" class="text-gray-500">
          Loading logs...
        </div>
        <div v-else-if="error" class="text-red-400">
          Error: {{ error }}
        </div>
        <div v-else-if="!logs" class="text-gray-500">
          No logs available
        </div>
        <pre v-else class="text-green-400 whitespace-pre-wrap">{{ logs }}</pre>
      </div>

      <!-- Footer -->
      <div class="px-4 py-3 border-t border-gray-700 flex items-center justify-between">
        <div class="text-xs text-gray-400">
          Showing last {{ tail }} lines
        </div>
        <button
          @click="handleClose"
          class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded text-sm font-medium transition-colors"
        >
          Close
        </button>
      </div>
    </div>
  </div>
</template>
