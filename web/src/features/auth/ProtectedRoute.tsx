import { Navigate } from '@tanstack/react-router'
import { useAuth } from './AuthContext'
import { usePendingProjectInvites } from '@/features/console/hooks/use-console-queries'

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading, userData } = useAuth()
  const isPending = userData?.status === 'pending'

  // Only check for pending invites if user is pending
  const { data: pendingInvites, isLoading: invitesLoading } = usePendingProjectInvites({
    enabled: isAuthenticated && isPending,
  })

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

  // If user is pending, check for pending invites before redirecting
  if (isPending) {
    // Wait for invites to load if we're checking
    if (invitesLoading) {
      return (
        <div className="min-h-screen flex items-center justify-center bg-background">
          <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full"></div>
        </div>
      )
    }

    // If user has pending invites, redirect to accept invite page
    if (pendingInvites && pendingInvites.length > 0) {
      return <Navigate to="/invites/accept" replace />
    }

    // No pending invites, redirect to onboarding
    return <Navigate to="/onboarding" replace />
  }

  // User is authenticated and ready, render the protected content
  return <>{children}</>
}
