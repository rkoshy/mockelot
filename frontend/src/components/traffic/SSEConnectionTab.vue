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

// Full log — fetched only when the event count changes (same optimised polling as WS)
const fullLog = ref<models.RequestLog | null>(null)
const autoScroll = ref(true)
const eventListEl = ref<HTMLElement | null>(null)
const expandedEventId = ref<string | null>(null)

// Event type filters — populated dynamically from seen events
const eventTypeFilters = ref<Record<string, boolean>>({ message: true })

// Content filter
const contentFilter = ref('')

// ── Live state from summary (reactive, no extra polling needed) ───────────
const isActive    = computed(() => props.logSummary.sse_is_open)
const eventCount  = computed(() => props.logSummary.sse_event_count ?? 0)
const bytesTotal  = computed(() => props.logSummary.sse_bytes_total ?? 0)

const connectionURL = computed(() => {
  const s = props.logSummary
  const path = s.path || ''
  if (s.target_host) {
    return `${s.target_host}:${s.target_port}${path}`
  }
  return path || '—'
})

const durationText = computed(() => {
  const opened = props.logSummary.sse_opened_at
  if (!opened) return '—'
  const start = new Date(opened).getTime()
  const endStr = props.logSummary.sse_closed_at
  const end = endStr ? new Date(endStr).getTime() : Date.now()
  const ms = end - start
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`
})

// ── Event list from full log (polled only when count changes) ─────────────
const filteredEvents = computed(() => {
  if (!fullLog.value?.sse_events) return []
  const content = contentFilter.value.trim().toLowerCase()
  return fullLog.value.sse_events.filter(e => {
    // Type filter: ensure the event_type has a bucket
    const typeBucket = e.event_type || 'message'
    if (eventTypeFilters.value[typeBucket] === false) return false
    if (content && !e.data.toLowerCase().includes(content)) return false
    return true
  })
})

const totalVisible = computed(() => filteredEvents.value.length)
const totalAll = computed(() => fullLog.value?.sse_events?.length ?? 0)

// Track last fetched count to skip redundant fetches.
let lastFetchedEventCount = -1

async function fetchEvents() {
  const count = eventCount.value
  if (count === lastFetchedEventCount && fullLog.value !== null) return
  lastFetchedEventCount = count

  const log = await serverStore.getLogDetails(props.logSummary.id, true) // force-fresh
  if (!log) return
  const prevLen = fullLog.value?.sse_events?.length ?? 0
  fullLog.value = log

  // Update known event types for filters
  if (log.sse_events) {
    for (const e of log.sse_events) {
      const t = e.event_type || 'message'
      if (!(t in eventTypeFilters.value)) {
        eventTypeFilters.value[t] = true
      }
    }
  }

  if (autoScroll.value && (log.sse_events?.length ?? 0) > prevLen) {
    await nextTick()
    eventListEl.value?.scrollTo({ top: eventListEl.value.scrollHeight, behavior: 'smooth' })
  }
}

// Poll at 500ms — same cadence as WS tab
let pollTimer: ReturnType<typeof setInterval> | null = setInterval(fetchEvents, 500)
fetchEvents()

// Stop polling once closed (after one final fetch to pick up trailing events).
watch(isActive, (open) => {
  if (!open && pollTimer) {
    setTimeout(() => {
      fetchEvents().then(() => {
        if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
      })
    }, 600)
  }
})

onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })

function toggleExpand(id: string) {
  expandedEventId.value = expandedEventId.value === id ? null : id
}

async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    // ignore clipboard errors
  }
}

function formatOffset(ms: number): string {
  if (ms < 1000) return `+${ms}ms`
  return `+${(ms / 1000).toFixed(3)}s`
}

function getEventTypeColor(eventType: string): string {
  switch (eventType) {
    case 'message': return 'text-teal-400'
    case 'error':   return 'text-red-400'
    case 'open':    return 'text-green-400'
    case 'close':   return 'text-orange-400'
    default:        return 'text-violet-400'
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`
  return `${(bytes / 1024).toFixed(1)}KB`
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes) return '0B'
  if (bytes < 1024) return `${bytes}B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)}KB`
  return `${(bytes / 1048576).toFixed(1)}MB`
}

// All known event types (for filter UI)
const knownEventTypes = computed(() => Object.keys(eventTypeFilters.value))
</script>

<template>
  <div class="flex-1 flex flex-col min-h-0">
    <!-- Stream header -->
    <div class="px-4 py-3 border-b border-gray-700 flex-shrink-0 bg-gray-900/50">
      <div class="flex items-center gap-3 mb-1">
        <span v-if="isActive" class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-green-400 animate-pulse"></span>
          <span class="text-xs text-green-400 font-medium">STREAMING</span>
        </span>
        <span v-else class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-gray-500"></span>
          <span class="text-xs text-gray-500 font-medium">CLOSED</span>
        </span>

        <span class="text-sm text-teal-300 font-mono truncate flex-1">{{ connectionURL }}</span>

        <button
          @click="emit('openInspector', logSummary)"
          class="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors flex-shrink-0"
          title="Inspect SSE request headers"
        >
          Request
        </button>
      </div>

      <div class="flex items-center gap-4 text-xs text-gray-500">
        <span>Duration: <span class="text-gray-300">{{ durationText }}</span></span>
        <span>
          <span class="text-teal-400">↓ {{ eventCount }}</span>
          <span class="ml-1">events</span>
        </span>
        <span v-if="bytesTotal">
          {{ formatBytes(bytesTotal) }} total
        </span>
        <span v-if="logSummary.sse_closed_at && !isActive" class="text-gray-600">stream ended</span>
      </div>
    </div>

    <!-- Filter bar — event type + auto-scroll -->
    <div class="px-4 py-1.5 border-b border-gray-700/60 flex-shrink-0 flex items-center gap-4 flex-wrap">
      <span class="text-xs text-gray-500 font-medium">Type:</span>
      <label
        v-for="typeName in knownEventTypes"
        :key="typeName"
        class="flex items-center gap-1 cursor-pointer select-none"
      >
        <input type="checkbox" v-model="eventTypeFilters[typeName]" class="w-3 h-3 rounded accent-blue-500" />
        <span :class="['text-xs font-medium', getEventTypeColor(typeName)]">{{ typeName }}</span>
      </label>
      <div class="ml-auto flex items-center gap-3">
        <span class="text-xs text-gray-500">
          {{ totalVisible }} / {{ totalAll }} events
        </span>
        <label class="flex items-center gap-1 cursor-pointer select-none">
          <input type="checkbox" v-model="autoScroll" class="w-3 h-3 rounded accent-blue-500" />
          <span class="text-xs text-gray-400">Auto-scroll</span>
        </label>
      </div>
    </div>

    <!-- Filter bar — content search -->
    <div class="px-4 py-1.5 border-b border-gray-700 flex-shrink-0 flex items-center gap-3">
      <div class="relative flex items-center">
        <input
          v-model="contentFilter"
          type="text"
          placeholder="Search data…"
          class="w-48 pl-2 pr-6 py-1 bg-gray-700 border border-gray-600 rounded text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-blue-500"
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
    </div>

    <!-- Event list -->
    <div ref="eventListEl" class="flex-1 overflow-y-auto font-mono">
      <div v-if="filteredEvents.length === 0" class="flex flex-col items-center justify-center h-full text-gray-600">
        <svg class="w-12 h-12 mb-3 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
        </svg>
        <p class="text-sm">
          {{ totalAll === 0
              ? (isActive ? 'Waiting for events…' : 'No events captured')
              : 'No events match the current filters' }}
        </p>
      </div>

      <div v-else class="divide-y divide-gray-800/60">
        <div
          v-for="evt in filteredEvents"
          :key="evt.id"
          class="cursor-pointer hover:bg-gray-800/40 transition-colors"
          @click="toggleExpand(evt.id)"
        >
          <!-- Summary row -->
          <div class="flex items-center gap-3 px-4 py-1.5 text-xs">
            <span class="w-20 text-gray-500 flex-shrink-0">{{ formatOffset(evt.offset_ms) }}</span>
            <span class="text-teal-400 w-4 text-center font-bold flex-shrink-0">↓</span>
            <span :class="['w-20 flex-shrink-0 font-medium truncate', getEventTypeColor(evt.event_type || 'message')]">
              {{ evt.event_type || 'message' }}
            </span>
            <span class="w-14 text-right text-gray-500 flex-shrink-0">{{ formatSize(evt.data_size) }}</span>
            <span v-if="evt.event_id" class="text-gray-600 flex-shrink-0 text-xs">#{{ evt.event_id }}</span>
            <span class="flex-1 truncate text-gray-400">{{ evt.data }}</span>
            <span class="text-gray-600 flex-shrink-0">{{ expandedEventId === evt.id ? '▲' : '▼' }}</span>
          </div>

          <!-- Expanded data -->
          <div v-if="expandedEventId === evt.id" class="px-4 pb-3">
            <div class="flex items-center justify-between mb-1">
              <span class="text-xs text-gray-500 font-mono">
                {{ evt.data_size }} bytes · event {{ evt.event_type || 'message' }}
                <span v-if="evt.event_id" class="text-gray-600"> · id: {{ evt.event_id }}</span>
                <span v-if="evt.retry" class="text-gray-600"> · retry: {{ evt.retry }}ms</span>
              </span>
              <div class="flex items-center gap-2">
                <button
                  @click.stop="copyToClipboard(evt.data)"
                  class="px-2 py-0.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors"
                  title="Copy data to clipboard"
                >
                  Copy data
                </button>
                <button
                  @click.stop="copyToClipboard(evt.raw_text)"
                  class="px-2 py-0.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors"
                  title="Copy full raw event block"
                >
                  Copy raw
                </button>
              </div>
            </div>
            <pre class="bg-gray-900 rounded p-3 text-xs text-gray-300 whitespace-pre-wrap break-all overflow-auto max-h-96">{{ evt.data }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
