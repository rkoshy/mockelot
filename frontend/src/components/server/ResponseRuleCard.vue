<script lang="ts" setup>
import { ref, watch, computed, onUnmounted } from 'vue'
import { models } from '../../types/models'
import {
  HTTP_METHODS,
  STATUS_CODES,
  RESPONSE_MODES,
  RESPONSE_MODE_LABELS,
  VALIDATION_MODES,
  VALIDATION_MODE_LABELS,
  VALIDATION_MATCH_TYPES,
  VALIDATION_MATCH_TYPE_LABELS,
  type ResponseMode,
  type ValidationMode,
  type ValidationMatchType
} from '../../types/models'
import BodyEditorModal from '../shared/BodyEditorModal.vue'
import ContentTypeSelector from '../shared/ContentTypeSelector.vue'
import ComboBox from '../shared/ComboBox.vue'
import ScriptEditorModal from '../shared/ScriptEditorModal.vue'
import ResponseEditorContent from './ResponseEditorContent.vue'
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
  isExpanded: boolean
  index: number
}>()

const emit = defineEmits<{
  (e: 'toggle'): void
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

// Active tab for editor content
const activeTab = ref<'request' | 'response'>('request')

// Force response tab for system endpoints (like Rejections)
watch(() => serverStore.currentEndpoint?.is_system, (isSystem) => {
  if (isSystem) {
    activeTab.value = 'response'
  }
}, { immediate: true })

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
const savedStateBeforeGoToError = ref<{
  wasExpanded: boolean
  activeTabBefore: 'request' | 'response'
  panelWasOpen: boolean
} | null>(null)

// Handle "Go To Error" from error dialog
function handleGoToError(error: ScriptErrorInfo) {
  // Close error dialog
  showErrorDialog.value = false

  // Save current state for restoration
  savedStateBeforeGoToError.value = {
    wasExpanded: props.isExpanded,
    activeTabBefore: activeTab.value,
    panelWasOpen: showEditorPanel.value
  }

  // Store error info to pass to editor
  currentScriptError.value = error

  // Enter "Go To Error" mode
  isGoToErrorMode.value = true

  // Expand card if it wasn't already (so user sees context)
  if (!props.isExpanded) {
    emit('toggle')
  }

  // Open the full editor panel with auto-launch of script editor
  autoOpenScriptEditor.value = true
  showEditorPanel.value = true
}

// Restore state after "Go To Error" modal closes
function restoreStateAfterGoToError() {
  if (!isGoToErrorMode.value || !savedStateBeforeGoToError.value) {
    return
  }

  const saved = savedStateBeforeGoToError.value

  // Close the panel
  showEditorPanel.value = false

  // Restore tab
  activeTab.value = saved.activeTabBefore

  // Restore expansion state
  if (!saved.wasExpanded && props.isExpanded) {
    emit('toggle')
  }

  // Clear state
  isGoToErrorMode.value = false
  savedStateBeforeGoToError.value = null
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

// Keyboard shortcuts
function handleKeydown(e: KeyboardEvent) {
  // Ignore when typing in inputs or textareas
  if (['INPUT', 'TEXTAREA'].includes((e.target as HTMLElement).tagName)) {
    return
  }

  if (e.key === 'e' || e.key === 'E') {
    e.preventDefault()
    emit('toggle')
  } else if (e.ctrlKey && e.key === 'b') {
    e.preventDefault()
    // Context-aware: open body editor or script editor
    if (currentMode.value === 'script') {
      // Script mode - no body editor, this shortcut not applicable
      return
    }
    // Open body editor (handled in ResponseEditorContent)
  } else if (e.ctrlKey && e.key === 's') {
    e.preventDefault()
    // Open script editor (validation or response)
    // Handled in ResponseEditorContent
  } else if (e.ctrlKey && e.key === 'Enter') {
    e.preventDefault()
    applyChanges()
  }
}

// Add/remove keyboard listener when card is expanded
watch(() => props.isExpanded, (newVal) => {
  if (newVal) {
    document.addEventListener('keydown', handleKeydown)
  } else {
    document.removeEventListener('keydown', handleKeydown)
  }
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div
    ref="cardRef"
    class="rounded-lg border overflow-hidden transition-all"
    :class="[
      isEnabled ? 'bg-gray-800 border-gray-700' : 'bg-gray-900 border-gray-800 opacity-60',
      { 'ring-2 ring-blue-500': isExpanded, 'opacity-50': isDragging }
    ]"
    @dragover.prevent="onDragOver"
    @drop="onDrop"
  >
    <!-- Collapsed Header (always visible) - entire header is draggable -->
    <div
      class="px-3 py-2 cursor-grab active:cursor-grabbing hover:bg-gray-750 transition-colors select-none"
      draggable="true"
      @dragstart="onHandleDragStart"
      @dragend="onHandleDragEnd"
      @click="emit('toggle')"
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

        <!-- Expand/Collapse Arrow -->
        <svg
          class="w-4 h-4 text-gray-400 transition-transform flex-shrink-0"
          :class="{ 'rotate-180': isExpanded }"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
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

    <!-- Expanded Content -->
    <div v-if="isExpanded" class="border-t border-gray-700 flex flex-col">
      <!-- Tab Navigation -->
      <div class="flex items-center border-b border-gray-700">
        <!-- Tabs -->
        <div class="flex flex-1">
          <!-- Request tab - hidden for system endpoints -->
          <button
            v-if="!serverStore.currentEndpoint?.is_system"
            @click="activeTab = 'request'"
            :class="[
              'px-4 py-2 text-sm font-medium transition-colors',
              activeTab === 'request'
                ? 'text-blue-400 border-b-2 border-blue-400'
                : 'text-gray-400 hover:text-gray-300'
            ]"
          >
            Request
          </button>
          <button
            @click="activeTab = 'response'"
            :class="[
              'px-4 py-2 text-sm font-medium transition-colors',
              activeTab === 'response'
                ? 'text-blue-400 border-b-2 border-blue-400'
                : 'text-gray-400 hover:text-gray-300'
            ]"
          >
            Response
          </button>
        </div>

        <!-- Full Editor Button -->
        <button
          @click="showEditorPanel = true"
          class="mr-3 px-2 py-1 text-xs bg-gray-700 hover:bg-gray-600 rounded text-gray-300 transition-colors flex items-center gap-1"
          title="Open in full editor panel"
        >
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
          </svg>
          Full Editor
        </button>
      </div>

      <!-- Tab Content -->
      <div class="p-4">
        <ResponseEditorContent
          :local-response="localResponse"
          :active-tab="activeTab"
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
          :status-code-options="statusCodeOptions"
          :is-system-endpoint="serverStore.currentEndpoint?.is_system || false"
          @update:local-response="handleLocalResponseUpdate"
          @update:current-mode="currentMode = $event"
          @update:validation-mode="validationMode = $event"
          @update:validation-match-type="validationMatchType = $event"
          @update:validation-pattern="validationPattern = $event"
          @update:validation-script="validationScript = $event"
          @update:content-type="contentType = $event"
          @update:use-global-c-o-r-s="useGlobalCORS = $event; applyChanges()"
          @apply-changes="applyChanges"
          @clear-body="clearBody"
        />
      </div>

      <!-- Action Buttons -->
      <div class="flex items-center justify-between gap-2 p-4 pt-0">
        <!-- Destructive action - left -->
        <button
          @click="emit('delete')"
          class="p-1.5 text-red-400 hover:text-red-300 hover:bg-red-900/20 rounded transition-colors"
          title="Delete this response"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </button>

        <!-- Primary actions - right -->
        <div class="flex gap-2">
          <button
            @click="resetChanges"
            :disabled="!isDirty"
            :class="[
              'px-3 py-1.5 rounded text-sm font-medium transition-colors',
              isDirty
                ? 'bg-gray-700 hover:bg-gray-600 text-white'
                : 'bg-gray-800 text-gray-600 cursor-not-allowed'
            ]"
            title="Reset to original values"
          >
            Reset
          </button>
          <button
            @click="applyChanges"
            :disabled="!isDirty"
            :class="[
              'px-3 py-1.5 rounded text-sm font-medium transition-colors',
              isDirty
                ? 'bg-blue-600 hover:bg-blue-700 text-white'
                : 'bg-blue-900 text-blue-700 cursor-not-allowed'
            ]"
          >
            Save
          </button>
        </div>
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
      @save="localResponse = $event; applyChanges(); showEditorPanel = false; autoOpenScriptEditor = false; currentScriptError = null; isGoToErrorMode = false"
      @close="isGoToErrorMode ? restoreStateAfterGoToError() : (showEditorPanel = false, autoOpenScriptEditor = false, currentScriptError = null)"
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
