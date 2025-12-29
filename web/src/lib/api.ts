// API helper for authenticated requests using TokenManager

import { TokenManager } from '@/features/auth/tokenManager'

// Create a singleton instance for use across the app
const tokenManager = new TokenManager()

export class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    message?: string
  ) {
    super(message || `${status} ${statusText}`)
    this.name = 'ApiError'
  }
}

type RequestOptions = Omit<RequestInit, 'headers'> & {
  headers?: Record<string, string>
}

/**
 * Make an authenticated API request.
 * Automatically includes Bearer token from TokenManager and credentials.
 */
export async function apiFetch<T = unknown>(
  url: string,
  options: RequestOptions = {}
): Promise<T> {
  const token = await tokenManager.get()

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...options.headers,
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(url, {
    ...options,
    credentials: 'include',
    headers,
  })

  if (!response.ok) {
    let message: string | undefined
    try {
      const body = await response.json()
      message = body.error || body.message
    } catch {
      // ignore json parse errors
    }
    throw new ApiError(response.status, response.statusText, message)
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return undefined as T
  }

  return response.json()
}

/**
 * GET request helper
 */
export function apiGet<T = unknown>(url: string, options?: RequestOptions): Promise<T> {
  return apiFetch<T>(url, { ...options, method: 'GET' })
}

/**
 * POST request helper
 */
export function apiPost<T = unknown>(
  url: string,
  body?: unknown,
  options?: RequestOptions
): Promise<T> {
  return apiFetch<T>(url, {
    ...options,
    method: 'POST',
    body: body ? JSON.stringify(body) : undefined,
  })
}

/**
 * PUT request helper
 */
export function apiPut<T = unknown>(
  url: string,
  body?: unknown,
  options?: RequestOptions
): Promise<T> {
  return apiFetch<T>(url, {
    ...options,
    method: 'PUT',
    body: body ? JSON.stringify(body) : undefined,
  })
}

/**
 * DELETE request helper
 */
export function apiDelete<T = unknown>(url: string, options?: RequestOptions): Promise<T> {
  return apiFetch<T>(url, { ...options, method: 'DELETE' })
}
