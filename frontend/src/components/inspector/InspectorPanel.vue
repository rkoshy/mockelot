<script lang="ts" setup>
import { ref, computed } from 'vue'
import { useServerStore } from '../../stores/server'
import BodyEditorModal from '../shared/BodyEditorModal.vue'
import FormatterSelector from '../shared/FormatterSelector.vue'
import PrometheusViewer from '../shared/PrometheusViewer.vue'
import { formatContent, detectContentType, supportsFormatting } from '../../utils/formatter'
import { isPrometheusMetrics } from '../../utils/prometheus-formatter'

const serverStore = useServerStore()
const activeTab = ref<'headers' | 'body' | 'query'>('headers')
const showBodyModal = ref(false)
const isRaw = ref(false)
const formattedBody = ref('')
const formatterOverride = ref('') // Empty means auto-detect
const viewMode = ref<'text' | 'table'>('text') // For Prometheus table view

const selectedLog = computed(() => serverStore.selectedLog)

// Get content type from request headers
const requestContentType = computed(() => {
  if (!selectedLog.value) return ''
  const headers = selectedLog.value.Headers
  for (const [key, values] of Object.entries(headers)) {
    if (key.toLowerCase() === 'content-type' && values.length > 0) {
      return values[0]
    }
  }
  return ''
})

// Detected content type (from header or auto-detected)
const detectedContentType = computed(() => {
  if (!selectedLog.value?.Body) return ''
  return requestContentType.value || detectContentType(selectedLog.value.Body)
})

// Effective content type (override or detected)
const effectiveContentType = computed(() => {
  return formatterOverride.value || detectedContentType.value
})

const canFormat = computed(() => supportsFormatting(effectiveContentType.value))

// Check if current content is Prometheus (supports table view)
const isPrometheus = computed(() => {
  if (!selectedLog.value?.Body) return false
  const type = effectiveContentType.value.toLowerCase()
  return type.includes('version=0.0.4') ||
         type === 'application/openmetrics-text' ||
         isPrometheusMetrics(selectedLog.value.Body)
})

// Format body when switching to formatted view
async function formatBody() {
  if (!selectedLog.value?.Body || !canFormat.value) return
  try {
    formattedBody.value = await formatContent(selectedLog.value.Body, effectiveContentType.value)
  } catch {
    formattedBody.value = selectedLog.value.Body
  }
}

// Display body
const displayBody = computed(() => {
  if (!selectedLog.value?.Body) return ''
  if (isRaw.value || !canFormat.value) {
    return selectedLog.value.Body
  }
  return formattedBody.value || selectedLog.value.Body
})

// Watch for body changes and format
import { watch } from 'vue'
watch(() => selectedLog.value?.Body, async () => {
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
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Header -->
    <div class="p-3 border-b border-gray-700 flex-shrink-0">
      <h2 class="text-lg font-semibold text-white">Request Inspector</h2>
    </div>

    <!-- Empty State -->
    <div v-if="!selectedLog" class="flex-1 flex items-center justify-center">
      <div class="text-center text-gray-500">
        <svg class="w-16 h-16 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
                d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
        </svg>
        <p class="text-lg">No request selected</p>
        <p class="text-sm mt-1">Click a request in the log to inspect it</p>
      </div>
    </div>

    <!-- Inspector Content -->
    <template v-else>
      <!-- Request Summary -->
      <div class="p-3 bg-gray-800/50 border-b border-gray-700 flex-shrink-0">
        <div class="flex items-center gap-2 mb-2">
          <span class="px-2 py-0.5 bg-blue-600 rounded text-xs font-bold text-white">
            {{ selectedLog.Method }}
          </span>
          <span class="px-2 py-0.5 bg-gray-700 rounded text-xs font-mono text-gray-300">
            {{ selectedLog.StatusCode }}
          </span>
        </div>
        <p class="text-sm text-gray-300 font-mono break-all">{{ selectedLog.Path }}</p>
        <div class="mt-2 flex items-center gap-4 text-xs text-gray-500">
          <span>{{ formatTimestamp(selectedLog.Timestamp) }}</span>
          <span>{{ selectedLog.SourceIP }}</span>
          <span>{{ selectedLog.Protocol }}</span>
        </div>
      </div>

      <!-- Tabs -->
      <div class="flex border-b border-gray-700 flex-shrink-0">
        <button
          @click="activeTab = 'headers'"
          :class="[
            'px-4 py-2 text-sm font-medium transition-colors',
            activeTab === 'headers'
              ? 'text-blue-400 border-b-2 border-blue-400'
              : 'text-gray-400 hover:text-gray-300'
          ]"
        >
          Headers
          <span class="ml-1 px-1.5 py-0.5 bg-gray-700 rounded text-xs">
            {{ Object.keys(selectedLog.Headers).length }}
          </span>
        </button>
        <button
          @click="activeTab = 'body'"
          :class="[
            'px-4 py-2 text-sm font-medium transition-colors',
            activeTab === 'body'
              ? 'text-blue-400 border-b-2 border-blue-400'
              : 'text-gray-400 hover:text-gray-300'
          ]"
        >
          Body
          <span v-if="selectedLog.Body" class="ml-1 px-1.5 py-0.5 bg-gray-700 rounded text-xs">
            {{ selectedLog.Body.length }}
          </span>
        </button>
        <button
          @click="activeTab = 'query'"
          :class="[
            'px-4 py-2 text-sm font-medium transition-colors',
            activeTab === 'query'
              ? 'text-blue-400 border-b-2 border-blue-400'
              : 'text-gray-400 hover:text-gray-300'
          ]"
        >
          Query
          <span class="ml-1 px-1.5 py-0.5 bg-gray-700 rounded text-xs">
            {{ Object.keys(selectedLog.QueryParams).length }}
          </span>
        </button>
      </div>

      <!-- Tab Content -->
      <div class="flex-1 overflow-y-auto p-3">
        <!-- Headers Tab -->
        <div v-if="activeTab === 'headers'" class="space-y-1">
          <div
            v-for="(header, index) in formatHeaders(selectedLog.Headers)"
            :key="index"
            class="flex gap-2 py-1 border-b border-gray-800 last:border-0"
          >
            <span class="text-blue-400 text-sm font-medium flex-shrink-0">{{ header.key }}:</span>
            <span class="text-gray-300 text-sm break-all">{{ header.value }}</span>
          </div>
          <div v-if="Object.keys(selectedLog.Headers).length === 0" class="text-gray-500 text-sm">
            No headers
          </div>
        </div>

        <!-- Body Tab -->
        <div v-if="activeTab === 'body'" class="flex flex-col h-full">
          <div v-if="selectedLog.Body" class="flex flex-col flex-1 min-h-0">
            <!-- Body toolbar -->
            <div class="flex items-center justify-between mb-2 flex-shrink-0">
              <div class="flex items-center gap-2 flex-wrap">
                <span v-if="detectedContentType" class="px-2 py-0.5 bg-gray-700 rounded text-xs text-gray-400 font-mono">
                  {{ detectedContentType }}
                </span>
                <!-- Formatter Override Selector -->
                <FormatterSelector v-model="formatterOverride" />
                <!-- View Mode Toggle (for Prometheus) -->
                <div v-if="isPrometheus" class="flex items-center bg-gray-700 rounded overflow-hidden">
                  <button
                    @click="viewMode = 'text'"
                    :class="[
                      'px-2 py-0.5 text-xs transition-colors',
                      viewMode === 'text'
                        ? 'bg-blue-600 text-white'
                        : 'text-gray-300 hover:bg-gray-600'
                    ]"
                  >
                    Text
                  </button>
                  <button
                    @click="viewMode = 'table'"
                    :class="[
                      'px-2 py-0.5 text-xs transition-colors',
                      viewMode === 'table'
                        ? 'bg-blue-600 text-white'
                        : 'text-gray-300 hover:bg-gray-600'
                    ]"
                  >
                    Table
                  </button>
                </div>
                <!-- Raw/Formatted toggle -->
                <button
                  v-if="canFormat && viewMode === 'text'"
                  @click="isRaw = !isRaw"
                  :class="[
                    'px-2 py-0.5 rounded text-xs transition-colors',
                    isRaw
                      ? 'bg-gray-700 text-gray-400'
                      : 'bg-blue-600 text-white'
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

            <!-- Prometheus Table View -->
            <div
              v-if="isPrometheus && viewMode === 'table'"
              class="bg-gray-800 rounded p-3 flex-1 overflow-auto"
            >
              <PrometheusViewer :content="selectedLog.Body" />
            </div>

            <!-- Text View -->
            <div v-else class="bg-gray-800 rounded p-3 flex-1 overflow-auto">
              <pre class="text-sm text-gray-300 font-mono whitespace-pre-wrap break-all">{{ displayBody }}</pre>
            </div>
          </div>
          <div v-else class="text-gray-500 text-sm">
            No body content
          </div>
        </div>

        <!-- Query Tab -->
        <div v-if="activeTab === 'query'" class="space-y-1">
          <div
            v-for="(param, index) in formatQueryParams(selectedLog.QueryParams)"
            :key="index"
            class="flex gap-2 py-1 border-b border-gray-800 last:border-0"
          >
            <span class="text-purple-400 text-sm font-medium flex-shrink-0">{{ param.key }}:</span>
            <span class="text-gray-300 text-sm break-all">{{ param.value }}</span>
          </div>
          <div v-if="Object.keys(selectedLog.QueryParams).length === 0" class="text-gray-500 text-sm">
            No query parameters
          </div>
        </div>
      </div>

      <!-- Body Modal (read-only) -->
      <BodyEditorModal
        v-if="selectedLog.Body"
        :model-value="selectedLog.Body"
        v-model:visible="showBodyModal"
        :content-type="effectiveContentType"
        :read-only="true"
        title="Request Body"
      />
    </template>
  </div>
</template>
