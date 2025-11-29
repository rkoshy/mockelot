import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { main, models } from '../../wailsjs/go/models'
import {
  StartServer,
  StopServer,
  GetServerStatus,
  GetItems,
  SetItems,
  AddGroup,
  GetRequestLogs,
  ClearRequestLogs,
  ImportOpenAPISpecWithDialog
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export const useServerStore = defineStore('server', () => {
  // State
  const status = ref<main.ServerStatus>(new main.ServerStatus({ running: false, port: 8080 }))
  const requestLogs = ref<models.RequestLog[]>([])
  const selectedLogId = ref<string | null>(null)
  const items = ref<models.ResponseItem[]>([])
  const expandedItemId = ref<string | null>(null)

  // Getters
  const isRunning = computed(() => status.value.running)
  const port = computed(() => status.value.port)
  const selectedLog = computed(() =>
    requestLogs.value.find(log => log.ID === selectedLogId.value) || null
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

    // Load initial items
    refreshItems()
  }

  return {
    // State
    status,
    requestLogs,
    selectedLogId,
    items,
    expandedItemId,
    // Getters
    isRunning,
    port,
    selectedLog,
    // Actions
    startServer,
    stopServer,
    refreshStatus,
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
    initEventListeners,
  }
})
