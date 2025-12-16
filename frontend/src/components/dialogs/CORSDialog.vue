<script lang="ts" setup>
import { ref, watch } from 'vue'
import CORSTab from './CORSTab.vue'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  close: []
  apply: [config: any]
}>()

const corsTabRef = ref<InstanceType<typeof CORSTab> | null>(null)
const isValid = ref(true)

function handleValidationChange(valid: boolean) {
  isValid.value = valid
}

function handleClose() {
  emit('close')
}

function handleApply() {
  if (corsTabRef.value?.getConfig) {
    const config = corsTabRef.value.getConfig()
    emit('apply', config)
  }
}

// Close on Escape key
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    handleClose()
  }
}

watch(() => props.show, (show) => {
  if (show) {
    document.addEventListener('keydown', handleKeydown)
  } else {
    document.removeEventListener('keydown', handleKeydown)
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="dialog">
      <div
        v-if="show"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      >
        <div class="bg-gray-800 rounded-lg shadow-xl w-full max-w-4xl max-h-[90vh] overflow-hidden flex flex-col">
          <!-- Header -->
          <div class="flex items-center justify-between px-6 py-4 border-b border-gray-700">
            <h2 class="text-xl font-semibold text-white">CORS Configuration</h2>
            <button
              @click="handleClose"
              class="p-1 hover:bg-gray-700 rounded transition-colors text-gray-400 hover:text-white"
            >
              <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- Content -->
          <div class="flex-1 overflow-y-auto px-6 py-6">
            <CORSTab
              ref="corsTabRef"
              @validation-change="handleValidationChange"
            />
          </div>

          <!-- Footer -->
          <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-700">
            <button
              @click="handleClose"
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded transition-colors"
            >
              Cancel
            </button>
            <button
              @click="handleApply"
              :disabled="!isValid"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Apply
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dialog-enter-active,
.dialog-leave-active {
  transition: opacity 0.2s ease;
}

.dialog-enter-from,
.dialog-leave-to {
  opacity: 0;
}

.dialog-enter-active .bg-gray-800,
.dialog-leave-active .bg-gray-800 {
  transition: transform 0.2s ease;
}

.dialog-enter-from .bg-gray-800,
.dialog-leave-to .bg-gray-800 {
  transform: scale(0.95);
}
</style>
