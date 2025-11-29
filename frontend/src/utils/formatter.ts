import * as prettier from 'prettier'
import * as xmlPlugin from '@prettier/plugin-xml'
import { formatPrometheusMetrics, minifyPrometheusMetrics, isPrometheusMetrics } from './prometheus-formatter'

// Common content types for the dropdown
export const COMMON_CONTENT_TYPES = [
  { value: 'application/json', label: 'JSON' },
  { value: 'application/xml', label: 'XML' },
  { value: 'text/xml', label: 'XML (text)' },
  { value: 'text/html', label: 'HTML' },
  { value: 'text/plain', label: 'Plain Text' },
  { value: 'text/plain; version=0.0.4', label: 'Prometheus Metrics' },
  { value: 'application/openmetrics-text', label: 'OpenMetrics' },
  { value: 'text/css', label: 'CSS' },
  { value: 'application/javascript', label: 'JavaScript' },
  { value: 'application/x-www-form-urlencoded', label: 'Form URL Encoded' },
  { value: 'multipart/form-data', label: 'Multipart Form' },
]

// Formatter types for the formatter selector (only types that support formatting)
export const FORMATTER_TYPES = [
  { value: '', label: 'Auto' },
  { value: 'application/json', label: 'JSON' },
  { value: 'application/xml', label: 'XML' },
  { value: 'text/html', label: 'HTML' },
  { value: 'text/css', label: 'CSS' },
  { value: 'application/javascript', label: 'JavaScript' },
  { value: 'text/plain; version=0.0.4', label: 'Prometheus' },
  { value: 'application/x-www-form-urlencoded', label: 'URL Encoded' },
]

// Detect content type from string or infer from content
export function detectContentType(content: string, contentTypeHeader?: string): string {
  if (contentTypeHeader) {
    // Extract main type, ignoring charset etc.
    return contentTypeHeader.split(';')[0].trim().toLowerCase()
  }

  // Try to auto-detect from content
  const trimmed = content.trim()

  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      JSON.parse(trimmed)
      return 'application/json'
    } catch {
      // Not valid JSON
    }
  }

  if (trimmed.startsWith('<?xml') || trimmed.startsWith('<') && trimmed.includes('</')) {
    if (trimmed.toLowerCase().includes('<!doctype html') || trimmed.toLowerCase().includes('<html')) {
      return 'text/html'
    }
    return 'application/xml'
  }

  // Check for Prometheus metrics format
  if (isPrometheusMetrics(trimmed)) {
    return 'text/plain; version=0.0.4'
  }

  return 'text/plain'
}

// Format content based on content type
export async function formatContent(content: string, contentType: string): Promise<string> {
  if (!content.trim()) return content

  const type = contentType.split(';')[0].trim().toLowerCase()

  try {
    switch (type) {
      case 'application/json':
        // Use JSON.parse/stringify for JSON (faster than prettier for JSON)
        const parsed = JSON.parse(content)
        return JSON.stringify(parsed, null, 2)

      case 'application/xml':
      case 'text/xml':
        return await prettier.format(content, {
          parser: 'xml',
          plugins: [xmlPlugin],
          xmlWhitespaceSensitivity: 'ignore',
          tabWidth: 2,
        })

      case 'text/html':
        return await prettier.format(content, {
          parser: 'html',
          tabWidth: 2,
          printWidth: 100,
        })

      case 'text/css':
        return await prettier.format(content, {
          parser: 'css',
          tabWidth: 2,
        })

      case 'application/javascript':
      case 'text/javascript':
        return await prettier.format(content, {
          parser: 'babel',
          tabWidth: 2,
          semi: true,
          singleQuote: true,
        })

      case 'application/x-www-form-urlencoded':
        // Only format if content actually looks like URL-encoded data
        // (protects against incorrect Content-Type headers)
        if (isUrlEncoded(content)) {
          return formatUrlEncoded(content)
        }
        return content

      case 'text/plain; version=0.0.4':
      case 'application/openmetrics-text':
        // Prometheus/OpenMetrics format
        return formatPrometheusMetrics(content)

      default:
        // Check if it looks like Prometheus even without explicit content-type
        if (type === 'text/plain' && isPrometheusMetrics(content)) {
          return formatPrometheusMetrics(content)
        }
        return content
    }
  } catch (error) {
    console.warn('Format error:', error)
    return content // Return original if formatting fails
  }
}

// Minify content based on content type
export function minifyContent(content: string, contentType: string): string {
  if (!content.trim()) return content

  const type = contentType.toLowerCase()
  const baseType = type.split(';')[0].trim()

  try {
    switch (baseType) {
      case 'application/json':
        return JSON.stringify(JSON.parse(content))

      case 'application/xml':
      case 'text/xml':
      case 'text/html':
        // Remove extra whitespace between tags
        return content
          .replace(/>\s+</g, '><')
          .replace(/\s+/g, ' ')
          .trim()

      case 'text/plain':
        // Check for Prometheus content type with version
        if (type.includes('version=0.0.4') || isPrometheusMetrics(content)) {
          return minifyPrometheusMetrics(content)
        }
        return content

      case 'application/openmetrics-text':
        return minifyPrometheusMetrics(content)

      default:
        return content
    }
  } catch {
    return content
  }
}

// Check if content looks like valid URL-encoded data
function isUrlEncoded(content: string): boolean {
  // URL-encoded data should contain key=value pairs
  // Must have at least one = and should not look like JSON/XML
  if (!content.includes('=')) return false
  const trimmed = content.trim()
  if (trimmed.startsWith('{') || trimmed.startsWith('[') || trimmed.startsWith('<')) {
    return false
  }
  // Basic validation: try parsing and see if we get meaningful results
  try {
    const params = new URLSearchParams(content)
    let hasValidPair = false
    params.forEach((value, key) => {
      if (key.trim()) hasValidPair = true
    })
    return hasValidPair
  } catch {
    return false
  }
}

// Format URL encoded data
function formatUrlEncoded(content: string): string {
  try {
    const params = new URLSearchParams(content)
    const lines: string[] = []
    params.forEach((value, key) => {
      lines.push(`${decodeURIComponent(key)}=${decodeURIComponent(value)}`)
    })

    return lines.length > 0 ? lines.join('\n') : content
  } catch {
    return content
  }
}

// Check if content type supports formatting
export function supportsFormatting(contentType: string): boolean {
  const type = contentType.toLowerCase()
  const baseType = type.split(';')[0].trim()

  // Direct matches
  if ([
    'application/json',
    'application/xml',
    'text/xml',
    'text/html',
    'text/css',
    'application/javascript',
    'text/javascript',
    'application/x-www-form-urlencoded',
    'application/openmetrics-text',
  ].includes(baseType)) {
    return true
  }

  // Prometheus metrics (text/plain with version=0.0.4)
  if (baseType === 'text/plain' && type.includes('version=0.0.4')) {
    return true
  }

  return false
}
