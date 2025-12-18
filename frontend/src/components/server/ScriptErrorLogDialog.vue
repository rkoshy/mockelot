<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
import { useServerStore } from '../../stores/server'
import { GetScriptErrors, ClearScriptErrors } from '../../../wailsjs/go/main/App'

const props = defineProps<{
  visible: boolean
  responseId: string
}>()

export interface ScriptErrorInfo {
  error: string
  timestamp: string
  method: string
  path: string
}

const emit = defineEmits<{
  close: []
  goToError: [error: ScriptErrorInfo]
}>()

const serverStore = useServerStore()

// Get errors from store
const errors = computed(() => {
  return serverStore.getScriptErrors(props.responseId)
})

// Clear all errors for this response
async function clearErrors() {
  try {
    await ClearScriptErrors(props.responseId)
    emit('close')
  } catch (error) {
    console.error('Failed to clear script errors:', error)
  }
}

function handleClose() {
  emit('close')
}

// Close on Escape key
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.visible) {
    handleClose()
  }
}

watch(() => props.visible, (newVal) => {
  if (newVal) {
    window.addEventListener('keydown', handleKeydown)
  } else {
    window.removeEventListener('keydown', handleKeydown)
  }
})

// Format timestamp
function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleString()
}
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="visible"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
        @click.self="handleClose"
      >
        <div class="bg-gray-800 rounded-lg shadow-xl max-w-4xl w-full mx-4 border border-gray-700 max-h-[80vh] flex flex-col">
          <!-- Header -->
          <div class="px-6 py-4 border-b border-gray-700 flex items-center justify-between">
            <div>
              <h3 class="text-lg font-semibold text-white">Script Execution Errors</h3>
              <p class="text-sm text-gray-400 mt-1">
                {{ errors.length }} error{{ errors.length !== 1 ? 's' : '' }} logged for this response
              </p>
            </div>
            <button
              @click="handleClose"
              class="text-gray-400 hover:text-gray-300 transition-colors"
              title="Close"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- Body - Scrollable Error List -->
          <div class="flex-1 overflow-y-auto px-6 py-4">
            <div v-if="errors.length === 0" class="text-center text-gray-400 py-8">
              No errors logged
            </div>
            <div v-else class="space-y-4">
              <div
                v-for="(error, index) in errors"
                :key="index"
                class="bg-gray-900 border border-red-900 rounded-lg p-4"
              >
                <!-- Error Header -->
                <div class="flex items-start justify-between mb-2">
                  <div class="flex items-center gap-2">
                    <div class="bg-red-600 text-white rounded px-2 py-0.5 text-xs font-bold">
                      ERROR
                    </div>
                    <span class="text-sm text-gray-400">{{ formatTimestamp(error.timestamp) }}</span>
                  </div>
                  <div class="flex items-center gap-2 text-xs text-gray-500">
                    <span class="font-mono">{{ error.method }}</span>
                    <span>→</span>
                    <span class="font-mono">{{ error.path }}</span>
                  </div>
                </div>

                <!-- Error Message -->
                <div class="bg-gray-950 border border-gray-800 rounded p-3 mt-2">
                  <pre class="text-sm text-red-400 font-mono whitespace-pre-wrap break-words">{{ error.error }}</pre>
                </div>

                <!-- Go To Error Button -->
                <div class="mt-2 flex justify-end">
                  <button
                    @click="emit('goToError', error)"
                    class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-xs rounded transition-colors flex items-center gap-1"
                  >
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                    </svg>
                    Go To Error
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-gray-700 flex justify-between items-center">
            <button
              @click="clearErrors"
              :disabled="errors.length === 0"
              :class="[
                'px-4 py-2 rounded text-sm font-medium transition-colors',
                errors.length > 0
                  ? 'bg-red-600 hover:bg-red-700 text-white'
                  : 'bg-gray-700 text-gray-500 cursor-not-allowed'
              ]"
            >
              Clear All Errors
            </button>
            <button
              @click="handleClose"
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-active .bg-gray-800,
.modal-leave-active .bg-gray-800 {
  transition: transform 0.2s ease;
}

.modal-enter-from .bg-gray-800,
.modal-leave-to .bg-gray-800 {
  transform: scale(0.95);
}
</style>
