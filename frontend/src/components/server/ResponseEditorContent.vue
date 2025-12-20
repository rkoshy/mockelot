<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
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
import HeaderValidationList from '../dialogs/HeaderValidationList.vue'
import type { ScriptErrorInfo } from './ScriptErrorLogDialog.vue'

const props = withDefaults(defineProps<{
  localResponse: models.MethodResponse
  activeTab: 'request' | 'response'
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
  statusCodeOptions: Array<{ value: number, label: string }>
  isInPanel?: boolean
  scriptError?: ScriptErrorInfo | null
  isSystemEndpoint?: boolean
}>(), {
  isInPanel: false,
  scriptError: null,
  isSystemEndpoint: false
})

const emit = defineEmits<{
  'update:localResponse': [response: models.MethodResponse]
  'update:currentMode': [mode: ResponseMode]
  'update:validationMode': [mode: ValidationMode]
  'update:validationMatchType': [type: ValidationMatchType]
  'update:validationPattern': [pattern: string]
  'update:validationScript': [script: string]
  'update:contentType': [type: string]
  'update:useGlobalCORS': [value: boolean]
  'applyChanges': []
  'clearBody': []
}>()

// Headers management
const newHeaderKey = ref('')
const newHeaderValue = ref('')

// Modal states
const showBodyEditor = ref(false)
const showScriptEditor = ref(false)
const showValidationScriptEditor = ref(false)

// Refs for script textareas and line number divs
const validationScriptTextareaRef = ref<HTMLTextAreaElement | null>(null)
const validationScriptLineNumbersRef = ref<HTMLDivElement | null>(null)
const responseScriptTextareaRef = ref<HTMLTextAreaElement | null>(null)
const responseScriptLineNumbersRef = ref<HTMLDivElement | null>(null)

// Calculate line numbers for validation script
const validationScriptLineNumbers = computed(() => {
  const lines = (props.validationScript || '').split('\n')
  return Array.from({ length: lines.length }, (_, i) => i + 1)
})

// Calculate line numbers for response script
const responseScriptLineNumbers = computed(() => {
  const lines = (props.localResponse.script_body || '').split('\n')
  return Array.from({ length: lines.length }, (_, i) => i + 1)
})

// Sync scroll for validation script
function handleValidationScriptScroll() {
  if (validationScriptTextareaRef.value && validationScriptLineNumbersRef.value) {
    validationScriptLineNumbersRef.value.scrollTop = validationScriptTextareaRef.value.scrollTop
  }
}

// Sync scroll for response script
function handleResponseScriptScroll() {
  if (responseScriptTextareaRef.value && responseScriptLineNumbersRef.value) {
    responseScriptLineNumbersRef.value.scrollTop = responseScriptTextareaRef.value.scrollTop
  }
}

// Expose modal state so parent components can control it
defineExpose({
  showScriptEditor,
  showValidationScriptEditor
})

// Computed row counts based on context (inline vs panel)
const validationScriptRows = computed(() => props.isInPanel ? 20 : 12)
const responseScriptRows = computed(() => props.isInPanel ? 20 : 12)
const responseBodyRows = computed(() => props.isInPanel ? 15 : 8)

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
  const methods = [...props.localResponse.methods]
  const index = methods.indexOf(method)
  if (index > -1) {
    methods.splice(index, 1)
  } else {
    methods.push(method)
  }
  const updated = new models.MethodResponse({ ...props.localResponse, methods })
  emit('update:localResponse', updated)
}

// Add header
function addHeader() {
  if (newHeaderKey.value.trim() && newHeaderValue.value.trim()) {
    const updated = new models.MethodResponse({
      ...props.localResponse,
      headers: {
        ...props.localResponse.headers,
        [newHeaderKey.value.trim()]: newHeaderValue.value.trim()
      }
    })
    emit('update:localResponse', updated)
    newHeaderKey.value = ''
    newHeaderValue.value = ''
  }
}

// Remove header
function removeHeader(key: string) {
  const headers = { ...props.localResponse.headers }
  delete headers[key]
  const updated = new models.MethodResponse({ ...props.localResponse, headers })
  emit('update:localResponse', updated)
}

// Update path pattern
function updatePathPattern(value: string) {
  const updated = new models.MethodResponse({ ...props.localResponse, path_pattern: value })
  emit('update:localResponse', updated)
}

// Update status (combined code and text)
function updateStatus(payload: { value: number | string; text: string }) {
  const code = typeof payload.value === 'number' ? payload.value : Number(payload.value)
  console.log('[ResponseEditorContent] updateStatus called:', {
    newCode: code,
    newText: payload.text,
    oldCode: props.localResponse.status_code,
    oldText: props.localResponse.status_text
  })

  const updated = new models.MethodResponse({
    ...props.localResponse,
    status_code: code,
    status_text: payload.text
  })

  console.log('[ResponseEditorContent] Emitting update:localResponse with status_code:', updated.status_code, 'status_text:', updated.status_text)
  emit('update:localResponse', updated)
}

// Update response delay
function updateResponseDelay(delay: number) {
  const updated = new models.MethodResponse({ ...props.localResponse, response_delay: delay })
  emit('update:localResponse', updated)
}

// Update body
function updateBody(body: string) {
  const updated = new models.MethodResponse({ ...props.localResponse, body })
  emit('update:localResponse', updated)
}

// Update script body
function updateScriptBody(script_body: string) {
  const updated = new models.MethodResponse({ ...props.localResponse, script_body })
  emit('update:localResponse', updated)
}

// Update header validations
function updateHeaderValidations(headers: models.HeaderValidation[]) {
  const updated = new models.MethodResponse({
    ...props.localResponse,
    request_validation: {
      ...props.localResponse.request_validation,
      headers: headers
    }
  })
  emit('update:localResponse', updated)
}

// Watch for status code changes in localResponse
watch(() => props.localResponse.status_code, (newVal, oldVal) => {
  console.log('[ResponseEditorContent] props.localResponse.status_code changed:', {
    from: oldVal,
    to: newVal
  })
})
</script>

<template>
  <div class="space-y-4">
    <!-- Request Tab Content -->
    <template v-if="activeTab === 'request'">
      <!-- Path Pattern -->
      <div class="space-y-1">
        <label class="block text-xs font-medium text-gray-400">Path Pattern (supports regex: ^...)</label>
        <input
          :value="localResponse.path_pattern"
          @input="updatePathPattern(($event.target as HTMLInputElement).value)"
          type="text"
          placeholder="/* or ^/api/v[0-9]+/"
          :disabled="isSystemEndpoint"
          :class="[
            'w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-sm text-white font-mono focus:outline-none focus:border-blue-500',
            isSystemEndpoint ? 'opacity-50 cursor-not-allowed' : ''
          ]"
        />
      </div>

      <!-- HTTP Methods -->
      <div class="space-y-1">
        <label class="block text-xs font-medium text-gray-400">HTTP Methods</label>
        <div class="flex flex-wrap gap-1">
          <button
            v-for="method in HTTP_METHODS"
            :key="method"
            @click="!isSystemEndpoint && toggleMethod(method)"
            :disabled="isSystemEndpoint"
            :class="[
              'px-2 py-0.5 rounded text-xs font-medium transition-colors',
              localResponse.methods.includes(method)
                ? getMethodColor(method) + ' text-white'
                : 'bg-gray-700 text-gray-400 hover:bg-gray-600',
              isSystemEndpoint ? 'opacity-50 cursor-not-allowed' : ''
            ]"
          >
            {{ method }}
          </button>
        </div>
      </div>

      <!-- Global CORS -->
      <div class="space-y-1">
        <label class="flex items-center gap-2 cursor-pointer" :class="{ 'opacity-50 cursor-not-allowed': handlesOptions || isSystemEndpoint }">
          <input
            :checked="useGlobalCORS"
            @change="emit('update:useGlobalCORS', ($event.target as HTMLInputElement).checked); emit('applyChanges')"
            type="checkbox"
            :disabled="handlesOptions || isSystemEndpoint"
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

      <!-- Request Body Validation (inline, no accordion) -->
      <div class="space-y-2">
        <label class="block text-xs font-medium text-gray-400">
          Request Body Validation
          <span v-if="validationMode !== 'none'" class="text-[10px] text-purple-400 ml-1">({{ VALIDATION_MODE_LABELS[validationMode] }})</span>
        </label>

        <!-- Validation Mode Selector -->
        <div class="flex gap-1">
          <button
            v-for="mode in VALIDATION_MODES"
            :key="mode"
            @click="emit('update:validationMode', mode)"
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
                @click="emit('update:validationMatchType', matchType)"
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
              :value="validationPattern"
              @input="emit('update:validationPattern', ($event.target as HTMLInputElement).value)"
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
              :value="validationPattern"
              @input="emit('update:validationPattern', ($event.target as HTMLInputElement).value)"
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
            <!-- Validation Script Textarea with Line Numbers -->
            <div class="flex border border-gray-600 rounded overflow-hidden bg-gray-900">
              <!-- Line Numbers -->
              <div
                ref="validationScriptLineNumbersRef"
                class="flex flex-col py-1.5 px-2 bg-gray-800 border-r border-gray-600 text-gray-500 text-xs font-mono select-none overflow-hidden"
              >
                <div
                  v-for="lineNum in validationScriptLineNumbers"
                  :key="lineNum"
                  class="leading-[1.375rem] text-right pr-2"
                  style="min-height: 22px"
                >
                  {{ lineNum }}
                </div>
              </div>
              <!-- Textarea -->
              <textarea
                ref="validationScriptTextareaRef"
                :value="validationScript"
                @input="emit('update:validationScript', ($event.target as HTMLTextAreaElement).value)"
                @scroll="handleValidationScriptScroll"
                :rows="validationScriptRows"
                placeholder="// Set result.valid = true/false
// Extract variables: result.vars.userId = ...

const json = JSON.parse(body);
result.valid = json.userId !== undefined;
result.vars.userId = json.userId;
result.vars.action = json.action || 'default';"
                class="flex-1 px-2 py-1.5 bg-gray-900 text-xs text-white
                       font-mono focus:outline-none focus:border-purple-500 resize-y leading-[1.375rem]"
                style="outline: none; border: none;"
              />
            </div>
          </div>
        </template>

        <!-- None mode description -->
        <p v-else class="text-[10px] text-gray-500">
          No validation - this response will match if path and method match.
        </p>
      </div>

      <!-- Header Validation (inline, no accordion) -->
      <div class="space-y-2">
        <HeaderValidationList
          :model-value="localResponse.request_validation?.headers || []"
          @update:model-value="updateHeaderValidations"
        />
      </div>

      <!-- Validation Script Editor Modal -->
      <ScriptEditorModal
        :model-value="validationScript"
        @update:model-value="emit('update:validationScript', $event)"
        v-model:visible="showValidationScriptEditor"
        title="Edit Validation Script"
        :error-info="scriptError"
      />
    </template>

    <!-- Response Tab Content -->
    <template v-if="activeTab === 'response'">
      <!-- Status Code -->
      <div class="space-y-1">
        <label class="block text-[10px] font-medium text-gray-500">Status Code</label>
        <ComboBox
          :model-value="localResponse.status_code"
          :model-text="localResponse.status_text"
          :options="statusCodeOptions"
          @update="updateStatus"
        />
      </div>

      <!-- Response Delay -->
      <div class="space-y-1">
        <label class="block text-[10px] font-medium text-gray-500">Response Delay (ms)</label>
        <input
          :value="localResponse.response_delay"
          @input="updateResponseDelay(Number(($event.target as HTMLInputElement).value))"
          type="number"
          min="0"
          max="60000"
          :disabled="isSystemEndpoint"
          :class="[
            'w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-xs text-white focus:outline-none focus:border-blue-500',
            isSystemEndpoint ? 'opacity-50 cursor-not-allowed' : ''
          ]"
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

        <!-- Add Header Form -->
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
            @click="emit('update:currentMode', mode)"
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
              @click="emit('clearBody')"
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
            :model-value="contentType"
            @update:model-value="emit('update:contentType', $event)"
            @clear="emit('clearBody')"
          />
        </div>

        <!-- Body Textarea -->
        <textarea
          :value="localResponse.body"
          @input="updateBody(($event.target as HTMLTextAreaElement).value)"
          :rows="responseBodyRows"
          :placeholder="bodyPlaceholder"
          class="w-full px-2 py-1.5 bg-gray-900 border border-gray-600 rounded text-xs text-white
                 font-mono focus:outline-none focus:border-blue-500 resize-y"
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

        <!-- Script Textarea with Line Numbers -->
        <div class="flex border border-gray-600 rounded overflow-hidden bg-gray-900">
          <!-- Line Numbers -->
          <div
            ref="responseScriptLineNumbersRef"
            class="flex flex-col py-1.5 px-2 bg-gray-800 border-r border-gray-600 text-gray-500 text-xs font-mono select-none overflow-hidden"
          >
            <div
              v-for="lineNum in responseScriptLineNumbers"
              :key="lineNum"
              class="leading-[1.375rem] text-right pr-2"
              style="min-height: 22px"
            >
              {{ lineNum }}
            </div>
          </div>
          <!-- Textarea -->
          <textarea
            ref="responseScriptTextareaRef"
            :value="localResponse.script_body"
            @input="updateScriptBody(($event.target as HTMLTextAreaElement).value)"
            @scroll="handleResponseScriptScroll"
            :rows="responseScriptRows"
            placeholder="// Access request data via 'request' object
// Modify response via 'response' object

const userId = request.pathParams.id;
response.status = 200;
response.body = JSON.stringify({ userId });"
            class="flex-1 px-2 py-1.5 bg-gray-900 text-xs text-white
                   font-mono focus:outline-none focus:border-blue-500 resize-y leading-[1.375rem]"
            style="outline: none; border: none;"
          />
        </div>
      </div>

      <!-- Body Editor Modal (for Static/Template) -->
      <BodyEditorModal
        :model-value="localResponse.body || ''"
        @update:model-value="updateBody($event)"
        v-model:visible="showBodyEditor"
        :content-type="contentType"
        title="Edit Response Body"
      />

      <!-- Script Editor Modal (for Script mode) -->
      <ScriptEditorModal
        :model-value="localResponse.script_body || ''"
        @update:model-value="updateScriptBody($event)"
        v-model:visible="showScriptEditor"
        title="Edit Script"
        :error-info="scriptError"
      />
    </template>
  </div>
</template>
