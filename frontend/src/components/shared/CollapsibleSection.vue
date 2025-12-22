<template>
  <div class="border border-gray-700 rounded bg-gray-800">
    <button
      @click="toggleOpen"
      class="w-full flex items-center justify-between p-3 text-left hover:bg-gray-750 transition-colors"
    >
      <span class="font-medium text-gray-200">{{ title }}</span>
      <svg
        :class="['w-5 h-5 transform transition-transform', { 'rotate-180': isOpen }]"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </button>

    <div v-if="isOpen" class="p-4 border-t border-gray-700">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

interface Props {
  title: string
  defaultOpen?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  defaultOpen: false,
})

const isOpen = ref(props.defaultOpen)

function toggleOpen() {
  isOpen.value = !isOpen.value
}
</script>
