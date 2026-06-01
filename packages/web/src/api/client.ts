import type { ApiResponse, PaginatedResponse } from './types'

const BASE =
  typeof window === 'undefined'
    ? `${import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'}/api/v1`
    : '/api/v1'

function getToken(): string | null {
  if (typeof localStorage === 'undefined') return null
  return localStorage.getItem('access_token')
}

async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(`${BASE}${path}`, { ...options, headers })
  const json: ApiResponse<T> = await res.json()

  if (json.code !== 0 && json.code !== 200) {
    throw new Error(json.msg || 'request failed')
  }
  return json.data as T
}

async function requestPaginated<T>(
  path: string,
  options: RequestInit = {},
): Promise<{ data: T; total: number }> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(`${BASE}${path}`, { ...options, headers })
  const json: PaginatedResponse<T> = await res.json()

  if (json.code !== 0 && json.code !== 200) {
    throw new Error(json.msg || 'request failed')
  }
  return {
    data: json.data as T,
    total: json.total,
  }
}

export function get<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'GET' })
}

export function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
}

export function postPaginated<T>(path: string, body?: unknown): Promise<{ data: T; total: number }> {
  return requestPaginated<T>(path, {
    method: 'POST',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
}

export function postForm<T>(path: string, form: FormData): Promise<T> {
  return request<T>(path, { method: 'POST', body: form, headers: {} })
}
