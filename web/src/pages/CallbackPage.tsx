import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Cookies from 'js-cookie'

export default function CallbackPage() {
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const handleCallback = async () => {
      try {
        const urlParams = new URLSearchParams(window.location.search)
        const code = urlParams.get('code')
        const state = urlParams.get('state')
        const errorParam = urlParams.get('error')
        
        if (errorParam) {
          setError(`Authentication failed: ${errorParam}`)
          return
        }

        if (!code) {
          setError('No authorization code received')
          return
        }

        // Verify state parameter
        const storedState = sessionStorage.getItem('oauth_state')
        if (!storedState || storedState !== state) {
          setError('Invalid state parameter')
          return
        }

        // Get code verifier
        const codeVerifier = sessionStorage.getItem('code_verifier')
        if (!codeVerifier) {
          setError('Code verifier not found')
          return
        }

        // Exchange code for tokens
        const response = await exchangeCodeForTokens(code, codeVerifier)
        
        if (!response.ok) {
          const errorData = await response.json()
          setError(`Token exchange failed: ${errorData.error || 'Unknown error'}`)
          return
        }

        const tokens = await response.json()
        
        // Store access token in secure cookie
        Cookies.set('rocketship_token', tokens.access_token, {
          secure: true,
          sameSite: 'strict',
          expires: tokens.expires_in ? new Date(Date.now() + tokens.expires_in * 1000) : 7
        })

        // Clean up session storage
        sessionStorage.removeItem('oauth_state')
        sessionStorage.removeItem('code_verifier')
        
        // Redirect to original destination or dashboard
        const redirectUri = sessionStorage.getItem('oauth_redirect') || '/'
        sessionStorage.removeItem('oauth_redirect')
        
        navigate(redirectUri, { replace: true })
      } catch (err) {
        setError(`Authentication error: ${err instanceof Error ? err.message : 'Unknown error'}`)
      }
    }

    handleCallback()
  }, [navigate])

  if (error) {
    return (
      <div style={{ 
        display: 'flex', 
        justifyContent: 'center', 
        alignItems: 'center', 
        minHeight: '100vh' 
      }}>
        <div className="card" style={{ width: '400px', textAlign: 'center' }}>
          <h2 style={{ color: '#dc2626', marginBottom: '1rem' }}>
            Authentication Failed
          </h2>
          <div className="error" style={{ textAlign: 'left' }}>
            {error}
          </div>
          <button 
            className="btn btn-primary" 
            onClick={() => navigate('/login')}
            style={{ marginTop: '1rem' }}
          >
            Try Again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div style={{ 
      display: 'flex', 
      justifyContent: 'center', 
      alignItems: 'center', 
      minHeight: '100vh' 
    }}>
      <div className="card" style={{ width: '400px', textAlign: 'center' }}>
        <h2 style={{ color: '#3b82f6', marginBottom: '1rem' }}>
          Completing Authentication...
        </h2>
        <div className="loading">
          Processing your login...
        </div>
      </div>
    </div>
  )
}

async function exchangeCodeForTokens(code: string, codeVerifier: string): Promise<Response> {
  const issuer = process.env.REACT_APP_OIDC_ISSUER || 'https://your-keycloak/realms/rocketship'
  const clientId = process.env.REACT_APP_OIDC_CLIENT_ID || 'rocketship'
  const redirectUri = `${window.location.origin}/callback`
  
  const tokenUrl = `${issuer}/protocol/openid-connect/token`
  
  const body = new URLSearchParams({
    grant_type: 'authorization_code',
    client_id: clientId,
    code: code,
    redirect_uri: redirectUri,
    code_verifier: codeVerifier
  })

  return fetch(tokenUrl, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded'
    },
    body: body
  })
}