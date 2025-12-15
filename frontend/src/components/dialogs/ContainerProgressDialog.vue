<script lang="ts" setup>
import { ref, watch } from 'vue'

interface ProgressEvent {
  endpoint_id: string
  stage: string      // "pulling", "creating", "starting", "ready", "error"
  message: string
  progress: number   // 0-100
}

const props = defineProps<{
  show: boolean
  endpointName: string
}>()

const emit = defineEmits<{
  close: []
  cancel: []
}>()

const currentStage = ref<string>('pulling')
const message = ref<string>('Initializing...')
const progress = ref<number>(0)
const hasError = ref<boolean>(false)
const errorMessage = ref<string>('')

// Watch for show prop changes to reset state
watch(() => props.show, (newVal) => {
  if (newVal) {
    currentStage.value = 'pulling'
    message.value = 'Initializing...'
    progress.value = 0
    hasError.value = false
    errorMessage.value = ''
  }
})

// Handle progress updates (called by parent component)
function updateProgress(event: ProgressEvent) {
  currentStage.value = event.stage
  message.value = event.message
  progress.value = event.progress

  if (event.stage === 'error') {
    hasError.value = true
    errorMessage.value = event.message
  } else if (event.stage === 'ready') {
    // Auto-close after 1 second when ready
    setTimeout(() => {
      emit('close')
    }, 1000)
  }
}

// Stage display names
function getStageLabel(stage: string): string {
  switch (stage) {
    case 'pulling': return 'Pulling Image'
    case 'creating': return 'Creating Container'
    case 'starting': return 'Starting Container'
    case 'ready': return 'Ready'
    case 'error': return 'Error'
    default: return 'Processing'
  }
}

// Stage icons
function getStageIcon(stage: string): string {
  switch (stage) {
    case 'ready': return '✓'
    case 'error': return '✗'
    default: return '⋯'
  }
}

// Helper to get progress percentage for each stage
function getStageProgress(stage: string): number {
  switch (stage) {
    case 'pulling': return 25
    case 'creating': return 50
    case 'starting': return 75
    case 'ready': return 100
    default: return 0
  }
}

defineExpose({ updateProgress })
</script>

<template>
  <!-- DIAGNOSTIC: Using inline style for transparency to ensure it works -->
  <Transition name="modal">
    <div
      v-if="show"
      class="fixed inset-0 z-50 flex items-center justify-center"
      style="background-color: rgba(0, 0, 0, 0.3)"
    >
        <div class="bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4 border border-gray-700">
          <!-- Header -->
          <div class="px-6 py-4 border-b border-gray-700">
            <h3 class="text-lg font-semibold text-white">Starting Container</h3>
            <p class="text-sm text-gray-400 mt-1">{{ endpointName }}</p>
          </div>

          <!-- Body -->
          <div class="px-6 py-6 space-y-4">
            <!-- Progress Stages -->
            <div class="space-y-3">
              <div
                v-for="stage in ['pulling', 'creating', 'starting', 'ready']"
                :key="stage"
                class="flex items-center gap-3"
              >
                <div
                  :class="[
                    'w-6 h-6 rounded-full flex items-center justify-center text-sm',
                    currentStage === stage ? 'bg-blue-600 text-white animate-pulse' :
                    progress >= getStageProgress(stage) ? 'bg-green-600 text-white' :
                    'bg-gray-600 text-gray-400'
                  ]"
                >
                  <span v-if="progress >= getStageProgress(stage)">{{ getStageIcon(stage) }}</span>
                  <span v-else>{{ getStageIcon('pending') }}</span>
                </div>
                <span
                  :class="[
                    'text-sm',
                    currentStage === stage ? 'text-white font-medium' :
                    progress >= getStageProgress(stage) ? 'text-green-400' :
                    'text-gray-400'
                  ]"
                >
                  {{ getStageLabel(stage) }}
                </span>
              </div>
            </div>

            <!-- Current Message -->
            <div class="mt-4 p-3 bg-gray-700/50 rounded">
              <p class="text-sm text-gray-300">{{ message }}</p>
            </div>

            <!-- Progress Bar -->
            <div class="w-full bg-gray-700 rounded-full h-2">
              <div
                :class="[
                  'h-2 rounded-full transition-all duration-300',
                  hasError ? 'bg-red-500' : 'bg-blue-600'
                ]"
                :style="{ width: `${progress}%` }"
              ></div>
            </div>

            <!-- Error Message -->
            <div v-if="hasError" class="p-3 bg-red-900/30 border border-red-700 rounded">
              <p class="text-sm text-red-400">{{ errorMessage }}</p>
            </div>
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-gray-700 flex justify-end gap-2">
            <!-- Cancel button during active stages -->
            <button
              v-if="!hasError && currentStage !== 'ready'"
              @click="emit('cancel')"
              class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded transition-colors"
            >
              Cancel
            </button>
            <!-- Close button after completion or error -->
            <button
              v-if="hasError || currentStage === 'ready'"
              @click="emit('close')"
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </Transition>
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

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.animate-pulse {
  animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}
</style>
