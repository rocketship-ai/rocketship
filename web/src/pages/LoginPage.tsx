import React from 'react'
import { useAuth } from '../auth/AuthContext'
import { Navigate } from 'react-router-dom'

export default function LoginPage() {
  const { user, login, isLoading } = useAuth()

  if (isLoading) {
    return <div className="loading">Loading...</div>
  }

  if (user) {
    return <Navigate to="/" replace />
  }

  return (
    <div style={{ 
      display: 'flex', 
      justifyContent: 'center', 
      alignItems: 'center', 
      minHeight: '100vh',
      background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)'
    }}>
      <div className="card" style={{ 
        width: '400px', 
        textAlign: 'center',
        background: 'white',
        borderRadius: '12px',
        boxShadow: '0 10px 25px rgba(0, 0, 0, 0.1)'
      }}>
        <div style={{ marginBottom: '2rem' }}>
          <h1 style={{ 
            fontSize: '2rem', 
            fontWeight: '700', 
            color: '#3b82f6', 
            marginBottom: '0.5rem' 
          }}>
            Rocketship
          </h1>
          <p style={{ color: '#64748b' }}>
            Testing framework for modern applications
          </p>
        </div>
        
        <button 
          className="btn btn-primary" 
          onClick={() => login('/')}
          style={{ 
            width: '100%', 
            padding: '0.75rem 1.5rem',
            fontSize: '1rem',
            borderRadius: '8px'
          }}
        >
          Sign in with OIDC
        </button>
        
        <p style={{ 
          marginTop: '1.5rem', 
          fontSize: '0.875rem', 
          color: '#64748b' 
        }}>
          Secure authentication via OpenID Connect
        </p>
      </div>
    </div>
  )
}