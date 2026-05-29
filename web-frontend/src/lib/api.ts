// Thin fetch wrapper. Attaches the JWT, normalises errors, and triggers a
// global logout on 401 so a stale token can't leave the UI in a broken state.

export class ApiRequestError extends Error {
  status: number
  payload: unknown
  constructor(status: number, message: string, payload: unknown) {
    super(message)
    this.name = 'ApiRequestError'
    this.status = status
    this.payload = payload
  }
}

let onUnauthorized: (() => void) | null = null
let tokenGetter: () => string | null = () => null

// Wired once at app boot by AuthProvider so api.ts stays free of store imports.
export function configureApi(opts: {
  getToken: () => string | null
  onUnauthorized: () => void
}) {
  tokenGetter = opts.getToken
  onUnauthorized = opts.onUnauthorized
}

interface RequestOptions {
  method?: string
  body?: unknown
  signal?: AbortSignal
}

export async function apiRequest<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  const token = tokenGetter()
  if (token) headers.Authorization = `Bearer ${token}`

  let body: string | undefined
  if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }

  const res = await fetch(`/api/v1${path}`, {
    method: opts.method ?? 'GET',
    headers,
    body,
    signal: opts.signal,
  })

  if (res.status === 401) {
    onUnauthorized?.()
    throw new ApiRequestError(401, '登录已过期，请重新登录', null)
  }

  const text = await res.text()
  const data = text ? safeParse(text) : null

  if (!res.ok) {
    const msg = extractError(data) ?? `请求失败（${res.status}）`
    throw new ApiRequestError(res.status, msg, data)
  }
  return data as T
}

function safeParse(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return text
  }
}

function extractError(data: unknown): string | null {
  if (data && typeof data === 'object' && 'error' in data) {
    const e = (data as { error: unknown }).error
    if (typeof e === 'string') return e
  }
  return null
}
