<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import { useServerStore } from '../../stores/server'
import { ExportLogs } from '../../../wailsjs/go/main/App'
import RequestInspectorModal from '../inspector/RequestInspectorModal.vue'
import WSConnectionTab from './WSConnectionTab.vue'
import SSEConnectionTab from './SSEConnectionTab.vue'
import ConfirmDialog from '../dialogs/ConfirmDialog.vue'
import type { models } from '../../../wailsjs/go/models'

const serverStore = useServerStore()

// Modal state
const showInspectorModal = ref(false)
const inspectorLog = ref<models.RequestLogSummary | null>(null)

// URL filter
const urlFilter = ref('')

// Method filter state
const methodFilters = ref<Record<string, boolean>>({
  GET: true,
  POST: true,
  PUT: true,
  DELETE: true,
  PATCH: true,
  HEAD: true,
  OPTIONS: true,
  WS: true,
  SSE: true,
  OTHER: true,
})

const ALL_METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS', 'WS', 'SSE', 'OTHER']

function setAllFilters(val: boolean) {
  for (const m of ALL_METHODS) methodFilters.value[m] = val
}

function getMethodBucket(log: models.RequestLogSummary): string {
  if (log.is_websocket) return 'WS'
  if (log.is_sse) return 'SSE'
  const m = (log.method || '').toUpperCase()
  return ALL_METHODS.includes(m) ? m : 'OTHER'
}

// Tab state: 'traffic' = main log; string = WS/SSE connection ID open in a tab
const activeTab = ref<string>('traffic')
// Map from connectionID → log summary (for WS tabs)
const wsConnectionTabs = ref<Map<string, models.RequestLogSummary>>(new Map())
// Map from connectionID → log summary (for SSE tabs)
const sseConnectionTabs = ref<Map<string, models.RequestLogSummary>>(new Map())

function openWSTab(log: models.RequestLogSummary) {
  if (!wsConnectionTabs.value.has(log.id)) {
    wsConnectionTabs.value.set(log.id, log)
  }
  activeTab.value = log.id
}

function closeWSTab(id: string) {
  wsConnectionTabs.value.delete(id)
  if (activeTab.value === id) {
    activeTab.value = 'traffic'
  }
}

function openSSETab(log: models.RequestLogSummary) {
  if (!sseConnectionTabs.value.has(log.id)) {
    sseConnectionTabs.value.set(log.id, log)
  }
  activeTab.value = log.id
}

function closeSSETab(id: string) {
  sseConnectionTabs.value.delete(id)
  if (activeTab.value === id) {
    activeTab.value = 'traffic'
  }
}

// Keep WS and SSE tab summaries in sync as logs update
watch(() => serverStore.requestLogs, (logs) => {
  for (const [id] of wsConnectionTabs.value) {
    const updated = logs.find(l => l.id === id)
    if (updated) wsConnectionTabs.value.set(id, updated)
  }
  for (const [id] of sseConnectionTabs.value) {
    const updated = logs.find(l => l.id === id)
    if (updated) sseConnectionTabs.value.set(id, updated)
  }
}, { deep: false })

// WS tabs visible for the currently selected endpoint.
// When no endpoint is selected (showing all), all WS tabs are visible.
const visibleWSTabs = computed(() => {
  const endpointId = serverStore.selectedEndpointId
  if (!endpointId) return [...wsConnectionTabs.value.entries()]
  return [...wsConnectionTabs.value.entries()].filter(([, wsLog]) => wsLog.endpoint_id === endpointId)
})

// SSE tabs visible for the currently selected endpoint.
const visibleSSETabs = computed(() => {
  const endpointId = serverStore.selectedEndpointId
  if (!endpointId) return [...sseConnectionTabs.value.entries()]
  return [...sseConnectionTabs.value.entries()].filter(([, sseLog]) => sseLog.endpoint_id === endpointId)
})

// When the endpoint selection changes, hide any active WS/SSE tab that belongs
// to a different endpoint and fall back to the Traffic Log tab.
watch(() => serverStore.selectedEndpointId, () => {
  if (activeTab.value !== 'traffic') {
    const wsVisible = visibleWSTabs.value.some(([id]) => id === activeTab.value)
    const sseVisible = visibleSSETabs.value.some(([id]) => id === activeTab.value)
    if (!wsVisible && !sseVisible) activeTab.value = 'traffic'
  }
})

// Filter logs
const filteredLogs = computed(() => {
  const endpointId = serverStore.selectedEndpointId
  const filter = urlFilter.value.trim().toLowerCase()

  let logs = endpointId
    ? serverStore.requestLogs.filter(log => log.endpoint_id === endpointId)
    : [...serverStore.requestLogs]

  // Method filter
  logs = logs.filter(log => methodFilters.value[getMethodBucket(log)])

  if (filter) {
    logs = logs.filter(log => {
      const path = (log.path || '').toLowerCase()
      const host = log.target_host ? `${log.target_host}:${log.target_port}`.toLowerCase() : ''
      return path.includes(filter) || host.includes(filter)
    })
  }

  return logs.reverse()
})

function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp)
  const hours = date.getHours().toString().padStart(2, '0')
  const minutes = date.getMinutes().toString().padStart(2, '0')
  const seconds = date.getSeconds().toString().padStart(2, '0')
  const ms = date.getMilliseconds().toString().padStart(3, '0')
  return `${hours}:${minutes}:${seconds}.${ms}`
}

function getMethodColor(method: string): string {
  const colors: Record<string, string> = {
    GET: 'text-green-400',
    POST: 'text-blue-400',
    PUT: 'text-yellow-400',
    DELETE: 'text-red-400',
    PATCH: 'text-purple-400',
    OPTIONS: 'text-gray-400'
  }
  return colors[method] || 'text-gray-400'
}

function getStatusColor(code?: number): string {
  if (code === undefined || code === null) return 'text-gray-400'
  if (code >= 200 && code < 300) return 'text-green-400'
  if (code >= 300 && code < 400) return 'text-yellow-400'
  if (code >= 400 && code < 500) return 'text-orange-400'
  if (code >= 500) return 'text-red-400'
  return 'text-gray-400'
}

function formatStatus(log: models.RequestLogSummary): string {
  if (log.client_status === 0 || log.pending) return 'pending'
  if (log.validation_failed || log.response_failed) return '-'
  return log.client_status?.toString() || 'N/A'
}

function getFailureBadgeText(log: models.RequestLogSummary): string | null {
  if (log.validation_failed) return '(V)'
  if (log.response_failed) return '(R)'
  return null
}

function getFailureBadgeColor(log: models.RequestLogSummary): string {
  if (log.validation_failed) return 'text-yellow-600'
  if (log.response_failed) return 'text-red-600'
  return ''
}

function getFailureBadgeTitle(log: models.RequestLogSummary): string {
  if (log.validation_failed) return 'Validation Failed - Request did not match validation rules, no HTTP response sent'
  if (log.response_failed) return 'Response Failed - Error generating response, jumped to Rejections endpoint'
  return ''
}

function formatRTT(rtt: number | undefined): string {
  if (rtt === undefined || rtt === null) return '-'
  return `${rtt}ms`
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes) return '0B'
  if (bytes < 1024) return `${bytes}B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)}KB`
  return `${(bytes / 1048576).toFixed(1)}MB`
}

// Abbreviate a WS/SSE path for display in a tab label
function wsTabLabel(log: models.RequestLogSummary): string {
  const path = log.path || log.target_host || 'ws'
  return path.length > 20 ? '…' + path.slice(-20) : path
}

function sseTabLabel(log: models.RequestLogSummary): string {
  const path = log.path || log.target_host || 'sse'
  return path.length > 20 ? '…' + path.slice(-20) : path
}

async function handleExportJSON() {
  try { await ExportLogs('json') } catch (e) { console.error(e) }
}

async function handleExportCSV() {
  try { await ExportLogs('csv') } catch (e) { console.error(e) }
}

function openInspector(log: models.RequestLogSummary, event: Event) {
  event.stopPropagation()
  inspectorLog.value = log
  showInspectorModal.value = true
}

function closeInspector() {
  showInspectorModal.value = false
}

// ── Smart Clear (WS-aware) ───────────────────────────────────────────────
const showClearDialog = ref(false)
const activeWSCount = ref(0)

function handleClear() {
  // Count active WS/SSE connections in the scope being cleared
  const endpointId = serverStore.selectedEndpointId
  const scopedLogs = endpointId
    ? serverStore.requestLogs.filter(l => l.endpoint_id === endpointId)
    : serverStore.requestLogs
  const activeWS = scopedLogs.filter(l => (l.is_websocket && l.ws_is_open) || (l.is_sse && l.sse_is_open)).length

  if (activeWS === 0) {
    // No active WS — clear immediately
    if (endpointId) {
      serverStore.clearLogsForEndpoint(endpointId)
    } else {
      serverStore.clearLogs()
    }
    return
  }

  // Active WS exist — show confirmation dialog
  activeWSCount.value = activeWS
  showClearDialog.value = true
}

function handleClearAll() {
  showClearDialog.value = false
  const endpointId = serverStore.selectedEndpointId
  if (endpointId) {
    serverStore.clearLogsForEndpoint(endpointId)
  } else {
    serverStore.clearLogs()
  }
}

function handleClearInactive() {
  showClearDialog.value = false
  const endpointId = serverStore.selectedEndpointId
  if (endpointId) {
    serverStore.clearInactiveLogsForEndpoint(endpointId)
  } else {
    serverStore.clearInactiveLogs()
  }
}

function handleClearCancel() {
  showClearDialog.value = false
}

// Expose openWSTab/openSSETab so the inspector modal can call them
defineExpose({ openWSTab, openSSETab })
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Tab bar -->
    <div class="flex items-center border-b border-gray-700 bg-gray-850 flex-shrink-0 overflow-x-auto">
      <!-- Traffic Log tab (always present) -->
      <button
        @click="activeTab = 'traffic'"
        :class="[
          'px-4 py-2 text-xs font-medium flex-shrink-0 transition-colors border-b-2',
          activeTab === 'traffic'
            ? 'text-white border-blue-500 bg-gray-800'
            : 'text-gray-400 border-transparent hover:text-gray-200 hover:bg-gray-800/50'
        ]"
      >
        Traffic Log
      </button>

      <!-- WS connection tabs (only for the selected endpoint) -->
      <div
        v-for="[id, wsLog] in visibleWSTabs"
        :key="id"
        :class="[
          'flex items-center gap-1 px-3 py-2 flex-shrink-0 border-b-2 cursor-pointer transition-colors',
          activeTab === id
            ? 'border-cyan-500 bg-gray-800'
            : 'border-transparent hover:bg-gray-800/50'
        ]"
        @click="activeTab = id"
      >
        <!-- Active pulse dot -->
        <span
          v-if="wsLog.pending"
          class="w-1.5 h-1.5 rounded-full bg-green-400 animate-pulse flex-shrink-0"
        />
        <span :class="['text-xs font-medium', activeTab === id ? 'text-cyan-400' : 'text-gray-400']">
          {{ wsTabLabel(wsLog) }}
        </span>
        <button
          @click.stop="closeWSTab(id)"
          class="ml-0.5 text-gray-500 hover:text-gray-300 transition-colors"
          title="Close tab"
        >
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- SSE stream tabs (only for the selected endpoint) -->
      <div
        v-for="[id, sseLog] in visibleSSETabs"
        :key="'sse-' + id"
        :class="[
          'flex items-center gap-1 px-3 py-2 flex-shrink-0 border-b-2 cursor-pointer transition-colors',
          activeTab === id
            ? 'border-teal-500 bg-gray-800'
            : 'border-transparent hover:bg-gray-800/50'
        ]"
        @click="activeTab = id"
      >
        <!-- Active pulse dot for open SSE streams -->
        <span
          v-if="sseLog.sse_is_open"
          class="w-1.5 h-1.5 rounded-full bg-teal-400 animate-pulse flex-shrink-0"
        />
        <span class="text-xs font-bold text-teal-500 flex-shrink-0">SSE</span>
        <span :class="['text-xs font-medium', activeTab === id ? 'text-teal-400' : 'text-gray-400']">
          {{ sseTabLabel(sseLog) }}
        </span>
        <button
          @click.stop="closeSSETab(id)"
          class="ml-0.5 text-gray-500 hover:text-gray-300 transition-colors"
          title="Close tab"
        >
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <!-- ── TRAFFIC LOG TAB ── -->
    <template v-if="activeTab === 'traffic'">
      <!-- Header (inline method filters + URL filter + actions — wraps on narrow windows) -->
      <div class="flex flex-wrap items-center gap-x-3 gap-y-2 px-3 py-2 border-b border-gray-700 flex-shrink-0">
        <!-- Method filter checkboxes -->
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
          <label
            v-for="method in ALL_METHODS"
            :key="method"
            class="flex items-center gap-1 cursor-pointer select-none"
          >
            <input
              type="checkbox"
              v-model="methodFilters[method]"
              class="w-3 h-3 rounded accent-blue-500"
            />
            <span :class="['text-xs font-bold', method === 'WS' ? 'text-cyan-400' : getMethodColor(method)]">
              {{ method }}
            </span>
          </label>
          <button @click="setAllFilters(true)"  class="px-1.5 py-0.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-400">All</button>
          <button @click="setAllFilters(false)" class="px-1.5 py-0.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-400">None</button>
        </div>

        <!-- Spacer -->
        <div class="flex-1 min-w-0"></div>

        <!-- URL Filter -->
        <div class="relative flex items-center">
          <input
            v-model="urlFilter"
            type="text"
            placeholder="Filter by URL..."
            class="w-40 pl-2 pr-6 py-1 bg-gray-700 border border-gray-600 rounded text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-blue-500"
          />
          <button
            v-if="urlFilter"
            @click="urlFilter = ''"
            class="absolute right-1 text-gray-400 hover:text-white"
            title="Clear filter"
          >
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <span class="text-xs text-gray-400 whitespace-nowrap">{{ filteredLogs.length }} reqs</span>
        <button @click="handleExportJSON" :disabled="filteredLogs.length === 0" class="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 disabled:opacity-50 disabled:cursor-not-allowed">JSON</button>
        <button @click="handleExportCSV"  :disabled="filteredLogs.length === 0" class="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 disabled:opacity-50 disabled:cursor-not-allowed">CSV</button>
        <button
          @click="handleClear"
          :disabled="filteredLogs.length === 0"
          class="px-2 py-1 bg-red-600 hover:bg-red-700 rounded text-xs text-white disabled:opacity-50 disabled:cursor-not-allowed"
        >{{ serverStore.selectedEndpointId ? 'Clear' : 'Clear All' }}</button>
      </div>

      <!-- Log List -->
      <div class="flex-1 overflow-y-auto">
        <div v-if="filteredLogs.length === 0" class="flex items-center justify-center h-full">
          <div class="text-center text-gray-500">
            <svg class="w-16 h-16 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
                    d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
            <p class="text-lg">No requests yet</p>
            <p class="text-sm mt-1">Start the server and send some requests</p>
          </div>
        </div>

        <div v-else class="divide-y divide-gray-800">
          <div
            v-for="log in filteredLogs"
            :key="log.id"
            @click="serverStore.selectLog(log.id)"
            :class="[
              'px-3 py-2 cursor-pointer transition-colors group',
              serverStore.selectedLogId === log.id
                ? 'bg-blue-900/30 border-l-2 border-blue-500'
                : 'hover:bg-gray-800/50'
            ]"
          >
            <div class="flex items-center gap-3">
              <!-- Time -->
              <span class="text-xs text-gray-500 font-mono w-24 flex-shrink-0">
                {{ formatTimestamp(log.timestamp) }}
              </span>

              <!-- Method / WS / SSE badge -->
              <span v-if="log.is_websocket" class="text-xs font-bold w-14 flex-shrink-0 text-cyan-400 flex items-center gap-1">
                WS
                <span v-if="log.ws_is_open" class="w-1.5 h-1.5 rounded-full bg-green-400 animate-pulse flex-shrink-0" title="Connection open" />
              </span>
              <span v-else-if="log.is_sse" class="text-xs font-bold w-14 flex-shrink-0 text-teal-400 flex items-center gap-1">
                SSE
                <span v-if="log.sse_is_open" class="w-1.5 h-1.5 rounded-full bg-teal-400 animate-pulse flex-shrink-0" title="Stream open" />
              </span>
              <span v-else :class="['text-xs font-bold w-14 flex-shrink-0', getMethodColor(log.method || 'GET')]">
                {{ log.method || 'N/A' }}
              </span>

              <!-- Status / WS frame counts / SSE event counts -->
              <span v-if="log.is_websocket" class="text-xs font-mono w-16 flex-shrink-0">
                <span class="text-violet-400">↑{{ log.ws_frames_sent ?? 0 }}</span>
                <span class="text-teal-400"> ↓{{ log.ws_frames_recv ?? 0 }}</span>
              </span>
              <span v-else-if="log.is_sse" class="text-xs font-mono w-16 flex-shrink-0">
                <span class="text-teal-400">↓{{ log.sse_event_count ?? 0 }}</span>
              </span>
              <span v-else :class="['text-xs font-mono w-16 flex-shrink-0', getStatusColor(log.client_status || 0)]">
                {{ formatStatus(log) }}
              </span>

              <!-- Failure Badge -->
              <span
                v-if="getFailureBadgeText(log)"
                :class="['text-xs font-mono font-bold flex-shrink-0', getFailureBadgeColor(log)]"
                :title="getFailureBadgeTitle(log)"
              >
                {{ getFailureBadgeText(log) }}
              </span>

              <!-- RTT / WS total bytes / SSE total bytes -->
              <span v-if="log.is_websocket" class="text-xs text-gray-400 font-mono w-14 flex-shrink-0 text-right">
                {{ formatBytes(log.ws_bytes_total) }}
              </span>
              <span v-else-if="log.is_sse" class="text-xs text-gray-400 font-mono w-14 flex-shrink-0 text-right">
                {{ formatBytes(log.sse_bytes_total) }}
              </span>
              <span v-else class="text-xs text-gray-400 font-mono w-14 flex-shrink-0 text-right">
                {{ formatRTT(log.client_rtt) }}
              </span>

              <!-- Path / SOCKS5 Target -->
              <span class="text-sm text-gray-300 truncate flex-1 font-mono">
                <span v-if="(log.is_websocket || log.is_sse) && log.path">{{ log.path }}</span>
                <span v-else-if="log.target_host">{{ log.target_host }}:{{ log.target_port }}</span>
                <span v-else>{{ log.path || 'N/A' }}</span>
              </span>

              <!-- Source IP -->
              <span class="text-xs text-gray-500 flex-shrink-0">{{ log.source_ip || 'N/A' }}</span>

              <!-- WS open-tab button — always visible for WS rows -->
              <button
                v-if="log.is_websocket"
                @click.stop="openWSTab(log)"
                class="px-1.5 py-0.5 bg-cyan-900/50 hover:bg-cyan-700/60 border border-cyan-700/50 hover:border-cyan-500 rounded text-cyan-400 hover:text-cyan-300 text-xs font-medium transition-colors flex-shrink-0 flex items-center gap-0.5"
                title="Open WebSocket frame tab"
              >
                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
                Tab
              </button>

              <!-- SSE open-tab button — always visible for SSE rows -->
              <button
                v-if="log.is_sse"
                @click.stop="openSSETab(log)"
                class="px-1.5 py-0.5 bg-teal-900/50 hover:bg-teal-700/60 border border-teal-700/50 hover:border-teal-500 rounded text-teal-400 hover:text-teal-300 text-xs font-medium transition-colors flex-shrink-0 flex items-center gap-0.5"
                title="Open SSE event tab"
              >
                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
                Tab
              </button>

              <!-- Eye Icon Button -->
              <button
                @click="openInspector(log, $event)"
                class="p-1 hover:bg-gray-700 rounded text-gray-400 hover:text-blue-400 transition-colors flex-shrink-0"
                title="Inspect request"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- ── WS CONNECTION TABS ── -->
    <template v-for="[id, wsLog] in visibleWSTabs" :key="id">
      <WSConnectionTab
        v-if="activeTab === id"
        :log-summary="wsLog"
        @open-inspector="(l) => { inspectorLog = l; showInspectorModal = true }"
      />
    </template>

    <!-- ── SSE STREAM TABS ── -->
    <template v-for="[id, sseLog] in visibleSSETabs" :key="'sse-' + id">
      <SSEConnectionTab
        v-if="activeTab === id"
        :log-summary="sseLog"
        @open-inspector="(l) => { inspectorLog = l; showInspectorModal = true }"
      />
    </template>

    <!-- Request Inspector Modal -->
    <RequestInspectorModal
      :show="showInspectorModal"
      :log="inspectorLog"
      @close="closeInspector"
      @open-ws-tab="openWSTab"
    />

    <!-- Clear Confirmation Dialog (shown when active WS connections exist) -->
    <ConfirmDialog
      :show="showClearDialog"
      title="Clear Traffic Logs"
      :message="`There ${activeWSCount === 1 ? 'is 1 active connection' : 'are ' + activeWSCount + ' active connections'} (WebSocket/SSE).\n\nINACTIVE — Clear historical logs and closed connections. Keep active connections.\n\nALL — Clear everything including active WebSocket/SSE connections.`"
      primary-text="All"
      secondary-text="Inactive"
      cancel-text="Cancel"
      @primary="handleClearAll"
      @secondary="handleClearInactive"
      @cancel="handleClearCancel"
    />
  </div>
</template>
