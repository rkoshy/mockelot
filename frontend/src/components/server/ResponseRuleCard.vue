<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
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

// Headers management
const newHeaderKey = ref('')
const newHeaderValue = ref('')

// Body editor modal
const showBodyEditor = ref(false)

// Script editor modal
const showScriptEditor = ref(false)

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

// Validation section accordion state
const showValidationSection = ref(false)

// Response section accordion state
const showResponseSection = ref(false)

// Validation script editor modal
const showValidationScriptEditor = ref(false)

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

// Sync with props changes
watch(() => props.response, (newVal) => {
  localResponse.value = new models.MethodResponse({ ...newVal })
}, { deep: true })

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

// Toggle HTTP method
function toggleMethod(method: string) {
  const methods = [...localResponse.value.methods]
  const index = methods.indexOf(method)
  if (index > -1) {
    methods.splice(index, 1)
  } else {
    methods.push(method)
  }
  localResponse.value.methods = methods
}

// Add header
function addHeader() {
  if (newHeaderKey.value.trim() && newHeaderValue.value.trim()) {
    localResponse.value.headers = {
      ...localResponse.value.headers,
      [newHeaderKey.value.trim()]: newHeaderValue.value.trim()
    }
    newHeaderKey.value = ''
    newHeaderValue.value = ''
  }
}

// Remove header
function removeHeader(key: string) {
  const headers = { ...localResponse.value.headers }
  delete headers[key]
  localResponse.value.headers = headers
}

// Update status code and text together
function updateStatusCode(code: number) {
  localResponse.value.status_code = code
  const found = STATUS_CODES.find(s => s.code === code)
  if (found) {
    localResponse.value.status_text = found.text
  }
}

// Apply changes
function applyChanges() {
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
    <div v-if="isExpanded" class="border-t border-gray-700 p-4 space-y-4">
      <!-- Path Pattern -->
      <div class="space-y-1">
        <label class="block text-xs font-medium text-gray-400">Path Pattern (supports regex: ^...)</label>
        <input
          v-model="localResponse.path_pattern"
          type="text"
          placeholder="/* or ^/api/v[0-9]+/"
          class="w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-sm text-white font-mono
                 focus:outline-none focus:border-blue-500"
        />
      </div>

      <!-- HTTP Methods -->
      <div class="space-y-1">
        <label class="block text-xs font-medium text-gray-400">HTTP Methods</label>
        <div class="flex flex-wrap gap-1">
          <button
            v-for="method in HTTP_METHODS"
            :key="method"
            @click="toggleMethod(method)"
            :class="[
              'px-2 py-0.5 rounded text-xs font-medium transition-colors',
              localResponse.methods.includes(method)
                ? getMethodColor(method) + ' text-white'
                : 'bg-gray-700 text-gray-400 hover:bg-gray-600'
            ]"
          >
            {{ method }}
          </button>
        </div>
      </div>

      <!-- Global CORS -->
      <div class="space-y-1">
        <label class="flex items-center gap-2 cursor-pointer" :class="{ 'opacity-50 cursor-not-allowed': handlesOptions }">
          <input
            v-model="useGlobalCORS"
            type="checkbox"
            :disabled="handlesOptions"
            @change="applyChanges"
            class="w-3.5 h-3.5 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
          />
          <span class="text-xs font-medium text-gray-400">Use Global CORS</span>
        </label>
        <p v-if="handlesOptions" class="text-[10px] text-yellow-400 ml-5">
          ℹ️ CORS override disabled - this entry handles OPTIONS requests
        </p>
        <p v-else-if="useGlobalCORS" class="text-[10px] text-gray-500 ml-5">
          Global CORS headers will be applied to this response (if enabled in server config)
        </p>
        <p v-else class="text-[10px] text-gray-500 ml-5">
          Global CORS will NOT be applied to this response, even if enabled globally
        </p>
      </div>

      <!-- Request Validation (Accordion) -->
      <div class="space-y-1">
        <button
          @click="showValidationSection = !showValidationSection"
          class="flex items-center justify-between w-full text-left py-1.5 border-b border-gray-600 hover:border-gray-500 transition-colors"
        >
          <label class="text-xs font-medium text-gray-400 cursor-pointer">
            Request Body Validation
            <span v-if="validationMode !== 'none'" class="text-[10px] text-purple-400 ml-1">({{ VALIDATION_MODE_LABELS[validationMode] }})</span>
          </label>
          <svg
            class="w-4 h-4 text-gray-400 transition-transform"
            :class="{ 'rotate-180': showValidationSection }"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        <div v-if="showValidationSection" class="space-y-2 pt-1">
          <!-- Validation Mode Selector -->
          <div class="flex gap-1">
            <button
              v-for="mode in VALIDATION_MODES"
              :key="mode"
              @click="validationMode = mode"
              :class="[
                'px-2 py-1 rounded text-xs font-medium transition-colors',
                validationMode === mode
                  ? 'bg-purple-600 text-white'
                  : 'bg-gray-700 text-gray-400 hover:bg-gray-600'
              ]"
            >
              {{ VALIDATION_MODE_LABELS[mode] }}
            </button>
          </div>

          <!-- Static Validation Options -->
          <template v-if="validationMode === 'static'">
            <div class="space-y-2">
              <div class="flex gap-1">
                <button
                  v-for="matchType in VALIDATION_MATCH_TYPES"
                  :key="matchType"
                  @click="validationMatchType = matchType"
                  :class="[
                    'px-2 py-0.5 rounded text-[10px] font-medium transition-colors',
                    validationMatchType === matchType
                      ? 'bg-gray-600 text-white'
                      : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
                  ]"
                >
                  {{ VALIDATION_MATCH_TYPE_LABELS[matchType] }}
                </button>
              </div>
              <input
                v-model="validationPattern"
                type="text"
                placeholder="Text to match in request body..."
                class="w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-xs text-white
                       focus:outline-none focus:border-purple-500"
              />
            </div>
          </template>

          <!-- Regex Validation Options -->
          <template v-else-if="validationMode === 'regex'">
            <div class="space-y-2">
              <input
                v-model="validationPattern"
                type="text"
                :placeholder="'(?P<userId>\\d+)'"
                class="w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-xs text-white font-mono
                       focus:outline-none focus:border-purple-500"
              />
              <p class="text-[10px] text-gray-500">
                Use <code class="text-purple-400">(?P&lt;name&gt;pattern)</code> to extract named variables.
                Variables available as <code class="text-yellow-400">request.vars.name</code> in scripts or
                <code class="text-yellow-400" v-pre>{{.Vars.name}}</code> in templates.
              </p>
            </div>
          </template>

          <!-- Script Validation Options -->
          <template v-else-if="validationMode === 'script'">
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <p class="text-[10px] text-gray-500">JavaScript validation with variable extraction</p>
                <button
                  @click="showValidationScriptEditor = true"
                  class="px-2 py-0.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors flex items-center gap-1"
                >
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                  </svg>
                  Edit
                </button>
              </div>
              <textarea
                v-model="validationScript"
                rows="4"
                placeholder="// Set result.valid = true/false
// Extract variables: result.vars.userId = ...

const json = JSON.parse(body);
result.valid = json.userId !== undefined;
result.vars.userId = json.userId;
result.vars.action = json.action || 'default';"
                class="w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-xs text-white
                       font-mono focus:outline-none focus:border-purple-500 resize-none"
              />
            </div>
          </template>

          <!-- None mode description -->
          <p v-else class="text-[10px] text-gray-500">
            No validation - this response will match if path and method match.
          </p>
        </div>
      </div>

      <!-- Validation Script Editor Modal -->
      <ScriptEditorModal
        :model-value="validationScript"
        @update:model-value="validationScript = $event"
        v-model:visible="showValidationScriptEditor"
        title="Edit Validation Script"
      />

      <!-- Response Body (Accordion) -->
      <div class="space-y-1">
        <button
          @click="showResponseSection = !showResponseSection"
          class="flex items-center justify-between w-full text-left py-1.5 border-b border-gray-600 hover:border-gray-500 transition-colors"
        >
          <label class="text-xs font-medium text-gray-400 cursor-pointer">
            Response Status &amp; Body
            <span class="text-[10px] text-blue-400 ml-1">({{ localResponse.status_code }} - {{ RESPONSE_MODE_LABELS[currentMode] }})</span>
          </label>
          <svg
            class="w-4 h-4 text-gray-400 transition-transform"
            :class="{ 'rotate-180': showResponseSection }"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        <div v-if="showResponseSection" class="space-y-3 pt-1">
          <!-- Status Code -->
          <div class="space-y-1">
            <label class="block text-[10px] font-medium text-gray-500">Status Code</label>
            <ComboBox
              :model-value="localResponse.status_code"
              :model-text="localResponse.status_text"
              :options="statusCodeOptions"
              @update:model-value="localResponse.status_code = Number($event)"
              @update:model-text="localResponse.status_text = $event"
            />
          </div>

          <!-- Response Delay -->
          <div class="space-y-1">
            <label class="block text-[10px] font-medium text-gray-500">Response Delay (ms)</label>
            <input
              v-model.number="localResponse.response_delay"
              type="number"
              min="0"
              max="60000"
              class="w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-xs text-white
                     focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- Response Headers -->
          <div class="space-y-1">
            <label class="block text-[10px] font-medium text-gray-500">Response Headers</label>

            <!-- Existing Headers -->
            <div class="space-y-1">
              <div
                v-for="(value, key) in localResponse.headers"
                :key="key"
                class="flex items-center gap-2 bg-gray-900 px-2 py-1 rounded text-xs"
              >
                <span class="text-blue-400 flex-shrink-0">{{ key }}:</span>
                <span class="text-gray-300 truncate flex-1">{{ value }}</span>
                <button
                  @click="removeHeader(String(key))"
                  class="text-red-400 hover:text-red-300 flex-shrink-0"
                >
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>

            <!-- Add Header Form - stacked vertically -->
            <div class="space-y-2">
              <input
                v-model="newHeaderKey"
                type="text"
                placeholder="Header name"
                class="w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-xs text-white
                       focus:outline-none focus:border-blue-500"
              />
              <div class="flex gap-2">
                <input
                  v-model="newHeaderValue"
                  type="text"
                  placeholder="Value"
                  class="flex-1 px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-xs text-white
                         focus:outline-none focus:border-blue-500"
                />
                <button
                  @click="addHeader"
                  class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 rounded text-xs text-white font-medium"
                >
                  Add
                </button>
              </div>
            </div>
          </div>

          <!-- Response Mode Selector -->
          <div class="space-y-1">
            <label class="block text-[10px] font-medium text-gray-500">Response Mode</label>
            <div class="flex gap-1">
              <button
                v-for="mode in RESPONSE_MODES"
                :key="mode"
                @click="currentMode = mode"
                :class="[
                  'px-2 py-1 rounded text-xs font-medium transition-colors',
                  currentMode === mode
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-700 text-gray-400 hover:bg-gray-600'
                ]"
              >
                {{ RESPONSE_MODE_LABELS[mode] }}
              </button>
            </div>
            <p class="text-[10px] text-gray-500">
              <template v-if="currentMode === 'static'">Simple response with no processing</template>
              <template v-else-if="currentMode === 'template'">Use <span v-pre>{{.PathParams.id}}</span> syntax for dynamic values</template>
              <template v-else>JavaScript with access to request/response objects</template>
            </p>
          </div>

          <!-- Response Body Section (Static and Template modes) -->
          <div v-if="currentMode !== 'script'" class="space-y-2">
            <div class="flex items-center justify-between">
              <label class="block text-[10px] font-medium text-gray-500">
                Body Content
                <span v-if="currentMode === 'template'" class="text-yellow-500 ml-1">(Template)</span>
              </label>
              <div class="flex items-center gap-2">
                <button
                  v-if="hasBody"
                  @click="clearBody"
                  class="px-2 py-0.5 bg-gray-700 hover:bg-red-600 rounded text-xs text-gray-300 hover:text-white transition-colors"
                  title="Clear body and content type"
                >
                  Clear
                </button>
                <button
                  @click="showBodyEditor = true"
                  class="px-2 py-0.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors flex items-center gap-1"
                >
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
                  </svg>
                  Expand
                </button>
              </div>
            </div>

            <!-- Content-Type Selector -->
            <div class="space-y-1">
              <label class="block text-[10px] font-medium text-gray-500">Content-Type</label>
              <ContentTypeSelector
                v-model="contentType"
                @clear="clearBody"
              />
            </div>

            <!-- Body Textarea -->
            <textarea
              v-model="localResponse.body"
              rows="4"
              :placeholder="bodyPlaceholder"
              class="w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-xs text-white
                     font-mono focus:outline-none focus:border-blue-500 resize-none"
            />

            <!-- Template Help -->
            <div v-if="currentMode === 'template'" class="text-[10px] text-gray-500 bg-gray-900 rounded p-2">
              <span class="font-semibold text-yellow-500">Template Variables:</span>
              <code class="ml-1" v-pre>{{.Method}}</code>,
              <code v-pre>{{.Path}}</code>,
              <code v-pre>{{.PathParams.id}}</code>,
              <code v-pre>{{.Body.Raw}}</code>,
              <code v-pre>{{.GetQueryParam "key"}}</code>
            </div>
          </div>

          <!-- Script Body Section (Script mode) -->
          <div v-else class="space-y-2">
            <div class="flex items-center justify-between">
              <label class="block text-[10px] font-medium text-gray-500">
                Script Body
                <span class="text-yellow-500 ml-1">(JavaScript)</span>
              </label>
              <button
                @click="showScriptEditor = true"
                class="px-2 py-0.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors flex items-center gap-1"
              >
                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                </svg>
                Edit Script
              </button>
            </div>

            <!-- Script Textarea -->
            <textarea
              v-model="localResponse.script_body"
              rows="6"
              placeholder="// Access request data via 'request' object
// Modify response via 'response' object

const userId = request.pathParams.id;
response.status = 200;
response.body = JSON.stringify({ userId });"
              class="w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-xs text-white
                     font-mono focus:outline-none focus:border-blue-500 resize-none"
            />
          </div>
        </div>
      </div>

      <!-- Body Editor Modal (for Static/Template) -->
      <BodyEditorModal
        :model-value="localResponse.body || ''"
        @update:model-value="localResponse.body = $event"
        v-model:visible="showBodyEditor"
        :content-type="contentType"
        title="Edit Response Body"
      />

      <!-- Script Editor Modal (for Script mode) -->
      <ScriptEditorModal
        :model-value="localResponse.script_body || ''"
        @update:model-value="localResponse.script_body = $event"
        v-model:visible="showScriptEditor"
        title="Edit Script"
      />

      <!-- Action Buttons -->
      <div class="flex gap-2 pt-2">
        <button
          @click="applyChanges"
          class="flex-1 py-1.5 bg-blue-600 hover:bg-blue-700 rounded text-sm font-medium text-white transition-colors"
        >
          Save Changes
        </button>
        <button
          @click="emit('delete')"
          class="px-3 py-1.5 bg-red-600 hover:bg-red-700 rounded text-sm font-medium text-white transition-colors"
        >
          Delete
        </button>
      </div>
    </div>
  </div>
</template>
