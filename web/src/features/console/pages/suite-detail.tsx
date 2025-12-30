import { ArrowLeft, Search, GitBranch, Hash, Clock, CheckCircle2, XCircle, Plus, X, Edit2, ToggleRight, ToggleLeft, Play, Loader2, AlertCircle, RefreshCw, FileCode } from 'lucide-react';
import { EnvBadge, InitiatorBadge } from '../components/status-badge';
import { MultiSelectDropdown } from '../components/multi-select-dropdown';
import { TestItem } from '../components/test-item';
import { useMemo, useState } from 'react';
import { environments } from '../data/mock-data';
import { useSuite, useSuiteRuns, type SuiteRunSummary } from '../hooks/use-console-queries';
import { SourceRefBadge } from '../components/SourceRefBadge';

interface SuiteDetailProps {
  suiteId: string;
  onBack: () => void;
  onViewRun: (runId: string) => void;
  onViewTest?: (testId: string) => void;
}

// Helper to display branch name
function BranchDisplay({ branch }: { branch: string }) {
  return (
    <span className="inline-flex items-center gap-1 text-xs text-[#666666]">
      <GitBranch className="w-3 h-3" />
      {branch}
    </span>
  );
}

// Map API status to UI status
function mapStatus(status: string): 'success' | 'failed' | 'running' {
  switch (status.toUpperCase()) {
    case 'PASSED':
      return 'success';
    case 'RUNNING':
      return 'running';
    case 'FAILED':
    case 'CANCELLED':
    default:
      return 'failed';
  }
}

// Format duration in human-readable format
function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
}

// Format relative time
function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / (1000 * 60));
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

export function SuiteDetail({ suiteId, onBack, onViewRun, onViewTest }: SuiteDetailProps) {
  const { data: suite, isLoading, error, refetch } = useSuite(suiteId);
  const { data: runs, isLoading: runsLoading, error: runsError, refetch: refetchRuns } = useSuiteRuns(suiteId);

  const [activeTab, setActiveTab] = useState<'activity' | 'tests' | 'schedules' | 'variables' | 'lifecycle-hooks' | 'retry-policy' | 'alerts'>('activity');
  const [selectedBranches, setSelectedBranches] = useState<string[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedInitiators, setSelectedInitiators] = useState<string[]>([]);
  const [_selectedEnvironments, _setSelectedEnvironments] = useState<string[]>([]);
  const [showAddScheduleModal, setShowAddScheduleModal] = useState(false);
  const [editingSchedule, setEditingSchedule] = useState<string | null>(null);
  const [newScheduleEnv, setNewScheduleEnv] = useState('staging');
  const [newScheduleCron, setNewScheduleCron] = useState('');
  const [newScheduleEnabled, setNewScheduleEnabled] = useState(true);
  const [openDropdown, setOpenDropdown] = useState<string | null>(null);

  // Tab configuration
  const tabs = [
    { id: 'activity', label: 'Activity', enabled: true },
    { id: 'tests', label: 'Tests', enabled: true },
    { id: 'schedules', label: 'Schedules', enabled: false },
    { id: 'variables', label: 'Variables', enabled: false },
    { id: 'lifecycle-hooks', label: 'Lifecycle Hooks', enabled: false },
    { id: 'retry-policy', label: 'Retry Policy', enabled: false },
    { id: 'alerts', label: 'Alerts & Notifications', enabled: false },
  ] as const;

  // Suite schedules - placeholder (no DB persistence yet)
  const schedules = [
    {
      id: 'sched-1',
      env: 'staging',
      cron: '*/30 * * * *',
      enabled: false,
      lastRun: '15 minutes ago',
      nextRun: 'in 15 minutes',
    },
    {
      id: 'sched-2',
      env: 'production',
      cron: '0 */6 * * *',
      enabled: true,
      lastRun: '2 hours ago',
      nextRun: 'in 4 hours',
    },
  ];

  // Convert suite tests to the format expected by TestItem component
  const suiteTests = (suite?.tests || []).map((test) => ({
    id: test.id,
    name: test.name,
    type: 'HTTP' as const,
    tags: [] as string[],
    steps: Array.from({ length: test.step_count }, (_, i) => ({
      method: 'GET' as const,
      name: `Step ${i + 1}`,
    })),
  }));

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

  if (isLoading) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <button
            onClick={onBack}
            className="flex items-center gap-2 text-[#666666] hover:text-black transition-colors mb-6"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to Suite Activity
          </button>
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-6 h-6 animate-spin text-[#666666]" />
            <span className="ml-3 text-[#666666]">Loading suite...</span>
          </div>
        </div>
      </div>
    );
  }

  if (error || !suite) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <button
            onClick={onBack}
            className="flex items-center gap-2 text-[#666666] hover:text-black transition-colors mb-6"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to Suite Activity
          </button>
          <div className="bg-red-50 border border-red-200 rounded-lg p-6 flex items-start gap-3">
            <AlertCircle className="w-5 h-5 text-red-500 flex-shrink-0 mt-0.5" />
            <div className="flex-1">
              <p className="text-red-700 font-medium">
                {!suite ? 'Suite not found' : 'Failed to load suite'}
              </p>
              <p className="text-red-600 text-sm mt-1">
                {error instanceof Error ? error.message : 'An unexpected error occurred'}
              </p>
            </div>
            <button
              onClick={() => refetch()}
              className="flex items-center gap-2 px-3 py-1.5 text-sm text-red-700 hover:bg-red-100 rounded transition-colors"
            >
              <RefreshCw className="w-4 h-4" />
              Retry
            </button>
          </div>
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

                    {/* Initiator filter */}
                    <div>
                      <label className="text-xs text-[#999999] mb-1 block">Initiator</label>
                      <MultiSelectDropdown
                        label="Initiators"
                        items={['ci', 'manual', 'schedule']}
                        selectedItems={selectedInitiators}
                        onSelectionChange={setSelectedInitiators}
                        isOpen={openDropdown === 'initiators'}
                        onToggle={() => setOpenDropdown(openDropdown === 'initiators' ? null : 'initiators')}
                      />
                    </div>
                  </div>
                </div>

                {/* Activity List */}
                <div className="space-y-6">
                  {branches.map((branch) => {
                    if (selectedBranches.length > 0 && !selectedBranches.includes(branch)) return null;
                    const branchRuns = runsByBranch[branch] || [];

                    // Filter runs based on search and initiator
                    const filteredRuns = branchRuns.filter((run) => {
                      // Initiator filter
                      if (selectedInitiators.length > 0 && !selectedInitiators.includes(run.initiator_type)) {
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
                          {filteredRuns.map((run) => {
                            const status = mapStatus(run.status);
                            // Prefer commit message, then "Commit <sha>", then "Manual run"
                            const title = run.commit_message
                              || (run.commit_sha ? `Commit ${run.commit_sha.slice(0, 7)}` : 'Manual run');

                            return (
                              <div
                                key={run.id}
                                onClick={() => onViewRun(run.id)}
                                className="p-4 hover:bg-[#fafafa] transition-colors cursor-pointer"
                              >
                                <div className="flex items-center justify-between">
                                  <div className="flex items-center gap-4 flex-1">
                                    {/* Status icon */}
                                    <div>
                                      {status === 'success' && (
                                        <CheckCircle2 className="w-5 h-5 text-[#4CBB17]" />
                                      )}
                                      {status === 'failed' && (
                                        <XCircle className="w-5 h-5 text-[#ef0000]" />
                                      )}
                                      {status === 'running' && (
                                        <Loader2 className="w-5 h-5 text-[#4CBB17] animate-spin" />
                                      )}
                                    </div>

                                    {/* Run info */}
                                    <div className="flex-1">
                                      <p className="text-sm mb-1 truncate max-w-lg">{title}</p>
                                      <div className="flex items-center gap-3 flex-wrap">
                                        <BranchDisplay branch={run.branch} />
                                        {run.commit_sha && (
                                          <span className="inline-flex items-center gap-1 text-xs text-[#666666] font-mono">
                                            <Hash className="w-3 h-3" />
                                            {run.commit_sha.slice(0, 7)}
                                          </span>
                                        )}
                                        {run.environment && <EnvBadge env={run.environment} />}
                                        <InitiatorBadge initiator={run.initiator_type} />
                                        {run.initiator_type === 'manual' && run.initiator_name && (
                                          <span className="text-xs text-[#666666]">@{run.initiator_name}</span>
                                        )}
                                      </div>
                                    </div>

                                    {/* Metadata */}
                                    <div className="flex items-center gap-4 text-sm text-[#666666]">
                                      <span>{formatRelativeTime(run.created_at)}</span>
                                      {run.duration_ms !== undefined && (
                                        <span>{formatDuration(run.duration_ms)}</span>
                                      )}
                                    </div>
                                  </div>
                                </div>
                              </div>
                            );
                          })}
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
            {schedules.length > 0 ? (
              <div className="space-y-3">
                {schedules.map((schedule) => (
                  <div
                    key={schedule.id}
                    className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6"
                  >
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        <div className="flex items-center gap-3 mb-2">
                          <span className="text-sm px-2 py-1 bg-[#fafafa] rounded border border-[#e5e5e5] text-[#666666]">
                            {schedule.env}
                          </span>
                          <code className="text-sm font-mono px-2 py-1 bg-[#fafafa] rounded border border-[#e5e5e5]">
                            {schedule.cron}
                          </code>
                          <span className={`text-sm px-2 py-1 rounded border ${
                            schedule.enabled 
                              ? 'bg-green-50 text-green-700 border-green-200' 
                              : 'bg-gray-50 text-gray-700 border-gray-200'
                          }`}>
                            {schedule.enabled ? 'Enabled' : 'Disabled'}
                          </span>
                        </div>
                        <div className="flex items-center gap-4 text-sm text-[#666666]">
                          <span>Last run: {schedule.lastRun}</span>
                          {schedule.enabled && (
                            <>
                              <span>•</span>
                              <span>Next run: {schedule.nextRun}</span>
                            </>
                          )}
                        </div>
                      </div>

                      <button 
                        onClick={() => {
                          setEditingSchedule(schedule.id);
                          setNewScheduleEnv(schedule.env);
                          setNewScheduleCron(schedule.cron);
                          setNewScheduleEnabled(schedule.enabled);
                        }}
                        className="p-2 text-[#666666] hover:text-black transition-colors"
                      >
                        <Edit2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
                <p className="text-[#666666]">No schedules configured for this suite</p>
              </div>
            )}

            {/* Add Schedule Button - moved to bottom right */}
            <div className="mt-4 flex justify-end">
              <button
                onClick={() => setShowAddScheduleModal(true)}
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
      {showAddScheduleModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-xl w-full max-w-md mx-4">
            {/* Modal Header */}
            <div className="flex items-center justify-between p-6 border-b border-[#e5e5e5]">
              <h3>Add Schedule</h3>
              <button
                onClick={() => setShowAddScheduleModal(false)}
                className="text-[#666666] hover:text-black transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Modal Body */}
            <div className="p-6 space-y-4">
              {/* Environment */}
              <div>
                <label className="text-sm mb-2 block">Environment</label>
                <select
                  value={newScheduleEnv}
                  onChange={(e) => setNewScheduleEnv(e.target.value)}
                  className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-black/5"
                >
                  {environments.map((env) => (
                    <option key={env.name} value={env.name}>
                      {env.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* Cron Expression */}
              <div>
                <label className="text-sm mb-2 block">Cron Expression</label>
                <input
                  type="text"
                  value={newScheduleCron}
                  onChange={(e) => setNewScheduleCron(e.target.value)}
                  placeholder="*/30 * * * *"
                  className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md text-sm font-mono focus:outline-none focus:ring-2 focus:ring-black/5"
                />
                <p className="text-xs text-[#999999] mt-1">
                  Example: */30 * * * * (every 30 minutes)
                </p>
              </div>

              {/* Enabled Toggle */}
              <div className="flex items-center justify-between">
                <label className="text-sm">Enabled</label>
                <button
                  onClick={() => setNewScheduleEnabled(!newScheduleEnabled)}
                  className={`p-2 rounded transition-colors ${
                    newScheduleEnabled ? 'text-[#4CBB17]' : 'text-[#999999]'
                  }`}
                >
                  {newScheduleEnabled ? (
                    <ToggleRight className="w-6 h-6" />
                  ) : (
                    <ToggleLeft className="w-6 h-6" />
                  )}
                </button>
              </div>
            </div>

            {/* Modal Footer */}
            <div className="flex items-center justify-end gap-3 p-6 border-t border-[#e5e5e5]">
              <button
                onClick={() => setShowAddScheduleModal(false)}
                className="px-4 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => {
                  // TODO: Add schedule logic
                  setShowAddScheduleModal(false);
                  setNewScheduleCron('');
                }}
                className="px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors"
              >
                Add Schedule
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Edit Schedule Modal */}
      {editingSchedule && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-xl w-full max-w-md mx-4">
            {/* Modal Header */}
            <div className="flex items-center justify-between p-6 border-b border-[#e5e5e5]">
              <h3>Edit Schedule</h3>
              <button
                onClick={() => setEditingSchedule(null)}
                className="text-[#666666] hover:text-black transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Modal Body */}
            <div className="p-6 space-y-4">
              {/* Environment */}
              <div>
                <label className="text-sm mb-2 block">Environment</label>
                <select
                  value={newScheduleEnv}
                  onChange={(e) => setNewScheduleEnv(e.target.value)}
                  className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-black/5"
                >
                  {environments.map((env) => (
                    <option key={env.name} value={env.name}>
                      {env.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* Cron Expression */}
              <div>
                <label className="text-sm mb-2 block">Cron Expression</label>
                <input
                  type="text"
                  value={newScheduleCron}
                  onChange={(e) => setNewScheduleCron(e.target.value)}
                  placeholder="*/30 * * * *"
                  className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md text-sm font-mono focus:outline-none focus:ring-2 focus:ring-black/5"
                />
                <p className="text-xs text-[#999999] mt-1">
                  Example: */30 * * * * (every 30 minutes)
                </p>
              </div>

              {/* Enabled Toggle */}
              <div className="flex items-center justify-between">
                <label className="text-sm">Enabled</label>
                <button
                  onClick={() => setNewScheduleEnabled(!newScheduleEnabled)}
                  className={`p-2 rounded transition-colors ${
                    newScheduleEnabled ? 'text-[#4CBB17]' : 'text-[#999999]'
                  }`}
                >
                  {newScheduleEnabled ? (
                    <ToggleRight className="w-6 h-6" />
                  ) : (
                    <ToggleLeft className="w-6 h-6" />
                  )}
                </button>
              </div>
            </div>

            {/* Modal Footer */}
            <div className="flex items-center justify-end gap-3 p-6 border-t border-[#e5e5e5]">
              <button
                onClick={() => setEditingSchedule(null)}
                className="px-4 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => {
                  // TODO: Save schedule changes
                  setEditingSchedule(null);
                }}
                className="px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors"
              >
                Save Changes
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}