import { Navigate } from '@tanstack/react-router'
import { useAuth } from './AuthContext'

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading, userData } = useAuth()

  if (isLoading) {
    // Show loading spinner while checking auth status
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full"></div>
      </div>
    )
  }

  if (!isAuthenticated) {
    // Redirect to login if not authenticated
    return <Navigate to="/login" replace />
  }

  // If user needs onboarding, redirect to onboarding page
  if (userData?.status === 'pending') {
    return <Navigate to="/onboarding" replace />
  }

  // User is authenticated and ready, render the protected content
  return <>{children}</>
}
