import { Outlet, useLocation, useNavigate } from '@tanstack/react-router'
import { useAuth } from '@/features/auth/AuthContext'
import { Sidebar } from './components/sidebar'
import { Header } from './components/header'

type ActivePage = 'overview' | 'test-health' | 'suite-activity' | 'environments' | 'projects' | 'profile'
type DetailViewType = 'suite' | 'test' | 'suite-run' | 'test-run' | 'project' | null

export function ConsoleLayout() {
  const { userData } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()
  const pathname = location.pathname

  // Determine active page and detail view from pathname
  const getActivePageAndDetail = (): { activePage: ActivePage; isDetailView: boolean; detailViewType: DetailViewType } => {
    // Detail views
    if (pathname.match(/^\/suites\/[^/]+\/runs\/[^/]+$/)) {
      return { activePage: 'suite-activity', isDetailView: true, detailViewType: 'suite-run' }
    }
    if (pathname.match(/^\/suites\/[^/]+$/)) {
      return { activePage: 'suite-activity', isDetailView: true, detailViewType: 'suite' }
    }
    if (pathname.match(/^\/test-runs\/[^/]+$/)) {
      // Test run can come from suite-run or test-detail, but default to suite-activity
      return { activePage: 'suite-activity', isDetailView: true, detailViewType: 'test-run' }
    }
    if (pathname.match(/^\/tests\/[^/]+$/)) {
      return { activePage: 'test-health', isDetailView: true, detailViewType: 'test' }
    }
    if (pathname.match(/^\/projects\/[^/]+$/)) {
      return { activePage: 'projects', isDetailView: true, detailViewType: 'project' }
    }

    // List views
    if (pathname === '/test-health') {
      return { activePage: 'test-health', isDetailView: false, detailViewType: null }
    }
    if (pathname === '/suite-activity') {
      return { activePage: 'suite-activity', isDetailView: false, detailViewType: null }
    }
    if (pathname === '/projects') {
      return { activePage: 'projects', isDetailView: false, detailViewType: null }
    }
    if (pathname === '/environments') {
      return { activePage: 'environments', isDetailView: false, detailViewType: null }
    }
    if (pathname === '/profile') {
      return { activePage: 'profile', isDetailView: false, detailViewType: null }
    }

    // Default to overview
    return { activePage: 'overview', isDetailView: false, detailViewType: null }
  }

  const { activePage, isDetailView, detailViewType } = getActivePageAndDetail()

  // Extract suiteId from pathname for suite detail view
  const getSuiteId = (): string | undefined => {
    const suiteMatch = pathname.match(/^\/suites\/([^/]+)/)
    return suiteMatch ? suiteMatch[1] : undefined
  }
  const suiteId = getSuiteId()

  // Get page title
  const getPageTitle = (): string => {
    if (isDetailView) {
      switch (detailViewType) {
        case 'suite-run':
          return 'Suite Run Details'
        case 'suite':
          return 'Suite Details'
        case 'test-run':
          return 'Test Run Details'
        case 'test':
          return 'Test Details'
        case 'project':
          return 'Project Details'
      }
    }

    switch (activePage) {
      case 'overview':
        return 'Overview'
      case 'test-health':
        return 'Test Health'
      case 'suite-activity':
        return 'Suite Activity'
      case 'environments':
        return 'Environments & Access'
      case 'projects':
        return 'Projects'
      case 'profile':
        return 'Profile Settings'
      default:
        return 'Rocketship Cloud'
    }
  }

  // Handle sidebar navigation
  const handleNavigate = (page: string) => {
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
  }

  // User info for sidebar (no fallback to username - name is required from onboarding)
  const userName = userData?.user?.name || '—'
  // Prefer actual organization name, fall back to pending registration name, then default
  const orgName = userData?.organization?.name || userData?.pending_registration?.org_name || 'Rocketship'

  return (
    <div className="min-h-screen bg-[#fafafa]">
      <Sidebar
        activePage={activePage}
        onNavigate={handleNavigate}
        userName={userName}
        orgName={orgName}
      />

      <div className="ml-16">
        <Header
          title={getPageTitle()}
          activePage={activePage}
          isDetailView={isDetailView}
          detailViewType={detailViewType}
          suiteId={suiteId}
        />

        <main>
          <Outlet />
        </main>
      </div>
    </div>
  )
}
