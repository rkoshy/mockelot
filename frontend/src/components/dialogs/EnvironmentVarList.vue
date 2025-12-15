<script lang="ts" setup>
import { ref, watch } from 'vue'
import { models } from '../../../wailsjs/go/models'

const props = defineProps<{
  modelValue: models.EnvironmentVar[]
}>()

const emit = defineEmits<{
  'update:modelValue': [envVars: models.EnvironmentVar[]]
}>()

interface EnvVarRow extends models.EnvironmentVar {
  id: string
}

const envVars = ref<EnvVarRow[]>([])

// Initialize with props
if (props.modelValue && props.modelValue.length > 0) {
  envVars.value = props.modelValue.map((e, i) => ({
    ...e,
    id: `env-${i}-${Date.now()}`
  }))
}

// Add new environment variable row
function addEnvVar() {
  envVars.value.push({
    id: `env-${envVars.value.length}-${Date.now()}`,
    name: '',
    value: '',
    expression: ''
  })
  emitEnvVars()
}

// Remove environment variable row
function removeEnvVar(index: number) {
  envVars.value.splice(index, 1)
  emitEnvVars()
}

// Emit environment variables update
function emitEnvVars() {
  const validEnvVars = envVars.value
    .filter(e => e.name.trim() !== '')
    .map(({ id, ...e }) => e)
  emit('update:modelValue', validEnvVars)
}

// Watch for changes
watch(envVars, () => {
  emitEnvVars()
}, { deep: true })
</script>

<template>
  <div class="space-y-4">
    <!-- Environment Variables Table -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <h4 class="text-sm font-medium text-white">Environment Variables</h4>
        <button
          @click="addEnvVar"
          class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded transition-colors flex items-center gap-1"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Add Variable
        </button>
      </div>

      <!-- Environment Variable Rows -->
      <div v-if="envVars.length > 0" class="space-y-3">
        <div
          v-for="(envVar, index) in envVars"
          :key="envVar.id"
          class="flex gap-2 items-start p-3 bg-gray-700/50 rounded border border-gray-600"
        >
          <div class="flex-1 space-y-2">
            <!-- Variable Name -->
            <div>
              <label class="block text-xs text-gray-400 mb-1">Variable Name</label>
              <input
                v-model="envVar.name"
                type="text"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono
                       focus:outline-none focus:border-blue-500"
                placeholder="e.g., DATABASE_URL, API_KEY"
              />
            </div>

            <!-- Value Mode Selection -->
            <div class="flex gap-4">
              <label class="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  :name="`mode-${envVar.id}`"
                  :checked="!envVar.expression || envVar.expression === ''"
                  @change="envVar.expression = ''"
                  class="text-blue-600 focus:ring-2 focus:ring-blue-500"
                />
                <span class="text-xs text-gray-300">Static Value</span>
              </label>
              <label class="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  :name="`mode-${envVar.id}`"
                  :checked="!!(envVar.expression && envVar.expression !== '')"
                  @change="envVar.expression = envVar.expression || ''"
                  class="text-blue-600 focus:ring-2 focus:ring-blue-500"
                />
                <span class="text-xs text-gray-300">JavaScript Expression</span>
              </label>
            </div>

            <!-- Static Value -->
            <div v-if="!envVar.expression || envVar.expression === ''">
              <label class="block text-xs text-gray-400 mb-1">Value</label>
              <input
                v-model="envVar.value"
                type="text"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono
                       focus:outline-none focus:border-blue-500"
                placeholder="Static value"
              />
            </div>

            <!-- JavaScript Expression -->
            <div v-else>
              <label class="block text-xs text-gray-400 mb-1">JavaScript Expression</label>
              <input
                v-model="envVar.expression"
                type="text"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono
                       focus:outline-none focus:border-blue-500"
                placeholder="e.g., new Date().toISOString()"
              />
            </div>
          </div>

          <!-- Remove Button -->
          <button
            @click="removeEnvVar(index)"
            class="p-2 text-gray-400 hover:text-red-400 transition-colors mt-6"
            title="Remove environment variable"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <div v-else class="text-center py-8 text-gray-400 text-sm">
        No environment variables. Click "Add Variable" to create one.
      </div>
    </div>

    <!-- Helper Info -->
    <div class="p-4 bg-gray-700/50 rounded border border-gray-600">
      <p class="text-sm font-medium text-white mb-2">Environment Variables</p>
      <div class="space-y-2 text-xs text-gray-300">
        <p><span class="text-green-400 font-medium">Static Value:</span> Use a fixed string value</p>
        <p><span class="text-purple-400 font-medium">JavaScript Expression:</span> Dynamically compute value when container starts</p>
      </div>
    </div>

    <!-- Examples -->
    <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
      <p class="text-sm font-medium text-blue-300 mb-2">Example Use Cases</p>
      <div class="space-y-3 text-xs text-blue-200">
        <div>
          <p class="font-medium text-blue-300">Static configuration:</p>
          <p class="font-mono text-gray-300 mt-1">Name: NODE_ENV</p>
          <p class="font-mono text-gray-300">Value: production</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">API credentials:</p>
          <p class="font-mono text-gray-300 mt-1">Name: API_KEY</p>
          <p class="font-mono text-gray-300">Value: sk-1234567890abcdef</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">Dynamic timestamp:</p>
          <p class="font-mono text-gray-300 mt-1">Name: START_TIME</p>
          <p class="font-mono text-gray-300">Expression: new Date().toISOString()</p>
          <p class="text-gray-400 mt-1">Computed when container starts</p>
        </div>
      </div>
    </div>
  </div>
</template>
