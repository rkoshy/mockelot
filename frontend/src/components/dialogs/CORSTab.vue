<script lang="ts" setup>
import { ref, watch, computed, onMounted } from 'vue'
import { useServerStore } from '../../stores/server'
import CORSHeaderList from './CORSHeaderList.vue'
import CORSScript from './CORSScript.vue'
import StyledSelect from '../shared/StyledSelect.vue'
import { GetCORSConfig } from '../../../wailsjs/go/main/App'
import { models } from '../../types/models'

const emit = defineEmits<{
  validationChange: [valid: boolean]
}>()

const serverStore = useServerStore()

// CORS Configuration
const corsEnabled = ref(false)
const corsMode = ref('headers')
const headerExpressions = ref<Array<{name: string, expression: string}>>([])
const corsScript = ref('')
const optionsStatus = ref(200)

// Validation state
const headerListValid = ref(true)
const scriptValid = ref(true)

// Load CORS config
async function loadCORSConfig() {
  try {
    const config = await GetCORSConfig()
    corsEnabled.value = config.enabled
    corsMode.value = config.mode || 'headers'
    headerExpressions.value = config.header_expressions || []
    corsScript.value = config.script || ''
    optionsStatus.value = config.options_default_status || 200
  } catch (error) {
    console.error('Failed to load CORS config:', error)
  }
}

onMounted(() => {
  loadCORSConfig()
})

// Validation
const isValid = computed(() => {
  if (!corsEnabled.value) return true
  if (corsMode.value === 'headers') {
    return headerListValid.value
  } else {
    return scriptValid.value
  }
})

watch(isValid, (valid) => {
  emit('validationChange', valid)
})

function handleHeaderListValidation(valid: boolean) {
  headerListValid.value = valid
}

function handleScriptValidation(valid: boolean) {
  scriptValid.value = valid
}

function handleHeaderListUpdate(headers: Array<{name: string, expression: string}>) {
  headerExpressions.value = headers
}

function handleScriptUpdate(script: string) {
  corsScript.value = script
}

// Expose config for parent
defineExpose({
  getConfig: () => ({
    enabled: corsEnabled.value,
    mode: corsMode.value,
    header_expressions: headerExpressions.value,
    script: corsScript.value,
    options_default_status: optionsStatus.value,
  })
})

const currentComponent = computed(() => {
  return corsMode.value === 'headers' ? CORSHeaderList : CORSScript
})

// OPTIONS status options
const optionsStatusOptions = computed(() => [
  {
    value: 200,
    label: '200 OK (Default)',
    description: 'Standard success response'
  },
  {
    value: 204,
    label: '204 No Content',
    description: 'Success with no body'
  }
])
</script>

<template>
  <div class="space-y-6">
    <!-- Enable CORS -->
    <div>
      <label class="flex items-center gap-2 cursor-pointer">
        <input
          v-model="corsEnabled"
          type="checkbox"
          class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
        />
        <span class="text-sm font-medium text-white">Enable Global CORS</span>
      </label>
      <p class="mt-1 text-xs text-gray-400 ml-6">
        Apply CORS headers to all responses (can be overridden per-entry or per-group)
      </p>
    </div>

    <div v-if="corsEnabled" class="space-y-6">
      <!-- CORS Mode Tabs -->
      <div>
        <div class="flex border-b border-gray-700">
          <button
            @click="corsMode = 'headers'"
            :class="[
              'px-4 py-2 text-sm font-medium border-b-2 transition-colors',
              corsMode === 'headers'
                ? 'border-blue-500 text-blue-500'
                : 'border-transparent text-gray-400 hover:text-gray-300'
            ]"
          >
            Header List
          </button>
          <button
            @click="corsMode = 'script'"
            :class="[
              'px-4 py-2 text-sm font-medium border-b-2 transition-colors',
              corsMode === 'script'
                ? 'border-blue-500 text-blue-500'
                : 'border-transparent text-gray-400 hover:text-gray-300'
            ]"
          >
            Custom Script
          </button>
        </div>

        <!-- Mode Description -->
        <div class="mt-3 p-3 bg-gray-700/50 rounded">
          <p v-if="corsMode === 'headers'" class="text-xs text-gray-300">
            Define CORS headers with JavaScript expressions evaluated per-request.
          </p>
          <p v-else class="text-xs text-gray-300">
            Use custom JavaScript to set CORS headers with full request context.
          </p>
        </div>
      </div>

      <!-- Mode Content -->
      <div>
        <component
          :is="currentComponent"
          :initial-headers="headerExpressions"
          :initial-script="corsScript"
          @validation-change="corsMode === 'headers' ? handleHeaderListValidation($event) : handleScriptValidation($event)"
          @update:headers="handleHeaderListUpdate"
          @update:script="handleScriptUpdate"
        />
      </div>

      <!-- OPTIONS Response Status -->
      <div class="border-t border-gray-700 pt-6">
        <h4 class="text-sm font-semibold text-white mb-3">OPTIONS Preflight Response</h4>

        <label class="block text-sm font-medium text-gray-300 mb-2">
          Default Status Code
        </label>
        <div class="w-64">
          <StyledSelect
            v-model="optionsStatus"
            :options="optionsStatusOptions"
          />
        </div>
        <p class="mt-2 text-xs text-gray-400">
          Status code returned for CORS preflight OPTIONS requests
        </p>
      </div>

      <!-- Info -->
      <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
        <p class="text-sm font-medium text-blue-300 mb-2">CORS Precedence</p>
        <p class="text-xs text-blue-300">
          Explicit OPTIONS handler > Per-entry override > Per-group override > Global CORS
        </p>
      </div>
    </div>
  </div>
</template>
