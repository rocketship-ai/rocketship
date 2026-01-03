import { Navigate } from '@tanstack/react-router'
import { useAuth } from './AuthContext'

export function AuthOnlyRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full"></div>
      </div>
    )
  }

  if (!isAuthenticated) {
    const returnTo = window.location.pathname + window.location.search
    sessionStorage.setItem('rocketship.returnTo', returnTo)
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}
