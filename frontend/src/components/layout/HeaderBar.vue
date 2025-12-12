<script lang="ts" setup>
import { ref, computed } from 'vue'
import { useServerStore } from '../../stores/server'
import { SaveConfig, LoadConfig, SetHTTPSConfig, SetCertMode, SetCORSConfig, SetHTTP2Enabled } from '../../../wailsjs/go/main/App'
import { models } from '../../types/models'
import ConfirmDialog from '../dialogs/ConfirmDialog.vue'
import ServerConfigDialog from '../dialogs/ServerConfigDialog.vue'

const serverStore = useServerStore()
const portInput = ref(8080)
const isLoading = ref(false)
const errorMessage = ref('')
const showImportDialog = ref(false)
const showServerConfigDialog = ref(false)
const serverConfigDialogTab = ref<'http' | 'https'>('http')
const serverConfigDialogRef = ref<InstanceType<typeof ServerConfigDialog> | null>(null)

const statusText = computed(() => {
  if (!serverStore.isRunning) return 'Stopped'

  const config = serverStore.config
  if (config?.https_enabled) {
    return `Running on :${serverStore.port} (HTTP) :${config.https_port || 8443} (HTTPS)`
  }
  return `Running on :${serverStore.port}`
})

async function toggleServer() {
  isLoading.value = true
  errorMessage.value = ''

  try {
    if (serverStore.isRunning) {
      await serverStore.stopServer()
    } else {
      await serverStore.startServer(portInput.value)
    }
  } catch (error) {
    errorMessage.value = String(error)
  } finally {
    isLoading.value = false
  }
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
  </header>
</template>
