<script lang="ts" setup>
import { ref, watch } from 'vue'
import { models } from '../../../wailsjs/go/models'

const props = defineProps<{
  modelValue: models.VolumeMapping[]
}>()

const emit = defineEmits<{
  'update:modelValue': [volumes: models.VolumeMapping[]]
}>()

interface VolumeRow extends models.VolumeMapping {
  id: string
}

const volumes = ref<VolumeRow[]>([])

// Initialize with props
if (props.modelValue && props.modelValue.length > 0) {
  volumes.value = props.modelValue.map((v, i) => ({
    ...v,
    id: `volume-${i}-${Date.now()}`
  }))
}

// Add new volume row
function addVolume() {
  volumes.value.push({
    id: `volume-${volumes.value.length}-${Date.now()}`,
    host_path: '',
    container_path: '',
    read_only: false
  })
  emitVolumes()
}

// Remove volume row
function removeVolume(index: number) {
  volumes.value.splice(index, 1)
  emitVolumes()
}

// Emit volumes update
function emitVolumes() {
  const validVolumes = volumes.value
    .filter(v => v.host_path.trim() !== '' && v.container_path.trim() !== '')
    .map(({ id, ...v }) => v)
  emit('update:modelValue', validVolumes)
}

// Watch for changes
watch(volumes, () => {
  emitVolumes()
}, { deep: true })
</script>

<template>
  <div class="space-y-4">
    <!-- Volumes Table -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <h4 class="text-sm font-medium text-white">Volume Mappings</h4>
        <button
          @click="addVolume"
          class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded transition-colors flex items-center gap-1"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Add Volume
        </button>
      </div>

      <!-- Volume Rows -->
      <div v-if="volumes.length > 0" class="space-y-3">
        <div
          v-for="(volume, index) in volumes"
          :key="volume.id"
          class="flex gap-2 items-start p-3 bg-gray-700/50 rounded border border-gray-600"
        >
          <div class="flex-1 space-y-2">
            <!-- Host Path -->
            <div>
              <label class="block text-xs text-gray-400 mb-1">Host Path</label>
              <input
                v-model="volume.host_path"
                type="text"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono
                       focus:outline-none focus:border-blue-500"
                placeholder="/path/on/host"
              />
            </div>

            <!-- Container Path -->
            <div>
              <label class="block text-xs text-gray-400 mb-1">Container Path</label>
              <input
                v-model="volume.container_path"
                type="text"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono
                       focus:outline-none focus:border-blue-500"
                placeholder="/path/in/container"
              />
            </div>

            <!-- Read Only -->
            <div>
              <label class="flex items-center gap-2 cursor-pointer">
                <input
                  v-model="volume.read_only"
                  type="checkbox"
                  class="w-4 h-4 bg-gray-700 border-gray-600 rounded text-blue-600
                         focus:ring-2 focus:ring-blue-500"
                />
                <span class="text-xs text-gray-300">
                  Read-only mount
                </span>
              </label>
            </div>
          </div>

          <!-- Remove Button -->
          <button
            @click="removeVolume(index)"
            class="p-2 text-gray-400 hover:text-red-400 transition-colors mt-6"
            title="Remove volume mapping"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <div v-else class="text-center py-8 text-gray-400 text-sm">
        No volume mappings. Click "Add Volume" to create one.
      </div>
    </div>

    <!-- Helper Info -->
    <div class="p-4 bg-gray-700/50 rounded border border-gray-600">
      <p class="text-sm font-medium text-white mb-2">Volume Mapping</p>
      <div class="space-y-2 text-xs text-gray-300">
        <p>Volume mappings allow the container to access directories from the host system.</p>
        <p class="text-yellow-400 font-medium">Security Warning:</p>
        <p class="text-gray-400">
          Be careful when mapping host directories. Only map directories that the container needs access to.
          Consider using read-only mounts when possible.
        </p>
      </div>
    </div>

    <!-- Examples -->
    <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
      <p class="text-sm font-medium text-blue-300 mb-2">Example Use Cases</p>
      <div class="space-y-3 text-xs text-blue-200">
        <div>
          <p class="font-medium text-blue-300">Serve static files:</p>
          <p class="font-mono text-gray-300 mt-1">Host: /home/user/www</p>
          <p class="font-mono text-gray-300">Container: /usr/share/nginx/html</p>
          <p class="text-gray-400 mt-1">Mount local files into nginx container</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">Persistent data:</p>
          <p class="font-mono text-gray-300 mt-1">Host: /var/data/app</p>
          <p class="font-mono text-gray-300">Container: /data</p>
          <p class="text-gray-400 mt-1">Preserve data across container restarts</p>
        </div>
        <div>
          <p class="font-medium text-blue-300">Configuration files (read-only):</p>
          <p class="font-mono text-gray-300 mt-1">Host: /etc/app/config.json</p>
          <p class="font-mono text-gray-300">Container: /app/config.json</p>
          <p class="font-mono text-gray-300">Read-only: ✓</p>
          <p class="text-gray-400 mt-1">Mount config without allowing container to modify it</p>
        </div>
      </div>
    </div>
  </div>
</template>
