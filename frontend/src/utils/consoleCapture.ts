/**
 * Console Capture Utility
 * Intercepts console.log/error/warn/debug and forwards to log management system
 * while preserving original console functionality
 */

export interface ConsoleLogEntry {
  id: string
  timestamp: string
  level: 'debug' | 'info' | 'warn' | 'error'
  source: string
  message: string
  category: 'frontend'
  args: any[]
}

export class ConsoleCapture {
  private originalConsole: {
    log: typeof console.log
    error: typeof console.error
    warn: typeof console.warn
    debug: typeof console.debug
  }

  private logCallback: ((entry: ConsoleLogEntry) => void) | null = null
  private isInstalled = false

  constructor() {
    // Store original console methods
    this.originalConsole = {
      log: console.log.bind(console),
      error: console.error.bind(console),
      warn: console.warn.bind(console),
      debug: console.debug.bind(console)
    }
  }

  /**
   * Install console override and start capturing
   * @param callback Function to receive captured log entries
   */
  install(callback: (entry: ConsoleLogEntry) => void): void {
    if (this.isInstalled) {
      this.originalConsole.warn('ConsoleCapture already installed')
      return
    }

    this.logCallback = callback
    this.isInstalled = true

    // Override console.log
    console.log = (...args: any[]) => {
      this.captureLog('info', args)
      this.originalConsole.log(...args)
    }

    // Override console.error
    console.error = (...args: any[]) => {
      this.captureLog('error', args)
      this.originalConsole.error(...args)
    }

    // Override console.warn
    console.warn = (...args: any[]) => {
      this.captureLog('warn', args)
      this.originalConsole.warn(...args)
    }

    // Override console.debug
    console.debug = (...args: any[]) => {
      this.captureLog('debug', args)
      this.originalConsole.debug(...args)
    }
  }

  /**
   * Restore original console methods
   */
  restore(): void {
    if (!this.isInstalled) {
      return
    }

    console.log = this.originalConsole.log
    console.error = this.originalConsole.error
    console.warn = this.originalConsole.warn
    console.debug = this.originalConsole.debug

    this.isInstalled = false
    this.logCallback = null
  }

  /**
   * Capture a log entry and send to callback
   */
  private captureLog(level: ConsoleLogEntry['level'], args: any[]): void {
    if (!this.logCallback) {
      return
    }

    // Format message from arguments
    const message = args
      .map(arg => {
        if (typeof arg === 'string') {
          return arg
        } else if (arg instanceof Error) {
          return `${arg.name}: ${arg.message}`
        } else if (typeof arg === 'object') {
          try {
            return JSON.stringify(arg)
          } catch {
            return String(arg)
          }
        } else {
          return String(arg)
        }
      })
      .join(' ')

    // Extract source from stack trace (optional)
    const source = this.extractSource()

    // Create log entry
    const entry: ConsoleLogEntry = {
      id: this.generateId(),
      timestamp: new Date().toISOString(),
      level,
      source,
      message,
      category: 'frontend',
      args
    }

    // Send to callback
    try {
      this.logCallback(entry)
    } catch (error) {
      // Don't let callback errors break console functionality
      this.originalConsole.error('ConsoleCapture callback error:', error)
    }
  }

  /**
   * Extract source file/line from stack trace
   */
  private extractSource(): string {
    try {
      const stack = new Error().stack
      if (!stack) return 'frontend'

      // Parse stack trace to get caller location (skip internal frames)
      const lines = stack.split('\n')
      // Skip first 3 lines (Error, captureLog, console override)
      for (let i = 3; i < lines.length; i++) {
        const line = lines[i]
        // Look for file reference
        const match = line.match(/at\s+(.+?)\s+\((.+?):(\d+):(\d+)\)/) ||
                     line.match(/at\s+(.+?):(\d+):(\d+)/)
        if (match) {
          if (match.length === 5) {
            // Function name present
            const filename = match[2].split('/').pop() || match[2]
            return `${filename}:${match[3]}`
          } else if (match.length === 4) {
            // No function name
            const filename = match[1].split('/').pop() || match[1]
            return `${filename}:${match[2]}`
          }
        }
      }
    } catch {
      // Ignore stack trace parsing errors
    }
    return 'frontend'
  }

  /**
   * Generate unique ID for log entry
   */
  private generateId(): string {
    // Simple unique ID based on timestamp + random
    return `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`
  }
}

// Export singleton instance
export const consoleCapture = new ConsoleCapture()
