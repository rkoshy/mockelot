<script lang="ts" setup>
import { ref, computed, watch } from 'vue'

export interface DevServerProgressData {
  endpoint_id: string
  stage: string       // 'installing' | 'starting' | 'ready' | 'error' | 'stopped'
  message: string
  progress: number    // 0-100
  error_type?: string
}

const props = defineProps<{
  show: boolean
  endpointName: string
  progress: DevServerProgressData | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

// Auto-close on ready after short delay
watch(() => props.progress?.stage, (stage) => {
  if (stage === 'ready') {
    setTimeout(() => emit('close'), 1500)
  }
})

const stageLabel = computed(() => {
  const s = props.progress?.stage
  if (!s) return 'Starting'
  const labels: Record<string, string> = {
    installing: 'Installing dependencies',
    starting:   'Starting dev server',
    ready:      'Ready',
    error:      'Error',
    stopped:    'Stopped',
  }
  return labels[s] ?? s
})

// Which stage index we're at (for step visualization)
const stages = ['installing', 'starting', 'ready']
const activeStageIndex = computed(() => {
  const s = props.progress?.stage
  if (!s || s === 'stopped') return -1
  if (s === 'error') return -1
  return stages.indexOf(s)
})

function stageClass(idx: number) {
  const current = activeStageIndex.value
  const stage = props.progress?.stage
  if (stage === 'error') return 'text-red-400'
  if (idx < current || stage === 'ready') return 'text-green-400'
  if (idx === current) return 'text-orange-400'
  return 'text-gray-500'
}

function stageIcon(idx: number) {
  const current = activeStageIndex.value
  const stage = props.progress?.stage
  if (stage === 'error' && idx === current) return '✗'
  if (idx < current || stage === 'ready') return '✓'
  if (idx === current) return '●'
  return '○'
}
</script>

<template>
  <div
    v-if="show"
    class="fixed inset-0 bg-black/70 flex items-center justify-center p-4 z-50"
  >
    <div class="bg-gray-800 rounded-xl border border-gray-700 w-[480px] shadow-2xl">
      <!-- Header -->
      <div class="px-6 pt-6 pb-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Dev Server Starting</h3>
        <p class="text-sm text-gray-400 mt-1">{{ endpointName }}</p>
      </div>

      <!-- Stage visualization -->
      <div class="px-6 py-5 space-y-3">
        <div
          v-for="(s, idx) in stages"
          :key="s"
          class="flex items-center gap-3"
        >
          <span class="text-lg font-mono w-5 text-center" :class="stageClass(idx)">
            {{ stageIcon(idx) }}
          </span>
          <span class="text-sm" :class="stageClass(idx)">
            {{ s === 'installing' ? 'Install dependencies' : s === 'starting' ? 'Launch process' : 'Server ready' }}
          </span>
          <!-- spinner for active stage -->
          <svg
            v-if="progress?.stage === s && s !== 'ready'"
            class="w-4 h-4 animate-spin text-orange-400 ml-auto"
            fill="none" viewBox="0 0 24 24"
          >
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
          </svg>
        </div>
      </div>

      <!-- Progress bar -->
      <div class="px-6 pb-2">
        <div class="w-full bg-gray-700 rounded-full h-1.5">
          <div
            class="h-1.5 rounded-full transition-all duration-500"
            :class="progress?.stage === 'error' ? 'bg-red-500' : 'bg-orange-500'"
            :style="{ width: `${progress?.progress ?? 0}%` }"
          />
        </div>
      </div>

      <!-- Message -->
      <div class="px-6 pb-4">
        <p class="text-sm text-gray-300">{{ progress?.message }}</p>
        <p v-if="progress?.stage === 'error'" class="text-xs text-red-400 mt-1">
          Check the Dev Server Console for details.
        </p>
      </div>

      <!-- Footer -->
      <div class="px-6 py-4 border-t border-gray-700 flex justify-end gap-3">
        <button
          v-if="progress?.stage === 'error' || progress?.stage === 'stopped'"
          @click="emit('close')"
          class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded text-sm font-medium"
        >
          Close
        </button>
        <p
          v-else-if="progress?.stage === 'ready'"
          class="text-sm text-green-400 self-center"
        >
          ✓ Server is ready — closing...
        </p>
        <span v-else class="text-xs text-gray-500 self-center">Please wait...</span>
      </div>
    </div>
  </div>
</template>
