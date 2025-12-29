import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'
import { tokenManager } from './tokenManager'

interface User {
  id: string
  email: string
  name: string
  username: string
}

interface Organization {
  id: string
  name: string
  slug: string
}

interface UserData {
  user: User
  roles: string[]
  status: 'pending' | 'ready'
  organization?: Organization
  pending_registration?: {
    registration_id: string
    org_name: string
    email: string
    expires_at: string
    resend_available_at: string
    attempts: number
    max_attempts: number
  }
}

interface AuthContextType {
  isAuthenticated: boolean
  isLoading: boolean
  userData: UserData | null
  login: () => Promise<void>
  logout: () => void
  checkAuth: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [userData, setUserData] = useState<UserData | null>(null)

  // Check authentication status by calling the API (with refresh on expiry)
  const checkAuth = async () => {
    console.log('[AuthContext] Checking authentication status...')
    setIsLoading(true)

    const loadProfile = async (forceRefresh: boolean) => {
      const token = forceRefresh ? await tokenManager.forceRefresh() : await tokenManager.get()
      if (!token) {
        console.log('[AuthContext] No token available')
        setUserData(null)
        setIsAuthenticated(false)
        return
      }

      const response = await fetch('/api/users/me', {
        method: 'GET',
        credentials: 'include', // Send cookies with request
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
      })

      console.log('[AuthContext] Check auth response:', response.status, response.ok)

      // If access token expired mid-request, force a refresh and retry once
      if (response.status === 401 && !forceRefresh) {
        console.log('[AuthContext] Access token expired, forcing refresh...')
        return loadProfile(true)
      }

      if (!response.ok) {
        console.log('[AuthContext] User is not authenticated')
        setUserData(null)
        setIsAuthenticated(false)
        return
      }

      const data: UserData = await response.json()
      console.log('[AuthContext] User is authenticated, status:', data.status)
      setUserData(data)
      setIsAuthenticated(true)
    }

    try {
      await loadProfile(false)
    } catch (error) {
      console.error('[AuthContext] Auth check failed:', error)
      setUserData(null)
      setIsAuthenticated(false)
    } finally {
      setIsLoading(false)
    }
  }

  // Check auth status on mount
  useEffect(() => {
    checkAuth()
  }, [])

  // Keep tokens warm to avoid idle expiry kicking users back to login
  useEffect(() => {
    const id = window.setInterval(() => {
      tokenManager.get().catch(() => tokenManager.clear())
    }, 5 * 60 * 1000) // every 5 minutes
    return () => window.clearInterval(id)
  }, [])

  const login = async () => {
    // After successful login, the cookies are already set by the server
    // Fetch user data to get status (pending/ready) for routing
    await checkAuth()
  }

  const logout = async () => {
    console.log('[AuthContext] Logging out...')
    try {
      // Call logout endpoint to clear cookies on server
      const response = await fetch('/logout', {
        method: 'POST',
        credentials: 'include', // Send cookies
      })
      console.log('[AuthContext] Logout response:', response.status, response.ok)
    } catch (error) {
      console.error('[AuthContext] Logout API call failed:', error)
      // Continue with client-side logout even if API call fails
    }

    // Clear client state
    console.log('[AuthContext] Setting isAuthenticated to false')
    setIsAuthenticated(false)
    setUserData(null)

    // Redirect to login page (clear query params)
    console.log('[AuthContext] Redirecting to /login')
    window.location.href = '/login'
  }

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, userData, login, logout, checkAuth }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
