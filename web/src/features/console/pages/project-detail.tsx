import { ArrowLeft, FolderOpen, ExternalLink, GitBranch, FileCode, Layers, Clock, Plus, Loader2 } from 'lucide-react';
import { useState } from 'react';
import { useProject, useProjectSuites, useProjectEnvironments, useProjectSchedules, useCreateProjectSchedule, useUpdateProjectSchedule, useDeleteProjectSchedule, type ProjectSchedule } from '../hooks/use-console-queries';
import { SourceRefBadge } from '../components/SourceRefBadge';
import { LoadingState, ErrorState, Card } from '../components/ui';
import { ScheduleCard, type ScheduleCardData } from '../components/schedule-card';
import { ScheduleFormModal, type ScheduleFormData } from '../components/schedule-form-modal';
import { formatRelativeTime } from '../lib/format';

interface ProjectDetailProps {
  projectId: string;
  onBack: () => void;
  onViewSuite?: (suiteId: string) => void;
}

export function ProjectDetail({ projectId, onBack, onViewSuite }: ProjectDetailProps) {
  const { data: project, isLoading: projectLoading, error: projectError, refetch: refetchProject } = useProject(projectId);
  const { data: suites, isLoading: suitesLoading, error: suitesError, refetch: refetchSuites } = useProjectSuites(projectId);
  const { data: environments = [] } = useProjectEnvironments(projectId);

  // Schedules
  const { data: projectSchedules = [], isLoading: schedulesLoading, refetch: refetchSchedules } = useProjectSchedules(projectId, { enabled: !!projectId });
  const createScheduleMutation = useCreateProjectSchedule(projectId);
  const updateScheduleMutation = useUpdateProjectSchedule();
  const deleteScheduleMutation = useDeleteProjectSchedule();

  // Tab state
  const [activeTab, setActiveTab] = useState<'suites' | 'schedules'>('suites');

  // Schedule modal state
  const [showAddScheduleModal, setShowAddScheduleModal] = useState(false);
  const [editingSchedule, setEditingSchedule] = useState<ProjectSchedule | null>(null);
  const [scheduleError, setScheduleError] = useState<string | null>(null);

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

  const isLoading = projectLoading || suitesLoading;
  const error = projectError || suitesError;

  // Back button component (shared across states)
  const BackButton = () => (
    <button
      onClick={onBack}
      className="flex items-center gap-2 text-[#666666] hover:text-black mb-6 transition-colors"
    >
      <ArrowLeft className="w-4 h-4" />
      Back to Projects
    </button>
  );

  if (isLoading) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <BackButton />
          <LoadingState message="Loading project..." />
        </div>
      </div>
    );
  }

  if (error || !project) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <BackButton />
          <ErrorState
            title={!project ? 'Project not found' : 'Failed to load project'}
            message={error instanceof Error ? error.message : 'An unexpected error occurred'}
            onRetry={() => { refetchProject(); refetchSuites(); }}
          />
        </div>
      </div>
    );
  }

  const projectSuites = suites || [];

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto">
        {/* Back Button */}
        <BackButton />

        {/* Project Header */}
        <Card className="mb-6">
          <div className="flex items-start justify-between mb-4">
            <div className="flex items-center gap-3">
              <FolderOpen className="w-6 h-6 text-[#666666]" />
              <h2>{project.name}</h2>
              <SourceRefBadge sourceRef={project.source_ref} defaultBranch={project.default_branch} />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm">
                <span className="text-[#999999] w-32">Repository:</span>
                <a
                  href={project.repo_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[#666666] hover:text-black font-mono flex items-center gap-1"
                >
                  {project.repo_url}
                  <ExternalLink className="w-3 h-3" />
                </a>
              </div>
              <div className="flex items-center gap-2 text-sm">
                <span className="text-[#999999] w-32">Path scope:</span>
                <span className="text-[#666666] font-mono">
                  {project.path_scope.length > 0 ? project.path_scope.join(', ') : '(root)'}
                </span>
              </div>
            </div>

            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm">
                <span className="text-[#999999] w-32">Default branch:</span>
                <span className="inline-flex items-center gap-1 text-[#666666]">
                  <GitBranch className="w-3 h-3" />
                  {project.default_branch}
                </span>
              </div>
              <div className="flex items-center gap-2 text-sm">
                <span className="text-[#999999] w-32">Last scan:</span>
                <span className="text-[#666666]">
                  {project.last_scan
                    ? new Date(project.last_scan.created_at).toLocaleString()
                    : 'Not yet scanned'
                  }
                </span>
              </div>
            </div>
          </div>
        </Card>

        {/* Stats Grid */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
          <Card>
            <div className="flex items-center gap-2 mb-2">
              <FileCode className="w-4 h-4 text-[#666666]" />
              <span className="text-sm text-[#999999]">Suites</span>
            </div>
            <div className="text-2xl">{project.suite_count}</div>
          </Card>

          <Card>
            <div className="flex items-center gap-2 mb-2">
              <FileCode className="w-4 h-4 text-[#666666]" />
              <span className="text-sm text-[#999999]">Tests</span>
            </div>
            <div className="text-2xl">{project.test_count}</div>
          </Card>

          <Card>
            <div className="flex items-center gap-2 mb-2">
              <Layers className="w-4 h-4 text-[#666666]" />
              <span className="text-sm text-[#999999]">Environments</span>
            </div>
            <div className="text-2xl">{environments?.length ?? 0}</div>
          </Card>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 border-b border-[#e5e5e5]">
          {(['suites', 'schedules'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`px-4 py-2 capitalize transition-colors ${
                activeTab === tab
                  ? 'border-b-2 border-black text-black'
                  : 'text-[#666666] hover:text-black'
              }`}
            >
              {tab}
            </button>
          ))}
        </div>

        {/* Suites Tab */}
        {activeTab === 'suites' && (
          <div className="mb-6">
            {projectSuites.length === 0 ? (
              <Card className="text-center">
                <p className="text-[#666666]">No suites found for this project</p>
              </Card>
            ) : (
              <div className="space-y-3">
                {projectSuites.map((suite) => (
                  <Card
                    key={suite.id}
                    padding="sm"
                    onClick={() => onViewSuite?.(suite.id)}
                    className="hover:shadow-md transition-shadow cursor-pointer p-5"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex-1">
                        <div className="flex items-center gap-3 mb-1">
                          <FileCode className="w-4 h-4 text-[#666666]" />
                          <span className="text-sm">{suite.name}</span>
                          <SourceRefBadge sourceRef={suite.source_ref} defaultBranch={project.default_branch} />
                        </div>
                        {suite.file_path && (
                          <p className="text-xs text-[#666666] font-mono ml-7">{suite.file_path}</p>
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-[#999999]">{suite.test_count} tests</span>
                      </div>
                    </div>
                  </Card>
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

            {/* Add Schedule Button */}
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
        environments={environments}
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
        environments={environments}
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
