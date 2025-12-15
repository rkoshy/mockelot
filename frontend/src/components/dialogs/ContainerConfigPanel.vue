<script lang="ts" setup>
import { ref, computed } from 'vue'
import { ValidateAndInspectDockerImage, PullDockerImage, RestartContainer } from '../../../wailsjs/go/main/App'
import VolumeList from './VolumeList.vue'
import EnvironmentVarList from './EnvironmentVarList.vue'
import { models } from '../../../wailsjs/go/models'

const props = defineProps<{
  config: models.ContainerConfig
  endpointId?: string
  isRunning?: boolean
}>()

const emit = defineEmits<{
  'update:config': [config: models.ContainerConfig]
}>()

// Local state
const imageName = ref(props.config.image_name || '')
const containerPort = ref(props.config.container_port || 80)
const pullOnStartup = ref(props.config.pull_on_startup !== false)
const volumes = ref<models.VolumeMapping[]>(props.config.volumes || [])
const environment = ref<models.EnvironmentVar[]>(props.config.environment || [])
const exposedPorts = ref<string[]>(props.config.exposed_ports || [])

// Image inspection result
const imageInfo = ref<models.DockerImageInfo | null>(null)
const selectedPort = ref<string>('')  // For radio button selection when multiple ports available

// Initialize selectedPort if we have exposed_ports
if (exposedPorts.value.length > 0) {
  const currentPortStr = containerPort.value.toString()
  const portInList = exposedPorts.value.some(p => p.startsWith(currentPortStr + '/'))

  if (portInList) {
    selectedPort.value = currentPortStr
    // Reconstruct imageInfo if we have ports but no imageInfo
    if (!imageInfo.value) {
      imageInfo.value = {
        image_name: imageName.value,
        exposed_ports: exposedPorts.value,
        volumes: [],
        environment: {},
        is_http_service: false
      } as models.DockerImageInfo
    }
  } else {
    selectedPort.value = 'other'
  }
} else {
  selectedPort.value = ''
}

// Sub-tab state
const activeSubTab = ref<'image' | 'volumes' | 'environment'>('image')

// Image pull state
const pullingImage = ref(false)
const imagePullResult = ref<{ success: boolean; message: string } | null>(null)

// Image validation state
const validatingImage = ref(false)
const imageValidationResult = ref<{ success: boolean; message: string } | null>(null)

// Container restart state
const restartingContainer = ref(false)

// Computed config object
// Note: proxy_config is managed separately at the top level (in EndpointSettingsDialog)
const updatedConfig = computed((): models.ContainerConfig => new models.ContainerConfig({
  image_name: imageName.value,
  container_port: containerPort.value,
  exposed_ports: exposedPorts.value,
  pull_on_startup: pullOnStartup.value,
  volumes: volumes.value,
  environment: environment.value,
  proxy_config: props.config.proxy_config // Preserve existing proxy_config
}))

// Emit updates
function emitUpdate() {
  emit('update:config', updatedConfig.value)
}

// Pull Docker image
async function pullImage() {
  if (!imageName.value.trim()) {
    imagePullResult.value = {
      success: false,
      message: 'Please enter an image name first'
    }
    return
  }

  pullingImage.value = true
  imagePullResult.value = null

  try {
    await PullDockerImage(imageName.value)
    imagePullResult.value = {
      success: true,
      message: 'Image pulled successfully'
    }
  } catch (error) {
    imagePullResult.value = {
      success: false,
      message: String(error).replace('Error: ', '')
    }
  } finally {
    pullingImage.value = false
  }
}

// Validate Docker image
async function validateImage() {
  if (!imageName.value.trim()) {
    imageValidationResult.value = {
      success: false,
      message: 'Please enter an image name first'
    }
    return
  }

  validatingImage.value = true
  imageValidationResult.value = null

  try {
    // Inspect the image (validates and extracts metadata)
    const info = await ValidateAndInspectDockerImage(imageName.value.trim())
    imageInfo.value = info
    imageValidationResult.value = {
      success: true,
      message: 'Image inspected successfully'
    }

    // Store exposed ports in config
    if (info.exposed_ports && info.exposed_ports.length > 0) {
      exposedPorts.value = info.exposed_ports

      // Auto-select first port if current port not in list
      const firstPort = info.exposed_ports[0].split('/')[0]
      const currentPortStr = containerPort.value.toString()
      const portInList = info.exposed_ports.some(p => p.startsWith(currentPortStr + '/'))

      if (portInList) {
        selectedPort.value = currentPortStr
      } else {
        selectedPort.value = firstPort
        containerPort.value = parseInt(firstPort, 10)
      }
    } else {
      exposedPorts.value = []
      selectedPort.value = ''
    }

    emitUpdate()
  } catch (error) {
    imageValidationResult.value = {
      success: false,
      message: String(error).replace('Error: ', '')
    }
    imageInfo.value = null
    exposedPorts.value = []
  } finally {
    validatingImage.value = false
  }
}

// Restart container
async function restartContainer() {
  if (!props.endpointId) return

  restartingContainer.value = true
  try {
    await RestartContainer(props.endpointId)
  } catch (error) {
    console.error('Failed to restart container:', error)
  } finally {
    restartingContainer.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <h3 class="text-lg font-semibold text-white border-b border-gray-700 pb-2">
      Container Configuration
    </h3>

    <!-- Sub-Tabs -->
    <div class="flex border-b border-gray-700">
      <button
        @click="activeSubTab = 'image'"
        :class="[
          'px-3 py-2 text-sm font-medium transition-colors',
          activeSubTab === 'image'
            ? 'text-blue-400 border-b-2 border-blue-400'
            : 'text-gray-400 hover:text-gray-300'
        ]"
      >
        Image
      </button>
      <button
        @click="activeSubTab = 'volumes'"
        :class="[
          'px-3 py-2 text-sm font-medium transition-colors',
          activeSubTab === 'volumes'
            ? 'text-blue-400 border-b-2 border-blue-400'
            : 'text-gray-400 hover:text-gray-300'
        ]"
      >
        Volumes
      </button>
      <button
        @click="activeSubTab = 'environment'"
        :class="[
          'px-3 py-2 text-sm font-medium transition-colors',
          activeSubTab === 'environment'
            ? 'text-blue-400 border-b-2 border-blue-400'
            : 'text-gray-400 hover:text-gray-300'
        ]"
      >
        Environment
      </button>
    </div>

    <!-- Image Tab -->
    <div v-if="activeSubTab === 'image'" class="space-y-6 p-4">
      <!-- Image Name -->
    <div>
      <label class="block text-sm font-medium text-gray-300 mb-2">
        Docker Image *
      </label>
      <div class="flex gap-2">
        <input
          v-model="imageName"
          @blur="emitUpdate"
          type="text"
          placeholder="nginx:latest"
          class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400
                 focus:outline-none focus:border-blue-500"
        />
        <button
          @click="pullImage"
          :disabled="pullingImage || !imageName.trim()"
          class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors
                 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
        >
          <svg v-if="pullingImage" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
          </svg>
          <span>{{ pullingImage ? 'Pulling...' : 'Pull' }}</span>
        </button>
        <button
          @click="validateImage"
          :disabled="validatingImage || !imageName.trim()"
          class="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded transition-colors
                 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
        >
          <svg v-if="validatingImage" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
          </svg>
          <span>{{ validatingImage ? 'Checking...' : 'Validate' }}</span>
        </button>
      </div>
      <p v-if="imagePullResult" :class="[
        'mt-2 text-sm',
        imagePullResult.success ? 'text-green-400' : 'text-red-400'
      ]">
        {{ imagePullResult.success ? '✓' : '✗' }} {{ imagePullResult.message }}
      </p>
      <p v-if="imageValidationResult" :class="[
        'mt-2 text-sm',
        imageValidationResult.success ? 'text-green-400' : 'text-yellow-400'
      ]">
        {{ imageValidationResult.success ? '✓' : '⚠' }} {{ imageValidationResult.message }}
      </p>
      <p class="mt-1 text-xs text-gray-400">
        Container image name and tag (e.g., nginx:latest, postgres:15-alpine)
      </p>
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
            @change="containerPort = parseInt(selectedPort, 10); emitUpdate()"
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
        @blur="emitUpdate"
        type="number"
        min="1"
        max="65535"
        placeholder="e.g., 8080"
        class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400
               focus:outline-none focus:border-blue-500"
      />
      <p class="mt-1 text-xs text-gray-400">
        The port inside the container that your application listens on
      </p>
    </div>

      <!-- Pull on Startup -->
      <div>
        <label class="flex items-center gap-2 cursor-pointer">
          <input
            v-model="pullOnStartup"
            @change="emitUpdate"
            type="checkbox"
            class="w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600
                   focus:ring-2 focus:ring-blue-500"
          />
          <span class="text-sm text-gray-300">
            Pull image on server startup
          </span>
        </label>
        <p class="ml-6 mt-1 text-xs text-gray-400">
          Automatically pull the latest image when the mock server starts
        </p>
      </div>

      <!-- Info Box -->
      <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
        <p class="text-sm font-medium text-blue-300 mb-2">About Container Endpoints</p>
        <div class="space-y-2 text-xs text-blue-200">
          <p>
            Container endpoints run Docker containers to handle requests. The container is started when
            the mock server starts and stopped when it stops.
          </p>
          <p class="text-gray-300">
            <strong>Port Mapping:</strong> Mockelot automatically maps the container port to a random host port.
            Requests matching the path prefix are forwarded to the container.
          </p>
          <p class="text-yellow-300">
            <strong>Security:</strong> Be careful with volume mappings and environment variables.
            Only map directories the container needs and avoid exposing sensitive data.
          </p>
        </div>
      </div>
    </div>

    <!-- Volumes Tab -->
    <div v-if="activeSubTab === 'volumes'" class="space-y-6 p-4">
      <VolumeList
        v-model="volumes"
        @update:modelValue="emitUpdate"
      />
    </div>

    <!-- Environment Tab -->
    <div v-if="activeSubTab === 'environment'" class="space-y-6 p-4">
      <EnvironmentVarList
        v-model="environment"
        @update:modelValue="emitUpdate"
      />
    </div>
  </div>
</template>
