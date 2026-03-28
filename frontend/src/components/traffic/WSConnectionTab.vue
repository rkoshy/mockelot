<script lang="ts" setup>
import { ref, computed, watch, onUnmounted, nextTick } from 'vue'
import { useServerStore } from '../../stores/server'
import { ExportWSTrace, TerminateWSConnection, SetWSConnectionBlocked, ClearWSFrames } from '../../../wailsjs/go/main/App'
import type { models } from '../../../wailsjs/go/models'

interface Props {
  logSummary: models.RequestLogSummary
}

const props = defineProps<Props>()
const emit = defineEmits<{
  openInspector: [log: models.RequestLogSummary]
}>()

const serverStore = useServerStore()

// Full log — fetched only when the frame count changes (Option B: optimised polling)
const fullLog = ref<models.RequestLog | null>(null)
const autoScroll = ref(true)
const frameListEl = ref<HTMLElement | null>(null)
const expandedFrameId = ref<string | null>(null)

// Opcode filters
const opcodeFilters = ref({ TEXT: true, BINARY: true, 'PING/PONG': true, CLOSE: true })
const OPCODE_BUCKETS: Record<string, string> = {
  TEXT: 'TEXT', BINARY: 'BINARY', PING: 'PING/PONG', PONG: 'PING/PONG',
  CLOSE: 'CLOSE', CONTINUATION: 'TEXT',
}

// Content + size filters
const contentFilter = ref('')
const sizeMin = ref<number | null>(null)
const sizeMax = ref<number | null>(null)

// ── Live state from summary (reactive, no polling needed) ──────────────────
const isActive   = computed(() => props.logSummary.ws_is_open)
const framesSent = computed(() => props.logSummary.ws_frames_sent ?? 0)
const framesRecv = computed(() => props.logSummary.ws_frames_recv ?? 0)
const totalFrames = computed(() => framesSent.value + framesRecv.value)

const connectionURL = computed(() => {
  const s = props.logSummary
  const path = s.path || ''
  if (s.target_host) {
    const scheme = s.ws_is_open || s.ws_opened_at ? 'wss' : 'ws'
    return `${scheme}://${s.target_host}:${s.target_port}${path}`
  }
  return path || '—'
})

const durationText = computed(() => {
  const opened = props.logSummary.ws_opened_at
  if (!opened) return '—'
  const start = new Date(opened).getTime()
  const endStr = props.logSummary.ws_closed_at
  const end = endStr ? new Date(endStr).getTime() : Date.now()
  const ms = end - start
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`
})

const closeLabel = computed(() => {
  const code = props.logSummary.ws_close_code
  if (!code) return 'CLOSED'
  return `CLOSED (${code})`
})

// ── Frame list from full log (polled only when count changes) ──────────────
const filteredFrames = computed(() => {
  if (!fullLog.value?.websocket_events) return []
  const content = contentFilter.value.trim().toLowerCase()
  const min = sizeMin.value
  const max = sizeMax.value
  return fullLog.value.websocket_events.filter(e => {
    if (!opcodeFilters.value[OPCODE_BUCKETS[e.opcode] ?? 'TEXT']) return false
    if (min !== null && e.data_size < min) return false
    if (max !== null && e.data_size > max) return false
    if (content && !(e.data_preview ?? '').toLowerCase().includes(content)) return false
    return true
  })
})

const totalVisible = computed(() => filteredFrames.value.length)
const totalAll = computed(() => fullLog.value?.websocket_events?.length ?? 0)

// Track the last frame count we fetched at so we skip redundant fetches.
let lastFetchedFrameCount = -1

async function fetchFrames() {
  const count = totalFrames.value
  if (count === lastFetchedFrameCount && fullLog.value !== null) return
  lastFetchedFrameCount = count

  const log = await serverStore.getLogDetails(props.logSummary.id, true) // force-fresh
  if (!log) return
  const prevLen = fullLog.value?.websocket_events?.length ?? 0
  fullLog.value = log

  if (autoScroll.value && (log.websocket_events?.length ?? 0) > prevLen) {
    await nextTick()
    frameListEl.value?.scrollTo({ top: frameListEl.value.scrollHeight, behavior: 'smooth' })
  }
}

// Poll at 500ms — skips the backend call unless frame count changed.
let pollTimer: ReturnType<typeof setInterval> | null = setInterval(fetchFrames, 500)
fetchFrames()

// Stop polling once closed (after one final fetch to pick up any trailing frames).
watch(isActive, (open) => {
  if (!open && pollTimer) {
    setTimeout(() => {
      fetchFrames().then(() => {
        if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
      })
    }, 600) // brief delay so CloseWebSocketConnection summary has propagated
  }
})

onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })

function toggleExpand(id: string) {
  expandedFrameId.value = expandedFrameId.value === id ? null : id
}

async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    // fallback: select text
  }
}

async function handleExportTrace() {
  try {
    await ExportWSTrace(props.logSummary.id)
  } catch (e) {
    console.error('ExportWSTrace failed:', e)
  }
}

async function handleClearFrames() {
  try {
    await ClearWSFrames(props.logSummary.id)
    if (fullLog.value) {
      fullLog.value.websocket_events = []
    }
    lastFetchedFrameCount = -1
  } catch (e) {
    console.error('ClearWSFrames failed:', e)
  }
}

// ── TERMINATE and BLOCKED ─────────────────────────────────────────────────
const isBlocked = ref(false)

async function handleTerminate() {
  try {
    await TerminateWSConnection(props.logSummary.id)
  } catch (e) {
    console.error('TerminateWSConnection failed:', e)
  }
}

async function handleSetBlocked(blocked: boolean) {
  isBlocked.value = blocked
  try {
    await SetWSConnectionBlocked(props.logSummary.id, blocked)
  } catch (e) {
    console.error('SetWSConnectionBlocked failed:', e)
    isBlocked.value = !blocked // revert on failure
  }
}

// Reset blocked state when the connection is (re-)opened
watch(isActive, (open) => { if (open) isBlocked.value = false })

function formatOffset(ms: number): string {
  if (ms < 1000) return `+${ms}ms`
  return `+${(ms / 1000).toFixed(3)}s`
}

function getOpcodeColor(opcode: string): string {
  switch (opcode) {
    case 'TEXT':   return 'text-gray-300'
    case 'BINARY': return 'text-yellow-400'
    case 'PING': case 'PONG': return 'text-gray-500'
    case 'CLOSE':  return 'text-red-400'
    default:       return 'text-gray-400'
  }
}

function getDirectionColor(dir: string): string {
  return dir === 'send' ? 'text-violet-400' : 'text-teal-400'
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`
  return `${(bytes / 1024).toFixed(1)}KB`
}
</script>

<template>
  <div class="flex-1 flex flex-col min-h-0">
    <!-- Connection header (all live data from summary — no full-log fetch needed) -->
    <div class="px-4 py-3 border-b border-gray-700 flex-shrink-0 bg-gray-900/50">
      <div class="flex items-center gap-3 mb-1">
        <span v-if="isActive" class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-green-400 animate-pulse"></span>
          <span class="text-xs text-green-400 font-medium">ACTIVE</span>
        </span>
        <span v-else class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-gray-500"></span>
          <span class="text-xs text-gray-500 font-medium">{{ closeLabel }}</span>
        </span>

        <span class="text-sm text-cyan-300 font-mono truncate flex-1">{{ connectionURL }}</span>

        <!-- BLOCKED toggle — only when connection is active -->
        <label
          v-if="isActive"
          class="flex items-center gap-1.5 cursor-pointer select-none flex-shrink-0"
          title="Block all frame forwarding (connections stay open, traffic is dropped)"
        >
          <input
            type="checkbox"
            :checked="isBlocked"
            @change="handleSetBlocked(($event.target as HTMLInputElement).checked)"
            class="w-3.5 h-3.5 rounded accent-orange-500"
          />
          <span :class="['text-xs font-medium', isBlocked ? 'text-orange-400' : 'text-gray-400']">
            {{ isBlocked ? 'BLOCKED' : 'BLOCK' }}
          </span>
        </label>

        <!-- TERMINATE button — only when connection is active -->
        <button
          v-if="isActive"
          @click="handleTerminate"
          class="px-2 py-1 bg-red-800/60 hover:bg-red-700/80 border border-red-700/60 hover:border-red-500 rounded text-xs text-red-300 hover:text-red-200 font-medium transition-colors flex-shrink-0"
          title="Force-close this WebSocket connection (both client and backend sides)"
        >
          TERMINATE
        </button>

        <button
          @click="emit('openInspector', logSummary)"
          class="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors flex-shrink-0"
          title="Inspect upgrade handshake"
        >
          Upgrade Request
        </button>
        <button
          @click="handleClearFrames"
          :disabled="totalAll === 0"
          class="px-2 py-1 bg-red-800/60 hover:bg-red-700/80 border border-red-700/60 hover:border-red-500 rounded text-xs text-red-300 hover:text-red-200 font-medium transition-colors flex-shrink-0 disabled:opacity-50 disabled:cursor-not-allowed"
          title="Clear all captured frames for this connection"
        >
          Clear
        </button>
        <button
          @click="handleExportTrace"
          :disabled="filteredFrames.length === 0"
          class="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors flex-shrink-0 disabled:opacity-50 disabled:cursor-not-allowed"
          title="Export full trace to file"
        >
          Export Trace
        </button>
      </div>

      <div class="flex items-center gap-4 text-xs text-gray-500">
        <span>Duration: <span class="text-gray-300">{{ durationText }}</span></span>
        <span>
          <span class="text-violet-400">↑ {{ framesSent }}</span>
          <span class="mx-1">·</span>
          <span class="text-teal-400">↓ {{ framesRecv }}</span>
          <span class="ml-1">frames</span>
        </span>
        <span v-if="logSummary.ws_bytes_total">
          {{ logSummary.ws_bytes_total < 1024
              ? logSummary.ws_bytes_total + 'B'
              : (logSummary.ws_bytes_total / 1024).toFixed(1) + 'KB' }}
          total
        </span>
      </div>
    </div>

    <!-- Filter bar — row 1: opcode + auto-scroll -->
    <div class="px-4 py-1.5 border-b border-gray-700/60 flex-shrink-0 flex items-center gap-4">
      <span class="text-xs text-gray-500 font-medium">Show:</span>
      <label v-for="(_, bucket) in opcodeFilters" :key="bucket" class="flex items-center gap-1 cursor-pointer select-none">
        <input type="checkbox" v-model="opcodeFilters[bucket]" class="w-3 h-3 rounded accent-blue-500" />
        <span :class="['text-xs font-medium', getOpcodeColor(bucket === 'PING/PONG' ? 'PING' : bucket)]">{{ bucket }}</span>
      </label>
      <div class="ml-auto flex items-center gap-3">
        <span class="text-xs text-gray-500">
          {{ totalVisible }} / {{ totalAll }} frames
        </span>
        <label class="flex items-center gap-1 cursor-pointer select-none">
          <input type="checkbox" v-model="autoScroll" class="w-3 h-3 rounded accent-blue-500" />
          <span class="text-xs text-gray-400">Auto-scroll</span>
        </label>
      </div>
    </div>

    <!-- Filter bar — row 2: content search + size range -->
    <div class="px-4 py-1.5 border-b border-gray-700 flex-shrink-0 flex items-center gap-3 flex-wrap">
      <!-- Content search -->
      <div class="relative flex items-center">
        <input
          v-model="contentFilter"
          type="text"
          placeholder="Search content…"
          class="w-44 pl-2 pr-6 py-1 bg-gray-700 border border-gray-600 rounded text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-blue-500"
        />
        <button
          v-if="contentFilter"
          @click="contentFilter = ''"
          class="absolute right-1 text-gray-400 hover:text-white"
          title="Clear"
        >
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- Size range -->
      <div class="flex items-center gap-1 text-xs text-gray-400">
        <span>Size:</span>
        <input
          v-model.number="sizeMin"
          type="number"
          min="0"
          placeholder="min"
          class="w-16 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-blue-500"
        />
        <span>–</span>
        <input
          v-model.number="sizeMax"
          type="number"
          min="0"
          placeholder="max"
          class="w-16 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-blue-500"
        />
        <span>bytes</span>
        <button
          v-if="sizeMin !== null || sizeMax !== null"
          @click="sizeMin = null; sizeMax = null"
          class="ml-1 text-gray-500 hover:text-gray-300 text-xs"
          title="Clear size filter"
        >
          ✕
        </button>
      </div>
    </div>

    <!-- Frame list -->
    <div ref="frameListEl" class="flex-1 overflow-y-auto font-mono">
      <div v-if="filteredFrames.length === 0" class="flex flex-col items-center justify-center h-full text-gray-600">
        <svg class="w-12 h-12 mb-3 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
        </svg>
        <p class="text-sm">
          {{ totalAll === 0
              ? (isActive ? 'Waiting for frames…' : 'No frames captured')
              : 'No frames match the current filters' }}
        </p>
      </div>

      <div v-else class="divide-y divide-gray-800/60">
        <div
          v-for="frame in filteredFrames"
          :key="frame.id"
          class="cursor-pointer hover:bg-gray-800/40 transition-colors"
          @click="toggleExpand(frame.id)"
        >
          <!-- Summary row -->
          <div class="flex items-center gap-3 px-4 py-1.5 text-xs">
            <span class="w-20 text-gray-500 flex-shrink-0">{{ formatOffset(frame.offset_ms) }}</span>
            <span :class="['w-4 text-center font-bold flex-shrink-0', getDirectionColor(frame.direction)]">
              {{ frame.direction === 'send' ? '↑' : '↓' }}
            </span>
            <span :class="['w-16 flex-shrink-0 font-medium', getOpcodeColor(frame.opcode)]">{{ frame.opcode }}</span>
            <span class="w-14 text-right text-gray-500 flex-shrink-0">{{ formatSize(frame.data_size) }}</span>
            <span class="flex-1 truncate text-gray-400">
              <template v-if="frame.opcode === 'PING' || frame.opcode === 'PONG'">
                <span class="text-gray-600 italic">(heartbeat)</span>
              </template>
              <template v-else-if="frame.opcode === 'CLOSE'">
                <span class="text-red-400">{{ frame.close_code }} {{ frame.close_text || 'Connection closed' }}</span>
              </template>
              <template v-else-if="frame.opcode === 'BINARY'">
                <span class="text-gray-600 italic">[binary — {{ formatSize(frame.data_size) }}]</span>
              </template>
              <template v-else>{{ frame.data_preview }}</template>
            </span>
            <span class="text-gray-600 flex-shrink-0">{{ expandedFrameId === frame.id ? '▲' : '▼' }}</span>
          </div>

          <!-- Expanded data -->
          <div v-if="expandedFrameId === frame.id && frame.data_preview" class="px-4 pb-3">
            <div class="flex items-center justify-between mb-1">
              <span class="text-xs text-gray-500 font-mono">
                {{ frame.opcode === 'BINARY' ? 'hex' : 'text' }} · {{ frame.data_size }} bytes total
              </span>
              <button
                @click.stop="copyToClipboard(frame.data_preview)"
                class="px-2 py-0.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors"
                title="Copy to clipboard"
              >
                Copy
              </button>
            </div>
            <pre class="bg-gray-900 rounded p-3 text-xs text-gray-300 whitespace-pre-wrap break-all overflow-auto max-h-96">{{ frame.data_preview }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
