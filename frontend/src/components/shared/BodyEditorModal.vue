<script lang="ts" setup>
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import { formatContent, minifyContent, supportsFormatting, detectContentType, FORMATTER_TYPES } from '../../utils/formatter'
import { isPrometheusMetrics } from '../../utils/prometheus-formatter'
import FormatterSelector from './FormatterSelector.vue'
import PrometheusViewer from './PrometheusViewer.vue'

const props = defineProps<{
  modelValue: string
  visible: boolean
  contentType?: string
  readOnly?: boolean
  title?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'update:visible', value: boolean): void
  (e: 'save'): void
}>()

const localValue = ref(props.modelValue)
const isRaw = ref(false)
const isFormatting = ref(false)
const formattedValue = ref('')
const formatterOverride = ref('') // Empty means auto-detect
const viewMode = ref<'text' | 'table'>('text') // For Prometheus table view

// Detected or provided content type
const detectedContentType = computed(() => {
  return props.contentType || detectContentType(localValue.value)
})

// Effective content type (override or detected)
const effectiveContentType = computed(() => {
  return formatterOverride.value || detectedContentType.value
})

const canFormat = computed(() => supportsFormatting(effectiveContentType.value))

// Check if current content is Prometheus (supports table view)
const isPrometheus = computed(() => {
  const type = effectiveContentType.value.toLowerCase()
  return type.includes('version=0.0.4') ||
         type === 'application/openmetrics-text' ||
         isPrometheusMetrics(localValue.value)
})

watch(() => props.modelValue, (newVal) => {
  localValue.value = newVal
  if (!isRaw.value && canFormat.value) {
    formatCurrentContent()
  }
})

watch(() => props.visible, async (isVisible) => {
  if (isVisible) {
    // Always sync with latest prop value when opening
    localValue.value = props.modelValue
    formattedValue.value = '' // Reset formatted value to force re-format
    isRaw.value = false
    formatterOverride.value = '' // Reset to auto on open
    viewMode.value = 'text' // Reset to text view
    if (canFormat.value) {
      await formatCurrentContent()
    }
  }
}, { immediate: true })

// Re-format when formatter override changes
watch(formatterOverride, async () => {
  if (!isRaw.value && canFormat.value) {
    await formatCurrentContent()
  }
})

async function formatCurrentContent() {
  if (!canFormat.value) return

  isFormatting.value = true
  try {
    formattedValue.value = await formatContent(localValue.value, effectiveContentType.value)
  } catch {
    formattedValue.value = localValue.value
  }
  isFormatting.value = false
}

// Display value based on raw mode
const displayValue = computed({
  get: () => isRaw.value ? localValue.value : (formattedValue.value || localValue.value),
  set: (val) => {
    localValue.value = val
    if (!isRaw.value) {
      formattedValue.value = val
    }
  }
})

function handleSave() {
  // Always save the raw value (minified if was formatted)
  emit('update:modelValue', localValue.value)
  emit('save')
  emit('update:visible', false)
}

function handleCancel() {
  emit('update:visible', false)
}

async function handleFormat() {
  isFormatting.value = true
  try {
    const formatted = await formatContent(localValue.value, effectiveContentType.value)
    localValue.value = formatted
    formattedValue.value = formatted
    isRaw.value = false
  } catch {
    // Keep as-is if formatting fails
  }
  isFormatting.value = false
}

function handleMinify() {
  const minified = minifyContent(localValue.value, effectiveContentType.value)
  localValue.value = minified
  formattedValue.value = minified
}

function toggleRaw() {
  isRaw.value = !isRaw.value
  if (!isRaw.value && canFormat.value) {
    formatCurrentContent()
  }
}

// Handle escape key to close
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    handleCancel()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <!-- Backdrop -->
      <div class="absolute inset-0 bg-black/70" />

      <!-- Modal -->
      <div class="relative w-[90vw] h-[90vh] bg-gray-800 rounded-lg border border-gray-600 shadow-2xl flex flex-col">
        <!-- Header -->
        <div class="flex items-center justify-between px-4 py-3 border-b border-gray-700">
          <div class="flex items-center gap-3">
            <h3 class="text-lg font-semibold text-white">{{ title || 'Edit Body' }}</h3>
            <span v-if="detectedContentType" class="px-2 py-0.5 bg-gray-700 rounded text-xs text-gray-300 font-mono">
              {{ detectedContentType }}
            </span>
            <!-- Formatter Override Selector -->
            <FormatterSelector v-model="formatterOverride" />
          </div>
          <div class="flex items-center gap-2">
            <!-- View Mode Toggle (for Prometheus) -->
            <div v-if="isPrometheus" class="flex items-center bg-gray-700 rounded overflow-hidden">
              <button
                @click="viewMode = 'text'"
                :class="[
                  'px-3 py-1.5 text-xs transition-colors',
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
                  'px-3 py-1.5 text-xs transition-colors',
                  viewMode === 'table'
                    ? 'bg-blue-600 text-white'
                    : 'text-gray-300 hover:bg-gray-600'
                ]"
              >
                Table
              </button>
            </div>
            <!-- Separator -->
            <div v-if="isPrometheus && canFormat" class="w-px h-5 bg-gray-600"></div>
            <!-- Raw/Formatted toggle -->
            <button
              v-if="canFormat && viewMode === 'text'"
              @click="toggleRaw"
              :class="[
                'px-3 py-1.5 rounded text-xs transition-colors',
                isRaw
                  ? 'bg-gray-700 text-gray-300 hover:bg-gray-600'
                  : 'bg-blue-600 text-white'
              ]"
            >
              {{ isRaw ? 'Raw' : 'Formatted' }}
            </button>
            <button
              v-if="canFormat && !readOnly && viewMode === 'text'"
              @click="handleFormat"
              :disabled="isFormatting"
              class="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors disabled:opacity-50"
            >
              {{ isFormatting ? 'Formatting...' : 'Format' }}
            </button>
            <button
              v-if="canFormat && !readOnly && viewMode === 'text'"
              @click="handleMinify"
              class="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors"
            >
              Minify
            </button>
            <button
              @click="handleCancel"
              class="p-1.5 text-gray-400 hover:text-white transition-colors"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        <!-- Editor -->
        <div class="flex-1 p-4 overflow-hidden">
          <!-- Prometheus Table View -->
          <div
            v-if="isPrometheus && viewMode === 'table'"
            class="w-full h-full overflow-auto bg-gray-900 border border-gray-600 rounded-lg p-4"
          >
            <PrometheusViewer :content="localValue" />
          </div>

          <!-- Text Editor View -->
          <textarea
            v-else
            v-model="displayValue"
            :readonly="readOnly"
            class="w-full h-full px-3 py-2 bg-gray-900 border border-gray-600 rounded-lg text-sm text-white
                   font-mono focus:outline-none focus:border-blue-500 resize-none"
            :class="{ 'cursor-not-allowed opacity-75': readOnly }"
            placeholder='{"message": "Hello, World!"}'
            spellcheck="false"
          />
        </div>

        <!-- Footer -->
        <div class="flex items-center justify-between px-4 py-3 border-t border-gray-700">
          <div class="text-xs text-gray-500">
            {{ localValue.length }} characters
          </div>
          <div class="flex items-center gap-3">
            <button
              @click="handleCancel"
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded text-sm text-gray-300 transition-colors"
            >
              {{ readOnly ? 'Close' : 'Cancel' }}
            </button>
            <button
              v-if="!readOnly"
              @click="handleSave"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-sm text-white font-medium transition-colors"
            >
              Apply Changes
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
