<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { SetOverlaySimulationMode } from '../../../wailsjs/go/main/App'
import type { models } from '../../../wailsjs/go/models'

const props = defineProps<{ endpoint: models.Endpoint }>()

// ── Config state ──────────────────────────────────────────────────────────
const mode        = ref('normal')
const timeoutSecs = ref(30)
const statusCode  = ref(503)

// ── Dropdown visibility ───────────────────────────────────────────────────
const showModeDropdown   = ref(false)
const showStatusDropdown = ref(false)
const statusSearch       = ref('')
const panelRef = ref<HTMLElement | null>(null)

// ── Mode options ──────────────────────────────────────────────────────────
const MODE_OPTIONS = [
  { value: 'normal',    label: 'NORMAL',    desc: 'Pass through to real backend',        dot: 'text-green-400'  },
  { value: 'timeout',   label: 'TIMEOUT',   desc: 'Simulate connection timeout → 504',   dot: 'text-orange-400' },
  { value: 'dns_error', label: 'DNS ERROR', desc: 'Simulate DNS resolution failure → 502', dot: 'text-red-400'  },
  { value: 'other',     label: 'OTHER',     desc: 'Return a custom HTTP status code',    dot: 'text-purple-400' },
]

const currentModeOpt = computed(() =>
  MODE_OPTIONS.find(o => o.value === mode.value) ?? MODE_OPTIONS[0]
)

const modeButtonClass = computed(() =>
  mode.value === 'normal'
    ? 'bg-gray-700 border-gray-600 text-gray-200 hover:bg-gray-650'
    : 'bg-orange-900/50 border-orange-600 text-orange-300 hover:bg-orange-900/70'
)

// ── HTTP status codes ─────────────────────────────────────────────────────
const HTTP_CODES = [
  // 1xx
  { code: 100, label: 'Continue' },
  { code: 101, label: 'Switching Protocols' },
  { code: 102, label: 'Processing' },
  { code: 103, label: 'Early Hints' },
  // 2xx
  { code: 200, label: 'OK' },
  { code: 201, label: 'Created' },
  { code: 202, label: 'Accepted' },
  { code: 203, label: 'Non-Authoritative Information' },
  { code: 204, label: 'No Content' },
  { code: 205, label: 'Reset Content' },
  { code: 206, label: 'Partial Content' },
  { code: 207, label: 'Multi-Status' },
  { code: 208, label: 'Already Reported' },
  { code: 226, label: 'IM Used' },
  // 3xx
  { code: 300, label: 'Multiple Choices' },
  { code: 301, label: 'Moved Permanently' },
  { code: 302, label: 'Found' },
  { code: 303, label: 'See Other' },
  { code: 304, label: 'Not Modified' },
  { code: 307, label: 'Temporary Redirect' },
  { code: 308, label: 'Permanent Redirect' },
  // 4xx
  { code: 400, label: 'Bad Request' },
  { code: 401, label: 'Unauthorized' },
  { code: 402, label: 'Payment Required' },
  { code: 403, label: 'Forbidden' },
  { code: 404, label: 'Not Found' },
  { code: 405, label: 'Method Not Allowed' },
  { code: 406, label: 'Not Acceptable' },
  { code: 407, label: 'Proxy Authentication Required' },
  { code: 408, label: 'Request Timeout' },
  { code: 409, label: 'Conflict' },
  { code: 410, label: 'Gone' },
  { code: 411, label: 'Length Required' },
  { code: 412, label: 'Precondition Failed' },
  { code: 413, label: 'Content Too Large' },
  { code: 414, label: 'URI Too Long' },
  { code: 415, label: 'Unsupported Media Type' },
  { code: 416, label: 'Range Not Satisfiable' },
  { code: 417, label: 'Expectation Failed' },
  { code: 418, label: "I'm a Teapot" },
  { code: 421, label: 'Misdirected Request' },
  { code: 422, label: 'Unprocessable Content' },
  { code: 423, label: 'Locked' },
  { code: 424, label: 'Failed Dependency' },
  { code: 425, label: 'Too Early' },
  { code: 426, label: 'Upgrade Required' },
  { code: 428, label: 'Precondition Required' },
  { code: 429, label: 'Too Many Requests' },
  { code: 431, label: 'Request Header Fields Too Large' },
  { code: 451, label: 'Unavailable For Legal Reasons' },
  // 5xx
  { code: 500, label: 'Internal Server Error' },
  { code: 501, label: 'Not Implemented' },
  { code: 502, label: 'Bad Gateway' },
  { code: 503, label: 'Service Unavailable' },
  { code: 504, label: 'Gateway Timeout' },
  { code: 505, label: 'HTTP Version Not Supported' },
  { code: 506, label: 'Variant Also Negotiates' },
  { code: 507, label: 'Insufficient Storage' },
  { code: 508, label: 'Loop Detected' },
  { code: 510, label: 'Not Extended' },
  { code: 511, label: 'Network Authentication Required' },
]

function codeGroup(code: number): string {
  if (code < 200) return '1xx Informational'
  if (code < 300) return '2xx Success'
  if (code < 400) return '3xx Redirection'
  if (code < 500) return '4xx Client Error'
  return '5xx Server Error'
}

const filteredGroups = computed(() => {
  const q = statusSearch.value.trim().toLowerCase()
  const filtered = q
    ? HTTP_CODES.filter(c => c.code.toString().includes(q) || c.label.toLowerCase().includes(q))
    : HTTP_CODES

  const groups: { group: string; codes: typeof HTTP_CODES }[] = []
  const seen = new Map<string, typeof groups[0]>()
  for (const c of filtered) {
    const g = codeGroup(c.code)
    if (!seen.has(g)) {
      const entry = { group: g, codes: [] as typeof HTTP_CODES }
      seen.set(g, entry)
      groups.push(entry)
    }
    seen.get(g)!.codes.push(c)
  }
  return groups
})

const selectedCodeLabel = computed(() => {
  const c = HTTP_CODES.find(c => c.code === statusCode.value)
  return c ? `${c.code} ${c.label}` : `${statusCode.value}`
})

// ── Actions ───────────────────────────────────────────────────────────────
async function selectMode(m: string) {
  mode.value = m
  showModeDropdown.value = false
  await send()
}

async function selectStatus(code: number) {
  statusCode.value = code
  showStatusDropdown.value = false
  await send()
}

async function send() {
  try {
    await SetOverlaySimulationMode(props.endpoint.id, {
      mode: mode.value,
      timeout_secs: timeoutSecs.value,
      status_code: statusCode.value,
    } as any)
  } catch (e) {
    console.error('SetOverlaySimulationMode failed:', e)
  }
}

// Close dropdowns on click outside
function onDocClick(e: MouseEvent) {
  if (panelRef.value && !panelRef.value.contains(e.target as Node)) {
    showModeDropdown.value  = false
    showStatusDropdown.value = false
  }
}
onMounted(() => document.addEventListener('mousedown', onDocClick))
onUnmounted(() => document.removeEventListener('mousedown', onDocClick))
</script>

<template>
  <div ref="panelRef" class="p-4 bg-gray-800 rounded border border-gray-700">
    <div class="flex items-start justify-between gap-4">
      <div class="flex-1">
        <h3 class="text-sm font-semibold text-white">Simulation Mode</h3>
        <p class="text-xs text-gray-400 mt-0.5">Simulate network failures for this overlay domain</p>
      </div>

      <!-- ── Mode dropdown ─────────────────────────────────────────────── -->
      <div class="relative flex-shrink-0">
        <button
          @click.stop="showModeDropdown = !showModeDropdown; showStatusDropdown = false"
          :class="['flex items-center gap-2 px-3 py-1.5 rounded border text-sm font-medium min-w-[170px] transition-colors', modeButtonClass]"
        >
          <span :class="['text-xs', currentModeOpt.dot]">●</span>
          <span class="flex-1 text-left">{{ currentModeOpt.label }}</span>
          <svg class="w-3.5 h-3.5 opacity-60 flex-shrink-0" :class="{ 'rotate-180': showModeDropdown }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        <Transition name="dd">
          <div v-if="showModeDropdown" class="absolute right-0 top-full mt-1 z-50 w-64 bg-gray-800 border border-gray-600 rounded shadow-xl overflow-hidden">
            <div
              v-for="opt in MODE_OPTIONS"
              :key="opt.value"
              @click="selectMode(opt.value)"
              :class="[
                'flex items-start gap-3 px-4 py-2.5 cursor-pointer transition-colors',
                mode === opt.value ? 'bg-gray-700' : 'hover:bg-gray-750'
              ]"
            >
              <span :class="['text-xs mt-0.5 flex-shrink-0', opt.dot]">●</span>
              <div>
                <div :class="['text-sm font-medium', mode === opt.value ? 'text-white' : 'text-gray-200']">{{ opt.label }}</div>
                <div class="text-xs text-gray-400 mt-0.5">{{ opt.desc }}</div>
              </div>
            </div>
          </div>
        </Transition>
      </div>
    </div>

    <!-- ── Timeout input (shown only for TIMEOUT mode) ──────────────────── -->
    <div v-if="mode === 'timeout'" class="mt-3 flex items-center gap-3">
      <label class="text-xs text-gray-400 flex-shrink-0">Timeout duration:</label>
      <input
        v-model.number="timeoutSecs"
        type="number"
        min="1"
        max="300"
        @change="send"
        class="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-sm text-white focus:outline-none focus:border-orange-500"
      />
      <span class="text-xs text-gray-400">seconds (then 504)</span>
    </div>

    <!-- ── Status code picker (shown only for OTHER mode) ───────────────── -->
    <div v-if="mode === 'other'" class="mt-3">
      <div class="flex items-center gap-3">
        <label class="text-xs text-gray-400 flex-shrink-0">Status code:</label>

        <!-- Status code custom dropdown -->
        <div class="relative">
          <button
            @click.stop="showStatusDropdown = !showStatusDropdown; showModeDropdown = false"
            class="flex items-center gap-2 px-3 py-1.5 bg-gray-700 border border-gray-600 hover:border-purple-500 rounded text-sm text-gray-200 min-w-[240px] transition-colors"
          >
            <span class="flex-1 text-left font-mono">{{ selectedCodeLabel }}</span>
            <svg class="w-3.5 h-3.5 opacity-60 flex-shrink-0" :class="{ 'rotate-180': showStatusDropdown }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
          </button>

          <Transition name="dd">
            <div v-if="showStatusDropdown" class="absolute left-0 top-full mt-1 z-50 w-72 bg-gray-800 border border-gray-600 rounded shadow-xl flex flex-col" style="max-height: 320px">
              <!-- Search -->
              <div class="p-2 border-b border-gray-700 flex-shrink-0">
                <input
                  v-model="statusSearch"
                  type="text"
                  placeholder="Search codes…"
                  class="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-purple-500"
                  @click.stop
                />
              </div>

              <!-- Grouped list -->
              <div class="overflow-y-auto flex-1">
                <template v-for="group in filteredGroups" :key="group.group">
                  <div class="px-3 py-1 text-xs font-semibold text-gray-500 bg-gray-900/60 sticky top-0">
                    {{ group.group }}
                  </div>
                  <div
                    v-for="c in group.codes"
                    :key="c.code"
                    @click="selectStatus(c.code)"
                    :class="[
                      'flex items-center gap-3 px-3 py-1.5 cursor-pointer transition-colors',
                      statusCode === c.code ? 'bg-purple-900/40 text-purple-300' : 'hover:bg-gray-700 text-gray-200'
                    ]"
                  >
                    <span class="font-mono text-xs w-8 flex-shrink-0 font-bold">{{ c.code }}</span>
                    <span class="text-xs">{{ c.label }}</span>
                  </div>
                </template>
                <div v-if="filteredGroups.length === 0" class="px-3 py-4 text-xs text-gray-500 text-center">
                  No codes match "{{ statusSearch }}"
                </div>
              </div>
            </div>
          </Transition>
        </div>
      </div>
    </div>

    <!-- Active simulation warning -->
    <div
      v-if="mode !== 'normal'"
      class="mt-3 px-3 py-2 bg-orange-900/30 border border-orange-700/50 rounded text-xs text-orange-300 flex items-start gap-2"
    >
      <span class="flex-shrink-0">⚠</span>
      <span>
        All traffic to
        <span class="font-mono">{{ endpoint.name?.replace('Overlay: ', '') }}</span>
        will receive
        <span v-if="mode === 'timeout'">a simulated {{ timeoutSecs }}s timeout → 504</span>
        <span v-else-if="mode === 'dns_error'">a simulated DNS error → 502</span>
        <span v-else>HTTP {{ statusCode }} {{ HTTP_CODES.find(c => c.code === statusCode)?.label ?? '' }}</span>
      </span>
    </div>
  </div>
</template>

<style scoped>
.dd-enter-active, .dd-leave-active { transition: opacity 0.1s, transform 0.1s; }
.dd-enter-from, .dd-leave-to { opacity: 0; transform: translateY(-4px); }
</style>
