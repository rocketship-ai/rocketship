import React from 'react'
import { useAuth } from '../auth/AuthContext'

export default function DashboardPage() {
  const { user } = useAuth()

  return (
    <div>
      <h1 style={{ marginBottom: '2rem' }}>Dashboard</h1>
      
      <div className="card">
        <h2 style={{ marginBottom: '1rem' }}>Welcome to Rocketship</h2>
        <p style={{ color: '#64748b', marginBottom: '1rem' }}>
          You are successfully authenticated as <strong>{user?.name}</strong> ({user?.email})
        </p>
        {user?.isAdmin && (
          <div className="success">
            <strong>Admin Access:</strong> You have administrative privileges.
          </div>
        )}
      </div>

      <div className="card">
        <h3 style={{ marginBottom: '1rem' }}>Quick Actions</h3>
        <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap' }}>
          <button className="btn btn-primary">Run Tests</button>
          <button className="btn btn-secondary">View Test History</button>
          <button className="btn btn-secondary">Manage Teams</button>
          <button className="btn btn-secondary">API Tokens</button>
        </div>
      </div>

      <div className="card">
        <h3 style={{ marginBottom: '1rem' }}>Recent Test Runs</h3>
        <p style={{ color: '#64748b' }}>
          No test runs found. Start by running your first test!
        </p>
      </div>
    </div>
  )
}