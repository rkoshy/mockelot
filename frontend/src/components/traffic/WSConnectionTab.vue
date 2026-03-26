<script lang="ts" setup>
import { ref, computed, watch, onUnmounted, nextTick } from 'vue'
import { useServerStore } from '../../stores/server'
import type { models } from '../../../wailsjs/go/models'

interface Props {
  logSummary: models.RequestLogSummary
}

const props = defineProps<Props>()
const emit = defineEmits<{
  openInspector: [log: models.RequestLogSummary]
}>()

const serverStore = useServerStore()

// Full log (polled periodically to pick up new frames)
const fullLog = ref<models.RequestLog | null>(null)
const autoScroll = ref(true)
const frameListEl = ref<HTMLElement | null>(null)
const expandedFrameId = ref<string | null>(null)

// Opcode filters
const opcodeFilters = ref({
  TEXT: true,
  BINARY: true,
  'PING/PONG': true,
  CLOSE: true,
})

const OPCODE_BUCKETS: Record<string, string> = {
  TEXT: 'TEXT',
  BINARY: 'BINARY',
  PING: 'PING/PONG',
  PONG: 'PING/PONG',
  CLOSE: 'CLOSE',
  CONTINUATION: 'TEXT',
}

function getOpcodeBucket(opcode: string): string {
  return OPCODE_BUCKETS[opcode] || 'TEXT'
}

// Derived: is the connection still active (pending = still open)?
const isActive = computed(() => props.logSummary.pending)

// Filtered frames
const filteredFrames = computed(() => {
  if (!fullLog.value?.websocket_events) return []
  return fullLog.value.websocket_events.filter(e =>
    opcodeFilters.value[getOpcodeBucket(e.opcode)]
  )
})

const framesSent = computed(() =>
  fullLog.value?.websocket_events?.filter(e => e.direction === 'send').length ?? 0
)
const framesRecv = computed(() =>
  fullLog.value?.websocket_events?.filter(e => e.direction === 'recv').length ?? 0
)

// Duration: time from connection start to last frame (or now if active)
const durationText = computed(() => {
  if (!fullLog.value?.websocket_events?.length) return '—'
  const last = fullLog.value.websocket_events.at(-1)!
  const ms = last.offset_ms
  return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`
})

// Connection URL
const connectionURL = computed(() => {
  const s = props.logSummary
  return s.path
    ? (s.target_host
        ? `wss://${s.target_host}:${s.target_port}${s.path}`
        : s.path)
    : s.target_host
        ? `wss://${s.target_host}`
        : '—'
})

// Poll for new frames while the tab is open
let pollTimer: ReturnType<typeof setInterval> | null = null

async function fetchFrames() {
  const log = await serverStore.getLogDetails(props.logSummary.id)
  if (log) {
    const prevCount = fullLog.value?.websocket_events?.length ?? 0
    fullLog.value = log
    // Auto-scroll if new frames arrived
    if (autoScroll.value && log.websocket_events && log.websocket_events.length > prevCount) {
      await nextTick()
      frameListEl.value?.scrollTo({ top: frameListEl.value.scrollHeight, behavior: 'smooth' })
    }
  }
}

// Start polling immediately and on interval
fetchFrames()
pollTimer = setInterval(fetchFrames, 800)

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})

// Stop polling when connection is closed
watch(isActive, (active) => {
  if (!active && pollTimer) {
    // Do one final fetch then stop
    fetchFrames().then(() => {
      if (pollTimer) clearInterval(pollTimer)
      pollTimer = null
    })
  }
})

function toggleExpand(id: string) {
  expandedFrameId.value = expandedFrameId.value === id ? null : id
}

function formatOffset(ms: number): string {
  if (ms < 1000) return `+${ms}ms`
  return `+${(ms / 1000).toFixed(3)}s`
}

function getOpcodeColor(opcode: string): string {
  switch (opcode) {
    case 'TEXT': return 'text-gray-300'
    case 'BINARY': return 'text-yellow-400'
    case 'PING': case 'PONG': return 'text-gray-500'
    case 'CLOSE': return 'text-red-400'
    default: return 'text-gray-400'
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
    <!-- Connection header -->
    <div class="px-4 py-3 border-b border-gray-700 flex-shrink-0 bg-gray-900/50">
      <div class="flex items-center gap-3 mb-1">
        <!-- Active / closed indicator -->
        <span v-if="isActive" class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-green-400 animate-pulse"></span>
          <span class="text-xs text-green-400 font-medium">ACTIVE</span>
        </span>
        <span v-else class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-gray-500"></span>
          <span class="text-xs text-gray-500 font-medium">CLOSED</span>
        </span>

        <span class="text-sm text-cyan-300 font-mono truncate flex-1">{{ connectionURL }}</span>

        <button
          @click="emit('openInspector', logSummary)"
          class="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors flex-shrink-0"
          title="Open upgrade request inspector"
        >
          Upgrade Request
        </button>
      </div>

      <div class="flex items-center gap-4 text-xs text-gray-500">
        <span>Duration: <span class="text-gray-300">{{ durationText }}</span></span>
        <span>
          <span class="text-violet-400">↑ {{ framesSent }}</span>
          <span class="mx-1">·</span>
          <span class="text-teal-400">↓ {{ framesRecv }}</span>
          <span class="ml-1 text-gray-500">frames</span>
        </span>
      </div>
    </div>

    <!-- Filter + auto-scroll bar -->
    <div class="px-4 py-2 border-b border-gray-700 flex-shrink-0 flex items-center gap-4">
      <span class="text-xs text-gray-500 font-medium">Show:</span>
      <label v-for="(_, bucket) in opcodeFilters" :key="bucket" class="flex items-center gap-1 cursor-pointer select-none">
        <input type="checkbox" v-model="opcodeFilters[bucket]" class="w-3 h-3 rounded accent-blue-500" />
        <span :class="['text-xs font-medium', getOpcodeColor(bucket === 'PING/PONG' ? 'PING' : bucket)]">
          {{ bucket }}
        </span>
      </label>
      <div class="ml-auto flex items-center gap-1">
        <label class="flex items-center gap-1 cursor-pointer select-none">
          <input type="checkbox" v-model="autoScroll" class="w-3 h-3 rounded accent-blue-500" />
          <span class="text-xs text-gray-400">Auto-scroll</span>
        </label>
      </div>
    </div>

    <!-- Frame list -->
    <div ref="frameListEl" class="flex-1 overflow-y-auto font-mono">
      <!-- Empty state -->
      <div v-if="filteredFrames.length === 0" class="flex flex-col items-center justify-center h-full text-gray-600">
        <svg class="w-12 h-12 mb-3 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
        </svg>
        <p class="text-sm">{{ isActive ? 'Waiting for frames…' : 'No frames captured' }}</p>
      </div>

      <!-- Frame rows -->
      <div v-else class="divide-y divide-gray-800/60">
        <div
          v-for="frame in filteredFrames"
          :key="frame.id"
          class="cursor-pointer hover:bg-gray-800/40 transition-colors"
          @click="toggleExpand(frame.id)"
        >
          <!-- Summary row -->
          <div class="flex items-center gap-3 px-4 py-1.5 text-xs">
            <!-- Offset -->
            <span class="w-20 text-gray-500 flex-shrink-0">{{ formatOffset(frame.offset_ms) }}</span>

            <!-- Direction arrow -->
            <span :class="['w-4 text-center font-bold flex-shrink-0', getDirectionColor(frame.direction)]">
              {{ frame.direction === 'send' ? '↑' : '↓' }}
            </span>

            <!-- Opcode -->
            <span :class="['w-16 flex-shrink-0 font-medium', getOpcodeColor(frame.opcode)]">
              {{ frame.opcode }}
            </span>

            <!-- Size -->
            <span class="w-14 text-right text-gray-500 flex-shrink-0">{{ formatSize(frame.data_size) }}</span>

            <!-- Preview -->
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
              <template v-else>
                {{ frame.data_preview }}
              </template>
            </span>

            <!-- Expand chevron -->
            <span class="text-gray-600 flex-shrink-0">
              {{ expandedFrameId === frame.id ? '▲' : '▼' }}
            </span>
          </div>

          <!-- Expanded data -->
          <div
            v-if="expandedFrameId === frame.id && frame.data_preview"
            class="px-4 pb-3"
          >
            <pre class="bg-gray-900 rounded p-3 text-xs text-gray-300 whitespace-pre-wrap break-all overflow-auto max-h-64">{{ frame.data_preview }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
