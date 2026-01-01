import { ArrowLeft, RotateCw, Download, GitBranch, Hash, CheckCircle2, XCircle, Clock, Loader2 } from 'lucide-react';
import { EnvBadge, TriggerBadge, UsernameBadge, ConfigSourceBadge, BadgeDot } from '../components/status-badge';
import { TestItem } from '../components/test-item';
import { LogsPanel } from '../components/logs-panel';
import { useState } from 'react';
import { useRun, useRunTests, useRunLogs, type RunTest } from '../hooks/use-console-queries';
import { LoadingState, ErrorState } from '../components/ui';
import { formatDuration, formatDateTime, mapRunStatus, mapTestStatus, mapStepStatusForSummary } from '../lib/format';

interface SuiteRunDetailProps {
  suiteRunId: string;
  onBack: () => void;
  onViewTestRun: (testRunId: string) => void;
}

// Transform RunTest to TestItem format
function transformTestRun(test: RunTest) {
  // Transform step summaries for step chips display
  const steps = (test.steps || []).map(step => ({
    name: step.name,
    plugin: step.plugin,
    status: mapStepStatusForSummary(step.status),
  }));

  return {
    id: test.id,
    name: test.name,
    status: mapTestStatus(test.status),
    duration: formatDuration(test.duration_ms),
    steps,
  };
}

export function SuiteRunDetail({ suiteRunId, onBack, onViewTestRun }: SuiteRunDetailProps) {
  const [activeTab, setActiveTab] = useState<'test-runs' | 'logs' | 'artifacts'>('test-runs');

  // Fetch run data from API
  const { data: runData, isLoading: runLoading, error: runError } = useRun(suiteRunId);
  const { data: testsData, isLoading: testsLoading } = useRunTests(suiteRunId);
  const { data: logsData, isLoading: logsLoading } = useRunLogs(suiteRunId);

  // Back button component (shared across states)
  const BackButton = () => (
    <button
      onClick={onBack}
      className="flex items-center gap-2 text-[#666666] hover:text-black mb-4 transition-colors"
    >
      <ArrowLeft className="w-4 h-4" />
      <span>Back to suite</span>
    </button>
  );

  // Loading state
  if (runLoading) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <BackButton />
          <LoadingState message="Loading run details..." />
        </div>
      </div>
    );
  }

  // Error state
  if (runError || !runData) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <BackButton />
          <ErrorState
            title="Failed to load run details"
            message={runError?.message || 'Run not found'}
          />
        </div>
      </div>
    );
  }

  // Transform data for display
  const isUncommitted = runData.config_source === 'uncommitted';
  const suiteRun = {
    id: runData.id,
    suiteName: runData.suite_name || 'Suite Run',
    status: mapRunStatus(runData.status),
    env: runData.environment || 'default',
    trigger: runData.initiator_type as 'ci' | 'manual' | 'schedule',
    initiatorName: runData.initiator_name || '',
    configSource: {
      type: (isUncommitted ? 'uncommitted' : 'repo_commit') as 'repo_commit' | 'uncommitted',
      sha: runData.commit_sha || runData.bundle_sha || '',
    },
    duration: formatDuration(runData.duration_ms),
    started: formatDateTime(runData.started_at),
    ended: formatDateTime(runData.ended_at),
    branch: runData.branch || 'main',
    commit: runData.commit_sha?.substring(0, 7) || '—',
    isUncommitted,
    passed: runData.passed_tests,
    failed: runData.failed_tests,
    skipped: runData.skipped_tests,
  };

  // Transform test runs
  const testRuns = (testsData || []).map(transformTestRun);

  // Format logs for display
  const logs = (logsData || [])
    .map(log => `[${new Date(log.logged_at).toLocaleTimeString()}] ${log.message}`)
    .join('\n') || 'No logs available';

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-6">
          <button
            onClick={onBack}
            className="flex items-center gap-2 text-[#666666] hover:text-black mb-4 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to suite</span>
          </button>

          <div className="flex items-start justify-between mb-4">
            <div>
              <h1 className="mb-2">{suiteRun.suiteName}</h1>
              <div className="flex items-center gap-3 flex-wrap">
                <div>
                  {suiteRun.status === 'success' && (
                    <CheckCircle2 className="w-5 h-5 text-[#4CBB17]" />
                  )}
                  {suiteRun.status === 'failed' && (
                    <XCircle className="w-5 h-5 text-[#ef0000]" />
                  )}
                  {suiteRun.status === 'running' && (
                    <Clock className="w-5 h-5 text-[#4CBB17] animate-spin" />
                  )}
                </div>
                <EnvBadge env={suiteRun.env} />
                {/* Only show ConfigSourceBadge for uncommitted runs - repo_commit is redundant with commit shown below */}
                {suiteRun.configSource.type === 'uncommitted' && (
                  <ConfigSourceBadge type="uncommitted" />
                )}
                <BadgeDot />
                <TriggerBadge trigger={suiteRun.trigger} />
                {suiteRun.initiatorName && (
                  <UsernameBadge username={suiteRun.initiatorName} />
                )}
              </div>
            </div>

            <div className="flex items-center gap-2">
              <button disabled className="flex items-center gap-2 px-4 py-2 bg-white border border-[#e5e5e5] rounded-md text-[#cccccc] cursor-not-allowed">
                <RotateCw className="w-4 h-4" />
                <span>Rerun</span>
              </button>
              <button disabled className="flex items-center gap-2 px-4 py-2 bg-white border border-[#e5e5e5] rounded-md text-[#cccccc] cursor-not-allowed">
                <Download className="w-4 h-4" />
                <span>Export</span>
              </button>
            </div>
          </div>

          {/* Stats */}
          <div className="grid grid-cols-4 gap-4">
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Duration</p>
              <p className="text-xl">{suiteRun.duration}</p>
            </div>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Passed</p>
              <p className="text-xl text-[#228b22]">{suiteRun.passed}</p>
            </div>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Failed</p>
              <p className={`text-xl ${suiteRun.failed > 0 ? 'text-[#ef0000]' : 'text-[#999999]'}`}>{suiteRun.failed}</p>
            </div>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Skipped</p>
              <p className="text-xl text-[#999999]">{suiteRun.skipped}</p>
            </div>
          </div>
        </div>

        {/* Metadata */}
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 mb-6">
          <div className="grid grid-cols-2 gap-6">
            <div>
              <p className="text-xs text-[#999999] mb-1">Started</p>
              <p className="text-sm">{suiteRun.started}</p>
            </div>
            <div>
              <p className="text-xs text-[#999999] mb-1">Ended</p>
              <p className="text-sm">{suiteRun.ended}</p>
            </div>
            <div>
              <p className="text-xs text-[#999999] mb-1">Branch</p>
              <p className="text-sm flex items-center gap-2">
                <GitBranch className="w-3 h-3" />
                {suiteRun.branch}
              </p>
            </div>
            <div>
              <p className="text-xs text-[#999999] mb-1">
                {suiteRun.isUncommitted ? 'Base Commit' : 'Commit'}
              </p>
              <p className="text-sm flex items-center gap-2">
                <Hash className="w-3 h-3" />
                {suiteRun.commit}
              </p>
            </div>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 border-b border-[#e5e5e5]">
          {(['test-runs', 'logs', 'artifacts'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => tab !== 'artifacts' && setActiveTab(tab)}
              disabled={tab === 'artifacts'}
              className={`px-4 py-2 capitalize transition-colors ${
                tab === 'artifacts'
                  ? 'text-[#cccccc] cursor-not-allowed'
                  : activeTab === tab
                  ? 'border-b-2 border-black text-black'
                  : 'text-[#666666] hover:text-black'
              }`}
            >
              {tab === 'test-runs' ? 'Test Runs' : tab}
            </button>
          ))}
        </div>

        {/* Tab Content */}
        {activeTab === 'test-runs' && (
          <div className="space-y-3">
            {testsLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="w-5 h-5 animate-spin text-[#666666]" />
                <span className="ml-2 text-[#666666]">Loading tests...</span>
              </div>
            ) : testRuns.length === 0 ? (
              <div className="text-center py-8 text-[#666666]">No tests found</div>
            ) : (
              testRuns.map((testRun) => (
                <TestItem
                  key={testRun.id}
                  test={testRun}
                  onClick={() => onViewTestRun(testRun.id)}
                />
              ))
            )}
          </div>
        )}

        {activeTab === 'logs' && (
          <LogsPanel logs={logs} isLoading={logsLoading} />
        )}

        {activeTab === 'artifacts' && (
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
            <p className="text-[#666666]">No artifacts available</p>
          </div>
        )}
      </div>
    </div>
  );
}