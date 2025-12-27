import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider, useAuth } from '@/features/auth/AuthContext'
import { ProtectedRoute } from '@/features/auth/ProtectedRoute'
import LoginPage from '@/features/auth/LoginPage'
import SignupPage from '@/features/auth/SignupPage'
import OnboardingPage from '@/features/auth/OnboardingPage'
import { ConsoleApp } from '@/features/console/ConsoleApp'

function RootRedirect() {
  const { isAuthenticated, isLoading, userData } = useAuth()

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full"></div>
      </div>
    )
  }

  // Redirect based on authentication and onboarding status
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  // If user needs onboarding, redirect to onboarding page
  if (userData?.status === 'pending') {
    return <Navigate to="/onboarding" replace />
  }

  // User is authenticated and ready, go to dashboard
  return <Navigate to="/dashboard" replace />
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/signup" element={<SignupPage />} />
      <Route path="/onboarding" element={<OnboardingPage />} />
      <Route
        path="/dashboard"
        element={
          <ProtectedRoute>
            <ConsoleApp />
          </ProtectedRoute>
        }
      />
      {/* Redirect /runs to the new console UI (Suite Activity) */}
      <Route
        path="/runs"
        element={
          <ProtectedRoute>
            <Navigate to="/dashboard?view=suite-activity" replace />
          </ProtectedRoute>
        }
      />
      <Route path="/" element={<RootRedirect />} />
    </Routes>
  )
}

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </AuthProvider>
  )
}

export default App
