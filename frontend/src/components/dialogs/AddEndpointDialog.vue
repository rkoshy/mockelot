<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
import CustomSelect from '../common/CustomSelect.vue'
import VolumeList from './VolumeList.vue'
import EnvironmentVarList from './EnvironmentVarList.vue'
import StatusTranslationList from './StatusTranslationList.vue'
import HeaderManipulationList from './HeaderManipulationList.vue'
import { ValidateAndInspectDockerImage, PullDockerImage, TestContainerConfig, GetDefaultContainerHeaders } from '../../../wailsjs/go/main/App'
import type { models } from '../../../wailsjs/go/models'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  confirm: [config: any] // Full endpoint configuration object
  cancel: []
}>()

// Wizard state
const currentStep = ref(1)
const endpointType = ref('mock')

// Basic config (Step 1)
const name = ref('')
const pathPrefix = ref('/')
const translationMode = ref('none')

// Container config (Steps 2-5)
const containerImageName = ref('')
const containerPort = ref(8080)
const pullOnStartup = ref(true)
const restartOnServerStart = ref(false)
const restartPolicy = ref('always')
const healthCheckEnabled = ref(false)
const healthCheckInterval = ref(30)
const healthCheckPath = ref('/')
const volumes = ref<Array<{host_path: string, container_path: string, read_only: boolean}>>([])
const environment = ref<Array<{name: string, value: string}>>([])
const hostNetworking = ref(false)
const dockerSocketAccess = ref(false)

// Image validation state
const imageValidationStatus = ref<'idle' | 'validating' | 'pulling' | 'success' | 'error'>('idle')
const imageValidationMessage = ref('')

// Image inspection result
const imageInfo = ref<models.DockerImageInfo | null>(null)
const selectedPort = ref<string>('')  // For radio button selection when multiple ports available

// Container test state
const containerTestStatus = ref<'idle' | 'testing' | 'success' | 'error'>('idle')
const containerTestMessage = ref('')
const containerTestSkipped = ref(false)

// Proxy config (Steps 2-3)
const backendURL = ref('')
const proxyTimeout = ref(30)
const statusPassthrough = ref(true)
const statusTranslations = ref<Array<{from_pattern: string, to_code: number}>>([])
const requestHeaders = ref<models.HeaderManipulation[]>([])
const responseHeaders = ref<models.HeaderManipulation[]>([])

// Dropdown options
const endpointTypeOptions = [
  { value: 'mock', label: 'Mock - Script-based responses' },
  { value: 'proxy', label: 'Proxy - Reverse proxy with translation' },
  { value: 'container', label: 'Container - Docker container' }
]

const translationModeOptions = [
  { value: 'none', label: 'None - Use path as-is' },
  { value: 'strip', label: 'Strip - Remove prefix before matching' },
  { value: 'translate', label: 'Translate - Regex match/replace' }
]

const restartPolicyOptions = [
  { value: 'no', label: 'No - Never restart' },
  { value: 'always', label: 'Always - Always restart' },
  { value: 'unless-stopped', label: 'Unless Stopped - Restart unless manually stopped' },
  { value: 'on-failure', label: 'On Failure - Restart only on failure' }
]

// Computed properties
const totalSteps = computed(() => {
  if (endpointType.value === 'container') return 7 // Added proxy configuration step
  if (endpointType.value === 'proxy') return 3
  return 1
})

const canGoNext = computed(() => {
  if (currentStep.value === 1) {
    return name.value.trim() && pathPrefix.value.trim()
  }
  if (endpointType.value === 'container') {
    if (currentStep.value === 2) {
      return containerImageName.value.trim() && containerPort.value > 0 && imageValidationStatus.value === 'success'
    }
    if (currentStep.value === 7) {
      // Can proceed if test succeeded or was skipped
      return containerTestStatus.value === 'success' || containerTestSkipped.value
    }
  }
  if (endpointType.value === 'proxy') {
    if (currentStep.value === 2) {
      return backendURL.value.trim()
    }
  }
  return true
})

const stepTitle = computed(() => {
  if (currentStep.value === 1) return 'Basic Configuration'
  if (endpointType.value === 'container') {
    if (currentStep.value === 2) return 'Container Settings'
    if (currentStep.value === 3) return 'Volume Mappings'
    if (currentStep.value === 4) return 'Environment Variables'
    if (currentStep.value === 5) return 'Special Permissions'
    if (currentStep.value === 6) return 'Proxy Configuration'
    if (currentStep.value === 7) return 'Test Container'
  }
  if (endpointType.value === 'proxy') {
    if (currentStep.value === 2) return 'Backend Configuration'
    if (currentStep.value === 3) return 'Headers & Status Codes'
  }
  return ''
})

// Reset form when dialog opens
watch(() => props.show, (newVal) => {
  if (newVal) {
    resetForm()
    window.addEventListener('keydown', handleKeydown)
  } else {
    window.removeEventListener('keydown', handleKeydown)
  }
})

function resetForm() {
  currentStep.value = 1
  endpointType.value = 'mock'
  name.value = ''
  pathPrefix.value = '/'
  translationMode.value = 'none'

  // Container fields
  containerImageName.value = ''
  containerPort.value = 8080
  pullOnStartup.value = true
  restartOnServerStart.value = false
  restartPolicy.value = 'always'
  healthCheckEnabled.value = false
  healthCheckInterval.value = 30
  healthCheckPath.value = '/'
  volumes.value = []
  environment.value = []
  hostNetworking.value = false
  dockerSocketAccess.value = false
  imageValidationStatus.value = 'idle'
  imageValidationMessage.value = ''
  imageInfo.value = null
  selectedPort.value = ''
  containerTestStatus.value = 'idle'
  containerTestMessage.value = ''
  containerTestSkipped.value = false

  // Proxy fields
  backendURL.value = ''
  proxyTimeout.value = 30
  statusPassthrough.value = true
  statusTranslations.value = []
  requestHeaders.value = []
  responseHeaders.value = []
}

async function handleValidateImage() {
  if (!containerImageName.value.trim()) return

  imageValidationStatus.value = 'validating'
  imageValidationMessage.value = 'Inspecting image...'

  try {
    // Inspect the image (validates and extracts metadata)
    const info = await ValidateAndInspectDockerImage(containerImageName.value.trim())
    imageInfo.value = info
    imageValidationStatus.value = 'success'
    imageValidationMessage.value = 'Image inspected successfully'

    // Auto-populate fields from inspection results
    // Port selection
    if (info.exposed_ports && info.exposed_ports.length > 0) {
      // Extract numeric port from "80/tcp" format
      const firstPort = info.exposed_ports[0].split('/')[0]
      selectedPort.value = firstPort
      containerPort.value = parseInt(firstPort, 10)
    }

    // Pre-populate volumes from image
    if (info.volumes && info.volumes.length > 0) {
      volumes.value = info.volumes.map(vol => ({
        host_path: '',  // User must specify host path
        container_path: vol,
        read_only: false
      }))
    }

    // Pre-populate environment variables from image defaults
    if (info.environment) {
      environment.value = Object.entries(info.environment).map(([name, value]) => ({
        name,
        value,
        expression: ''
      }))
    }

    // Auto-configure health check based on image analysis
    if (info.is_http_service && info.suggested_health_check_path) {
      healthCheckEnabled.value = true
      healthCheckPath.value = info.suggested_health_check_path
    } else {
      healthCheckEnabled.value = false
      healthCheckPath.value = '/'
    }

  } catch (error) {
    // Image not found locally, try to pull
    imageValidationStatus.value = 'pulling'
    imageValidationMessage.value = 'Image not found locally. Pulling from registry...'

    try {
      await PullDockerImage(containerImageName.value.trim())
      // After pull, inspect again
      const info = await ValidateAndInspectDockerImage(containerImageName.value.trim())
      imageInfo.value = info
      imageValidationStatus.value = 'success'
      imageValidationMessage.value = 'Image pulled and inspected successfully'

      // Auto-populate fields (same as above)
      if (info.exposed_ports && info.exposed_ports.length > 0) {
        const firstPort = info.exposed_ports[0].split('/')[0]
        selectedPort.value = firstPort
        containerPort.value = parseInt(firstPort, 10)
      }

      if (info.volumes && info.volumes.length > 0) {
        volumes.value = info.volumes.map(vol => ({
          host_path: '',
          container_path: vol,
          read_only: false
        }))
      }

      if (info.environment) {
        environment.value = Object.entries(info.environment).map(([name, value]) => ({
          name,
          value,
          expression: ''
        }))
      }

      if (info.is_http_service && info.suggested_health_check_path) {
        healthCheckEnabled.value = true
        healthCheckPath.value = info.suggested_health_check_path
      } else {
        healthCheckEnabled.value = false
        healthCheckPath.value = '/'
      }

    } catch (pullError) {
      imageValidationStatus.value = 'error'
      imageValidationMessage.value = `Failed to pull image: ${pullError}`
      imageInfo.value = null
    }
  }
}

async function handleTestContainer() {
  containerTestStatus.value = 'testing'
  containerTestMessage.value = 'Starting temporary container...'
  containerTestSkipped.value = false

  try {
    // Build container configuration for testing
    const testConfig = {
      image_name: containerImageName.value.trim(),
      container_port: containerPort.value,
      volumes: volumes.value,
      environment: environment.value,
      host_networking: hostNetworking.value,
      docker_socket_access: dockerSocketAccess.value,
      health_check_enabled: healthCheckEnabled.value,
      health_check_path: healthCheckPath.value
    }

    // Call backend to test container configuration
    await TestContainerConfig(testConfig)

    containerTestStatus.value = 'success'
    containerTestMessage.value = 'Container started successfully and is responding!'
  } catch (error) {
    containerTestStatus.value = 'error'
    containerTestMessage.value = `Container test failed: ${error}`
  }
}

function handleSkipTest() {
  containerTestSkipped.value = true
  containerTestStatus.value = 'idle'
  containerTestMessage.value = 'Test skipped - proceeding to save'
}

async function loadDefaultContainerHeaders() {
  try {
    const defaults = await GetDefaultContainerHeaders()
    requestHeaders.value = defaults
  } catch (error) {
    console.error('Failed to load default container headers:', error)
  }
}

async function loadDefaultProxyHeaders() {
  try {
    const defaults = await GetDefaultContainerHeaders()
    // For regular proxy endpoints, filter out container-specific Host header
    requestHeaders.value = defaults.filter(h => h.name !== 'Host')
  } catch (error) {
    console.error('Failed to load default proxy headers:', error)
  }
}

async function handleNext() {
  if (!canGoNext.value) return
  if (currentStep.value < totalSteps.value) {
    currentStep.value++

    // Auto-load default container headers when entering proxy configuration step
    if (endpointType.value === 'container' && currentStep.value === 6 && requestHeaders.value.length === 0) {
      await loadDefaultContainerHeaders()
    }

    // Auto-load default proxy headers when entering headers step for regular proxy endpoints
    if (endpointType.value === 'proxy' && currentStep.value === 3 && requestHeaders.value.length === 0) {
      await loadDefaultProxyHeaders()
    }
  } else {
    handleFinish()
  }
}

function handleBack() {
  if (currentStep.value > 1) {
    currentStep.value--
  }
}

function handleFinish() {
  if (!name.value.trim() || !pathPrefix.value.trim()) {
    return
  }

  // Build full configuration object based on endpoint type
  const config: any = {
    name: name.value.trim(),
    path_prefix: pathPrefix.value.trim(),
    translation_mode: translationMode.value,
    type: endpointType.value
  }

  // Add type-specific configuration
  if (endpointType.value === 'container') {
    config.container_config = {
      image_name: containerImageName.value.trim(),
      container_port: containerPort.value,
      exposed_ports: imageInfo.value?.exposed_ports || [],
      pull_on_startup: pullOnStartup.value,
      restart_on_server_start: restartOnServerStart.value,
      restart_policy: restartPolicy.value,
      volumes: volumes.value,
      environment: environment.value,
      host_networking: hostNetworking.value,
      docker_socket_access: dockerSocketAccess.value,
      proxy_config: {
        backend_url: backendURL.value.trim() || 'http://localhost:' + containerPort.value,
        timeout_seconds: proxyTimeout.value,
        status_passthrough: statusPassthrough.value,
        status_translation: statusTranslations.value,
        inbound_headers: requestHeaders.value,
        outbound_headers: responseHeaders.value,
        health_check_enabled: healthCheckEnabled.value,
        health_check_interval: healthCheckInterval.value,
        health_check_path: healthCheckPath.value
      }
    }
  } else if (endpointType.value === 'proxy') {
    config.proxy_config = {
      backend_url: backendURL.value.trim(),
      timeout_seconds: proxyTimeout.value,
      status_passthrough: statusPassthrough.value,
      status_translation: statusTranslations.value,
      inbound_headers: requestHeaders.value,
      outbound_headers: responseHeaders.value
    }
  }

  emit('confirm', config)
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
        class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-80"
      >
        <!-- Wizard Dialog - 80% of screen -->
        <div class="bg-gray-800 rounded-lg shadow-xl w-[80vw] h-[80vh] mx-4 border border-gray-700 flex flex-col">
          <!-- Header -->
          <div class="px-6 py-4 border-b border-gray-700 flex items-center justify-between flex-shrink-0">
            <div>
              <h3 class="text-xl font-semibold text-white">Add New Endpoint</h3>
              <p class="text-sm text-gray-400 mt-1">Step {{ currentStep }} of {{ totalSteps }}: {{ stepTitle }}</p>
            </div>
            <button
              @click="handleCancel"
              class="p-1 hover:bg-gray-700 rounded transition-colors text-gray-400 hover:text-white"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- Progress Bar -->
          <div class="px-6 py-3 border-b border-gray-700 flex-shrink-0">
            <div class="flex gap-2">
              <div
                v-for="step in totalSteps"
                :key="step"
                class="flex-1 h-2 rounded-full transition-all"
                :class="step <= currentStep ? 'bg-blue-600' : 'bg-gray-700'"
              ></div>
            </div>
          </div>

          <!-- Body - Scrollable -->
          <div class="flex-1 overflow-y-auto px-6 py-6">
            <!-- Step 1: Basic Configuration -->
            <div v-if="currentStep === 1" class="space-y-6">
              <!-- Name -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Endpoint Name <span class="text-red-400">*</span>
                </label>
                <input
                  v-model="name"
                  type="text"
                  placeholder="e.g., API v1, User Service"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  autofocus
                />
              </div>

              <!-- Endpoint Type -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Endpoint Type <span class="text-red-400">*</span>
                </label>
                <CustomSelect
                  v-model="endpointType"
                  :options="endpointTypeOptions"
                />
                <p class="mt-2 text-sm text-gray-400">
                  <template v-if="endpointType === 'mock'">
                    Define custom mock responses with JavaScript templates and validation scripts
                  </template>
                  <template v-else-if="endpointType === 'proxy'">
                    Forward requests to a backend server with optional header manipulation and status code translation
                  </template>
                  <template v-else>
                    Run a Docker/Podman container to handle requests with full control over configuration
                  </template>
                </p>
              </div>

              <!-- Path Prefix -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Path Prefix <span class="text-red-400">*</span>
                </label>
                <input
                  v-model="pathPrefix"
                  type="text"
                  placeholder="e.g., /api/v1"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p class="mt-2 text-sm text-gray-400">
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
                <p class="mt-2 text-sm text-gray-400">
                  <template v-if="translationMode === 'none'">
                    Paths match exactly as received (e.g., /api/v1/users matches /api/v1/users)
                  </template>
                  <template v-else-if="translationMode === 'strip'">
                    Prefix is removed before matching (e.g., /api/v1/users becomes /users)
                  </template>
                  <template v-else>
                    Use regex to transform paths - configure pattern after creation
                  </template>
                </p>
              </div>
            </div>

            <!-- Step 2: Container Settings -->
            <div v-if="currentStep === 2 && endpointType === 'container'" class="space-y-6">
              <!-- Image Name -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Container Image <span class="text-red-400">*</span>
                </label>
                <div class="flex gap-2">
                  <input
                    v-model="containerImageName"
                    type="text"
                    placeholder="e.g., nginx:latest, postgres:14-alpine"
                    class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                  <button
                    @click="handleValidateImage"
                    :disabled="!containerImageName.trim() || imageValidationStatus === 'validating' || imageValidationStatus === 'pulling'"
                    class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded transition-colors"
                  >
                    {{ imageValidationStatus === 'validating' || imageValidationStatus === 'pulling' ? 'Checking...' : 'Validate' }}
                  </button>
                </div>
                <div v-if="imageValidationStatus !== 'idle'" class="mt-2 p-2 rounded text-sm" :class="{
                  'bg-yellow-900/30 border border-yellow-700 text-yellow-400': imageValidationStatus === 'validating' || imageValidationStatus === 'pulling',
                  'bg-green-900/30 border border-green-700 text-green-400': imageValidationStatus === 'success',
                  'bg-red-900/30 border border-red-700 text-red-400': imageValidationStatus === 'error'
                }">
                  {{ imageValidationMessage }}
                </div>
              </div>

              <!-- Detected Ports (show if any ports detected) -->
              <div v-if="imageInfo && imageInfo.exposed_ports && imageInfo.exposed_ports.length > 0" class="border border-gray-700 rounded p-4">
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Container Port <span class="text-red-400">*</span>
                </label>
                <p class="text-xs text-gray-400 mb-3">
                  The following ports are exposed by this container image. Select which port your application uses:
                </p>
                <div class="space-y-2">
                  <div
                    v-for="port in imageInfo.exposed_ports"
                    :key="port"
                    class="flex items-center gap-2"
                  >
                    <input
                      :id="`port-${port}`"
                      type="radio"
                      :value="port.split('/')[0]"
                      v-model="selectedPort"
                      @change="containerPort = parseInt(selectedPort, 10)"
                      class="w-4 h-4 text-blue-600 bg-gray-700 border-gray-600 focus:ring-blue-500"
                    />
                    <label :for="`port-${port}`" class="text-sm text-gray-300 cursor-pointer">
                      Port <span class="font-mono text-blue-400">{{ port }}</span>
                    </label>
                  </div>
                  <!-- "Other" option -->
                  <div class="flex items-center gap-2">
                    <input
                      id="port-other"
                      type="radio"
                      value="other"
                      v-model="selectedPort"
                      class="w-4 h-4 text-blue-600 bg-gray-700 border-gray-600 focus:ring-blue-500"
                    />
                    <label for="port-other" class="text-sm text-gray-300 cursor-pointer">
                      Other (custom port)
                    </label>
                  </div>
                </div>
              </div>

              <!-- Container Port Manual Entry (only show if "Other" selected or no ports detected) -->
              <div v-if="!imageInfo || !imageInfo.exposed_ports || imageInfo.exposed_ports.length === 0 || selectedPort === 'other'">
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Container Port <span class="text-red-400">*</span>
                </label>
                <input
                  v-model.number="containerPort"
                  type="number"
                  min="1"
                  max="65535"
                  placeholder="e.g., 8080"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p class="mt-2 text-sm text-gray-400">
                  The port inside the container that your application listens on
                </p>
              </div>

              <!-- Pull on Startup -->
              <div class="flex items-start gap-3">
                <input
                  v-model="pullOnStartup"
                  type="checkbox"
                  id="pull-on-startup"
                  class="mt-1 w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600 focus:ring-blue-500"
                />
                <div>
                  <label for="pull-on-startup" class="block text-sm font-medium text-gray-300">
                    Pull image on startup
                  </label>
                  <p class="text-sm text-gray-400 mt-1">
                    Automatically pull the latest version of the image when the server starts
                  </p>
                </div>
              </div>

              <!-- Restart on Server Start -->
              <div class="flex items-start gap-3">
                <input
                  v-model="restartOnServerStart"
                  type="checkbox"
                  id="restart-on-server-start"
                  class="mt-1 w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600 focus:ring-blue-500"
                />
                <div>
                  <label for="restart-on-server-start" class="block text-sm font-medium text-gray-300">
                    Restart container when server starts
                  </label>
                  <p class="text-sm text-gray-400 mt-1">
                    If container is already running when server starts, restart it
                  </p>
                </div>
              </div>

              <!-- Restart Policy -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Restart Policy
                </label>
                <CustomSelect
                  v-model="restartPolicy"
                  :options="restartPolicyOptions"
                />
              </div>

              <!-- Health Check -->
              <div class="border border-gray-700 rounded p-4">
                <div class="flex items-start gap-3 mb-4">
                  <input
                    v-model="healthCheckEnabled"
                    type="checkbox"
                    id="health-check-enabled"
                    class="mt-1 w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600 focus:ring-blue-500"
                  />
                  <div>
                    <label for="health-check-enabled" class="block text-sm font-medium text-gray-300">
                      Enable health checks
                    </label>
                    <p class="text-sm text-gray-400 mt-1">
                      Periodically check if the container is responding
                    </p>
                  </div>
                </div>

                <div v-if="healthCheckEnabled" class="space-y-4 pl-7">
                  <div>
                    <label class="block text-sm font-medium text-gray-300 mb-2">
                      Health Check Path
                    </label>
                    <input
                      v-model="healthCheckPath"
                      type="text"
                      placeholder="e.g., /health, /api/health"
                      class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                    />
                  </div>

                  <div>
                    <label class="block text-sm font-medium text-gray-300 mb-2">
                      Check Interval (seconds)
                    </label>
                    <input
                      v-model.number="healthCheckInterval"
                      type="number"
                      min="5"
                      max="300"
                      class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                    />
                  </div>
                </div>
              </div>
            </div>

            <!-- Step 3: Volume Mappings (Container) -->
            <div v-if="currentStep === 3 && endpointType === 'container'" class="space-y-4">
              <p class="text-sm text-gray-400">
                Mount host directories or files into the container. This allows the container to persist data or access configuration files.
              </p>
              <VolumeList v-model="volumes" />
            </div>

            <!-- Step 4: Environment Variables (Container) -->
            <div v-if="currentStep === 4 && endpointType === 'container'" class="space-y-4">
              <p class="text-sm text-gray-400">
                Set environment variables that will be available inside the container.
              </p>
              <EnvironmentVarList v-model="environment" />
            </div>

            <!-- Step 5: Special Permissions (Container) -->
            <div v-if="currentStep === 5 && endpointType === 'container'" class="space-y-6">
              <p class="text-sm text-gray-400 mb-4">
                Configure special permissions and network settings for the container.
              </p>

              <!-- Host Networking -->
              <div class="border border-gray-700 rounded p-4">
                <div class="flex items-start gap-3">
                  <input
                    v-model="hostNetworking"
                    type="checkbox"
                    id="host-networking"
                    class="mt-1 w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600 focus:ring-blue-500"
                  />
                  <div>
                    <label for="host-networking" class="block text-sm font-medium text-gray-300">
                      Use host networking
                    </label>
                    <p class="text-sm text-gray-400 mt-1">
                      Container uses the host's network stack directly. Useful for containers that need to bind to specific ports or access local services.
                    </p>
                    <p class="text-xs text-yellow-400 mt-2">
                      ⚠️ Warning: This bypasses Docker network isolation and may pose security risks.
                    </p>
                  </div>
                </div>
              </div>

              <!-- Docker Socket Access -->
              <div class="border border-gray-700 rounded p-4">
                <div class="flex items-start gap-3">
                  <input
                    v-model="dockerSocketAccess"
                    type="checkbox"
                    id="docker-socket-access"
                    class="mt-1 w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600 focus:ring-blue-500"
                  />
                  <div>
                    <label for="docker-socket-access" class="block text-sm font-medium text-gray-300">
                      Allow Docker socket access
                    </label>
                    <p class="text-sm text-gray-400 mt-1">
                      Mount the Docker socket into the container, allowing it to control Docker on the host.
                    </p>
                    <p class="text-xs text-gray-500 mt-2">
                      Unix: /var/run/docker.sock | Windows: //./pipe/docker_engine
                    </p>
                    <p class="text-xs text-red-400 mt-2">
                      ⚠️ Danger: Container will have full control over Docker. Only enable for trusted images.
                    </p>
                  </div>
                </div>
              </div>
            </div>

            <!-- Step 6: Proxy Configuration (Container) -->
            <div v-if="currentStep === 6 && endpointType === 'container'" class="space-y-6">
              <p class="text-sm text-gray-400 mb-4">
                Configure how requests are proxied to your container. The backend URL will automatically point to your container.
              </p>

              <!-- Backend URL (optional override) -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Backend URL (Optional Override)
                </label>
                <input
                  v-model="backendURL"
                  type="text"
                  :placeholder="`http://localhost:${containerPort} (auto-detected)`"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p class="mt-2 text-sm text-gray-400">
                  Leave empty to auto-detect from container port. Override only if needed.
                </p>
              </div>

              <!-- Timeout -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Request Timeout (seconds)
                </label>
                <input
                  v-model.number="proxyTimeout"
                  type="number"
                  min="1"
                  max="300"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p class="mt-2 text-sm text-gray-400">
                  Maximum time to wait for container response
                </p>
              </div>

              <!-- Request Headers -->
              <div>
                <div class="flex items-center justify-between mb-3">
                  <h4 class="text-sm font-medium text-gray-300">Request Headers (to container)</h4>
                  <button
                    v-if="requestHeaders.length > 0"
                    @click="loadDefaultContainerHeaders"
                    class="text-xs text-blue-400 hover:text-blue-300 transition-colors"
                  >
                    Reset to Defaults
                  </button>
                </div>
                <p class="text-sm text-gray-400 mb-3">
                  These headers are automatically configured for container communication
                </p>
                <HeaderManipulationList v-model="requestHeaders" direction="inbound" />
              </div>

              <!-- Response Headers -->
              <div>
                <h4 class="text-sm font-medium text-gray-300 mb-3">Response Headers (from container)</h4>
                <p class="text-sm text-gray-400 mb-3">
                  Manipulate headers before returning to the client
                </p>
                <HeaderManipulationList v-model="responseHeaders" direction="outbound" />
              </div>

              <!-- Info Box -->
              <div class="p-3 bg-blue-900/20 border border-blue-800 rounded">
                <p class="text-sm text-blue-300">
                  <strong>Note:</strong> Default headers are pre-configured for optimal container communication.
                  Headers like X-Forwarded-For, X-Real-IP, and Host are automatically set to forward client information to the container.
                </p>
              </div>
            </div>

            <!-- Step 7: Test Container (Container) -->
            <div v-if="currentStep === 7 && endpointType === 'container'" class="space-y-6">
              <p class="text-sm text-gray-400 mb-4">
                Test your container configuration by starting a temporary container. This verifies that the image can be pulled,
                the container starts successfully, and responds on the configured port.
              </p>

              <!-- Test Status Display -->
              <div class="border border-gray-700 rounded p-4">
                <div class="flex items-start gap-3">
                  <!-- Status Icon -->
                  <div class="flex-shrink-0 mt-1">
                    <!-- Idle state -->
                    <svg v-if="containerTestStatus === 'idle'" class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <!-- Testing state (spinner) -->
                    <svg v-else-if="containerTestStatus === 'testing'" class="w-5 h-5 text-blue-400 animate-spin" fill="none" viewBox="0 0 24 24">
                      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    <!-- Success state -->
                    <svg v-else-if="containerTestStatus === 'success'" class="w-5 h-5 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <!-- Error state -->
                    <svg v-else-if="containerTestStatus === 'error'" class="w-5 h-5 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                  </div>

                  <!-- Status Text -->
                  <div class="flex-1">
                    <div class="flex items-center gap-2">
                      <span :class="[
                        'text-sm font-medium',
                        containerTestStatus === 'idle' ? 'text-gray-300' :
                        containerTestStatus === 'testing' ? 'text-blue-400' :
                        containerTestStatus === 'success' ? 'text-green-400' :
                        'text-red-400'
                      ]">
                        <template v-if="containerTestStatus === 'idle'">Ready to Test</template>
                        <template v-else-if="containerTestStatus === 'testing'">Testing...</template>
                        <template v-else-if="containerTestStatus === 'success'">Test Passed</template>
                        <template v-else>Test Failed</template>
                      </span>
                      <span v-if="containerTestSkipped" class="text-xs text-yellow-400">(Skipped)</span>
                    </div>
                    <p v-if="containerTestMessage" :class="[
                      'text-sm mt-1',
                      containerTestStatus === 'success' ? 'text-green-300' :
                      containerTestStatus === 'error' ? 'text-red-300' :
                      'text-gray-400'
                    ]">
                      {{ containerTestMessage }}
                    </p>
                  </div>
                </div>
              </div>

              <!-- Action Buttons -->
              <div class="flex gap-3">
                <button
                  @click="handleTestContainer"
                  :disabled="containerTestStatus === 'testing'"
                  class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded transition-colors flex items-center gap-2"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  Test Container
                </button>
                <button
                  @click="handleSkipTest"
                  :disabled="containerTestStatus === 'testing'"
                  class="px-4 py-2 bg-gray-700 hover:bg-gray-600 disabled:bg-gray-600 disabled:cursor-not-allowed text-gray-300 rounded transition-colors"
                >
                  Skip Test
                </button>
              </div>

              <!-- Info Box -->
              <div class="p-3 bg-blue-900/20 border border-blue-800 rounded">
                <p class="text-sm text-blue-300">
                  <strong>Note:</strong> Testing will start a temporary container using your configuration.
                  The container will be automatically removed after testing. You can skip this step if you're confident in your settings.
                </p>
              </div>
            </div>

            <!-- Step 2: Backend Configuration (Proxy) -->
            <div v-if="currentStep === 2 && endpointType === 'proxy'" class="space-y-6">
              <!-- Backend URL -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Backend URL <span class="text-red-400">*</span>
                </label>
                <input
                  v-model="backendURL"
                  type="text"
                  placeholder="e.g., http://localhost:3000, https://api.example.com"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p class="mt-2 text-sm text-gray-400">
                  The backend server URL where requests will be forwarded
                </p>
              </div>

              <!-- Timeout -->
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  Request Timeout (seconds)
                </label>
                <input
                  v-model.number="proxyTimeout"
                  type="number"
                  min="1"
                  max="300"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p class="mt-2 text-sm text-gray-400">
                  Maximum time to wait for backend response before timing out
                </p>
              </div>

              <!-- Status Passthrough -->
              <div class="flex items-start gap-3">
                <input
                  v-model="statusPassthrough"
                  type="checkbox"
                  id="status-passthrough"
                  class="mt-1 w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600 focus:ring-blue-500"
                />
                <div>
                  <label for="status-passthrough" class="block text-sm font-medium text-gray-300">
                    Pass through backend status codes
                  </label>
                  <p class="text-sm text-gray-400 mt-1">
                    Return the exact status code from the backend (disable to use status code translations)
                  </p>
                </div>
              </div>
            </div>

            <!-- Step 3: Headers & Status Codes (Proxy) -->
            <div v-if="currentStep === 3 && endpointType === 'proxy'" class="space-y-6">
              <!-- Request Headers -->
              <div>
                <h4 class="text-sm font-medium text-gray-300 mb-3">Request Headers</h4>
                <p class="text-sm text-gray-400 mb-3">
                  Manipulate headers before forwarding to the backend
                </p>
                <HeaderManipulationList v-model="requestHeaders" direction="inbound" />
              </div>

              <!-- Response Headers -->
              <div>
                <h4 class="text-sm font-medium text-gray-300 mb-3">Response Headers</h4>
                <p class="text-sm text-gray-400 mb-3">
                  Manipulate headers before returning to the client
                </p>
                <HeaderManipulationList v-model="responseHeaders" direction="outbound" />
              </div>

              <!-- Status Translations -->
              <div v-if="!statusPassthrough">
                <h4 class="text-sm font-medium text-gray-300 mb-3">Status Code Translations</h4>
                <p class="text-sm text-gray-400 mb-3">
                  Transform backend status codes to different codes for the client
                </p>
                <StatusTranslationList v-model="statusTranslations" />
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-gray-700 flex justify-between flex-shrink-0">
            <button
              v-if="currentStep > 1"
              @click="handleBack"
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors"
            >
              Back
            </button>
            <div v-else></div>

            <div class="flex gap-3">
              <button
                @click="handleCancel"
                class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors"
              >
                Cancel
              </button>
              <button
                @click="handleNext"
                :disabled="!canGoNext"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {{ currentStep < totalSteps ? 'Next' : 'Create Endpoint' }}
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

.modal-enter-active > div,
.modal-leave-active > div {
  transition: transform 0.2s ease;
}

.modal-enter-from > div,
.modal-leave-to > div {
  transform: scale(0.95);
}
</style>
