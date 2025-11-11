import { createContext, useContext, useState, useEffect, ReactNode } from 'react'

interface User {
  id: string
  email: string
  name: string
  username: string
}

interface UserData {
  user: User
  roles: string[]
  status: 'pending' | 'ready'
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

  // Check authentication status by calling the API
  const checkAuth = async () => {
    console.log('[AuthContext] Checking authentication status...')
    try {
      const response = await fetch('/api/users/me', {
        method: 'GET',
        credentials: 'include', // Send cookies with request
        headers: {
          'Content-Type': 'application/json',
        },
      })

      console.log('[AuthContext] Check auth response:', response.status, response.ok)

      if (response.ok) {
        const data: UserData = await response.json()
        console.log('[AuthContext] User is authenticated, status:', data.status)
        setUserData(data)
        setIsAuthenticated(true)
      } else {
        console.log('[AuthContext] User is not authenticated')
        setUserData(null)
        setIsAuthenticated(false)
      }
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
