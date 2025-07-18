import React from 'react'
import { Outlet } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

export default function Layout() {
  const { user, logout } = useAuth()

  return (
    <div>
      <header className="header">
        <div className="container">
          <nav className="nav">
            <div className="logo">Rocketship</div>
            <ul className="nav-links">
              <li><a href="/">Dashboard</a></li>
              <li><a href="/tests">Tests</a></li>
              <li><a href="/teams">Teams</a></li>
            </ul>
            <div>
              {user ? (
                <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                  <span>Welcome, {user.name}</span>
                  <button className="btn btn-secondary" onClick={logout}>
                    Logout
                  </button>
                </div>
              ) : (
                <a href="/login" className="btn btn-primary">Login</a>
              )}
            </div>
          </nav>
        </div>
      </header>
      <main className="container">
        <Outlet />
      </main>
    </div>
  )
}