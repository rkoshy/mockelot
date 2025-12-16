<script lang="ts" setup>
import { ref, watch } from 'vue'
import type { models } from '../../../wailsjs/go/models'
import CustomSelect from '../common/CustomSelect.vue'

const props = defineProps<{
  modelValue: models.HeaderManipulation[]
  direction: 'inbound' | 'outbound'
  showResetDefaults?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [headers: models.HeaderManipulation[]]
  'reset-defaults': []
}>()

interface HeaderRow extends models.HeaderManipulation {
  id: string
}

const headers = ref<HeaderRow[]>([])

// Mode options for dropdown
const modeOptions = [
  { value: 'drop', label: 'Drop - Remove header' },
  { value: 'replace', label: 'Replace - Set static value' },
  { value: 'expression', label: 'Expression - Dynamic value (JS)' }
]

// Initialize with props
if (props.modelValue && props.modelValue.length > 0) {
  headers.value = props.modelValue.map((h, i) => ({
    ...h,
    id: `header-${i}-${Date.now()}`
  }))
}

// Watch for changes to modelValue from parent (e.g., when reset button is clicked)
// Use a flag to prevent infinite loop
let isUpdatingFromProp = false
watch(() => props.modelValue, (newValue) => {
  if (isUpdatingFromProp) return // Prevent loop

  isUpdatingFromProp = true
  if (newValue) {
    headers.value = newValue.map((h, i) => ({
      ...h,
      id: `header-${i}-${Date.now()}`
    }))
  } else {
    headers.value = []
  }
  isUpdatingFromProp = false
}, { deep: true })

// Add new header row
function addHeader() {
  headers.value.push({
    id: `header-${headers.value.length}-${Date.now()}`,
    name: '',
    mode: 'replace',
    value: '',
    expression: ''
  })
  emitHeaders()
}

// Remove header row
function removeHeader(index: number) {
  headers.value.splice(index, 1)
  emitHeaders()
}

// Emit headers update
function emitHeaders() {
  if (isUpdatingFromProp) return // Prevent emitting while updating from prop

  const validHeaders = headers.value
    .filter(h => h.name.trim() !== '')
    .map(({ id, ...h }) => h)
  emit('update:modelValue', validHeaders)
}

// Watch for changes
watch(headers, () => {
  emitHeaders()
}, { deep: true })
</script>

<template>
  <div class="space-y-4">
    <!-- Headers Table -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <h4 class="text-sm font-medium text-white">
          {{ direction === 'inbound' ? 'Inbound' : 'Outbound' }} Header Manipulation
        </h4>
        <div class="flex gap-2">
          <button
            v-if="showResetDefaults"
            @click="emit('reset-defaults')"
            class="px-3 py-1 bg-yellow-600 hover:bg-yellow-700 text-white text-sm rounded transition-colors flex items-center gap-1"
            title="Reset to default container headers"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Reset to Defaults
          </button>
          <button
            @click="addHeader"
            class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded transition-colors flex items-center gap-1"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            Add Header Rule
          </button>
        </div>
      </div>

      <!-- Header Rows -->
      <div v-if="headers.length > 0" class="space-y-3">
        <div
          v-for="(header, index) in headers"
          :key="header.id"
          class="flex gap-2 items-start p-3 bg-gray-700/50 rounded border border-gray-600"
        >
          <div class="flex-1 space-y-2">
            <!-- Header Name -->
            <div>
              <label class="block text-xs text-gray-400 mb-1">Header Name</label>
              <input
                v-model="header.name"
                type="text"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                       focus:outline-none focus:border-blue-500"
                placeholder="e.g., Authorization, X-Custom-Header"
              />
            </div>

            <!-- Mode Selection -->
            <div>
              <label class="block text-xs text-gray-400 mb-1">Mode</label>
              <CustomSelect
                v-model="header.mode"
                :options="modeOptions"
              />
            </div>

            <!-- Value or Expression based on mode -->
            <div v-if="header.mode === 'replace'">
              <label class="block text-xs text-gray-400 mb-1">Value</label>
              <input
                v-model="header.value"
                type="text"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                       focus:outline-none focus:border-blue-500"
                placeholder="Static header value"
              />
            </div>

            <div v-else-if="header.mode === 'expression'">
              <label class="block text-xs text-gray-400 mb-1">JavaScript Expression</label>
              <input
                v-model="header.expression"
                type="text"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono
                       focus:outline-none focus:border-blue-500"
                placeholder="e.g., request.headers['User-Agent']"
              />
            </div>
          </div>

          <!-- Remove Button -->
          <button
            @click="removeHeader(index)"
            class="p-2 text-gray-400 hover:text-red-400 transition-colors mt-6"
            title="Remove header rule"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <div v-else class="text-center py-8 text-gray-400 text-sm">
        No header manipulation rules. Click "Add Header Rule" to create one.
      </div>
    </div>

    <!-- Helper Info -->
    <div class="p-4 bg-gray-700/50 rounded border border-gray-600">
      <p class="text-sm font-medium text-white mb-2">Available Variables (Expression Mode)</p>
      <div class="space-y-1 text-xs text-gray-300 font-mono">
        <p><span class="text-blue-400">request.method</span> - HTTP method (GET, POST, etc.)</p>
        <p><span class="text-blue-400">request.path</span> - Request path</p>
        <p><span class="text-blue-400">request.headers</span> - Original request headers</p>
      </div>

      <p class="text-sm font-medium text-white mt-4 mb-2">Mode Descriptions</p>
      <div class="space-y-2 text-xs text-gray-300">
        <p><span class="text-yellow-400 font-medium">Drop:</span> Removes the header completely</p>
        <p><span class="text-green-400 font-medium">Replace:</span> Sets header to a static value</p>
        <p><span class="text-purple-400 font-medium">Expression:</span> Dynamically compute value using JavaScript</p>
      </div>
    </div>

    <!-- Examples -->
    <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
      <p class="text-sm font-medium text-blue-300 mb-2">Example Use Cases</p>
      <div class="space-y-3 text-xs text-blue-200">
        <div>
          <p class="font-medium text-blue-300">Add authentication token:</p>
          <p class="font-mono text-gray-300 mt-1">Mode: Replace, Name: Authorization, Value: Bearer YOUR_TOKEN</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">Forward user agent:</p>
          <p class="font-mono text-gray-300 mt-1">Mode: Expression, Name: X-Forwarded-User-Agent</p>
          <p class="font-mono text-gray-300">Expression: request.headers['User-Agent']</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">Remove sensitive header:</p>
          <p class="font-mono text-gray-300 mt-1">Mode: Drop, Name: X-Internal-Token</p>
        </div>
      </div>
    </div>
  </div>
</template>
