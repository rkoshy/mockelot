<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
import { models } from '../../types/models'
import {
  STATUS_CODES,
  type ResponseMode,
  type ValidationMode,
  type ValidationMatchType
} from '../../types/models'
import ResponseEditorPanel from './ResponseEditorPanel.vue'
import ScriptErrorLogDialog, { type ScriptErrorInfo } from './ScriptErrorLogDialog.vue'
import { useServerStore } from '../../stores/server'

const serverStore = useServerStore()

// Convert STATUS_CODES to combobox options
const statusCodeOptions = computed(() =>
  STATUS_CODES.map(s => ({ value: s.code, label: `${s.code} - ${s.text}` }))
)

const props = defineProps<{
  response: models.MethodResponse
  index: number
}>()

const emit = defineEmits<{
  (e: 'update', response: models.MethodResponse): void
  (e: 'delete'): void
  (e: 'dragstart', event: DragEvent): void
  (e: 'dragover', event: DragEvent): void
  (e: 'drop', event: DragEvent): void
}>()

// Local copy for editing
const localResponse = ref<models.MethodResponse>(new models.MethodResponse({ ...props.response }))

// Current response mode (defaults to 'static')
const currentMode = computed({
  get: () => (localResponse.value.response_mode as ResponseMode) || 'static',
  set: (value: ResponseMode) => {
    localResponse.value.response_mode = value
  }
})

// Request validation computed properties
const validationMode = computed({
  get: () => (localResponse.value.request_validation?.mode as ValidationMode) || 'none',
  set: (value: ValidationMode) => {
    if (!localResponse.value.request_validation) {
      localResponse.value.request_validation = new models.RequestValidation({})
    }
    localResponse.value.request_validation.mode = value
  }
})

const validationMatchType = computed({
  get: () => (localResponse.value.request_validation?.match_type as ValidationMatchType) || 'contains',
  set: (value: ValidationMatchType) => {
    if (!localResponse.value.request_validation) {
      localResponse.value.request_validation = new models.RequestValidation({})
    }
    localResponse.value.request_validation.match_type = value
  }
})

const validationPattern = computed({
  get: () => localResponse.value.request_validation?.pattern || '',
  set: (value: string) => {
    if (!localResponse.value.request_validation) {
      localResponse.value.request_validation = new models.RequestValidation({})
    }
    localResponse.value.request_validation.pattern = value
  }
})

const validationScript = computed({
  get: () => localResponse.value.request_validation?.script || '',
  set: (value: string) => {
    if (!localResponse.value.request_validation) {
      localResponse.value.request_validation = new models.RequestValidation({})
    }
    localResponse.value.request_validation.script = value
  }
})

// Slide-in panel state
const showEditorPanel = ref(false)

// Script error dialog state
const showErrorDialog = ref(false)

// Auto-open script editor flag
const autoOpenScriptEditor = ref(false)

// Current script error info for "Go To Error" navigation
const currentScriptError = ref<ScriptErrorInfo | null>(null)

// "Go To Error" mode - tracks if we need to restore state
const isGoToErrorMode = ref(false)
const savedPanelState = ref<boolean>(false)

// Handle "Go To Error" from error dialog
function handleGoToError(error: ScriptErrorInfo) {
  // Close error dialog
  showErrorDialog.value = false

  // Save current panel state for restoration
  savedPanelState.value = showEditorPanel.value

  // Store error info to pass to editor
  currentScriptError.value = error

  // Enter "Go To Error" mode
  isGoToErrorMode.value = true

  // Open the full editor panel with auto-launch of script editor
  autoOpenScriptEditor.value = true
  showEditorPanel.value = true
}

// Restore state after "Go To Error" modal closes
function restoreStateAfterGoToError() {
  if (!isGoToErrorMode.value) {
    return
  }

  // Close the panel if it wasn't open before
  if (!savedPanelState.value) {
    showEditorPanel.value = false
  }

  // Clear state
  isGoToErrorMode.value = false
  savedPanelState.value = false
  autoOpenScriptEditor.value = false
  currentScriptError.value = null
}

// Check if this response has script errors
const hasErrors = computed(() => {
  if (!props.response.id) {
    return false
  }
  return serverStore.hasScriptErrors(props.response.id)
})

// Get error count
const errorCount = computed(() => {
  if (!props.response.id) return 0
  return serverStore.getScriptErrors(props.response.id).length
})

// Whether response is enabled (defaults to true)
const isEnabled = computed({
  get: () => localResponse.value.enabled !== false,
  set: (value: boolean) => {
    localResponse.value.enabled = value
  }
})

// Content-Type computed property (extracts from headers)
const contentType = computed({
  get: () => localResponse.value.headers?.['Content-Type'] || '',
  set: (value: string) => {
    if (value) {
      localResponse.value.headers = {
        ...localResponse.value.headers,
        'Content-Type': value
      }
    } else {
      const headers = { ...localResponse.value.headers }
      delete headers['Content-Type']
      localResponse.value.headers = headers
    }
  }
})

// Check if response has body
const hasBody = computed(() => !!localResponse.value.body || !!contentType.value)

// Body placeholder based on current mode
const bodyPlaceholder = computed(() => {
  if (currentMode.value === 'template') {
    return '{"userId": "{{.PathParams.id}}", "query": "{{.GetQueryParam \\"q\\"}}"}'
  }
  return contentType.value ? 'Enter response body...' : 'No body (select content type to add one)'
})

// CORS configuration
const handlesOptions = computed(() => localResponse.value.methods.includes('OPTIONS'))

const useGlobalCORS = computed({
  get: () => {
    // If undefined, return true (default: use global CORS)
    return localResponse.value.use_global_cors !== false
  },
  set: (value: boolean) => {
    localResponse.value.use_global_cors = value
  }
})

// Clear content type and body
function clearBody() {
  localResponse.value.body = ''
  const headers = { ...localResponse.value.headers }
  delete headers['Content-Type']
  localResponse.value.headers = headers
}

// Track dirty state
const isDirty = computed(() => {
  return JSON.stringify(localResponse.value) !== JSON.stringify(props.response)
})

// Reset to original
function resetChanges() {
  localResponse.value = new models.MethodResponse({ ...props.response })
}

// Sync with props changes only when not dirty (prevents overwriting user edits)
watch(() => props.response, (newVal, oldVal) => {
  console.log('[ResponseRuleCard] props.response watcher fired:', {
    isDirty: isDirty.value,
    willSync: !isDirty.value,
    newStatusCode: newVal.status_code,
    oldStatusCode: oldVal?.status_code,
    localStatusCode: localResponse.value.status_code
  })
  // Only sync if we don't have unsaved changes
  if (!isDirty.value) {
    console.log('[ResponseRuleCard] Syncing localResponse from props')
    localResponse.value = new models.MethodResponse({ ...newVal })
  } else {
    console.log('[ResponseRuleCard] Skipping sync - changes are dirty')
  }
}, { deep: true })

// Watch localResponse status_code changes
watch(() => localResponse.value.status_code, (newVal, oldVal) => {
  console.log('[ResponseRuleCard] localResponse.status_code changed:', {
    from: oldVal,
    to: newVal,
    isDirty: isDirty.value
  })
})

// Method badge colors
function getMethodColor(method: string): string {
  const colors: Record<string, string> = {
    GET: 'bg-green-600',
    POST: 'bg-blue-600',
    PUT: 'bg-yellow-600',
    DELETE: 'bg-red-600',
    PATCH: 'bg-purple-600',
    OPTIONS: 'bg-gray-600'
  }
  return colors[method] || 'bg-gray-600'
}

// Handle local response updates from ResponseEditorContent
function handleLocalResponseUpdate(updatedResponse: models.MethodResponse) {
  console.log('[ResponseRuleCard] handleLocalResponseUpdate called:', {
    newStatusCode: updatedResponse.status_code,
    oldStatusCode: localResponse.value.status_code
  })
  localResponse.value = updatedResponse
}

// Apply changes
function applyChanges() {
  console.log('[ResponseRuleCard] applyChanges called:', {
    status_code: localResponse.value.status_code,
    status_text: localResponse.value.status_text,
    isDirty: isDirty.value
  })
  emit('update', localResponse.value)
}

// Drag handlers
const isDragging = ref(false)
const cardRef = ref<HTMLElement | null>(null)

function onHandleDragStart(e: DragEvent) {
  isDragging.value = true

  // Create a custom drag image from the entire card
  if (e.dataTransfer && cardRef.value) {
    // Clone the card element for the drag image
    const dragImage = cardRef.value.cloneNode(true) as HTMLElement
    dragImage.style.position = 'absolute'
    dragImage.style.top = '-9999px'
    dragImage.style.left = '-9999px'
    dragImage.style.width = cardRef.value.offsetWidth + 'px'
    dragImage.style.opacity = '0.9'
    document.body.appendChild(dragImage)

    e.dataTransfer.setDragImage(dragImage, 20, 20)

    // Remove the clone after a short delay
    setTimeout(() => {
      document.body.removeChild(dragImage)
    }, 0)
  }

  emit('dragstart', e)
}

function onHandleDragEnd() {
  isDragging.value = false
}

function onDragOver(e: DragEvent) {
  emit('dragover', e)
}

function onDrop(e: DragEvent) {
  emit('drop', e)
}

</script>

<template>
  <div
    ref="cardRef"
    class="rounded-lg border overflow-hidden transition-all"
    :class="[
      isEnabled ? 'bg-gray-800 border-gray-700' : 'bg-gray-900 border-gray-800 opacity-60',
      { 'ring-2 ring-blue-500': showEditorPanel, 'opacity-50': isDragging }
    ]"
    @dragover.prevent="onDragOver"
    @drop="onDrop"
  >
    <!-- Card Header - entire header is draggable, click opens Full Editor -->
    <div
      class="px-3 py-2 cursor-grab active:cursor-grabbing hover:bg-gray-750 transition-colors select-none"
      draggable="true"
      @dragstart="onHandleDragStart"
      @dragend="onHandleDragEnd"
      @click="showEditorPanel = true"
    >
      <!-- Top Row: Priority, Path, Status, Arrow -->
      <div class="flex items-center gap-2">
        <!-- Priority Number -->
        <span class="text-xs text-gray-500 font-mono w-4 flex-shrink-0">{{ index + 1 }}</span>

        <!-- Drag Handle Icon -->
        <div class="text-gray-500 flex-shrink-0">
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path d="M7 2a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 2zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 8zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 14zm6-8a2 2 0 1 0-.001-4.001A2 2 0 0 0 13 6zm0 2a2 2 0 1 0 .001 4.001A2 2 0 0 0 13 8zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 13 14z"/>
          </svg>
        </div>

        <!-- Path Pattern -->
        <span class="text-sm text-gray-200 font-mono truncate flex-1">
          {{ response.path_pattern }}
        </span>

        <!-- Status Code -->
        <span class="text-xs text-gray-400 flex-shrink-0">
          {{ response.status_code }}
        </span>

        <!-- Enable/Disable Toggle -->
        <button
          @click.stop="isEnabled = !isEnabled; applyChanges()"
          class="flex-shrink-0 p-0.5 rounded transition-colors"
          :class="isEnabled ? 'text-green-500 hover:text-green-400' : 'text-gray-600 hover:text-gray-500'"
          :title="isEnabled ? 'Enabled - click to disable' : 'Disabled - click to enable'"
        >
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path v-if="isEnabled" fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
            <path v-else fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
          </svg>
        </button>

        <!-- Script Error Indicator -->
        <button
          v-if="hasErrors"
          @click.stop="showErrorDialog = true"
          class="flex-shrink-0 flex items-center gap-1 px-1.5 py-0.5 bg-red-900/30 hover:bg-red-900/50 rounded transition-colors"
          :title="`${errorCount} script error${errorCount > 1 ? 's' : ''} - click to view`"
        >
          <svg class="w-4 h-4 text-red-500" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9 9a1 1 0 012 0v4a1 1 0 11-2 0V9zm1-4a1 1 0 100 2 1 1 0 000-2z" clip-rule="evenodd" />
          </svg>
          <span class="text-xs font-medium text-red-400">
            {{ errorCount }}
          </span>
        </button>

        <!-- Edit Icon (opens Full Editor) -->
        <svg
          class="w-4 h-4 text-gray-400 flex-shrink-0"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          title="Click to edit"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
        </svg>
      </div>

      <!-- Bottom Row: Method Badges -->
      <div class="flex gap-1 mt-1 ml-10">
        <span
          v-for="method in response.methods"
          :key="method"
          :class="['px-1.5 py-0.5 rounded text-[10px] font-bold text-white', getMethodColor(method)]"
        >
          {{ method }}
        </span>
      </div>
    </div>

    <!-- Slide-in Editor Panel -->
    <ResponseEditorPanel
      :visible="showEditorPanel"
      :local-response="localResponse"
      :current-mode="currentMode"
      :validation-mode="validationMode"
      :validation-match-type="validationMatchType"
      :validation-pattern="validationPattern"
      :validation-script="validationScript"
      :content-type="contentType"
      :has-body="hasBody"
      :body-placeholder="bodyPlaceholder"
      :handles-options="handlesOptions"
      :use-global-c-o-r-s="useGlobalCORS"
      :auto-open-script-editor="autoOpenScriptEditor"
      :script-error="currentScriptError"
      :is-go-to-error-mode="isGoToErrorMode"
      :is-system-endpoint="serverStore.currentEndpoint?.is_system || false"
      @save="localResponse = $event; applyChanges(); showEditorPanel = false; autoOpenScriptEditor = false; currentScriptError = null; isGoToErrorMode = false"
      @close="isGoToErrorMode ? restoreStateAfterGoToError() : (showEditorPanel = false, autoOpenScriptEditor = false, currentScriptError = null)"
      @delete="showEditorPanel = false; emit('delete')"
      @script-modal-closed="restoreStateAfterGoToError"
    />

    <!-- Script Error Log Dialog -->
    <ScriptErrorLogDialog
      v-if="response.id"
      :visible="showErrorDialog"
      :response-id="response.id"
      @close="showErrorDialog = false"
      @go-to-error="handleGoToError"
    />
  </div>
</template>
