/**
 * Log Store - Manages application logs with localStorage persistence
 * Handles both backend (Go) and frontend (console) logs
 */

import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import type { ConsoleLogEntry } from '../utils/consoleCapture'

// Log entry interface (matches backend logger.LogEntry)
export interface LogEntry {
  id: string
  timestamp: string
  level: string
  source: string
  message: string
  category: 'backend' | 'frontend'
}

// Metadata for localStorage
interface LogMetadata {
  version: number
  totalLogs: number
  oldestTimestamp: string
  newestTimestamp: string
  estimatedSize: number
}

interface LogStorage {
  version: number
  logs: LogEntry[]
  metadata: LogMetadata
}

const STORAGE_KEY = 'mockelot_app_logs'
const MAX_STORAGE_SIZE = 5 * 1024 * 1024 // 5MB limit
const ROTATION_THRESHOLD = 4.5 * 1024 * 1024 // 4.5MB - rotate before hitting limit
const ROTATION_PERCENTAGE = 0.2 // Remove oldest 20% when rotating

export const useLogStore = defineStore('logs', () => {
  // State
  const logs = ref<LogEntry[]>([])
  const maxStorageSize = ref(MAX_STORAGE_SIZE)
  const currentSize = ref(0)
  const isAutoScroll = ref(true)
  const isPaused = ref(false)

  // Search and filter state
  const searchQuery = ref('')
  const searchRegex = ref(false)
  const selectedLevels = ref<Set<string>>(new Set(['DEBUG', 'INFO', 'WARN', 'ERROR']))
  const selectedSources = ref<Set<string>>(new Set(['backend', 'frontend']))

  // Computed
  const filteredLogs = computed(() => {
    let filtered = logs.value

    // Filter by level
    if (selectedLevels.value.size > 0) {
      filtered = filtered.filter(log => selectedLevels.value.has(log.level.toUpperCase()))
    }

    // Filter by source
    if (selectedSources.value.size > 0) {
      filtered = filtered.filter(log => selectedSources.value.has(log.category))
    }

    // Search filter
    if (searchQuery.value) {
      if (searchRegex.value) {
        try {
          const regex = new RegExp(searchQuery.value, 'i')
          filtered = filtered.filter(log => regex.test(log.message))
        } catch {
          // Invalid regex, fallback to text search
          const query = searchQuery.value.toLowerCase()
          filtered = filtered.filter(log => log.message.toLowerCase().includes(query))
        }
      } else {
        const query = searchQuery.value.toLowerCase()
        filtered = filtered.filter(log => log.message.toLowerCase().includes(query))
      }
    }

    return filtered
  })

  const logCount = computed(() => logs.value.length)
  const filteredLogCount = computed(() => filteredLogs.value.length)

  // Level counts for filter badges
  const levelCounts = computed(() => {
    const counts: Record<string, number> = {
      DEBUG: 0,
      INFO: 0,
      WARN: 0,
      ERROR: 0
    }
    logs.value.forEach(log => {
      const level = log.level.toUpperCase()
      if (level in counts) {
        counts[level]++
      }
    })
    return counts
  })

  // Actions

  /**
   * Add a single log entry
   */
  function addLog(entry: LogEntry | ConsoleLogEntry) {
    if (isPaused.value) {
      return
    }

    // Convert ConsoleLogEntry to LogEntry if needed
    const logEntry: LogEntry = 'args' in entry
      ? {
          id: entry.id,
          timestamp: entry.timestamp,
          level: entry.level.toUpperCase(),
          source: entry.source,
          message: entry.message,
          category: entry.category
        }
      : entry

    logs.value.push(logEntry)
    updateSize()
    checkRotation()
  }

  /**
   * Add multiple log entries (batch)
   */
  function addLogs(entries: LogEntry[]) {
    if (isPaused.value || entries.length === 0) {
      return
    }

    logs.value.push(...entries)
    updateSize()
    checkRotation()
  }

  /**
   * Clear all logs
   */
  function clearLogs() {
    logs.value = []
    currentSize.value = 0
    saveToStorage()
  }

  /**
   * Export logs in specified format
   */
  function exportLogs(format: 'json' | 'txt' | 'csv'): string {
    switch (format) {
      case 'json':
        return JSON.stringify(logs.value, null, 2)

      case 'txt':
        return logs.value
          .map(log => `[${log.timestamp}] [${log.level}] [${log.source}] ${log.message}`)
          .join('\n')

      case 'csv':
        const header = 'Timestamp,Level,Source,Category,Message\n'
        const rows = logs.value
          .map(log => {
            const message = log.message.replace(/"/g, '""') // Escape quotes
            return `"${log.timestamp}","${log.level}","${log.source}","${log.category}","${message}"`
          })
          .join('\n')
        return header + rows

      default:
        return ''
    }
  }

  /**
   * Search logs with optional regex
   */
  function searchLogs(query: string, useRegex: boolean = false): LogEntry[] {
    if (!query) {
      return logs.value
    }

    if (useRegex) {
      try {
        const regex = new RegExp(query, 'i')
        return logs.value.filter(log => regex.test(log.message))
      } catch {
        // Invalid regex, fallback to text search
        const lowerQuery = query.toLowerCase()
        return logs.value.filter(log => log.message.toLowerCase().includes(lowerQuery))
      }
    } else {
      const lowerQuery = query.toLowerCase()
      return logs.value.filter(log => log.message.toLowerCase().includes(lowerQuery))
    }
  }

  /**
   * Filter logs by level
   */
  function filterByLevel(levels: string[]): LogEntry[] {
    const levelSet = new Set(levels.map(l => l.toUpperCase()))
    return logs.value.filter(log => levelSet.has(log.level.toUpperCase()))
  }

  /**
   * Filter logs by source
   */
  function filterBySource(sources: string[]): LogEntry[] {
    const sourceSet = new Set(sources)
    return logs.value.filter(log => sourceSet.has(log.category))
  }

  /**
   * Rotate logs when approaching size limit (FIFO)
   */
  function rotateLogs() {
    if (logs.value.length === 0) {
      return
    }

    const removeCount = Math.floor(logs.value.length * ROTATION_PERCENTAGE)
    logs.value = logs.value.slice(removeCount) // Remove oldest 20%
    updateSize()
    saveToStorage()
  }

  /**
   * Check if rotation is needed and perform it
   */
  function checkRotation() {
    if (currentSize.value > ROTATION_THRESHOLD) {
      rotateLogs()
    }
  }

  /**
   * Estimate current storage size
   */
  function updateSize() {
    const jsonString = JSON.stringify(logs.value)
    currentSize.value = new Blob([jsonString]).size
  }

  /**
   * Save logs to localStorage (debounced in practice via watch)
   */
  function saveToStorage() {
    try {
      const metadata: LogMetadata = {
        version: 1,
        totalLogs: logs.value.length,
        oldestTimestamp: logs.value[0]?.timestamp || '',
        newestTimestamp: logs.value[logs.value.length - 1]?.timestamp || '',
        estimatedSize: currentSize.value
      }

      const storage: LogStorage = {
        version: 1,
        logs: logs.value,
        metadata
      }

      localStorage.setItem(STORAGE_KEY, JSON.stringify(storage))
    } catch (error) {
      console.error('Failed to save logs to localStorage:', error)
      // If quota exceeded, force rotation and try again
      if (error instanceof DOMException && error.name === 'QuotaExceededError') {
        rotateLogs()
        try {
          const storage: LogStorage = {
            version: 1,
            logs: logs.value,
            metadata: {
              version: 1,
              totalLogs: logs.value.length,
              oldestTimestamp: logs.value[0]?.timestamp || '',
              newestTimestamp: logs.value[logs.value.length - 1]?.timestamp || '',
              estimatedSize: currentSize.value
            }
          }
          localStorage.setItem(STORAGE_KEY, JSON.stringify(storage))
        } catch {
          // Give up if still failing
          console.error('Failed to save logs even after rotation')
        }
      }
    }
  }

  /**
   * Load logs from localStorage
   */
  function loadFromStorage() {
    try {
      const stored = localStorage.getItem(STORAGE_KEY)
      if (!stored) {
        return
      }

      const storage: LogStorage = JSON.parse(stored)
      if (storage.version === 1 && Array.isArray(storage.logs)) {
        logs.value = storage.logs
        updateSize()
      }
    } catch (error) {
      console.error('Failed to load logs from localStorage:', error)
    }
  }

  // Watch for changes and debounce save to localStorage
  let saveTimeout: number | null = null
  watch(
    () => logs.value.length,
    () => {
      if (saveTimeout !== null) {
        clearTimeout(saveTimeout)
      }
      saveTimeout = window.setTimeout(() => {
        saveToStorage()
      }, 1000) // Debounce 1 second
    }
  )

  // Load logs on initialization
  loadFromStorage()

  return {
    // State
    logs,
    maxStorageSize,
    currentSize,
    isAutoScroll,
    isPaused,
    searchQuery,
    searchRegex,
    selectedLevels,
    selectedSources,

    // Computed
    filteredLogs,
    logCount,
    filteredLogCount,
    levelCounts,

    // Actions
    addLog,
    addLogs,
    clearLogs,
    exportLogs,
    searchLogs,
    filterByLevel,
    filterBySource,
    rotateLogs,
    saveToStorage,
    loadFromStorage
  }
})
