<script lang="ts" setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { COMMON_CONTENT_TYPES } from '../../utils/formatter'

const props = defineProps<{
  modelValue: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'clear'): void
}>()

const isOpen = ref(false)
const inputRef = ref<HTMLInputElement | null>(null)
const dropdownRef = ref<HTMLDivElement | null>(null)

// Filter options based on input
const filteredOptions = computed(() => {
  const search = props.modelValue.toLowerCase()
  if (!search) return COMMON_CONTENT_TYPES
  return COMMON_CONTENT_TYPES.filter(
    opt => opt.value.toLowerCase().includes(search) || opt.label.toLowerCase().includes(search)
  )
})

function selectOption(value: string) {
  emit('update:modelValue', value)
  isOpen.value = false
}

function handleInput(e: Event) {
  const target = e.target as HTMLInputElement
  emit('update:modelValue', target.value)
  isOpen.value = true
}

function handleFocus() {
  isOpen.value = true
}

function handleClear() {
  emit('clear')
  isOpen.value = false
}

// Close dropdown when clicking outside
function handleClickOutside(e: MouseEvent) {
  if (
    dropdownRef.value &&
    !dropdownRef.value.contains(e.target as Node) &&
    inputRef.value &&
    !inputRef.value.contains(e.target as Node)
  ) {
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
  <div class="relative">
    <div class="flex gap-1">
      <div class="relative flex-1">
        <input
          ref="inputRef"
          :value="modelValue"
          @input="handleInput"
          @focus="handleFocus"
          :disabled="disabled"
          type="text"
          placeholder="Select or type content type..."
          class="w-full px-2 py-1.5 pr-8 bg-gray-900 border border-gray-600 rounded text-xs text-white
                 focus:outline-none focus:border-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
        />
        <!-- Dropdown arrow -->
        <button
          @click="isOpen = !isOpen"
          :disabled="disabled"
          class="absolute right-1 top-1/2 -translate-y-1/2 p-1 text-gray-400 hover:text-white disabled:opacity-50"
        >
          <svg class="w-3 h-3" :class="{ 'rotate-180': isOpen }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
      </div>
      <!-- Clear button -->
      <button
        v-if="modelValue"
        @click="handleClear"
        :disabled="disabled"
        class="px-2 py-1.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 transition-colors disabled:opacity-50"
        title="Clear content type and body"
      >
        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Dropdown -->
    <div
      v-if="isOpen && !disabled"
      ref="dropdownRef"
      class="absolute z-50 w-full mt-1 bg-gray-800 border border-gray-600 rounded-lg shadow-xl max-h-48 overflow-y-auto"
    >
      <div
        v-for="opt in filteredOptions"
        :key="opt.value"
        @click="selectOption(opt.value)"
        class="px-3 py-2 cursor-pointer hover:bg-gray-700 transition-colors"
        :class="{ 'bg-blue-900/30': opt.value === modelValue }"
      >
        <div class="text-xs text-white">{{ opt.label }}</div>
        <div class="text-[10px] text-gray-400 font-mono">{{ opt.value }}</div>
      </div>
      <div v-if="filteredOptions.length === 0" class="px-3 py-2 text-xs text-gray-500">
        No matching types. You can type a custom value.
      </div>
    </div>
  </div>
</template>
