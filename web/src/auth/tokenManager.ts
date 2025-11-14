// TokenManager provides in-memory token caching with automatic refresh
// Deduplicates concurrent refresh requests and respects token expiry

type TokenResponse = { access_token: string; expires_at?: number }

const SKEW_MS = 60_000 // 1 minute early refresh

function decodeExpMs(jwt: string): number {
  try {
    const [, payload] = jwt.split('.')
    const data = JSON.parse(atob(payload))
    return typeof data.exp === 'number' ? data.exp * 1000 : 0
  } catch {
    return 0
  }
}

export class TokenManager {
  private token: string | null = null
  private expMs = 0
  private inFlight: Promise<string | null> | null = null

  private needsRefresh(): boolean {
    return !this.token || Date.now() >= this.expMs - SKEW_MS
  }

  private async fetchToken(): Promise<string | null> {
    const res = await fetch('/api/token', { method: 'GET', credentials: 'include' })
    if (!res.ok) return null
    const data = (await res.json()) as TokenResponse
    const token = data.access_token
    // prefer server-provided expires_at; else decode
    this.expMs = data.expires_at ? data.expires_at * 1000 : decodeExpMs(token)
    this.token = token
    return token
  }

  async get(): Promise<string | null> {
    if (!this.needsRefresh()) return this.token
    if (!this.inFlight) this.inFlight = this.fetchToken().finally(() => (this.inFlight = null))
    return this.inFlight
  }

  async forceRefresh(): Promise<string | null> {
    this.token = null
    this.expMs = 0
    return this.get()
  }

  clear() {
    this.token = null
    this.expMs = 0
  }
}
