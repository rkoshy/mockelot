package logger

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LogLevel represents the severity of a log entry
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a single log entry
type LogEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	Category  string `json:"category"` // "backend" or "frontend"
}

// EventSender interface for sending events to frontend
type EventSender interface {
	SendEvent(source string, data interface{})
}

// Logger is a structured logger with levels and event emission
type Logger struct {
	minLevel    LogLevel
	eventSender EventSender
	entries     []LogEntry
	mutex       sync.RWMutex
	maxEntries  int
	source      string // Source identifier (e.g., "backend", "server", "app")
}

// NewLogger creates a new logger instance
func NewLogger(source string, minLevel LogLevel, maxEntries int, eventSender EventSender) *Logger {
	return &Logger{
		minLevel:    minLevel,
		eventSender: eventSender,
		entries:     make([]LogEntry, 0, maxEntries),
		maxEntries:  maxEntries,
		source:      source,
	}
}

// log is the internal logging method
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	// Skip if below minimum level
	if level < l.minLevel {
		return
	}

	message := fmt.Sprintf(format, args...)

	entry := LogEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level.String(),
		Source:    l.source,
		Message:   message,
		Category:  "backend",
	}

	l.mutex.Lock()
	// Circular buffer: remove oldest if at capacity
	if len(l.entries) >= l.maxEntries {
		l.entries = l.entries[1:]
	}
	l.entries = append(l.entries, entry)
	l.mutex.Unlock()

	// Send event to frontend
	if l.eventSender != nil {
		l.eventSender.SendEvent("log", entry)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an informational message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// GetLogs returns all stored log entries
func (l *Logger) GetLogs() []LogEntry {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// Return a copy to prevent external modification
	logs := make([]LogEntry, len(l.entries))
	copy(logs, l.entries)
	return logs
}

// GetLogsSince returns log entries since the given timestamp
func (l *Logger) GetLogsSince(since string) []LogEntry {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	var result []LogEntry
	for _, entry := range l.entries {
		if entry.Timestamp > since {
			result = append(result, entry)
		}
	}
	return result
}

// Clear clears all stored log entries
func (l *Logger) Clear() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.entries = make([]LogEntry, 0, l.maxEntries)
}

// SetMinLevel sets the minimum log level
func (l *Logger) SetMinLevel(level LogLevel) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.minLevel = level
}

// GetMinLevel returns the current minimum log level
func (l *Logger) GetMinLevel() LogLevel {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.minLevel
}

// Count returns the number of stored log entries
func (l *Logger) Count() int {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return len(l.entries)
}
