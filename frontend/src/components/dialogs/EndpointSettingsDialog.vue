<script lang="ts" setup>
import { ref, watch } from 'vue'
import { models } from '../../../wailsjs/go/models'
import { useServerStore } from '../../stores/server'
import ProxyConfigPanel from './ProxyConfigPanel.vue'
import ContainerConfigPanel from './ContainerConfigPanel.vue'
import CustomSelect from '../common/CustomSelect.vue'

const serverStore = useServerStore()

// Dropdown options
const translationModeOptions = [
  { value: 'none', label: 'None - Use path as-is' },
  { value: 'strip', label: 'Strip - Remove prefix before matching' },
  { value: 'translate', label: 'Translate - Regex match/replace' }
]

const props = defineProps<{
  show: boolean
  endpoint: models.Endpoint | null
}>()

const emit = defineEmits<{
  save: [endpoint: models.Endpoint]
  delete: []
  cancel: []
}>()

const name = ref('')
const pathPrefix = ref('/')
const translationMode = ref('none')
const translatePattern = ref('')
const translateReplace = ref('')
const enabled = ref(true)
const proxyConfig = ref<models.ProxyConfig | null>(null)
const containerConfig = ref<models.ContainerConfig | null>(null)
const activeTab = ref<'general' | 'proxy' | 'container'>('general')

// Load endpoint data when dialog opens
watch(() => props.show, (newVal) => {
  if (newVal && props.endpoint) {
    name.value = props.endpoint.name || ''
    pathPrefix.value = props.endpoint.path_prefix || '/'
    translationMode.value = props.endpoint.translation_mode || 'none'
    translatePattern.value = props.endpoint.translate_pattern || ''
    translateReplace.value = props.endpoint.translate_replace || ''
    enabled.value = props.endpoint.enabled !== false
    activeTab.value = 'general' // Reset to general tab

    // Load proxy config if this is a proxy endpoint
    if (props.endpoint.type === 'proxy' && props.endpoint.proxy_config) {
      proxyConfig.value = props.endpoint.proxy_config
    }

    // Load container config if this is a container endpoint
    if (props.endpoint.type === 'container' && props.endpoint.container_config) {
      containerConfig.value = props.endpoint.container_config
    }

    window.addEventListener('keydown', handleKeydown)
  } else if (!newVal) {
    window.removeEventListener('keydown', handleKeydown)
  }
})

function handleProxyConfigUpdate(config: models.ProxyConfig) {
  if (props.endpoint?.type === 'proxy') {
    proxyConfig.value = config
  } else if (props.endpoint?.type === 'container' && containerConfig.value) {
    // For container endpoints, update the proxy_config within containerConfig
    containerConfig.value = new models.ContainerConfig({
      ...containerConfig.value,
      proxy_config: config
    })
  }
}

function handleContainerConfigUpdate(config: models.ContainerConfig) {
  containerConfig.value = config
}

function handleSave() {
  if (!props.endpoint || !name.value.trim() || !pathPrefix.value.trim()) {
    return
  }

  const updatedEndpoint = new models.Endpoint({
    id: props.endpoint.id,
    name: name.value.trim(),
    path_prefix: pathPrefix.value.trim(),
    translation_mode: translationMode.value,
    translate_pattern: translationMode.value === 'translate' ? translatePattern.value.trim() : '',
    translate_replace: translationMode.value === 'translate' ? translateReplace.value.trim() : '',
    enabled: enabled.value,
    type: props.endpoint.type,
    items: props.endpoint.items,
    proxy_config: proxyConfig.value || undefined,
    container_config: containerConfig.value || undefined
  })

  emit('save', updatedEndpoint)
}

function handleDelete() {
  emit('delete')
}

function handleCancel() {
  emit('cancel')
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    handleCancel()
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
      >
        <div class="bg-gray-800 rounded-lg shadow-xl max-w-5xl w-full mx-4 border border-gray-700 flex flex-col max-h-[85vh]">
          <!-- Header -->
          <div class="px-6 py-4 border-b border-gray-700 flex-shrink-0">
            <h3 class="text-lg font-semibold text-white">Endpoint Settings</h3>
          </div>

          <!-- Tab Navigation -->
          <div class="flex border-b border-gray-700 flex-shrink-0">
            <button
              @click="activeTab = 'general'"
              :class="[
                'px-4 py-2 text-sm font-medium transition-colors',
                activeTab === 'general'
                  ? 'text-blue-400 border-b-2 border-blue-400'
                  : 'text-gray-400 hover:text-gray-300'
              ]"
            >
              General
            </button>
            <button
              v-if="endpoint?.type === 'proxy' || endpoint?.type === 'container'"
              @click="activeTab = 'proxy'"
              :class="[
                'px-4 py-2 text-sm font-medium transition-colors',
                activeTab === 'proxy'
                  ? 'text-blue-400 border-b-2 border-blue-400'
                  : 'text-gray-400 hover:text-gray-300'
              ]"
            >
              Proxy Settings
            </button>
            <button
              v-if="endpoint?.type === 'container'"
              @click="activeTab = 'container'"
              :class="[
                'px-4 py-2 text-sm font-medium transition-colors',
                activeTab === 'container'
                  ? 'text-blue-400 border-b-2 border-blue-400'
                  : 'text-gray-400 hover:text-gray-300'
              ]"
            >
              Container Settings
            </button>
          </div>

          <!-- Body -->
          <div class="px-6 py-4 space-y-4 overflow-y-auto flex-1 min-h-0">
            <!-- General Tab -->
            <div v-if="activeTab === 'general'" class="space-y-4">
              <!-- Enabled Toggle -->
              <div class="flex items-center justify-between">
                <label class="text-sm font-medium text-gray-300">
                  Endpoint Enabled
                </label>
                <label class="relative inline-flex items-center cursor-pointer">
                  <input v-model="enabled" type="checkbox" class="sr-only peer">
                  <div class="w-11 h-6 bg-gray-700 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-500 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                </label>
              </div>

              <!-- Name -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Name
                </label>
                <input
                  v-model="name"
                  type="text"
                  placeholder="e.g., API v1, Spec 1"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <!-- Path Prefix -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Path Prefix
                </label>
                <input
                  v-model="pathPrefix"
                  type="text"
                  placeholder="e.g., /api/v1"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p class="mt-1 text-xs text-gray-400">
                  All requests starting with this prefix will be handled by this endpoint
                </p>
              </div>

              <!-- Translation Mode -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Path Translation Mode
                </label>
                <CustomSelect
                  v-model="translationMode"
                  :options="translationModeOptions"
                />
              </div>

              <!-- Translation Pattern (only for translate mode) -->
              <div v-if="translationMode === 'translate'" class="space-y-3">
                <div>
                  <label class="block text-sm font-medium text-gray-300 mb-2">
                    Match Pattern (Regex)
                  </label>
                  <input
                    v-model="translatePattern"
                    type="text"
                    placeholder="e.g., ^/api/v1/(.*)"
                    class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
                  />
                </div>

                <div>
                  <label class="block text-sm font-medium text-gray-300 mb-2">
                    Replace With
                  </label>
                  <input
                    v-model="translateReplace"
                    type="text"
                    placeholder="e.g., /$1"
                    class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
                  />
                </div>

                <div class="bg-gray-900 border border-gray-700 rounded p-3">
                  <p class="text-xs text-gray-400 mb-2">Example:</p>
                  <p class="text-xs text-gray-300 font-mono">Pattern: ^/api/v1/(.*)</p>
                  <p class="text-xs text-gray-300 font-mono">Replace: /$1</p>
                  <p class="text-xs text-gray-400 mt-2">
                    Result: /api/v1/users → /users
                  </p>
                </div>
              </div>

              <!-- Translation Mode Info -->
              <div class="bg-gray-900 border border-gray-700 rounded p-3">
                <p class="text-xs text-gray-300">
                  <template v-if="translationMode === 'none'">
                    <strong>None Mode:</strong> Request paths are used exactly as received. Response patterns must match the full path including the prefix.
                  </template>
                  <template v-else-if="translationMode === 'strip'">
                    <strong>Strip Mode:</strong> The prefix is removed from request paths before matching response patterns. Example: /api/v1/users becomes /users
                  </template>
                  <template v-else>
                    <strong>Translate Mode:</strong> Uses regex to transform request paths before matching. Allows complex path rewriting with capture groups.
                  </template>
                </p>
              </div>
            </div>

            <!-- Proxy Settings Tab -->
            <div v-if="activeTab === 'proxy'" class="space-y-4">
              <!-- For proxy endpoints -->
              <ProxyConfigPanel
                v-if="endpoint?.type === 'proxy' && proxyConfig"
                :config="proxyConfig"
                :is-container-endpoint="false"
                @update:config="handleProxyConfigUpdate"
              />
              <!-- For container endpoints -->
              <ProxyConfigPanel
                v-if="endpoint?.type === 'container' && containerConfig?.proxy_config"
                :config="containerConfig.proxy_config"
                :is-container-endpoint="true"
                @update:config="handleProxyConfigUpdate"
              />
            </div>

            <!-- Container Settings Tab -->
            <div v-if="activeTab === 'container' && endpoint?.type === 'container' && containerConfig" class="space-y-4">
              <ContainerConfigPanel
                :config="containerConfig"
                :endpoint-id="endpoint?.id"
                :is-running="serverStore?.isRunning"
                @update:config="handleContainerConfigUpdate"
              />
            </div>
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-gray-700 flex justify-between flex-shrink-0">
            <button
              @click="handleDelete"
              class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded transition-colors"
            >
              Delete Endpoint
            </button>
            <div class="flex gap-3">
              <button
                @click="handleCancel"
                class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors"
              >
                Cancel
              </button>
              <button
                @click="handleSave"
                :disabled="!name.trim() || !pathPrefix.trim()"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Save Changes
              </button>
            </div>
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
