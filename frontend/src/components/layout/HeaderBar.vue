<script lang="ts" setup>
import { ref } from 'vue'
import { useServerStore } from '../../stores/server'
import { SaveConfig, LoadConfig } from '../../../wailsjs/go/main/App'
import ConfirmDialog from '../dialogs/ConfirmDialog.vue'

const serverStore = useServerStore()
const portInput = ref(8080)
const isLoading = ref(false)
const errorMessage = ref('')
const showImportDialog = ref(false)

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
</script>

<template>
  <header class="h-14 bg-gray-800 border-b border-gray-700 px-4 flex items-center justify-between flex-shrink-0">
    <!-- Left: Logo and Title -->
    <div class="flex items-center gap-3">
      <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
        <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2" />
        </svg>
      </div>
      <h1 class="text-lg font-semibold text-white">MockAgainTool</h1>
    </div>

    <!-- Center: Server Controls -->
    <div class="flex items-center gap-4">
      <!-- Port Input -->
      <div class="flex items-center gap-2">
        <label class="text-sm text-gray-400">Port:</label>
        <input
          v-model.number="portInput"
          type="number"
          min="1"
          max="65535"
          :disabled="serverStore.isRunning"
          class="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-sm text-white
                 focus:outline-none focus:border-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
        />
      </div>

      <!-- Start/Stop Button -->
      <button
        @click="toggleServer"
        :disabled="isLoading"
        :class="[
          'px-4 py-1.5 rounded font-medium text-sm transition-colors',
          serverStore.isRunning
            ? 'bg-red-600 hover:bg-red-700 text-white'
            : 'bg-green-600 hover:bg-green-700 text-white',
          isLoading && 'opacity-50 cursor-not-allowed'
        ]"
      >
        <span v-if="isLoading">...</span>
        <span v-else>{{ serverStore.isRunning ? 'Stop Server' : 'Start Server' }}</span>
      </button>

      <!-- Status Indicator -->
      <div class="flex items-center gap-2">
        <div
          :class="[
            'w-3 h-3 rounded-full',
            serverStore.isRunning ? 'bg-green-500 animate-pulse' : 'bg-gray-500'
          ]"
        />
        <span class="text-sm text-gray-400">
          {{ serverStore.isRunning ? `Running on :${serverStore.port}` : 'Stopped' }}
        </span>
      </div>

      <!-- Error Message -->
      <span v-if="errorMessage" class="text-sm text-red-400">{{ errorMessage }}</span>
    </div>

    <!-- Right: Config Actions -->
    <div class="flex items-center gap-2">
      <button
        @click="handleImportOpenAPI"
        class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 rounded text-sm text-white transition-colors"
      >
        Import OpenAPI
      </button>
      <button
        @click="handleLoadConfig"
        class="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 rounded text-sm text-gray-300 transition-colors"
      >
        Load Config
      </button>
      <button
        @click="handleSaveConfig"
        class="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 rounded text-sm text-gray-300 transition-colors"
      >
        Save Config
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
  </header>
</template>
