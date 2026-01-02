import {
  createRouter,
  createRootRoute,
  createRoute,
  Outlet,
  Navigate,
  useRouter,
} from '@tanstack/react-router'

import { AuthProvider, useAuth } from '@/features/auth/AuthContext'
import { ProtectedRoute } from '@/features/auth/ProtectedRoute'
import LoginPage from '@/features/auth/LoginPage'
import SignupPage from '@/features/auth/SignupPage'
import OnboardingPage from '@/features/auth/OnboardingPage'
import { ConsoleLayout } from '@/features/console/ConsoleLayout'
import { NotFoundPage } from './NotFoundPage'

// Page components
import { Overview } from '@/features/console/pages/overview'
import { TestHealth } from '@/features/console/pages/test-health'
import { SuiteActivity } from '@/features/console/pages/suite-activity'
import { SuiteDetail } from '@/features/console/pages/suite-detail'
import { SuiteRunDetail } from '@/features/console/pages/suite-run-detail'
import { TestDetail } from '@/features/console/pages/test-detail'
import { TestRunDetail } from '@/features/console/pages/test-run-detail'
import { Projects } from '@/features/console/pages/projects'
import { ProjectDetail } from '@/features/console/pages/project-detail'
import { Environments } from '@/features/console/pages/environments'
import { ProfileSettings } from '@/features/console/pages/profile-settings'

// Root redirect component
function RootRedirect() {
  const { isAuthenticated, isLoading, userData } = useAuth()

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full"></div>
      </div>
    )
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  if (userData?.status === 'pending') {
    return <Navigate to="/onboarding" replace />
  }

  return <Navigate to="/overview" replace />
}

// Root layout with AuthProvider
function RootLayout() {
  return (
    <AuthProvider>
      <Outlet />
    </AuthProvider>
  )
}

// Create root route
const rootRoute = createRootRoute({
  component: RootLayout,
  notFoundComponent: NotFoundPage,
})

// Index route - redirects based on auth state
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: RootRedirect,
})

// Auth routes
const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
})

const signupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/signup',
  component: SignupPage,
})

const onboardingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/onboarding',
  component: OnboardingPage,
})

// Pathless layout route for protected console pages
// This wraps all console routes with ProtectedRoute + ConsoleLayout
function ProtectedConsoleLayout() {
  return (
    <ProtectedRoute>
      <ConsoleLayout />
    </ProtectedRoute>
  )
}

const consoleLayoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: '_console',
  component: ProtectedConsoleLayout,
})

// Overview
const overviewRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/overview',
  component: function OverviewRoute() {
    const navigate = overviewRoute.useNavigate()
    return (
      <Overview
        onNavigate={(page) => {
          switch (page) {
            case 'overview':
              navigate({ to: '/overview' })
              break
            case 'test-health':
              navigate({ to: '/test-health' })
              break
            case 'suite-activity':
              navigate({ to: '/suite-activity' })
              break
            case 'projects':
              navigate({ to: '/projects' })
              break
            case 'environments':
              navigate({ to: '/environments' })
              break
            case 'profile':
              navigate({ to: '/profile' })
              break
          }
        }}
      />
    )
  },
})

// Test Health
const testHealthRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/test-health',
  component: function TestHealthRoute() {
    const navigate = testHealthRoute.useNavigate()
    return (
      <TestHealth
        onSelectTest={(testId) => navigate({ to: '/tests/$testId', params: { testId } })}
        onSelectSuite={(suiteId) => navigate({ to: '/suites/$suiteId', params: { suiteId } })}
      />
    )
  },
})

// Suite Activity
const suiteActivityRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/suite-activity',
  component: function SuiteActivityRoute() {
    const navigate = suiteActivityRoute.useNavigate()
    return (
      <SuiteActivity
        onSelectSuite={(suiteId) => navigate({ to: '/suites/$suiteId', params: { suiteId } })}
      />
    )
  },
})

// Suite Detail
const suiteDetailRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/suites/$suiteId',
  component: function SuiteDetailRoute() {
    const { suiteId } = suiteDetailRoute.useParams()
    const navigate = suiteDetailRoute.useNavigate()
    return (
      <SuiteDetail
        suiteId={suiteId}
        onBack={() => navigate({ to: '/suite-activity' })}
        onViewRun={(runId) =>
          navigate({ to: '/suites/$suiteId/runs/$suiteRunId', params: { suiteId, suiteRunId: runId } })
        }
        onViewTest={(testId) => navigate({ to: '/tests/$testId', params: { testId } })}
      />
    )
  },
})

// Suite Run Detail
const suiteRunDetailRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/suites/$suiteId/runs/$suiteRunId',
  component: function SuiteRunDetailRoute() {
    const { suiteId, suiteRunId } = suiteRunDetailRoute.useParams()
    const navigate = suiteRunDetailRoute.useNavigate()
    return (
      <SuiteRunDetail
        suiteRunId={suiteRunId}
        onBack={() => navigate({ to: '/suites/$suiteId', params: { suiteId } })}
        onViewTestRun={(testRunId) => navigate({ to: '/test-runs/$testRunId', params: { testRunId } })}
      />
    )
  },
})

// Test Detail
const testDetailRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/tests/$testId',
  component: function TestDetailRoute() {
    const { testId } = testDetailRoute.useParams()
    const navigate = testDetailRoute.useNavigate()
    return (
      <TestDetail
        testId={testId}
        onBack={() => navigate({ to: '/test-health' })}
        onViewRun={(runId) => navigate({ to: '/test-runs/$testRunId', params: { testRunId: runId } })}
        onViewSuite={(suiteId) => navigate({ to: '/suites/$suiteId', params: { suiteId } })}
      />
    )
  },
})

// Test Run Detail
const testRunDetailRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/test-runs/$testRunId',
  component: function TestRunDetailRoute() {
    const { testRunId } = testRunDetailRoute.useParams()
    const router = useRouter()
    return (
      <TestRunDetail
        testRunId={testRunId}
        onBack={() => router.history.back()}
      />
    )
  },
})

// Projects
const projectsRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/projects',
  component: function ProjectsRoute() {
    const navigate = projectsRoute.useNavigate()
    return (
      <Projects
        onSelectProject={(projectId) =>
          navigate({ to: '/projects/$projectId', params: { projectId } })
        }
      />
    )
  },
})

// Project Detail
const projectDetailRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/projects/$projectId',
  component: function ProjectDetailRoute() {
    const { projectId } = projectDetailRoute.useParams()
    const navigate = projectDetailRoute.useNavigate()
    return (
      <ProjectDetail
        projectId={projectId}
        onBack={() => navigate({ to: '/projects' })}
        onViewSuite={(suiteId) => navigate({ to: '/suites/$suiteId', params: { suiteId } })}
      />
    )
  },
})

// Environments
const environmentsRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/environments',
  validateSearch: (search: Record<string, unknown>): { project?: string } => ({
    project: typeof search.project === 'string' ? search.project : undefined,
  }),
  component: function EnvironmentsRoute() {
    const navigate = environmentsRoute.useNavigate()
    const { project: projectId } = environmentsRoute.useSearch()
    return (
      <Environments
        selectedProjectId={projectId}
        onProjectSelect={(id) => {
          navigate({ search: { project: id } })
        }}
        onNavigate={(page, params) => {
          switch (page) {
            case 'overview':
              navigate({ to: '/overview' })
              break
            case 'test-health':
              navigate({ to: '/test-health' })
              break
            case 'suite-activity':
              navigate({ to: '/suite-activity', search: params?.env ? { env: params.env } : undefined })
              break
            case 'projects':
              navigate({ to: '/projects' })
              break
            case 'environments':
              navigate({ to: '/environments' })
              break
            case 'profile':
              navigate({ to: '/profile' })
              break
          }
        }}
      />
    )
  },
})

// Profile Settings
const profileRoute = createRoute({
  getParentRoute: () => consoleLayoutRoute,
  path: '/profile',
  component: function ProfileRoute() {
    const { logout } = useAuth()
    return <ProfileSettings onLogout={logout} />
  },
})

// Build the route tree
const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  signupRoute,
  onboardingRoute,
  consoleLayoutRoute.addChildren([
    overviewRoute,
    testHealthRoute,
    suiteActivityRoute,
    suiteDetailRoute,
    suiteRunDetailRoute,
    testDetailRoute,
    testRunDetailRoute,
    projectsRoute,
    projectDetailRoute,
    environmentsRoute,
    profileRoute,
  ]),
])

// Create and export the router
export const router = createRouter({ routeTree })

// Type declaration for router
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
