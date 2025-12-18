<script lang="ts" setup>
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import type { ScriptErrorInfo } from '../server/ScriptErrorLogDialog.vue'

const props = defineProps<{
  modelValue: string
  visible: boolean
  title?: string
  errorInfo?: ScriptErrorInfo | null
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'update:visible', value: boolean): void
}>()

const showHelp = ref(true)
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const lineNumbersRef = ref<HTMLDivElement | null>(null)

// Calculate line numbers based on content
const lineNumbers = computed(() => {
  const lines = (props.modelValue || '').split('\n')
  return Array.from({ length: lines.length }, (_, i) => i + 1)
})

// Parse line number from error message
function parseErrorLine(errorMessage: string): number | null {
  // Try to match patterns like:
  // "at line 5", "Line 5:", "line 5:10", etc.
  const patterns = [
    /\bline\s+(\d+)/i,
    /\bat\s+(\d+):/i,
    /:(\d+):\d+/  // line:column format
  ]

  for (const pattern of patterns) {
    const match = errorMessage.match(pattern)
    if (match && match[1]) {
      return parseInt(match[1], 10)
    }
  }

  return null
}

// Position cursor on error line when modal opens with error info
watch(() => props.visible, (newVal) => {
  if (newVal && props.errorInfo && textareaRef.value) {
    nextTick(() => {
      if (textareaRef.value) {
        const errorLine = parseErrorLine(props.errorInfo!.error)
        if (errorLine !== null) {
          // Calculate character position for the error line
          const lines = props.modelValue.split('\n')
          let position = 0
          for (let i = 0; i < Math.min(errorLine - 1, lines.length); i++) {
            position += lines[i].length + 1 // +1 for newline
          }

          // Set cursor position and focus
          textareaRef.value.focus()
          textareaRef.value.setSelectionRange(position, position)

          // Scroll to make the line visible
          const lineHeight = 24 // matches style="min-height: 24px"
          const targetScroll = (errorLine - 1) * lineHeight - (textareaRef.value.clientHeight / 2)
          textareaRef.value.scrollTop = Math.max(0, targetScroll)
        }
      }
    })
  }
})

// Sync scroll between line numbers and textarea
function handleScroll() {
  if (textareaRef.value && lineNumbersRef.value) {
    lineNumbersRef.value.scrollTop = textareaRef.value.scrollTop
  }
}

// Handle escape key to close
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    emit('update:visible', false)
  }
}

// Insert template at cursor
function insertTemplate(template: string) {
  if (textareaRef.value) {
    const start = textareaRef.value.selectionStart
    const end = textareaRef.value.selectionEnd
    const text = props.modelValue
    const newText = text.substring(0, start) + template + text.substring(end)
    emit('update:modelValue', newText)

    // Set cursor after inserted text
    setTimeout(() => {
      if (textareaRef.value) {
        textareaRef.value.selectionStart = textareaRef.value.selectionEnd = start + template.length
        textareaRef.value.focus()
      }
    }, 0)
  }
}

// Common snippets
const snippets = [
  {
    name: 'Echo Request Body',
    code: `// Echo the request body back
response.body = request.body.raw;`
  },
  {
    name: 'JSON Response',
    code: `// Return a JSON response
const data = {
  message: "Hello, World!",
  timestamp: Date.now()
};
response.headers["Content-Type"] = "application/json";
response.body = JSON.stringify(data, null, 2);`
  },
  {
    name: 'Use Path Params',
    code: `// Use path parameters (e.g., /users/:id)
const userId = request.pathParams.id;
response.body = JSON.stringify({
  userId: userId,
  found: true
});`
  },
  {
    name: 'Use Query Params',
    code: `// Use query parameters (e.g., ?page=1&limit=10)
const page = request.queryParams.page ? request.queryParams.page[0] : "1";
const limit = request.queryParams.limit ? request.queryParams.limit[0] : "10";
response.body = JSON.stringify({
  page: parseInt(page),
  limit: parseInt(limit)
});`
  },
  {
    name: 'Conditional Response',
    code: `// Return different responses based on method
if (request.method === "POST") {
  response.status = 201;
  response.body = JSON.stringify({ created: true });
} else {
  response.status = 200;
  response.body = JSON.stringify({ data: [] });
}`
  },
  {
    name: 'Parse JSON Body',
    code: `// Parse and use the JSON request body
const body = request.body.json || {};
const name = body.name || "Unknown";
response.body = JSON.stringify({
  greeting: "Hello, " + name + "!"
});`
  }
]

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <!-- Backdrop -->
      <div class="absolute inset-0 bg-black/70" />

      <!-- Modal -->
      <div class="relative w-[90vw] h-[90vh] bg-gray-800 rounded-lg border border-gray-600 shadow-2xl flex flex-col">
        <!-- Header -->
        <div class="flex items-center justify-between px-4 py-3 border-b border-gray-700">
          <div class="flex items-center gap-3">
            <h3 class="text-lg font-semibold text-white">{{ title || 'Script Editor' }}</h3>
            <span class="px-2 py-0.5 bg-yellow-600 rounded text-xs text-white font-medium">JavaScript</span>
          </div>
          <div class="flex items-center gap-2">
            <button
              @click="showHelp = !showHelp"
              :class="[
                'px-3 py-1.5 rounded text-xs transition-colors',
                showHelp
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
              ]"
            >
              Help
            </button>
            <button
              @click="emit('update:visible', false)"
              class="p-1.5 text-gray-400 hover:text-white transition-colors"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        <!-- Error Display (shown when errorInfo is provided) -->
        <div v-if="errorInfo" class="px-4 py-3 bg-red-900/20 border-b border-red-900/50">
          <div class="flex items-start gap-3">
            <!-- Error Icon -->
            <div class="flex-shrink-0 mt-0.5">
              <svg class="w-5 h-5 text-red-500" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9 9a1 1 0 012 0v4a1 1 0 11-2 0V9zm1-4a1 1 0 100 2 1 1 0 000-2z" clip-rule="evenodd" />
              </svg>
            </div>

            <!-- Error Details -->
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-1">
                <h4 class="text-sm font-semibold text-red-400">Script Execution Error</h4>
                <span class="text-xs text-gray-500">{{ new Date(errorInfo.timestamp).toLocaleString() }}</span>
              </div>
              <div class="flex items-center gap-2 text-xs text-gray-400 mb-2">
                <span class="font-mono">{{ errorInfo.method }}</span>
                <span>→</span>
                <span class="font-mono">{{ errorInfo.path }}</span>
              </div>
              <div class="bg-gray-950 border border-gray-800 rounded p-2">
                <pre class="text-xs text-red-400 font-mono whitespace-pre-wrap break-words">{{ errorInfo.error }}</pre>
              </div>
            </div>
          </div>
        </div>

        <!-- Content -->
        <div class="flex flex-1 overflow-hidden">
          <!-- Editor -->
          <div class="flex-1 flex flex-col p-4 min-w-0">
            <div class="flex-1 flex border border-gray-600 rounded-lg overflow-hidden bg-gray-900">
              <!-- Line Numbers -->
              <div
                ref="lineNumbersRef"
                class="flex flex-col py-2 px-2 bg-gray-800 border-r border-gray-600 text-gray-500 text-sm font-mono select-none overflow-hidden"
              >
                <div
                  v-for="lineNum in lineNumbers"
                  :key="lineNum"
                  class="leading-6 text-right pr-2"
                  style="min-height: 24px"
                >
                  {{ lineNum }}
                </div>
              </div>
              <!-- Code Editor -->
              <textarea
                ref="textareaRef"
                :value="modelValue"
                @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
                @scroll="handleScroll"
                class="flex-1 w-full px-3 py-2 bg-gray-900 text-sm text-white
                       font-mono focus:outline-none resize-none leading-6"
                placeholder="// Write your JavaScript code here...
// Access request data via 'request' object
// Modify response via 'response' object

const userId = request.pathParams.id;
response.status = 200;
response.body = JSON.stringify({ userId: userId });"
                spellcheck="false"
                style="outline: none; border: none;"
              />
            </div>
          </div>

          <!-- Help Panel -->
          <div
            v-if="showHelp"
            class="w-80 border-l border-gray-700 flex flex-col overflow-hidden"
          >
            <div class="flex-1 overflow-y-auto p-4 space-y-4">
              <!-- Request Object -->
              <div>
                <h4 class="text-sm font-semibold text-blue-400 mb-2">request (read-only)</h4>
                <div class="text-xs text-gray-300 font-mono space-y-1 bg-gray-900 rounded p-2">
                  <div><span class="text-purple-400">.method</span> <span class="text-gray-500">// "GET", "POST"</span></div>
                  <div><span class="text-purple-400">.path</span> <span class="text-gray-500">// "/api/users/123"</span></div>
                  <div><span class="text-purple-400">.pathParams</span> <span class="text-gray-500">// {id: "123"}</span></div>
                  <div><span class="text-purple-400">.queryParams</span> <span class="text-gray-500">// {page: ["1"]}</span></div>
                  <div><span class="text-purple-400">.headers</span> <span class="text-gray-500">// {"Content-Type": [...]}</span></div>
                  <div><span class="text-purple-400">.body.raw</span> <span class="text-gray-500">// raw body string</span></div>
                  <div><span class="text-purple-400">.body.json</span> <span class="text-gray-500">// parsed JSON</span></div>
                  <div><span class="text-purple-400">.body.form</span> <span class="text-gray-500">// form data</span></div>
                </div>
              </div>

              <!-- Response Object -->
              <div>
                <h4 class="text-sm font-semibold text-green-400 mb-2">response (writable)</h4>
                <div class="text-xs text-gray-300 font-mono space-y-1 bg-gray-900 rounded p-2">
                  <div><span class="text-purple-400">.status</span> <span class="text-gray-500">// 200</span></div>
                  <div><span class="text-purple-400">.headers</span> <span class="text-gray-500">// {"Content-Type": "..."}</span></div>
                  <div><span class="text-purple-400">.body</span> <span class="text-gray-500">// response body string</span></div>
                  <div><span class="text-purple-400">.delay</span> <span class="text-gray-500">// delay in ms</span></div>
                </div>
              </div>

              <!-- Utilities -->
              <div>
                <h4 class="text-sm font-semibold text-yellow-400 mb-2">Utilities</h4>
                <div class="text-xs text-gray-300 font-mono space-y-1 bg-gray-900 rounded p-2">
                  <div><span class="text-purple-400">JSON.stringify(obj)</span></div>
                  <div><span class="text-purple-400">JSON.parse(str)</span></div>
                  <div><span class="text-purple-400">console.log(...)</span></div>
                </div>
              </div>

              <!-- Snippets -->
              <div>
                <h4 class="text-sm font-semibold text-orange-400 mb-2">Snippets</h4>
                <div class="space-y-1">
                  <button
                    v-for="snippet in snippets"
                    :key="snippet.name"
                    @click="insertTemplate(snippet.code)"
                    class="w-full text-left px-2 py-1.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors"
                  >
                    {{ snippet.name }}
                  </button>
                </div>
              </div>

              <!-- Path Params Note -->
              <div class="bg-blue-900/30 border border-blue-500 rounded p-2">
                <h4 class="text-xs font-semibold text-blue-400 mb-1">Path Parameters</h4>
                <p class="text-xs text-gray-400">
                  Use <code class="text-blue-300">/users/:id</code> or <code class="text-blue-300">/users/{id}</code>
                  in your path pattern. Access via <code class="text-blue-300">request.pathParams.id</code>
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer -->
        <div class="flex items-center justify-between px-4 py-3 border-t border-gray-700">
          <div class="text-xs text-gray-500">
            {{ modelValue.length }} characters | 5s timeout limit
          </div>
          <button
            @click="emit('update:visible', false)"
            class="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-sm text-white font-medium transition-colors"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
