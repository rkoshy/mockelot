<script lang="ts" setup>
import { computed } from 'vue'
import { useServerStore } from '../../stores/server'
import { ExportLogs } from '../../../wailsjs/go/main/App'

const serverStore = useServerStore()

// Reverse logs to show newest first
const reversedLogs = computed(() => [...serverStore.requestLogs].reverse())

function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
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

function getStatusColor(code: number): string {
  if (code >= 200 && code < 300) return 'text-green-400'
  if (code >= 300 && code < 400) return 'text-yellow-400'
  if (code >= 400 && code < 500) return 'text-orange-400'
  if (code >= 500) return 'text-red-400'
  return 'text-gray-400'
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
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between p-3 border-b border-gray-700 flex-shrink-0">
      <h2 class="text-lg font-semibold text-white">Traffic Log</h2>
      <div class="flex items-center gap-2">
        <span class="text-sm text-gray-400">{{ serverStore.requestLogs.length }} requests</span>
        <button
          @click="handleExportJSON"
          :disabled="serverStore.requestLogs.length === 0"
          class="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Export JSON
        </button>
        <button
          @click="handleExportCSV"
          :disabled="serverStore.requestLogs.length === 0"
          class="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Export CSV
        </button>
        <button
          @click="serverStore.clearLogs"
          :disabled="serverStore.requestLogs.length === 0"
          class="px-2 py-1 bg-red-600 hover:bg-red-700 rounded text-xs text-white disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Clear
        </button>
      </div>
    </div>

    <!-- Log List -->
    <div class="flex-1 overflow-y-auto">
      <!-- Empty State -->
      <div v-if="serverStore.requestLogs.length === 0" class="flex items-center justify-center h-full">
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
          v-for="log in reversedLogs"
          :key="log.ID"
          @click="serverStore.selectLog(log.ID)"
          :class="[
            'px-3 py-2 cursor-pointer transition-colors',
            serverStore.selectedLogId === log.ID
              ? 'bg-blue-900/30 border-l-2 border-blue-500'
              : 'hover:bg-gray-800/50'
          ]"
        >
          <div class="flex items-center gap-3">
            <!-- Time -->
            <span class="text-xs text-gray-500 font-mono w-16 flex-shrink-0">
              {{ formatTimestamp(log.Timestamp) }}
            </span>

            <!-- Method Badge -->
            <span :class="['text-xs font-bold w-14 flex-shrink-0', getMethodColor(log.Method)]">
              {{ log.Method }}
            </span>

            <!-- Status Badge -->
            <span :class="['text-xs font-mono w-8 flex-shrink-0', getStatusColor(log.StatusCode)]">
              {{ log.StatusCode }}
            </span>

            <!-- Path -->
            <span class="text-sm text-gray-300 truncate flex-1 font-mono">
              {{ log.Path }}
            </span>

            <!-- Source IP -->
            <span class="text-xs text-gray-500 flex-shrink-0">
              {{ log.SourceIP }}
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
