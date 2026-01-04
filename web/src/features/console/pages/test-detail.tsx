import { ArrowLeft, Play, Edit3, Loader2 } from 'lucide-react';
import { StatusBadge, EnvBadge, TriggerBadge, UsernameBadge } from '../components/status-badge';
import { MultiSelectDropdown } from '../components/multi-select-dropdown';
import { RunStepCard } from '../components/run-steps';
import { useState, useMemo } from 'react';
import { useTestDetail, useTestRuns, type TestDetailStep, type RunStep } from '../hooks/use-console-queries';
import { useConsoleEnvironmentFilter } from '../hooks/use-console-filters';
import { formatDuration, formatRelativeTime } from '../lib/format';

interface TestDetailProps {
  testId: string;
  onBack: () => void;
  onViewRun: (runId: string) => void;
  onViewSuite?: (suiteId: string) => void;
}

// Map API status to UI status
function mapRunStatus(status: string): 'pending' | 'success' | 'failed' | 'running' {
  switch (status.toUpperCase()) {
    case 'PASSED':
      return 'success';
    case 'FAILED':
    case 'CANCELLED':
    case 'TIMEOUT':
      return 'failed';
    case 'RUNNING':
      return 'running';
    case 'PENDING':
    default:
      return 'pending';
  }
}

// Convert TestDetailStep to RunStep format for RunStepCard
function convertToRunStep(step: TestDetailStep): RunStep {
  return {
    id: `step-${step.step_index}`,
    run_test_id: '',
    step_index: step.step_index,
    name: step.name,
    plugin: step.plugin,
    status: 'DEFINITION', // Test definitions - not executed yet
    assertions_passed: 0,
    assertions_failed: 0,
    created_at: new Date().toISOString(),
    // Populate step_config for planned display
    step_config: {
      name: step.name,
      plugin: step.plugin,
      config: step.config,
      assertions: step.assertions,
      save: step.save,
      retry: step.retry,
    },
  };
}

export function TestDetail({ testId, onBack, onViewRun, onViewSuite }: TestDetailProps) {
  const [activeTab, setActiveTab] = useState<'steps' | 'schedules'>('steps');
  const [selectedTriggers, setSelectedTriggers] = useState<string[]>([]);
  const [showTriggerDropdown, setShowTriggerDropdown] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const runsPerPage = 10;

  // Fetch test detail data
  const { data: testDetail, isLoading: testLoading, error: testError } = useTestDetail(testId);

  // Get environment filter from header (uses project-scoped localStorage)
  const projectId = testDetail?.project_id || '';
  const { selectedEnvironmentId } = useConsoleEnvironmentFilter(projectId);

  // Fetch test runs with filters
  const { data: runs = [], isLoading: runsLoading } = useTestRuns(testId, {
    triggers: selectedTriggers.length > 0 ? selectedTriggers : undefined,
    environmentId: selectedEnvironmentId,
    limit: 100, // Fetch enough for pagination
  });

  // Filter and paginate runs
  const filteredRuns = useMemo(() => {
    return runs;
  }, [runs]);

  const paginatedRuns = useMemo(() => {
    const start = (currentPage - 1) * runsPerPage;
    return filteredRuns.slice(start, start + runsPerPage);
  }, [filteredRuns, currentPage, runsPerPage]);

  const totalPages = Math.max(1, Math.ceil(filteredRuns.length / runsPerPage));

  // Convert steps to RunStep format
  const stepsForDisplay = useMemo(() => {
    if (!testDetail?.steps) return [];
    return testDetail.steps.map(convertToRunStep);
  }, [testDetail?.steps]);

  // Loading state
  if (testLoading) {
    return (
      <div className="p-8 flex items-center justify-center min-h-[400px]">
        <div className="flex items-center gap-3 text-[#666666]">
          <Loader2 className="w-5 h-5 animate-spin" />
          <span>Loading test details...</span>
        </div>
      </div>
    );
  }

  // Error state
  if (testError || !testDetail) {
    return (
      <div className="p-8">
        <button
          onClick={onBack}
          className="flex items-center gap-2 text-[#666666] hover:text-black mb-4 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Back to tests</span>
        </button>
        <div className="bg-red-50 border border-red-200 rounded-lg p-6 text-center">
          <p className="text-red-600">Failed to load test details</p>
          <p className="text-sm text-[#666666] mt-2">{testError?.message || 'Test not found'}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8 flex gap-6">
      {/* Left Sidebar - Recent Test Runs */}
      <div className="w-80 flex-shrink-0">
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm sticky top-8">
          <div className="p-4 border-b border-[#e5e5e5]">
            <h3 className="mb-3">Recent Runs</h3>

            {/* Trigger Filter */}
            <div>
              <label className="text-xs text-[#999999] mb-1 block">Trigger</label>
              <MultiSelectDropdown
                label="Triggers"
                items={['ci', 'manual', 'schedule']}
                selectedItems={selectedTriggers}
                onSelectionChange={(triggers) => {
                  setSelectedTriggers(triggers);
                  setCurrentPage(1); // Reset to first page when filter changes
                }}
                isOpen={showTriggerDropdown}
                onToggle={() => setShowTriggerDropdown(!showTriggerDropdown)}
                showAllOption={true}
                placeholder="All triggers"
              />
            </div>
          </div>

          {/* Recent runs list */}
          <div>
            {runsLoading && filteredRuns.length === 0 ? (
              <div className="p-8 text-center">
                <Loader2 className="w-5 h-5 animate-spin mx-auto text-[#666666]" />
                <p className="text-sm text-[#666666] mt-2">Loading runs...</p>
              </div>
            ) : filteredRuns.length === 0 ? (
              <div className="p-8 text-center">
                <p className="text-sm text-[#666666]">No runs found</p>
                <p className="text-xs text-[#999999] mt-1">
                  {selectedTriggers.length > 0 || selectedEnvironmentId
                    ? 'Try adjusting your filters'
                    : 'This test has not been run yet'}
                </p>
              </div>
            ) : (
              paginatedRuns.map((run) => {
                const status = mapRunStatus(run.status);
                const isLive = run.status === 'RUNNING';

                return (
                  <div
                    key={run.id}
                    onClick={() => onViewRun(run.id)}
                    className="p-4 hover:bg-[#fafafa] transition-colors cursor-pointer border-b border-[#e5e5e5] last:border-b-0"
                  >
                    {/* Top row: Status with text on left, time on right */}
                    <div className="flex items-start justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <StatusBadge status={status} isLive={isLive} />
                        <span className="text-sm">
                          {status === 'success' ? 'Passed' : status === 'failed' ? 'Failed' : status === 'running' ? 'Running' : 'Pending'}
                        </span>
                      </div>
                      <span className="text-xs text-[#999999]">
                        {formatRelativeTime(run.started_at || run.created_at)}
                      </span>
                    </div>

                    {/* Middle row: Badges */}
                    <div className="flex items-center gap-2 mb-2 flex-wrap">
                      {run.environment && <EnvBadge env={run.environment} />}
                      <TriggerBadge trigger={run.trigger} />
                      {run.initiator_name && (
                        <UsernameBadge username={run.initiator_name} />
                      )}
                    </div>

                    {/* Bottom row: Duration */}
                    <div className="flex items-center gap-2 text-xs text-[#666666]">
                      {run.duration_ms !== undefined && (
                        <span>{formatDuration(run.duration_ms)}</span>
                      )}
                    </div>
                  </div>
                );
              })
            )}
          </div>

          {/* Pagination */}
          {filteredRuns.length > runsPerPage && (
            <div className="p-4 border-t border-[#e5e5e5]">
              <div className="flex items-center justify-between">
                <button
                  onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
                  disabled={currentPage === 1}
                  className="text-xs px-3 py-1.5 bg-white border border-[#e5e5e5] rounded hover:bg-[#fafafa] disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Previous
                </button>
                <span className="text-xs text-[#666666]">
                  Page {currentPage} of {totalPages}
                </span>
                <button
                  onClick={() => setCurrentPage(Math.min(totalPages, currentPage + 1))}
                  disabled={currentPage >= totalPages}
                  className="text-xs px-3 py-1.5 bg-white border border-[#e5e5e5] rounded hover:bg-[#fafafa] disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Next
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 min-w-0">
        {/* Header */}
        <div className="mb-6">
          <button
            onClick={onBack}
            className="flex items-center gap-2 text-[#666666] hover:text-black mb-4 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to tests</span>
          </button>

          <div className="flex items-start justify-between mb-6">
            <div>
              <h1 className="mb-0">{testDetail.name}</h1>
              <p className="text-sm text-[#666666] mt-2">
                Part of{' '}
                <button
                  onClick={() => onViewSuite?.(testDetail.suite_id)}
                  className="text-black hover:underline"
                >
                  {testDetail.suite_name}
                </button>
              </p>
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
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 border-b border-[#e5e5e5]">
          <button
            onClick={() => setActiveTab('steps')}
            className={`px-4 py-2 transition-colors ${
              activeTab === 'steps'
                ? 'border-b-2 border-black text-black'
                : 'text-[#666666] hover:text-black'
            }`}
          >
            Steps
          </button>
          <button
            disabled
            className="px-4 py-2 text-[#999999] cursor-not-allowed opacity-50"
          >
            Schedules
          </button>
        </div>

        {/* Tab Content */}
        {activeTab === 'steps' && (
          <div className="space-y-3">
            {stepsForDisplay.length === 0 ? (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-8 text-center">
                <p className="text-[#666666]">No steps defined</p>
                <p className="text-sm text-[#999999] mt-1">
                  This test has no step definitions
                </p>
              </div>
            ) : (
              stepsForDisplay.map((step) => (
                <RunStepCard
                  key={step.id}
                  step={step}
                  stepNumber={step.step_index + 1}
                />
              ))
            )}
          </div>
        )}

        {activeTab === 'schedules' && (
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-8 text-center">
            <p className="text-[#666666]">Schedules are configured at Suite/Project level</p>
            <p className="text-sm text-[#999999] mt-1">Coming soon</p>
          </div>
        )}
      </div>
    </div>
  );
}
