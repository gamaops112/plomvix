export class ApiError extends Error {
  public readonly code: string
  public readonly requestId?: string

  constructor(code: string, message: string, requestId?: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.requestId = requestId
  }
}

type Envelope<T> = { status: 'ok'; data: T; request_id: string }

let _onSessionExpired: (() => void) | null = null
export function setSessionExpiredHandler(fn: () => void) {
  _onSessionExpired = fn
}

async function apiRequest<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
  }
  if (body !== undefined) init.body = JSON.stringify(body)

  const res = await fetch(path, init)

  if (res.status === 401) {
    _onSessionExpired?.()
    throw new ApiError('UNAUTHORIZED', 'Session expired')
  }

  const json = await res.json()

  if (json.status === 'error') {
    throw new ApiError(
      json.error?.code ?? 'UNKNOWN',
      json.error?.message ?? 'Unknown error',
      json.request_id,
    )
  }

  return (json as Envelope<T>).data
}

export const apiGet    = <T>(path: string)                => apiRequest<T>('GET',    path)
export const apiPost   = <T>(path: string, body: unknown = {}) => apiRequest<T>('POST',   path, body)
export const apiPut    = <T>(path: string, body: unknown) => apiRequest<T>('PUT',    path, body)
export const apiPatch  = <T>(path: string, body: unknown) => apiRequest<T>('PATCH',  path, body)
export const apiDelete = <T>(path: string)                => apiRequest<T>('DELETE', path)
