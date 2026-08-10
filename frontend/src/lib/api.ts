// Klien REST tipis untuk backend Steca POS.

const TOKEN_KEY = 'pos_steca_token'

export const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

/** ApiError membawa pesan yang sudah ramah pengguna dari backend. */
export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

interface RequestOptions {
  method?: string
  body?: unknown
  /** Endpoint publik tidak mengirim header Authorization. */
  auth?: boolean
  signal?: AbortSignal
}

async function parseError(response: Response): Promise<ApiError> {
  let code = 'error'
  let message = `Permintaan gagal (${response.status})`
  try {
    const payload = await response.json()
    if (payload?.error) {
      code = payload.error.code ?? code
      message = payload.error.message ?? message
    }
  } catch {
    // Body bukan JSON — pakai pesan bawaan.
  }
  return new ApiError(response.status, code, message)
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, auth = true, signal } = options
  const headers: Record<string, string> = {}

  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (auth) {
    const token = getToken()
    if (token) headers.Authorization = `Bearer ${token}`
  }

  const response = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  })

  if (response.status === 401 && auth) {
    clearToken()
  }
  if (!response.ok) {
    throw await parseError(response)
  }
  if (response.status === 204) {
    return undefined as T
  }
  return (await response.json()) as T
}

/** uploadFile mengirim multipart form ke endpoint unggah gambar produk. */
export async function uploadFile<T>(path: string, file: File): Promise<T> {
  const form = new FormData()
  form.append('file', file)

  const headers: Record<string, string> = {}
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`

  const response = await fetch(`${API_BASE}${path}`, { method: 'POST', headers, body: form })
  if (!response.ok) throw await parseError(response)
  return (await response.json()) as T
}

/** Bentuk respons standar backend: { data: ... }. */
export interface Envelope<T> {
  data: T
}
