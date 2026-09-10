type ManagementPage = 'users' | 'groups'
type DiagnosticValue = string | number | boolean | null | undefined

export interface ManagementPageLogger {
  info: (event: string, details?: Record<string, DiagnosticValue>) => void
  warn: (event: string, details?: Record<string, DiagnosticValue>) => void
}

let nextInstance = 0

export function createManagementPageLogger(page: ManagementPage): ManagementPageLogger {
  const instance = `${page}-${Date.now().toString(36)}-${++nextInstance}`
  const payload = (event: string, details: Record<string, DiagnosticValue> = {}) => ({
    page,
    instance,
    event,
    at: new Date().toISOString(),
    ...details
  })

  return {
    info: (event, details) => console.info('[management-page]', payload(event, details)),
    warn: (event, details) => console.warn('[management-page]', payload(event, details))
  }
}

export function managementRequestErrorDetails(error: unknown): Record<string, DiagnosticValue> {
  if (!error || typeof error !== 'object') return { error_type: typeof error }
  const candidate = error as {
    name?: unknown
    code?: unknown
    status?: unknown
    response?: { status?: unknown }
  }
  return {
    error_name: typeof candidate.name === 'string' ? candidate.name : undefined,
    error_code:
      typeof candidate.code === 'string' || typeof candidate.code === 'number'
        ? String(candidate.code)
        : undefined,
    status:
      typeof candidate.response?.status === 'number'
        ? candidate.response.status
        : typeof candidate.status === 'number'
          ? candidate.status
          : undefined
  }
}
