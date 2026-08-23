<script lang="ts" setup>
import { ref, watch } from 'vue'
import { models } from '../../../wailsjs/go/models'

const props = defineProps<{
  modelValue: models.TranslationRule[]
}>()

const emit = defineEmits<{
  'update:modelValue': [rules: models.TranslationRule[]]
}>()

const rules = ref<models.TranslationRule[]>(props.modelValue?.map(r => ({ ...r })) ?? [])

watch(() => props.modelValue, (v) => {
  rules.value = v?.map(r => ({ ...r })) ?? []
}, { deep: true })

function emitUpdate() {
  emit('update:modelValue', rules.value.map(r => new models.TranslationRule({ ...r })))
}

function addRule() {
  rules.value.push(new models.TranslationRule({ pattern: '', replace: '' }))
  emitUpdate()
}

function removeRule(i: number) {
  rules.value.splice(i, 1)
  emitUpdate()
}

function moveUp(i: number) {
  if (i === 0) return
  ;[rules.value[i - 1], rules.value[i]] = [rules.value[i], rules.value[i - 1]]
  emitUpdate()
}

function moveDown(i: number) {
  if (i >= rules.value.length - 1) return
  ;[rules.value[i], rules.value[i + 1]] = [rules.value[i + 1], rules.value[i]]
  emitUpdate()
}
</script>

<template>
  <div class="space-y-2">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <span class="text-sm font-medium text-gray-300">Translation Rules</span>
        <p class="text-xs text-gray-500 mt-0.5">
          Tried in order — first matching regex wins.
          Use <code class="text-gray-400">$1</code>, <code class="text-gray-400">$2</code> … for capture groups.
        </p>
      </div>
      <button
        @click="addRule"
        class="flex items-center gap-1 px-2 py-1 text-xs bg-gray-700 hover:bg-gray-600 text-blue-400 hover:text-blue-300 rounded transition-colors"
      >
        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        Add Rule
      </button>
    </div>

    <!-- Empty state -->
    <div v-if="rules.length === 0" class="py-4 text-center text-xs text-gray-500 border border-dashed border-gray-700 rounded">
      No rules — add one to start. All requests will pass through unchanged.
    </div>

    <!-- Rule rows -->
    <div
      v-for="(rule, i) in rules"
      :key="i"
      class="bg-gray-900 border border-gray-700 rounded p-2 space-y-2"
    >
      <!-- Row header: index + reorder + delete -->
      <div class="flex items-center gap-1">
        <span class="text-[10px] text-gray-600 font-mono w-4 text-center select-none">{{ i + 1 }}</span>
        <span class="flex-1 text-[10px] text-gray-500">
          {{ rule.pattern ? 'if path matches' : '(no pattern)' }}
        </span>
        <!-- Reorder -->
        <button @click="moveUp(i)" :disabled="i === 0" class="p-0.5 rounded text-gray-500 hover:text-gray-200 disabled:opacity-30 transition-colors" title="Move up">
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" /></svg>
        </button>
        <button @click="moveDown(i)" :disabled="i === rules.length - 1" class="p-0.5 rounded text-gray-500 hover:text-gray-200 disabled:opacity-30 transition-colors" title="Move down">
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
        </button>
        <!-- Delete -->
        <button @click="removeRule(i)" class="p-0.5 rounded text-red-500 hover:text-red-400 hover:bg-red-900/30 transition-colors" title="Remove rule">
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
        </button>
      </div>

      <!-- Pattern input -->
      <div>
        <label class="block text-xs text-gray-500 mb-1">Match Pattern</label>
        <input
          v-model="rule.pattern"
          @input="emitUpdate"
          type="text"
          placeholder="e.g., ^/iris([^/]*)/([a-zA-Z0-9_-]+)$"
          class="w-full px-2 py-1.5 bg-gray-800 border border-gray-600 rounded text-xs text-white
                 placeholder-gray-600 focus:outline-none focus:border-blue-500 font-mono"
        />
      </div>

      <!-- Replace input -->
      <div>
        <label class="block text-xs text-gray-500 mb-1">Replace With</label>
        <input
          v-model="rule.replace"
          @input="emitUpdate"
          type="text"
          placeholder="e.g., pages/$2.shtml"
          class="w-full px-2 py-1.5 bg-gray-800 border border-gray-600 rounded text-xs text-white
                 placeholder-gray-600 focus:outline-none focus:border-blue-500 font-mono"
        />
      </div>
    </div>
  </div>
</template>
