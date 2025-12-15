<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted, nextTick, watch, provide } from 'vue'
import { useServerStore } from '../../stores/server'
import { SaveConfig, LoadConfig, SetHTTPSConfig, SetCertMode, SetCORSConfig, SetHTTP2Enabled, StartContainers, PollEvents } from '../../../wailsjs/go/main/App'
import { models } from '../../../wailsjs/go/models'
import ConfirmDialog from '../dialogs/ConfirmDialog.vue'
import ServerConfigDialog from '../dialogs/ServerConfigDialog.vue'
import ContainerProgressDialog from '../dialogs/ContainerProgressDialog.vue'
import { EventsOn } from '../../../wailsjs/runtime/runtime'

// Event structure from backend
interface BackendEvent {
  source: string
  data: any
}

// Type for event handler callbacks
type EventCallback = (data: any) => void

const serverStore = useServerStore()
const portInput = ref(8080)
const isLoading = ref(false)
const errorMessage = ref('')
const showImportDialog = ref(false)
const showServerConfigDialog = ref(false)
const serverConfigDialogTab = ref<'http' | 'https'>('http')
const serverConfigDialogRef = ref<InstanceType<typeof ServerConfigDialog> | null>(null)

// Container progress dialog state
const showProgressDialog = ref(false)
const progressEndpointName = ref('')
const progressDialogRef = ref<InstanceType<typeof ContainerProgressDialog>>()
const pendingProgressEvents = ref<any[]>([])

// Event log for debugging
const eventLog = ref<Array<{time: string, type: string, data: string}>>([])
const showEventLog = ref(false)
const maxEventLogEntries = 50

// Event handler map for polling-based event distribution - now supports multiple handlers per event type
const eventHandlers = new Map<string, EventCallback[]>()

// Polling state
let pollTimeoutId: number | null = null
let isPolling = false

function logEvent(type: string, data: any) {
  const timestamp = new Date().toISOString()

  eventLog.value.unshift({
    time: timestamp,
    type: type,
    data: JSON.stringify(data, null, 2)
  })

  // Keep only last 50 events
  if (eventLog.value.length > maxEventLogEntries) {
    eventLog.value = eventLog.value.slice(0, maxEventLogEntries)
  }
}

// Register event handler for polling-based distribution
// Returns unregister function for cleanup
function registerEventListener(eventName: string, callback: EventCallback): () => void {
  // Initialize array for this event type if not exists
  if (!eventHandlers.has(eventName)) {
    eventHandlers.set(eventName, [])
  }

  // Add callback to array
  const handlers = eventHandlers.get(eventName)!
  handlers.push(callback)

  // Return unregister function
  return () => {
    const idx = handlers.indexOf(callback)
    if (idx !== -1) {
      handlers.splice(idx, 1)
    }
  }
}

// Provide registration function to child components
provide('registerEventListener', registerEventListener)

// Poll for events from backend (recursive with setTimeout to avoid overlapping calls)
async function pollEvents() {
  // Prevent overlapping calls
  if (isPolling) {
    return
  }

  isPolling = true

  try {
    const events: BackendEvent[] = await PollEvents()

    if (events && events.length > 0) {
      for (const event of events) {
        // Log to event log
        logEvent(event.source, event.data)

        // Call all registered handlers for this event type
        const handlers = eventHandlers.get(event.source)
        if (handlers && handlers.length > 0) {
          handlers.forEach(handler => handler(event.data))
        }
      }
    }
  } catch (error) {
    console.error('[pollEvents] Error polling events:', error)
  } finally {
    isPolling = false

    // Schedule next poll only after current poll completes (recursive setTimeout)
    if (pollTimeoutId !== null) {
      pollTimeoutId = window.setTimeout(pollEvents, 1000) // Poll every 1 second
    }
  }
}

// Start polling
function startPolling() {
  if (pollTimeoutId !== null) {
    return // Already polling
  }

  pollTimeoutId = 1 // Set to non-null to enable polling
  // Delay first poll to let UI initialize
  setTimeout(() => pollEvents(), 1000)
}

// Stop polling
function stopPolling() {
  if (pollTimeoutId !== null) {
    clearTimeout(pollTimeoutId)
    pollTimeoutId = null
  }
}

// Generate consistent color for event type based on hash
function getEventColor(eventType: string): string {
  let hash = 0
  for (let i = 0; i < eventType.length; i++) {
    hash = eventType.charCodeAt(i) + ((hash << 5) - hash)
  }

  const colors = [
    'text-green-400',
    'text-blue-400',
    'text-purple-400',
    'text-yellow-400',
    'text-pink-400',
    'text-cyan-400',
    'text-orange-400',
    'text-red-400'
  ]

  return colors[Math.abs(hash) % colors.length]
}

const statusText = computed(() => {
  if (!serverStore.isRunning) return 'Stopped'

  const config = serverStore.config
  if (config?.https_enabled) {
    return `Running on :${serverStore.port} (HTTP) :${config.https_port || 8443} (HTTPS)`
  }
  return `Running on :${serverStore.port}`
})

// Store unregister functions for cleanup
const unregisterFunctions = ref<Array<() => void>>([])

// Listen for container progress events
onMounted(async () => {
  // Register event listeners (all events are automatically logged)

  // System test event - to verify event bridge is working
  unregisterFunctions.value.push(
    registerEventListener('system:test', (event: any) => {
      // Test event received - silently processed
    })
  )

  // Container progress - needs special handling for dialog
  unregisterFunctions.value.push(
    registerEventListener('ctr:progress', (event: any) => {
      if (progressDialogRef.value) {
        // Process any pending events first
        while (pendingProgressEvents.value.length > 0) {
          const pendingEvent = pendingProgressEvents.value.shift()
          progressDialogRef.value.updateProgress(pendingEvent)
        }
        // Process current event
        progressDialogRef.value.updateProgress(event)
      } else {
        pendingProgressEvents.value.push(event)
      }
    })
  )

  // Container status - update store
  unregisterFunctions.value.push(
    registerEventListener('ctr:status', (data: any) => {
      if (data.endpoint_id) {
        const status = new models.ContainerStatus({
          endpoint_id: data.endpoint_id,
          running: data.running,
          status: data.status,
          gone: data.gone,
          last_check: data.last_check  // Already a string (ISO8601/RFC3339 format)
        })
        serverStore.containerStatus.set(data.endpoint_id, status)
      }
    })
  )

  // Container stats - update store
  unregisterFunctions.value.push(
    registerEventListener('ctr:stats', (data: any) => {
      if (data.endpoint_id) {
        const stats = new models.ContainerStats({
          endpoint_id: data.endpoint_id,
          cpu_percent: data.cpu_percent,
          memory_usage_mb: data.memory_usage_mb,
          memory_limit_mb: data.memory_limit_mb,
          memory_percent: data.memory_percent,
          network_rx_bytes: data.network_rx_bytes,
          network_tx_bytes: data.network_tx_bytes,
          block_read_bytes: data.block_read_bytes,
          block_write_bytes: data.block_write_bytes,
          pids: data.pids,
          last_check: data.last_check  // Already a string (ISO8601/RFC3339 format)
        })
        serverStore.containerStats.set(data.endpoint_id, stats)
      }
    })
  )

  // If server is already running when component mounts, trigger container startup
  if (serverStore.isRunning) {
    try {
      await StartContainers()
    } catch (error) {
      console.error('Failed to call StartContainers():', error)
    }
  }

  // Start event polling
  startPolling()
})

// Clean up polling and event handlers on unmount
onUnmounted(() => {
  stopPolling()

  // Unregister all event listeners
  unregisterFunctions.value.forEach(unregister => unregister())
  unregisterFunctions.value = []
})

// Watch for dialog ref to become available and process pending events
watch(progressDialogRef, (newRef) => {
  if (newRef && pendingProgressEvents.value.length > 0) {
    while (pendingProgressEvents.value.length > 0) {
      const event = pendingProgressEvents.value.shift()
      newRef.updateProgress(event)
    }
  }
})

async function toggleServer() {
  isLoading.value = true
  errorMessage.value = ''

  try {
    if (serverStore.isRunning) {
      await serverStore.stopServer()
    } else {
      // Check for container endpoints and show progress dialog
      const containerEndpoints = serverStore.endpoints.filter(e => e.type === 'container' && e.enabled)
      if (containerEndpoints.length > 0) {
        progressEndpointName.value = containerEndpoints[0].name
        showProgressDialog.value = true
        await nextTick()
      }

      await serverStore.startServer(portInput.value)

      // Now that server is running and dialog is ready, start containers
      if (containerEndpoints.length > 0) {
        // Give extra time for event listeners to be fully registered
        await new Promise(resolve => setTimeout(resolve, 500))
        try {
          await StartContainers()
        } catch (error) {
          console.error('[HeaderBar] Failed to start containers:', error)
          errorMessage.value = 'Server started but failed to start containers: ' + String(error)
        }
      }
    }
  } catch (error) {
    errorMessage.value = String(error)
  } finally {
    isLoading.value = false
  }
}

function handleProgressClose() {
  showProgressDialog.value = false
  pendingProgressEvents.value = [] // Clear any pending events
}

async function handleProgressCancel() {
  // Stop the server to cancel container startup
  try {
    await serverStore.stopServer()
  } catch (error) {
    console.error('Failed to cancel container startup:', error)
  }
  showProgressDialog.value = false
  pendingProgressEvents.value = [] // Clear any pending events
}

async function handleSaveConfig() {
  try {
    await SaveConfig()
  } catch (error) {
    errorMessage.value = String(error)
  }
}

async function handleLoadConfig() {
  try {
    const config = await LoadConfig()
    if (config) {
      portInput.value = config.port
      // Refresh items from backend after config load
      await serverStore.refreshItems()
    }
  } catch (error) {
    errorMessage.value = String(error)
  }
}

function handleImportOpenAPI() {
  showImportDialog.value = true
}

async function handleAppend() {
  showImportDialog.value = false
  try {
    await serverStore.importOpenAPISpec(true) // append mode
  } catch (error) {
    errorMessage.value = String(error)
  }
}

async function handleReplace() {
  showImportDialog.value = false
  try {
    await serverStore.importOpenAPISpec(false) // replace mode
  } catch (error) {
    errorMessage.value = String(error)
  }
}

function handleCancelImport() {
  showImportDialog.value = false
}

function openServerConfig(tab: 'http' | 'https' = 'http') {
  serverConfigDialogTab.value = tab
  showServerConfigDialog.value = true
}

async function handleServerConfigApply() {
  showServerConfigDialog.value = false

  try {
    // Get configuration from dialog components
    const httpTabRef = serverConfigDialogRef.value?.httpTab
    const httpsTabRef = serverConfigDialogRef.value?.httpsTab
    const corsTabRef = serverConfigDialogRef.value?.corsTab

    // Update HTTP port
    if (httpTabRef?.getPort) {
      const newPort = httpTabRef.getPort()
      if (newPort !== portInput.value) {
        portInput.value = newPort
      }
    }

    // Get HTTP redirect setting from HTTP tab
    const httpRedirect = httpTabRef?.getRedirect ? httpTabRef.getRedirect() : false

    // Get HTTP/2 setting from HTTP tab
    const http2Enabled = httpTabRef?.getHTTP2Enabled ? httpTabRef.getHTTP2Enabled() : false

    // Update HTTPS configuration
    if (httpsTabRef?.getConfig) {
      const httpsConfig = httpsTabRef.getConfig()
      await SetHTTPSConfig(
        httpsConfig.enabled,
        httpsConfig.port,
        httpRedirect  // Now from HTTP tab instead of HTTPS tab
      )
      await SetCertMode(
        httpsConfig.certMode,
        httpsConfig.certPaths,
        httpsConfig.certNames || []
      )
    }

    // Update CORS configuration
    if (corsTabRef?.getConfig) {
      const corsConfig = corsTabRef.getConfig()
      const corsConfigModel = new models.CORSConfig(corsConfig)
      await SetCORSConfig(corsConfigModel)
    }

    // Update HTTP/2 setting
    await SetHTTP2Enabled(http2Enabled)

    // Refresh config from backend to get updated values
    await serverStore.refreshConfig()
  } catch (error) {
    errorMessage.value = String(error)
  }
}

function handleServerConfigClose() {
  showServerConfigDialog.value = false
}
</script>

<template>
  <header class="h-14 bg-gray-800 border-b border-gray-700 px-4 flex items-center justify-between flex-shrink-0">
    <!-- Left: Logo, Title, and Config Actions -->
    <div class="flex items-center gap-3">
      <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
        <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2" />
        </svg>
      </div>
      <h1 class="text-lg font-semibold text-white">Mockelot</h1>

      <!-- Load/Save Config Icons -->
      <div class="flex items-center gap-1 ml-4">
        <button
          @click="handleLoadConfig"
          class="p-2 bg-gray-700 hover:bg-gray-600 rounded text-gray-300 hover:text-white transition-colors"
          title="Load Configuration"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
          </svg>
        </button>
        <button
          @click="handleSaveConfig"
          class="p-2 bg-gray-700 hover:bg-gray-600 rounded text-gray-300 hover:text-white transition-colors"
          title="Save Configuration"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4" />
          </svg>
        </button>
      </div>

      <!-- Separator -->
      <div class="h-6 w-px bg-gray-600 ml-4"></div>

      <!-- Import OpenAPI Icon -->
      <button
        @click="handleImportOpenAPI"
        class="p-2 bg-gray-700 hover:bg-gray-600 rounded text-gray-300 hover:text-white transition-colors ml-4"
        title="Import OpenAPI Specification"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
          <path d="M12 0L2.524 6v12L12 24l9.476-6V6L12 0zm0 2.5l7.476 4.75v9.5L12 21.5l-7.476-4.75v-9.5L12 2.5z"/>
          <path d="M12 6L7 9v6l5 3 5-3V9l-5-3zm0 2.5L15 10v4l-3 1.8L9 14v-4l3-1.5z"/>
        </svg>
      </button>
    </div>

    <!-- Center: Status -->
    <div class="flex items-center gap-2">
      <div
        :class="[
          'w-3 h-3 rounded-full',
          serverStore.isRunning ? 'bg-green-500 animate-pulse' : 'bg-gray-500'
        ]"
      />
      <span class="text-sm text-gray-400">
        {{ statusText }}
      </span>
      <span v-if="errorMessage" class="text-sm text-red-400 ml-2">{{ errorMessage }}</span>
    </div>

    <!-- Right: Server Controls and Config -->
    <div class="flex items-center gap-2">
      <!-- Start Server Icon -->
      <button
        @click="toggleServer"
        v-if="!serverStore.isRunning"
        :disabled="isLoading"
        class="p-2 bg-green-600 hover:bg-green-700 rounded text-white transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        title="Start Server"
      >
        <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
          <path d="M6.3 2.841A1.5 1.5 0 004 4.11V15.89a1.5 1.5 0 002.3 1.269l9.344-5.89a1.5 1.5 0 000-2.538L6.3 2.84z" />
        </svg>
      </button>

      <!-- Stop Server Icon -->
      <button
        @click="toggleServer"
        v-else
        :disabled="isLoading"
        class="p-2 bg-red-600 hover:bg-red-700 rounded text-white transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        title="Stop Server"
      >
        <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd" />
        </svg>
      </button>

      <!-- Server Configuration Gear Icon -->
      <button
        @click="openServerConfig('http')"
        class="p-2 bg-gray-700 hover:bg-gray-600 rounded text-gray-300 hover:text-white transition-colors"
        title="Server Configuration (HTTP, HTTPS)"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </button>

      <!-- Event Log Toggle -->
      <button
        @click="showEventLog = !showEventLog"
        :class="[
          'relative p-2 rounded text-gray-300 hover:text-white transition-colors',
          showEventLog ? 'bg-blue-600 hover:bg-blue-700' : 'bg-gray-700 hover:bg-gray-600'
        ]"
        title="Toggle Event Log"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        <span v-if="eventLog.length > 0" class="absolute -top-1 -right-1 bg-red-500 text-white text-xs rounded-full w-5 h-5 flex items-center justify-center">
          {{ eventLog.length > 9 ? '9+' : eventLog.length }}
        </span>
      </button>
    </div>

    <!-- Import OpenAPI Dialog -->
    <ConfirmDialog
      :show="showImportDialog"
      title="Import OpenAPI Specification"
      message="How would you like to import the OpenAPI specification?"
      primary-text="Append"
      secondary-text="Replace"
      cancel-text="Cancel"
      @primary="handleAppend"
      @secondary="handleReplace"
      @cancel="handleCancelImport"
    />

    <!-- Server Configuration Dialog -->
    <ServerConfigDialog
      ref="serverConfigDialogRef"
      :show="showServerConfigDialog"
      :initial-tab="serverConfigDialogTab"
      @close="handleServerConfigClose"
      @apply="handleServerConfigApply"
    />

    <!-- Container Progress Dialog -->
    <ContainerProgressDialog
      ref="progressDialogRef"
      :show="showProgressDialog"
      :endpoint-name="progressEndpointName"
      @close="handleProgressClose"
      @cancel="handleProgressCancel"
    />

    <!-- Event Log Panel -->
    <div v-if="showEventLog" class="fixed bottom-0 left-0 right-0 bg-gray-800 border-t border-gray-700 max-h-96 overflow-auto z-50">
      <div class="p-4">
        <div class="flex items-center justify-between mb-2">
          <h3 class="text-sm font-semibold text-white">Event Log (Last {{ eventLog.length }} events)</h3>
          <button
            @click="eventLog = []"
            class="text-xs text-gray-400 hover:text-white"
          >
            Clear
          </button>
        </div>
        <div v-if="eventLog.length === 0" class="text-sm text-gray-500 italic">
          No events received yet...
        </div>
        <div v-else class="space-y-2">
          <div
            v-for="(event, index) in eventLog"
            :key="index"
            class="bg-gray-900 p-2 rounded text-xs font-mono"
          >
            <div class="flex items-center justify-between mb-1">
              <span :class="['font-semibold', getEventColor(event.type)]">
                {{ event.type }}
              </span>
              <span class="text-gray-500">{{ event.time }}</span>
            </div>
            <pre class="text-gray-300 whitespace-pre-wrap">{{ event.data }}</pre>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>
