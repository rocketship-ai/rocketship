import { CheckCircle2, Circle, AlertCircle, X, Loader2, ExternalLink } from 'lucide-react';
import { Line, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ComposedChart } from 'recharts';
import { useState, useEffect, useRef, useMemo } from 'react';
import { toast } from 'sonner';
import {
  useOverviewSetup,
  useOverviewMetrics,
  useGithubAppRepos,
  useConnectGithubRepo,
  useSyncGithubApp,
  type SetupData,
  type GitHubRepo,
  type ConnectRepoResult,
} from '../hooks/use-console-queries';
import { useConsoleProjectFilter, useConsoleEnvironmentFilter } from '../hooks/use-console-filters';
import { formatDuration } from '../lib/format';
import { Button } from '../components/ui';

interface OverviewProps {
  onNavigate: (page: string) => void;
}

export function Overview({ onNavigate }: OverviewProps) {
  // Setup data from API via React Query
  const { data: setupData, isLoading: setupLoading } = useOverviewSetup();

  // Project filter for scoped metrics
  const { selectedProjectIds } = useConsoleProjectFilter();

  // Environment filter - only applies when exactly one project is selected
  const effectiveProjectId = selectedProjectIds.length === 1 ? selectedProjectIds[0] : '';
  const { selectedEnvironmentId } = useConsoleEnvironmentFilter(effectiveProjectId);
  const effectiveEnvironmentId = selectedProjectIds.length === 1 ? selectedEnvironmentId : undefined;

  // Stabilize project IDs ordering for query key consistency
  const sortedProjectIds = useMemo(() => [...selectedProjectIds].sort(), [selectedProjectIds]);

  // Overview metrics from API (respects project and environment filters)
  const { data: metricsData } = useOverviewMetrics({
    projectIds: sortedProjectIds.length > 0 ? sortedProjectIds : undefined,
    environmentId: effectiveEnvironmentId,
    days: 7,
  });

  // GitHub repos query (only fetched when modal opens)
  const {
    data: repos,
    isLoading: reposLoading,
    error: reposError,
    refetch: fetchRepos,
    isFetched: reposFetched,
  } = useGithubAppRepos();

  // Mutations
  const connectMutation = useConnectGithubRepo();
  const syncMutation = useSyncGithubApp();

  // Connect modal state
  const [connectModalOpen, setConnectModalOpen] = useState(false);
  const [selectedRepo, setSelectedRepo] = useState<GitHubRepo | null>(null);
  const [connectResult, setConnectResult] = useState<ConnectRepoResult | null>(null);

  // Load repos when modal opens
  useEffect(() => {
    if (connectModalOpen && !reposFetched && !reposLoading) {
      fetchRepos();
    }
  }, [connectModalOpen, reposFetched, reposLoading, fetchRepos]);

  // Open connect modal
  const openConnectModal = () => {
    setConnectModalOpen(true);
    setSelectedRepo(null);
    setConnectResult(null);
  };

  // Close connect modal
  const closeConnectModal = () => {
    setConnectModalOpen(false);
    setSelectedRepo(null);
    setConnectResult(null);
  };

  // Handle connecting a repository
  const handleConnect = async () => {
    if (!selectedRepo) return;

    try {
      const result = await connectMutation.mutateAsync(selectedRepo.full_name);
      setConnectResult(result);
    } catch {
      // Error is available via connectMutation.error
    }
  };

  // Handle syncing GitHub App
  const handleSync = async () => {
    try {
      const result = await syncMutation.mutateAsync();
      if (result.synced) {
        toast.success('GitHub App installation synced!');
      } else {
        toast.info(result.message || 'No installation found to sync');
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to sync');
    }
  };

  // Helper to get step by ID
  const getStep = (id: string) => setupData?.steps.find((s) => s.id === id);

  // Determine if setup is complete
  const setupComplete = setupData
    ? setupData.steps.every((step) => step.complete)
    : false;

  // Track previous setupComplete state to detect transition (only after initial load)
  const prevSetupComplete = useRef<boolean | null>(null);
  const hasInitiallyLoaded = useRef(false);

  // Show toast only when setup transitions from incomplete to complete (not on initial load)
  useEffect(() => {
    // Skip until initial load is complete
    if (setupLoading) return;

    // Mark initial load as complete, but don't show toast for it
    if (!hasInitiallyLoaded.current) {
      hasInitiallyLoaded.current = true;
      prevSetupComplete.current = setupComplete;
      return;
    }

    // Show toast only on actual transition from incomplete to complete
    if (prevSetupComplete.current === false && setupComplete === true) {
      toast.success('Setup complete! You can now start monitoring your tests.');
    }
    prevSetupComplete.current = setupComplete;
  }, [setupComplete, setupLoading]);

  // Build setup items from API data
  const getSetupItems = (data: SetupData) => {
    return data.steps.map((step) => {
      switch (step.id) {
        case 'create_account':
          return {
            key: step.id,
            done: step.complete,
            label: step.title,
            action: null,
            onClick: null,
          };
        case 'create_org':
          return {
            key: step.id,
            done: step.complete,
            label: step.title,
            action: step.complete ? null : 'Create org',
            onClick: step.complete ? null : () => onNavigate('settings'),
          };
        case 'install_github_app':
          return {
            key: step.id,
            done: step.complete,
            label: step.title,
            action: step.complete ? null : 'Install App',
            onClick: step.complete ? null : () => {
              if (data.github_install_url) {
                window.open(data.github_install_url, '_blank');
              }
            },
          };
        case 'connect_repo':
          return {
            key: step.id,
            done: step.complete,
            label: step.title,
            action: step.complete ? null : 'Connect',
            onClick: step.complete ? null : openConnectModal,
          };
        default:
          return {
            key: step.id,
            done: step.complete,
            label: step.title,
            action: null,
            onClick: null,
          };
      }
    });
  };

  const setupItems = setupData ? getSetupItems(setupData) : [];

  // Check if GitHub App is installed (step 2 complete)
  const isGitHubAppInstalled = getStep('install_github_app')?.complete ?? false;

  // Process metrics data from API
  const nowMetrics = useMemo(() => {
    const now = metricsData?.now;
    return [
      {
        label: 'Failing Monitors',
        value: now?.failing_monitors != null ? String(now.failing_monitors) : '—',
      },
      {
        label: 'Failing Tests (24h)',
        value: now?.failing_tests_24h != null ? String(now.failing_tests_24h) : '—',
      },
      {
        label: 'Runs in Progress',
        value: now?.runs_in_progress != null ? String(now.runs_in_progress) : '—',
      },
      {
        label: 'Pass Rate (24h)',
        value: now?.pass_rate_24h != null ? `${Math.round(now.pass_rate_24h)}%` : '—',
      },
      {
        label: 'Median Duration (24h)',
        value: now?.median_duration_ms_24h != null ? formatDuration(now.median_duration_ms_24h) : '—',
      },
    ];
  }, [metricsData]);

  // Pass rate over time - transform API data for chart
  const passRateData = useMemo(() => {
    if (!metricsData?.pass_rate_over_time) return [];
    return metricsData.pass_rate_over_time.map((point) => ({
      date: point.date.slice(5), // MM-DD format
      passRate: point.pass_rate,
      volume: point.volume,
    }));
  }, [metricsData]);

  // Failures by suite - transform API data for chart
  const failuresBySuite = useMemo(() => {
    if (!metricsData?.failures_by_suite_24h) return [];
    return metricsData.failures_by_suite_24h.map((item) => ({
      suite: item.suite,
      passes: item.passes,
      failures: item.failures,
    }));
  }, [metricsData]);

  return (
    <div className="flex-1 min-w-0 p-8">
      <div className="max-w-[1600px] mx-auto">
        {/* Setup Banner - only show when setup is incomplete, load silently */}
        {!setupLoading && !setupComplete && setupData ? (
          <div className="bg-[#f6a724]/10 border-2 border-[#f6a724] rounded-lg p-6 mb-6">
            <div className="flex items-start gap-4">
              <AlertCircle className="w-6 h-6 text-[#f6a724] flex-shrink-0 mt-1" />
              <div className="flex-1">
                <h2 className="mb-2">Finish setup to start monitoring</h2>
                <p className="text-sm text-[#666666] mb-4">
                  Complete these steps to unlock continuous monitoring and CI integration
                </p>
                <div className="space-y-3 mb-4">
                  {setupItems.map((item) => (
                    <div key={item.key} className="flex items-center gap-3">
                      {item.done ? (
                        <CheckCircle2 className="w-5 h-5 text-[#4CBB17]" />
                      ) : (
                        <Circle className="w-5 h-5 text-[#999999]" />
                      )}
                      <span className={`text-sm ${item.done ? 'text-[#666666]' : 'text-black'}`}>
                        {item.label}
                      </span>
                    </div>
                  ))}
                </div>
                {/* Primary CTA based on current step */}
                {!getStep('create_org')?.complete ? (
                  <Button onClick={() => onNavigate('settings')}>
                    Create organization
                  </Button>
                ) : !isGitHubAppInstalled && setupData.github_install_url ? (
                  <div className="flex items-center gap-3">
                    <a
                      href={setupData.github_install_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-2 px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors"
                    >
                      <ExternalLink className="w-4 h-4" />
                      Install GitHub App
                    </a>
                    <Button
                      variant="secondary"
                      onClick={handleSync}
                      loading={syncMutation.isPending}
                    >
                      {syncMutation.isPending ? 'Syncing...' : 'Already installed? Sync'}
                    </Button>
                  </div>
                ) : isGitHubAppInstalled && !getStep('connect_repo')?.complete ? (
                  <Button onClick={openConnectModal}>
                    Connect repository
                  </Button>
                ) : null}
              </div>
            </div>
          </div>
        ) : null}

        {/* "Now" Row - 5 Tiles */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4 mb-6">
          {nowMetrics.map((metric, idx) => (
            <div
              key={idx}
              className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-5"
            >
              <div className="flex items-start justify-between mb-3">
                <span className="text-sm text-[#666666]">{metric.label}</span>
              </div>
              <div className="text-3xl">
                {metric.value}
              </div>
            </div>
          ))}
        </div>

        {/* Main Charts Row */}
        <div className="grid grid-cols-1 lg:grid-cols-5 gap-6 mb-6">
          {/* Pass Rate Over Time - Takes 3 columns */}
          <div className="lg:col-span-3 bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <div className="flex items-center justify-between mb-6">
              <h3>Pass Rate Over Time</h3>
            </div>
            {passRateData.length === 0 ? (
              <div className="flex items-center justify-center h-[280px] text-[#999999]">
                <div className="text-center">
                  <p className="text-sm">No run data yet</p>
                  <p className="text-xs mt-1">Charts will appear once tests are run</p>
                </div>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={280}>
                <ComposedChart data={passRateData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#e5e5e5" />
                  <XAxis
                    dataKey="date"
                    tick={{ fill: '#666666', fontSize: 12 }}
                    stroke="#e5e5e5"
                  />
                  <YAxis
                    yAxisId="left"
                    tick={{ fill: '#666666', fontSize: 12 }}
                    stroke="#e5e5e5"
                    domain={[85, 100]}
                    label={{ value: 'Pass Rate (%)', angle: -90, position: 'insideLeft', style: { fill: '#666666', fontSize: 12 } }}
                  />
                  <YAxis
                    yAxisId="right"
                    orientation="right"
                    tick={{ fill: '#999999', fontSize: 12 }}
                    stroke="#e5e5e5"
                    label={{ value: 'Volume', angle: 90, position: 'insideRight', style: { fill: '#999999', fontSize: 12 } }}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'white',
                      border: '1px solid #e5e5e5',
                      borderRadius: '6px',
                      fontSize: '12px'
                    }}
                  />
                  <Bar
                    yAxisId="right"
                    dataKey="volume"
                    fill="#e5e5e5"
                    opacity={0.3}
                    name="Run Volume"
                  />
                  <Line
                    yAxisId="left"
                    type="monotone"
                    dataKey="passRate"
                    stroke="#4CBB17"
                    strokeWidth={3}
                    dot={{ fill: '#4CBB17', r: 4 }}
                    name="Pass Rate"
                  />
                </ComposedChart>
              </ResponsiveContainer>
            )}
          </div>

          {/* Failures by Suite - Takes 2 columns */}
          <div className="lg:col-span-2 bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <h3 className="mb-6">Recent Failures by Suite (24h)</h3>
            {failuresBySuite.length === 0 ? (
              <div className="flex items-center justify-center h-[280px] text-[#999999]">
                <div className="text-center">
                  <p className="text-sm">No failure data yet</p>
                  <p className="text-xs mt-1">Charts will appear once tests are run</p>
                </div>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={280}>
                <BarChart data={failuresBySuite} layout="vertical">
                  <CartesianGrid strokeDasharray="3 3" stroke="#e5e5e5" />
                  <XAxis
                    type="number"
                    tick={{ fill: '#666666', fontSize: 12 }}
                    stroke="#e5e5e5"
                  />
                  <YAxis
                    type="category"
                    dataKey="suite"
                    tick={{ fill: '#666666', fontSize: 11 }}
                    stroke="#e5e5e5"
                    width={120}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'white',
                      border: '1px solid #e5e5e5',
                      borderRadius: '6px',
                      fontSize: '12px'
                    }}
                  />
                  <Bar dataKey="passes" stackId="a" fill="#4CBB17" name="Passes" />
                  <Bar dataKey="failures" stackId="a" fill="#ef0000" name="Failures" />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>
      </div>

      {/* Connect Repository Modal */}
      {connectModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-black/50"
            onClick={closeConnectModal}
          />

          {/* Modal */}
          <div className="relative bg-white rounded-lg shadow-xl w-full max-w-lg mx-4 max-h-[90vh] overflow-hidden">
            {/* Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-[#e5e5e5]">
              <h2 className="text-lg font-semibold">Connect Repository</h2>
              <button
                onClick={closeConnectModal}
                className="p-1 hover:bg-[#f5f5f5] rounded-md transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Content */}
            <div className="p-6 overflow-y-auto max-h-[calc(90vh-140px)]">
              {/* Success state */}
              {connectResult && (
                <div className="space-y-4">
                  <div className="bg-[#4CBB17]/10 border border-[#4CBB17] rounded-lg p-4">
                    <div className="flex items-center gap-2 mb-2">
                      <CheckCircle2 className="w-5 h-5 text-[#4CBB17]" />
                      <span className="font-medium text-[#4CBB17]">Repository connected!</span>
                    </div>
                    <div className="text-sm text-[#666666]">
                      <p>Project <strong>{connectResult.name}</strong> created</p>
                    </div>
                  </div>
                  <Button onClick={closeConnectModal} className="w-full">
                    Done
                  </Button>
                </div>
              )}

              {/* Error state */}
              {connectMutation.error && !connectResult && (
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-4">
                  <p className="text-sm text-red-600">
                    {connectMutation.error instanceof Error
                      ? connectMutation.error.message
                      : 'Failed to connect repository'}
                  </p>
                </div>
              )}

              {/* Repository selection */}
              {!connectResult && (
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-2">
                      Repository
                    </label>
                    {reposLoading ? (
                      <div className="flex items-center gap-2 p-3 bg-[#fafafa] rounded-md">
                        <Loader2 className="w-4 h-4 animate-spin" />
                        <span className="text-sm text-[#666666]">Loading repositories...</span>
                      </div>
                    ) : reposError ? (
                      <div className="p-3 bg-red-50 border border-red-200 rounded-md">
                        <p className="text-sm text-red-600">
                          {reposError instanceof Error ? reposError.message : 'Failed to load repositories'}
                        </p>
                        <button
                          onClick={() => fetchRepos()}
                          className="text-sm text-red-700 hover:underline mt-1"
                        >
                          Try again
                        </button>
                      </div>
                    ) : !repos || repos.length === 0 ? (
                      <div className="p-3 bg-[#fafafa] border border-[#e5e5e5] rounded-md">
                        <p className="text-sm text-[#666666]">No repositories found.</p>
                        <p className="text-xs text-[#999999] mt-1">
                          Make sure the GitHub App has access to your repositories.
                        </p>
                      </div>
                    ) : (
                      <select
                        value={selectedRepo?.full_name || ''}
                        onChange={(e) => {
                          const repo = repos.find((r) => r.full_name === e.target.value);
                          setSelectedRepo(repo || null);
                        }}
                        className="w-full px-3 py-2 border border-[#e5e5e5] rounded-md bg-white focus:outline-none focus:ring-2 focus:ring-black/20"
                      >
                        <option value="">Select a repository...</option>
                        {repos.map((repo) => (
                          <option key={repo.full_name} value={repo.full_name}>
                            {repo.full_name} {repo.private ? '(private)' : ''}
                          </option>
                        ))}
                      </select>
                    )}
                  </div>

                  {/* Selected repo info */}
                  {selectedRepo && (
                    <div className="bg-[#fafafa] rounded-md p-3 text-sm">
                      <a
                        href={selectedRepo.html_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center gap-1 text-[#666666] hover:text-black"
                      >
                        <ExternalLink className="w-3 h-3" />
                        View on GitHub
                      </a>
                    </div>
                  )}

                  {/* Connect button */}
                  <Button
                    onClick={handleConnect}
                    disabled={!selectedRepo}
                    loading={connectMutation.isPending}
                    className="w-full"
                  >
                    {connectMutation.isPending ? 'Connecting...' : 'Connect repository'}
                  </Button>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
