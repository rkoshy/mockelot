<script lang="ts" setup>
import { ref, watch } from 'vue'

const props = defineProps<{
  show: boolean
  title: string
  message: string
  primaryText?: string
  secondaryText?: string
  cancelText?: string
}>()

const emit = defineEmits<{
  primary: []
  secondary: []
  cancel: []
}>()

function handlePrimary() {
  emit('primary')
}

function handleSecondary() {
  emit('secondary')
}

function handleCancel() {
  emit('cancel')
}

// Close on Escape key
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    handleCancel()
  }
}

watch(() => props.show, (newVal) => {
  if (newVal) {
    window.addEventListener('keydown', handleKeydown)
  } else {
    window.removeEventListener('keydown', handleKeydown)
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
        @click.self="handleCancel"
      >
        <div class="bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4 border border-gray-700">
          <!-- Header -->
          <div class="px-6 py-4 border-b border-gray-700">
            <h3 class="text-lg font-semibold text-white">{{ title }}</h3>
          </div>

          <!-- Body -->
          <div class="px-6 py-4">
            <p class="text-gray-300 whitespace-pre-line">{{ message }}</p>
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-gray-700 flex justify-end gap-3">
            <button
              @click="handleCancel"
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors"
            >
              {{ cancelText || 'Cancel' }}
            </button>
            <button
              v-if="secondaryText"
              @click="handleSecondary"
              class="px-4 py-2 bg-orange-600 hover:bg-orange-700 text-white rounded transition-colors"
            >
              {{ secondaryText }}
            </button>
            <button
              @click="handlePrimary"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors"
            >
              {{ primaryText || 'Confirm' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-active .bg-gray-800,
.modal-leave-active .bg-gray-800 {
  transition: transform 0.2s ease;
}

.modal-enter-from .bg-gray-800,
.modal-leave-to .bg-gray-800 {
  transform: scale(0.95);
}
</style>
