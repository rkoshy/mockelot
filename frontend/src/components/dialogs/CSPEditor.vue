<script lang="ts" setup>
import { ref, computed, watch, nextTick } from 'vue'
import { GetDefaultCSPConfig } from '../../../wailsjs/go/main/App'
import { models } from '../../../wailsjs/go/models'

const props = defineProps<{
  modelValue: models.CSPConfig | null | undefined
}>()

const emit = defineEmits<{
  'update:modelValue': [csp: models.CSPConfig]
}>()

// ── Constants ────────────────────────────────────────────────────────────────

const ALL_DIRECTIVES: { name: string; description: string; noSources?: boolean }[] = [
  { name: 'default-src',               description: 'Fallback for all fetch directives' },
  { name: 'script-src',                description: 'Valid sources for JavaScript' },
  { name: 'script-src-attr',           description: 'Valid sources for inline event handlers (onclick=, onload=, etc.)' },
  { name: 'script-src-elem',           description: 'Valid sources for <script> elements' },
  { name: 'style-src',                 description: 'Valid sources for stylesheets' },
  { name: 'img-src',                   description: 'Valid sources for images' },
  { name: 'connect-src',               description: 'Valid targets for fetch, XHR, WebSocket' },
  { name: 'font-src',                  description: 'Valid sources for fonts' },
  { name: 'frame-src',                 description: 'Valid sources for frames / iframes' },
  { name: 'frame-ancestors',           description: 'Controls which pages can embed this page' },
  { name: 'form-action',               description: 'Valid endpoints for form submission' },
  { name: 'media-src',                 description: 'Valid sources for audio and video' },
  { name: 'object-src',                description: 'Valid sources for <object> and <embed>' },
  { name: 'worker-src',                description: 'Valid sources for Workers and ServiceWorkers' },
  { name: 'manifest-src',              description: 'Valid sources for Web App Manifests' },
  { name: 'child-src',                 description: 'Valid sources for Workers + nested browsing' },
  { name: 'base-uri',                  description: 'Restricts URLs in <base>' },
  { name: 'navigate-to',               description: 'Restricts URLs the document can navigate to' },
  { name: 'upgrade-insecure-requests', description: 'Upgrade HTTP to HTTPS automatically', noSources: true },
  { name: 'block-all-mixed-content',   description: 'Block all HTTP on HTTPS pages', noSources: true },
  { name: 'report-uri',                description: 'URL to receive violation reports (deprecated)' },
  { name: 'report-to',                 description: 'Reporting endpoint group name' },
]

const KEYWORD_SOURCES = [
  { value: "'self'",           label: "'self'",           title: 'Same origin' },
  { value: "'none'",           label: "'none'",           title: 'Block all sources' },
  { value: "'unsafe-inline'",  label: "'unsafe-inline'",  title: 'Allow inline code (risky)' },
  { value: "'unsafe-eval'",    label: "'unsafe-eval'",    title: 'Allow eval() (risky)' },
  { value: "'strict-dynamic'", label: "'strict-dynamic'", title: 'Trust nonce/hash-allowlisted scripts' },
  { value: 'blob:',            label: 'blob:',            title: 'Blob URLs' },
  { value: 'data:',            label: 'data:',            title: 'Data URLs' },
  { value: 'ws:',              label: 'ws:',              title: 'WebSocket (insecure)' },
  { value: 'wss:',             label: 'wss:',             title: 'WebSocket (secure)' },
  { value: 'https:',           label: 'https:',           title: 'Any HTTPS source' },
  { value: '*',                label: '*',                title: 'Any source (dangerous)' },
]

// ── Local State ───────────────────────────────────────────────────────────────

interface LocalDirective {
  id: string
  name: string
  sources: string[]
  expanded: boolean
  noSources: boolean
  hostInput: string
}

const enabled    = ref<boolean>(props.modelValue?.enabled ?? false)
const directives = ref<LocalDirective[]>([])
const loading    = ref(false)

// Paste-CSP panel state
const showPastePanel = ref(false)
const pasteText      = ref('')
const pasteError     = ref('')

// Add-directive panel (inline, no dropdown)
const showAddPanel = ref(false)

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

// ── Prop → local sync ────────────────────────────────────────────────────────

let syncFromProp = false
watch(() => props.modelValue, (v) => {
  if (syncFromProp) return
  syncFromProp = true
  enabled.value    = v?.enabled ?? false
  directives.value = buildLocals(v)
  nextTick(() => { syncFromProp = false })
}, { deep: true })

// ── Computed ──────────────────────────────────────────────────────────────────

const availableDirectives = computed(() =>
  ALL_DIRECTIVES.filter(d => !directives.value.some(ld => ld.name === d.name))
)

const effectiveHeader = computed((): string => {
  if (!enabled.value || directives.value.length === 0) return ''
  return directives.value
    .filter(d => d.name)
    .map(d => d.noSources || d.sources.length === 0
      ? d.name
      : `${d.name} ${d.sources.join(' ')}`)
    .join(';\n')
})

const currentConfig = computed((): models.CSPConfig => new models.CSPConfig({
  enabled: enabled.value,
  directives: directives.value.map(d => new models.CSPDirective({
    name: d.name,
    sources: [...d.sources],
  })),
}))

// ── Emit ──────────────────────────────────────────────────────────────────────

function emitUpdate() {
  if (syncFromProp) return
  emit('update:modelValue', currentConfig.value)
}

watch(enabled,    emitUpdate)
watch(directives, emitUpdate, { deep: true })

// ── Actions ───────────────────────────────────────────────────────────────────

async function loadDefaults() {
  loading.value = true
  try {
    const defaults = await GetDefaultCSPConfig()
    if (defaults) {
      enabled.value    = defaults.enabled ?? true
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
  showAddPanel.value = false
}

function removeDirective(index: number) {
  directives.value.splice(index, 1)
}

function toggleSource(d: LocalDirective, kw: string) {
  const idx = d.sources.indexOf(kw)
  if (idx >= 0) {
    d.sources.splice(idx, 1)
  } else {
    if (kw !== "'none'") {
      const ni = d.sources.indexOf("'none'")
      if (ni >= 0) d.sources.splice(ni, 1)
    } else {
      d.sources.splice(0, d.sources.length)
    }
    d.sources.push(kw)
  }
}

function hasSource(d: LocalDirective, kw: string) {
  return d.sources.includes(kw)
}

function addHost(d: LocalDirective) {
  const host = d.hostInput.trim()
  if (!host || d.sources.includes(host)) { d.hostInput = ''; return }
  d.sources.push(host)
  d.hostInput = ''
}

function removeSource(d: LocalDirective, src: string) {
  const idx = d.sources.indexOf(src)
  if (idx >= 0) d.sources.splice(idx, 1)
}

function isKeyword(src: string) {
  return KEYWORD_SOURCES.some(k => k.value === src)
}

function customSources(d: LocalDirective) {
  return d.sources.filter(s => !isKeyword(s))
}

// ── Parse pasted CSP ─────────────────────────────────────────────────────────

// A valid CSP directive name: hyphenated fetch/navigation/document directives,
// or the known single-word ones. This rejects nginx keywords like "always".
const CSP_DIRECTIVE_RE = /^[a-z][-a-z]+(-(src|uri|action|ancestors|to))?$|^(upgrade-insecure-requests|block-all-mixed-content|report-uri|report-to|base-uri|navigate-to)$/

function isValidDirectiveName(name: string): boolean {
  // Must contain at least one hyphen OR be a known single-word directive,
  // and must not be a bare nginx/shell keyword like "always".
  return CSP_DIRECTIVE_RE.test(name) || ALL_DIRECTIVES.some(d => d.name === name)
}

function parseCSPString(raw: string): LocalDirective[] | null {
  let text = raw.trim()

  // Strip optional nginx add_header prefix: add_header Content-Security-Policy "..." always;
  text = text.replace(/^add_header\s+content-security-policy\s+["']?/i, '').replace(/["']?\s*always\s*;?\s*$/i, '').trim()

  // Strip optional bare header prefix: Content-Security-Policy:
  text = text.replace(/^content-security-policy\s*:\s*/i, '').trim()

  // Strip surrounding quotes the user may have copy-pasted from nginx config
  if ((text.startsWith('"') && text.endsWith('"')) ||
      (text.startsWith("'") && text.endsWith("'"))) {
    text = text.slice(1, -1).trim()
  }

  // Strip trailing nginx "always" that may remain after semicolon
  text = text.replace(/;\s*always\s*;?\s*$/i, '').trim()

  if (!text) return null

  const result: LocalDirective[] = []
  const parts = text.split(';').map(p => p.trim()).filter(Boolean)
  for (const part of parts) {
    const tokens = part.split(/\s+/)
    const name = tokens[0].toLowerCase()
    if (!name) continue
    // Skip non-CSP tokens (e.g. nginx "always" leaking in after a semicolon)
    if (!isValidDirectiveName(name)) continue
    const def = directiveDef(name)
    const sources = tokens.slice(1)
    result.push({
      id: `d-${result.length}-${Date.now()}`,
      name,
      sources,
      expanded: false,
      noSources: def?.noSources ?? false,
      hostInput: '',
    })
  }
  // Auto-derive script-src-attr and script-src-elem from script-src if not present.
  // Older CSPs (nginx configs predating ~2022) don't include them, but modern browsers
  // enforce them as separate buckets. We add them automatically on paste.
  const scriptSrc = result.find(d => d.name === 'script-src')
  if (scriptSrc) {
    for (const derived of ['script-src-attr', 'script-src-elem']) {
      if (!result.some(d => d.name === derived)) {
        result.push({
          id: `d-${result.length}-${Date.now()}`,
          name: derived,
          // script-src-attr: inline event handlers only need 'unsafe-inline' (or 'unsafe-hashes')
          // script-src-elem: inline <script> blocks — copy script-src sources + 'unsafe-inline'
          sources: derived === 'script-src-attr'
            ? ["'unsafe-inline'"]
            : [...scriptSrc.sources, "'unsafe-inline'"],
          expanded: false,
          noSources: false,
          hostInput: '',
        })
      }
    }
  }

  return result.length > 0 ? result : null
}

function applyPaste() {
  pasteError.value = ''
  const parsed = parseCSPString(pasteText.value)
  if (!parsed) {
    pasteError.value = 'Could not parse a valid CSP. Check the format and try again.'
    return
  }
  directives.value  = parsed
  enabled.value     = true
  pasteText.value   = ''
  showPastePanel.value = false
}

// ── Copy header ───────────────────────────────────────────────────────────────

const copyFeedback = ref(false)
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
  <div class="space-y-4 p-4">

    <!-- ── Top bar: enable toggle + action buttons ────────────────────────── -->
    <div class="flex items-center justify-between gap-2 flex-wrap">
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

      <div class="flex gap-2">
        <button
          @click="showPastePanel = !showPastePanel; showAddPanel = false"
          class="px-3 py-1.5 bg-gray-600 hover:bg-gray-500 text-white text-xs rounded
                 transition-colors flex items-center gap-1.5"
          title="Paste an existing CSP header to import it"
        >
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
          </svg>
          Paste CSP
        </button>
        <button
          @click="loadDefaults"
          :disabled="loading"
          class="px-3 py-1.5 bg-gray-600 hover:bg-gray-500 text-white text-xs rounded
                 transition-colors flex items-center gap-1.5 disabled:opacity-50"
          title="Load a sensible default CSP"
        >
          <svg v-if="loading" class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
          </svg>
          <svg v-else class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          Load Defaults
        </button>
      </div>
    </div>

    <!-- ── Paste CSP panel (inline, no floating) ──────────────────────────── -->
    <div v-if="showPastePanel" class="border border-indigo-700 rounded bg-indigo-900/20 p-3 space-y-2">
      <p class="text-xs text-indigo-300 font-medium">Paste a CSP header value</p>
      <p class="text-xs text-gray-400">
        Paste the full header value or just the directive string. Any existing directives will be replaced.
      </p>
      <textarea
        v-model="pasteText"
        rows="3"
        placeholder="e.g. default-src 'self'; script-src 'self' 'unsafe-eval'; img-src *"
        class="w-full px-3 py-2 bg-gray-900 border border-gray-600 rounded text-white text-xs font-mono
               focus:outline-none focus:border-indigo-500 resize-none"
      />
      <p v-if="pasteError" class="text-xs text-red-400">{{ pasteError }}</p>
      <div class="flex gap-2">
        <button
          @click="applyPaste"
          class="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-700 text-white text-xs rounded transition-colors"
        >Apply</button>
        <button
          @click="showPastePanel = false; pasteText = ''; pasteError = ''"
          class="px-3 py-1.5 bg-gray-600 hover:bg-gray-500 text-white text-xs rounded transition-colors"
        >Cancel</button>
      </div>
    </div>

    <!-- ── Info banner ────────────────────────────────────────────────────── -->
    <div class="p-3 bg-blue-900/20 border border-blue-800 rounded text-xs text-blue-200">
      CSP tells browsers which sources are trusted for scripts, styles, images and other resources.
      When enabled, Mockelot injects
      <code class="text-blue-300 font-mono">Content-Security-Policy</code>
      on every response, overwriting any upstream value.
    </div>

    <div v-if="enabled" class="space-y-4">

      <!-- ── Directive list ─────────────────────────────────────────────── -->
      <div class="space-y-2">
        <h4 class="text-sm font-medium text-white">Directives</h4>

        <!-- Empty state -->
        <div
          v-if="directives.length === 0 && !showAddPanel"
          class="text-center py-6 text-gray-400 text-sm border border-dashed border-gray-600 rounded"
        >
          No directives. Use <strong>Load Defaults</strong>, <strong>Paste CSP</strong>, or <strong>Add Directive</strong>.
        </div>

        <!-- Directive cards -->
        <div
          v-for="(d, idx) in directives"
          :key="d.id"
          class="border border-gray-600 rounded"
        >
          <!-- Header row -->
          <div
            class="flex items-center gap-3 px-3 py-2 bg-gray-700/60 cursor-pointer select-none"
            :class="d.expanded ? 'rounded-t' : 'rounded'"
            @click="d.expanded = !d.expanded"
          >
            <svg
              class="w-4 h-4 text-gray-400 flex-shrink-0 transition-transform"
              :class="d.expanded ? 'rotate-90' : ''"
              fill="none" stroke="currentColor" viewBox="0 0 24 24"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>

            <span class="font-mono text-sm text-blue-300 flex-shrink-0 w-44 truncate">{{ d.name }}</span>

            <!-- Source preview when collapsed -->
            <div v-if="!d.expanded" class="flex flex-wrap gap-1 flex-1 min-w-0 overflow-hidden">
              <span
                v-for="src in d.sources.slice(0, 5)" :key="src"
                class="px-1.5 py-0.5 bg-gray-600 text-gray-200 text-xs rounded font-mono"
              >{{ src }}</span>
              <span v-if="d.sources.length > 5" class="text-xs text-gray-500">+{{ d.sources.length - 5 }}</span>
              <span v-if="d.noSources" class="text-xs text-gray-500 italic">(flag)</span>
              <span v-if="!d.noSources && d.sources.length === 0" class="text-xs text-gray-500 italic">inherits default-src</span>
            </div>

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
            <p v-if="d.noSources" class="text-xs text-gray-400 italic">
              This directive acts as a flag — no source list needed.
            </p>

            <!-- Keyword pills -->
            <div v-if="!d.noSources">
              <p class="text-xs text-gray-400 mb-2">Keywords</p>
              <div class="flex flex-wrap gap-1.5">
                <button
                  v-for="kw in KEYWORD_SOURCES" :key="kw.value"
                  @click="toggleSource(d, kw.value)"
                  :title="kw.title"
                  :class="[
                    'px-2 py-1 rounded text-xs font-mono transition-all border',
                    hasSource(d, kw.value)
                      ? 'bg-blue-600 border-blue-500 text-white'
                      : 'bg-gray-700 border-gray-600 text-gray-300 hover:border-gray-400'
                  ]"
                >{{ kw.label }}</button>
              </div>
            </div>

            <!-- Custom host sources -->
            <div v-if="!d.noSources">
              <p class="text-xs text-gray-400 mb-2">Hosts / Schemes</p>
              <div class="flex flex-wrap gap-1.5 mb-2">
                <span
                  v-for="src in customSources(d)" :key="src"
                  class="flex items-center gap-1 px-2 py-0.5 bg-indigo-700/60 border border-indigo-600
                         rounded text-xs font-mono text-indigo-200"
                >
                  {{ src }}
                  <button @click="removeSource(d, src)" class="text-indigo-400 hover:text-red-400 leading-none">×</button>
                </span>
              </div>
              <div class="flex gap-2">
                <input
                  v-model="d.hostInput"
                  type="text"
                  placeholder="e.g. *.example.com"
                  class="flex-1 px-3 py-1.5 bg-gray-700 border border-gray-600 rounded text-white
                         text-xs font-mono focus:outline-none focus:border-blue-500"
                  @keydown.enter.prevent="addHost(d)"
                />
                <button
                  @click="addHost(d)"
                  class="px-3 py-1.5 bg-gray-600 hover:bg-gray-500 text-white text-xs rounded transition-colors"
                >Add</button>
              </div>
            </div>
          </div>
        </div>

        <!-- ── Add Directive — inline panel, no floating dropdown ────────── -->
        <div v-if="!showAddPanel && availableDirectives.length > 0">
          <button
            @click="showAddPanel = true; showPastePanel = false"
            class="w-full py-2 border border-dashed border-gray-600 rounded text-xs text-gray-400
                   hover:border-gray-400 hover:text-gray-300 transition-colors flex items-center justify-center gap-1.5"
          >
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            Add Directive
          </button>
        </div>

        <div v-if="showAddPanel" class="border border-gray-600 rounded bg-gray-800/60 p-3 space-y-2">
          <div class="flex items-center justify-between mb-1">
            <p class="text-xs font-medium text-gray-300">Choose a directive to add</p>
            <button
              @click="showAddPanel = false"
              class="text-gray-500 hover:text-gray-300 text-xs transition-colors"
            >✕ Close</button>
          </div>
          <div class="grid grid-cols-1 gap-1">
            <button
              v-for="d in availableDirectives" :key="d.name"
              @click="addDirective(d.name)"
              class="text-left px-3 py-2 rounded hover:bg-gray-700 transition-colors border border-transparent
                     hover:border-gray-600 flex items-baseline gap-3"
            >
              <span class="font-mono text-xs text-blue-300 flex-shrink-0 w-44">{{ d.name }}</span>
              <span class="text-xs text-gray-400 truncate">{{ d.description }}</span>
            </button>
          </div>
        </div>
      </div>

      <!-- ── Effective Header preview ───────────────────────────────────── -->
      <div class="border border-gray-600 rounded overflow-hidden">
        <div class="flex items-center justify-between px-3 py-2 bg-gray-700/60 border-b border-gray-600">
          <span class="text-xs font-medium text-gray-300">Effective Header</span>
          <button
            @click="copyHeader"
            :class="[
              'px-2 py-1 text-xs rounded transition-colors flex items-center gap-1',
              copyFeedback
                ? 'bg-green-700 text-green-100'
                : 'bg-gray-600 hover:bg-gray-500 text-gray-300'
            ]"
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
            {{ directives.length === 0 ? 'No directives — no CSP header will be sent.' : 'Add directives above.' }}
          </div>
        </div>
      </div>

    </div>

    <!-- Disabled placeholder -->
    <div v-else class="py-4 text-center text-gray-500 text-sm">
      Enable CSP above to configure directives.
    </div>

  </div>
</template>
