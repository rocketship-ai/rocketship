import { Link, useRouter } from '@tanstack/react-router'
import { useAuth } from '@/features/auth/AuthContext'

export function NotFoundPage() {
  const { isAuthenticated, isLoading, userData } = useAuth()
  const router = useRouter()

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full"></div>
      </div>
    )
  }

  // Determine primary CTA based on auth state
  const getPrimaryCTA = () => {
    if (!isAuthenticated) {
      return { to: '/login', label: 'Go to login' }
    }
    if (userData?.status === 'pending') {
      return { to: '/onboarding', label: 'Continue onboarding' }
    }
    return { to: '/overview', label: 'Go to overview' }
  }

  const primaryCTA = getPrimaryCTA()

  return (
    <div className="min-h-screen flex items-center justify-center bg-[#fafafa] p-4">
      <div className="text-center max-w-md">
        <h1 className="text-6xl font-bold text-gray-900 mb-4">404</h1>
        <h2 className="text-xl font-semibold text-gray-700 mb-2">Page not found</h2>
        <p className="text-gray-500 mb-8">
          The page you're looking for doesn't exist or has been moved.
        </p>

        <div className="flex flex-col sm:flex-row gap-3 justify-center">
          <Link
            to={primaryCTA.to}
            className="px-6 py-2.5 bg-black text-white rounded-md hover:bg-gray-800 transition-colors font-medium"
          >
            {primaryCTA.label}
          </Link>

          <button
            onClick={() => router.history.back()}
            className="px-6 py-2.5 bg-white text-gray-700 border border-gray-300 rounded-md hover:bg-gray-50 transition-colors font-medium"
          >
            Go back
          </button>
        </div>
      </div>
    </div>
  )
}
