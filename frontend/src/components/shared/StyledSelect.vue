<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

export interface SelectOption {
  value: string | number
  label: string
  description?: string
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

// Get display text for current value
const displayText = computed(() => {
  const option = props.options.find(opt => opt.value === props.modelValue)
  return option?.label || props.placeholder || 'Select an option'
})

function toggleDropdown(e: MouseEvent) {
  e.stopPropagation()
  if (!props.disabled) {
    isOpen.value = !isOpen.value
  }
}

function closeDropdown() {
  isOpen.value = false
}

function selectOption(option: SelectOption, e: MouseEvent) {
  e.stopPropagation()
  e.preventDefault()
  emit('update:modelValue', option.value)
  closeDropdown()
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    closeDropdown()
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
    <!-- Display Button -->
    <button
      type="button"
      @click="toggleDropdown"
      :disabled="disabled"
      :class="[
        'w-full flex items-center justify-between gap-2 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm',
        'transition-colors focus:outline-none',
        disabled ? 'opacity-50 cursor-not-allowed' : 'hover:border-gray-500 focus:border-blue-500',
        isOpen ? 'border-blue-500' : ''
      ]"
    >
      <span class="truncate text-left">{{ displayText }}</span>
      <svg
        :class="[
          'w-4 h-4 text-gray-400 flex-shrink-0 transition-transform',
          isOpen ? 'rotate-180' : ''
        ]"
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
        class="absolute z-50 mt-1 w-full rounded-md bg-gray-800 border border-gray-600 shadow-lg overflow-hidden"
        @click.stop
        @mousedown.stop
      >
        <div class="py-1">
          <button
            v-for="option in options"
            :key="option.value"
            type="button"
            @mousedown.stop.prevent="selectOption(option, $event)"
            class="w-full px-3 py-2 text-left text-sm transition-colors"
            :class="[
              option.value === modelValue
                ? 'bg-blue-600 text-white'
                : 'text-gray-300 hover:bg-gray-700'
            ]"
          >
            <div class="font-medium">{{ option.label }}</div>
            <div v-if="option.description" class="text-xs opacity-75 mt-0.5">
              {{ option.description }}
            </div>
          </button>
        </div>
      </div>
    </Transition>
  </div>
</template>
