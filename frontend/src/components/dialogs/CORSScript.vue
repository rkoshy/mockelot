<script lang="ts" setup>
import { ref, watch } from 'vue'
import { ValidateCORSScript } from '../../../wailsjs/go/main/App'

const props = defineProps<{
  initialScript?: string
}>()

const emit = defineEmits<{
  validationChange: [valid: boolean]
  'update:script': [script: string]
}>()

const script = ref(props.initialScript || `// CORS Script Example
// Return an object with header names and values

(function() {
  const headers = {};

  // Dynamic origin reflection
  if (request.origin) {
    headers['Access-Control-Allow-Origin'] = request.origin;
    headers['Access-Control-Allow-Credentials'] = 'true';
  } else {
    headers['Access-Control-Allow-Origin'] = '*';
  }

  // Allow common methods
  headers['Access-Control-Allow-Methods'] = 'GET, POST, PUT, DELETE, OPTIONS, PATCH';

  // Allow common headers
  headers['Access-Control-Allow-Headers'] = 'Content-Type, Authorization';

  // Cache preflight for 1 hour
  headers['Access-Control-Max-Age'] = '3600';

  return headers;
})()`)

const validationError = ref('')
const isValid = ref(true)
const isValidating = ref(false)

// Debounce validation
let validationTimeout: number | null = null

async function validateScript() {
  if (!script.value.trim()) {
    validationError.value = ''
    isValid.value = false
    emit('validationChange', false)
    return
  }

  isValidating.value = true

  try {
    await ValidateCORSScript(script.value)
    validationError.value = ''
    isValid.value = true
    emit('validationChange', true)
  } catch (error) {
    validationError.value = String(error).replace('Error: ', '').replace('syntax error: ', '')
    isValid.value = false
    emit('validationChange', false)
  } finally {
    isValidating.value = false
  }

  emitScript()
}

function emitScript() {
  emit('update:script', script.value)
}

// Watch for script changes with debounce
watch(script, () => {
  if (validationTimeout !== null) {
    clearTimeout(validationTimeout)
  }
  validationTimeout = window.setTimeout(() => {
    validateScript()
  }, 500)
})

// Initial validation
validateScript()
</script>

<template>
  <div class="space-y-4">
    <!-- Code Editor ---->
    <div>
      <div class="flex items-center justify-between mb-2">
        <h4 class="text-sm font-medium text-white">CORS Script</h4>
        <div class="flex items-center gap-2">
          <span v-if="isValidating" class="text-xs text-gray-400">Validating...</span>
          <span v-else-if="isValid && script.trim()" class="text-xs text-green-400">✓ Valid</span>
          <span v-else-if="validationError" class="text-xs text-red-400">⚠ Invalid</span>
        </div>
      </div>

      <textarea
        v-model="script"
        :class="[
          'w-full px-3 py-2 bg-gray-700 border rounded text-white text-sm font-mono',
          'focus:outline-none resize-none',
          validationError ? 'border-red-500' : 'border-gray-600 focus:border-blue-500'
        ]"
        rows="12"
        placeholder="Write JavaScript to return CORS headers object"
      ></textarea>

      <p v-if="validationError" class="mt-2 text-xs text-red-400">
        {{ validationError }}
      </p>
    </div>

    <!-- Helper Info ---->
    <div class="p-4 bg-gray-700/50 rounded border border-gray-600">
      <p class="text-sm font-medium text-white mb-2">Available Variables</p>
      <div class="space-y-1 text-xs text-gray-300 font-mono">
        <p><span class="text-blue-400">request.method</span> - HTTP method (string)</p>
        <p><span class="text-blue-400">request.path</span> - Request path (string)</p>
        <p><span class="text-blue-400">request.origin</span> - Origin header (string)</p>
        <p><span class="text-blue-400">request.headers</span> - All headers (object)</p>
      </div>

      <p class="text-sm font-medium text-white mt-4 mb-2">Helper Functions</p>
      <div class="space-y-1 text-xs text-gray-300 font-mono">
        <p><span class="text-green-400">matchOrigin(pattern)</span> - Match origin against wildcard pattern</p>
        <p><span class="text-green-400">allowOrigins([...])</span> - Check if origin is in allowed list</p>
        <p><span class="text-green-400">getOrigin()</span> - Get request origin header</p>
        <p><span class="text-green-400">getHeader(name)</span> - Get specific request header</p>
      </div>

      <p class="text-sm font-medium text-white mt-4 mb-2">Return Value</p>
      <div class="text-xs text-gray-300">
        <p>Script must return an object with header names as keys and values as strings:</p>
        <pre class="mt-2 p-2 bg-gray-800 rounded font-mono">
{
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'GET, POST',
  'Access-Control-Allow-Headers': 'Content-Type'
}</pre>
      </div>
    </div>

    <!-- Examples ---->
    <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
      <p class="text-sm font-medium text-blue-300 mb-2">Example Scripts</p>

      <div class="space-y-4 text-xs">
        <!-- Example 1: Dynamic Origin -->
        <div>
          <p class="text-gray-400 mb-1">// Dynamic origin with credentials</p>
          <pre class="p-2 bg-gray-800 rounded text-blue-300 font-mono overflow-x-auto">
(function() {
  const headers = {};
  if (request.origin) {
    headers['Access-Control-Allow-Origin'] = request.origin;
    headers['Access-Control-Allow-Credentials'] = 'true';
  } else {
    headers['Access-Control-Allow-Origin'] = '*';
  }
  return headers;
})()</pre>
        </div>

        <!-- Example 2: Whitelist Origins -->
        <div>
          <p class="text-gray-400 mb-1">// Whitelist specific origins</p>
          <pre class="p-2 bg-gray-800 rounded text-blue-300 font-mono overflow-x-auto">
(function() {
  const allowed = ['https://app.example.com', 'https://admin.example.com'];
  const headers = {};

  if (allowOrigins(allowed)) {
    headers['Access-Control-Allow-Origin'] = request.origin;
    headers['Access-Control-Allow-Credentials'] = 'true';
  } else {
    headers['Access-Control-Allow-Origin'] = 'null';
  }

  headers['Access-Control-Allow-Methods'] = 'GET, POST, PUT, DELETE';
  headers['Access-Control-Allow-Headers'] = 'Content-Type, Authorization';

  return headers;
})()</pre>
        </div>

        <!-- Example 3: Pattern Matching -->
        <div>
          <p class="text-gray-400 mb-1">// Wildcard pattern matching</p>
          <pre class="p-2 bg-gray-800 rounded text-blue-300 font-mono overflow-x-auto">
(function() {
  const headers = {};

  if (matchOrigin('https://*.example.com')) {
    headers['Access-Control-Allow-Origin'] = request.origin;
  } else {
    headers['Access-Control-Allow-Origin'] = 'null';
  }

  return headers;
})()</pre>
        </div>

        <!-- Example 4: Conditional Headers -->
        <div>
          <p class="text-gray-400 mb-1">// Conditional headers based on method</p>
          <pre class="p-2 bg-gray-800 rounded text-blue-300 font-mono overflow-x-auto">
(function() {
  const headers = {
    'Access-Control-Allow-Origin': request.origin || '*',
    'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS'
  };

  // Only allow custom headers for authenticated requests
  if (getHeader('Authorization')) {
    headers['Access-Control-Allow-Headers'] = 'Content-Type, Authorization, X-Custom-Header';
  } else {
    headers['Access-Control-Allow-Headers'] = 'Content-Type';
  }

  return headers;
})()</pre>
        </div>
      </div>
    </div>

    <!-- Important Notes ---->
    <div class="p-4 bg-yellow-900/20 border border-yellow-800 rounded">
      <p class="text-sm font-medium text-yellow-300 mb-2">Important Notes</p>
      <div class="space-y-1 text-xs text-yellow-300">
        <p>• Script must be wrapped in an IIFE (Immediately Invoked Function Expression)</p>
        <p>• Script execution has a 2-second timeout</p>
        <p>• Return value must be an object with string keys and string values</p>
        <p>• Invalid scripts will prevent CORS from being applied</p>
        <p>• Test your script thoroughly before enabling in production</p>
      </div>
    </div>
  </div>
</template>
