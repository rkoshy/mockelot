<script lang="ts" setup>
import { ref, computed } from 'vue'
import { useServerStore } from '../../stores/server'
import { ExportLogs } from '../../../wailsjs/go/main/App'
import RequestInspectorModal from '../inspector/RequestInspectorModal.vue'
import type { models } from '../../../wailsjs/go/models'

const serverStore = useServerStore()

// Modal state
const showInspectorModal = ref(false)
const inspectorLog = ref<models.RequestLogSummary | null>(null)

// Filter logs by selected endpoint, then reverse to show newest first
const filteredLogs = computed(() => {
  const endpointId = serverStore.selectedEndpointId
  if (!endpointId) {
    return [...serverStore.requestLogs].reverse()
  }
  return serverStore.requestLogs
    .filter(log => log.endpoint_id === endpointId)
    .reverse()
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
  // Check if pending (status is 0 or pending flag is true)
  if (log.client_status === 0 || log.pending) {
    return 'pending'
  }
  // If validation or response failed, there's no HTTP status
  if (log.validation_failed || log.response_failed) {
    return '-'
  }
  return log.client_status?.toString() || 'N/A'
}

function getFailureBadgeText(log: models.RequestLogSummary): string | null {
  if (log.validation_failed) {
    return '(V)'
  }
  if (log.response_failed) {
    return '(R)'
  }
  return null
}

function getFailureBadgeColor(log: models.RequestLogSummary): string {
  if (log.validation_failed) {
    return 'text-yellow-600'
  }
  if (log.response_failed) {
    return 'text-red-600'
  }
  return ''
}

function getFailureBadgeTitle(log: models.RequestLogSummary): string {
  if (log.validation_failed) {
    return 'Validation Failed - Request did not match validation rules, no HTTP response sent'
  }
  if (log.response_failed) {
    return 'Response Failed - Error generating response, jumped to Rejections endpoint'
  }
  return ''
}

function formatRTT(rtt: number | undefined): string {
  if (rtt === undefined || rtt === null) return '-'
  return `${rtt}ms`
}

async function handleExportJSON() {
  try {
    await ExportLogs('json')
  } catch (error) {
    console.error('Failed to export logs:', error)
  }
}

async function handleExportCSV() {
  try {
    await ExportLogs('csv')
  } catch (error) {
    console.error('Failed to export logs:', error)
  }
}

function openInspector(log: models.RequestLogSummary, event: Event) {
  event.stopPropagation() // Prevent row click
  inspectorLog.value = log
  showInspectorModal.value = true
}

function closeInspector() {
  showInspectorModal.value = false
}
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between p-3 border-b border-gray-700 flex-shrink-0">
      <h2 class="text-lg font-semibold text-white">Traffic Log</h2>
      <div class="flex items-center gap-2">
        <span class="text-sm text-gray-400">{{ filteredLogs.length }} requests</span>
        <button
          @click="handleExportJSON"
          :disabled="filteredLogs.length === 0"
          class="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Export JSON
        </button>
        <button
          @click="handleExportCSV"
          :disabled="filteredLogs.length === 0"
          class="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Export CSV
        </button>
        <button
          @click="serverStore.clearLogs"
          :disabled="filteredLogs.length === 0"
          class="px-2 py-1 bg-red-600 hover:bg-red-700 rounded text-xs text-white disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Clear
        </button>
      </div>
    </div>

    <!-- Log List -->
    <div class="flex-1 overflow-y-auto">
      <!-- Empty State -->
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

      <!-- Request Items -->
      <div v-else class="divide-y divide-gray-800">
        <div
          v-for="log in filteredLogs"
          :key="log.id"
          @click="serverStore.selectLog(log.id)"
          :class="[
            'px-3 py-2 cursor-pointer transition-colors',
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

            <!-- Method Badge -->
            <span :class="['text-xs font-bold w-14 flex-shrink-0', getMethodColor(log.method || 'GET')]">
              {{ log.method || 'N/A' }}
            </span>

            <!-- Status Badge -->
            <span :class="['text-xs font-mono w-16 flex-shrink-0', getStatusColor(log.client_status || 0)]">
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

            <!-- RTT -->
            <span class="text-xs text-gray-400 font-mono w-14 flex-shrink-0 text-right">
              {{ formatRTT(log.client_rtt) }}
            </span>

            <!-- Path / SOCKS5 Target -->
            <span class="text-sm text-gray-300 truncate flex-1 font-mono">
              <span v-if="log.target_host">
                {{ log.target_host }}:{{ log.target_port }}
              </span>
              <span v-else>
                {{ log.path || 'N/A' }}
              </span>
            </span>

            <!-- Source IP -->
            <span class="text-xs text-gray-500 flex-shrink-0">
              {{ log.source_ip || 'N/A' }}
            </span>

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

    <!-- Request Inspector Modal -->
    <RequestInspectorModal
      :show="showInspectorModal"
      :log="inspectorLog"
      @close="closeInspector"
    />
  </div>
</template>
