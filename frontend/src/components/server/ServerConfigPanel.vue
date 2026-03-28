<script lang="ts" setup>
import { ref, inject, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { useServerStore } from '../../stores/server'
import ResponseRuleCard from './ResponseRuleCard.vue'
import ResponseGroupCard from './ResponseGroupCard.vue'
import AddEndpointDialog from '../dialogs/AddEndpointDialog.vue'
import EndpointSettingsDialog from '../dialogs/EndpointSettingsDialog.vue'
import ConfirmDialog from '../dialogs/ConfirmDialog.vue'
import ContainerConsoleDialog from '../dialogs/ContainerConsoleDialog.vue'
import TrafficLogPanel from '../traffic/TrafficLogPanel.vue'
import ServerTab from './tabs/ServerTab.vue'
import SOCKS5DomainsPanel from '../socks5/SOCKS5DomainsPanel.vue'
import { models } from '../../types/models'
import { StartContainer, StopContainer, DeleteContainer } from '../../../wailsjs/go/main/App'
import OverlaySimPanel from './OverlaySimPanel.vue'
import ProxySimPanel from './ProxySimPanel.vue'
import EndpointNavigator from './EndpointNavigator.vue'
import ProxyConfigPanel from '../dialogs/ProxyConfigPanel.vue'
import ContainerConfigPanel from '../dialogs/ContainerConfigPanel.vue'
import CustomSelect from '../common/CustomSelect.vue'
import DomainFilterInput from '../common/DomainFilterInput.vue'

const serverStore = useServerStore()

// Sorted endpoints by DisplayOrder (matches backend processing priority)
const sortedEndpoints = computed(() => {
  return [...serverStore.endpoints].sort((a, b) =>
    (a.display_order ?? 0) - (b.display_order ?? 0)
  )
})

// Track selected tab (server vs endpoint)
const selectedTab = ref<'server' | string>('server')  // Default to Server tab

// Reset to Server tab when a new config file is loaded
watch(() => serverStore.currentFilePath, () => {
  selectedTab.value = 'server'
})

// Close drawer when switching to Server tab
watch(selectedTab, (tab) => {
  if (tab === 'server') showSettingsDrawer.value = false
})

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

// Settings drawer state
const showSettingsDrawer = ref(false)

// Dialog state
const showAddEndpointDialog = ref(false)
const showEndpointSettingsDialog = ref(false)
const showDeleteConfirmDialog = ref(false)
const showContainerConsoleDialog = ref(false)
const showImportDialog = ref(false)
const importError = ref<string>('')
const endpointToDelete = ref<string>('')
const consoleEndpointId = ref<string>('')
const consoleEndpointName = ref<string>('')

let resizeObserver: ResizeObserver | null = null

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

// Tab drag and drop state (kept for EndpointNavigator compat)
const tabDraggedIndex = ref<number | null>(null)
const tabDragOverIndex = ref<number | null>(null)

function onTabDragStart(index: number, endpoint: models.Endpoint, event: DragEvent) {
  if (endpoint.is_system) {
    event.preventDefault()
    return
  }
  tabDraggedIndex.value = index
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(index))
  }
}

function onTabDragOver(index: number, endpoint: models.Endpoint, event: DragEvent) {
  if (endpoint.is_system) return
  event.preventDefault()
  tabDragOverIndex.value = index
}

function onTabDrop(index: number, endpoint: models.Endpoint, event: DragEvent) {
  event.preventDefault()
  if (endpoint.is_system || tabDraggedIndex.value === null || tabDraggedIndex.value === index) {
    tabDraggedIndex.value = null
    tabDragOverIndex.value = null
    return
  }

  // Convert sorted visual indices to non-system indices for the store
  const nonSystem = sortedEndpoints.value.filter(ep => !ep.is_system)
  const fromEp = sortedEndpoints.value[tabDraggedIndex.value]
  const toEp = sortedEndpoints.value[index]

  const fromNsIndex = nonSystem.findIndex(ep => ep.id === fromEp?.id)
  const toNsIndex = nonSystem.findIndex(ep => ep.id === toEp?.id)

  if (fromNsIndex >= 0 && toNsIndex >= 0) {
    serverStore.reorderEndpointTabs(fromNsIndex, toNsIndex)
  }

  tabDraggedIndex.value = null
  tabDragOverIndex.value = null
}

function onTabDragEnd() {
  tabDraggedIndex.value = null
  tabDragOverIndex.value = null
}

// Navigator event handlers
function onNavSelect(id: string) {
  if (id === 'server') {
    selectedTab.value = 'server'
  } else {
    selectEndpoint(id)
  }
}

function onNavSettings(endpoint: models.Endpoint) {
  selectEndpoint(endpoint.id)
  showEndpointSettingsDialog.value = true
}

function onNavDelete(endpoint: models.Endpoint) {
  selectEndpoint(endpoint.id)
  endpointToDelete.value = endpoint.name ?? ''
  showDeleteConfirmDialog.value = true
}

function selectEndpoint(id: string) {
  console.log('[ServerConfigPanel.selectEndpoint] Tab clicked, id:', id)
  console.log('[ServerConfigPanel.selectEndpoint] Current items before switch:', serverStore.items?.length || 0)
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

// ── Inline endpoint editing ──────────────────────────────────────────────
const inlineActiveTab = ref<'general' | 'proxy' | 'container'>('general')

// Dropdown options for inline editor
const translationModeOptions = [
  { value: 'none', label: 'None - Use path as-is' },
  { value: 'strip', label: 'Strip - Remove prefix before matching' },
  { value: 'translate', label: 'Translate - Regex match/replace' }
]

const domainFilterModeOptions = [
  { value: 'any', label: 'Any Domain', description: 'Match requests from any domain (including direct HTTP)' },
  { value: 'all', label: 'All SOCKS5 Domains', description: 'Only match requests from domains in SOCKS5 Domain Takeover' },
  { value: 'specific', label: 'Specific Domains', description: 'Only match requests from specific domain patterns' }
]

const inlineName = ref('')
const inlinePathPrefix = ref('/')
const inlineTranslationMode = ref('none')
const inlineTranslatePattern = ref('')
const inlineTranslateReplace = ref('')
const inlineEnabled = ref(true)
const inlineDomainFilterMode = ref('any')
const inlineDomainFilterPatterns = ref<string[]>([])
const inlineProxyConfig = ref<models.ProxyConfig | null>(null)
const inlineContainerConfig = ref<models.ContainerConfig | null>(null)

// Load inline editor state when the selected endpoint ID changes (not on every reactive tick)
let lastLoadedEndpointId = ''
watch(() => serverStore.currentEndpoint?.id, (newId) => {
  const ep = serverStore.currentEndpoint
  if (!ep || !newId) return
  if (newId === lastLoadedEndpointId) return  // Same endpoint, skip reload
  lastLoadedEndpointId = newId
  inlineName.value = ep.name || ''
  inlinePathPrefix.value = ep.path_prefix || '/'
  inlineTranslationMode.value = ep.translation_mode || 'none'
  inlineTranslatePattern.value = ep.translate_pattern || ''
  inlineTranslateReplace.value = ep.translate_replace || ''
  inlineEnabled.value = ep.enabled !== false
  inlineDomainFilterMode.value = ep.domain_filter?.mode || 'any'
  inlineDomainFilterPatterns.value = ep.domain_filter?.patterns || []
  inlineProxyConfig.value = ep.proxy_config || null
  inlineContainerConfig.value = ep.container_config || null
  inlineActiveTab.value = 'general'
}, { immediate: true })

let inlineSaveTimer: ReturnType<typeof setTimeout> | null = null
function debouncedInlineSave() {
  if (inlineSaveTimer) clearTimeout(inlineSaveTimer)
  inlineSaveTimer = setTimeout(saveInlineEndpoint, 500)
}

async function saveInlineEndpoint() {
  const ep = serverStore.currentEndpoint
  if (!ep || !inlineName.value.trim() || !inlinePathPrefix.value.trim()) return

  const domainFilter = inlineDomainFilterMode.value !== 'any' ? new models.DomainFilter({
    mode: inlineDomainFilterMode.value,
    patterns: inlineDomainFilterMode.value === 'specific' ? inlineDomainFilterPatterns.value : []
  }) : undefined

  const updated = new models.Endpoint({
    id: ep.id,
    name: inlineName.value.trim(),
    path_prefix: inlinePathPrefix.value.trim(),
    translation_mode: inlineTranslationMode.value,
    translate_pattern: inlineTranslationMode.value === 'translate' ? inlineTranslatePattern.value.trim() : '',
    translate_replace: inlineTranslationMode.value === 'translate' ? inlineTranslateReplace.value.trim() : '',
    enabled: inlineEnabled.value,
    type: ep.type,
    items: ep.items,
    proxy_config: inlineProxyConfig.value || undefined,
    container_config: inlineContainerConfig.value || undefined,
    domain_filter: domainFilter
  })

  try {
    await serverStore.updateEndpointById(updated)
  } catch (error) {
    console.error('Failed to save inline endpoint:', error)
  }
}

function handleInlineProxyConfigUpdate(config: models.ProxyConfig) {
  const ep = serverStore.currentEndpoint
  if (!ep) return

  if (ep.type === 'proxy') {
    inlineProxyConfig.value = config
  } else if (ep.type === 'container' && inlineContainerConfig.value) {
    inlineContainerConfig.value = new models.ContainerConfig({
      ...inlineContainerConfig.value,
      proxy_config: config
    })
  }
  debouncedInlineSave()
}

function handleInlineContainerConfigUpdate(config: models.ContainerConfig) {
  inlineContainerConfig.value = config
  debouncedInlineSave()
}

// Overlay endpoint helpers
function isOverlayEndpoint(endpoint: models.Endpoint): boolean {
  return endpoint.id?.startsWith('system-overlay-') || false
}

function getOverlayDisplayName(endpoint: models.Endpoint): string {
  // Remove "Overlay: " prefix if present
  if (endpoint.name?.startsWith('Overlay: ')) {
    return endpoint.name.substring(9)
  }
  return endpoint.name || ''
}

// System endpoint helpers - show server info for SOCKS5 and Rejections
function showsServerInfo(endpoint: models.Endpoint): boolean {
  return endpoint.id === 'system-socks5-proxy' || endpoint.id === 'system-rejections'
}

function getServerURL(): string {
  const config = serverStore.config
  const status = serverStore.status
  if (!config) {
    return `http://localhost:${status.port}`
  }
  const protocol = config.https_enabled ? 'https://' : 'http://'
  const port = config.https_enabled ? config.https_port : config.port
  return `${protocol}localhost:${port}`
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

// Drawer keyboard handler (Escape to close)
function onDrawerKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && showSettingsDrawer.value) {
    showSettingsDrawer.value = false
  }
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

// OpenAPI Import handlers
function handleImportOpenAPI() {
  showImportDialog.value = true
}

async function handleAppendImport() {
  showImportDialog.value = false
  try {
    await serverStore.importOpenAPISpec(true) // append mode
  } catch (error) {
    importError.value = String(error)
  }
}

async function handleReplaceImport() {
  showImportDialog.value = false
  try {
    await serverStore.importOpenAPISpec(false) // replace mode
  } catch (error) {
    importError.value = String(error)
  }
}

function handleCancelImport() {
  showImportDialog.value = false
}

// Register for container progress events and drawer keydown
onMounted(() => {
  document.addEventListener('keydown', onDrawerKeydown)
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

// Cleanup on unmount
onUnmounted(() => {
  document.removeEventListener('keydown', onDrawerKeydown)
  if (unregisterProgressListener) {
    unregisterProgressListener()
    unregisterProgressListener = null
  }
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
})
</script>

<template>
  <div class="h-full flex flex-row overflow-hidden">
    <!-- Vertical Endpoint Navigator -->
    <EndpointNavigator
      :endpoints="sortedEndpoints"
      :selected-id="selectedTab"
      @select="onNavSelect"
      @add-endpoint="showAddEndpointDialog = true"
      @settings="onNavSettings"
      @delete-ep="onNavDelete"
    />

    <!-- Main content column (endpoint controls + panels) -->
    <div class="flex-1 flex flex-col min-w-0 overflow-hidden">


    <!-- Endpoint Info Banner with SETTINGS button -->
    <div v-if="serverStore.currentEndpoint" class="px-3 py-2 bg-gray-800/50 border-b border-gray-700 flex-shrink-0">
      <div class="flex items-center justify-between">
        <div class="flex-1">
          <p class="text-xs text-gray-400">
            <span class="font-medium text-gray-300">Type:</span> {{ isOverlayEndpoint(serverStore.currentEndpoint) ? 'Overlay' : typeDisplayName(serverStore.currentEndpoint.type || 'mock') }}
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
              <span class="font-medium text-gray-300">Backend:</span>
              {{ (serverStore.currentEndpoint.proxy_config.backend_url ?? '').replace(/^\/\//, '') }}
            </template>
            <!-- Container-specific info -->
            <template v-if="serverStore.currentEndpoint.type === 'container' && serverStore.currentEndpoint.container_config">
              <span class="mx-2">•</span>
              <span class="font-medium text-gray-300">Image:</span> {{ serverStore.currentEndpoint.container_config.image_name }}
            </template>
          </p>
        </div>
        <!-- SETTINGS button -->
        <button
          v-if="!serverStore.currentEndpoint.is_system || isOverlayEndpoint(serverStore.currentEndpoint)"
          @click="showSettingsDrawer = !showSettingsDrawer"
          :class="[
            'ml-3 px-3 py-1.5 rounded border text-xs font-medium transition-colors flex items-center gap-1.5 flex-shrink-0',
            showSettingsDrawer
              ? 'bg-blue-600 border-blue-500 text-white'
              : 'bg-gray-700 border-gray-600 text-gray-300 hover:bg-gray-600 hover:text-white'
          ]"
        >
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          SETTINGS
        </button>
      </div>
    </div>

    <!-- Server Tab Content (NEW) -->
    <ServerTab v-if="selectedTab === 'server'" class="flex-1" />

    <!-- Endpoint Content -->
    <template v-else>
      <!-- Traffic Log (full width) -->
      <div class="flex-1 overflow-hidden flex flex-col min-h-0 relative">
        <TrafficLogPanel />

        <!-- Settings Drawer (absolute overlay, right side — independent of traffic log reactivity) -->
        <Transition name="drawer">
          <div
            v-if="showSettingsDrawer && serverStore.currentEndpoint"
            :key="serverStore.currentEndpoint.id"
            class="absolute top-0 right-0 bottom-0 border-l border-gray-700 bg-gray-800 flex flex-col overflow-hidden z-30 shadow-2xl"
            :style="{ width: 'clamp(350px, 40vw, 550px)' }"
          >
            <!-- Drawer header -->
            <div class="flex items-center justify-between px-4 py-2 border-b border-gray-700 flex-shrink-0">
              <h3 class="text-sm font-semibold text-white">Endpoint Settings</h3>
              <button
                @click="showSettingsDrawer = false"
                class="p-1 text-gray-400 hover:text-white rounded hover:bg-gray-700 transition-colors"
                title="Close settings"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <!-- Drawer body -->
            <div class="flex-1 overflow-y-auto">

              <!-- ── MOCK ENDPOINT DRAWER CONTENT ── -->
              <template v-if="serverStore.currentEndpoint.type === 'mock'">
                <!-- Mock controls (+ Group / + Response / + OpenAPI) -->
                <div v-if="!serverStore.currentEndpoint.is_system" class="flex items-center gap-2 p-3 border-b border-gray-700 flex-shrink-0">
                  <button @click="serverStore.addNewGroup" class="px-2 py-1 bg-blue-800 hover:bg-blue-700 rounded text-xs text-white font-medium flex items-center gap-1" title="Add group">
                    <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" /></svg>
                    + Group
                  </button>
                  <button @click="serverStore.addNewResponse" class="px-2 py-1 bg-blue-600 hover:bg-blue-700 rounded text-xs text-white font-medium flex items-center gap-1">
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
                    + Response
                  </button>
                  <button @click="handleImportOpenAPI" class="px-2 py-1 bg-green-700 hover:bg-green-600 rounded text-xs text-white font-medium flex items-center gap-1" title="Import OpenAPI">
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" /></svg>
                    + OpenAPI
                  </button>
                </div>

                <!-- SOCKS5 Proxy domains -->
                <SOCKS5DomainsPanel v-if="serverStore.currentEndpoint.id === 'system-socks5-proxy'" />

                <!-- Mock rules list -->
                <div v-else class="p-3 space-y-2" @dragend="onDragEnd">
                  <div v-if="!serverStore.items || serverStore.items.length === 0" class="flex items-center justify-center h-32">
                    <div class="text-center text-gray-500">
                      <p class="text-sm">No response rules configured</p>
                      <p class="text-xs mt-1">Click "+ Response" or "+ Group" to get started</p>
                    </div>
                  </div>
                  <div
                    v-for="(item, index) in serverStore.items"
                    :key="getItemId(item)"
                    :class="['transition-all', dragOverIndex === index && draggedIndex !== index ? 'border-t-2 border-blue-500 pt-2' : '']"
                  >
                    <ResponseRuleCard
                      v-if="item.type === 'response' && item.response"
                      :response="item.response"
                      :is-expanded="serverStore.expandedItemId === item.response.id"
                      :is-highlighted="serverStore.highlightedResponseId === item.response.id"
                      :index="index"
                      @toggle="serverStore.toggleExpanded(item.response?.id || '')"
                      @update="handleResponseUpdate(index, $event)"
                      @delete="handleDelete(index)"
                      @dragstart="onDragStart(index, $event)"
                      @dragover="onDragOver(index, $event)"
                      @drop="onDrop(index, $event)"
                    />
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

                <!-- Inline editor for mock endpoints (below rules) -->
                <div v-if="!serverStore.currentEndpoint.is_system" class="p-4 border-t border-gray-700">
                  <div class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">Endpoint Configuration</div>
                  <!-- Inline General Settings -->
                  <div class="space-y-3">
                    <div class="flex items-center justify-between">
                      <label class="text-xs font-medium text-gray-300">Enabled</label>
                      <label class="relative inline-flex items-center cursor-pointer">
                        <input v-model="inlineEnabled" type="checkbox" class="sr-only peer" @change="debouncedInlineSave">
                        <div class="w-9 h-5 bg-gray-700 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-500 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-600"></div>
                      </label>
                    </div>
                    <div>
                      <label class="block text-xs font-medium text-gray-400 mb-1">Path Prefix</label>
                      <input v-model="inlinePathPrefix" type="text" @input="debouncedInlineSave" class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-sm text-white font-mono focus:outline-none focus:border-blue-500" />
                    </div>
                    <div>
                      <label class="block text-xs font-medium text-gray-400 mb-1">Translation</label>
                      <CustomSelect v-model="inlineTranslationMode" :options="translationModeOptions" @update:model-value="debouncedInlineSave" />
                    </div>
                  </div>
                </div>
              </template>

              <!-- ── OVERLAY ENDPOINT DRAWER CONTENT ── -->
              <template v-else-if="isOverlayEndpoint(serverStore.currentEndpoint)">
                <div class="p-4 space-y-4">
                  <OverlaySimPanel :endpoint="serverStore.currentEndpoint" />
                  <div class="p-4 bg-gray-800 rounded border border-gray-700">
                    <h3 class="text-lg font-semibold text-white mb-2">Overlay</h3>
                    <p class="text-sm text-gray-400">
                      Overlay endpoints automatically proxy traffic to the real domain.
                      Use the simulation panel above to inject faults for testing.
                    </p>
                  </div>
                </div>
              </template>

              <!-- ── PROXY / CONTAINER ENDPOINT DRAWER CONTENT ── -->
              <template v-else-if="serverStore.currentEndpoint">
                <div class="p-4 space-y-4">
                  <!-- Proxy Simulation Mode (for non-overlay proxy endpoints) -->
                  <ProxySimPanel
                    v-if="serverStore.currentEndpoint.type === 'proxy'"
                    :endpoint="serverStore.currentEndpoint"
                  />

                  <!-- Container Controls -->
                  <template v-if="serverStore.currentEndpoint.type === 'container'">
                    <div class="p-4 bg-gray-800 rounded border border-gray-700">
                      <h4 class="text-sm font-semibold text-white mb-2">Container Controls</h4>
                      <div class="flex flex-wrap gap-2">
                        <button v-if="canStartContainer(serverStore.currentEndpoint.id)" @click="handleStartContainer(serverStore.currentEndpoint.id)" :disabled="!!containerActionLoading[serverStore.currentEndpoint.id]" class="px-2 py-1 bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded text-xs font-medium transition-colors">
                          {{ containerActionLoading[serverStore.currentEndpoint.id] === 'start' ? 'Starting...' : 'Start' }}
                        </button>
                        <button v-if="canStopContainer(serverStore.currentEndpoint.id)" @click="handleStopContainer(serverStore.currentEndpoint.id)" :disabled="!!containerActionLoading[serverStore.currentEndpoint.id]" class="px-2 py-1 bg-orange-600 hover:bg-orange-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded text-xs font-medium transition-colors">
                          {{ containerActionLoading[serverStore.currentEndpoint.id] === 'stop' ? 'Stopping...' : 'Stop' }}
                        </button>
                        <button v-if="canStopContainer(serverStore.currentEndpoint.id)" @click="handleRestartContainer(serverStore.currentEndpoint.id)" :disabled="!!containerActionLoading[serverStore.currentEndpoint.id]" class="px-2 py-1 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded text-xs font-medium transition-colors">
                          {{ containerActionLoading[serverStore.currentEndpoint.id] === 'restart' ? 'Restarting...' : 'Restart' }}
                        </button>
                        <button v-if="canStopContainer(serverStore.currentEndpoint.id)" @click="handleShowConsole(serverStore.currentEndpoint.id, serverStore.currentEndpoint.name)" class="px-2 py-1 bg-gray-700 hover:bg-gray-600 text-white rounded text-xs font-medium transition-colors flex items-center gap-1">
                          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>
                          Console
                        </button>
                      </div>
                      <!-- Progress -->
                      <div v-if="containerProgress[serverStore.currentEndpoint.id]" class="mt-3 p-2 bg-blue-900/20 border border-blue-700 rounded">
                        <div class="flex items-center justify-between mb-1">
                          <span class="text-xs font-medium text-blue-300 flex items-center gap-1">
                            <span class="w-1.5 h-1.5 bg-blue-400 rounded-full animate-pulse"></span>
                            {{ containerProgress[serverStore.currentEndpoint.id].stage }}
                          </span>
                          <span class="text-xs text-blue-400">{{ containerProgress[serverStore.currentEndpoint.id].progress }}%</span>
                        </div>
                        <div class="w-full bg-gray-700 rounded-full h-1">
                          <div class="bg-blue-500 h-1 rounded-full transition-all duration-300" :style="{ width: `${containerProgress[serverStore.currentEndpoint.id].progress}%` }"></div>
                        </div>
                      </div>
                      <div v-if="containerActionError[serverStore.currentEndpoint.id]" class="mt-2 p-2 bg-red-900/30 border border-red-700 rounded text-red-400 text-xs">
                        {{ containerActionError[serverStore.currentEndpoint.id] }}
                      </div>
                    </div>
                  </template>

                  <!-- Health Status -->
                  <div v-if="needsHealthIndicator(serverStore.currentEndpoint)" class="p-4 bg-gray-800 rounded border border-gray-700">
                    <h4 class="text-sm font-semibold text-white mb-2">Health Status</h4>
                    <div v-if="serverStore.getEndpointHealth(serverStore.currentEndpoint.id)">
                      <div class="flex items-center gap-2 mb-1">
                        <span :class="['text-lg', healthIndicatorClass(serverStore.currentEndpoint.id)]">●</span>
                        <span :class="['text-sm font-medium', serverStore.getEndpointHealth(serverStore.currentEndpoint.id)?.healthy ? 'text-green-400' : 'text-red-400']">
                          {{ serverStore.getEndpointHealth(serverStore.currentEndpoint.id)?.healthy ? 'Healthy' : 'Unhealthy' }}
                        </span>
                      </div>
                      <p class="text-xs text-gray-400">Last: {{ new Date(serverStore.getEndpointHealth(serverStore.currentEndpoint.id)?.last_check || '').toLocaleTimeString() }}</p>
                    </div>
                    <div v-else class="text-xs text-gray-400">Waiting for health check...</div>
                  </div>

                  <!-- Inline Endpoint Editor -->
                  <div class="p-4 bg-gray-800 rounded border border-gray-700">
                    <div class="flex border-b border-gray-700 mb-4 -mt-1">
                      <button @click="inlineActiveTab = 'general'" :class="['px-3 py-1.5 text-xs font-medium transition-colors', inlineActiveTab === 'general' ? 'text-blue-400 border-b-2 border-blue-400' : 'text-gray-400 hover:text-gray-300']">General</button>
                      <button v-if="serverStore.currentEndpoint.type === 'proxy' || serverStore.currentEndpoint.type === 'container'" @click="inlineActiveTab = 'proxy'" :class="['px-3 py-1.5 text-xs font-medium transition-colors', inlineActiveTab === 'proxy' ? 'text-blue-400 border-b-2 border-blue-400' : 'text-gray-400 hover:text-gray-300']">Proxy Settings</button>
                      <button v-if="serverStore.currentEndpoint.type === 'container'" @click="inlineActiveTab = 'container'" :class="['px-3 py-1.5 text-xs font-medium transition-colors', inlineActiveTab === 'container' ? 'text-blue-400 border-b-2 border-blue-400' : 'text-gray-400 hover:text-gray-300']">Container</button>
                    </div>

                    <div v-if="inlineActiveTab === 'general'" class="space-y-3">
                      <div class="flex items-center justify-between">
                        <label class="text-xs font-medium text-gray-300">Enabled</label>
                        <label class="relative inline-flex items-center cursor-pointer">
                          <input v-model="inlineEnabled" type="checkbox" class="sr-only peer" @change="debouncedInlineSave">
                          <div class="w-9 h-5 bg-gray-700 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-500 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-600"></div>
                        </label>
                      </div>
                      <div>
                        <label class="block text-xs font-medium text-gray-400 mb-1">Name</label>
                        <input v-model="inlineName" type="text" @input="debouncedInlineSave" class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-sm text-white focus:outline-none focus:border-blue-500" />
                      </div>
                      <div>
                        <label class="block text-xs font-medium text-gray-400 mb-1">Domain Filter</label>
                        <CustomSelect v-model="inlineDomainFilterMode" :options="domainFilterModeOptions" @update:model-value="debouncedInlineSave" />
                      </div>
                      <div v-if="inlineDomainFilterMode === 'specific'">
                        <label class="block text-xs font-medium text-gray-400 mb-1">Domain Patterns</label>
                        <DomainFilterInput v-model="inlineDomainFilterPatterns" @update:model-value="debouncedInlineSave" />
                      </div>
                      <div>
                        <label class="block text-xs font-medium text-gray-400 mb-1">Path Prefix</label>
                        <input v-model="inlinePathPrefix" type="text" @input="debouncedInlineSave" class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-sm text-white font-mono focus:outline-none focus:border-blue-500" />
                      </div>
                      <div>
                        <label class="block text-xs font-medium text-gray-400 mb-1">Path Translation</label>
                        <CustomSelect v-model="inlineTranslationMode" :options="translationModeOptions" @update:model-value="debouncedInlineSave" />
                      </div>
                      <div v-if="inlineTranslationMode === 'translate'" class="space-y-2">
                        <div>
                          <label class="block text-xs font-medium text-gray-400 mb-1">Match Pattern</label>
                          <input v-model="inlineTranslatePattern" type="text" @input="debouncedInlineSave" class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-sm text-white font-mono focus:outline-none focus:border-blue-500" />
                        </div>
                        <div>
                          <label class="block text-xs font-medium text-gray-400 mb-1">Replace With</label>
                          <input v-model="inlineTranslateReplace" type="text" @input="debouncedInlineSave" class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-sm text-white font-mono focus:outline-none focus:border-blue-500" />
                        </div>
                      </div>
                    </div>

                    <div v-if="inlineActiveTab === 'proxy'" class="space-y-3">
                      <ProxyConfigPanel v-if="serverStore.currentEndpoint.type === 'proxy' && inlineProxyConfig" :config="inlineProxyConfig" :is-container-endpoint="false" @update:config="handleInlineProxyConfigUpdate" />
                      <ProxyConfigPanel v-if="serverStore.currentEndpoint.type === 'container' && inlineContainerConfig?.proxy_config" :config="inlineContainerConfig.proxy_config" :is-container-endpoint="true" @update:config="handleInlineProxyConfigUpdate" />
                    </div>

                    <div v-if="inlineActiveTab === 'container' && serverStore.currentEndpoint.type === 'container' && inlineContainerConfig" class="space-y-3">
                      <ContainerConfigPanel :config="inlineContainerConfig" :endpoint-id="serverStore.currentEndpoint.id" :is-running="serverStore.isRunning" @update:config="handleInlineContainerConfigUpdate" />
                    </div>
                  </div>

                  <!-- Container Metrics -->
                  <div v-if="serverStore.currentEndpoint.type === 'container' && getContainerStats(serverStore.currentEndpoint.id)" class="p-4 bg-gray-800 rounded border border-gray-700">
                    <h4 class="text-sm font-semibold text-white mb-2">Container Metrics</h4>
                    <div class="space-y-2 text-xs">
                      <div class="flex justify-between"><span class="text-gray-400">CPU:</span><span class="text-white font-mono">{{ formatCPU(getContainerStats(serverStore.currentEndpoint.id)!.cpu_percent) }}</span></div>
                      <div class="flex justify-between"><span class="text-gray-400">Memory:</span><span class="text-white font-mono">{{ formatMemory(getContainerStats(serverStore.currentEndpoint.id)!.memory_usage_mb) }} ({{ formatPercent(getContainerStats(serverStore.currentEndpoint.id)!.memory_percent) }})</span></div>
                      <div class="flex justify-between"><span class="text-gray-400">Net RX/TX:</span><span class="text-white font-mono">{{ formatBytes(getContainerStats(serverStore.currentEndpoint.id)!.network_rx_bytes) }} / {{ formatBytes(getContainerStats(serverStore.currentEndpoint.id)!.network_tx_bytes) }}</span></div>
                      <div class="flex justify-between"><span class="text-gray-400">PIDs:</span><span class="text-white font-mono">{{ getContainerStats(serverStore.currentEndpoint.id)!.pids }}</span></div>
                    </div>
                  </div>
                </div>
              </template>

            </div>
          </div>
        </Transition>
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

    <!-- Import OpenAPI Dialog -->
    <ConfirmDialog
      :show="showImportDialog"
      title="Import OpenAPI Specification"
      message="How would you like to import the OpenAPI specification?"
      primary-text="Append"
      secondary-text="Replace"
      cancel-text="Cancel"
      @primary="handleAppendImport"
      @secondary="handleReplaceImport"
      @cancel="handleCancelImport"
    />
    </div><!-- end main content column -->
  </div><!-- end outer flex-row -->
</template>

<style scoped>
.drawer-enter-active { transition: transform 0.25s ease, opacity 0.2s ease; }
.drawer-leave-active { transition: transform 0.2s ease, opacity 0.15s ease; }
.drawer-enter-from, .drawer-leave-to { transform: translateX(100%); opacity: 0; }
</style>
