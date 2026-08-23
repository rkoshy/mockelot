<script lang="ts" setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { TestProxyConnection, GetDefaultContainerHeaders } from '../../../wailsjs/go/main/App'
import HeaderManipulationList from './HeaderManipulationList.vue'
import StatusTranslationList from './StatusTranslationList.vue'
import CSPEditor from './CSPEditor.vue'
import { models } from '../../../wailsjs/go/models'
import { Codemirror } from 'vue-codemirror'
import { javascript } from '@codemirror/lang-javascript'
import { oneDark } from '@codemirror/theme-one-dark'
import { indentSelection } from '@codemirror/commands'
import type { EditorView } from '@codemirror/view'

const props = defineProps<{
  config: models.ProxyConfig
  isContainerEndpoint?: boolean
  isFileServerEndpoint?: boolean
}>()

const emit = defineEmits<{
  'update:config': [config: models.ProxyConfig]
}>()

// Local state
const backendURL = ref(props.config.backend_url || '')
const timeoutSeconds = ref(props.config.timeout_seconds || 30)
const statusPassthrough = ref(props.config.status_passthrough !== undefined ? props.config.status_passthrough : true)
const bodyTransform = ref(props.config.body_transform || '')
const healthCheckEnabled = ref(props.config.health_check_enabled || false)
const healthCheckInterval = ref(props.config.health_check_interval || 30)
const healthCheckPath = ref(props.config.health_check_path || '/')
const inboundHeaders = ref<models.HeaderManipulation[]>(props.config.inbound_headers || [])
const outboundHeaders = ref<models.HeaderManipulation[]>(props.config.outbound_headers || [])
const statusTranslation = ref<models.StatusTranslation[]>(props.config.status_translation || [])
const csp = ref<models.CSPConfig | null | undefined>(props.config.csp ?? null)

// Sub-tab state — file server endpoints skip Backend and Health tabs
const activeSubTab = ref<'backend' | 'headers' | 'csp' | 'transformation' | 'health'>(
  props.isFileServerEndpoint ? 'headers' : 'backend'
)

// Connection test state
const testingConnection = ref(false)
const connectionTestResult = ref<{ success: boolean; message: string } | null>(null)

// Computed config object
const updatedConfig = computed((): models.ProxyConfig => new models.ProxyConfig({
  backend_url: backendURL.value,
  timeout_seconds: timeoutSeconds.value,
  status_passthrough: statusPassthrough.value,
  body_transform: bodyTransform.value,
  health_check_enabled: healthCheckEnabled.value,
  health_check_interval: healthCheckInterval.value,
  health_check_path: healthCheckPath.value,
  inbound_headers: inboundHeaders.value,
  outbound_headers: outboundHeaders.value,
  status_translation: statusTranslation.value,
  csp: csp.value ?? undefined,
}))

// Emit updates
function emitUpdate() {
  emit('update:config', updatedConfig.value)
}

// Test backend connection
async function testConnection() {
  if (!backendURL.value.trim()) {
    connectionTestResult.value = {
      success: false,
      message: 'Please enter a backend URL first'
    }
    return
  }

  testingConnection.value = true
  connectionTestResult.value = null

  try {
    await TestProxyConnection(backendURL.value)
    connectionTestResult.value = {
      success: true,
      message: 'Connection successful!'
    }
  } catch (error) {
    connectionTestResult.value = {
      success: false,
      message: String(error).replace('Error: ', '')
    }
  } finally {
    testingConnection.value = false
  }
}

// CodeMirror extensions and fullscreen state
const cmExtensions = [javascript(), oneDark]
const isBodyFullscreen = ref(false)
const cmView = ref<EditorView | null>(null)

function handleCmReady(payload: { view: EditorView }) {
  cmView.value = payload.view
}

function formatBodyTransform() {
  const view = cmView.value
  if (!view) return
  // Select all then indent
  const len = view.state.doc.length
  view.dispatch({ selection: { anchor: 0, head: len } })
  indentSelection(view)
  // Collapse selection to end
  view.dispatch({ selection: { anchor: view.state.selection.main.head } })
}

function toggleBodyFullscreen() {
  isBodyFullscreen.value = !isBodyFullscreen.value
}

watch(isBodyFullscreen, (fs) => {
  document.body.style.overflow = fs ? 'hidden' : ''
})

function onBodyEditorKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && isBodyFullscreen.value) {
    e.stopImmediatePropagation()
    isBodyFullscreen.value = false
  }
}

onMounted(() => document.addEventListener('keydown', onBodyEditorKeydown))
onUnmounted(() => {
  document.removeEventListener('keydown', onBodyEditorKeydown)
  document.body.style.overflow = ''
})

// Reset to default headers
async function resetToDefaults() {
  try {
    const defaults = await GetDefaultContainerHeaders()

    if (props.isContainerEndpoint) {
      // For containers, use all RFC-compliant container headers (includes Host manipulation)
      inboundHeaders.value = defaults
    } else {
      // For regular proxy endpoints, use all defaults except the container-specific Host header
      // Keep: hop-by-hop header drops (RFC 7230) + X-Forwarded-* headers
      inboundHeaders.value = defaults.filter(h => h.name !== 'Host')
    }
    emitUpdate()
  } catch (error) {
    console.error('Failed to load default headers:', error)
    // Optionally show user feedback
    alert('Failed to load default headers. Please try again or check the console for errors.')
  }
}
</script>

<template>
  <div class="space-y-4">
    <h3 class="text-lg font-semibold text-white border-b border-gray-700 pb-2">
      {{ isFileServerEndpoint ? 'Response Configuration' : 'Proxy Configuration' }}
    </h3>

    <!-- Sub-Tabs -->
    <div class="flex border-b border-gray-700">
      <button
        v-if="!isFileServerEndpoint"
        @click="activeSubTab = 'backend'"
        :class="[
          'px-3 py-2 text-sm font-medium transition-colors',
          activeSubTab === 'backend'
            ? 'text-blue-400 border-b-2 border-blue-400'
            : 'text-gray-400 hover:text-gray-300'
        ]"
      >
        Backend
      </button>
      <button
        @click="activeSubTab = 'headers'"
        :class="[
          'px-3 py-2 text-sm font-medium transition-colors',
          activeSubTab === 'headers'
            ? 'text-blue-400 border-b-2 border-blue-400'
            : 'text-gray-400 hover:text-gray-300'
        ]"
      >
        Headers
      </button>
      <button
        @click="activeSubTab = 'csp'"
        :class="[
          'px-3 py-2 text-sm font-medium transition-colors',
          activeSubTab === 'csp'
            ? 'text-blue-400 border-b-2 border-blue-400'
            : 'text-gray-400 hover:text-gray-300'
        ]"
      >
        CSP
      </button>
      <button
        @click="activeSubTab = 'transformation'"
        :class="[
          'px-3 py-2 text-sm font-medium transition-colors',
          activeSubTab === 'transformation'
            ? 'text-blue-400 border-b-2 border-blue-400'
            : 'text-gray-400 hover:text-gray-300'
        ]"
      >
        Transformation
      </button>
      <button
        v-if="!isFileServerEndpoint"
        @click="activeSubTab = 'health'"
        :class="[
          'px-3 py-2 text-sm font-medium transition-colors',
          activeSubTab === 'health'
            ? 'text-blue-400 border-b-2 border-blue-400'
            : 'text-gray-400 hover:text-gray-300'
        ]"
      >
        Health
      </button>
    </div>

    <!-- Backend Tab -->
    <div v-if="activeSubTab === 'backend'" class="space-y-6 p-4">
      <!-- Backend URL -->
      <div>
        <label class="block text-sm font-medium text-gray-300 mb-2">
          Backend URL {{ isContainerEndpoint ? '' : '*' }}
        </label>
        <div class="flex gap-2">
          <input
            v-model="backendURL"
            @blur="emitUpdate"
            type="text"
            :placeholder="isContainerEndpoint ? 'Auto-configured from container port mapping' : 'http://localhost:8080'"
            :disabled="isContainerEndpoint"
            :class="[
              'flex-1 px-3 py-2 border rounded text-white placeholder-gray-400',
              isContainerEndpoint
                ? 'bg-gray-800 border-gray-700 cursor-not-allowed'
                : 'bg-gray-700 border-gray-600 focus:outline-none focus:border-blue-500'
            ]"
          />
          <button
            v-if="!isContainerEndpoint"
            @click="testConnection"
            :disabled="testingConnection || !backendURL.trim()"
            class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded transition-colors
                   disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            <svg v-if="testingConnection" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
            </svg>
            <span>{{ testingConnection ? 'Testing...' : 'Test' }}</span>
          </button>
        </div>
        <p v-if="connectionTestResult && !isContainerEndpoint" :class="[
          'mt-2 text-sm',
          connectionTestResult.success ? 'text-green-400' : 'text-red-400'
        ]">
          {{ connectionTestResult.success ? '✓' : '✗' }} {{ connectionTestResult.message }}
        </p>
        <p class="mt-1 text-xs text-gray-400">
          <template v-if="isContainerEndpoint">
            Automatically set to http://127.0.0.1:&lt;dynamic-port&gt; where dynamic-port is assigned by Docker/Podman at container startup
          </template>
          <template v-else>
            The URL of the backend server to proxy requests to
          </template>
        </p>
      </div>

      <!-- Timeout -->
      <div>
        <label class="block text-sm font-medium text-gray-300 mb-2">
          Timeout (seconds)
        </label>
        <input
          v-model.number="timeoutSeconds"
          @blur="emitUpdate"
          type="number"
          min="1"
          max="300"
          class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white
                 focus:outline-none focus:border-blue-500"
        />
        <p class="mt-1 text-xs text-gray-400">
          Maximum time to wait for backend response (default: 30)
        </p>
      </div>

      <!-- Info Box -->
      <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
        <p class="text-sm font-medium text-blue-300 mb-2">About Proxy Endpoints</p>
        <div class="space-y-2 text-xs text-blue-200">
          <p>
            Proxy endpoints forward all matching requests to a backend server with optional header manipulation,
            status code translation, and body transformation.
          </p>
          <p class="text-gray-300">
            <strong>WebSocket support:</strong> WebSocket connections are automatically detected and proxied bidirectionally.
          </p>
        </div>
      </div>
    </div>

    <!-- Headers Tab -->
    <div v-if="activeSubTab === 'headers'" class="space-y-6 p-4">
      <!-- Inbound Headers -->
      <div>
        <HeaderManipulationList
          v-model="inboundHeaders"
          direction="inbound"
          @update:modelValue="emitUpdate"
          :show-reset-defaults="true"
          @reset-defaults="resetToDefaults"
        />
        <p class="mt-2 text-xs text-gray-400">
          Headers to modify on requests <strong>to</strong> the backend{{ isContainerEndpoint ? ' container' : '' }}
        </p>
      </div>

      <!-- Outbound Headers -->
      <div class="border-t border-gray-700 pt-6">
        <HeaderManipulationList
          v-model="outboundHeaders"
          direction="outbound"
          @update:modelValue="emitUpdate"
        />
        <p class="mt-2 text-xs text-gray-400">
          Headers to modify on responses <strong>from</strong> the backend
        </p>
      </div>
    </div>

    <!-- CSP Tab -->
    <div v-if="activeSubTab === 'csp'">
      <CSPEditor
        v-model="csp"
        @update:modelValue="emitUpdate"
      />
    </div>

    <!-- Transformation Tab -->
    <div v-if="activeSubTab === 'transformation'" class="space-y-6 p-4">
      <!-- Status Code Translation -->
      <div>
        <div class="mb-4">
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              v-model="statusPassthrough"
              @change="emitUpdate"
              type="checkbox"
              class="w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600
                     focus:ring-2 focus:ring-blue-500"
            />
            <span class="text-sm text-gray-300">
              Status Code Pass-through (no translation)
            </span>
          </label>
          <p class="ml-6 mt-1 text-xs text-gray-400">
            If enabled, backend status codes are returned as-is. If disabled, use translation rules below.
          </p>
        </div>

        <div v-if="!statusPassthrough">
          <StatusTranslationList
            v-model="statusTranslation"
            @update:modelValue="emitUpdate"
          />
        </div>
      </div>

      <!-- Body Transformation -->
      <div class="border-t border-gray-700 pt-6">
        <div class="flex items-center justify-between mb-2">
          <label class="text-sm font-medium text-gray-300">
            Body Transformation (JavaScript)
          </label>
          <div class="flex gap-1">
            <button
              @click="formatBodyTransform"
              class="px-2 py-1 bg-gray-600 hover:bg-gray-500 rounded text-xs text-gray-300 transition-colors"
              title="Re-indent code"
            >Format</button>
            <button
              @click="toggleBodyFullscreen"
              class="px-2 py-1 bg-gray-600 hover:bg-gray-500 rounded text-xs text-gray-300 transition-colors flex items-center gap-1"
              title="Fullscreen editor"
            >
              <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
              </svg>
              Fullscreen
            </button>
          </div>
        </div>
        <div class="border border-gray-600 rounded overflow-hidden">
          <Codemirror
            v-model="bodyTransform"
            :extensions="cmExtensions"
            @change="emitUpdate"
            @ready="handleCmReady"
            placeholder="// Optional: Transform response body&#10;// Available: body (string), contentType (string)&#10;&#10;const data = JSON.parse(body);&#10;data.modified = true;&#10;JSON.stringify(data)"
            :style="{ height: '200px' }"
          />
        </div>
        <p class="mt-1 text-xs text-gray-400">
          Optional JavaScript to transform the response body. Return the modified body as a string.
        </p>
      </div>

      <!-- Fullscreen editor overlay -->
      <Teleport to="body">
        <Transition name="fs-editor">
          <div v-if="isBodyFullscreen" class="fixed inset-0 z-50 bg-gray-900 flex flex-col">
            <div class="flex items-center justify-between px-4 py-2 bg-gray-800 border-b border-gray-700 flex-shrink-0">
              <span class="text-sm text-white font-medium">Body Transformation (JavaScript)</span>
              <div class="flex gap-2">
                <button
                  @click="formatBodyTransform"
                  class="px-3 py-1 bg-gray-600 hover:bg-gray-500 rounded text-xs text-gray-300 transition-colors"
                >Format</button>
                <button
                  @click="toggleBodyFullscreen"
                  class="px-3 py-1 bg-blue-600 hover:bg-blue-700 rounded text-xs text-white transition-colors"
                >Close Fullscreen</button>
              </div>
            </div>
            <div class="flex-1">
              <Codemirror
                v-model="bodyTransform"
                :extensions="cmExtensions"
                @change="emitUpdate"
                @ready="handleCmReady"
                :style="{ height: '100%' }"
              />
            </div>
          </div>
        </Transition>
      </Teleport>
    </div>

    <!-- Health Tab -->
    <div v-if="activeSubTab === 'health'" class="space-y-6 p-4">
      <label class="flex items-center gap-2 cursor-pointer">
        <input
          v-model="healthCheckEnabled"
          @change="emitUpdate"
          type="checkbox"
          class="w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600
                 focus:ring-2 focus:ring-blue-500"
        />
        <span class="text-sm font-medium text-gray-300">
          Enable Health Checks
        </span>
      </label>

      <div v-if="healthCheckEnabled" class="ml-6 space-y-4">
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">
            Health Check Interval (seconds)
          </label>
          <input
            v-model.number="healthCheckInterval"
            @blur="emitUpdate"
            type="number"
            min="5"
            max="300"
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white
                   focus:outline-none focus:border-blue-500"
          />
          <p class="mt-1 text-xs text-gray-400">
            How often to check backend health (default: 30)
          </p>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">
            Health Check Path
          </label>
          <input
            v-model="healthCheckPath"
            @blur="emitUpdate"
            type="text"
            placeholder="/"
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white
                   focus:outline-none focus:border-blue-500"
          />
          <p class="mt-1 text-xs text-gray-400">
            Path to check for backend health (default: /)
          </p>
        </div>
      </div>

      <div v-if="!healthCheckEnabled" class="ml-6 p-3 bg-gray-900/50 border border-gray-700 rounded">
        <p class="text-sm text-gray-400">
          Health checks are disabled. Enable them to monitor backend availability.
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fs-editor-enter-active, .fs-editor-leave-active { transition: opacity 0.15s ease; }
.fs-editor-enter-from, .fs-editor-leave-to { opacity: 0; }
</style>
