<script lang="ts" setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useServerStore } from '../../stores/server'
import type { models } from '../../../wailsjs/go/models'

// ── Props / Emits ─────────────────────────────────────────────────────────
const props = defineProps<{
  endpoints: models.Endpoint[]
  selectedId: string
}>()

const emit = defineEmits<{
  select:       [id: string]
  addEndpoint:  []
  settings:     [endpoint: models.Endpoint]
  deleteEp:     [endpoint: models.Endpoint]
}>()

const serverStore = useServerStore()

// ── Width (resizable, persisted) ──────────────────────────────────────────
const navWidth = ref(220)
const isResizing = ref(false)

onMounted(() => {
  const w = parseInt(localStorage.getItem('mockelot-nav-width') ?? '')
  if (w) navWidth.value = Math.min(320, Math.max(160, w))
})

function startResize(e: MouseEvent) {
  isResizing.value = true
  const startX = e.clientX
  const startW = navWidth.value
  const onMove = (e: MouseEvent) => {
    navWidth.value = Math.min(320, Math.max(160, startW + (e.clientX - startX)))
  }
  const onUp = () => {
    isResizing.value = false
    localStorage.setItem('mockelot-nav-width', String(navWidth.value))
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

// ── Filter ────────────────────────────────────────────────────────────────
const filter = ref('')
const filterInputRef = ref<HTMLInputElement | null>(null)
const isFiltering = computed(() => filter.value.trim().length > 0)

// ── Group collapse (persisted) ────────────────────────────────────────────
const GROUPS = ['mock', 'proxy', 'container', 'overlays'] as const
const collapsed = ref<Record<string, boolean>>(Object.fromEntries(GROUPS.map(k => [k, false])))

onMounted(() => {
  try {
    const s = JSON.parse(localStorage.getItem('mockelot-nav-collapsed') ?? '{}')
    Object.assign(collapsed.value, s)
  } catch {}
})

watch(collapsed, v => localStorage.setItem('mockelot-nav-collapsed', JSON.stringify(v)), { deep: true })

function toggleGroup(key: string) {
  collapsed.value[key] = !collapsed.value[key]
}

// ── Endpoint grouping ─────────────────────────────────────────────────────
function isOverlay(ep: models.Endpoint) { return ep.id?.startsWith('system-overlay-') }
function isSystemNonOverlay(ep: models.Endpoint) { return !!ep.is_system && !isOverlay(ep) }

const mockEps      = computed(() => props.endpoints.filter(ep => ep.type === 'mock'      && !ep.is_system))
const proxyEps     = computed(() => props.endpoints.filter(ep => ep.type === 'proxy'     && !ep.is_system && !isOverlay(ep)))
const containerEps = computed(() => props.endpoints.filter(ep => ep.type === 'container' && !ep.is_system))
const overlayEps   = computed(() => props.endpoints.filter(ep => isOverlay(ep)))
const systemEps    = computed(() => props.endpoints.filter(ep => isSystemNonOverlay(ep)))

// All user-filterable endpoints in display order
const allFilterable = computed(() => [
  ...mockEps.value, ...proxyEps.value, ...containerEps.value,
  ...overlayEps.value, ...systemEps.value,
])

const filteredList = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return []
  return allFilterable.value.filter(ep => {
    const name = (ep.name ?? '').toLowerCase()
    const path = (ep.path_prefix ?? '').toLowerCase()
    return name.includes(q) || path.includes(q)
  })
})

// ── Display helpers ───────────────────────────────────────────────────────
function displayName(ep: models.Endpoint): string {
  if (ep.name?.startsWith('Overlay: ')) return ep.name.slice(9)
  return ep.name ?? ep.id ?? ''
}

function typeLabel(ep: models.Endpoint): { text: string; cls: string } {
  if (isOverlay(ep))              return { text: 'O', cls: 'text-orange-400' }
  if (ep.type === 'proxy')        return { text: 'P', cls: 'text-blue-400'   }
  if (ep.type === 'container')    return { text: 'C', cls: 'text-purple-400' }
  if (ep.id === 'system-socks5-proxy') return { text: '✦', cls: 'text-yellow-400' }
  if (ep.id === 'system-rejections')   return { text: '✗', cls: 'text-red-400'    }
  return { text: 'M', cls: 'text-green-400' }
}

// Status dot for container/proxy/overlay
function statusDot(ep: models.Endpoint): { show: boolean; cls: string; pulse: boolean; title: string } {
  if (ep.type === 'container') {
    const s = serverStore.getContainerStatus(ep.id)
    if (!s) return { show: false, cls: '', pulse: false, title: '' }
    const running = s.status === 'running'
    return { show: true, cls: running ? 'bg-green-400' : 'bg-gray-500', pulse: running, title: s.status }
  }
  if (ep.type === 'proxy' && !isOverlay(ep)) {
    // Show pulsing green dot if a WS connection is open for this proxy endpoint
    const hasOpenWS = serverStore.requestLogs.some(l => l.endpoint_id === ep.id && l.is_websocket && l.ws_is_open)
    if (hasOpenWS) return { show: true, cls: 'bg-green-400', pulse: true, title: 'WS connection open' }
    const h = serverStore.getEndpointHealth(ep.id)
    if (!h) return { show: false, cls: '', pulse: false, title: '' }
    return { show: true, cls: h.healthy ? 'bg-green-400' : 'bg-red-400', pulse: false, title: h.healthy ? 'Healthy' : 'Unhealthy' }
  }
  if (isOverlay(ep)) {
    // show pulsing dot if a WSS connection is open for this endpoint
    const hasOpenWS = serverStore.requestLogs.some(l => l.endpoint_id === ep.id && l.is_websocket && l.ws_is_open)
    if (hasOpenWS) return { show: true, cls: 'bg-green-400', pulse: true, title: 'WSS connection open' }
  }
  return { show: false, cls: '', pulse: false, title: '' }
}

// ── Activity indicator (up/down arrow showing traffic recency) ────────────
const activityTick = ref(0)
let activityTimer: ReturnType<typeof setInterval> | null = null
onMounted(() => { activityTimer = setInterval(() => activityTick.value++, 1000) })
onUnmounted(() => { if (activityTimer) clearInterval(activityTimer) })

function activityDot(ep: models.Endpoint): { show: boolean; cls: string; title: string } {
  // Touch activityTick to create reactive dependency for time-based updates
  void activityTick.value

  const epId = ep.id ?? ''
  const logs = serverStore.requestLogs
  let latestTime = 0
  for (const l of logs) {
    if (l.endpoint_id !== epId) continue
    const t = new Date(l.timestamp).getTime()
    if (t > latestTime) latestTime = t
  }
  if (latestTime === 0) return { show: false, cls: '', title: '' }

  const elapsed = Date.now() - latestTime
  if (elapsed <= 5000)  return { show: true, cls: 'text-green-400',  title: 'Active traffic' }
  if (elapsed <= 60000) return { show: true, cls: 'text-yellow-400', title: 'Recent traffic' }
  return { show: true, cls: 'text-gray-500', title: 'Historical traffic' }
}

// ── Keyboard navigation ───────────────────────────────────────────────────
const focusedId   = ref<string | null>(null)
const listRef     = ref<HTMLElement | null>(null)
const rootRef     = ref<HTMLElement | null>(null)

function visibleIds(): string[] {
  if (isFiltering.value) return ['server', ...filteredList.value.map(ep => ep.id ?? '')]
  const ids = ['server']
  if (!collapsed.value.mock)      ids.push(...mockEps.value.map(ep => ep.id ?? ''))
  if (!collapsed.value.proxy)     ids.push(...proxyEps.value.map(ep => ep.id ?? ''))
  if (!collapsed.value.container) ids.push(...containerEps.value.map(ep => ep.id ?? ''))
  if (!collapsed.value.overlays)  ids.push(...overlayEps.value.map(ep => ep.id ?? ''))
  ids.push(...systemEps.value.map(ep => ep.id ?? ''))
  return ids
}

async function onKeyDown(e: KeyboardEvent) {
  const ids = visibleIds()
  const cur = focusedId.value ?? props.selectedId
  const idx = ids.indexOf(cur)

  if (e.key === 'ArrowDown') {
    e.preventDefault()
    focusedId.value = ids[Math.min(idx + 1, ids.length - 1)]
    await nextTick()
    document.getElementById(`nav-item-${focusedId.value}`)?.scrollIntoView({ block: 'nearest' })
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    focusedId.value = ids[Math.max(idx - 1, 0)]
    await nextTick()
    document.getElementById(`nav-item-${focusedId.value}`)?.scrollIntoView({ block: 'nearest' })
  } else if (e.key === 'Enter' && focusedId.value) {
    emit('select', focusedId.value)
  } else if (e.key === 'Escape') {
    filter.value = ''
    focusedId.value = null
    filterInputRef.value?.focus()
  }
}

// When filter changes, focus first result
watch(filter, async () => {
  focusedId.value = null
  await nextTick()
  if (isFiltering.value && filteredList.value.length > 0) {
    focusedId.value = filteredList.value[0].id ?? null
  }
})

// ── Drag-to-reorder (within mock/proxy/container groups) ──────────────────
const draggedId  = ref<string | null>(null)
const dragOverId = ref<string | null>(null)

function canDrag(ep: models.Endpoint) { return !ep.is_system }

function onDragStart(ep: models.Endpoint, e: DragEvent) {
  if (!canDrag(ep)) { e.preventDefault(); return }
  draggedId.value = ep.id ?? null
  if (e.dataTransfer) { e.dataTransfer.effectAllowed = 'move'; e.dataTransfer.setData('text/plain', ep.id ?? '') }
}

function onDragOver(ep: models.Endpoint, e: DragEvent) {
  if (ep.is_system || !draggedId.value || draggedId.value === ep.id) return
  e.preventDefault()
  dragOverId.value = ep.id ?? null
}

function onDrop(ep: models.Endpoint) {
  const fromId = draggedId.value
  const toId   = ep.id
  draggedId.value  = null
  dragOverId.value = null
  if (!fromId || !toId || fromId === toId) return

  const nonSystem = props.endpoints.filter(e => !e.is_system)
  const fromNs = nonSystem.findIndex(e => e.id === fromId)
  const toNs   = nonSystem.findIndex(e => e.id === toId)
  if (fromNs >= 0 && toNs >= 0) serverStore.reorderEndpointTabs(fromNs, toNs)
}

function onDragEnd() { draggedId.value = null; dragOverId.value = null }

// ── Item class helper ─────────────────────────────────────────────────────
function itemCls(id: string, isDragging = false): string {
  const selected = props.selectedId === id
  const focused  = focusedId.value === id
  return [
    'flex items-center gap-1.5 px-2 py-1.5 text-xs cursor-pointer transition-colors group relative',
    'border-l-2',
    selected
      ? 'bg-gray-700 border-blue-400 text-white'
      : focused
        ? 'bg-gray-700/60 border-gray-600 text-gray-200'
        : 'border-transparent text-gray-400 hover:bg-gray-700/50 hover:text-gray-200',
    dragOverId.value === id ? 'border-t-2 border-t-blue-400' : '',
    isDragging ? 'opacity-40' : '',
  ].join(' ')
}
</script>

<template>
  <div
    ref="rootRef"
    class="flex flex-col border-r border-gray-700 bg-gray-800 flex-shrink-0 relative overflow-hidden"
    :style="{ width: navWidth + 'px' }"
    :class="{ 'select-none': isResizing }"
    role="navigation"
    tabindex="-1"
    @keydown="onKeyDown"
  >
    <!-- Filter input -->
    <div class="p-2 border-b border-gray-700 flex-shrink-0">
      <div class="relative">
        <svg class="absolute left-2 top-1/2 -translate-y-1/2 w-3 h-3 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <input
          ref="filterInputRef"
          v-model="filter"
          type="text"
          placeholder="Filter endpoints…"
          class="w-full pl-6 pr-5 py-1 bg-gray-700 border border-gray-600 rounded text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-blue-500"
        />
        <button v-if="filter" @click="filter = ''" class="absolute right-1.5 top-1/2 -translate-y-1/2 text-gray-400 hover:text-white">
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Scrollable list -->
    <div ref="listRef" class="flex-1 overflow-y-auto py-1" role="list">

      <!-- SERVER (always pinned, never filtered out) -->
      <div
        :id="`nav-item-server`"
        :class="itemCls('server')"
        role="listitem"
        :aria-current="selectedId === 'server' ? 'true' : undefined"
        @click="emit('select', 'server')"
      >
        <svg class="w-3 h-3 text-gray-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <span class="flex-1 font-medium truncate">Server</span>
      </div>

      <!-- FILTERED flat list -->
      <template v-if="isFiltering">
        <div v-if="filteredList.length === 0" class="px-3 py-6 text-xs text-gray-500 text-center">
          No endpoints match
        </div>
        <div
          v-for="ep in filteredList"
          :key="ep.id"
          :id="`nav-item-${ep.id}`"
          :class="itemCls(ep.id ?? '')"
          role="listitem"
          :aria-current="selectedId === ep.id ? 'true' : undefined"
          @click="emit('select', ep.id ?? '')"
        >
          <span :class="['flex-shrink-0 text-xs font-bold w-3 text-center', typeLabel(ep).cls]">{{ typeLabel(ep).text }}</span>
          <span class="flex-1 truncate">{{ displayName(ep) }}</span>
          <!-- Indicators (fixed-width container) -->
          <div class="flex items-center gap-1 flex-shrink-0 w-8 justify-end">
            <svg v-if="activityDot(ep).show" :class="['w-3 h-3', activityDot(ep).cls]" :title="activityDot(ep).title" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 18V6M5 9l3-3 3 3" /><path d="M16 6v12M13 15l3 3 3-3" /></svg>
            <span :class="['w-1.5 h-1.5 rounded-full', statusDot(ep).show ? statusDot(ep).cls : 'invisible', statusDot(ep).pulse ? 'animate-pulse' : '']" :title="statusDot(ep).title" />
          </div>
        </div>
      </template>

      <!-- GROUPED list (when not filtering) -->
      <template v-else>

        <!-- ── Helper macro: group section ─────────────────────────────── -->
        <!-- MOCK -->
        <template v-if="mockEps.length > 0">
          <button
            class="w-full flex items-center gap-1 px-2 py-1 text-[10px] font-semibold text-gray-500 uppercase tracking-wide hover:text-gray-300 transition-colors"
            :aria-expanded="!collapsed.mock"
            @click="toggleGroup('mock')"
          >
            <svg class="w-2.5 h-2.5 transition-transform" :class="{ '-rotate-90': collapsed.mock }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
            Mock
            <span class="ml-auto text-gray-600">{{ mockEps.length }}</span>
          </button>
          <div v-show="!collapsed.mock" class="overflow-hidden">
            <div
              v-for="ep in mockEps" :key="ep.id"
              :id="`nav-item-${ep.id}`"
              :class="itemCls(ep.id ?? '', draggedId === ep.id)"
              role="listitem"
              :aria-current="selectedId === ep.id ? 'true' : undefined"
              :draggable="true"
              @click="emit('select', ep.id ?? '')"
              @dragstart="onDragStart(ep, $event)"
              @dragover="onDragOver(ep, $event)"
              @drop="onDrop(ep)"
              @dragend="onDragEnd"
            >
              <span class="flex-shrink-0 text-xs font-bold w-3 text-center text-green-400">M</span>
              <span class="flex-1 truncate font-mono text-[11px]">{{ ep.path_prefix }}</span>
              <!-- Indicators (fixed-width container) -->
              <div class="flex items-center gap-1 flex-shrink-0 w-8 justify-end">
                <svg v-if="activityDot(ep).show" :class="['w-3 h-3', activityDot(ep).cls]" :title="activityDot(ep).title" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 18V6M5 9l3-3 3 3" /><path d="M16 6v12M13 15l3 3 3-3" /></svg>
                <span class="w-1.5 h-1.5 invisible" />
              </div>
              <!-- Hover actions -->
              <div class="hidden group-hover:flex items-center gap-0.5 flex-shrink-0 pointer-events-auto z-10">
                <button @click.stop="emit('deleteEp', ep)" class="p-0.5 rounded hover:bg-red-900/50 text-red-400" title="Delete">
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                </button>
              </div>
            </div>
          </div>
        </template>

        <!-- PROXY -->
        <template v-if="proxyEps.length > 0">
          <button class="w-full flex items-center gap-1 px-2 py-1 text-[10px] font-semibold text-gray-500 uppercase tracking-wide hover:text-gray-300 transition-colors" :aria-expanded="!collapsed.proxy" @click="toggleGroup('proxy')">
            <svg class="w-2.5 h-2.5 transition-transform" :class="{ '-rotate-90': collapsed.proxy }" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
            Proxy
            <span class="ml-auto text-gray-600">{{ proxyEps.length }}</span>
          </button>
          <div v-show="!collapsed.proxy">
            <div v-for="ep in proxyEps" :key="ep.id" :id="`nav-item-${ep.id}`" :class="itemCls(ep.id ?? '', draggedId === ep.id)" role="listitem" :aria-current="selectedId === ep.id ? 'true' : undefined" :draggable="true" @click="emit('select', ep.id ?? '')" @dragstart="onDragStart(ep, $event)" @dragover="onDragOver(ep, $event)" @drop="onDrop(ep)" @dragend="onDragEnd">
              <span class="flex-shrink-0 text-xs font-bold w-3 text-center text-blue-400">P</span>
              <span class="flex-1 truncate">{{ ep.name }}</span>
              <div class="flex items-center gap-1 flex-shrink-0 w-8 justify-end">
                <svg v-if="activityDot(ep).show" :class="['w-3 h-3', activityDot(ep).cls]" :title="activityDot(ep).title" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 18V6M5 9l3-3 3 3" /><path d="M16 6v12M13 15l3 3 3-3" /></svg>
                <span :class="['w-1.5 h-1.5 rounded-full', statusDot(ep).show ? statusDot(ep).cls : 'invisible', statusDot(ep).pulse ? 'animate-pulse' : '']" :title="statusDot(ep).title" />
              </div>
              <div class="hidden group-hover:flex items-center gap-0.5 flex-shrink-0">
                <button @click.stop="emit('deleteEp', ep)" class="p-0.5 rounded hover:bg-red-900/50 text-red-400"><svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg></button>
              </div>
            </div>
          </div>
        </template>

        <!-- CONTAINER -->
        <template v-if="containerEps.length > 0">
          <button class="w-full flex items-center gap-1 px-2 py-1 text-[10px] font-semibold text-gray-500 uppercase tracking-wide hover:text-gray-300 transition-colors" :aria-expanded="!collapsed.container" @click="toggleGroup('container')">
            <svg class="w-2.5 h-2.5 transition-transform" :class="{ '-rotate-90': collapsed.container }" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
            Container
            <span class="ml-auto text-gray-600">{{ containerEps.length }}</span>
          </button>
          <div v-show="!collapsed.container">
            <div v-for="ep in containerEps" :key="ep.id" :id="`nav-item-${ep.id}`" :class="itemCls(ep.id ?? '', draggedId === ep.id)" role="listitem" :aria-current="selectedId === ep.id ? 'true' : undefined" :draggable="true" @click="emit('select', ep.id ?? '')" @dragstart="onDragStart(ep, $event)" @dragover="onDragOver(ep, $event)" @drop="onDrop(ep)" @dragend="onDragEnd">
              <span class="flex-shrink-0 text-xs font-bold w-3 text-center text-purple-400">C</span>
              <span class="flex-1 truncate">{{ ep.name }}</span>
              <div class="flex items-center gap-1 flex-shrink-0 w-8 justify-end">
                <svg v-if="activityDot(ep).show" :class="['w-3 h-3', activityDot(ep).cls]" :title="activityDot(ep).title" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 18V6M5 9l3-3 3 3" /><path d="M16 6v12M13 15l3 3 3-3" /></svg>
                <span :class="['w-1.5 h-1.5 rounded-full', statusDot(ep).show ? statusDot(ep).cls : 'invisible', statusDot(ep).pulse ? 'animate-pulse' : '']" :title="statusDot(ep).title" />
              </div>
              <div class="hidden group-hover:flex items-center gap-0.5 flex-shrink-0">
                <button @click.stop="emit('deleteEp', ep)" class="p-0.5 rounded hover:bg-red-900/50 text-red-400"><svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg></button>
              </div>
            </div>
          </div>
        </template>

        <!-- OVERLAYS -->
        <template v-if="overlayEps.length > 0">
          <button class="w-full flex items-center gap-1 px-2 py-1 text-[10px] font-semibold text-gray-500 uppercase tracking-wide hover:text-gray-300 transition-colors" :aria-expanded="!collapsed.overlays" @click="toggleGroup('overlays')">
            <svg class="w-2.5 h-2.5 transition-transform" :class="{ '-rotate-90': collapsed.overlays }" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
            Overlays
            <span class="ml-auto text-gray-600">{{ overlayEps.length }}</span>
          </button>
          <div v-show="!collapsed.overlays">
            <div v-for="ep in overlayEps" :key="ep.id" :id="`nav-item-${ep.id}`" :class="itemCls(ep.id ?? '')" role="listitem" :aria-current="selectedId === ep.id ? 'true' : undefined" @click="emit('select', ep.id ?? '')">
              <span class="flex-shrink-0 text-xs font-bold w-3 text-center text-orange-400">O</span>
              <span class="flex-1 truncate font-mono text-[11px]">{{ displayName(ep) }}</span>
              <div class="flex items-center gap-1 flex-shrink-0 w-8 justify-end">
                <svg v-if="activityDot(ep).show" :class="['w-3 h-3', activityDot(ep).cls]" :title="activityDot(ep).title" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 18V6M5 9l3-3 3 3" /><path d="M16 6v12M13 15l3 3 3-3" /></svg>
                <span :class="['w-1.5 h-1.5 rounded-full', statusDot(ep).show ? statusDot(ep).cls : 'invisible', statusDot(ep).pulse ? 'animate-pulse' : '']" :title="statusDot(ep).title" />
              </div>
            </div>
          </div>
        </template>

        <!-- SYSTEM (non-collapsible) -->
        <template v-if="systemEps.length > 0">
          <div class="px-2 py-1 text-[10px] font-semibold text-gray-600 uppercase tracking-wide mt-1">System</div>
          <div v-for="ep in systemEps" :key="ep.id" :id="`nav-item-${ep.id}`" :class="itemCls(ep.id ?? '')" role="listitem" :aria-current="selectedId === ep.id ? 'true' : undefined" @click="emit('select', ep.id ?? '')">
            <span :class="['flex-shrink-0 text-xs font-bold w-3 text-center', typeLabel(ep).cls]">{{ typeLabel(ep).text }}</span>
            <span class="flex-1 truncate">{{ ep.name }}</span>
            <div class="flex items-center gap-1 flex-shrink-0 w-8 justify-end">
              <svg v-if="activityDot(ep).show" :class="['w-3 h-3', activityDot(ep).cls]" :title="activityDot(ep).title" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 18V6M5 9l3-3 3 3" /><path d="M16 6v12M13 15l3 3 3-3" /></svg>
              <span class="w-1.5 h-1.5 invisible" />
            </div>
          </div>
        </template>

      </template>
    </div>

    <!-- + Endpoint button -->
    <div class="p-2 border-t border-gray-700 flex-shrink-0">
      <button
        @click="emit('addEndpoint')"
        class="w-full flex items-center justify-center gap-1 px-2 py-1.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-blue-400 hover:text-blue-300 font-medium transition-colors"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        Add Endpoint
      </button>
    </div>

    <!-- Resize handle -->
    <div
      class="absolute right-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-blue-500/60 transition-colors"
      @mousedown.prevent="startResize"
      title="Drag to resize"
    />
  </div>
</template>
