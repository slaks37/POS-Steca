// Klien REST tipis untuk backend Steca POS.

import { Capacitor } from '@capacitor/core'

const TOKEN_KEY = 'pos_steca_token'

/** isNative menandai aplikasi sedang berjalan di dalam WebView Android. */
export const isNative = Capacitor.isNativePlatform()

/**
 * resolveApiBase menentukan alamat backend.
 *
 * Di web, nilai relatif "/api/v1" sudah cukup karena dev server Vite
 * mem-proxy-nya (lihat vite.config.ts) dan hosting produksi berada satu
 * domain dengan backend.
 *
 * Di Android, WebView berjalan pada origin https://localhost sehingga URL
 * relatif akan menunjuk ke perangkat itu sendiri dan selalu gagal. Build
 * Android karena itu wajib menyetel VITE_API_BASE_URL ke URL penuh backend.
 */
function resolveApiBase(): string {
  const configured = import.meta.env.VITE_API_BASE_URL?.trim()
  if (configured) return configured.replace(/\/$/, '')

  if (isNative) {
    // Jangan diam-diam gagal: beri pesan yang jelas di logcat saat APK dibuat
    // tanpa VITE_API_BASE_URL.
    console.error(
      'VITE_API_BASE_URL belum disetel. Build Android harus memakai URL penuh backend, ' +
        'misal https://api.tokoanda.com/api/v1. Jalankan `npm run build:android` setelah ' +
        'menyalin .env.android.example menjadi .env.android.',
    )
  }
  return '/api/v1'
}

export const API_BASE = resolveApiBase()

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
