<script lang="ts" setup>
import { ref, inject, onMounted, onUnmounted } from 'vue'
import { useServerStore } from '../../stores/server'
import ResponseRuleCard from './ResponseRuleCard.vue'
import ResponseGroupCard from './ResponseGroupCard.vue'
import AddEndpointDialog from '../dialogs/AddEndpointDialog.vue'
import EndpointSettingsDialog from '../dialogs/EndpointSettingsDialog.vue'
import ConfirmDialog from '../dialogs/ConfirmDialog.vue'
import ContainerConsoleDialog from '../dialogs/ContainerConsoleDialog.vue'
import TrafficLogPanel from '../traffic/TrafficLogPanel.vue'
import ServerTab from './tabs/ServerTab.vue'
import { models } from '../../types/models'
import { StartContainer, StopContainer, DeleteContainer } from '../../../wailsjs/go/main/App'

const serverStore = useServerStore()

// Track selected tab (server vs endpoint)
const selectedTab = ref<'server' | string>('server')  // Default to Server tab

// Inject event registration function from HeaderBar
type EventCallback = (data: any) => void
const registerEventListener = inject<(eventName: string, callback: EventCallback) => () => void>('registerEventListener')

// Container control state
// Track which action is currently loading for each endpoint ('start', 'stop', 'delete', 'restart', or '')
const containerActionLoading = ref<Record<string, string>>({})
const containerActionError = ref<Record<string, string>>({})

// Container progress state
interface ContainerProgress {
  endpoint_id: string
  stage: string       // pulling, creating, starting, ready
  message: string
  progress: number    // 0-100
}
const containerProgress = ref<Record<string, ContainerProgress>>({})
let unregisterProgressListener: (() => void) | null = null

// Resizable divider state
const dividerPosition = ref(550) // pixels
const isDraggingDivider = ref(false)

// Dialog state
const showAddEndpointDialog = ref(false)
const showEndpointSettingsDialog = ref(false)
const showDeleteConfirmDialog = ref(false)
const showContainerConsoleDialog = ref(false)
const endpointToDelete = ref<string>('')
const consoleEndpointId = ref<string>('')
const consoleEndpointName = ref<string>('')

// Drag and drop state
const draggedIndex = ref<number | null>(null)
const dragOverIndex = ref<number | null>(null)

function onDragStart(index: number, event: DragEvent) {
  // Prevent dragging system endpoints
  const endpoint = serverStore.endpoints[index]
  if (endpoint?.is_system) {
    event.preventDefault()
    return
  }

  draggedIndex.value = index
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(index))
  }
}

function onDragOver(index: number, event: DragEvent) {
  // Prevent dropping on system endpoints
  const endpoint = serverStore.endpoints[index]
  if (endpoint?.is_system) {
    return
  }

  event.preventDefault()
  dragOverIndex.value = index
}

function onDrop(index: number, event: DragEvent) {
  event.preventDefault()

  // Prevent dropping on system endpoints
  const endpoint = serverStore.endpoints[index]
  if (endpoint?.is_system) {
    draggedIndex.value = null
    dragOverIndex.value = null
    return
  }

  if (draggedIndex.value === null || draggedIndex.value === index) {
    draggedIndex.value = null
    dragOverIndex.value = null
    return
  }

  serverStore.reorderItems(draggedIndex.value, index)

  draggedIndex.value = null
  dragOverIndex.value = null
}

function onDragEnd() {
  draggedIndex.value = null
  dragOverIndex.value = null
}

function selectEndpoint(id: string) {
  selectedTab.value = id
  serverStore.selectEndpoint(id)
}

function getItemId(item: models.ResponseItem): string {
  return item.type === 'response' ? item.response?.id || '' : item.group?.id || ''
}

async function handleResponseUpdate(index: number, response: models.MethodResponse) {
  const item = new models.ResponseItem({
    type: 'response',
    response: response
  })
  await serverStore.updateItem(index, item)
}

async function handleGroupUpdate(index: number, group: models.ResponseGroup) {
  const item = new models.ResponseItem({
    type: 'group',
    group: group
  })
  await serverStore.updateItem(index, item)
}

async function handleDelete(index: number) {
  await serverStore.removeItem(index)
}

// Endpoint actions
async function handleAddEndpoint(config: any) {
  try {
    await serverStore.addNewEndpointWithConfig(config)
    showAddEndpointDialog.value = false
  } catch (error) {
    console.error('Failed to add endpoint:', error)
  }
}

function handleCancelAddEndpoint() {
  showAddEndpointDialog.value = false
}

async function handleSaveEndpointSettings(endpoint: models.Endpoint) {
  try {
    await serverStore.updateEndpointById(endpoint)
    showEndpointSettingsDialog.value = false
  } catch (error) {
    console.error('Failed to update endpoint:', error)
  }
}

function handleDeleteEndpoint() {
  if (!serverStore.currentEndpoint) return

  endpointToDelete.value = serverStore.currentEndpoint.name
  showDeleteConfirmDialog.value = true
}

async function confirmDeleteEndpoint() {
  if (!serverStore.currentEndpoint) return

  try {
    await serverStore.deleteEndpointById(serverStore.currentEndpoint.id)
    showEndpointSettingsDialog.value = false
    showDeleteConfirmDialog.value = false
  } catch (error) {
    console.error('Failed to delete endpoint:', error)
    showDeleteConfirmDialog.value = false
  }
}

function cancelDeleteEndpoint() {
  showDeleteConfirmDialog.value = false
  endpointToDelete.value = ''
}

function handleCancelEndpointSettings() {
  showEndpointSettingsDialog.value = false
}

// Type badge helpers
function typeBadgeClass(type: string): string {
  switch (type) {
    case 'proxy':
      return 'bg-green-900 text-green-300'
    case 'container':
      return 'bg-purple-900 text-purple-300'
    case 'mock':
    default:
      return 'bg-blue-900 text-blue-300'
  }
}

function typeDisplayName(type: string): string {
  switch (type) {
    case 'proxy':
      return 'Proxy'
    case 'container':
      return 'Container'
    case 'mock':
    default:
      return 'Mock'
  }
}

// Health indicator helpers
function needsHealthIndicator(endpoint: models.Endpoint): boolean {
  if (endpoint.type === 'proxy' && endpoint.proxy_config?.health_check_enabled) {
    return true
  }
  if (endpoint.type === 'container' && endpoint.container_config?.proxy_config?.health_check_enabled) {
    return true
  }
  return false
}

function healthIndicatorClass(endpointId: string): string {
  const health = serverStore.getEndpointHealth(endpointId)
  if (!health) {
    return 'text-gray-500'
  }
  return health.healthy ? 'text-green-400' : 'text-red-400'
}

// Container status helpers
function containerStatusClass(endpointId: string): string {
  const status = serverStore.getContainerStatus(endpointId)
  if (!status) {
    return 'bg-gray-900/30 border-gray-700 text-gray-400'
  }
  if (status.gone) {
    return 'bg-orange-900/30 border-orange-700 text-orange-400'
  }
  if (!status.running) {
    return 'bg-red-900/30 border-red-700 text-red-400'
  }
  return 'bg-green-900/30 border-green-700 text-green-400'
}

function containerStatusText(endpointId: string): string {
  const status = serverStore.getContainerStatus(endpointId)
  if (!status) {
    return 'NS' // Not Started
  }
  if (status.gone) {
    return '!' // Gone/Missing
  }
  if (status.running) {
    return 'R' // Running
  }
  // Map Docker status to short display text
  switch (status.status) {
    case 'exited':
      return 'E' // Exited
    case 'dead':
      return 'D' // Dead
    case 'paused':
      return 'P' // Paused
    case 'restarting':
      return 'RS' // Restarting
    default:
      return 'S' // Stopped
  }
}

// Container control helpers
function canStartContainer(endpointId: string): boolean {
  const status = serverStore.getContainerStatus(endpointId)
  if (!status) return true // Not started yet, can start
  return !status.running || status.gone // Can start if not running or gone
}

function canStopContainer(endpointId: string): boolean {
  const status = serverStore.getContainerStatus(endpointId)
  if (!status) return false // Not started, can't stop
  return status.running && !status.gone // Can stop if running and not gone
}

function canDeleteContainer(endpointId: string): boolean {
  const status = serverStore.getContainerStatus(endpointId)
  if (!status) return false // Not started, nothing to delete
  return !status.gone // Can delete if not already gone
}

// Container control actions
async function handleStartContainer(endpointId: string) {
  // Find endpoint to get name
  const endpoint = serverStore.endpoints.find(ep => ep.id === endpointId)
  if (!endpoint) {
    console.error('Endpoint not found:', endpointId)
    return
  }

  // HeaderBar will show progress dialog when it receives ctr:progress events
  containerActionLoading.value[endpointId] = 'start'
  containerActionError.value[endpointId] = ''

  try {
    await StartContainer(endpointId)
  } catch (error) {
    containerActionError.value[endpointId] = String(error)
    console.error('Failed to start container:', error)
  } finally {
    containerActionLoading.value[endpointId] = ''
  }
}

async function handleStopContainer(endpointId: string) {
  containerActionLoading.value[endpointId] = 'stop'
  containerActionError.value[endpointId] = ''

  try {
    await StopContainer(endpointId)
  } catch (error) {
    containerActionError.value[endpointId] = String(error)
    console.error('Failed to stop container:', error)
  } finally {
    containerActionLoading.value[endpointId] = ''
  }
}

async function handleDeleteContainer(endpointId: string) {
  containerActionLoading.value[endpointId] = 'delete'
  containerActionError.value[endpointId] = ''

  try {
    await DeleteContainer(endpointId)
  } catch (error) {
    containerActionError.value[endpointId] = String(error)
    console.error('Failed to delete container:', error)
  } finally {
    containerActionLoading.value[endpointId] = ''
  }
}

async function handleRestartContainer(endpointId: string) {
  containerActionLoading.value[endpointId] = 'restart'
  containerActionError.value[endpointId] = ''

  try {
    await serverStore.restartContainerEndpoint(endpointId)
  } catch (error) {
    containerActionError.value[endpointId] = String(error)
    console.error('Failed to restart container:', error)
  } finally {
    containerActionLoading.value[endpointId] = ''
  }
}

function handleShowConsole(endpointId: string, endpointName: string) {
  consoleEndpointId.value = endpointId
  consoleEndpointName.value = endpointName
  showContainerConsoleDialog.value = true
}

function handleCloseConsole() {
  showContainerConsoleDialog.value = false
}

// Resizable divider handlers
function startDragging(event: MouseEvent) {
  isDraggingDivider.value = true
  event.preventDefault()

  const container = event.currentTarget as HTMLElement
  const containerRect = container.parentElement?.getBoundingClientRect()

  const onMouseMove = (e: MouseEvent) => {
    if (!isDraggingDivider.value || !containerRect) return

    const relativeX = e.clientX - containerRect.left

    // Clamp between 200px and container width - 200px
    const minWidth = 200
    const maxWidth = containerRect.width - 200
    dividerPosition.value = Math.max(minWidth, Math.min(maxWidth, relativeX))
  }

  const onMouseUp = () => {
    isDraggingDivider.value = false
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

// Open endpoint settings
function openEndpointSettings(endpoint: models.Endpoint) {
  serverStore.selectEndpoint(endpoint.id)
  showEndpointSettingsDialog.value = true
}

// Container stats wrappers
function getContainerStatus(endpointId: string) {
  return serverStore.getContainerStatus(endpointId)
}

function getContainerStats(endpointId: string) {
  return serverStore.getContainerStats(endpointId)
}

// Formatting helpers for container metrics
function formatCPU(cpuPercent: number): string {
  return `${cpuPercent.toFixed(2)}%`
}

function formatMemory(mb: number): string {
  if (mb < 1024) {
    return `${mb.toFixed(2)} MB`
  }
  return `${(mb / 1024).toFixed(2)} GB`
}

function formatPercent(percent: number): string {
  return `${percent.toFixed(2)}%`
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(2)} KB`
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

// Register for container progress events (for inline progress indicator)
onMounted(() => {
  if (registerEventListener) {
    unregisterProgressListener = registerEventListener('ctr:progress', (data: any) => {
      if (data.endpoint_id) {
        // Update progress state for inline indicator
        containerProgress.value[data.endpoint_id] = {
          endpoint_id: data.endpoint_id,
          stage: data.stage || '',
          message: data.message || '',
          progress: data.progress || 0
        }

        // Clear progress when complete
        if (data.stage === 'ready') {
          setTimeout(() => {
            delete containerProgress.value[data.endpoint_id]
          }, 3000)
        }
      }
    })
  }
})

// Unregister progress listener on unmount
onUnmounted(() => {
  if (unregisterProgressListener) {
    unregisterProgressListener()
    unregisterProgressListener = null
  }
})
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Horizontal Tab Bar -->
    <div class="flex items-stretch border-b border-gray-700 flex-shrink-0 bg-gray-800">
      <div class="flex-1 flex overflow-x-auto">
        <!-- SERVER Tab (NEW) -->
        <div
          :class="[
            'relative px-4 py-2 text-sm font-medium border-r border-gray-700 transition-colors flex items-center gap-1 min-w-[100px] cursor-pointer',
            selectedTab === 'server'
              ? 'bg-gray-900 text-white border-b-2 border-b-blue-500'
              : 'text-gray-400 hover:text-gray-200 hover:bg-gray-750'
          ]"
          @click="selectedTab = 'server'"
        >
          <span class="font-semibold">Server</span>
        </div>

        <!-- Endpoint Tabs -->
        <div
          v-for="endpoint in serverStore.endpoints"
          :key="endpoint.id"
          :class="[
            'relative px-3 py-2 text-sm font-medium border-r border-gray-700 transition-colors flex flex-col items-start gap-1 min-w-[140px] group',
            selectedTab === endpoint.id
              ? 'bg-gray-900 text-white border-b-2'
              : 'text-gray-400 hover:text-gray-200 hover:bg-gray-750',
            endpoint.name === 'Rejections'
              ? 'border-b-2 border-b-red-600'
              : selectedTab === endpoint.id
                ? 'border-b-blue-500'
                : '',
            endpoint.is_system
              ? 'cursor-default bg-gray-800/50 border-l-2 border-l-yellow-600'
              : 'cursor-pointer'
          ]"
        >
          <!-- Invisible clickable overlay covering entire tab -->
          <div
            @click="selectEndpoint(endpoint.id)"
            class="absolute inset-0 z-0"
            :title="`Switch to ${endpoint.name}`"
          ></div>

          <!-- Row 1: Name, Type, Status, Health, Settings -->
          <div class="flex items-center gap-1.5 w-full relative z-10 pointer-events-none">
            <span class="font-semibold truncate flex-shrink">{{ endpoint.name }}</span>

            <!-- System Endpoint Badge -->
            <span
              v-if="endpoint.is_system"
              class="px-1 py-0.5 text-[10px] rounded font-medium flex-shrink-0 bg-yellow-900/50 text-yellow-400 border border-yellow-700"
              title="System endpoint - cannot be deleted or reordered"
            >
              SYS
            </span>

            <!-- Type Badge -->
            <span
              :class="[
                'px-1 py-0.5 text-[10px] rounded font-medium flex-shrink-0',
                typeBadgeClass(endpoint.type || 'mock')
              ]"
            >
              {{ typeDisplayName(endpoint.type || 'mock')[0] }}
            </span>

            <!-- Container Status Indicator -->
            <span
              v-if="endpoint.type === 'container'"
              :class="[
                'px-1 py-0.5 text-[10px] rounded font-bold border flex-shrink-0',
                containerStatusClass(endpoint.id)
              ]"
              :title="'Container: ' + containerStatusText(endpoint.id)"
            >
              {{ containerStatusText(endpoint.id) }}
            </span>

            <!-- Health Indicator -->
            <span
              v-if="needsHealthIndicator(endpoint)"
              :class="[
                'text-sm leading-none flex-shrink-0',
                healthIndicatorClass(endpoint.id)
              ]"
              :title="serverStore.getEndpointHealth(endpoint.id)?.healthy ? 'Healthy' : 'Unhealthy'"
            >
              ●
            </span>
            <span v-if="!endpoint.enabled" class="text-[10px] opacity-50 flex-shrink-0">(off)</span>

            <!-- Settings Gear Icon - always reserve space, only show on selected non-system tabs -->
            <button
              v-if="!endpoint.is_system"
              @click.stop="openEndpointSettings(endpoint)"
              :class="[
                'ml-auto p-1 rounded transition-colors flex-shrink-0 pointer-events-auto relative z-20',
                serverStore.selectedEndpointId === endpoint.id
                  ? 'hover:bg-gray-700 opacity-100'
                  : 'opacity-0 pointer-events-none'
              ]"
              title="Endpoint Settings"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
            </button>
            <!-- Spacer for system endpoints to maintain consistent width -->
            <div v-else class="ml-auto w-6 h-6 flex-shrink-0"></div>
          </div>

          <!-- Row 2: Path prefix and container image (if container) -->
          <div class="flex items-center gap-1.5 text-xs text-gray-500 w-full relative z-10 pointer-events-none">
            <span class="font-mono truncate">{{ endpoint.path_prefix }}</span>
            <span v-if="endpoint.type === 'container' && endpoint.container_config?.image_name" class="truncate text-[10px]">
              • {{ endpoint.container_config.image_name.split(':')[0] }}
            </span>
          </div>
        </div>
        <button
          @click="showAddEndpointDialog = true"
          class="px-4 py-3 text-sm font-medium text-blue-400 hover:text-blue-300 hover:bg-gray-750 whitespace-nowrap flex items-center gap-1 border-r border-gray-700"
          title="Create a new endpoint"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Endpoint
        </button>
      </div>
    </div>

    <!-- Endpoint Controls (only for mock endpoints, not system endpoints) -->
    <div v-if="serverStore.currentEndpoint?.type === 'mock' && !serverStore.currentEndpoint?.is_system" class="flex items-center justify-between p-3 border-b border-gray-700 flex-shrink-0">
      <div class="flex gap-2">
        <button
          @click="serverStore.addNewGroup"
          class="px-3 py-1 bg-blue-800 hover:bg-blue-700 rounded text-sm text-white font-medium flex items-center gap-1"
          title="Add a new group to organize responses"
        >
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
          </svg>
          + Group
        </button>
        <button
          @click="serverStore.addNewResponse"
          class="px-3 py-1 bg-blue-600 hover:bg-blue-700 rounded text-sm text-white font-medium flex items-center gap-1"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          + Response
        </button>
      </div>
    </div>

    <!-- Endpoint Info Banner -->
    <div v-if="serverStore.currentEndpoint" class="px-3 py-2 bg-gray-800/50 border-b border-gray-700 flex-shrink-0">
      <div class="flex items-center justify-between">
        <div class="flex-1">
          <p class="text-xs text-gray-400">
            <span class="font-medium text-gray-300">Type:</span> {{ typeDisplayName(serverStore.currentEndpoint.type || 'mock') }}
            <span class="mx-2">•</span>
            <span class="font-medium text-gray-300">Prefix:</span> {{ serverStore.currentEndpoint.path_prefix }}
            <span class="mx-2">•</span>
            <span class="font-medium text-gray-300">Mode:</span>
            <span v-if="serverStore.currentEndpoint.translation_mode === 'none'">None (use path as-is)</span>
            <span v-else-if="serverStore.currentEndpoint.translation_mode === 'strip'">Strip prefix</span>
            <span v-else>Translate (regex)</span>
            <!-- Proxy-specific info -->
            <template v-if="serverStore.currentEndpoint.type === 'proxy' && serverStore.currentEndpoint.proxy_config">
              <span class="mx-2">•</span>
              <span class="font-medium text-gray-300">Backend:</span> {{ serverStore.currentEndpoint.proxy_config.backend_url }}
            </template>
            <!-- Container-specific info -->
            <template v-if="serverStore.currentEndpoint.type === 'container' && serverStore.currentEndpoint.container_config">
              <span class="mx-2">•</span>
              <span class="font-medium text-gray-300">Image:</span> {{ serverStore.currentEndpoint.container_config.image_name }}
            </template>
          </p>
        </div>
      </div>
    </div>

    <!-- Server Tab Content (NEW) -->
    <ServerTab v-if="selectedTab === 'server'" class="flex-1" />

    <!-- Endpoint Content -->
    <template v-else>
      <!-- Info Banner (Mock only) -->
      <div v-if="serverStore.currentEndpoint?.type === 'mock'" class="px-3 py-2 bg-gray-800/50 border-b border-gray-700 flex-shrink-0">
        <p class="text-xs text-gray-400">
          Rules are checked in order. First matching rule wins. Drag to reorder. Use groups to organize related rules.
        </p>
      </div>

      <!-- Resizable Content Area -->
    <div class="flex-1 flex flex-row min-h-0">
      <!-- Left Section: Mock Rules OR Proxy/Container Status -->
      <div
        :style="{ width: dividerPosition + 'px' }"
        :class="[
          'overflow-y-auto flex flex-col min-h-0',
          serverStore.currentEndpoint?.name === 'Rejections' ? 'bg-red-950/20' : ''
        ]"
      >
        <!-- Mock Endpoint: Rules List -->
        <div v-if="serverStore.currentEndpoint?.type === 'mock'" class="flex-1 overflow-y-auto p-3 space-y-2" @dragend="onDragEnd">
      <!-- Empty State -->
      <div v-if="!serverStore.items || serverStore.items.length === 0" class="flex items-center justify-center h-32">
        <div class="text-center text-gray-500">
          <p class="text-sm">No response rules configured</p>
          <p class="text-xs mt-1">Click "+ Response" or "+ Group" to get started</p>
        </div>
      </div>

      <!-- Items (Responses and Groups) -->
      <div
        v-for="(item, index) in serverStore.items"
        :key="getItemId(item)"
        :class="[
          'transition-all',
          dragOverIndex === index && draggedIndex !== index ? 'border-t-2 border-blue-500 pt-2' : ''
        ]"
      >
        <!-- Response Card -->
        <ResponseRuleCard
          v-if="item.type === 'response' && item.response"
          :response="item.response"
          :is-expanded="serverStore.expandedItemId === item.response.id"
          :index="index"
          @toggle="serverStore.toggleExpanded(item.response?.id || '')"
          @update="handleResponseUpdate(index, $event)"
          @delete="handleDelete(index)"
          @dragstart="onDragStart(index, $event)"
          @dragover="onDragOver(index, $event)"
          @drop="onDrop(index, $event)"
        />

        <!-- Group Card -->
        <ResponseGroupCard
          v-else-if="item.type === 'group' && item.group"
          :group="item.group"
          :index="index"
          @update="handleGroupUpdate(index, $event)"
          @delete="handleDelete(index)"
          @dragstart="onDragStart(index, $event)"
          @dragover="onDragOver(index, $event)"
          @drop="onDrop(index, $event)"
        />
      </div>
        </div>

        <!-- Proxy/Container Endpoint: Status View -->
        <div v-else-if="serverStore.currentEndpoint" class="flex-1 overflow-y-auto p-4">
      <div class="max-w-2xl mx-auto space-y-4">
        <!-- Endpoint Type Info -->
        <div class="p-4 bg-gray-800 rounded border border-gray-700">
          <h3 class="text-lg font-semibold text-white mb-2">
            {{ typeDisplayName(serverStore.currentEndpoint.type || 'mock') }} Endpoint
          </h3>
          <p class="text-sm text-gray-400 mb-3">
            <template v-if="serverStore.currentEndpoint.type === 'proxy'">
              All requests to this prefix are forwarded to the backend server with optional header manipulation,
              status translation, and body transformation.
            </template>
            <template v-else-if="serverStore.currentEndpoint.type === 'container'">
              All requests to this prefix are forwarded to the Docker container.
              The container is started when the mock server starts and stopped when it stops.
            </template>
          </p>

          <!-- Container Control Buttons (only for container endpoints) -->
          <template v-if="serverStore.currentEndpoint.type === 'container'">
            <div class="flex gap-2 mb-2">
              <button
                v-if="canStartContainer(serverStore.currentEndpoint.id)"
                @click="handleStartContainer(serverStore.currentEndpoint.id)"
                :disabled="!!containerActionLoading[serverStore.currentEndpoint.id]"
                class="px-3 py-1.5 bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded text-sm font-medium transition-colors"
              >
                {{ containerActionLoading[serverStore.currentEndpoint.id] === 'start' ? 'Starting...' : 'Start' }}
              </button>
              <button
                v-if="canStopContainer(serverStore.currentEndpoint.id)"
                @click="handleStopContainer(serverStore.currentEndpoint.id)"
                :disabled="!!containerActionLoading[serverStore.currentEndpoint.id]"
                class="px-3 py-1.5 bg-orange-600 hover:bg-orange-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded text-sm font-medium transition-colors"
              >
                {{ containerActionLoading[serverStore.currentEndpoint.id] === 'stop' ? 'Stopping...' : 'Stop' }}
              </button>
              <button
                v-if="canStopContainer(serverStore.currentEndpoint.id)"
                @click="handleDeleteContainer(serverStore.currentEndpoint.id)"
                :disabled="!!containerActionLoading[serverStore.currentEndpoint.id]"
                class="px-3 py-1.5 bg-red-600 hover:bg-red-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded text-sm font-medium transition-colors"
              >
                {{ containerActionLoading[serverStore.currentEndpoint.id] === 'delete' ? 'Deleting...' : 'Delete' }}
              </button>
              <button
                v-if="canStopContainer(serverStore.currentEndpoint.id)"
                @click="handleRestartContainer(serverStore.currentEndpoint.id)"
                :disabled="!!containerActionLoading[serverStore.currentEndpoint.id] || !canStopContainer(serverStore.currentEndpoint.id)"
                class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded text-sm font-medium transition-colors"
              >
                {{ containerActionLoading[serverStore.currentEndpoint.id] === 'restart' ? 'Restarting...' : 'Restart' }}
              </button>
              <button
                v-if="canStopContainer(serverStore.currentEndpoint.id)"
                @click="handleShowConsole(serverStore.currentEndpoint.id, serverStore.currentEndpoint.name)"
                class="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-white rounded text-sm font-medium transition-colors flex items-center gap-1"
                title="View container console output"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
                Console
              </button>
            </div>

            <!-- Container Progress Indicator -->
            <div
              v-if="containerProgress[serverStore.currentEndpoint.id]"
              class="mt-3 p-3 bg-blue-900/20 border border-blue-700 rounded"
            >
              <div class="flex items-center justify-between mb-2">
                <div class="flex items-center gap-2">
                  <div class="w-2 h-2 bg-blue-400 rounded-full animate-pulse"></div>
                  <span class="text-sm font-medium text-blue-300">
                    {{ containerProgress[serverStore.currentEndpoint.id].stage.charAt(0).toUpperCase() + containerProgress[serverStore.currentEndpoint.id].stage.slice(1) }}
                  </span>
                </div>
                <span class="text-xs text-blue-400">
                  {{ containerProgress[serverStore.currentEndpoint.id].progress }}%
                </span>
              </div>
              <p class="text-xs text-blue-200 mb-2">
                {{ containerProgress[serverStore.currentEndpoint.id].message }}
              </p>
              <!-- Progress bar -->
              <div class="w-full bg-gray-700 rounded-full h-1.5">
                <div
                  class="bg-blue-500 h-1.5 rounded-full transition-all duration-300"
                  :style="{ width: `${containerProgress[serverStore.currentEndpoint.id].progress}%` }"
                ></div>
              </div>
            </div>

            <!-- Error message -->
            <div
              v-if="containerActionError[serverStore.currentEndpoint.id]"
              class="p-2 bg-red-900/30 border border-red-700 rounded text-red-400 text-sm"
            >
              {{ containerActionError[serverStore.currentEndpoint.id] }}
            </div>
          </template>
        </div>

        <!-- Health Status (if health checks enabled) -->
        <div
          v-if="needsHealthIndicator(serverStore.currentEndpoint)"
          class="p-4 bg-gray-800 rounded border border-gray-700"
        >
          <h4 class="text-md font-semibold text-white mb-3">Health Status</h4>
          <div v-if="serverStore.getEndpointHealth(serverStore.currentEndpoint.id)">
            <div class="flex items-center gap-2 mb-2">
              <span
                :class="[
                  'text-2xl',
                  healthIndicatorClass(serverStore.currentEndpoint.id)
                ]"
              >
                ●
              </span>
              <span
                :class="[
                  'text-lg font-medium',
                  serverStore.getEndpointHealth(serverStore.currentEndpoint.id)?.healthy
                    ? 'text-green-400'
                    : 'text-red-400'
                ]"
              >
                {{ serverStore.getEndpointHealth(serverStore.currentEndpoint.id)?.healthy ? 'Healthy' : 'Unhealthy' }}
              </span>
            </div>
            <p class="text-xs text-gray-400">
              Last check: {{ new Date(serverStore.getEndpointHealth(serverStore.currentEndpoint.id)?.last_check || '').toLocaleString() }}
            </p>
            <p v-if="serverStore.getEndpointHealth(serverStore.currentEndpoint.id)?.error_message" class="text-xs text-red-400 mt-2">
              {{ serverStore.getEndpointHealth(serverStore.currentEndpoint.id)?.error_message }}
            </p>
          </div>
          <div v-else class="text-sm text-gray-400">
            Waiting for health check data...
          </div>
        </div>

        <!-- Configuration Summary -->
        <div class="p-4 bg-gray-800 rounded border border-gray-700">
          <h4 class="text-md font-semibold text-white mb-3">Configuration</h4>
          <div class="space-y-2 text-sm">
            <div class="flex justify-between">
              <span class="text-gray-400">Path Prefix:</span>
              <span class="text-white font-mono">{{ serverStore.currentEndpoint.path_prefix }}</span>
            </div>
            <template v-if="serverStore.currentEndpoint.type === 'proxy' && serverStore.currentEndpoint.proxy_config">
              <div class="flex justify-between">
                <span class="text-gray-400">Backend URL:</span>
                <span class="text-white font-mono">{{ serverStore.currentEndpoint.proxy_config.backend_url }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-400">Timeout:</span>
                <span class="text-white">{{ serverStore.currentEndpoint.proxy_config.timeout_seconds || 30 }}s</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-400">Status Translation:</span>
                <span class="text-white">{{ serverStore.currentEndpoint.proxy_config.status_passthrough ? 'Pass-through' : 'Enabled' }}</span>
              </div>
            </template>
            <template v-if="serverStore.currentEndpoint.type === 'container' && serverStore.currentEndpoint.container_config">
              <div class="flex justify-between">
                <span class="text-gray-400">Image:</span>
                <span class="text-white font-mono">{{ serverStore.currentEndpoint.container_config.image_name }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-400">Container Port:</span>
                <span class="text-white">{{ serverStore.currentEndpoint.container_config.container_port }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-400">Pull on Startup:</span>
                <span class="text-white">{{ serverStore.currentEndpoint.container_config.pull_on_startup ? 'Yes' : 'No' }}</span>
              </div>
            </template>
          </div>
        </div>

        <!-- Container Metrics (only show for container endpoints) -->
        <div
          v-if="serverStore.currentEndpoint.type === 'container'"
          class="p-4 bg-gray-800 rounded border border-gray-700"
        >
          <div class="flex items-center justify-between mb-3">
            <h4 class="text-md font-semibold text-white">Container Metrics</h4>
            <div class="flex items-center gap-2">
              <span
                v-if="getContainerStatus(serverStore.currentEndpoint.id)"
                :class="[
                  'px-3 py-1 text-sm rounded font-medium border',
                  containerStatusClass(serverStore.currentEndpoint.id)
                ]"
              >
                {{ containerStatusText(serverStore.currentEndpoint.id) }}
              </span>
            </div>
          </div>

          <div v-if="getContainerStats(serverStore.currentEndpoint.id)" class="space-y-3">
            <!-- CPU Usage -->
            <div>
              <div class="flex justify-between text-sm mb-1">
                <span class="text-gray-400">CPU Usage:</span>
                <span class="text-white font-mono">{{ formatCPU(getContainerStats(serverStore.currentEndpoint.id)!.cpu_percent) }}</span>
              </div>
              <div class="w-full bg-gray-700 rounded-full h-2">
                <div
                  class="bg-blue-600 h-2 rounded-full transition-all duration-300"
                  :style="{ width: `${Math.min(getContainerStats(serverStore.currentEndpoint.id)!.cpu_percent, 100)}%` }"
                ></div>
              </div>
            </div>

            <!-- Memory Usage -->
            <div>
              <div class="flex justify-between text-sm mb-1">
                <span class="text-gray-400">Memory Usage:</span>
                <span class="text-white font-mono">
                  {{ formatMemory(getContainerStats(serverStore.currentEndpoint.id)!.memory_usage_mb) }} /
                  {{ formatMemory(getContainerStats(serverStore.currentEndpoint.id)!.memory_limit_mb) }}
                  ({{ formatPercent(getContainerStats(serverStore.currentEndpoint.id)!.memory_percent) }})
                </span>
              </div>
              <div class="w-full bg-gray-700 rounded-full h-2">
                <div
                  class="bg-green-600 h-2 rounded-full transition-all duration-300"
                  :style="{ width: `${Math.min(getContainerStats(serverStore.currentEndpoint.id)!.memory_percent, 100)}%` }"
                ></div>
              </div>
            </div>

            <!-- Network I/O -->
            <div class="grid grid-cols-2 gap-3">
              <div>
                <span class="text-gray-400 text-sm">Network RX:</span>
                <div class="text-white font-mono text-sm">{{ formatBytes(getContainerStats(serverStore.currentEndpoint.id)!.network_rx_bytes) }}</div>
              </div>
              <div>
                <span class="text-gray-400 text-sm">Network TX:</span>
                <div class="text-white font-mono text-sm">{{ formatBytes(getContainerStats(serverStore.currentEndpoint.id)!.network_tx_bytes) }}</div>
              </div>
            </div>

            <!-- Block I/O -->
            <div class="grid grid-cols-2 gap-3">
              <div>
                <span class="text-gray-400 text-sm">Block Read:</span>
                <div class="text-white font-mono text-sm">{{ formatBytes(getContainerStats(serverStore.currentEndpoint.id)!.block_read_bytes) }}</div>
              </div>
              <div>
                <span class="text-gray-400 text-sm">Block Write:</span>
                <div class="text-white font-mono text-sm">{{ formatBytes(getContainerStats(serverStore.currentEndpoint.id)!.block_write_bytes) }}</div>
              </div>
            </div>

            <!-- PIDs -->
            <div class="flex justify-between text-sm">
              <span class="text-gray-400">Processes:</span>
              <span class="text-white font-mono">{{ getContainerStats(serverStore.currentEndpoint.id)!.pids }}</span>
            </div>

            <!-- Last Updated -->
            <div class="text-xs text-gray-500 text-right">
              Updated: {{ new Date(getContainerStats(serverStore.currentEndpoint.id)!.last_check).toLocaleTimeString() }}
            </div>
          </div>

          <div v-else class="text-sm text-gray-400">
            No metrics available
          </div>
        </div>

        <!-- Action Hint -->
        <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
          <p class="text-sm text-blue-300">
            Use the Settings button above to configure {{ serverStore.currentEndpoint.type === 'proxy' ? 'proxy' : 'container' }} options.
          </p>
        </div>
      </div>
        </div>
      </div>

      <!-- Resizable Divider -->
      <div
        @mousedown="startDragging"
        class="w-1 bg-gray-700 hover:bg-blue-500 cursor-col-resize flex-shrink-0 transition-colors"
        :class="{ 'bg-blue-500': isDraggingDivider }"
      ></div>

      <!-- Right Section: Traffic Log -->
      <div :style="{ width: `calc(100% - ${dividerPosition}px)` }" class="overflow-hidden flex flex-col min-h-0">
        <TrafficLogPanel />
      </div>
    </div>
    </template>
    <!-- End Endpoint Content -->

    <!-- Dialogs -->
    <AddEndpointDialog
      :show="showAddEndpointDialog"
      @confirm="handleAddEndpoint"
      @cancel="handleCancelAddEndpoint"
    />
    <EndpointSettingsDialog
      :show="showEndpointSettingsDialog"
      :endpoint="serverStore.currentEndpoint"
      @save="handleSaveEndpointSettings"
      @delete="handleDeleteEndpoint"
      @cancel="handleCancelEndpointSettings"
    />
    <ConfirmDialog
      :show="showDeleteConfirmDialog"
      title="Delete Endpoint"
      :message="`Are you sure you want to delete endpoint &quot;${endpointToDelete}&quot;?\n\nAll response rules in this endpoint will be deleted.`"
      primary-text="Delete"
      cancel-text="Cancel"
      @primary="confirmDeleteEndpoint"
      @cancel="cancelDeleteEndpoint"
    />
    <ContainerConsoleDialog
      :show="showContainerConsoleDialog"
      :endpoint-id="consoleEndpointId"
      :endpoint-name="consoleEndpointName"
      @close="handleCloseConsole"
    />
  </div>
</template>
