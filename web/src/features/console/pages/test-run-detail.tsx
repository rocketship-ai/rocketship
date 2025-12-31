import { ArrowLeft, Play, AlertCircle, Edit3, Loader2 } from 'lucide-react';
import { StatusBadge, EnvBadge, TriggerBadge, UsernameBadge, ConfigSourceBadge, BadgeDot } from '../components/status-badge';
import { useState } from 'react';
import { useTestRun, useTestRunLogs, useTestRunSteps } from '../hooks/use-console-queries';
import { RunStepCard } from '../components/run-steps';
import { formatDuration, formatDateTime } from '../lib/http';

interface TestRunDetailProps {
  testRunId: string;
  onBack: () => void;
}

// Map API status to UI status for StatusBadge
function mapStatus(status: string): 'pending' | 'success' | 'failed' | 'running' {
  switch (status.toUpperCase()) {
    case 'PASSED':
      return 'success';
    case 'FAILED':
    case 'CANCELLED':
      return 'failed';
    case 'RUNNING':
      return 'running';
    case 'PENDING':
    default:
      return 'pending';
  }
}

export function TestRunDetail({ testRunId, onBack }: TestRunDetailProps) {
  const [activeTab, setActiveTab] = useState<'steps' | 'logs' | 'artifacts'>('steps');

  // Fetch test run data from API
  const { data: testRunData, isLoading: testRunLoading, error: testRunError } = useTestRun(testRunId);
  const { data: logsData, isLoading: logsLoading } = useTestRunLogs(testRunId);
  const { data: stepsData, isLoading: stepsLoading } = useTestRunSteps(testRunId);

  // Loading state
  if (testRunLoading) {
    return (
      <div className="p-8 flex items-center justify-center min-h-[400px]">
        <div className="flex items-center gap-3 text-[#666666]">
          <Loader2 className="w-5 h-5 animate-spin" />
          <span>Loading test run details...</span>
        </div>
      </div>
    );
  }

  // Error state
  if (testRunError || !testRunData) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <button
            onClick={onBack}
            className="flex items-center gap-2 text-[#666666] hover:text-black mb-4 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to test</span>
          </button>
          <div className="bg-red-50 border border-red-200 rounded-lg p-6 text-center">
            <p className="text-red-600">Failed to load test run details</p>
            <p className="text-sm text-[#666666] mt-2">{testRunError?.message || 'Test run not found'}</p>
          </div>
        </div>
      </div>
    );
  }

  const { test, run } = testRunData;

  // Transform test run data for display
  const testRun = {
    id: test.id,
    testName: test.name || 'Test Run',
    status: mapStatus(test.status),
    env: run.environment || 'default',
    trigger: run.initiator_type as 'ci' | 'manual' | 'schedule',
    initiatorName: run.initiator_name || '',
    configSource: {
      type: (run.config_source === 'repo' ? 'repo' : 'uncommitted') as 'repo' | 'uncommitted',
      sha: run.commit_sha || run.bundle_sha || '',
    },
    duration: formatDuration(test.duration_ms),
    started: formatDateTime(test.started_at),
    ended: formatDateTime(test.ended_at),
    branch: run.branch || 'main',
    commit: run.commit_sha?.substring(0, 7) || '—',
  };

  // Format logs for display
  const logs = (logsData || [])
    .map(log => `[${new Date(log.logged_at).toLocaleTimeString()}] ${log.message}`)
    .join('\n') || 'No logs available';

  // Use step counts from the test to show summary info
  const passedCount = test.passed_steps;
  const failedCount = test.failed_steps;
  const totalSteps = test.step_count;

  // For now, show error message from test if available
  const failingStep = test.error_message ? {
    name: 'Test Execution',
    error: test.error_message,
    status: 'failed' as const,
  } : null;

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
            <span>Back to test</span>
          </button>

          <div className="flex items-start justify-between mb-4">
            <div>
              <h1 className="mb-2">{testRun.testName}</h1>
              <div className="flex items-center gap-3 flex-wrap">
                <StatusBadge status={testRun.status} />
                <EnvBadge env={testRun.env} />
                <ConfigSourceBadge type={testRun.configSource.type} sha={testRun.configSource.sha} />
                <BadgeDot />
                <TriggerBadge trigger={testRun.trigger} />
                {testRun.initiatorName && (
                  <UsernameBadge username={testRun.initiatorName} />
                )}
              </div>
            </div>

            <div className="flex items-center gap-2">
              <button
                disabled
                className="flex items-center gap-2 px-4 py-2 bg-white border border-[#e5e5e5] rounded-md text-[#999999] cursor-not-allowed opacity-60"
              >
                <Edit3 className="w-4 h-4" />
                <span>Edit</span>
              </button>
              <button
                disabled
                className="flex items-center gap-2 px-4 py-2 bg-black text-white rounded-md cursor-not-allowed opacity-60"
              >
                <Play className="w-4 h-4" />
                <span>Run now</span>
              </button>
            </div>
          </div>

          {/* Stats */}
          <div className="grid grid-cols-3 gap-4">
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Duration</p>
              <p className="text-sm">{testRun.duration}</p>
            </div>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Passed</p>
              <p className="text-sm text-[#228b22]">{passedCount} {passedCount === 1 ? 'step' : 'steps'}</p>
            </div>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Failed</p>
              <p className={`text-sm ${failedCount > 0 ? 'text-[#ef0000]' : 'text-[#666666]'}`}>{failedCount} {failedCount === 1 ? 'step' : 'steps'}</p>
            </div>
          </div>
        </div>

        {/* Failing Step Alert */}
        {failingStep && (
          <div className="bg-[#ef0000]/5 border border-[#ef0000]/20 rounded-lg p-4 mb-6">
            <div className="flex items-start gap-3">
              <AlertCircle className="w-5 h-5 text-[#ef0000] flex-shrink-0 mt-0.5" />
              <div className="flex-1">
                <p className="text-sm mb-1">
                  <strong>{failingStep.name}</strong> failed
                </p>
                {failingStep.error && (
                  <p className="text-sm text-[#666666]">{failingStep.error}</p>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Tabs */}
        <div className="flex gap-1 mb-6 border-b border-[#e5e5e5]">
          <button
            onClick={() => setActiveTab('steps')}
            className={`px-4 py-2 capitalize transition-colors ${
              activeTab === 'steps'
                ? 'border-b-2 border-black text-black'
                : 'text-[#666666] hover:text-black'
            }`}
          >
            steps
          </button>
          <button
            onClick={() => setActiveTab('logs')}
            className={`px-4 py-2 capitalize transition-colors ${
              activeTab === 'logs'
                ? 'border-b-2 border-black text-black'
                : 'text-[#666666] hover:text-black'
            }`}
          >
            logs
          </button>
          <button
            disabled
            className="px-4 py-2 text-[#999999] cursor-not-allowed opacity-50"
          >
            Artifacts
          </button>
        </div>

        {/* Tab Content */}
        {activeTab === 'steps' && (
          <div className="space-y-3">
            {stepsLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="w-5 h-5 animate-spin text-[#666666]" />
                <span className="ml-2 text-[#666666]">Loading steps...</span>
              </div>
            ) : !stepsData || stepsData.length === 0 ? (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-8 text-center">
                <p className="text-[#666666] mb-2">No step data available</p>
                <p className="text-sm text-[#999999]">
                  {totalSteps > 0
                    ? `This test has ${totalSteps} steps (${passedCount} passed, ${failedCount} failed)`
                    : 'No step information available'}
                </p>
              </div>
            ) : (
              stepsData.map((step, index) => (
                <RunStepCard key={step.id} step={step} stepNumber={(step.step_index ?? index) + 1} />
              ))
            )}
          </div>
        )}

        {activeTab === 'logs' && (
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <div className="flex justify-end gap-2 mb-4">
              <button className="text-sm text-[#666666] hover:text-black transition-colors">
                Copy
              </button>
              <button className="text-sm text-[#666666] hover:text-black transition-colors">
                Download
              </button>
            </div>
            {logsLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="w-5 h-5 animate-spin text-[#666666]" />
                <span className="ml-2 text-[#666666]">Loading logs...</span>
              </div>
            ) : (
              <pre className="bg-black rounded p-4 font-mono text-xs text-[#00ff00] overflow-x-auto max-h-96 overflow-y-auto whitespace-pre-wrap">
                {logs}
              </pre>
            )}
          </div>
        )}

        {activeTab === 'artifacts' && (
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <p className="text-sm text-[#666666]">Artifacts tab is disabled</p>
          </div>
        )}
      </div>
    </div>
  );
}
