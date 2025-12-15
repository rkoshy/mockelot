<script lang="ts" setup>
import { ref, computed, onMounted } from 'vue'
import { GetRecentFiles, LoadConfigFromPath, RemoveRecentFile, LoadConfig } from '../../../wailsjs/go/main/App'

interface RecentFile {
  path: string
  last_accessed: string
  exists: boolean
}

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  close: []
  loaded: []
}>()

// State
const recentFiles = ref<RecentFile[]>([])
const loading = ref(false)
const error = ref('')

// Load recent files when dialog is shown
async function loadRecentFiles() {
  loading.value = true
  error.value = ''

  try {
    const files = await GetRecentFiles()
    recentFiles.value = files || []
  } catch (err) {
    console.error('Failed to load recent files:', err)
    error.value = String(err)
    recentFiles.value = []
  } finally {
    loading.value = false
  }
}

// Load file from path
async function loadFile(file: RecentFile) {
  if (!file.exists) {
    return // Don't load missing files
  }

  loading.value = true
  error.value = ''

  try {
    await LoadConfigFromPath(file.path)
    emit('loaded')
    emit('close')
  } catch (err) {
    console.error('Failed to load file:', err)
    error.value = `Failed to load ${getFileName(file.path)}: ${err}`
  } finally {
    loading.value = false
  }
}

// Remove file from recent list
async function removeFile(file: RecentFile, event: Event) {
  event.stopPropagation() // Prevent panel click

  try {
    await RemoveRecentFile(file.path)
    await loadRecentFiles() // Refresh list
  } catch (err) {
    console.error('Failed to remove file:', err)
    error.value = `Failed to remove file: ${err}`
  }
}

// Load from file picker
async function loadFromFile() {
  loading.value = true
  error.value = ''

  try {
    await LoadConfig()
    await loadRecentFiles() // Refresh list after successful load
    emit('loaded')
    emit('close')
  } catch (err) {
    if (err) { // Only show error if not cancelled
      console.error('Failed to load file:', err)
      error.value = `Failed to load file: ${err}`
    }
  } finally {
    loading.value = false
  }
}

// Extract file name from path
function getFileName(path: string): string {
  return path.split(/[\\/]/).pop() || path
}

// Format timestamp
function formatTimestamp(timestamp: string): string {
  try {
    const date = new Date(timestamp)
    return date.toLocaleString()
  } catch {
    return timestamp
  }
}

// Watch for dialog show
onMounted(() => {
  if (props.show) {
    loadRecentFiles()
  }
})

// Reload when dialog is shown
const showDialog = computed(() => props.show)
let previousShow = false
const unwatchShow = () => {
  const newShow = showDialog.value
  if (newShow && !previousShow) {
    loadRecentFiles()
  }
  previousShow = newShow
}

// Manual watcher
setInterval(unwatchShow, 100)

// Handle Escape key
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    emit('close')
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  return () => {
    window.removeEventListener('keydown', handleKeydown)
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-80"
        @click.self="emit('close')"
      >
        <!-- Dialog - 80% of screen -->
        <div class="bg-gray-800 rounded-lg shadow-xl w-[80vw] h-[80vh] mx-4 border border-gray-700 flex flex-col">
          <!-- Header -->
          <div class="px-6 py-4 border-b border-gray-700 flex items-center justify-between flex-shrink-0">
            <div>
              <h3 class="text-xl font-semibold text-white">Load Configuration</h3>
              <p class="text-sm text-gray-400 mt-1">Select a recent file or load from disk</p>
            </div>
            <div class="flex items-center gap-3">
              <!-- Load From File button -->
              <button
                @click="loadFromFile"
                :disabled="loading"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded transition-colors text-sm"
              >
                Load From File
              </button>
              <!-- Close button -->
              <button
                @click="emit('close')"
                class="p-1 hover:bg-gray-700 rounded transition-colors text-gray-400 hover:text-white"
              >
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>

          <!-- Error Message -->
          <div v-if="error" class="mx-6 mt-4 p-3 bg-red-900/30 border border-red-700 rounded text-red-400 text-sm">
            {{ error }}
          </div>

          <!-- Body - Scrollable with 3-column grid -->
          <div class="flex-1 overflow-y-auto px-6 py-6">
            <!-- Loading State -->
            <div v-if="loading" class="flex items-center justify-center h-full">
              <div class="text-center">
                <svg class="animate-spin h-8 w-8 text-blue-400 mx-auto mb-3" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <p class="text-gray-400">Loading recent files...</p>
              </div>
            </div>

            <!-- Empty State -->
            <div v-else-if="!loading && recentFiles.length === 0" class="flex items-center justify-center h-full">
              <div class="text-center">
                <svg class="w-16 h-16 text-gray-600 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
                <p class="text-gray-400 text-lg mb-2">No recent files</p>
                <p class="text-gray-500 text-sm">Load a configuration file to get started</p>
                <button
                  @click="loadFromFile"
                  class="mt-4 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors text-sm"
                >
                  Load From File
                </button>
              </div>
            </div>

            <!-- File Grid - 3 columns -->
            <div v-else class="grid grid-cols-3 gap-4">
              <div
                v-for="file in recentFiles"
                :key="file.path"
                @click="loadFile(file)"
                :class="[
                  'relative border-2 rounded-lg p-4 transition-all cursor-pointer',
                  file.exists
                    ? 'border-gray-700 bg-gray-900/50 hover:border-blue-500 hover:bg-gray-900'
                    : 'border-red-700 bg-red-900/20 cursor-not-allowed'
                ]"
              >
                <!-- File Info -->
                <div class="mb-3">
                  <h4 :class="[
                    'font-medium text-sm mb-1 truncate',
                    file.exists ? 'text-white' : 'text-red-400'
                  ]">
                    {{ getFileName(file.path) }}
                  </h4>
                  <p class="text-xs text-gray-500 truncate mb-1" :title="file.path">
                    {{ file.path }}
                  </p>
                  <p class="text-xs text-gray-500">
                    {{ formatTimestamp(file.last_accessed) }}
                  </p>
                  <p v-if="!file.exists" class="text-xs text-red-400 mt-1">
                    File not found
                  </p>
                </div>

                <!-- Bottom Section: LOAD button and Garbage can -->
                <div class="flex items-center justify-between">
                  <!-- LOAD button (only if file exists) -->
                  <button
                    v-if="file.exists"
                    @click.stop="loadFile(file)"
                    class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors text-xs"
                  >
                    LOAD
                  </button>
                  <div v-else></div>

                  <!-- Garbage can icon -->
                  <button
                    @click="removeFile(file, $event)"
                    class="p-1 hover:bg-red-900/50 rounded transition-colors text-gray-400 hover:text-red-400"
                    title="Remove from recent files"
                  >
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-gray-700 flex justify-end flex-shrink-0">
            <button
              @click="emit('close')"
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

.modal-enter-active > div,
.modal-leave-active > div {
  transition: transform 0.2s ease;
}

.modal-enter-from > div,
.modal-leave-to > div {
  transform: scale(0.95);
}
</style>
