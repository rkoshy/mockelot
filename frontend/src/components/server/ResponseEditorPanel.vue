<script lang="ts" setup>
import { ref, computed, watch, nextTick } from 'vue'
import { models } from '../../types/models'
import {
  STATUS_CODES,
  type ResponseMode,
  type ValidationMode,
  type ValidationMatchType
} from '../../types/models'
import ResponseEditorContent from './ResponseEditorContent.vue'
import type { ScriptErrorInfo } from './ScriptErrorLogDialog.vue'

const props = withDefaults(defineProps<{
  visible: boolean
  localResponse: models.MethodResponse
  currentMode: ResponseMode
  validationMode: ValidationMode
  validationMatchType: ValidationMatchType
  validationPattern: string
  validationScript: string
  contentType: string
  hasBody: boolean
  bodyPlaceholder: string
  handlesOptions: boolean
  useGlobalCORS: boolean
  autoOpenScriptEditor?: boolean
  scriptError?: ScriptErrorInfo | null
  isGoToErrorMode?: boolean
}>(), {
  autoOpenScriptEditor: false,
  scriptError: null,
  isGoToErrorMode: false
})

const emit = defineEmits<{
  close: []
  save: [response: models.MethodResponse]
  scriptModalClosed: []
}>()

const activeTab = ref<'request' | 'response'>('request')

// Ref to ResponseEditorContent for programmatic control
const editorContentRef = ref<InstanceType<typeof ResponseEditorContent> | null>(null)

// Create a local copy to work with
const panelResponse = ref<models.MethodResponse>(new models.MethodResponse({ ...props.localResponse }))
const panelCurrentMode = ref<ResponseMode>(props.currentMode)
const panelValidationMode = ref<ValidationMode>(props.validationMode)
const panelValidationMatchType = ref<ValidationMatchType>(props.validationMatchType)
const panelValidationPattern = ref(props.validationPattern)
const panelValidationScript = ref(props.validationScript)
const panelContentType = ref(props.contentType)
const panelUseGlobalCORS = ref(props.useGlobalCORS)

// Store original state for dirty tracking
const originalState = ref({
  response: new models.MethodResponse({ ...props.localResponse }),
  mode: props.currentMode,
  validationMode: props.validationMode,
  validationMatchType: props.validationMatchType,
  validationPattern: props.validationPattern,
  validationScript: props.validationScript,
  contentType: props.contentType,
  useGlobalCORS: props.useGlobalCORS
})

// Track dirty state by comparing individual values
const isDirty = computed(() => {
  return JSON.stringify(panelResponse.value) !== JSON.stringify(originalState.value.response) ||
         panelCurrentMode.value !== originalState.value.mode ||
         panelValidationMode.value !== originalState.value.validationMode ||
         panelValidationMatchType.value !== originalState.value.validationMatchType ||
         panelValidationPattern.value !== originalState.value.validationPattern ||
         panelValidationScript.value !== originalState.value.validationScript ||
         panelContentType.value !== originalState.value.contentType ||
         panelUseGlobalCORS.value !== originalState.value.useGlobalCORS
})

// Convert STATUS_CODES to combobox options
const statusCodeOptions = computed(() =>
  STATUS_CODES.map(s => ({ value: s.code, label: `${s.code} - ${s.text}` }))
)

// Watch for script modal close in "Go To Error" mode
watch(() => editorContentRef.value?.showScriptEditor, (newVal, oldVal) => {
  // Detect when script modal closes (true -> false) in Go To Error mode
  if (props.isGoToErrorMode && oldVal === true && newVal === false) {
    emit('scriptModalClosed')
  }
})

// Reset state when dialog opens
watch(() => props.visible, (newVal) => {
  if (newVal) {
    activeTab.value = 'request'
    panelResponse.value = new models.MethodResponse({ ...props.localResponse })
    panelCurrentMode.value = props.currentMode
    panelValidationMode.value = props.validationMode
    panelValidationMatchType.value = props.validationMatchType
    panelValidationPattern.value = props.validationPattern
    panelValidationScript.value = props.validationScript
    panelContentType.value = props.contentType
    panelUseGlobalCORS.value = props.useGlobalCORS

    // Reset original state
    originalState.value = {
      response: new models.MethodResponse({ ...props.localResponse }),
      mode: props.currentMode,
      validationMode: props.validationMode,
      validationMatchType: props.validationMatchType,
      validationPattern: props.validationPattern,
      validationScript: props.validationScript,
      contentType: props.contentType,
      useGlobalCORS: props.useGlobalCORS
    }

    // Auto-open script editor if requested (for "Go To Error" navigation)
    if (props.autoOpenScriptEditor) {
      nextTick(() => {
        if (editorContentRef.value) {
          // Switch to response tab (most errors are in response scripts)
          activeTab.value = 'response'
          // Open the script editor modal after another tick to ensure tab switch is complete
          nextTick(() => {
            if (editorContentRef.value) {
              editorContentRef.value.showScriptEditor = true
            }
          })
        }
      })
    }
  }
})

function handleReset() {
  panelResponse.value = new models.MethodResponse({ ...originalState.value.response })
  panelCurrentMode.value = originalState.value.mode
  panelValidationMode.value = originalState.value.validationMode
  panelValidationMatchType.value = originalState.value.validationMatchType
  panelValidationPattern.value = originalState.value.validationPattern
  panelValidationScript.value = originalState.value.validationScript
  panelContentType.value = originalState.value.contentType
  panelUseGlobalCORS.value = originalState.value.useGlobalCORS
}

function handleSave() {
  // Apply all changes to the response
  panelResponse.value.response_mode = panelCurrentMode.value

  // Apply request validation
  if (!panelResponse.value.request_validation) {
    panelResponse.value.request_validation = new models.RequestValidation({})
  }
  panelResponse.value.request_validation.mode = panelValidationMode.value
  panelResponse.value.request_validation.match_type = panelValidationMatchType.value
  panelResponse.value.request_validation.pattern = panelValidationPattern.value
  panelResponse.value.request_validation.script = panelValidationScript.value

  // Apply CORS setting
  panelResponse.value.use_global_cors = panelUseGlobalCORS.value

  // Apply Content-Type header
  if (panelContentType.value) {
    panelResponse.value.headers = {
      ...panelResponse.value.headers,
      'Content-Type': panelContentType.value
    }
  } else {
    // Remove Content-Type if empty
    const headers = { ...panelResponse.value.headers }
    delete headers['Content-Type']
    panelResponse.value.headers = headers
  }

  emit('save', panelResponse.value)
}

function handleCancel() {
  emit('close')
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.visible) {
    handleCancel()
  }
}

// Clear body function
function clearBody() {
  const headers = { ...panelResponse.value.headers }
  delete headers['Content-Type']
  panelResponse.value = new models.MethodResponse({
    ...panelResponse.value,
    body: '',
    headers
  })
  panelContentType.value = ''
}

watch(() => props.visible, (show) => {
  if (show) {
    document.addEventListener('keydown', handleKeydown)
  } else {
    document.removeEventListener('keydown', handleKeydown)
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="slide-panel">
      <div v-if="visible" class="fixed inset-0 z-50 flex">
        <!-- Backdrop (click to close) -->
        <div @click="handleCancel" class="flex-1 bg-black/50"></div>

        <!-- Panel (slides from right) -->
        <div class="w-[60vw] min-w-[600px] max-w-[1200px] bg-gray-800 overflow-y-auto shadow-2xl flex flex-col">
          <!-- Header -->
          <div class="px-6 py-4 border-b border-gray-700 flex items-center justify-between flex-shrink-0">
            <h3 class="text-lg font-semibold text-white">Edit Response</h3>
            <button
              @click="handleCancel"
              class="p-1.5 text-gray-400 hover:text-white transition-colors"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- Tab Navigation -->
          <div class="flex border-b border-gray-700 flex-shrink-0">
            <button
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

          <!-- Content -->
          <div class="flex-1 overflow-y-auto p-6">
            <ResponseEditorContent
              ref="editorContentRef"
              :local-response="panelResponse"
              :active-tab="activeTab"
              :current-mode="panelCurrentMode"
              :validation-mode="panelValidationMode"
              :validation-match-type="panelValidationMatchType"
              :validation-pattern="panelValidationPattern"
              :validation-script="panelValidationScript"
              :content-type="panelContentType"
              :has-body="hasBody"
              :body-placeholder="bodyPlaceholder"
              :handles-options="handlesOptions"
              :use-global-c-o-r-s="panelUseGlobalCORS"
              :status-code-options="statusCodeOptions"
              :is-in-panel="true"
              :script-error="scriptError"
              @update:local-response="panelResponse = $event"
              @update:current-mode="panelCurrentMode = $event"
              @update:validation-mode="panelValidationMode = $event"
              @update:validation-match-type="panelValidationMatchType = $event"
              @update:validation-pattern="panelValidationPattern = $event"
              @update:validation-script="panelValidationScript = $event"
              @update:content-type="panelContentType = $event"
              @update:use-global-c-o-r-s="panelUseGlobalCORS = $event"
              @clear-body="clearBody"
            />
          </div>

          <!-- Footer -->
          <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-700 flex-shrink-0">
            <button
              @click="handleCancel"
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded transition-colors"
            >
              Cancel
            </button>
            <button
              @click="handleReset"
              :disabled="!isDirty"
              :class="[
                'px-4 py-2 rounded transition-colors',
                isDirty
                  ? 'bg-gray-700 hover:bg-gray-600 text-white'
                  : 'bg-gray-800 text-gray-600 cursor-not-allowed'
              ]"
              title="Reset to original values"
            >
              Reset
            </button>
            <button
              @click="handleSave"
              :disabled="!isDirty"
              :class="[
                'px-4 py-2 rounded transition-colors',
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
    </Transition>
  </Teleport>
</template>

<style scoped>
.slide-panel-enter-active,
.slide-panel-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.slide-panel-enter-from .bg-gray-800,
.slide-panel-leave-to .bg-gray-800 {
  transform: translateX(100%);
}

.slide-panel-enter-from,
.slide-panel-leave-to {
  opacity: 0;
}
</style>
