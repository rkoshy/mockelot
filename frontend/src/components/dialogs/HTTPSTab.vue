<script lang="ts" setup>
import { ref, watch, onMounted, computed } from 'vue'
import { useServerStore } from '../../stores/server'
import { GetCACertInfo, RegenerateCA, DownloadCACert, SelectCertFile, GetDefaultCertNames } from '../../../wailsjs/go/main/App'
import { models } from '../../types/models'
import ConfirmDialog from './ConfirmDialog.vue'
import StyledSelect from '../shared/StyledSelect.vue'

const emit = defineEmits<{
  validationChange: [valid: boolean]
}>()

const serverStore = useServerStore()

// HTTPS Configuration
const httpsEnabled = ref(false)
const httpsPort = ref(8443)
const certMode = ref('auto')

// CA Certificate Info
const caInfo = ref<models.CACertInfo | null>(null)
const isLoadingCAInfo = ref(false)

// Certificate Paths
const caCertPath = ref('')
const caKeyPath = ref('')
const serverCertPath = ref('')
const serverKeyPath = ref('')
const serverBundlePath = ref('')

// Certificate Names (CN/SAN)
const useCustomCertNames = ref(false)
const customCertNames = ref('')
const defaultCertNames = ref<string[]>([])

// UI State
const showRegenerateConfirm = ref(false)
const errorMessage = ref('')

// Load CA info
async function loadCAInfo() {
  isLoadingCAInfo.value = true
  try {
    caInfo.value = await GetCACertInfo()
  } catch (error) {
    console.error('Failed to load CA info:', error)
  } finally {
    isLoadingCAInfo.value = false
  }
}

// Load default cert names
async function loadDefaultCertNames() {
  try {
    defaultCertNames.value = await GetDefaultCertNames()
  } catch (error) {
    console.error('Failed to load default cert names:', error)
  }
}

// Load HTTPS configuration from server store
function loadHTTPSConfig() {
  const config = serverStore.config
  if (config) {
    httpsEnabled.value = config.https_enabled || false
    httpsPort.value = config.https_port || 8443
    certMode.value = config.cert_mode || 'auto'

    // Load certificate paths if available
    if (config.cert_paths) {
      caCertPath.value = config.cert_paths.ca_cert_path || ''
      caKeyPath.value = config.cert_paths.ca_key_path || ''
      serverCertPath.value = config.cert_paths.server_cert_path || ''
      serverKeyPath.value = config.cert_paths.server_key_path || ''
      serverBundlePath.value = config.cert_paths.server_bundle_path || ''
    }

    // Load certificate names if available
    if (config.cert_names && config.cert_names.length > 0) {
      useCustomCertNames.value = true
      customCertNames.value = config.cert_names.join(', ')
    } else {
      useCustomCertNames.value = false
      customCertNames.value = ''
    }
  }
}

onMounted(() => {
  loadCAInfo()
  loadDefaultCertNames()
  loadHTTPSConfig()
})

// Regenerate CA
function confirmRegenerateCA() {
  showRegenerateConfirm.value = true
}

async function handleRegenerateCA() {
  showRegenerateConfirm.value = false
  try {
    await RegenerateCA()
    await loadCAInfo()
    errorMessage.value = ''
  } catch (error) {
    errorMessage.value = `Failed to regenerate CA: ${error}`
  }
}

function cancelRegenerateCA() {
  showRegenerateConfirm.value = false
}

// Download CA
async function downloadCA() {
  try {
    const path = await DownloadCACert()
    if (path) {
      console.log('CA certificate saved to:', path)
    }
    errorMessage.value = ''
  } catch (error) {
    errorMessage.value = `Failed to download CA: ${error}`
  }
}

// File selection
async function selectCACert() {
  try {
    const path = await SelectCertFile('Select CA Certificate')
    if (path) {
      caCertPath.value = path
    }
  } catch (error) {
    errorMessage.value = `Failed to select file: ${error}`
  }
}

async function selectCAKey() {
  try {
    const path = await SelectCertFile('Select CA Private Key')
    if (path) {
      caKeyPath.value = path
    }
  } catch (error) {
    errorMessage.value = `Failed to select file: ${error}`
  }
}

async function selectServerCert() {
  try {
    const path = await SelectCertFile('Select Server Certificate')
    if (path) {
      serverCertPath.value = path
    }
  } catch (error) {
    errorMessage.value = `Failed to select file: ${error}`
  }
}

async function selectServerKey() {
  try {
    const path = await SelectCertFile('Select Server Private Key')
    if (path) {
      serverKeyPath.value = path
    }
  } catch (error) {
    errorMessage.value = `Failed to select file: ${error}`
  }
}

async function selectServerBundle() {
  try {
    const path = await SelectCertFile('Select Certificate Bundle')
    if (path) {
      serverBundlePath.value = path
    }
  } catch (error) {
    errorMessage.value = `Failed to select file: ${error}`
  }
}

// Validation
watch([httpsPort, certMode, caCertPath, caKeyPath, serverCertPath, serverKeyPath], () => {
  // Validate port
  const portValid = httpsPort.value >= 1 && httpsPort.value <= 65535

  // Validate certificate paths based on mode
  let certPathsValid = true
  if (certMode.value === 'ca-provided') {
    certPathsValid = caCertPath.value !== '' && caKeyPath.value !== ''
  } else if (certMode.value === 'cert-provided') {
    certPathsValid = serverCertPath.value !== '' && serverKeyPath.value !== ''
  }

  emit('validationChange', portValid && certPathsValid)
})

// Format timestamp
function formatTimestamp(timestamp: string): string {
  if (!timestamp) return 'Not generated'
  const date = new Date(timestamp)
  return date.toLocaleString()
}

// Certificate mode options
const certModeOptions = computed(() => [
  {
    value: 'auto',
    label: 'Auto-generate (default)',
    description: 'Automatically generates certificates'
  },
  {
    value: 'ca-provided',
    label: 'Provide CA Cert + Key',
    description: 'Upload your own CA certificate and key'
  },
  {
    value: 'cert-provided',
    label: 'Provide Server Cert + Key + Bundle',
    description: 'Upload server certificate files'
  }
])

// Expose config and methods for parent
defineExpose({
  getConfig: () => ({
    enabled: httpsEnabled.value,
    port: httpsPort.value,
    certMode: certMode.value,
    certPaths: {
      ca_cert_path: caCertPath.value,
      ca_key_path: caKeyPath.value,
      server_cert_path: serverCertPath.value,
      server_key_path: serverKeyPath.value,
      server_bundle_path: serverBundlePath.value,
    },
    certNames: useCustomCertNames.value && customCertNames.value
      ? customCertNames.value.split(',').map(s => s.trim()).filter(s => s !== '')
      : []  // Empty array uses backend defaults (localhost, hostname, gateway IP)
  }),
  loadHTTPSConfig
})
</script>

<template>
  <div class="space-y-6">
    <!-- Enable HTTPS -->
    <div>
      <label class="flex items-center gap-2 cursor-pointer">
        <input
          v-model="httpsEnabled"
          type="checkbox"
          class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
        />
        <span class="text-sm font-medium text-white">Enable HTTPS</span>
      </label>
    </div>

    <div v-if="httpsEnabled" class="space-y-6">
      <!-- HTTPS Port -->
      <div>
        <label class="block text-sm font-medium text-gray-300 mb-2">
          HTTPS Port
        </label>
        <input
          v-model.number="httpsPort"
          type="number"
          min="1"
          max="65535"
          class="w-32 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white
                 focus:outline-none focus:border-blue-500"
          placeholder="8443"
        />
        <p class="mt-1 text-xs text-gray-400">
          Port for HTTPS server (default: 8443)
        </p>
      </div>

      <!-- Certificate Mode -->
      <div class="border-t border-gray-700 pt-6">
        <h4 class="text-sm font-semibold text-white mb-3">Certificate Mode</h4>

        <StyledSelect
          v-model="certMode"
          :options="certModeOptions"
        />

        <!-- Auto Mode Description -->
        <div v-if="certMode === 'auto'" class="mt-3 p-3 bg-gray-700/50 rounded">
          <p class="text-xs text-gray-300">
            Automatically generates a CA certificate (persistent) and server certificate (regenerated on each start).
          </p>
        </div>

        <!-- CA Provided Mode -->
        <div v-if="certMode === 'ca-provided'" class="mt-4 space-y-3">
          <div>
            <label class="block text-sm font-medium text-gray-300 mb-2">
              CA Certificate (.pem/.crt)
            </label>
            <div class="flex gap-2">
              <input
                v-model="caCertPath"
                type="text"
                class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                       focus:outline-none focus:border-blue-500"
                placeholder="Select CA certificate file"
                readonly
              />
              <button
                @click="selectCACert"
                class="px-3 py-2 bg-gray-600 hover:bg-gray-500 text-white text-sm rounded transition-colors"
              >
                Browse
              </button>
            </div>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-300 mb-2">
              CA Private Key (.pem/.key)
            </label>
            <div class="flex gap-2">
              <input
                v-model="caKeyPath"
                type="text"
                class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                       focus:outline-none focus:border-blue-500"
                placeholder="Select CA private key file"
                readonly
              />
              <button
                @click="selectCAKey"
                class="px-3 py-2 bg-gray-600 hover:bg-gray-500 text-white text-sm rounded transition-colors"
              >
                Browse
              </button>
            </div>
          </div>
        </div>

        <!-- Cert Provided Mode -->
        <div v-if="certMode === 'cert-provided'" class="mt-4 space-y-3">
          <div>
            <label class="block text-sm font-medium text-gray-300 mb-2">
              Server Certificate (.pem/.crt)
            </label>
            <div class="flex gap-2">
              <input
                v-model="serverCertPath"
                type="text"
                class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                       focus:outline-none focus:border-blue-500"
                placeholder="Select server certificate file"
                readonly
              />
              <button
                @click="selectServerCert"
                class="px-3 py-2 bg-gray-600 hover:bg-gray-500 text-white text-sm rounded transition-colors"
              >
                Browse
              </button>
            </div>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-300 mb-2">
              Server Private Key (.pem/.key)
            </label>
            <div class="flex gap-2">
              <input
                v-model="serverKeyPath"
                type="text"
                class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                       focus:outline-none focus:border-blue-500"
                placeholder="Select server private key file"
                readonly
              />
              <button
                @click="selectServerKey"
                class="px-3 py-2 bg-gray-600 hover:bg-gray-500 text-white text-sm rounded transition-colors"
              >
                Browse
              </button>
            </div>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-300 mb-2">
              Certificate Bundle (.pem) - Optional
            </label>
            <div class="flex gap-2">
              <input
                v-model="serverBundlePath"
                type="text"
                class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                       focus:outline-none focus:border-blue-500"
                placeholder="Select certificate bundle (optional)"
                readonly
              />
              <button
                @click="selectServerBundle"
                class="px-3 py-2 bg-gray-600 hover:bg-gray-500 text-white text-sm rounded transition-colors"
              >
                Browse
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Certificate Names (CN/SAN) - Only visible when generating certs -->
      <div v-if="certMode === 'auto' || certMode === 'ca-provided'" class="border-t border-gray-700 pt-6">
        <h4 class="text-sm font-semibold text-white mb-3">Certificate Names (CN/SAN)</h4>

        <!-- Default Names Info -->
        <div class="p-3 bg-gray-700/50 rounded mb-3">
          <p class="text-sm font-medium text-gray-300 mb-2">Default Names:</p>
          <p class="text-xs text-gray-400 font-mono">
            {{ defaultCertNames.length > 0 ? defaultCertNames.join(', ') : 'Loading...' }}
          </p>
          <p class="text-xs text-gray-500 mt-2">
            Automatically includes: localhost, machine hostname, and interface IP to default gateway
          </p>
        </div>

        <!-- Custom Names Toggle -->
        <label class="flex items-center gap-2 cursor-pointer mb-3">
          <input
            v-model="useCustomCertNames"
            type="checkbox"
            class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
          />
          <span class="text-sm font-medium text-white">Use Custom Names</span>
        </label>

        <!-- Custom Names Input -->
        <div v-if="useCustomCertNames">
          <label class="block text-sm font-medium text-gray-300 mb-2">
            DNS Names and IP Addresses (comma-separated)
          </label>
          <input
            v-model="customCertNames"
            type="text"
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm
                   focus:outline-none focus:border-blue-500"
            placeholder="e.g., example.com, 192.168.1.100, *.example.com"
          />
          <p class="mt-1 text-xs text-gray-400">
            Enter DNS names or IP addresses separated by commas. These will be used as Subject Alternative Names (SAN).
          </p>
        </div>
      </div>

      <!-- CA Certificate Section - Only visible when generating certs -->
      <div v-if="certMode === 'auto' || certMode === 'ca-provided'" class="border-t border-gray-700 pt-6">
        <h4 class="text-sm font-semibold text-white mb-3">CA Certificate</h4>

        <!-- CA Info -->
        <div class="p-3 bg-gray-700/50 rounded mb-3">
          <p class="text-sm text-gray-300">
            <span class="font-medium">Status:</span>
            <span v-if="isLoadingCAInfo" class="ml-2">Loading...</span>
            <span v-else-if="caInfo?.exists" class="ml-2 text-green-400">Generated</span>
            <span v-else class="ml-2 text-gray-400">Not generated</span>
          </p>
          <p v-if="caInfo?.exists && caInfo?.generated" class="text-sm text-gray-300 mt-1">
            <span class="font-medium">Generated:</span>
            <span class="ml-2">{{ formatTimestamp(caInfo.generated) }}</span>
          </p>
        </div>

        <!-- CA Actions -->
        <div class="flex gap-2">
          <button
            @click="confirmRegenerateCA"
            class="px-3 py-2 bg-orange-600 hover:bg-orange-700 text-white text-sm rounded transition-colors flex items-center gap-2"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Regenerate CA
          </button>
          <button
            @click="downloadCA"
            :disabled="!caInfo?.exists"
            :class="[
              'px-3 py-2 text-white text-sm rounded transition-colors flex items-center gap-2',
              caInfo?.exists
                ? 'bg-blue-600 hover:bg-blue-700'
                : 'bg-gray-600 cursor-not-allowed'
            ]"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            Download CA Certificate
          </button>
        </div>

        <div class="mt-3 p-3 bg-blue-900/20 border border-blue-800 rounded">
          <p class="text-xs text-blue-300">
            💡 Download and install the CA certificate in your browser/OS to trust the self-signed certificates.
          </p>
        </div>
      </div>

      <!-- Error Message -->
      <div v-if="errorMessage" class="p-3 bg-red-900/20 border border-red-800 rounded">
        <p class="text-sm text-red-300">⚠️ {{ errorMessage }}</p>
      </div>
    </div>

    <!-- Regenerate Confirmation Dialog -->
    <ConfirmDialog
      :show="showRegenerateConfirm"
      title="Regenerate CA Certificate?"
      message="This will invalidate all existing client trust. HTTPS will restart. Continue?"
      primary-text="Regenerate"
      cancel-text="Cancel"
      @primary="handleRegenerateCA"
      @cancel="cancelRegenerateCA"
    />
  </div>
</template>
