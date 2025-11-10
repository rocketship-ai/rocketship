import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import logoImage from '@/assets/no-name-transparent-reverse.png'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

export default function LoginPage() {
  const [deviceCode, setDeviceCode] = useState<string | null>(null)
  const [userCode, setUserCode] = useState<string | null>(null)
  const [verificationUri, setVerificationUri] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleLogin = async () => {
    setIsLoading(true)
    setError(null)
    try {
      // Call auth broker device flow endpoint (OAuth2 standard)
      const formData = new URLSearchParams()
      formData.append('client_id', 'rocketship-cli')
      formData.append('scope', 'read:org read:user user:email')

      const response = await fetch(`${API_BASE_URL}/device/code`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: formData.toString(),
      })

      if (!response.ok) {
        throw new Error('Failed to initiate device flow')
      }

      const data = await response.json()
      setDeviceCode(data.device_code)
      setUserCode(data.user_code)
      setVerificationUri(data.verification_uri)
    } catch (err) {
      console.error('Login failed:', err)
      setError('Failed to start authentication. Please try again.')
    } finally {
      setIsLoading(false)
    }
  }

  // Poll for authorization status
  useEffect(() => {
    if (!deviceCode) return

    const pollInterval = setInterval(async () => {
      try {
        // Use OAuth2 token endpoint with device_code grant
        const formData = new URLSearchParams()
        formData.append('grant_type', 'urn:ietf:params:oauth:grant-type:device_code')
        formData.append('device_code', deviceCode)
        formData.append('client_id', 'rocketship-cli')

        const response = await fetch(`${API_BASE_URL}/token`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
          },
          body: formData.toString(),
        })

        if (response.ok) {
          const data = await response.json()
          if (data.access_token) {
            // Store token and redirect to dashboard
            localStorage.setItem('access_token', data.access_token)
            window.location.href = '/dashboard'
          }
        } else if (response.status === 400) {
          // Expected responses: authorization_pending, slow_down, expired_token
          const error = await response.json()
          if (error.error === 'expired_token') {
            clearInterval(pollInterval)
            setError('Device code expired. Please try again.')
          }
        }
      } catch (err) {
        console.error('Polling failed:', err)
      }
    }, 5000)

    return () => clearInterval(pollInterval)
  }, [deviceCode])

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm">
        {!deviceCode ? (
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
                    Loading...
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
        ) : (
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
                <h1 className="text-2xl font-semibold text-white mb-2">Device verification</h1>
                <p className="text-sm text-gray-400">
                  Complete sign in on GitHub
                </p>
              </div>

              {/* Instructions */}
              <div className="space-y-6">
                <div className="space-y-3">
                  <p className="text-sm text-gray-400">
                    1. Visit this URL:
                  </p>
                  <a
                    href={verificationUri || '#'}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="block p-3 bg-gray-900 hover:bg-gray-800 rounded-md text-sm font-mono text-center transition-colors border border-gray-800 text-white"
                  >
                    {verificationUri}
                  </a>
                </div>

                <div className="space-y-3">
                  <p className="text-sm text-gray-400">
                    2. Enter this code:
                  </p>
                  <div className="p-4 bg-white rounded-md">
                    <div className="text-3xl font-mono font-bold text-center tracking-widest text-black">
                      {userCode}
                    </div>
                  </div>
                </div>
              </div>

              <div className="flex items-center justify-center gap-2 text-sm text-gray-400 pt-8">
                <svg className="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <span>Waiting for authorization...</span>
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
