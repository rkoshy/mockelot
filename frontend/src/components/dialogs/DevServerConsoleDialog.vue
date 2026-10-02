<script lang="ts" setup>
import { ref, watch, nextTick, onUnmounted } from 'vue'
import { GetDevServerLogs, ClearDevServerLogs } from '../../../wailsjs/go/main/App'

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
const tail = ref(5000)
const consoleEl = ref<HTMLElement | null>(null)

// Auto-refresh controls
const autoRefreshEnabled = ref(true)
const autoRefreshInterval = ref(2) // 2s feels live enough for a dev server
let refreshIntervalId: number | null = null

watch(() => props.show, async (newValue) => {
  if (newValue && props.endpointId) {
    await loadLogs()
    if (autoRefreshEnabled.value) startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
})

watch(autoRefreshEnabled, (newValue) => {
  if (newValue && props.show) startAutoRefresh()
  else stopAutoRefresh()
})

watch(autoRefreshInterval, () => {
  if (autoRefreshEnabled.value && props.show) {
    stopAutoRefresh()
    startAutoRefresh()
  }
})

function startAutoRefresh() {
  stopAutoRefresh()
  if (autoRefreshInterval.value > 0) {
    refreshIntervalId = window.setInterval(loadLogs, autoRefreshInterval.value * 1000)
  }
}

function stopAutoRefresh() {
  if (refreshIntervalId !== null) {
    clearInterval(refreshIntervalId)
    refreshIntervalId = null
  }
}

onUnmounted(() => stopAutoRefresh())

async function loadLogs() {
  loading.value = true
  error.value = ''
  try {
    logs.value = await GetDevServerLogs(props.endpointId, tail.value)
    // Auto-scroll to bottom after update
    await nextTick()
    if (consoleEl.value) {
      consoleEl.value.scrollTop = consoleEl.value.scrollHeight
    }
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
  }
}

async function handleClear() {
  await ClearDevServerLogs(props.endpointId)
  logs.value = ''
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
    <div class="bg-gray-800 rounded-lg border border-gray-700 w-[85vw] flex flex-col max-h-[90vh]">
      <!-- Header -->
      <div class="px-4 py-3 border-b border-gray-700 flex items-center justify-between">
        <h3 class="text-lg font-semibold text-white">
          Dev Server Console: <span class="text-orange-400">{{ endpointName }}</span>
        </h3>
        <div class="flex items-center gap-3">
          <!-- Auto-refresh controls -->
          <div class="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              id="ds-auto-refresh"
              v-model="autoRefreshEnabled"
              class="w-4 h-4 bg-gray-700 border-gray-600 rounded text-orange-500 focus:ring-orange-500"
            />
            <label for="ds-auto-refresh" class="text-gray-300 cursor-pointer">Auto-refresh</label>
            <input
              v-model.number="autoRefreshInterval"
              type="number"
              min="1"
              max="60"
              :disabled="!autoRefreshEnabled"
              class="w-16 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-sm disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-orange-500"
            />
            <span class="text-gray-400 text-xs">sec</span>
          </div>

          <!-- Clear -->
          <button
            @click="handleClear"
            class="px-3 py-1.5 bg-gray-700 hover:bg-red-900/60 text-gray-400 hover:text-red-300 rounded text-sm font-medium transition-colors"
          >
            Clear
          </button>

          <!-- Manual refresh -->
          <button
            @click="loadLogs"
            :disabled="loading"
            class="px-3 py-1.5 bg-orange-600 hover:bg-orange-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded text-sm font-medium transition-colors"
          >
            {{ loading ? 'Loading...' : 'Refresh' }}
          </button>

          <!-- Close -->
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

      <!-- Console output -->
      <div ref="consoleEl" class="flex-1 overflow-y-auto p-4 bg-black font-mono text-sm">
        <div v-if="loading && !logs" class="text-gray-500">Loading output...</div>
        <div v-else-if="error" class="text-red-400">Error: {{ error }}</div>
        <div v-else-if="!logs" class="text-gray-500">No output yet. Start the dev server to see output here.</div>
        <pre v-else class="text-green-300 whitespace-pre-wrap">{{ logs }}</pre>
      </div>

      <!-- Footer -->
      <div class="px-4 py-3 border-t border-gray-700 flex items-center justify-between">
        <div class="text-xs text-gray-400">Showing last {{ tail }} lines</div>
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
