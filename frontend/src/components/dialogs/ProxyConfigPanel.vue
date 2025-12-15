<script lang="ts" setup>
import { ref, computed } from 'vue'
import { TestProxyConnection, GetDefaultContainerHeaders } from '../../../wailsjs/go/main/App'
import HeaderManipulationList from './HeaderManipulationList.vue'
import StatusTranslationList from './StatusTranslationList.vue'
import { models } from '../../../wailsjs/go/models'

const props = defineProps<{
  config: models.ProxyConfig
  isContainerEndpoint?: boolean
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

// Sub-tab state
const activeSubTab = ref<'backend' | 'headers' | 'transformation' | 'health'>('backend')

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
  status_translation: statusTranslation.value
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
      Proxy Configuration
    </h3>

    <!-- Sub-Tabs -->
    <div class="flex border-b border-gray-700">
      <button
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
        <label class="block text-sm font-medium text-gray-300 mb-2">
          Body Transformation (JavaScript)
        </label>
        <textarea
          v-model="bodyTransform"
          @blur="emitUpdate"
          rows="8"
          class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white font-mono text-sm
                 focus:outline-none focus:border-blue-500"
          placeholder="// Optional: Transform response body&#10;// Available: body (string), contentType (string), JSON.parse, JSON.stringify&#10;&#10;const data = JSON.parse(body);&#10;data.modified = true;&#10;JSON.stringify(data)"
        />
        <p class="mt-1 text-xs text-gray-400">
          Optional JavaScript to transform the response body. Return the modified body as a string.
        </p>
      </div>
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
