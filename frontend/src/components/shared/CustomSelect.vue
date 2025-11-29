<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

export interface SelectOption {
  value: string | number
  label: string
}

const props = defineProps<{
  modelValue: string | number
  options: SelectOption[]
  disabled?: boolean
  placeholder?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string | number): void
}>()

const isOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

const selectedOption = computed(() =>
  props.options.find(opt => opt.value === props.modelValue)
)

function toggle() {
  if (!props.disabled) {
    isOpen.value = !isOpen.value
  }
}

function select(option: SelectOption) {
  emit('update:modelValue', option.value)
  isOpen.value = false
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
  <div ref="dropdownRef" class="relative">
    <!-- Trigger Button -->
    <button
      type="button"
      @click="toggle"
      :disabled="disabled"
      class="w-full flex items-center justify-between gap-2 px-2 py-1.5 bg-gray-800 border border-gray-600 rounded text-sm text-gray-300
             focus:outline-none focus:border-blue-500 disabled:opacity-50 disabled:cursor-not-allowed
             hover:border-gray-500 transition-colors"
    >
      <span class="truncate">
        {{ selectedOption?.label || placeholder || 'Select...' }}
      </span>
      <svg
        class="w-4 h-4 text-gray-400 flex-shrink-0 transition-transform"
        :class="{ 'rotate-180': isOpen }"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </button>

    <!-- Dropdown Menu -->
    <Transition
      enter-active-class="transition duration-100 ease-out"
      enter-from-class="transform scale-95 opacity-0"
      enter-to-class="transform scale-100 opacity-100"
      leave-active-class="transition duration-75 ease-in"
      leave-from-class="transform scale-100 opacity-100"
      leave-to-class="transform scale-95 opacity-0"
    >
      <div
        v-if="isOpen"
        class="absolute z-50 mt-1 w-full max-h-60 overflow-auto rounded-md bg-gray-800 border border-gray-600 shadow-lg"
      >
        <div class="py-1">
          <button
            v-for="option in options"
            :key="option.value"
            type="button"
            @click="select(option)"
            class="w-full px-3 py-1.5 text-left text-sm transition-colors"
            :class="[
              option.value === modelValue
                ? 'bg-blue-600 text-white'
                : 'text-gray-300 hover:bg-gray-700'
            ]"
          >
            {{ option.label }}
          </button>
        </div>
      </div>
    </Transition>
  </div>
</template>
