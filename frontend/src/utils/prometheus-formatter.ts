/**
 * Custom Prometheus metrics formatter
 *
 * Prometheus exposition format:
 * - Lines starting with # HELP are metric descriptions
 * - Lines starting with # TYPE are metric type declarations
 * - Metric lines: metric_name{label="value",...} value [timestamp]
 */

export interface MetricLine {
  type: 'help' | 'type' | 'metric' | 'comment' | 'empty'
  name?: string
  content: string
  labels?: Record<string, string>
  value?: string
  timestamp?: string
}

// Parse a single line of Prometheus metrics
export function parseLine(line: string): MetricLine {
  const trimmed = line.trim()

  if (!trimmed) {
    return { type: 'empty', content: '' }
  }

  // HELP comment
  if (trimmed.startsWith('# HELP')) {
    const rest = trimmed.slice(6).trim()
    if (!rest) {
      // Empty HELP line - treat as comment
      return { type: 'comment', content: '' }
    }
    const spaceIdx = rest.indexOf(' ')
    if (spaceIdx > 0) {
      return {
        type: 'help',
        name: rest.slice(0, spaceIdx),
        content: rest.slice(spaceIdx + 1)
      }
    }
    return { type: 'help', name: rest, content: '' }
  }

  // TYPE comment
  if (trimmed.startsWith('# TYPE')) {
    const rest = trimmed.slice(6).trim()
    if (!rest) {
      // Empty TYPE line - treat as comment
      return { type: 'comment', content: '' }
    }
    const spaceIdx = rest.indexOf(' ')
    if (spaceIdx > 0) {
      return {
        type: 'type',
        name: rest.slice(0, spaceIdx),
        content: rest.slice(spaceIdx + 1)
      }
    }
    return { type: 'type', name: rest, content: '' }
  }

  // Other comment
  if (trimmed.startsWith('#')) {
    return { type: 'comment', content: trimmed.slice(1).trim() }
  }

  // Metric line
  const labelStart = trimmed.indexOf('{')
  const labelEnd = trimmed.indexOf('}')

  if (labelStart > 0 && labelEnd > labelStart) {
    // Has labels
    const name = trimmed.slice(0, labelStart)
    const labelStr = trimmed.slice(labelStart + 1, labelEnd)
    const valueAndTimestamp = trimmed.slice(labelEnd + 1).trim().split(/\s+/)

    const labels: Record<string, string> = {}
    // Parse labels: key="value",key2="value2"
    const labelRegex = /(\w+)="([^"]*)"/g
    let match
    while ((match = labelRegex.exec(labelStr)) !== null) {
      labels[match[1]] = match[2]
    }

    return {
      type: 'metric',
      name,
      content: trimmed,
      labels,
      value: valueAndTimestamp[0],
      timestamp: valueAndTimestamp[1]
    }
  } else {
    // No labels
    const parts = trimmed.split(/\s+/)
    return {
      type: 'metric',
      name: parts[0],
      content: trimmed,
      labels: {},
      value: parts[1],
      timestamp: parts[2]
    }
  }
}

// Format labels with consistent spacing and alignment
function formatLabels(labels: Record<string, string>, indent: number = 0): string {
  const entries = Object.entries(labels)
  if (entries.length === 0) return ''

  if (entries.length <= 2) {
    // Single line for few labels
    return '{' + entries.map(([k, v]) => `${k}="${v}"`).join(', ') + '}'
  }

  // Multi-line for many labels
  const pad = ' '.repeat(indent)
  const labelLines = entries.map(([k, v]) => `${pad}  ${k}="${v}"`)
  return '{\n' + labelLines.join(',\n') + '\n' + pad + '}'
}

// Group metrics by name
export interface MetricGroup {
  name: string
  help?: string
  type?: string
  metrics: MetricLine[]
}

export function parsePrometheusMetrics(content: string): MetricGroup[] {
  const lines = content.split('\n').map(parseLine)
  return groupMetrics(lines)
}

// Suffixes for related metrics (summary, histogram)
const METRIC_SUFFIXES = ['_count', '_sum', '_total', '_bucket', '_max', '_active_count', '_duration_sum']

// Check if a metric name belongs to a group (exact match or suffix match)
function metricBelongsToGroup(metricName: string, groupName: string): boolean {
  if (metricName === groupName) return true

  // Check if metric is a suffix variant of the group
  for (const suffix of METRIC_SUFFIXES) {
    if (metricName === groupName + suffix) return true
  }

  // Also check if metric name starts with group name followed by underscore
  // This handles cases like metric_name_bucket{le="0.5"}
  if (metricName.startsWith(groupName + '_')) {
    const remainder = metricName.slice(groupName.length + 1)
    // Make sure it's a known suffix pattern, not a completely different metric
    for (const suffix of METRIC_SUFFIXES) {
      if (('_' + remainder) === suffix || remainder.startsWith(suffix.slice(1))) {
        return true
      }
    }
  }

  return false
}

function groupMetrics(lines: MetricLine[]): MetricGroup[] {
  const groups: MetricGroup[] = []
  let currentGroup: MetricGroup | null = null

  for (const line of lines) {
    if (line.type === 'help') {
      // Skip if no name
      if (!line.name) continue

      // Start a new group or update current
      if (!currentGroup || currentGroup.name !== line.name) {
        if (currentGroup && currentGroup.name) groups.push(currentGroup)
        currentGroup = { name: line.name, help: line.content, metrics: [] }
      } else {
        currentGroup.help = line.content
      }
    } else if (line.type === 'type') {
      // Skip if no name
      if (!line.name) continue

      if (!currentGroup || currentGroup.name !== line.name) {
        if (currentGroup && currentGroup.name) groups.push(currentGroup)
        currentGroup = { name: line.name, type: line.content, metrics: [] }
      } else {
        currentGroup.type = line.content
      }
    } else if (line.type === 'metric') {
      // Skip if no name
      if (!line.name) continue

      // Check if this metric belongs to current group (exact or suffix match)
      const belongsToCurrent = currentGroup && currentGroup.name && metricBelongsToGroup(line.name, currentGroup.name)

      if (!currentGroup || !belongsToCurrent) {
        if (currentGroup && currentGroup.name) groups.push(currentGroup)
        currentGroup = { name: line.name, metrics: [line] }
      } else {
        currentGroup.metrics.push(line)
      }
    }
    // Skip empty and generic comments for grouping
  }

  if (currentGroup && currentGroup.name) groups.push(currentGroup)

  // Filter out any groups with empty names or no metrics
  return groups.filter(g => g.name && (g.metrics.length > 0 || g.help || g.type))
}

/**
 * Format Prometheus metrics for display
 */
export function formatPrometheusMetrics(content: string): string {
  const lines = content.split('\n').map(parseLine)
  const groups = groupMetrics(lines)

  const output: string[] = []

  for (const group of groups) {
    // Add blank line between groups
    if (output.length > 0) {
      output.push('')
    }

    // HELP line
    if (group.help) {
      output.push(`# HELP ${group.name} ${group.help}`)
    }

    // TYPE line
    if (group.type) {
      output.push(`# TYPE ${group.name} ${group.type}`)
    }

    // Find max label width for alignment
    const labelWidths = group.metrics.map(m => {
      if (!m.labels || Object.keys(m.labels).length === 0) return 0
      const labelStr = Object.entries(m.labels).map(([k, v]) => `${k}="${v}"`).join(', ')
      return labelStr.length + 2 // for {}
    })
    const maxLabelWidth = Math.max(...labelWidths, 0)

    // Metric lines with aligned values
    for (const metric of group.metrics) {
      let line = metric.name!

      if (metric.labels && Object.keys(metric.labels).length > 0) {
        const labelStr = Object.entries(metric.labels).map(([k, v]) => `${k}="${v}"`).join(', ')
        line += `{${labelStr}}`
        // Pad for alignment
        const padding = maxLabelWidth - (labelStr.length + 2)
        if (padding > 0) {
          line += ' '.repeat(padding)
        }
      } else if (maxLabelWidth > 0) {
        line += ' '.repeat(maxLabelWidth)
      }

      line += ' ' + metric.value

      if (metric.timestamp) {
        line += ' ' + metric.timestamp
      }

      output.push(line)
    }
  }

  return output.join('\n')
}

/**
 * Check if content looks like Prometheus metrics
 */
export function isPrometheusMetrics(content: string): boolean {
  const lines = content.trim().split('\n')

  // Check for typical Prometheus patterns
  let hasMetricPattern = false
  let hasPromComment = false

  for (const line of lines.slice(0, 20)) { // Check first 20 lines
    const trimmed = line.trim()
    if (!trimmed) continue

    // Check for # HELP or # TYPE
    if (trimmed.startsWith('# HELP ') || trimmed.startsWith('# TYPE ')) {
      hasPromComment = true
    }

    // Check for metric pattern: name{labels} value or name value
    if (!trimmed.startsWith('#')) {
      // Should have a numeric value at the end
      const parts = trimmed.split(/\s+/)
      if (parts.length >= 2) {
        const lastPart = parts[parts.length - 1]
        // Check if it's a number (including scientific notation, NaN, +Inf, -Inf)
        if (/^[-+]?(\d+\.?\d*|\d*\.?\d+)([eE][-+]?\d+)?$/.test(lastPart) ||
            lastPart === 'NaN' || lastPart === '+Inf' || lastPart === '-Inf') {
          hasMetricPattern = true
        }
      }
    }

    if (hasPromComment && hasMetricPattern) {
      return true
    }
  }

  // If we have HELP/TYPE comments, it's likely Prometheus even without matching metrics
  return hasPromComment
}

/**
 * Minify Prometheus metrics (remove extra whitespace but preserve structure)
 */
export function minifyPrometheusMetrics(content: string): string {
  return content
    .split('\n')
    .map(line => line.trim())
    .filter(line => line) // Remove empty lines
    .join('\n')
}
