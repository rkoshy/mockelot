<script lang="ts" setup>
import { ref, watch } from 'vue'
import { ValidateCORSHeaderExpression } from '../../../wailsjs/go/main/App'

const props = defineProps<{
  initialHeaders?: Array<{name: string, expression: string}>
}>()

const emit = defineEmits<{
  validationChange: [valid: boolean]
  'update:headers': [headers: Array<{name: string, expression: string}>]
}>()

interface HeaderRow {
  name: string
  expression: string
  valid: boolean
  error: string
}

const headers = ref<HeaderRow[]>([])

// Initialize with props or defaults
if (props.initialHeaders && props.initialHeaders.length > 0) {
  headers.value = props.initialHeaders.map(h => ({
    name: h.name,
    expression: h.expression,
    valid: true,
    error: ''
  }))
} else {
  // Default CORS headers
  headers.value = [
    { name: 'Access-Control-Allow-Origin', expression: 'request.origin || "*"', valid: true, error: '' },
    { name: 'Access-Control-Allow-Methods', expression: '"GET, POST, PUT, DELETE, OPTIONS, PATCH"', valid: true, error: '' },
    { name: 'Access-Control-Allow-Headers', expression: '"Content-Type, Authorization"', valid: true, error: '' },
    { name: 'Access-Control-Max-Age', expression: '"3600"', valid: true, error: '' },
  ]
}

// Add new header row
function addHeader() {
  headers.value.push({
    name: '',
    expression: '',
    valid: false,
    error: ''
  })
}

// Remove header row
function removeHeader(index: number) {
  headers.value.splice(index, 1)
  emitHeaders()
}

// Validate expression
async function validateExpression(index: number) {
  const header = headers.value[index]

  if (!header.expression.trim()) {
    header.valid = false
    header.error = 'Expression required'
    emitValidation()
    return
  }

  try {
    await ValidateCORSHeaderExpression(header.expression)
    header.valid = true
    header.error = ''
  } catch (error) {
    header.valid = false
    header.error = String(error).replace('Error: ', '').replace('syntax error: ', '')
  }

  emitValidation()
  emitHeaders()
}

// Emit validation status
function emitValidation() {
  const allValid = headers.value.every(h => h.name.trim() !== '' && h.expression.trim() !== '' && h.valid)
  emit('validationChange', allValid)
}

// Emit headers update
function emitHeaders() {
  const validHeaders = headers.value
    .filter(h => h.name.trim() !== '' && h.expression.trim() !== '')
    .map(h => ({ name: h.name, expression: h.expression }))
  emit('update:headers', validHeaders)
}

// Watch for changes
watch(headers, () => {
  emitValidation()
  emitHeaders()
}, { deep: true })

// Initial validation
headers.value.forEach((_, index) => validateExpression(index))
</script>

<template>
  <div class="space-y-4">
    <!-- Headers Table -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <h4 class="text-sm font-medium text-white">CORS Headers</h4>
        <button
          @click="addHeader"
          class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded transition-colors flex items-center gap-1"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Add Header
        </button>
      </div>

      <!-- Header Rows -->
      <div class="space-y-3">
        <div
          v-for="(header, index) in headers"
          :key="index"
          class="flex gap-2 items-start"
        >
          <!-- Header Name -->
          <div class="flex-1">
            <input
              v-model="header.name"
              type="text"
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                     focus:outline-none focus:border-blue-500"
              placeholder="Header Name"
              @blur="validateExpression(index)"
            />
          </div>

          <!-- Expression -->
          <div class="flex-[2]">
            <input
              v-model="header.expression"
              type="text"
              :class="[
                'w-full px-3 py-2 bg-gray-700 border rounded text-white text-sm font-mono',
                'focus:outline-none',
                header.error && header.expression ? 'border-red-500' : 'border-gray-600 focus:border-blue-500'
              ]"
              placeholder="JavaScript expression"
              @blur="validateExpression(index)"
            />
            <p v-if="header.error && header.expression" class="mt-1 text-xs text-red-400">
              {{ header.error }}
            </p>
            <p v-else-if="header.valid && header.expression" class="mt-1 text-xs text-green-400">
              ✓ Valid
            </p>
          </div>

          <!-- Remove Button -->
          <button
            @click="removeHeader(index)"
            class="p-2 text-gray-400 hover:text-red-400 transition-colors"
            title="Remove header"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>
    </div>

    <!-- Helper Info -->
    <div class="p-4 bg-gray-700/50 rounded border border-gray-600">
      <p class="text-sm font-medium text-white mb-2">Available Variables</p>
      <div class="space-y-1 text-xs text-gray-300 font-mono">
        <p><span class="text-blue-400">request.method</span> - HTTP method</p>
        <p><span class="text-blue-400">request.path</span> - Request path</p>
        <p><span class="text-blue-400">request.origin</span> - Origin header</p>
        <p><span class="text-blue-400">request.headers</span> - All headers</p>
      </div>

      <p class="text-sm font-medium text-white mt-4 mb-2">Helper Functions</p>
      <div class="space-y-1 text-xs text-gray-300 font-mono">
        <p><span class="text-green-400">matchOrigin(pattern)</span> - Match origin against pattern</p>
        <p><span class="text-green-400">allowOrigins([...])</span> - Check if origin in list</p>
        <p><span class="text-green-400">getOrigin()</span> - Get request origin</p>
        <p><span class="text-green-400">getHeader(name)</span> - Get request header</p>
      </div>
    </div>

    <!-- Examples -->
    <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
      <p class="text-sm font-medium text-blue-300 mb-2">Example Expressions</p>
      <div class="space-y-1 text-xs text-blue-300 font-mono">
        <p><span class="text-gray-400">// Dynamic origin reflection</span></p>
        <p>request.origin || "*"</p>
        <p class="mt-2"><span class="text-gray-400">// Allow specific origins</span></p>
        <p>matchOrigin("https://*.example.com") ? request.origin : "null"</p>
        <p class="mt-2"><span class="text-gray-400">// Conditional credentials</span></p>
        <p>request.origin ? "true" : "false"</p>
      </div>
    </div>
  </div>
</template>
