import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAuth } from '@/features/auth/AuthContext';
import { Sidebar } from './components/sidebar';
import { Header } from './components/header';
import { Overview } from './pages/overview';
import { SuiteRunDetail } from './pages/suite-run-detail';
import { TestRunDetail } from './pages/test-run-detail';
import { TestHealth } from './pages/test-health';
import { TestDetail } from './pages/test-detail';
import { SuiteActivity } from './pages/suite-activity';
import { SuiteDetail } from './pages/suite-detail';
import { Environments } from './pages/environments';
import { Projects } from './pages/projects';
import { ProjectDetail } from './pages/project-detail';
import { ProfileSettings } from './pages/profile-settings';

type Page = 'overview' | 'test-health' | 'suite-activity' | 'environments' | 'projects' | 'profile';
type DetailView =
  | { type: 'suite-run'; id: string }
  | { type: 'test-run'; id: string }
  | { type: 'test'; id: string }
  | { type: 'suite'; id: string }
  | { type: 'project'; id: string }
  | null;

export function ConsoleApp() {
  const { userData, logout } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const [activePage, setActivePage] = useState<Page>('overview');
  const [detailView, setDetailView] = useState<DetailView>(null);
  const [previousView, setPreviousView] = useState<DetailView>(null);

  // Handle initial view from URL query param (e.g., /dashboard?view=suite-activity)
  useEffect(() => {
    const view = searchParams.get('view');
    if (view && ['overview', 'test-health', 'suite-activity', 'environments', 'projects', 'profile'].includes(view)) {
      setActivePage(view as Page);
      // Clear the query param after applying
      searchParams.delete('view');
      setSearchParams(searchParams, { replace: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleNavigate = (page: string) => {
    setActivePage(page as Page);
    setDetailView(null);
    setPreviousView(null);
  };

  const handleSelectSuiteRun = (runId: string) => {
    setPreviousView(detailView);
    setDetailView({ type: 'suite-run', id: runId });
  };

  const handleSelectTestRun = (runId: string) => {
    setPreviousView(detailView);
    setDetailView({ type: 'test-run', id: runId });
  };

  const handleSelectTest = (testId: string) => {
    setPreviousView(null);
    setDetailView({ type: 'test', id: testId });
  };

  const handleSelectSuite = (suiteId: string) => {
    setPreviousView(null);
    setDetailView({ type: 'suite', id: suiteId });
  };

  const handleSelectProject = (projectId: string) => {
    setPreviousView(null);
    setDetailView({ type: 'project', id: projectId });
  };

  const handleBackToList = () => {
    if (previousView) {
      setDetailView(previousView);
      setPreviousView(null);
    } else {
      setDetailView(null);
    }
  };

  const getPageTitle = () => {
    if (detailView?.type === 'suite-run') return 'Suite Run Details';
    if (detailView?.type === 'test-run') return 'Test Run Details';
    if (detailView?.type === 'test') return 'Test Details';
    if (detailView?.type === 'suite') return 'Suite Details';
    if (detailView?.type === 'project') return 'Project Details';

    switch (activePage) {
      case 'overview':
        return 'Overview';
      case 'test-health':
        return 'Test Health';
      case 'suite-activity':
        return 'Suite Activity';
      case 'environments':
        return 'Environments & Access';
      case 'projects':
        return 'Projects';
      case 'profile':
        return 'Profile Settings';
      default:
        return 'Rocketship Cloud';
    }
  };

  // User info for sidebar
  const userName = userData?.user?.name || 'User';
  const orgName = userData?.pending_registration?.org_name || 'Rocketship';

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
          isDetailView={detailView !== null}
          detailViewType={detailView?.type || null}
        />

        <main>
          {detailView?.type === 'suite-run' && (
            <SuiteRunDetail
              suiteRunId={detailView.id}
              onBack={handleBackToList}
              onViewTestRun={handleSelectTestRun}
            />
          )}

          {detailView?.type === 'test-run' && (
            <TestRunDetail
              testRunId={detailView.id}
              onBack={handleBackToList}
            />
          )}

          {detailView?.type === 'test' && (
            <TestDetail
              testId={detailView.id}
              onBack={handleBackToList}
              onViewRun={handleSelectTestRun}
              onViewSuite={handleSelectSuite}
            />
          )}

          {detailView?.type === 'suite' && (
            <SuiteDetail
              suiteId={detailView.id}
              onBack={handleBackToList}
              onViewRun={handleSelectSuiteRun}
              onViewTest={handleSelectTest}
            />
          )}

          {detailView?.type === 'project' && (
            <ProjectDetail
              projectId={detailView.id}
              onBack={handleBackToList}
              onViewSuite={handleSelectSuite}
            />
          )}

          {!detailView && activePage === 'overview' && (
            <Overview onNavigate={handleNavigate} />
          )}

          {!detailView && activePage === 'test-health' && (
            <TestHealth onSelectTest={handleSelectTest} onSelectSuite={handleSelectSuite} />
          )}

          {!detailView && activePage === 'suite-activity' && (
            <SuiteActivity onSelectSuite={handleSelectSuite} />
          )}

          {!detailView && activePage === 'environments' && (
            <Environments onNavigate={handleNavigate} />
          )}

          {!detailView && activePage === 'projects' && (
            <Projects onSelectProject={handleSelectProject} />
          )}

          {!detailView && activePage === 'profile' && (
            <ProfileSettings onLogout={logout} />
          )}
        </main>
      </div>
    </div>
  );
}

export default ConsoleApp;
