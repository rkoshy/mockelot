<template>
  <div class="domain-filter-input">
    <!-- Selected domain chips -->
    <div v-if="modelValue.length > 0" class="selected-chips mb-2">
      <span
        v-for="domain in modelValue"
        :key="domain"
        class="chip"
      >
        <span class="chip-text">{{ domain }}</span>
        <button
          type="button"
          @click="removeDomain(domain)"
          class="chip-remove"
          title="Remove domain"
        >
          ×
        </button>
      </span>
    </div>

    <!-- Input with autocomplete -->
    <div class="input-wrapper" ref="wrapperRef">
      <input
        ref="inputRef"
        v-model="inputValue"
        @input="handleInput"
        @focus="showDropdown = true"
        @blur="handleBlur"
        @keydown.enter.prevent="handleEnter"
        @keydown.escape="handleEscape"
        @keydown.down.prevent="navigateDown"
        @keydown.up.prevent="navigateUp"
        type="text"
        placeholder="Type or select domain..."
        class="domain-input"
      />

      <!-- Autocomplete dropdown -->
      <div
        v-if="showDropdown"
        class="dropdown"
      >
        <!-- SOCKS5 domains section -->
        <div v-if="filteredSOCKS5Domains.length > 0" class="dropdown-section">
          <div class="section-label">SOCKS5 Domains</div>
          <div
            v-for="(domain, index) in filteredSOCKS5Domains"
            :key="domain"
            @mousedown.prevent="addDomain(domain)"
            @mouseenter="highlightedIndex = index"
            :class="['dropdown-item', { highlighted: highlightedIndex === index }]"
          >
            <span class="domain-text">{{ domain }}</span>
            <span v-if="!modelValue.includes(domain)" class="add-hint">Click to add</span>
            <span v-else class="added-hint">✓ Added</span>
          </div>
        </div>

        <!-- Custom domain hint -->
        <div v-if="inputValue.trim()" class="dropdown-hint">
          <div v-if="filteredSOCKS5Domains.length > 0" class="dropdown-divider"></div>
          <div class="custom-hint">
            <span class="hint-text">Press Enter to add:</span>
            <span class="hint-domain">{{ inputValue.trim() }}</span>
          </div>
        </div>

        <!-- Empty state when no SOCKS5 domains -->
        <div v-if="filteredSOCKS5Domains.length === 0 && !inputValue.trim()" class="empty-state">
          <div class="empty-text">No SOCKS5 domains configured</div>
          <div class="empty-subtext">Type a custom domain and press Enter</div>
        </div>
      </div>
    </div>

    <!-- Helper text -->
    <p class="helper-text">
      Supports exact matches (api.example.com) and wildcards (*.example.com)
    </p>
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted } from 'vue'
import { GetSOCKS5Config } from '../../../wailsjs/go/main/App'

const props = defineProps<{
  modelValue: string[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const inputValue = ref('')
const showDropdown = ref(false)
const highlightedIndex = ref(0)
const socks5Domains = ref<string[]>([])
const inputRef = ref<HTMLInputElement | null>(null)
const wrapperRef = ref<HTMLDivElement | null>(null)

// Load SOCKS5 domains on mount
onMounted(async () => {
  try {
    const config = await GetSOCKS5Config()
    if (config.domain_takeover?.domains) {
      // Extract patterns from enabled domains only
      socks5Domains.value = config.domain_takeover.domains
        .filter(d => d.enabled)
        .map(d => d.pattern)
    }
  } catch (error) {
    console.error('Failed to load SOCKS5 config:', error)
  }
})

// Filter SOCKS5 domains based on input and exclude already selected
const filteredSOCKS5Domains = computed(() => {
  const input = inputValue.value.toLowerCase().trim()
  return socks5Domains.value
    .filter(domain => {
      // Don't show already selected domains
      if (props.modelValue.includes(domain)) return false
      // Filter by input text if any
      if (input) return domain.toLowerCase().includes(input)
      return true
    })
    .slice(0, 10) // Limit to 10 results
})

function addDomain(domain: string) {
  const trimmed = domain.trim()
  if (trimmed && !props.modelValue.includes(trimmed)) {
    emit('update:modelValue', [...props.modelValue, trimmed])
    inputValue.value = ''
    highlightedIndex.value = 0
    // Keep focus on input after adding
    inputRef.value?.focus()
  }
}

function removeDomain(domain: string) {
  emit('update:modelValue', props.modelValue.filter(d => d !== domain))
}

function handleInput() {
  showDropdown.value = true
  highlightedIndex.value = 0
}

function handleBlur() {
  // Delay to allow mousedown on dropdown items to fire
  setTimeout(() => {
    showDropdown.value = false
  }, 200)
}

function handleEnter() {
  if (highlightedIndex.value >= 0 && filteredSOCKS5Domains.value[highlightedIndex.value]) {
    // Add highlighted SOCKS5 domain
    addDomain(filteredSOCKS5Domains.value[highlightedIndex.value])
  } else if (inputValue.value.trim()) {
    // Add custom domain
    addDomain(inputValue.value)
  }
}

function handleEscape() {
  showDropdown.value = false
  inputValue.value = ''
}

function navigateDown() {
  if (filteredSOCKS5Domains.value.length > 0) {
    highlightedIndex.value = Math.min(
      highlightedIndex.value + 1,
      filteredSOCKS5Domains.value.length - 1
    )
  }
}

function navigateUp() {
  highlightedIndex.value = Math.max(highlightedIndex.value - 1, 0)
}
</script>

<style scoped>
.domain-filter-input {
  position: relative;
}

.selected-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.375rem 0.5rem;
  background-color: rgb(59 130 246 / 0.2);
  border: 1px solid rgb(59 130 246 / 0.4);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  color: rgb(147 197 253);
  transition: all 0.2s;
}

.chip:hover {
  background-color: rgb(59 130 246 / 0.3);
  border-color: rgb(59 130 246 / 0.6);
}

.chip-text {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.chip-remove {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  padding: 0;
  background-color: transparent;
  border: none;
  border-radius: 0.25rem;
  color: rgb(147 197 253);
  font-size: 1.25rem;
  line-height: 1;
  cursor: pointer;
  transition: all 0.2s;
}

.chip-remove:hover {
  background-color: rgb(59 130 246 / 0.3);
  color: white;
}

.input-wrapper {
  position: relative;
}

.domain-input {
  width: 100%;
  padding: 0.5rem 0.75rem;
  background-color: rgb(55 65 81);
  border: 1px solid rgb(75 85 99);
  border-radius: 0.375rem;
  color: white;
  font-size: 0.875rem;
  outline: none;
  transition: all 0.2s;
}

.domain-input::placeholder {
  color: rgb(107 114 128);
}

.domain-input:focus {
  border-color: rgb(59 130 246);
  box-shadow: 0 0 0 3px rgb(59 130 246 / 0.1);
}

.dropdown {
  position: absolute;
  top: calc(100% + 0.25rem);
  left: 0;
  right: 0;
  max-height: 16rem;
  overflow-y: auto;
  background-color: rgb(31 41 55);
  border: 1px solid rgb(75 85 99);
  border-radius: 0.375rem;
  box-shadow: 0 10px 15px -3px rgb(0 0 0 / 0.3);
  z-index: 50;
}

.dropdown-section {
  padding: 0.25rem;
}

.section-label {
  padding: 0.5rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(156 163 175);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.dropdown-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.625rem 0.75rem;
  border-radius: 0.25rem;
  cursor: pointer;
  transition: all 0.15s;
}

.dropdown-item:hover,
.dropdown-item.highlighted {
  background-color: rgb(55 65 81);
}

.domain-text {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.875rem;
  color: white;
}

.add-hint {
  font-size: 0.75rem;
  color: rgb(156 163 175);
}

.added-hint {
  font-size: 0.75rem;
  color: rgb(34 197 94);
  font-weight: 500;
}

.dropdown-divider {
  height: 1px;
  margin: 0.5rem 0;
  background-color: rgb(75 85 99);
}

.dropdown-hint {
  padding: 0.25rem;
}

.custom-hint {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.625rem 0.75rem;
  background-color: rgb(55 65 81);
  border-radius: 0.25rem;
}

.hint-text {
  font-size: 0.75rem;
  color: rgb(156 163 175);
}

.hint-domain {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.875rem;
  color: rgb(147 197 253);
  font-weight: 500;
}

.empty-state {
  padding: 1.5rem 1rem;
  text-align: center;
}

.empty-text {
  font-size: 0.875rem;
  color: rgb(156 163 175);
  margin-bottom: 0.25rem;
}

.empty-subtext {
  font-size: 0.75rem;
  color: rgb(107 114 128);
}

.helper-text {
  margin-top: 0.375rem;
  font-size: 0.75rem;
  color: rgb(156 163 175);
}

/* Scrollbar styling for dropdown */
.dropdown::-webkit-scrollbar {
  width: 0.5rem;
}

.dropdown::-webkit-scrollbar-track {
  background-color: rgb(31 41 55);
  border-radius: 0.375rem;
}

.dropdown::-webkit-scrollbar-thumb {
  background-color: rgb(75 85 99);
  border-radius: 0.375rem;
}

.dropdown::-webkit-scrollbar-thumb:hover {
  background-color: rgb(107 114 128);
}
</style>
