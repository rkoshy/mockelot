<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import { GetDefaultCSPConfig } from '../../../wailsjs/go/main/App'
import { models } from '../../../wailsjs/go/models'

const props = defineProps<{
  modelValue: models.CSPConfig | null | undefined
}>()

const emit = defineEmits<{
  'update:modelValue': [csp: models.CSPConfig | null]
}>()

// ── Constants ────────────────────────────────────────────────────────────────

/** All known CSP directive names with descriptions for the "Add Directive" menu */
const ALL_DIRECTIVES: { name: string; description: string; noSources?: boolean }[] = [
  { name: 'default-src',              description: 'Fallback for all fetch directives' },
  { name: 'script-src',               description: 'Valid sources for JavaScript' },
  { name: 'style-src',                description: 'Valid sources for stylesheets' },
  { name: 'img-src',                  description: 'Valid sources for images' },
  { name: 'connect-src',              description: 'Valid targets for fetch, XHR, WebSocket' },
  { name: 'font-src',                 description: 'Valid sources for fonts' },
  { name: 'frame-src',                description: 'Valid sources for frames / iframes' },
  { name: 'frame-ancestors',          description: 'Controls which pages can embed this page' },
  { name: 'form-action',              description: 'Valid endpoints for form submission' },
  { name: 'media-src',                description: 'Valid sources for audio and video' },
  { name: 'object-src',               description: 'Valid sources for <object> and <embed>' },
  { name: 'worker-src',               description: 'Valid sources for Workers and ServiceWorkers' },
  { name: 'manifest-src',             description: 'Valid sources for Web App Manifests' },
  { name: 'child-src',                description: 'Valid sources for Workers and nested browsing contexts' },
  { name: 'base-uri',                 description: 'Restricts URLs in <base>' },
  { name: 'navigate-to',             description: 'Restricts URLs the document can navigate to' },
  { name: 'upgrade-insecure-requests',description: 'Upgrade HTTP to HTTPS automatically', noSources: true },
  { name: 'block-all-mixed-content',  description: 'Block all HTTP resources on HTTPS pages', noSources: true },
  { name: 'report-uri',               description: 'URL to receive violation reports (deprecated, use report-to)' },
  { name: 'report-to',                description: 'Reporting endpoint group name' },
]

/** Common keyword sources shown as toggleable pills */
const KEYWORD_SOURCES = [
  { value: "'self'",          label: "'self'",          title: 'Same origin' },
  { value: "'none'",          label: "'none'",          title: 'Block all sources' },
  { value: "'unsafe-inline'", label: "'unsafe-inline'", title: 'Allow inline code (risky)' },
  { value: "'unsafe-eval'",   label: "'unsafe-eval'",   title: 'Allow eval() (risky)' },
  { value: "'strict-dynamic'",label: "'strict-dynamic'",title: 'Trust scripts allowed by nonces/hashes' },
  { value: 'blob:',           label: 'blob:',           title: 'Blob URLs' },
  { value: 'data:',           label: 'data:',           title: 'Data URLs' },
  { value: 'ws:',             label: 'ws:',             title: 'WebSocket (insecure)' },
  { value: 'wss:',            label: 'wss:',            title: 'WebSocket (secure)' },
  { value: 'https:',          label: 'https:',          title: 'Any HTTPS source' },
  { value: '*',               label: '*',               title: 'Any source (dangerous)' },
]

// ── Local State ──────────────────────────────────────────────────────────────

interface LocalDirective {
  id: string
  name: string
  sources: string[]
  expanded: boolean
  noSources: boolean
  hostInput: string // staging area for the host text input
}

const enabled = ref<boolean>(props.modelValue?.enabled ?? false)
const directives = ref<LocalDirective[]>([])
const showAddMenu = ref(false)
const loading = ref(false)

function directiveDef(name: string) {
  return ALL_DIRECTIVES.find(d => d.name === name)
}

function buildLocals(cfg: models.CSPConfig | null | undefined): LocalDirective[] {
  if (!cfg?.directives) return []
  return cfg.directives.map((d, i) => ({
    id: `d-${i}-${Date.now()}`,
    name: d.name,
    sources: [...(d.sources ?? [])],
    expanded: false,
    noSources: directiveDef(d.name)?.noSources ?? false,
    hostInput: '',
  }))
}

directives.value = buildLocals(props.modelValue)

// Sync from parent (e.g. when user clicks "load defaults")
let syncFromProp = false
watch(() => props.modelValue, (v) => {
  if (syncFromProp) return
  syncFromProp = true
  enabled.value = v?.enabled ?? false
  directives.value = buildLocals(v)
  syncFromProp = false
}, { deep: true })

// ── Computed ─────────────────────────────────────────────────────────────────

/** Directives not yet added — drives the Add menu */
const availableDirectives = computed(() =>
  ALL_DIRECTIVES.filter(d => !directives.value.some(ld => ld.name === d.name))
)

/** The effective CSP header value as a string for the preview box */
const effectiveHeader = computed((): string => {
  if (!enabled.value || directives.value.length === 0) return ''
  const parts = directives.value
    .filter(d => d.name)
    .map(d => {
      if (d.noSources || d.sources.length === 0) return d.name
      return `${d.name} ${d.sources.join(' ')}`
    })
  return parts.join(';\n')
})

/** The current CSPConfig to emit */
const currentConfig = computed((): models.CSPConfig => new models.CSPConfig({
  enabled: enabled.value,
  directives: directives.value.map(d => new models.CSPDirective({
    name: d.name,
    sources: [...d.sources],
  })),
}))

// ── Emit helpers ─────────────────────────────────────────────────────────────

function emitUpdate() {
  if (syncFromProp) return
  emit('update:modelValue', enabled.value || directives.value.length > 0
    ? currentConfig.value
    : null
  )
}

watch(enabled, emitUpdate)
watch(directives, emitUpdate, { deep: true })

// ── Actions ───────────────────────────────────────────────────────────────────

async function loadDefaults() {
  loading.value = true
  try {
    const defaults = await GetDefaultCSPConfig()
    if (defaults) {
      enabled.value = defaults.enabled ?? true
      directives.value = buildLocals(defaults)
    }
  } catch (e) {
    console.error('Failed to load default CSP config', e)
  } finally {
    loading.value = false
  }
}

function addDirective(name: string) {
  const def = directiveDef(name)
  directives.value.push({
    id: `d-${directives.value.length}-${Date.now()}`,
    name,
    sources: [],
    expanded: true,
    noSources: def?.noSources ?? false,
    hostInput: '',
  })
  showAddMenu.value = false
}

function removeDirective(index: number) {
  directives.value.splice(index, 1)
}

function toggleSource(d: LocalDirective, kw: string) {
  const idx = d.sources.indexOf(kw)
  if (idx >= 0) {
    d.sources.splice(idx, 1)
  } else {
    // Remove 'none' if adding something else
    if (kw !== "'none'") {
      const noneIdx = d.sources.indexOf("'none'")
      if (noneIdx >= 0) d.sources.splice(noneIdx, 1)
    } else {
      // Adding 'none' — clear everything else
      d.sources.splice(0, d.sources.length)
    }
    d.sources.push(kw)
  }
}

function hasSource(d: LocalDirective, kw: string): boolean {
  return d.sources.includes(kw)
}

function addHost(d: LocalDirective) {
  const host = d.hostInput.trim()
  if (!host || d.sources.includes(host)) {
    d.hostInput = ''
    return
  }
  d.sources.push(host)
  d.hostInput = ''
}

function removeSource(d: LocalDirective, src: string) {
  const idx = d.sources.indexOf(src)
  if (idx >= 0) d.sources.splice(idx, 1)
}

function isKeyword(src: string): boolean {
  return KEYWORD_SOURCES.some(k => k.value === src)
}

/** Custom (host/scheme) sources — not keywords */
function customSources(d: LocalDirective): string[] {
  return d.sources.filter(s => !isKeyword(s))
}

let copyFeedback = ref(false)
async function copyHeader() {
  try {
    await navigator.clipboard.writeText(
      `Content-Security-Policy: ${effectiveHeader.value.replace(/\n/g, ' ')}`
    )
    copyFeedback.value = true
    setTimeout(() => { copyFeedback.value = false }, 1500)
  } catch (_) {}
}
</script>

<template>
  <div class="space-y-5 p-4">

    <!-- ── Enable toggle + load defaults ──────────────────────────────────── -->
    <div class="flex items-center justify-between">
      <label class="flex items-center gap-3 cursor-pointer select-none">
        <div
          @click="enabled = !enabled"
          :class="[
            'relative inline-flex h-5 w-9 items-center rounded-full transition-colors',
            enabled ? 'bg-blue-600' : 'bg-gray-600'
          ]"
        >
          <span :class="[
            'inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform',
            enabled ? 'translate-x-4' : 'translate-x-1'
          ]" />
        </div>
        <span class="text-sm font-medium text-white">
          {{ enabled ? 'CSP Enabled' : 'CSP Disabled' }}
        </span>
      </label>

      <button
        @click="loadDefaults"
        :disabled="loading"
        class="px-3 py-1.5 bg-gray-600 hover:bg-gray-500 text-white text-xs rounded
               transition-colors flex items-center gap-1.5 disabled:opacity-50"
        title="Load sensible default directives"
      >
        <svg v-if="loading" class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
        </svg>
        <svg v-else class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        Load Defaults
      </button>
    </div>

    <!-- ── Info banner ─────────────────────────────────────────────────────── -->
    <div class="p-3 bg-blue-900/20 border border-blue-800 rounded text-xs text-blue-200">
      Content Security Policy (CSP) tells browsers which sources are trusted for scripts, styles,
      images and other resources. When enabled, Mockelot injects a
      <code class="text-blue-300 font-mono">Content-Security-Policy</code> header on every response,
      overwriting any value set by an upstream server.
    </div>

    <div v-if="enabled" class="space-y-4">

      <!-- ── Directives list ───────────────────────────────────────────────── -->
      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <h4 class="text-sm font-medium text-white">Directives</h4>

          <!-- Add Directive button + dropdown -->
          <div class="relative">
            <button
              @click="showAddMenu = !showAddMenu"
              class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-xs rounded
                     transition-colors flex items-center gap-1.5"
            >
              <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
              </svg>
              Add Directive
              <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
              </svg>
            </button>

            <!-- Dropdown menu -->
            <div
              v-if="showAddMenu"
              class="absolute right-0 top-full mt-1 z-50 w-80 bg-gray-800 border border-gray-600 rounded shadow-xl overflow-y-auto max-h-72"
            >
              <!-- click-outside shim -->
              <div
                class="fixed inset-0 z-[-1]"
                @click="showAddMenu = false"
              />
              <div v-if="availableDirectives.length === 0" class="px-4 py-3 text-xs text-gray-400">
                All directives already added.
              </div>
              <button
                v-for="d in availableDirectives"
                :key="d.name"
                @click="addDirective(d.name)"
                class="w-full text-left px-4 py-2 hover:bg-gray-700 transition-colors"
              >
                <div class="text-sm font-mono text-blue-300">{{ d.name }}</div>
                <div class="text-xs text-gray-400">{{ d.description }}</div>
              </button>
            </div>
          </div>
        </div>

        <!-- Empty state -->
        <div v-if="directives.length === 0" class="text-center py-8 text-gray-400 text-sm border border-dashed border-gray-600 rounded">
          No directives configured. Click "Load Defaults" for a baseline, or "Add Directive" to start from scratch.
        </div>

        <!-- Directive cards -->
        <div
          v-for="(d, idx) in directives"
          :key="d.id"
          class="border border-gray-600 rounded"
        >
          <!-- Directive header row -->
          <div
            class="flex items-center gap-3 px-3 py-2 bg-gray-700/60 cursor-pointer select-none rounded"
            :class="d.expanded ? 'rounded-b-none' : ''"
            @click="d.expanded = !d.expanded"
          >
            <!-- Expand chevron -->
            <svg
              class="w-4 h-4 text-gray-400 flex-shrink-0 transition-transform"
              :class="d.expanded ? 'rotate-90' : ''"
              fill="none" stroke="currentColor" viewBox="0 0 24 24"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>

            <!-- Directive name -->
            <span class="font-mono text-sm text-blue-300 flex-shrink-0">{{ d.name }}</span>

            <!-- Source pills preview (collapsed) -->
            <div v-if="!d.expanded" class="flex flex-wrap gap-1 flex-1 min-w-0">
              <span
                v-for="src in d.sources.slice(0, 6)"
                :key="src"
                class="px-1.5 py-0.5 bg-gray-600 text-gray-200 text-xs rounded font-mono truncate max-w-[160px]"
              >{{ src }}</span>
              <span v-if="d.sources.length > 6" class="text-xs text-gray-500">
                +{{ d.sources.length - 6 }} more
              </span>
              <span v-if="d.noSources" class="text-xs text-gray-500 italic">(no sources)</span>
              <span v-if="!d.noSources && d.sources.length === 0" class="text-xs text-gray-500 italic">inherits default-src</span>
            </div>

            <!-- Remove button -->
            <button
              @click.stop="removeDirective(idx)"
              class="ml-auto p-1 text-gray-500 hover:text-red-400 transition-colors flex-shrink-0"
              title="Remove directive"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- Expanded body -->
          <div v-if="d.expanded" class="p-3 bg-gray-800/60 space-y-3 border-t border-gray-700 rounded-b">

            <!-- No-sources directives (flags) -->
            <p v-if="d.noSources" class="text-xs text-gray-400 italic">
              This directive has no source list — it acts as a flag.
            </p>

            <!-- Keyword pills -->
            <div v-if="!d.noSources">
              <p class="text-xs text-gray-400 mb-2">Keywords</p>
              <div class="flex flex-wrap gap-1.5">
                <button
                  v-for="kw in KEYWORD_SOURCES"
                  :key="kw.value"
                  @click="toggleSource(d, kw.value)"
                  :title="kw.title"
                  :class="[
                    'px-2 py-1 rounded text-xs font-mono transition-all border',
                    hasSource(d, kw.value)
                      ? 'bg-blue-600 border-blue-500 text-white'
                      : 'bg-gray-700 border-gray-600 text-gray-300 hover:border-gray-400'
                  ]"
                >
                  {{ kw.label }}
                </button>
              </div>
            </div>

            <!-- Custom host sources -->
            <div v-if="!d.noSources">
              <p class="text-xs text-gray-400 mb-2">Hosts / Schemes</p>

              <!-- Existing custom sources as removable pills -->
              <div class="flex flex-wrap gap-1.5 mb-2">
                <span
                  v-for="src in customSources(d)"
                  :key="src"
                  class="flex items-center gap-1 px-2 py-0.5 bg-indigo-700/60 border border-indigo-600 rounded text-xs font-mono text-indigo-200"
                >
                  {{ src }}
                  <button
                    @click="removeSource(d, src)"
                    class="text-indigo-400 hover:text-red-400 transition-colors leading-none"
                    title="Remove"
                  >×</button>
                </span>
              </div>

              <!-- Host input -->
              <div class="flex gap-2">
                <input
                  v-model="d.hostInput"
                  type="text"
                  placeholder="e.g. *.example.com or https://api.example.com"
                  class="flex-1 px-3 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs font-mono
                         focus:outline-none focus:border-blue-500"
                  @keydown.enter.prevent="addHost(d)"
                />
                <button
                  @click="addHost(d)"
                  class="px-3 py-1.5 bg-gray-600 hover:bg-gray-500 text-white text-xs rounded transition-colors"
                >Add</button>
              </div>
            </div>

            <!-- report-uri / report-to hint -->
            <div
              v-if="d.name === 'report-uri' || d.name === 'report-to'"
              class="p-2 bg-yellow-900/20 border border-yellow-800 rounded text-xs text-yellow-200"
            >
              Add the report endpoint URL or group name as a host source above.
            </div>
          </div>
        </div>
      </div>

      <!-- ── Effective Header preview ──────────────────────────────────────── -->
      <div class="border border-gray-600 rounded overflow-hidden">
        <div class="flex items-center justify-between px-3 py-2 bg-gray-700/60 border-b border-gray-600">
          <span class="text-xs font-medium text-gray-300">Effective Header</span>
          <button
            @click="copyHeader"
            class="px-2 py-1 text-xs rounded transition-colors flex items-center gap-1"
            :class="copyFeedback
              ? 'bg-green-700 text-green-100'
              : 'bg-gray-600 hover:bg-gray-500 text-gray-300'"
          >
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            {{ copyFeedback ? 'Copied!' : 'Copy' }}
          </button>
        </div>
        <div class="p-3 bg-gray-900/50 font-mono text-xs">
          <div v-if="effectiveHeader">
            <span class="text-gray-500">Content-Security-Policy:</span>
            <pre class="text-green-300 whitespace-pre-wrap mt-1 leading-relaxed">{{ effectiveHeader }}</pre>
          </div>
          <div v-else class="text-gray-500 italic">
            {{ directives.length === 0 ? 'No directives — no CSP header will be sent.' : 'CSP disabled.' }}
          </div>
        </div>
      </div>

    </div>

    <!-- ── Disabled placeholder ──────────────────────────────────────────── -->
    <div v-else class="py-6 text-center text-gray-500 text-sm">
      Enable CSP above to configure directives.
    </div>

  </div>
</template>
