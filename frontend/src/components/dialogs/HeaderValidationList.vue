<script lang="ts" setup>
import { ref, watch } from 'vue'
import type { models } from '../../../wailsjs/go/models'
import CustomSelect from '../common/CustomSelect.vue'

const props = defineProps<{
  modelValue: models.HeaderValidation[]
}>()

const emit = defineEmits<{
  'update:modelValue': [headers: models.HeaderValidation[]]
}>()

interface HeaderRow extends models.HeaderValidation {
  id: string
}

const headers = ref<HeaderRow[]>([])

// Mode options for dropdown
const modeOptions = [
  { value: 'none', label: 'None - No validation (optional header)' },
  { value: 'exact', label: 'Exact - Must match exactly' },
  { value: 'contains', label: 'Contains - Must contain substring' },
  { value: 'regex', label: 'Regex - Match pattern' },
  { value: 'script', label: 'Script - Custom JavaScript validation' }
]

// Initialize with props
if (props.modelValue && props.modelValue.length > 0) {
  headers.value = props.modelValue.map((h, i) => ({
    ...h,
    id: `header-${i}-${Date.now()}`
  }))
}

// Watch for changes to modelValue from parent
let isUpdatingFromProp = false
watch(() => props.modelValue, (newValue) => {
  if (isUpdatingFromProp) return

  // Don't rebuild if the parent's value matches our filtered local value
  // This prevents losing locally-added empty headers that haven't been saved yet
  const currentFiltered = headers.value
    .filter(h => h.name.trim() !== '')
    .map(({ id, ...h }) => h)

  const newFiltered = (newValue || []).map(h => ({ ...h }))

  // Compare filtered values - if they're the same, don't rebuild
  if (JSON.stringify(currentFiltered) === JSON.stringify(newFiltered)) {
    return
  }

  isUpdatingFromProp = true
  if (newValue) {
    // Preserve existing IDs where possible to avoid Vue re-rendering issues
    headers.value = newValue.map((h, i) => {
      const existing = headers.value.find(existing =>
        existing.name === h.name &&
        existing.mode === h.mode &&
        existing.value === h.value &&
        existing.pattern === h.pattern &&
        existing.expression === h.expression
      )
      return {
        ...h,
        id: existing?.id || `header-${i}-${Date.now()}`
      }
    })
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
    mode: 'exact',
    value: '',
    pattern: '',
    expression: '',
    required: false
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
  if (isUpdatingFromProp) return

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
        <h4 class="text-sm font-medium text-white">Header Validation Rules</h4>
        <button
          @click="addHeader"
          class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded transition-colors flex items-center gap-1"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Add Header Validation
        </button>
      </div>

      <!-- Header Rows -->
      <div v-if="headers.length > 0" class="space-y-3">
        <div
          v-for="(header, index) in headers"
          :key="header.id"
          class="flex gap-2 items-start p-3 bg-gray-700/50 rounded border border-gray-600"
        >
          <div class="flex-1 space-y-2">
            <!-- Header Name and Required -->
            <div class="flex gap-2">
              <div class="flex-1">
                <label class="block text-xs text-gray-400 mb-1">Header Name</label>
                <input
                  v-model="header.name"
                  type="text"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                         focus:outline-none focus:border-blue-500"
                  placeholder="e.g., Authorization, Content-Type"
                />
              </div>
              <div class="flex items-end pb-2">
                <label class="flex items-center gap-2 text-xs text-gray-300 cursor-pointer">
                  <input
                    v-model="header.required"
                    type="checkbox"
                    class="w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600
                           focus:ring-blue-500 focus:ring-2"
                  />
                  Required
                </label>
              </div>
            </div>

            <!-- Mode Selection -->
            <div>
              <label class="block text-xs text-gray-400 mb-1">Validation Mode</label>
              <CustomSelect
                v-model="header.mode!"
                :options="modeOptions"
              />
            </div>

            <!-- Value or Pattern or Expression based on mode -->
            <div v-if="header.mode === 'exact' || header.mode === 'contains'">
              <label class="block text-xs text-gray-400 mb-1">
                {{ header.mode === 'exact' ? 'Expected Value (exact match)' : 'Expected Substring' }}
              </label>
              <input
                v-model="header.value"
                type="text"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                       focus:outline-none focus:border-blue-500"
                :placeholder="header.mode === 'exact' ? 'Header must match this value exactly' : 'Header must contain this substring'"
              />
            </div>

            <div v-else-if="header.mode === 'regex'">
              <label class="block text-xs text-gray-400 mb-1">Regular Expression Pattern</label>
              <input
                v-model="header.pattern"
                type="text"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono
                       focus:outline-none focus:border-blue-500"
                placeholder="e.g., ^Bearer\s+[A-Za-z0-9-._~+/]+=*$"
              />
            </div>

            <div v-else-if="header.mode === 'script'">
              <label class="block text-xs text-gray-400 mb-1">JavaScript Expression (must return boolean)</label>
              <textarea
                v-model="header.expression"
                rows="3"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono
                       focus:outline-none focus:border-blue-500 resize-y"
                placeholder="headerValue.startsWith('Bearer ') && headerValue.length > 20"
              ></textarea>
            </div>
          </div>

          <!-- Remove Button -->
          <button
            @click="removeHeader(index)"
            class="p-2 text-gray-400 hover:text-red-400 transition-colors mt-6"
            title="Remove validation rule"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <div v-else class="text-center py-8 text-gray-400 text-sm">
        No header validation rules. Click "Add Header Validation" to create one.
      </div>
    </div>

    <!-- Helper Info -->
    <div class="p-4 bg-gray-700/50 rounded border border-gray-600">
      <p class="text-sm font-medium text-white mb-2">Available Variables (Script Mode)</p>
      <div class="space-y-1 text-xs text-gray-300 font-mono">
        <p><span class="text-blue-400">headerValue</span> - The value of the header being validated</p>
        <p><span class="text-blue-400">headerName</span> - The name of the header</p>
        <p><span class="text-blue-400">request.method</span> - HTTP method (GET, POST, etc.)</p>
        <p><span class="text-blue-400">request.headers</span> - All request headers</p>
        <p><span class="text-blue-400">request.body</span> - Request body (if any)</p>
      </div>

      <p class="text-sm font-medium text-white mt-4 mb-2">Validation Logic</p>
      <div class="space-y-2 text-xs text-gray-300">
        <p><span class="text-yellow-400 font-medium">Required:</span> If checked, request fails if header is missing</p>
        <p><span class="text-green-400 font-medium">Optional:</span> If unchecked, validation only runs if header is present</p>
        <p><span class="text-purple-400 font-medium">AND Logic:</span> All header validations must pass (combined with body validation)</p>
      </div>
    </div>

    <!-- Examples -->
    <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
      <p class="text-sm font-medium text-blue-300 mb-2">Example Use Cases</p>
      <div class="space-y-3 text-xs text-blue-200">
        <div>
          <p class="font-medium text-blue-300">Require Bearer authentication:</p>
          <p class="font-mono text-gray-300 mt-1">Name: Authorization, Mode: Regex, Required: Yes</p>
          <p class="font-mono text-gray-300">Pattern: ^Bearer\s+.+$</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">Require JSON content type:</p>
          <p class="font-mono text-gray-300 mt-1">Name: Content-Type, Mode: Contains, Required: Yes</p>
          <p class="font-mono text-gray-300">Value: application/json</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">Custom API key validation:</p>
          <p class="font-mono text-gray-300 mt-1">Name: X-API-Key, Mode: Script, Required: Yes</p>
          <p class="font-mono text-gray-300">Expression: headerValue.length === 32 && /^[a-f0-9]+$/.test(headerValue)</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">Optional user agent check:</p>
          <p class="font-mono text-gray-300 mt-1">Name: User-Agent, Mode: Contains, Required: No</p>
          <p class="font-mono text-gray-300">Value: Mozilla</p>
        </div>
      </div>
    </div>
  </div>
</template>
