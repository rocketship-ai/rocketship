import React from 'react'
import { Routes, Route } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import CallbackPage from './pages/CallbackPage'
import DashboardPage from './pages/DashboardPage'
import { ProtectedRoute } from './auth/ProtectedRoute'

function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/callback" element={<CallbackPage />} />
        <Route path="/" element={<Layout />}>
          <Route index element={
            <ProtectedRoute>
              <DashboardPage />
            </ProtectedRoute>
          } />
        </Route>
      </Routes>
    </AuthProvider>
  )
}

export default App