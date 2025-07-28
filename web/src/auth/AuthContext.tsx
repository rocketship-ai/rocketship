import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import Cookies from 'js-cookie'

interface User {
  id: string
  email: string
  name: string
  isAdmin: boolean
}

interface AuthContextType {
  user: User | null
  token: string | null
  login: (redirectUri?: string) => void
  logout: () => void
  isLoading: boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}

interface AuthProviderProps {
  children: ReactNode
}

export function AuthProvider({ children }: AuthProviderProps) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    // Check for existing token in cookies
    const storedToken = Cookies.get('rocketship_token')
    if (storedToken) {
      setToken(storedToken)
      // In a real implementation, validate token and fetch user info
      // For now, just set a mock user
      setUser({
        id: '1',
        email: 'user@example.com',
        name: 'Demo User',
        isAdmin: false
      })
    }
    setIsLoading(false)
  }, [])

  const login = (redirectUri?: string) => {
    // Generate state parameter
    const state = Math.random().toString(36).substring(2, 15)
    
    // Store state and redirect URI
    sessionStorage.setItem('oauth_state', state)
    if (redirectUri) {
      sessionStorage.setItem('oauth_redirect', redirectUri)
    }

    // Get OIDC configuration from environment or API
    const issuer = process.env.REACT_APP_OIDC_ISSUER || 'https://your-keycloak/realms/rocketship'
    const clientId = process.env.REACT_APP_OIDC_CLIENT_ID || 'rocketship'
    const redirectUrl = `${window.location.origin}/callback`
    
    // Generate PKCE parameters
    const codeVerifier = generateCodeVerifier()
    const codeChallenge = generateCodeChallenge(codeVerifier)
    
    // Store code verifier
    sessionStorage.setItem('code_verifier', codeVerifier)
    
    // Build authorization URL
    const authUrl = new URL(`${issuer}/protocol/openid-connect/auth`)
    authUrl.searchParams.set('response_type', 'code')
    authUrl.searchParams.set('client_id', clientId)
    authUrl.searchParams.set('redirect_uri', redirectUrl)
    authUrl.searchParams.set('scope', 'openid profile email groups')
    authUrl.searchParams.set('state', state)
    authUrl.searchParams.set('code_challenge', codeChallenge)
    authUrl.searchParams.set('code_challenge_method', 'S256')
    
    // Redirect to authorization server
    window.location.href = authUrl.toString()
  }

  const logout = () => {
    // Remove token from cookies
    Cookies.remove('rocketship_token')
    setToken(null)
    setUser(null)
    
    // Redirect to login
    window.location.href = '/login'
  }

  const value = {
    user,
    token,
    login,
    logout,
    isLoading
  }

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  )
}

// PKCE helper functions
function generateCodeVerifier(): string {
  const array = new Uint8Array(32)
  crypto.getRandomValues(array)
  return base64URLEncode(array)
}

function generateCodeChallenge(verifier: string): string {
  const encoder = new TextEncoder()
  const data = encoder.encode(verifier)
  return crypto.subtle.digest('SHA-256', data).then(hash => {
    return base64URLEncode(new Uint8Array(hash))
  }).then(challenge => challenge).catch(() => verifier) // Fallback for older browsers
}

function base64URLEncode(array: Uint8Array): string {
  return btoa(String.fromCharCode(...array))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '')
}