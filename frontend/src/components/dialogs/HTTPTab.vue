<script lang="ts" setup>
import { ref, watch, onMounted } from 'vue'
import { useServerStore } from '../../stores/server'

const emit = defineEmits<{
  validationChange: [valid: boolean]
}>()

const serverStore = useServerStore()
const httpPort = ref(serverStore.port || 8080)
const httpRedirect = ref(false)
const http2Enabled = ref(false)

// Load configuration
function loadHTTPConfig() {
  const config = serverStore.config
  if (config) {
    httpPort.value = config.port || 8080
    httpRedirect.value = config.http_to_https_redirect || false
    http2Enabled.value = config.http2_enabled || false
  }
}

onMounted(() => {
  loadHTTPConfig()
})

watch(httpPort, (newPort) => {
  // Validate port number
  const isValid = newPort >= 1 && newPort <= 65535
  emit('validationChange', isValid)
})

// Expose values for parent to read
defineExpose({
  getPort: () => httpPort.value,
  getRedirect: () => httpRedirect.value,
  getHTTP2Enabled: () => http2Enabled.value,
  loadHTTPConfig
})
</script>

<template>
  <div class="space-y-4">
    <!-- Two-column layout: Port and Redirect -->
    <div class="grid grid-cols-2 gap-6">
      <!-- HTTP Port -->
      <div>
        <label class="block text-sm font-medium text-gray-300 mb-2">
          HTTP Port
        </label>
        <input
          v-model.number="httpPort"
          type="number"
          min="1"
          max="65535"
          class="w-32 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white
                 focus:outline-none focus:border-blue-500"
          placeholder="8080"
        />
        <p class="mt-1 text-xs text-gray-400">
          Port for HTTP server (default: 8080)
        </p>
      </div>

      <!-- Redirect to HTTPS -->
      <div>
        <label class="block text-sm font-medium text-gray-300 mb-2">
          Redirect
        </label>
        <label class="flex items-center gap-2 cursor-pointer">
          <input
            v-model="httpRedirect"
            type="checkbox"
            class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
          />
          <span class="text-sm font-medium text-white">Redirect HTTP to HTTPS (302)</span>
        </label>
        <p class="mt-1 text-xs text-gray-400">
          Automatically redirect all HTTP requests to HTTPS
        </p>
      </div>
    </div>

    <!-- HTTP/2 Support -->
    <div class="border-t border-gray-700 pt-4">
      <label class="flex items-center gap-2 cursor-pointer">
        <input
          v-model="http2Enabled"
          type="checkbox"
          class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
        />
        <span class="text-sm font-medium text-white">Enable HTTP/2</span>
      </label>
      <p class="mt-1 text-xs text-gray-400">
        Enable HTTP/2 support for both HTTP and HTTPS servers. Provides better performance with multiplexing and header compression (requires server restart)
      </p>
    </div>
  </div>
</template>
