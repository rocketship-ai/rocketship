import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import logoImage from '@/assets/no-name-transparent-reverse.png'
import { useAuth } from '@/contexts/AuthContext'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

// PKCE helper functions
function generateRandomString(length: number): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~'
  const array = new Uint8Array(length)
  crypto.getRandomValues(array)
  return Array.from(array, (byte) => chars[byte % chars.length]).join('')
}

async function sha256(plain: string): Promise<ArrayBuffer> {
  const encoder = new TextEncoder()
  const data = encoder.encode(plain)
  return await crypto.subtle.digest('SHA-256', data)
}

function base64UrlEncode(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '')
}

async function generateCodeChallenge(codeVerifier: string): Promise<string> {
  const hashed = await sha256(codeVerifier)
  return base64UrlEncode(hashed)
}

export default function LoginPage() {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const { login, isAuthenticated } = useAuth()
  const navigate = useNavigate()
  const hasProcessedCallback = useRef(false)

  // Redirect to dashboard if already authenticated
  useEffect(() => {
    if (isAuthenticated) {
      navigate('/dashboard', { replace: true })
    }
  }, [isAuthenticated, navigate])

  // Check for OAuth callback on mount
  useEffect(() => {
    const handleCallback = async () => {
      // Prevent double execution in React StrictMode
      if (hasProcessedCallback.current) {
        console.log('[LoginPage] Callback already processed, skipping duplicate execution')
        return
      }
      const params = new URLSearchParams(window.location.search)
      const code = params.get('code')
      const state = params.get('state')
      const errorParam = params.get('error')

      console.log('[LoginPage] Checking for OAuth callback params:', { code: code?.substring(0, 10) + '...', state: state?.substring(0, 10) + '...', error: errorParam })

      if (errorParam) {
        console.log('[LoginPage] OAuth error detected:', errorParam)
        setError(`Authorization failed: ${params.get('error_description') || errorParam}`)
        return
      }

      if (code && state) {
        console.log('[LoginPage] OAuth callback detected, starting token exchange...')
        hasProcessedCallback.current = true
        setIsLoading(true)
        try {
          // Retrieve stored state and code_verifier
          const storedState = sessionStorage.getItem('oauth_state')
          const codeVerifier = sessionStorage.getItem('oauth_code_verifier')
          const redirectUri = sessionStorage.getItem('oauth_redirect_uri')

          // Verify state matches (CSRF protection)
          if (state !== storedState) {
            throw new Error('Invalid state parameter - possible CSRF attack')
          }

          if (!codeVerifier || !redirectUri) {
            throw new Error('Missing OAuth session data')
          }

          // Exchange authorization code for tokens
          const formData = new URLSearchParams()
          formData.append('grant_type', 'authorization_code')
          formData.append('code', code)
          formData.append('redirect_uri', redirectUri)
          formData.append('code_verifier', codeVerifier)
          formData.append('client_id', 'rocketship-cli')

          const response = await fetch(`${API_BASE_URL}/token`, {
            method: 'POST',
            credentials: 'include', // Important: allows cookies to be set
            headers: {
              'Content-Type': 'application/x-www-form-urlencoded',
            },
            body: formData.toString(),
          })

          if (!response.ok) {
            const errorData = await response.json()
            throw new Error(errorData.error_description || 'Token exchange failed')
          }

          const data = await response.json()

          if (data.access_token) {
            // Cookies are now set by the server automatically
            // Just update auth state
            login()

            // Clear OAuth session data
            sessionStorage.removeItem('oauth_state')
            sessionStorage.removeItem('oauth_code_verifier')
            sessionStorage.removeItem('oauth_redirect_uri')

            // Navigate to dashboard
            navigate('/dashboard', { replace: true })
          } else {
            throw new Error('No access token received')
          }
        } catch (err) {
          console.error('OAuth callback failed:', err)
          setError(err instanceof Error ? err.message : 'Authentication failed')
          // Clear OAuth session data on error
          sessionStorage.removeItem('oauth_state')
          sessionStorage.removeItem('oauth_code_verifier')
          sessionStorage.removeItem('oauth_redirect_uri')
        } finally {
          setIsLoading(false)
        }
      }
    }

    handleCallback()
  }, [])

  const handleLogin = async () => {
    console.log('[LoginPage] Login button clicked, starting OAuth flow...')
    setIsLoading(true)
    setError(null)
    try {
      // Generate PKCE parameters
      const codeVerifier = generateRandomString(128)
      const codeChallenge = await generateCodeChallenge(codeVerifier)
      const state = generateRandomString(32)

      console.log('[LoginPage] Generated PKCE parameters, redirecting to GitHub...')

      // Construct redirect URI (where auth broker will redirect back to after GitHub)
      const redirectUri = `${window.location.origin}/login`

      // Store PKCE parameters and state in session storage
      sessionStorage.setItem('oauth_state', state)
      sessionStorage.setItem('oauth_code_verifier', codeVerifier)
      sessionStorage.setItem('oauth_redirect_uri', redirectUri)

      // Build authorization URL
      const params = new URLSearchParams({
        client_id: 'rocketship-cli',
        redirect_uri: redirectUri,
        state: state,
        code_challenge: codeChallenge,
        code_challenge_method: 'S256',
      })

      // Redirect to auth broker's /authorize endpoint
      window.location.href = `${API_BASE_URL}/authorize?${params.toString()}`
    } catch (err) {
      console.error('Login failed:', err)
      setError('Failed to start authentication. Please try again.')
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm">
        <Card className="bg-black border-border">
          <CardContent className="pt-12 pb-12 px-8">
            {/* Logo */}
            <div className="flex justify-center mb-8">
              <img
                src={logoImage}
                alt="Rocketship"
                className="h-16 w-auto"
              />
            </div>

            {/* Title */}
            <div className="text-center mb-8">
              <h1 className="text-2xl font-semibold text-white mb-2">Sign in to Rocketship</h1>
              <p className="text-sm text-gray-400">
                Professional testing platform
              </p>
            </div>

            {/* Error message */}
            {error && (
              <div className="mb-6 p-3 bg-red-950/50 border border-red-900 rounded-md">
                <p className="text-sm text-red-400">{error}</p>
              </div>
            )}

            {/* Sign in button */}
            <Button
              onClick={handleLogin}
              disabled={isLoading}
              className="w-full bg-white text-black hover:bg-gray-100"
              size="lg"
            >
              {isLoading ? (
                <>
                  <svg className="animate-spin -ml-1 mr-2 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  Signing in...
                </>
              ) : (
                <>
                  <svg className="mr-2 h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M10 0C4.477 0 0 4.484 0 10.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0110 4.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.203 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.942.359.31.678.921.678 1.856 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0020 10.017C20 4.484 15.522 0 10 0z" clipRule="evenodd" />
                  </svg>
                  Sign in with GitHub
                </>
              )}
            </Button>

            <p className="text-center mt-8 text-xs text-gray-500">
              By signing in, you agree to our Terms of Service
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
