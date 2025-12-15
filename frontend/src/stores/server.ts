import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { main, models } from '../../wailsjs/go/models'
import {
  StartServer,
  StopServer,
  GetServerStatus,
  GetConfig,
  GetItems,
  SetItems,
  AddGroup,
  GetRequestLogs,
  ClearRequestLogs,
  ImportOpenAPISpecWithDialog,
  GetCACertInfo,
  RegenerateCA,
  DownloadCACert,
  GetCORSConfig,
  SetCORSConfig,
  ValidateCORSScript,
  ValidateCORSHeaderExpression,
  GetEndpoints,
  AddEndpoint,
  AddEndpointWithConfig,
  UpdateEndpoint,
  DeleteEndpoint,
  GetSelectedEndpointId,
  SetSelectedEndpointId,
  GetEndpointHealth,
  TestProxyConnection,
  ValidateDockerImage,
  RestartContainer,
  GetContainerStatus,
  GetRequestLogDetails,
  PollRequestLogs
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

export const useServerStore = defineStore('server', () => {
  // State
  const status = ref<main.ServerStatus>(new main.ServerStatus({ running: false, port: 8080 }))
  const requestLogs = ref<models.RequestLogSummary[]>([])
  const requestLogCache = ref<Map<string, models.RequestLog>>(new Map())
  const selectedLogId = ref<string | null>(null)
  const items = ref<models.ResponseItem[]>([])
  const expandedItemId = ref<string | null>(null)
  const config = ref<models.AppConfig | null>(null)

  // Endpoint State
  const endpoints = ref<models.Endpoint[]>([])
  const selectedEndpointId = ref<string>('')

  // HTTPS & CORS State
  const caInfo = ref<models.CACertInfo | null>(null)
  const corsConfig = ref<models.CORSConfig | null>(null)

  // Health Status State
  const endpointHealth = ref<Map<string, models.HealthStatus>>(new Map())

  // Container Status State
  const containerStatus = ref<Map<string, models.ContainerStatus>>(new Map())

  // Container Stats State
  const containerStats = ref<Map<string, models.ContainerStats>>(new Map())

  // Getters
  const isRunning = computed(() => status.value.running)
  const port = computed(() => status.value.port)

  // selectedLog now returns the summary - full details must be fetched separately
  const selectedLog = computed(() =>
    requestLogs.value.find(log => log.id === selectedLogId.value) || null
  )

  // Get full log details from cache or fetch from backend
  async function getLogDetails(id: string): Promise<models.RequestLog | null> {
    // Check cache first
    if (requestLogCache.value.has(id)) {
      return requestLogCache.value.get(id)!
    }

    // Fetch from backend
    try {
      const fullLog = await GetRequestLogDetails(id)
      // Cache it
      requestLogCache.value.set(id, fullLog)
      return fullLog
    } catch (error) {
      console.error('Failed to fetch log details:', error)
      return null
    }
  }
  const currentEndpoint = computed(() =>
    endpoints.value.find(ep => ep.id === selectedEndpointId.value) || null
  )

  // Get health status for an endpoint
  function getEndpointHealth(endpointId: string): models.HealthStatus | undefined {
    return endpointHealth.value.get(endpointId)
  }

  // Get container status for an endpoint
  function getContainerStatus(endpointId: string): models.ContainerStatus | undefined {
    return containerStatus.value.get(endpointId)
  }

  // Get container stats for an endpoint
  function getContainerStats(endpointId: string): models.ContainerStats | undefined {
    return containerStats.value.get(endpointId)
  }

  // Actions
  async function startServer(serverPort: number) {
    try {
      await StartServer(serverPort)
      status.value = new main.ServerStatus({ running: true, port: serverPort })
    } catch (error) {
      status.value = new main.ServerStatus({ running: false, port: serverPort, error: String(error) })
      throw error
    }
  }

  async function stopServer() {
    try {
      await StopServer()
      status.value = new main.ServerStatus({ running: false, port: status.value.port })
    } catch (error) {
      status.value.error = String(error)
      throw error
    }
  }

  async function refreshStatus() {
    try {
      const newStatus = await GetServerStatus()
      status.value = newStatus
    } catch (error) {
      console.error('Failed to get server status:', error)
    }
  }

  async function refreshConfig() {
    try {
      const appConfig = await GetConfig()
      config.value = appConfig
      return appConfig
    } catch (error) {
      console.error('Failed to get config:', error)
      throw error
    }
  }

  async function refreshItems() {
    try {
      const result = await GetItems()
      items.value = result || []
    } catch (error) {
      console.error('Failed to get items:', error)
    }
  }

  async function saveItems() {
    try {
      await SetItems(items.value)
    } catch (error) {
      console.error('Failed to save items:', error)
      throw error
    }
  }

  async function addNewResponse() {
    const newResponse = new models.MethodResponse({
      id: crypto.randomUUID(),
      path_pattern: '/new-path',
      methods: ['GET', 'POST'],
      status_code: 200,
      status_text: 'OK',
      headers: {},
      body: '',
      response_delay: 0,
    })

    const item = new models.ResponseItem({
      type: 'response',
      response: newResponse
    })

    items.value = [...items.value, item]
    expandedItemId.value = newResponse.id || null
    await saveItems()
    return newResponse
  }

  async function addNewGroup() {
    try {
      const group = await AddGroup('New Group')
      await refreshItems()
      expandedItemId.value = group.id || null
      return group
    } catch (error) {
      console.error('Failed to add group:', error)
      throw error
    }
  }

  async function updateItem(index: number, item: models.ResponseItem) {
    items.value[index] = item
    await saveItems()
  }

  async function removeItem(index: number) {
    const item = items.value[index]
    const itemId = item.type === 'response' ? item.response?.id : item.group?.id

    items.value = items.value.filter((_, i) => i !== index)

    if (expandedItemId.value === itemId) {
      expandedItemId.value = null
    }

    await saveItems()
  }

  async function reorderItems(fromIndex: number, toIndex: number) {
    const newItems = [...items.value]
    const [removed] = newItems.splice(fromIndex, 1)
    newItems.splice(toIndex, 0, removed)
    items.value = newItems
    await saveItems()
  }

  function toggleExpanded(id: string) {
    if (expandedItemId.value === id) {
      expandedItemId.value = null
    } else {
      expandedItemId.value = id
    }
  }

  async function refreshLogs() {
    try {
      const logs = await GetRequestLogs()
      requestLogs.value = logs || []
    } catch (error) {
      console.error('Failed to get request logs:', error)
    }
  }

  async function clearLogs() {
    try {
      await ClearRequestLogs()
      requestLogs.value = []
      selectedLogId.value = null
    } catch (error) {
      console.error('Failed to clear logs:', error)
    }
  }

  function selectLog(id: string | null) {
    selectedLogId.value = id
  }

  async function importOpenAPISpec(appendMode: boolean) {
    try {
      await ImportOpenAPISpecWithDialog(appendMode)
      await refreshItems()
    } catch (error) {
      console.error('Failed to import OpenAPI spec:', error)
      throw error
    }
  }

  // Endpoint Actions
  async function refreshEndpoints() {
    try {
      const result = await GetEndpoints()
      endpoints.value = result || []
    } catch (error) {
      console.error('Failed to get endpoints:', error)
    }
  }

  async function selectEndpoint(id: string) {
    try {
      await SetSelectedEndpointId(id)
      selectedEndpointId.value = id
      // Refresh items for the newly selected endpoint
      await refreshItems()
    } catch (error) {
      console.error('Failed to select endpoint:', error)
      throw error
    }
  }

  async function addNewEndpoint(name: string, pathPrefix: string, translationMode: string, endpointType: string = 'mock') {
    try {
      const endpoint = await AddEndpoint(name, pathPrefix, translationMode, endpointType)
      await refreshEndpoints()
      // Auto-select the newly created endpoint
      await selectEndpoint(endpoint.id)
      return endpoint
    } catch (error) {
      console.error('Failed to add endpoint:', error)
      throw error
    }
  }

  async function addNewEndpointWithConfig(config: any) {
    try {
      const endpoint = await AddEndpointWithConfig(config)
      await refreshEndpoints()
      // Auto-select the newly created endpoint
      await selectEndpoint(endpoint.id)
      return endpoint
    } catch (error) {
      console.error('Failed to add endpoint with config:', error)
      throw error
    }
  }

  async function updateEndpointById(endpoint: models.Endpoint) {
    try {
      await UpdateEndpoint(endpoint)
      await refreshEndpoints()
    } catch (error) {
      console.error('Failed to update endpoint:', error)
      throw error
    }
  }

  async function deleteEndpointById(id: string) {
    try {
      await DeleteEndpoint(id)
      await refreshEndpoints()
      // If we deleted the selected endpoint, select the first remaining one
      if (selectedEndpointId.value === id && endpoints.value.length > 0) {
        await selectEndpoint(endpoints.value[0].id)
      }
    } catch (error) {
      console.error('Failed to delete endpoint:', error)
      throw error
    }
  }

  // HTTPS Actions
  async function loadCAInfo() {
    try {
      const info = await GetCACertInfo()
      caInfo.value = info
      return info
    } catch (error) {
      console.error('Failed to load CA info:', error)
      throw error
    }
  }

  async function regenerateCA() {
    try {
      await RegenerateCA()
      await loadCAInfo()
    } catch (error) {
      console.error('Failed to regenerate CA:', error)
      throw error
    }
  }

  async function downloadCA() {
    try {
      const path = await DownloadCACert()
      return path
    } catch (error) {
      console.error('Failed to download CA certificate:', error)
      throw error
    }
  }

  // CORS Actions
  async function loadCORSConfig() {
    try {
      const config = await GetCORSConfig()
      corsConfig.value = config
      return config
    } catch (error) {
      console.error('Failed to load CORS config:', error)
      throw error
    }
  }

  async function saveCORSConfig(config: models.CORSConfig) {
    try {
      await SetCORSConfig(config)
      corsConfig.value = config
    } catch (error) {
      console.error('Failed to save CORS config:', error)
      throw error
    }
  }

  async function validateCORSScript(script: string): Promise<void> {
    return ValidateCORSScript(script)
  }

  async function validateCORSHeaderExpression(expression: string): Promise<void> {
    return ValidateCORSHeaderExpression(expression)
  }

  // Health Check Actions
  async function refreshEndpointHealth(endpointId: string) {
    try {
      const health = await GetEndpointHealth(endpointId)
      endpointHealth.value.set(endpointId, health)
      return health
    } catch (error) {
      console.error('Failed to get endpoint health:', error)
      throw error
    }
  }

  async function testProxyConnection(backendURL: string): Promise<boolean> {
    try {
      await TestProxyConnection(backendURL)
      return true
    } catch (error) {
      return false
    }
  }

  async function validateDockerImage(imageName: string): Promise<void> {
    return ValidateDockerImage(imageName)
  }

  async function restartContainerEndpoint(endpointId: string): Promise<void> {
    try {
      await RestartContainer(endpointId)
    } catch (error) {
      console.error('Failed to restart container:', error)
      throw error
    }
  }

  // Start health polling for proxy and container endpoints
  let healthPollingInterval: number | null = null

  function startHealthPolling() {
    // Clear existing interval if any
    if (healthPollingInterval !== null) {
      clearInterval(healthPollingInterval)
    }

    // Poll every 10 seconds
    healthPollingInterval = window.setInterval(() => {
      // Only poll if server is running
      if (!isRunning.value) {
        return
      }

      endpoints.value.forEach(endpoint => {
        // Poll health for proxy endpoints with health checks enabled
        if (endpoint.type === 'proxy' && endpoint.proxy_config?.health_check_enabled) {
          refreshEndpointHealth(endpoint.id).catch(() => {
            // Ignore errors during polling
          })
        }
        // Poll health for container endpoints with health checks enabled
        else if (endpoint.type === 'container' && endpoint.container_config?.proxy_config?.health_check_enabled) {
          refreshEndpointHealth(endpoint.id).catch(() => {
            // Ignore errors during polling
          })
        }
      })
    }, 10000)
  }

  function stopHealthPolling() {
    if (healthPollingInterval !== null) {
      clearInterval(healthPollingInterval)
      healthPollingInterval = null
    }
  }

  // Start request log polling for efficient batching during high-volume traffic
  let requestLogPollingInterval: number | null = null

  function startRequestLogPolling() {
    // Clear existing interval if any
    if (requestLogPollingInterval !== null) {
      clearInterval(requestLogPollingInterval)
    }

    // Poll every 200ms (balance between responsiveness and performance)
    requestLogPollingInterval = window.setInterval(async () => {
      // Only poll if server is running
      if (!isRunning.value) {
        return
      }

      try {
        const summaries = await PollRequestLogs()
        if (summaries && summaries.length > 0) {
          // For each new summary, either update existing log (if ID matches) or append new
          const existingLogs = [...requestLogs.value]
          summaries.forEach(newLog => {
            const existingIndex = existingLogs.findIndex(log => log.id === newLog.id)
            if (existingIndex >= 0) {
              // Update existing log (e.g., pending → complete)
              existingLogs[existingIndex] = newLog
            } else {
              // Append new log
              existingLogs.push(newLog)
            }
          })
          requestLogs.value = existingLogs
        }
      } catch (error) {
        // Ignore errors during polling to prevent console spam
      }
    }, 200)
  }

  function stopRequestLogPolling() {
    if (requestLogPollingInterval !== null) {
      clearInterval(requestLogPollingInterval)
      requestLogPollingInterval = null
    }
  }

  // Set up event listeners (clean up existing ones first to prevent duplicates)
  function initEventListeners() {
    // Remove any existing listeners first
    EventsOff('server:status')
    EventsOff('logs:cleared')
    EventsOff('items:updated')
    EventsOff('endpoints:updated')
    EventsOff('endpoint:selected')
    // NOTE: ctr:* events are handled via polling in HeaderBar.vue

    // Set up fresh listeners
    EventsOn('server:status', (newStatus: main.ServerStatus) => {
      status.value = newStatus
    })

    // NOTE: request:received events are now handled via polling for better performance
    // during high-volume traffic (see startRequestLogPolling)

    EventsOn('logs:cleared', () => {
      requestLogs.value = []
      requestLogCache.value.clear()
      selectedLogId.value = null
    })

    EventsOn('items:updated', (newItems: models.ResponseItem[]) => {
      items.value = newItems
    })

    EventsOn('endpoints:updated', (newEndpoints: models.Endpoint[]) => {
      endpoints.value = newEndpoints
    })

    EventsOn('endpoint:selected', (endpointId: string) => {
      selectedEndpointId.value = endpointId
      refreshItems()
    })

    // NOTE: ctr:status, ctr:stats, and ctr:progress events are now handled via polling
    // in HeaderBar.vue, which updates the store directly

    // Load initial data
    refreshItems()
    refreshConfig()
    refreshEndpoints()

    // Load selected endpoint ID
    GetSelectedEndpointId().then(id => {
      selectedEndpointId.value = id || ''
    }).catch(error => {
      console.error('Failed to load selected endpoint ID:', error)
    })

    // Start health polling
    startHealthPolling()

    // Start request log polling
    startRequestLogPolling()
  }

  return {
    // State
    status,
    requestLogs,
    selectedLogId,
    items,
    expandedItemId,
    config,
    endpoints,
    selectedEndpointId,
    caInfo,
    corsConfig,
    endpointHealth,
    containerStatus,
    containerStats,
    // Getters
    isRunning,
    port,
    selectedLog,
    getLogDetails,
    currentEndpoint,
    getEndpointHealth,
    getContainerStatus,
    getContainerStats,
    // Actions
    startServer,
    stopServer,
    refreshStatus,
    refreshConfig,
    refreshItems,
    saveItems,
    addNewResponse,
    addNewGroup,
    updateItem,
    removeItem,
    reorderItems,
    toggleExpanded,
    refreshLogs,
    clearLogs,
    selectLog,
    importOpenAPISpec,
    // Endpoint Actions
    refreshEndpoints,
    selectEndpoint,
    addNewEndpoint,
    addNewEndpointWithConfig,
    updateEndpointById,
    deleteEndpointById,
    // HTTPS Actions
    loadCAInfo,
    regenerateCA,
    downloadCA,
    // CORS Actions
    loadCORSConfig,
    saveCORSConfig,
    validateCORSScript,
    validateCORSHeaderExpression,
    // Health Check Actions
    refreshEndpointHealth,
    testProxyConnection,
    validateDockerImage,
    restartContainerEndpoint,
    startHealthPolling,
    stopHealthPolling,
    startRequestLogPolling,
    stopRequestLogPolling,
    initEventListeners,
  }
})
