import { ArrowLeft, Search, GitBranch, Clock, Plus, Play, Loader2, FileCode, AlertCircle, RefreshCw } from 'lucide-react';
import { MultiSelectDropdown } from '../components/multi-select-dropdown';
import { TestItem } from '../components/test-item';
import { SuiteRunRow } from '../components/suite-run-row';
import { ScheduleCard, type ScheduleCardData } from '../components/schedule-card';
import { ScheduleFormModal, type ScheduleFormData } from '../components/schedule-form-modal';
import { useMemo, useState } from 'react';
import { useSuite, useSuiteRuns, useProjectEnvironments, useProjectSchedules, useCreateProjectSchedule, useUpdateProjectSchedule, useDeleteProjectSchedule, type SuiteRunSummary, type ProjectSchedule } from '../hooks/use-console-queries';
import { useConsoleEnvironmentFilter } from '../hooks/use-console-filters';
import { SourceRefBadge } from '../components/SourceRefBadge';
import { LoadingState, ErrorState } from '../components/ui';
import { formatRelativeTime } from '../lib/format';

interface SuiteDetailProps {
  suiteId: string;
  onBack: () => void;
  onViewRun: (runId: string) => void;
  onViewTest?: (testId: string) => void;
}


export function SuiteDetail({ suiteId, onBack, onViewRun, onViewTest }: SuiteDetailProps) {
  const { data: suite, isLoading, error, refetch } = useSuite(suiteId);

  // Get project environments and the local environment filter
  const projectId = suite?.project?.id || '';
  const { data: projectEnvironments = [] } = useProjectEnvironments(projectId);
  const { selectedEnvironmentId } = useConsoleEnvironmentFilter(projectId);

  // Use the local environment filter (from localStorage)
  const selectedEnvId = selectedEnvironmentId || undefined;

  // Fetch runs filtered by selected environment (or all runs if none selected)
  const { data: runs, isLoading: runsLoading, error: runsError, refetch: refetchRuns } = useSuiteRuns(suiteId, selectedEnvId);

  const [activeTab, setActiveTab] = useState<'activity' | 'tests' | 'schedules' | 'variables' | 'lifecycle-hooks' | 'retry-policy' | 'alerts'>('activity');
  const [selectedBranches, setSelectedBranches] = useState<string[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedTriggers, setSelectedTriggers] = useState<string[]>([]);
  const [showAddScheduleModal, setShowAddScheduleModal] = useState(false);
  const [editingSchedule, setEditingSchedule] = useState<ProjectSchedule | null>(null);
  const [openDropdown, setOpenDropdown] = useState<string | null>(null);
  const [scheduleError, setScheduleError] = useState<string | null>(null);

  // Tab configuration
  const tabs = [
    { id: 'activity', label: 'Activity', enabled: true },
    { id: 'tests', label: 'Tests', enabled: true },
    { id: 'schedules', label: 'Schedules', enabled: true },
    { id: 'variables', label: 'Variables', enabled: false },
    { id: 'lifecycle-hooks', label: 'Lifecycle Hooks', enabled: false },
    { id: 'retry-policy', label: 'Retry Policy', enabled: false },
    { id: 'alerts', label: 'Alerts & Notifications', enabled: false },
  ] as const;

  // Fetch project schedules from API
  const { data: projectSchedules = [], isLoading: schedulesLoading, refetch: refetchSchedules } = useProjectSchedules(projectId, { enabled: !!projectId });

  // Schedule mutations
  const createScheduleMutation = useCreateProjectSchedule(projectId);
  const updateScheduleMutation = useUpdateProjectSchedule();
  const deleteScheduleMutation = useDeleteProjectSchedule();


  // Map schedules to display format
  const schedules: (ScheduleCardData & { envId: string })[] = projectSchedules.map((s: ProjectSchedule) => ({
    id: s.id,
    env: s.environment.slug,
    envId: s.environment_id,
    name: s.name,
    cron: s.cron_expression,
    timezone: s.timezone,
    enabled: s.enabled,
    lastRun: s.last_run_at ? formatRelativeTime(s.last_run_at) : 'Never',
    nextRun: s.next_run_at ? formatRelativeTime(s.next_run_at) : 'Not scheduled',
    lastRunStatus: s.last_run_status,
  }));

  // Convert suite tests to the format expected by TestItem component
  const suiteTests = (suite?.tests || []).map((test) => {
    // Use step_summaries from API if available, otherwise fallback to step_count
    const steps = test.step_summaries && test.step_summaries.length > 0
      ? test.step_summaries.map((s) => ({
          plugin: s.plugin,
          name: s.name,
        }))
      : Array.from({ length: test.step_count }, (_, i) => ({
          plugin: 'unknown',
          name: `Step ${i + 1}`,
        }));

    return {
      id: test.id,
      name: test.name,
      steps,
    };
  });

  // Group runs by branch, sorted by latest run time
  // NOTE: This must be before early returns to satisfy React hooks rules
  const { runsByBranch, branches } = useMemo(() => {
    if (!runs || runs.length === 0) {
      return { runsByBranch: {} as Record<string, SuiteRunSummary[]>, branches: [] as string[] };
    }

    // Group runs by branch
    const grouped: Record<string, SuiteRunSummary[]> = {};
    for (const run of runs) {
      const branch = run.branch || 'unknown';
      if (!grouped[branch]) {
        grouped[branch] = [];
      }
      grouped[branch].push(run);
    }

    // Sort runs within each branch by created_at desc (already sorted from API, but ensure)
    for (const branch of Object.keys(grouped)) {
      grouped[branch].sort((a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      );
    }

    // Sort branches: main first, then by latest run time desc
    const sortedBranches = Object.keys(grouped).sort((a, b) => {
      // Main branch always comes first
      if (a === 'main') return -1;
      if (b === 'main') return 1;

      // Then sort by latest run time desc
      const aLatest = grouped[a][0]?.created_at || '';
      const bLatest = grouped[b][0]?.created_at || '';
      return new Date(bLatest).getTime() - new Date(aLatest).getTime();
    });

    return { runsByBranch: grouped, branches: sortedBranches };
  }, [runs]);

  // Back button component (shared across states)
  const BackButton = () => (
    <button
      onClick={onBack}
      className="flex items-center gap-2 text-[#666666] hover:text-black transition-colors mb-6"
    >
      <ArrowLeft className="w-4 h-4" />
      Back to Suite Activity
    </button>
  );

  if (isLoading) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <BackButton />
          <LoadingState message="Loading suite..." />
        </div>
      </div>
    );
  }

  if (error || !suite) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <BackButton />
          <ErrorState
            title={!suite ? 'Suite not found' : 'Failed to load suite'}
            message={error instanceof Error ? error.message : 'An unexpected error occurred'}
            onRetry={() => refetch()}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto">
        {/* Back Button */}
        <button
          onClick={onBack}
          className="flex items-center gap-2 text-[#666666] hover:text-black transition-colors mb-6"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Suite Activity
        </button>

        {/* Header */}
        <div className="mb-6">
          <div className="flex items-start justify-between mb-6">
            <div>
              <div className="flex items-center gap-3 mb-2">
                <h1 className="mb-0">{suite.name}</h1>
                <SourceRefBadge sourceRef={suite.source_ref} />
              </div>
              {suite.file_path && (
                <p className="text-sm text-[#666666] font-mono">{suite.file_path}</p>
              )}
              <p className="text-xs text-[#999999] mt-1">
                {suite.project.name} • {suite.test_count} tests
              </p>
            </div>

            <button
              disabled
              className="flex items-center gap-2 px-4 py-2 bg-black/50 text-white rounded-md cursor-not-allowed"
              title="Coming soon"
            >
              <Play className="w-4 h-4" />
              <span>Run suite</span>
            </button>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 border-b border-[#e5e5e5]">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => tab.enabled && setActiveTab(tab.id)}
              disabled={!tab.enabled}
              className={`px-4 py-2 transition-colors ${
                !tab.enabled
                  ? 'text-[#cccccc] cursor-not-allowed'
                  : activeTab === tab.id
                  ? 'border-b-2 border-black text-black'
                  : 'text-[#666666] hover:text-black'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Activity Tab */}
        {activeTab === 'activity' && (
          <div>
            {/* Loading state for runs */}
            {runsLoading && (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="w-6 h-6 animate-spin text-[#666666]" />
                <span className="ml-3 text-[#666666]">Loading run history...</span>
              </div>
            )}

            {/* Error state for runs */}
            {runsError && !runsLoading && (
              <div className="bg-red-50 border border-red-200 rounded-lg p-6 flex items-start gap-3">
                <AlertCircle className="w-5 h-5 text-red-500 flex-shrink-0 mt-0.5" />
                <div className="flex-1">
                  <p className="text-red-700 font-medium">Failed to load run history</p>
                  <p className="text-red-600 text-sm mt-1">
                    {runsError instanceof Error ? runsError.message : 'An unexpected error occurred'}
                  </p>
                </div>
                <button
                  onClick={() => refetchRuns()}
                  className="flex items-center gap-2 px-3 py-1.5 text-sm text-red-700 hover:bg-red-100 rounded transition-colors"
                >
                  <RefreshCw className="w-4 h-4" />
                  Retry
                </button>
              </div>
            )}

            {/* Empty state when no runs exist */}
            {!runsLoading && !runsError && branches.length === 0 && (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
                <Clock className="w-12 h-12 text-[#999999] mx-auto mb-4" />
                <h3 className="text-lg font-medium mb-2">No run history yet</h3>
                <p className="text-[#666666] text-sm mb-4">
                  Run this suite to see activity here.
                </p>
                <button
                  disabled
                  className="inline-flex items-center gap-2 px-4 py-2 bg-black/50 text-white rounded-md cursor-not-allowed"
                  title="Coming soon"
                >
                  <Play className="w-4 h-4" />
                  Run suite
                </button>
              </div>
            )}

            {/* Runs list */}
            {!runsLoading && !runsError && branches.length > 0 && (
              <>
                {/* Controls */}
                <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4 mb-6">
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
                    {/* Search - FIRST */}
                    <div className="lg:col-span-2">
                      <label className="text-xs text-[#999999] mb-1 block">Search</label>
                      <div className="relative">
                        <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-[#999999]" />
                        <input
                          type="text"
                          placeholder="Message or SHA..."
                          value={searchQuery}
                          onChange={(e) => setSearchQuery(e.target.value)}
                          className="w-full pl-10 pr-3 py-2 bg-white border border-[#e5e5e5] rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-black/5"
                        />
                      </div>
                    </div>

                    {/* Branch selector */}
                    <div>
                      <label className="text-xs text-[#999999] mb-1 block">Branch</label>
                      <MultiSelectDropdown
                        label="Branches"
                        items={branches}
                        selectedItems={selectedBranches}
                        onSelectionChange={setSelectedBranches}
                        isOpen={openDropdown === 'branches'}
                        onToggle={() => setOpenDropdown(openDropdown === 'branches' ? null : 'branches')}
                      />
                    </div>

                    {/* Trigger filter */}
                    <div>
                      <label className="text-xs text-[#999999] mb-1 block">Trigger</label>
                      <MultiSelectDropdown
                        label="Triggers"
                        items={['ci', 'manual', 'schedule']}
                        selectedItems={selectedTriggers}
                        onSelectionChange={setSelectedTriggers}
                        isOpen={openDropdown === 'triggers'}
                        onToggle={() => setOpenDropdown(openDropdown === 'triggers' ? null : 'triggers')}
                      />
                    </div>
                  </div>
                </div>

                {/* Activity List */}
                <div className="space-y-6">
                  {branches.map((branch) => {
                    if (selectedBranches.length > 0 && !selectedBranches.includes(branch)) return null;
                    const branchRuns = runsByBranch[branch] || [];

                    // Filter runs based on search and trigger
                    const filteredRuns = branchRuns.filter((run) => {
                      // Trigger filter
                      if (selectedTriggers.length > 0 && !selectedTriggers.includes(run.initiator_type)) {
                        return false;
                      }
                      // Search filter (commit_message or commit_sha)
                      if (searchQuery) {
                        const query = searchQuery.toLowerCase();
                        const matchesMessage = run.commit_message?.toLowerCase().includes(query);
                        const matchesSha = run.commit_sha?.toLowerCase().includes(query);
                        if (!matchesMessage && !matchesSha) {
                          return false;
                        }
                      }
                      return true;
                    });

                    if (filteredRuns.length === 0) return null;

                    return (
                      <div key={branch}>
                        {/* Branch header */}
                        <div className="flex items-center gap-2 mb-3">
                          <GitBranch className="w-4 h-4 text-[#666666]" />
                          <h3>{branch}</h3>
                          <span className="text-xs text-[#999999]">({filteredRuns.length} runs)</span>
                        </div>

                        {/* Runs list */}
                        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm divide-y divide-[#e5e5e5]">
                          {filteredRuns.map((run) => (
                            <SuiteRunRow
                              key={run.id}
                              run={run}
                              onClick={onViewRun}
                            />
                          ))}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </>
            )}
          </div>
        )}

        {/* Tests Tab */}
        {activeTab === 'tests' && (
          <div>
            {suiteTests.length === 0 ? (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
                <FileCode className="w-12 h-12 text-[#999999] mx-auto mb-4" />
                <h3 className="text-lg font-medium mb-2">No tests in this suite</h3>
                <p className="text-[#666666] text-sm">
                  Tests will appear here once they are defined in the suite configuration.
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {suiteTests.map((test) => (
                  <TestItem
                    key={test.id}
                    test={test}
                    onClick={() => onViewTest?.(test.id)}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        {/* Schedules Tab */}
        {activeTab === 'schedules' && (
          <div>
            {schedulesLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="w-6 h-6 animate-spin text-[#666666]" />
                <span className="ml-3 text-[#666666]">Loading schedules...</span>
              </div>
            ) : schedules.length > 0 ? (
              <div className="space-y-3">
                {schedules.map((schedule) => (
                  <ScheduleCard
                    key={schedule.id}
                    schedule={schedule}
                    onEdit={(scheduleId) => {
                      const s = projectSchedules.find(x => x.id === scheduleId);
                      if (s) {
                        setEditingSchedule(s);
                        setScheduleError(null);
                      }
                    }}
                    onDelete={(scheduleId) => {
                      if (confirm('Are you sure you want to delete this schedule?')) {
                        deleteScheduleMutation.mutate(scheduleId, {
                          onSuccess: () => refetchSchedules(),
                        });
                      }
                    }}
                  />
                ))}
              </div>
            ) : (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
                <Clock className="w-12 h-12 text-[#999999] mx-auto mb-4" />
                <h3 className="text-lg font-medium mb-2">No schedules configured</h3>
                <p className="text-[#666666] text-sm mb-4">
                  Create a schedule to run this project's tests automatically.
                </p>
              </div>
            )}

            {/* Add Schedule Button - moved to bottom right */}
            <div className="mt-4 flex justify-end">
              <button
                onClick={() => {
                  setShowAddScheduleModal(true);
                  setScheduleError(null);
                }}
                className="inline-flex items-center gap-2 px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors"
              >
                <Plus className="w-4 h-4" />
                Add Schedule
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Add Schedule Modal */}
      <ScheduleFormModal
        isOpen={showAddScheduleModal}
        mode="create"
        environments={projectEnvironments}
        isSubmitting={createScheduleMutation.isPending}
        error={scheduleError}
        onErrorClear={() => setScheduleError(null)}
        onClose={() => setShowAddScheduleModal(false)}
        onSubmit={(data: ScheduleFormData) => {
          createScheduleMutation.mutate({
            environment_id: data.environment_id,
            name: data.name,
            cron_expression: data.cron_expression,
            timezone: data.timezone,
            enabled: data.enabled,
          }, {
            onSuccess: () => {
              setShowAddScheduleModal(false);
              refetchSchedules();
            },
            onError: (error) => {
              setScheduleError(error instanceof Error ? error.message : 'Failed to create schedule');
            },
          });
        }}
      />

      {/* Edit Schedule Modal */}
      <ScheduleFormModal
        isOpen={!!editingSchedule}
        mode="edit"
        environments={projectEnvironments}
        initialValues={editingSchedule ? {
          name: editingSchedule.name,
          environment_id: editingSchedule.environment_id,
          cron_expression: editingSchedule.cron_expression,
          timezone: editingSchedule.timezone,
          enabled: editingSchedule.enabled,
        } : undefined}
        isSubmitting={updateScheduleMutation.isPending}
        error={scheduleError}
        onErrorClear={() => setScheduleError(null)}
        onClose={() => setEditingSchedule(null)}
        onSubmit={(data: ScheduleFormData) => {
          if (!editingSchedule) return;
          updateScheduleMutation.mutate({
            scheduleId: editingSchedule.id,
            data: {
              name: data.name,
              cron_expression: data.cron_expression,
              timezone: data.timezone,
              enabled: data.enabled,
            },
          }, {
            onSuccess: () => {
              setEditingSchedule(null);
              refetchSchedules();
            },
            onError: (error) => {
              setScheduleError(error instanceof Error ? error.message : 'Failed to update schedule');
            },
          });
        }}
      />
    </div>
  );
}