<script lang="ts" setup>
import { ref, onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  modelValue: string
  visible: boolean
  title?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'update:visible', value: boolean): void
}>()

const showHelp = ref(true)
const textareaRef = ref<HTMLTextAreaElement | null>(null)

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
      @click.self="emit('update:visible', false)"
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

        <!-- Content -->
        <div class="flex flex-1 overflow-hidden">
          <!-- Editor -->
          <div class="flex-1 flex flex-col p-4 min-w-0">
            <textarea
              ref="textareaRef"
              :value="modelValue"
              @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
              class="flex-1 w-full px-3 py-2 bg-gray-900 border border-gray-600 rounded-lg text-sm text-white
                     font-mono focus:outline-none focus:border-blue-500 resize-none"
              placeholder="// Write your JavaScript code here...
// Access request data via 'request' object
// Modify response via 'response' object

const userId = request.pathParams.id;
response.status = 200;
response.body = JSON.stringify({ userId: userId });"
              spellcheck="false"
            />
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
