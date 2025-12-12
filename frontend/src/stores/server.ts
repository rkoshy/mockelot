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
  ValidateCORSHeaderExpression
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export const useServerStore = defineStore('server', () => {
  // State
  const status = ref<main.ServerStatus>(new main.ServerStatus({ running: false, port: 8080 }))
  const requestLogs = ref<models.RequestLog[]>([])
  const selectedLogId = ref<string | null>(null)
  const items = ref<models.ResponseItem[]>([])
  const expandedItemId = ref<string | null>(null)
  const config = ref<models.AppConfig | null>(null)

  // HTTPS & CORS State
  const caInfo = ref<models.CACertInfo | null>(null)
  const corsConfig = ref<models.CORSConfig | null>(null)

  // Getters
  const isRunning = computed(() => status.value.running)
  const port = computed(() => status.value.port)
  const selectedLog = computed(() =>
    requestLogs.value.find(log => log.id === selectedLogId.value) || null
  )

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

  // Set up event listeners
  function initEventListeners() {
    EventsOn('server:status', (newStatus: main.ServerStatus) => {
      status.value = newStatus
    })

    EventsOn('request:received', (log: models.RequestLog) => {
      requestLogs.value = [...requestLogs.value, log]
    })

    EventsOn('logs:cleared', () => {
      requestLogs.value = []
      selectedLogId.value = null
    })

    EventsOn('items:updated', (newItems: models.ResponseItem[]) => {
      items.value = newItems
    })

    // Load initial data
    refreshItems()
    refreshConfig()
  }

  return {
    // State
    status,
    requestLogs,
    selectedLogId,
    items,
    expandedItemId,
    config,
    caInfo,
    corsConfig,
    // Getters
    isRunning,
    port,
    selectedLog,
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
    // HTTPS Actions
    loadCAInfo,
    regenerateCA,
    downloadCA,
    // CORS Actions
    loadCORSConfig,
    saveCORSConfig,
    validateCORSScript,
    validateCORSHeaderExpression,
    initEventListeners,
  }
})
