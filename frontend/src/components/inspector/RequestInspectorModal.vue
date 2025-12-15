<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import { models } from '../../../wailsjs/go/models'
import { useServerStore } from '../../stores/server'
import BodyEditorModal from '../shared/BodyEditorModal.vue'
import FormatterSelector from '../shared/FormatterSelector.vue'
import PrometheusViewer from '../shared/PrometheusViewer.vue'
import { formatContent, detectContentType, supportsFormatting } from '../../utils/formatter'
import { isPrometheusMetrics } from '../../utils/prometheus-formatter'

interface Props {
  show: boolean
  log: models.RequestLogSummary | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  close: []
}>()

const serverStore = useServerStore()

// Full log details (fetched on-demand)
const fullLog = ref<models.RequestLog | null>(null)
const isLoadingDetails = ref(false)

// Panel types for side-by-side view
type PanelType = 'request' | 'response'
const activePanelType = ref<PanelType>('request')

// Sub-panels for each side
type ClientPanel = 'headers' | 'body' | 'query'
type ResponsePanel = 'headers' | 'body'
type BackendRequestPanel = 'headers' | 'body' | 'query'
type BackendResponsePanel = 'headers' | 'body'

const activeClientPanel = ref<ClientPanel>('headers')
const activeClientResponsePanel = ref<ResponsePanel>('headers')
const activeBackendRequestPanel = ref<BackendRequestPanel>('headers')
const activeBackendResponsePanel = ref<BackendResponsePanel>('headers')

const showBodyModal = ref(false)
const isRaw = ref(false)
const formattedBody = ref('')
const formatterOverride = ref('') // Empty means auto-detect
const viewMode = ref<'text' | 'table'>('text') // For Prometheus table view

// Fetch full log details when modal opens
watch(() => props.show, async (newVal) => {
  if (newVal && props.log) {
    isLoadingDetails.value = true
    fullLog.value = await serverStore.getLogDetails(props.log.id)
    isLoadingDetails.value = false
  } else {
    fullLog.value = null
  }
}, { immediate: true })

// Helper to check if backend exists
const hasBackend = computed(() => {
  return !!(fullLog.value?.backend_request || fullLog.value?.backend_response)
})

// Get content type from client request headers
const requestContentType = computed(() => {
  if (!fullLog.value?.client_request) return ''
  const headers = fullLog.value.client_request.headers || {}
  for (const [key, values] of Object.entries(headers)) {
    const headerValues = values as string[]
    if (key.toLowerCase() === 'content-type' && headerValues.length > 0) {
      return headerValues[0]
    }
  }
  return ''
})

// Detected content type (from header or auto-detected)
const detectedContentType = computed(() => {
  if (!fullLog.value?.client_request?.body) return ''
  return requestContentType.value || detectContentType(fullLog.value.client_request.body)
})

// Effective content type (override or detected)
const effectiveContentType = computed(() => {
  return formatterOverride.value || detectedContentType.value
})

const canFormat = computed(() => supportsFormatting(effectiveContentType.value))

// Check if current content is Prometheus (supports table view)
const isPrometheus = computed(() => {
  if (!fullLog.value?.client_request?.body) return false
  const type = effectiveContentType.value.toLowerCase()
  return type.includes('version=0.0.4') ||
         type === 'application/openmetrics-text' ||
         isPrometheusMetrics(fullLog.value.client_request.body)
})

// Format body when switching to formatted view
async function formatBody() {
  if (!fullLog.value?.client_request?.body || !canFormat.value) return
  try {
    formattedBody.value = await formatContent(fullLog.value.client_request.body, effectiveContentType.value)
  } catch {
    formattedBody.value = fullLog.value.client_request.body
  }
}

// Display body
const displayBody = computed(() => {
  if (!fullLog.value?.client_request?.body) return ''
  if (isRaw.value || !canFormat.value) {
    return fullLog.value.client_request.body
  }
  return formattedBody.value || fullLog.value.client_request.body
})

// Watch for body changes and format
watch(() => fullLog.value?.client_request?.body, async () => {
  isRaw.value = false
  formatterOverride.value = '' // Reset override when log changes
  viewMode.value = 'text' // Reset view mode when log changes
  if (canFormat.value) {
    await formatBody()
  }
}, { immediate: true })

// Re-format when formatter override changes
watch(formatterOverride, async () => {
  if (!isRaw.value && canFormat.value) {
    await formatBody()
  }
})

function formatHeaders(headers: Record<string, string[]>): { key: string; value: string }[] {
  const result: { key: string; value: string }[] = []
  for (const [key, values] of Object.entries(headers)) {
    for (const value of values) {
      result.push({ key, value })
    }
  }
  return result
}

function formatQueryParams(params: Record<string, string[]>): { key: string; value: string }[] {
  const result: { key: string; value: string }[] = []
  for (const [key, values] of Object.entries(params)) {
    for (const value of values) {
      result.push({ key, value })
    }
  }
  return result
}

function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleString()
}

function formatMs(ms: number): string {
  return `${ms}ms`
}

// Reset panels when modal opens
watch(() => props.show, (newVal) => {
  if (newVal) {
    activePanelType.value = 'request'
    activeClientPanel.value = 'headers'
    activeClientResponsePanel.value = 'headers'
    activeBackendRequestPanel.value = 'headers'
    activeBackendResponsePanel.value = 'headers'
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show && fullLog"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-70"
        @click.self="emit('close')"
      >
        <!-- Modal Container (90% of window) -->
        <div class="bg-gray-800 rounded-lg shadow-xl w-[90%] h-[90%] mx-4 border border-gray-700 flex flex-col">
          <!-- Header -->
          <div class="px-6 py-4 border-b border-gray-700 flex items-center justify-between flex-shrink-0">
            <h2 class="text-lg font-semibold text-white">Request Inspector</h2>
            <button
              @click="emit('close')"
              class="p-1 hover:bg-gray-700 rounded text-gray-400 hover:text-white transition-colors"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- Request Summary -->
          <div class="px-6 py-3 bg-gray-900/50 border-b border-gray-700 flex-shrink-0">
            <div class="flex items-center gap-2 mb-2">
              <span class="px-2 py-0.5 bg-blue-600 rounded text-xs font-bold text-white">
                {{ fullLog.client_request?.method || 'N/A' }}
              </span>
              <span class="px-2 py-0.5 bg-gray-700 rounded text-xs font-mono text-gray-300">
                Client: {{ fullLog.client_response?.status_code || 'N/A' }}
              </span>
              <span v-if="fullLog.backend_response" class="px-2 py-0.5 bg-gray-700 rounded text-xs font-mono text-gray-300">
                Backend: {{ fullLog.backend_response.status_code }}
              </span>
            </div>
            <p class="text-sm text-gray-300 font-mono break-all mb-1">
              <span class="text-gray-500">Client:</span> {{ fullLog.client_request?.full_url || fullLog.client_request?.path || 'N/A' }}
            </p>
            <p v-if="fullLog.backend_request" class="text-sm text-gray-300 font-mono break-all">
              <span class="text-gray-500">Backend:</span> {{ fullLog.backend_request.full_url || 'N/A' }}
            </p>
            <div class="mt-2 flex items-center gap-4 text-xs text-gray-500">
              <span>{{ formatTimestamp(fullLog.timestamp) }}</span>
              <span>{{ fullLog.client_request?.source_ip || 'N/A' }}</span>
              <span>{{ fullLog.client_request?.protocol || 'N/A' }}</span>
              <span v-if="fullLog.client_response" class="text-blue-400">
                Client RTT: {{ formatMs(fullLog.client_response.rtt_ms || 0) }}
              </span>
              <span v-if="fullLog.backend_response" class="text-green-400">
                Backend RTT: {{ formatMs(fullLog.backend_response.rtt_ms || 0) }}
              </span>
            </div>
          </div>

          <!-- Side-by-side Panels -->
          <div class="flex-1 flex min-h-0 overflow-hidden">
            <!-- Left Panel: Client Request + Client Response -->
            <div :class="['flex-1 flex flex-col min-w-0', hasBackend ? 'border-r border-gray-700' : '']">
              <!-- Client Request Section (Top Half) -->
              <div class="flex-1 flex flex-col border-b border-gray-700 min-h-0">
                <div class="px-4 py-2 bg-gray-900/50 border-b border-gray-700 flex-shrink-0">
                  <h3 class="text-sm font-semibold text-blue-400">Client Request</h3>
                </div>

                <!-- Client Request Sub-tabs -->
              <div class="flex border-b border-gray-700 flex-shrink-0 bg-gray-900/30">
                <button
                  @click="activeClientPanel = 'headers'"
                  :class="[
                    'px-3 py-1.5 text-xs font-medium transition-colors',
                    activeClientPanel === 'headers'
                      ? 'text-blue-400 border-b-2 border-blue-400 bg-gray-900/50'
                      : 'text-gray-400 hover:text-gray-300'
                  ]"
                >
                  Headers
                  <span class="ml-1 px-1 py-0.5 bg-gray-700 rounded text-xs">
                    {{ Object.keys(fullLog.client_request?.headers || {}).length }}
                  </span>
                </button>
                <button
                  @click="activeClientPanel = 'body'"
                  :class="[
                    'px-3 py-1.5 text-xs font-medium transition-colors',
                    activeClientPanel === 'body'
                      ? 'text-blue-400 border-b-2 border-blue-400 bg-gray-900/50'
                      : 'text-gray-400 hover:text-gray-300'
                  ]"
                >
                  Body
                  <span v-if="fullLog.client_request?.body" class="ml-1 px-1 py-0.5 bg-gray-700 rounded text-xs">
                    {{ fullLog.client_request.body.length }}
                  </span>
                </button>
                <button
                  @click="activeClientPanel = 'query'"
                  :class="[
                    'px-3 py-1.5 text-xs font-medium transition-colors',
                    activeClientPanel === 'query'
                      ? 'text-blue-400 border-b-2 border-blue-400 bg-gray-900/50'
                      : 'text-gray-400 hover:text-gray-300'
                  ]"
                >
                  Query
                  <span class="ml-1 px-1 py-0.5 bg-gray-700 rounded text-xs">
                    {{ Object.keys(fullLog.client_request?.query_params || {}).length }}
                  </span>
                </button>
              </div>

              <!-- Client Request Content -->
              <div class="flex-1 overflow-y-auto px-4 py-3 min-h-0">
                <!-- Client Request Headers -->
                <div v-if="activeClientPanel === 'headers'" class="space-y-1">
                  <div
                    v-for="(header, index) in formatHeaders(fullLog.client_request?.headers || {})"
                    :key="index"
                    class="flex gap-2 py-1 border-b border-gray-800 last:border-0"
                  >
                    <span class="text-blue-400 text-xs font-medium flex-shrink-0">{{ header.key }}:</span>
                    <span class="text-gray-300 text-xs break-all">{{ header.value }}</span>
                  </div>
                  <div v-if="Object.keys(fullLog.client_request?.headers || {}).length === 0" class="text-gray-500 text-xs">
                    No headers
                  </div>
                </div>

                <!-- Client Request Body -->
                <div v-if="activeClientPanel === 'body'" class="flex flex-col h-full">
                  <div v-if="fullLog.client_request?.body" class="flex flex-col flex-1 min-h-0">
                    <div class="flex items-center justify-between mb-2 flex-shrink-0">
                      <div class="flex items-center gap-2 flex-wrap">
                        <span v-if="detectedContentType" class="px-2 py-0.5 bg-gray-700 rounded text-xs text-gray-400 font-mono">
                          {{ detectedContentType }}
                        </span>
                        <FormatterSelector v-model="formatterOverride" />
                        <div v-if="isPrometheus" class="flex items-center bg-gray-700 rounded overflow-hidden">
                          <button
                            @click="viewMode = 'text'"
                            :class="[
                              'px-2 py-0.5 text-xs transition-colors',
                              viewMode === 'text' ? 'bg-blue-600 text-white' : 'text-gray-300 hover:bg-gray-600'
                            ]"
                          >
                            Text
                          </button>
                          <button
                            @click="viewMode = 'table'"
                            :class="[
                              'px-2 py-0.5 text-xs transition-colors',
                              viewMode === 'table' ? 'bg-blue-600 text-white' : 'text-gray-300 hover:bg-gray-600'
                            ]"
                          >
                            Table
                          </button>
                        </div>
                        <button
                          v-if="canFormat && viewMode === 'text'"
                          @click="isRaw = !isRaw"
                          :class="[
                            'px-2 py-0.5 rounded text-xs transition-colors',
                            isRaw ? 'bg-gray-700 text-gray-400' : 'bg-blue-600 text-white'
                          ]"
                        >
                          {{ isRaw ? 'Raw' : 'Formatted' }}
                        </button>
                      </div>
                      <button
                        @click="showBodyModal = true"
                        class="px-2 py-0.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors flex items-center gap-1 flex-shrink-0"
                      >
                        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
                        </svg>
                        Expand
                      </button>
                    </div>
                    <div
                      v-if="isPrometheus && viewMode === 'table'"
                      class="bg-gray-900 rounded p-3 flex-1 overflow-auto"
                    >
                      <PrometheusViewer :content="fullLog.client_request.body" />
                    </div>
                    <div v-else class="bg-gray-900 rounded p-3 flex-1 overflow-auto">
                      <pre class="text-xs text-gray-300 font-mono whitespace-pre-wrap break-all">{{ displayBody }}</pre>
                    </div>
                  </div>
                  <div v-else class="text-gray-500 text-xs">
                    No body content
                  </div>
                </div>

                <!-- Client Request Query -->
                <div v-if="activeClientPanel === 'query'" class="space-y-1">
                  <div
                    v-for="(param, index) in formatQueryParams(fullLog.client_request?.query_params || {})"
                    :key="index"
                    class="flex gap-2 py-1 border-b border-gray-800 last:border-0"
                  >
                    <span class="text-purple-400 text-xs font-medium flex-shrink-0">{{ param.key }}:</span>
                    <span class="text-gray-300 text-xs break-all">{{ param.value }}</span>
                  </div>
                  <div v-if="Object.keys(fullLog.client_request?.query_params || {}).length === 0" class="text-gray-500 text-xs">
                    No query parameters
                  </div>
                </div>
              </div>
              </div>

              <!-- Client Response Section (Bottom Half) -->
              <div class="flex-1 flex flex-col min-h-0">
                <div class="px-4 py-2 bg-gray-900/50 border-b border-gray-700 flex-shrink-0">
                  <h3 class="text-sm font-semibold text-green-400">Client Response</h3>
                  <div class="mt-1 flex items-center gap-3 text-xs text-gray-400">
                    <span>Status: {{ fullLog.client_response?.status_code || 'N/A' }} {{ fullLog.client_response?.status_text || '' }}</span>
                    <span>Delay: {{ formatMs(fullLog.client_response?.delay_ms || 0) }}</span>
                    <span>RTT: {{ formatMs(fullLog.client_response?.rtt_ms || 0) }}</span>
                  </div>
                </div>

                <!-- Client Response Sub-tabs -->
                <div class="flex border-b border-gray-700 flex-shrink-0 bg-gray-900/30">
                  <button
                    @click="activeClientResponsePanel = 'headers'"
                    :class="[
                      'px-3 py-1.5 text-xs font-medium transition-colors',
                      activeClientResponsePanel === 'headers'
                        ? 'text-green-400 border-b-2 border-green-400 bg-gray-900/50'
                        : 'text-gray-400 hover:text-gray-300'
                    ]"
                  >
                    Headers
                    <span class="ml-1 px-1 py-0.5 bg-gray-700 rounded text-xs">
                      {{ Object.keys(fullLog.client_response?.headers || {}).length }}
                    </span>
                  </button>
                  <button
                    @click="activeClientResponsePanel = 'body'"
                    :class="[
                      'px-3 py-1.5 text-xs font-medium transition-colors',
                      activeClientResponsePanel === 'body'
                        ? 'text-green-400 border-b-2 border-green-400 bg-gray-900/50'
                        : 'text-gray-400 hover:text-gray-300'
                    ]"
                  >
                    Body
                    <span v-if="fullLog.client_response?.body" class="ml-1 px-1 py-0.5 bg-gray-700 rounded text-xs">
                      {{ fullLog.client_response.body.length }}
                    </span>
                  </button>
                </div>

                <!-- Client Response Content -->
                <div class="flex-1 overflow-y-auto px-4 py-3 min-h-0">
                  <div v-if="activeClientResponsePanel === 'headers'" class="space-y-1">
                    <div
                      v-for="(header, index) in formatHeaders(fullLog.client_response?.headers || {})"
                      :key="index"
                      class="flex gap-2 py-1 border-b border-gray-800 last:border-0"
                    >
                      <span class="text-green-400 text-xs font-medium flex-shrink-0">{{ header.key }}:</span>
                      <span class="text-gray-300 text-xs break-all">{{ header.value }}</span>
                    </div>
                    <div v-if="Object.keys(fullLog.client_response?.headers || {}).length === 0" class="text-gray-500 text-xs">
                      No headers
                    </div>
                  </div>

                  <div v-if="activeClientResponsePanel === 'body'" class="flex flex-col h-full">
                    <div v-if="fullLog.client_response?.body" class="bg-gray-900 rounded p-3 flex-1 overflow-auto">
                      <pre class="text-xs text-gray-300 font-mono whitespace-pre-wrap break-all">{{ fullLog.client_response.body }}</pre>
                    </div>
                    <div v-else class="text-gray-500 text-xs">
                      No body content
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Right Panel: Backend Request + Backend Response -->
            <div v-if="hasBackend" class="flex-1 flex flex-col min-w-0">
                <!-- Backend Request Section -->
                <div class="flex-1 flex flex-col border-b border-gray-700 min-h-0">
                  <div class="px-4 py-2 bg-gray-900/50 border-b border-gray-700 flex-shrink-0">
                    <h3 class="text-xs font-semibold text-yellow-400">Backend Request</h3>
                    <p class="text-xs text-gray-400 font-mono break-all mt-1">
                      {{ fullLog.backend_request?.full_url || fullLog.backend_request?.path || 'N/A' }}
                    </p>
                  </div>

                  <div class="flex border-b border-gray-700 flex-shrink-0 bg-gray-900/30">
                    <button
                      @click="activeBackendRequestPanel = 'headers'"
                      :class="[
                        'px-3 py-1.5 text-xs font-medium transition-colors',
                        activeBackendRequestPanel === 'headers'
                          ? 'text-yellow-400 border-b-2 border-yellow-400 bg-gray-900/50'
                          : 'text-gray-400 hover:text-gray-300'
                      ]"
                    >
                      Headers
                      <span class="ml-1 px-1 py-0.5 bg-gray-700 rounded text-xs">
                        {{ Object.keys(fullLog.backend_request?.headers || {}).length }}
                      </span>
                    </button>
                    <button
                      @click="activeBackendRequestPanel = 'body'"
                      :class="[
                        'px-3 py-1.5 text-xs font-medium transition-colors',
                        activeBackendRequestPanel === 'body'
                          ? 'text-yellow-400 border-b-2 border-yellow-400 bg-gray-900/50'
                          : 'text-gray-400 hover:text-gray-300'
                      ]"
                    >
                      Body
                      <span v-if="fullLog.backend_request?.body" class="ml-1 px-1 py-0.5 bg-gray-700 rounded text-xs">
                        {{ fullLog.backend_request.body.length }}
                      </span>
                    </button>
                    <button
                      @click="activeBackendRequestPanel = 'query'"
                      :class="[
                        'px-3 py-1.5 text-xs font-medium transition-colors',
                        activeBackendRequestPanel === 'query'
                          ? 'text-yellow-400 border-b-2 border-yellow-400 bg-gray-900/50'
                          : 'text-gray-400 hover:text-gray-300'
                      ]"
                    >
                      Query
                      <span class="ml-1 px-1 py-0.5 bg-gray-700 rounded text-xs">
                        {{ Object.keys(fullLog.backend_request?.query_params || {}).length }}
                      </span>
                    </button>
                  </div>

                  <div class="flex-1 overflow-y-auto px-4 py-3 min-h-0">
                    <div v-if="activeBackendRequestPanel === 'headers'" class="space-y-1">
                      <div
                        v-for="(header, index) in formatHeaders(fullLog.backend_request?.headers || {})"
                        :key="index"
                        class="flex gap-2 py-1 border-b border-gray-800 last:border-0"
                      >
                        <span class="text-yellow-400 text-xs font-medium flex-shrink-0">{{ header.key }}:</span>
                        <span class="text-gray-300 text-xs break-all">{{ header.value }}</span>
                      </div>
                    </div>

                    <div v-if="activeBackendRequestPanel === 'body'" class="flex flex-col h-full">
                      <div v-if="fullLog.backend_request?.body" class="bg-gray-900 rounded p-3 flex-1 overflow-auto">
                        <pre class="text-xs text-gray-300 font-mono whitespace-pre-wrap break-all">{{ fullLog.backend_request.body }}</pre>
                      </div>
                      <div v-else class="text-gray-500 text-xs">
                        No body content
                      </div>
                    </div>

                    <div v-if="activeBackendRequestPanel === 'query'" class="space-y-1">
                      <div
                        v-for="(param, index) in formatQueryParams(fullLog.backend_request?.query_params || {})"
                        :key="index"
                        class="flex gap-2 py-1 border-b border-gray-800 last:border-0"
                      >
                        <span class="text-purple-400 text-xs font-medium flex-shrink-0">{{ param.key }}:</span>
                        <span class="text-gray-300 text-xs break-all">{{ param.value }}</span>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Backend Response Section -->
                <div class="flex-1 flex flex-col min-h-0">
                  <div class="px-4 py-2 bg-gray-900/50 border-b border-gray-700 flex-shrink-0">
                    <h3 class="text-xs font-semibold text-orange-400">Backend Response</h3>
                    <div class="mt-1 flex items-center gap-3 text-xs text-gray-400">
                      <span>Status: {{ fullLog.backend_response?.status_code || 'N/A' }} {{ fullLog.backend_response?.status_text || '' }}</span>
                      <span>Delay: {{ formatMs(fullLog.backend_response?.delay_ms || 0) }}</span>
                      <span>RTT: {{ formatMs(fullLog.backend_response?.rtt_ms || 0) }}</span>
                    </div>
                  </div>

                  <div class="flex border-b border-gray-700 flex-shrink-0 bg-gray-900/30">
                    <button
                      @click="activeBackendResponsePanel = 'headers'"
                      :class="[
                        'px-3 py-1.5 text-xs font-medium transition-colors',
                        activeBackendResponsePanel === 'headers'
                          ? 'text-orange-400 border-b-2 border-orange-400 bg-gray-900/50'
                          : 'text-gray-400 hover:text-gray-300'
                      ]"
                    >
                      Headers
                      <span class="ml-1 px-1 py-0.5 bg-gray-700 rounded text-xs">
                        {{ Object.keys(fullLog.backend_response?.headers || {}).length }}
                      </span>
                    </button>
                    <button
                      @click="activeBackendResponsePanel = 'body'"
                      :class="[
                        'px-3 py-1.5 text-xs font-medium transition-colors',
                        activeBackendResponsePanel === 'body'
                          ? 'text-orange-400 border-b-2 border-orange-400 bg-gray-900/50'
                          : 'text-gray-400 hover:text-gray-300'
                      ]"
                    >
                      Body
                      <span v-if="fullLog.backend_response?.body" class="ml-1 px-1 py-0.5 bg-gray-700 rounded text-xs">
                        {{ fullLog.backend_response.body.length }}
                      </span>
                    </button>
                  </div>

                  <div class="flex-1 overflow-y-auto px-4 py-3 min-h-0">
                    <div v-if="activeBackendResponsePanel === 'headers'" class="space-y-1">
                      <div
                        v-for="(header, index) in formatHeaders(fullLog.backend_response?.headers || {})"
                        :key="index"
                        class="flex gap-2 py-1 border-b border-gray-800 last:border-0"
                      >
                        <span class="text-orange-400 text-xs font-medium flex-shrink-0">{{ header.key }}:</span>
                        <span class="text-gray-300 text-xs break-all">{{ header.value }}</span>
                      </div>
                    </div>

                    <div v-if="activeBackendResponsePanel === 'body'" class="flex flex-col h-full">
                      <div v-if="fullLog.backend_response?.body" class="bg-gray-900 rounded p-3 flex-1 overflow-auto">
                        <pre class="text-xs text-gray-300 font-mono whitespace-pre-wrap break-all">{{ fullLog.backend_response.body }}</pre>
                      </div>
                      <div v-else class="text-gray-500 text-xs">
                        No body content
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

          <!-- Body Modal (read-only) -->
          <BodyEditorModal
            v-if="fullLog.client_request?.body"
            :model-value="fullLog.client_request.body"
            v-model:visible="showBodyModal"
            :content-type="effectiveContentType"
            :read-only="true"
            title="Request Body"
          />
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
