import { createContext, useContext, useState, useEffect, ReactNode } from 'react'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

interface AuthContextType {
  isAuthenticated: boolean
  isLoading: boolean
  login: () => void
  logout: () => void
  checkAuth: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [isLoading, setIsLoading] = useState(true)

  // Check authentication status by calling the API
  const checkAuth = async () => {
    console.log('[AuthContext] Checking authentication status...')
    try {
      const response = await fetch(`${API_BASE_URL}/api/users/me`, {
        method: 'GET',
        credentials: 'include', // Send cookies with request
        headers: {
          'Content-Type': 'application/json',
        },
      })

      console.log('[AuthContext] Check auth response:', response.status, response.ok)

      if (response.ok) {
        console.log('[AuthContext] User is authenticated')
        setIsAuthenticated(true)
      } else {
        console.log('[AuthContext] User is not authenticated')
        setIsAuthenticated(false)
      }
    } catch (error) {
      console.error('[AuthContext] Auth check failed:', error)
      setIsAuthenticated(false)
    } finally {
      setIsLoading(false)
    }
  }

  // Check auth status on mount
  useEffect(() => {
    checkAuth()
  }, [])

  const login = () => {
    // After successful login, the cookies are already set by the server
    // Just update the auth state
    setIsAuthenticated(true)
  }

  const logout = async () => {
    console.log('[AuthContext] Logging out...')
    try {
      // Call logout endpoint to clear cookies on server
      const response = await fetch(`${API_BASE_URL}/logout`, {
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
    <AuthContext.Provider value={{ isAuthenticated, isLoading, login, logout, checkAuth }}>
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
