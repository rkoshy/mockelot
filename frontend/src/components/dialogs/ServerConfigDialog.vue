<script lang="ts" setup>
import { ref, watch } from 'vue'
import HTTPTab from './HTTPTab.vue'
import HTTPSTab from './HTTPSTab.vue'
import CORSTab from './CORSTab.vue'

const props = defineProps<{
  show: boolean
  initialTab?: 'http' | 'https' | 'cors'
}>()

const emit = defineEmits<{
  close: []
  apply: []
}>()

type TabType = 'http' | 'https' | 'cors'

const currentTab = ref<TabType>(props.initialTab || 'http')
const isValid = ref(true)

// Tab component refs
const httpTab = ref<InstanceType<typeof HTTPTab> | null>(null)
const httpsTab = ref<InstanceType<typeof HTTPSTab> | null>(null)
const corsTab = ref<InstanceType<typeof CORSTab> | null>(null)

// Expose refs for parent access
defineExpose({
  httpTab,
  httpsTab,
  corsTab
})

// Switch tab when initialTab prop changes
watch(() => props.initialTab, (newTab) => {
  if (newTab) {
    currentTab.value = newTab
  }
})

function setTab(tab: TabType) {
  currentTab.value = tab
}

function handleClose() {
  emit('close')
}

function handleApply() {
  if (isValid.value) {
    emit('apply')
  }
}

function handleValidationChange(valid: boolean) {
  isValid.value = valid
}

// Close on Escape key
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    handleClose()
  }
}

watch(() => props.show, (newVal) => {
  if (newVal) {
    window.addEventListener('keydown', handleKeydown)
    // Reload config when dialog opens
    if (httpsTab.value?.loadHTTPSConfig) {
      httpsTab.value.loadHTTPSConfig()
    }
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
        @click.self="handleClose"
      >
        <div class="bg-gray-800 rounded-lg shadow-xl max-w-3xl w-full mx-4 border border-gray-700 max-h-[90vh] flex flex-col">
          <!-- Header -->
          <div class="px-6 py-4 border-b border-gray-700 flex-shrink-0">
            <h3 class="text-lg font-semibold text-white">Server Configuration</h3>
          </div>

          <!-- Tabs -->
          <div class="flex border-b border-gray-700 px-6 flex-shrink-0">
            <button
              @click="setTab('http')"
              :class="[
                'px-4 py-3 text-sm font-medium border-b-2 transition-colors',
                currentTab === 'http'
                  ? 'border-blue-500 text-blue-500'
                  : 'border-transparent text-gray-400 hover:text-gray-300'
              ]"
            >
              HTTP
            </button>
            <button
              @click="setTab('https')"
              :class="[
                'px-4 py-3 text-sm font-medium border-b-2 transition-colors',
                currentTab === 'https'
                  ? 'border-blue-500 text-blue-500'
                  : 'border-transparent text-gray-400 hover:text-gray-300'
              ]"
            >
              HTTPS
            </button>
            <button
              @click="setTab('cors')"
              :class="[
                'px-4 py-3 text-sm font-medium border-b-2 transition-colors',
                currentTab === 'cors'
                  ? 'border-blue-500 text-blue-500'
                  : 'border-transparent text-gray-400 hover:text-gray-300'
              ]"
            >
              CORS
            </button>
          </div>

          <!-- Tab Content -->
          <div class="flex-1 overflow-y-auto p-6">
            <HTTPTab
              ref="httpTab"
              v-show="currentTab === 'http'"
              @validation-change="handleValidationChange"
            />
            <HTTPSTab
              ref="httpsTab"
              v-show="currentTab === 'https'"
              @validation-change="handleValidationChange"
            />
            <CORSTab
              ref="corsTab"
              v-show="currentTab === 'cors'"
              @validation-change="handleValidationChange"
            />
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-gray-700 flex justify-end gap-3 flex-shrink-0">
            <button
              @click="handleClose"
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors"
            >
              Cancel
            </button>
            <button
              @click="handleApply"
              :disabled="!isValid"
              :class="[
                'px-4 py-2 rounded transition-colors',
                isValid
                  ? 'bg-blue-600 hover:bg-blue-700 text-white'
                  : 'bg-gray-600 text-gray-400 cursor-not-allowed'
              ]"
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
