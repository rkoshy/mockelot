<script lang="ts" setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'

export interface ComboBoxOption {
  value: string | number
  label: string
}

const props = defineProps<{
  modelValue: string | number
  modelText?: string
  options: ComboBoxOption[]
  disabled?: boolean
  placeholder?: string
}>()

const emit = defineEmits<{
  (e: 'update', payload: { value: string | number; text: string }): void
}>()

const isOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
const searchText = ref('')
const isEditing = ref(false)

// Get display text for current value
const displayText = computed(() => {
  if (isEditing.value) {
    return searchText.value
  }
  // If we have modelText (custom text), use that
  if (props.modelText) {
    return `${props.modelValue} - ${props.modelText}`
  }
  // Otherwise look up in options
  const option = props.options.find(opt => opt.value === props.modelValue)
  return option?.label || String(props.modelValue)
})

// Filtered options based on search text
const filteredOptions = computed(() => {
  if (!searchText.value.trim()) {
    return props.options
  }
  const search = searchText.value.toLowerCase()
  return props.options.filter(opt =>
    opt.label.toLowerCase().includes(search) ||
    String(opt.value).includes(search)
  )
})

// Parse custom input like "220 - This is a test"
function parseCustomInput(input: string): { code: number; text: string } | null {
  const match = input.match(/^(\d{3})\s*-\s*(.+)$/)
  if (match) {
    return { code: parseInt(match[1], 10), text: match[2].trim() }
  }
  // Also accept just a number
  const numMatch = input.match(/^(\d{3})$/)
  if (numMatch) {
    const code = parseInt(numMatch[1], 10)
    // Look up standard text
    const option = props.options.find(opt => opt.value === code)
    return { code, text: option ? String(option.label).replace(/^\d+\s*-\s*/, '') : 'Custom' }
  }
  return null
}

function openDropdown(e: MouseEvent) {
  e.stopPropagation()
  if (!props.disabled && !isOpen.value) {
    isOpen.value = true
    isEditing.value = true
    searchText.value = ''
    nextTick(() => {
      inputRef.value?.focus()
    })
  }
}

function closeDropdown() {
  isOpen.value = false
  isEditing.value = false
  searchText.value = ''
}

function selectOption(option: ComboBoxOption) {
  console.log('[ComboBox] selectOption called:', {
    selectedValue: option.value,
    selectedLabel: option.label,
    previousValue: props.modelValue
  })
  emit('update', { value: option.value, text: '' })
  closeDropdown()
}

function handleOptionClick(option: ComboBoxOption, e: MouseEvent) {
  e.stopPropagation()
  e.preventDefault()
  selectOption(option)
}

function handleInputBlur() {
  // Delay to allow click on option to register
  setTimeout(() => {
    if (!isOpen.value) return

    // Check if user typed a custom value
    if (searchText.value.trim()) {
      const parsed = parseCustomInput(searchText.value)
      if (parsed) {
        emit('update', { value: parsed.code, text: parsed.text })
      }
    }
    closeDropdown()
  }, 200)
}

function handleKeyDown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    e.preventDefault()
    const parsed = parseCustomInput(searchText.value)
    if (parsed) {
      emit('update', { value: parsed.code, text: parsed.text })
      closeDropdown()
    } else if (filteredOptions.value.length === 1) {
      // Select the only filtered option
      selectOption(filteredOptions.value[0])
    }
  } else if (e.key === 'Escape') {
    closeDropdown()
  }
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    closeDropdown()
  }
}

// Watch for modelValue changes
watch(() => props.modelValue, (newVal, oldVal) => {
  console.log('[ComboBox] modelValue changed:', {
    from: oldVal,
    to: newVal,
    displayText: displayText.value
  })
})

// Watch for displayText changes
watch(displayText, (newVal, oldVal) => {
  console.log('[ComboBox] displayText recomputed:', {
    from: oldVal,
    to: newVal,
    modelValue: props.modelValue
  })
})

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
  <div ref="dropdownRef" class="relative">
    <!-- Display Button / Input -->
    <div
      v-if="!isEditing"
      @click.stop="openDropdown"
      :class="[
        'w-full flex items-center justify-between gap-2 px-2 py-1.5 bg-gray-800 border border-gray-600 rounded text-sm text-gray-300',
        'cursor-pointer hover:border-gray-500 transition-colors',
        disabled ? 'opacity-50 cursor-not-allowed' : ''
      ]"
    >
      <span class="truncate">{{ displayText }}</span>
      <svg
        class="w-4 h-4 text-gray-400 flex-shrink-0"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </div>

    <!-- Editable Input -->
    <div v-else class="relative" @click.stop>
      <input
        ref="inputRef"
        v-model="searchText"
        type="text"
        :placeholder="'Type code or search (e.g. 220 - Custom)'"
        @blur="handleInputBlur"
        @keydown="handleKeyDown"
        @click.stop
        class="w-full px-2 py-1.5 pr-8 bg-gray-800 border border-blue-500 rounded text-sm text-white
               focus:outline-none placeholder-gray-500"
      />
      <svg
        class="absolute right-2 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
    </div>

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
        @click.stop
        @mousedown.stop
      >
        <!-- Custom value hint -->
        <div v-if="searchText.trim()" class="px-3 py-1.5 text-xs text-gray-500 border-b border-gray-700">
          Press Enter to use custom: "{{ searchText }}"
        </div>

        <div class="py-1">
          <button
            v-for="option in filteredOptions"
            :key="option.value"
            type="button"
            @mousedown.stop.prevent="handleOptionClick(option, $event)"
            class="w-full px-3 py-1.5 text-left text-sm transition-colors"
            :class="[
              option.value === modelValue
                ? 'bg-blue-600 text-white'
                : 'text-gray-300 hover:bg-gray-700'
            ]"
          >
            {{ option.label }}
          </button>

          <!-- No results message -->
          <div
            v-if="filteredOptions.length === 0"
            class="px-3 py-2 text-sm text-gray-500 text-center"
          >
            No matching codes. Type a custom value like "220 - My Status"
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>
