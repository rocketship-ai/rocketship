import { Plus, Settings, Loader2, Key } from 'lucide-react';
import { useState, useMemo } from 'react';
import { useQueries } from '@tanstack/react-query';
import {
  useProjects,
  useProjectEnvironments,
  useCreateEnvironment,
  useUpdateEnvironment,
  useDeleteEnvironment,
  useCITokens,
  useCreateCIToken,
  useRevokeCIToken,
  consoleKeys,
} from '../hooks/use-console-queries';
import {
  useConsoleProjectFilter,
  useConsoleEnvironmentFilter,
  useConsoleEnvironmentSelectionMap,
} from '../hooks/use-console-filters';
import type { ProjectEnvironment, ProjectSummary } from '../hooks/use-console-queries';
import { EnvironmentCard } from '../components/environment-card';
import { EnvironmentModal, type EnvironmentSubmitData } from '../components/environment-modal';
import { CITokenModal } from '../components/ci-token-modal';
import { CITokenRevealModal } from '../components/ci-token-reveal-modal';
import { CITokensTable } from '../components/ci-tokens-table';
import { apiGet } from '@/lib/api';
import { useQueryClient } from '@tanstack/react-query';

// Helper type for All Projects view
interface EnvWithProject extends ProjectEnvironment {
  projectName: string;
}

export function Environments() {
  // Global project filter - derive effective project from it
  const { selectedProjectIds } = useConsoleProjectFilter();
  // If exactly one project is selected, use single-project mode; otherwise All Projects
  const selectedProjectId = selectedProjectIds.length === 1 ? selectedProjectIds[0] : '';
  const [showModal, setShowModal] = useState(false);
  const [editingEnv, setEditingEnv] = useState<ProjectEnvironment | null>(null);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const queryClient = useQueryClient();

  // CI Token state
  const [showCITokenModal, setShowCITokenModal] = useState(false);
  const [showTokenReveal, setShowTokenReveal] = useState(false);
  const [revealedToken, setRevealedToken] = useState({ value: '', name: '' });
  const [revokingTokenId, setRevokingTokenId] = useState<string | null>(null);

  // Get all projects for "All Projects" mode
  const { data: projects = [] } = useProjects();

  // Single project mode queries
  const { data: environments = [], isLoading: envsLoading } = useProjectEnvironments(selectedProjectId);

  // Local environment selection (from localStorage)
  const {
    selectedEnvironmentId,
    setSelectedEnvironmentId,
    clearSelectedEnvironmentId,
  } = useConsoleEnvironmentFilter(selectedProjectId);

  // For "All Projects" view, get the full selection map
  const environmentSelectionMap = useConsoleEnvironmentSelectionMap();

  // Get the selected project name for single project view
  const selectedProject = projects.find(p => p.id === selectedProjectId);

  // Mutations for single project mode
  const createMutation = useCreateEnvironment(selectedProjectId);
  const updateMutation = useUpdateEnvironment(selectedProjectId);
  const deleteMutation = useDeleteEnvironment(selectedProjectId);

  // CI Token queries and mutations
  const { data: ciTokens = [], isLoading: ciTokensLoading } = useCITokens();
  const createCITokenMutation = useCreateCIToken();
  const revokeCITokenMutation = useRevokeCIToken();

  // "All Projects" mode: fetch environments for all projects in parallel
  const allProjectEnvQueries = useQueries({
    queries: !selectedProjectId
      ? projects.map((project) => ({
          queryKey: consoleKeys.projectEnvironments(project.id),
          queryFn: () => apiGet<ProjectEnvironment[]>(`/api/projects/${project.id}/environments`),
        }))
      : [],
  });

  // Combine all environments with project names for "All Projects" view
  const allProjectsData = useMemo(() => {
    if (selectedProjectId || projects.length === 0) return { environments: [], projectMap: new Map<string, ProjectSummary>() };

    const envs: EnvWithProject[] = [];
    const projectMap = new Map<string, ProjectSummary>();

    projects.forEach((project, index) => {
      projectMap.set(project.id, project);
      const envQuery = allProjectEnvQueries[index];

      if (envQuery?.data) {
        envQuery.data.forEach((env) => {
          envs.push({ ...env, projectName: project.name });
        });
      }
    });

    return { environments: envs, projectMap };
  }, [selectedProjectId, projects, allProjectEnvQueries]);

  const isAllProjectsLoading = !selectedProjectId && allProjectEnvQueries.some((q) => q.isLoading);

  const openCreateModal = () => {
    setEditingEnv(null);
    setShowModal(true);
  };

  const openEditModal = (env: ProjectEnvironment) => {
    setEditingEnv(env);
    setShowModal(true);
  };

  const closeModal = () => {
    setShowModal(false);
    setEditingEnv(null);
  };

  const handleSubmit = async (data: EnvironmentSubmitData) => {
    if (editingEnv) {
      await updateMutation.mutateAsync({
        envId: editingEnv.id,
        data: {
          name: data.name,
          slug: data.slug,
          config_vars: data.configVars,
          env_secrets: Object.keys(data.secrets).length > 0 ? data.secrets : undefined,
        },
      });
    } else {
      await createMutation.mutateAsync({
        name: data.name,
        slug: data.slug,
        config_vars: data.configVars,
        env_secrets: data.secrets,
      });
    }
  };

  const handleDelete = async (envId: string) => {
    try {
      await deleteMutation.mutateAsync(envId);
      setDeleteConfirmId(null);
    } catch (err) {
      console.error('Failed to delete environment:', err);
    }
  };

  const handleSelectEnvironment = (envId: string) => {
    setSelectedEnvironmentId(envId);
  };

  const handleClearEnvironment = () => {
    clearSelectedEnvironmentId();
  };

  // CI Token handlers
  const handleCreateCIToken = async (data: {
    name: string;
    description: string;
    neverExpires: boolean;
    expiresAt: string | null;
    projects: { project_id: string; scope: 'read' | 'write' }[];
  }) => {
    const result = await createCITokenMutation.mutateAsync({
      name: data.name,
      description: data.description || undefined,
      never_expires: data.neverExpires,
      expires_at: data.expiresAt || undefined,
      projects: data.projects,
    });
    setShowCITokenModal(false);
    setRevealedToken({ value: result.token, name: data.name });
    setShowTokenReveal(true);
  };

  const handleRevokeCIToken = async (tokenId: string) => {
    setRevokingTokenId(tokenId);
    try {
      await revokeCITokenMutation.mutateAsync(tokenId);
    } finally {
      setRevokingTokenId(null);
    }
  };

  // Render "All Projects" view
  const renderAllProjectsView = () => {
    if (isAllProjectsLoading) {
      return (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="w-6 h-6 animate-spin text-[#666666]" />
          <span className="ml-3 text-[#666666]">Loading environments...</span>
        </div>
      );
    }

    if (allProjectsData.environments.length === 0) {
      return (
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
          <Settings className="w-12 h-12 text-[#999999] mx-auto mb-4" />
          <h3 className="text-lg font-medium mb-2">No environments configured</h3>
          <p className="text-sm text-[#666666]">
            Select a project above to create and manage environments.
          </p>
        </div>
      );
    }

    const handleAllProjectsSelect = (env: EnvWithProject) => {
      // Use the local filter for the env's project
      const currentFilters = JSON.parse(localStorage.getItem('rocketship.console.filters.v1') || '{}');
      const newFilters = {
        ...currentFilters,
        selectedEnvironmentIdByProjectId: {
          ...(currentFilters.selectedEnvironmentIdByProjectId || {}),
          [env.project_id]: env.id,
        },
      };
      localStorage.setItem('rocketship.console.filters.v1', JSON.stringify(newFilters));
      // Invalidate the filters query to trigger re-render
      queryClient.invalidateQueries({ queryKey: ['consoleFilters'] });
    };

    const handleAllProjectsClear = (projectId: string) => {
      // Use the local filter for the project
      const currentFilters = JSON.parse(localStorage.getItem('rocketship.console.filters.v1') || '{}');
      const { [projectId]: _, ...rest } = currentFilters.selectedEnvironmentIdByProjectId || {};
      const newFilters = {
        ...currentFilters,
        selectedEnvironmentIdByProjectId: rest,
      };
      localStorage.setItem('rocketship.console.filters.v1', JSON.stringify(newFilters));
      // Invalidate the filters query to trigger re-render
      queryClient.invalidateQueries({ queryKey: ['consoleFilters'] });
    };

    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {allProjectsData.environments.map((env) => {
          // Check local selection map for this project
          const isSelected = environmentSelectionMap[env.project_id] === env.id;

          return (
            <EnvironmentCard
              key={env.id}
              id={env.id}
              name={env.name}
              slug={env.slug}
              projectName={env.projectName}
              isSelected={isSelected}
              secretCount={env.env_secrets_keys.length}
              configVarCount={Object.keys(env.config_vars).length}
              onSelect={() => handleAllProjectsSelect(env)}
              onClear={() => handleAllProjectsClear(env.project_id)}
              isSelectPending={false}
              isClearPending={false}
              editDisabled={true}
              editDisabledReason="Select a project to edit environments"
            />
          );
        })}
      </div>
    );
  };

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto space-y-8">
        {/* All Projects View */}
        {!selectedProjectId && (
          <div>
            <div className="flex items-center justify-between mb-4">
              <h2>Environments</h2>
            </div>
            {renderAllProjectsView()}
          </div>
        )}

        {/* Single Project View */}
        {selectedProjectId && (
          <div>
            <div className="flex items-center justify-between mb-4">
              <h2>Environments</h2>
              <button
                onClick={openCreateModal}
                className="flex items-center gap-2 px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors"
              >
                <Plus className="w-4 h-4" />
                <span>New Environment</span>
              </button>
            </div>
            {envsLoading ? (
              <div className="text-center py-8 text-[#666666]">Loading environments...</div>
            ) : environments.length === 0 ? (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-8 text-center">
                <p className="text-[#666666] mb-4">No environments configured for this project.</p>
                <button
                  onClick={openCreateModal}
                  className="text-sm text-black hover:underline"
                >
                  Create your first environment
                </button>
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {environments.map((env) => (
                  <EnvironmentCard
                    key={env.id}
                    id={env.id}
                    name={env.name}
                    slug={env.slug}
                    projectName={selectedProject?.name}
                    isSelected={selectedEnvironmentId === env.id}
                    secretCount={env.env_secrets_keys.length}
                    configVarCount={Object.keys(env.config_vars).length}
                    onEdit={() => openEditModal(env)}
                    onDelete={() => setDeleteConfirmId(env.id)}
                    onSelect={() => handleSelectEnvironment(env.id)}
                    onClear={handleClearEnvironment}
                    isSelectPending={false}
                    isClearPending={false}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        {/* CI Tokens Section */}
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm">
          <div className="flex items-center justify-between p-4 border-b border-[#e5e5e5]">
            <div className="flex items-center gap-2">
              <Key className="w-5 h-5 text-[#666666]" />
              <h3>CI Tokens</h3>
            </div>
            <button
              onClick={() => setShowCITokenModal(true)}
              disabled={projects.length === 0}
              className="flex items-center gap-2 px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              title={projects.length === 0 ? 'Create a project first' : undefined}
            >
              <Plus className="w-4 h-4" />
              <span>New Token</span>
            </button>
          </div>
          <CITokensTable
            tokens={ciTokens}
            isLoading={ciTokensLoading}
            onRevoke={handleRevokeCIToken}
            isRevoking={revokeCITokenMutation.isPending}
            revokingTokenId={revokingTokenId}
          />
        </div>

        {/* Access Control Coming Soon */}
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 opacity-60">
          <h3 className="mb-2">Access Control</h3>
          <p className="text-sm text-[#666666]">
            Coming soon: Manage team member access and permissions.
          </p>
        </div>
      </div>

      {/* Environment Modal (Create/Edit) */}
      <EnvironmentModal
        key={editingEnv?.id ?? 'create'}
        isOpen={showModal}
        onClose={closeModal}
        onSubmit={handleSubmit}
        isSubmitting={createMutation.isPending || updateMutation.isPending}
        existingEnvironment={editingEnv ? {
          name: editingEnv.name,
          slug: editingEnv.slug,
          configVars: editingEnv.config_vars,
          secretKeys: editingEnv.env_secrets_keys,
        } : undefined}
      />

      {/* Delete Confirmation Modal */}
      {deleteConfirmId && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-lg p-6 max-w-md w-full mx-4">
            <h3 className="mb-4">Delete Environment</h3>
            <p className="text-sm text-[#666666] mb-6">
              Are you sure you want to delete this environment? This action cannot be undone.
              All secrets and configuration will be permanently removed.
            </p>
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setDeleteConfirmId(null)}
                className="px-4 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => handleDelete(deleteConfirmId)}
                disabled={deleteMutation.isPending}
                className="px-4 py-2 bg-[#ef0000] text-white rounded-md hover:bg-[#ef0000]/90 transition-colors disabled:opacity-50"
              >
                {deleteMutation.isPending ? 'Deleting...' : 'Delete Environment'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* CI Token Create Modal */}
      <CITokenModal
        isOpen={showCITokenModal}
        onClose={() => setShowCITokenModal(false)}
        onSubmit={handleCreateCIToken}
        isSubmitting={createCITokenMutation.isPending}
        projects={projects}
      />

      {/* CI Token Reveal Modal */}
      <CITokenRevealModal
        isOpen={showTokenReveal}
        onClose={() => {
          setShowTokenReveal(false);
          setRevealedToken({ value: '', name: '' });
        }}
        tokenValue={revealedToken.value}
        tokenName={revealedToken.name}
      />
    </div>
  );
}
