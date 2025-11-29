// Re-export Wails generated types
export { main, models } from '../../wailsjs/go/models'

// Response modes
export const RESPONSE_MODES = ['static', 'template', 'script'] as const
export type ResponseMode = typeof RESPONSE_MODES[number]

// Response mode labels for UI
export const RESPONSE_MODE_LABELS: Record<ResponseMode, string> = {
  static: 'Static',
  template: 'Template',
  script: 'Script',
}

// Response mode descriptions
export const RESPONSE_MODE_DESCRIPTIONS: Record<ResponseMode, string> = {
  static: 'Simple response with no processing',
  template: 'Go text/template with request context variables',
  script: 'JavaScript for complex logic and dynamic responses',
}

// Validation modes
export const VALIDATION_MODES = ['none', 'static', 'regex', 'script'] as const
export type ValidationMode = typeof VALIDATION_MODES[number]

// Validation mode labels for UI
export const VALIDATION_MODE_LABELS: Record<ValidationMode, string> = {
  none: 'None',
  static: 'Static',
  regex: 'Regex',
  script: 'Script',
}

// Validation mode descriptions
export const VALIDATION_MODE_DESCRIPTIONS: Record<ValidationMode, string> = {
  none: 'No validation - always match',
  static: 'Match exact text or check if body contains text',
  regex: 'Match regex pattern with named group extraction',
  script: 'JavaScript validation with variable extraction',
}

// Validation match types (for static mode)
export const VALIDATION_MATCH_TYPES = ['contains', 'exact'] as const
export type ValidationMatchType = typeof VALIDATION_MATCH_TYPES[number]

export const VALIDATION_MATCH_TYPE_LABELS: Record<ValidationMatchType, string> = {
  contains: 'Contains',
  exact: 'Exact Match',
}

// HTTP Methods
export const HTTP_METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS', 'PATCH'] as const
export type HttpMethod = typeof HTTP_METHODS[number]

// All HTTP Status codes (RFC 7231, RFC 6585, RFC 7538, RFC 8297, RFC 8470, WebDAV, etc.)
export const STATUS_CODES = [
  // 1xx Informational
  { code: 100, text: 'Continue' },
  { code: 101, text: 'Switching Protocols' },
  { code: 102, text: 'Processing' },
  { code: 103, text: 'Early Hints' },

  // 2xx Success
  { code: 200, text: 'OK' },
  { code: 201, text: 'Created' },
  { code: 202, text: 'Accepted' },
  { code: 203, text: 'Non-Authoritative Information' },
  { code: 204, text: 'No Content' },
  { code: 205, text: 'Reset Content' },
  { code: 206, text: 'Partial Content' },
  { code: 207, text: 'Multi-Status' },
  { code: 208, text: 'Already Reported' },
  { code: 226, text: 'IM Used' },

  // 3xx Redirection
  { code: 300, text: 'Multiple Choices' },
  { code: 301, text: 'Moved Permanently' },
  { code: 302, text: 'Found' },
  { code: 303, text: 'See Other' },
  { code: 304, text: 'Not Modified' },
  { code: 305, text: 'Use Proxy' },
  { code: 307, text: 'Temporary Redirect' },
  { code: 308, text: 'Permanent Redirect' },

  // 4xx Client Errors
  { code: 400, text: 'Bad Request' },
  { code: 401, text: 'Unauthorized' },
  { code: 402, text: 'Payment Required' },
  { code: 403, text: 'Forbidden' },
  { code: 404, text: 'Not Found' },
  { code: 405, text: 'Method Not Allowed' },
  { code: 406, text: 'Not Acceptable' },
  { code: 407, text: 'Proxy Authentication Required' },
  { code: 408, text: 'Request Timeout' },
  { code: 409, text: 'Conflict' },
  { code: 410, text: 'Gone' },
  { code: 411, text: 'Length Required' },
  { code: 412, text: 'Precondition Failed' },
  { code: 413, text: 'Payload Too Large' },
  { code: 414, text: 'URI Too Long' },
  { code: 415, text: 'Unsupported Media Type' },
  { code: 416, text: 'Range Not Satisfiable' },
  { code: 417, text: 'Expectation Failed' },
  { code: 418, text: "I'm a Teapot" },
  { code: 421, text: 'Misdirected Request' },
  { code: 422, text: 'Unprocessable Entity' },
  { code: 423, text: 'Locked' },
  { code: 424, text: 'Failed Dependency' },
  { code: 425, text: 'Too Early' },
  { code: 426, text: 'Upgrade Required' },
  { code: 428, text: 'Precondition Required' },
  { code: 429, text: 'Too Many Requests' },
  { code: 431, text: 'Request Header Fields Too Large' },
  { code: 451, text: 'Unavailable For Legal Reasons' },

  // 5xx Server Errors
  { code: 500, text: 'Internal Server Error' },
  { code: 501, text: 'Not Implemented' },
  { code: 502, text: 'Bad Gateway' },
  { code: 503, text: 'Service Unavailable' },
  { code: 504, text: 'Gateway Timeout' },
  { code: 505, text: 'HTTP Version Not Supported' },
  { code: 506, text: 'Variant Also Negotiates' },
  { code: 507, text: 'Insufficient Storage' },
  { code: 508, text: 'Loop Detected' },
  { code: 510, text: 'Not Extended' },
  { code: 511, text: 'Network Authentication Required' },
] as const
