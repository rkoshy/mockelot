<script lang="ts" setup>
import { ref, watch } from 'vue'
import type { models } from '../../../wailsjs/go/models'

const props = defineProps<{
  modelValue: models.StatusTranslation[]
}>()

const emit = defineEmits<{
  'update:modelValue': [translations: models.StatusTranslation[]]
}>()

interface TranslationRow extends models.StatusTranslation {
  id: string
}

const translations = ref<TranslationRow[]>([])

// Initialize with props
if (props.modelValue && props.modelValue.length > 0) {
  translations.value = props.modelValue.map((t, i) => ({
    ...t,
    id: `trans-${i}-${Date.now()}`
  }))
}

// Add new translation row
function addTranslation() {
  translations.value.push({
    id: `trans-${translations.value.length}-${Date.now()}`,
    from_pattern: '',
    to_code: 200
  })
  emitTranslations()
}

// Remove translation row
function removeTranslation(index: number) {
  translations.value.splice(index, 1)
  emitTranslations()
}

// Emit translations update
function emitTranslations() {
  const validTranslations = translations.value
    .filter(t => t.from_pattern.trim() !== '' && t.to_code > 0)
    .map(({ id, ...t }) => t)
  emit('update:modelValue', validTranslations)
}

// Watch for changes
watch(translations, () => {
  emitTranslations()
}, { deep: true })
</script>

<template>
  <div class="space-y-4">
    <!-- Translations Table -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <h4 class="text-sm font-medium text-white">Status Code Translation Rules</h4>
        <button
          @click="addTranslation"
          class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded transition-colors flex items-center gap-1"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Add Rule
        </button>
      </div>

      <!-- Translation Rows -->
      <div v-if="translations.length > 0" class="space-y-3">
        <div
          v-for="(trans, index) in translations"
          :key="trans.id"
          class="flex gap-2 items-center p-3 bg-gray-700/50 rounded border border-gray-600"
        >
          <!-- From Pattern -->
          <div class="flex-1">
            <label class="block text-xs text-gray-400 mb-1">From Pattern</label>
            <input
              v-model="trans.from_pattern"
              type="text"
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono
                     focus:outline-none focus:border-blue-500"
              placeholder="e.g., 5xx, 404, 2xx"
            />
          </div>

          <!-- Arrow -->
          <div class="pt-5">
            <svg class="w-6 h-6 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3" />
            </svg>
          </div>

          <!-- To Code -->
          <div class="flex-1">
            <label class="block text-xs text-gray-400 mb-1">To Status Code</label>
            <input
              v-model.number="trans.to_code"
              type="number"
              min="100"
              max="599"
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono
                     focus:outline-none focus:border-blue-500"
              placeholder="200"
            />
          </div>

          <!-- Remove Button -->
          <button
            @click="removeTranslation(index)"
            class="p-2 text-gray-400 hover:text-red-400 transition-colors mt-5"
            title="Remove translation rule"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <div v-else class="text-center py-8 text-gray-400 text-sm">
        No status translation rules. Backend status codes will pass through unchanged.
      </div>
    </div>

    <!-- Helper Info -->
    <div class="p-4 bg-gray-700/50 rounded border border-gray-600">
      <p class="text-sm font-medium text-white mb-2">Pattern Syntax</p>
      <div class="space-y-2 text-xs text-gray-300">
        <p><span class="text-blue-400 font-mono">404</span> - Exact status code match</p>
        <p><span class="text-blue-400 font-mono">5xx</span> - Wildcard match (500-599)</p>
        <p><span class="text-blue-400 font-mono">2xx</span> - Wildcard match (200-299)</p>
        <p><span class="text-blue-400 font-mono">4xx</span> - Wildcard match (400-499)</p>
      </div>

      <p class="text-sm font-medium text-white mt-4 mb-2">How It Works</p>
      <p class="text-xs text-gray-300">
        Rules are evaluated in order. The first matching pattern determines the translated status code.
        If no rules match, the original status code passes through.
      </p>
    </div>

    <!-- Examples -->
    <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
      <p class="text-sm font-medium text-blue-300 mb-2">Example Use Cases</p>
      <div class="space-y-3 text-xs text-blue-200">
        <div>
          <p class="font-medium text-blue-300">Hide backend errors from clients:</p>
          <p class="font-mono text-gray-300 mt-1">Pattern: 5xx → Status: 403</p>
          <p class="text-gray-400 mt-1">Translates all 500-level errors to 403 Forbidden</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">Convert specific errors:</p>
          <p class="font-mono text-gray-300 mt-1">Pattern: 404 → Status: 200</p>
          <p class="text-gray-400 mt-1">Makes not-found responses appear successful</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">Normalize client errors:</p>
          <p class="font-mono text-gray-300 mt-1">Pattern: 4xx → Status: 400</p>
          <p class="text-gray-400 mt-1">Converts all client errors to generic 400</p>
        </div>
      </div>
    </div>
  </div>
</template>
