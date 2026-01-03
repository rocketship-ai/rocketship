import { Plus, Settings, Loader2, Key, Users, Crown, Search, X, ChevronDown, Mail, Clock, Send } from 'lucide-react';
import { useState, useMemo } from 'react';
import { useQueries } from '@tanstack/react-query';
import {
  useProjects,
  useProjectEnvironments,
  useCreateEnvironment,
  useUpdateEnvironment,
  useDeleteEnvironment,
  useCreateEnvironmentForProject,
  useUpdateEnvironmentForProject,
  useDeleteEnvironmentForProject,
  useCITokens,
  useCreateCIToken,
  useRevokeCIToken,
  useProfile,
  useOrgOwners,
  useAllProjectMembers,
  useProjectMembers,
  useUpdateProjectMemberRoleForProject,
  useRemoveProjectMemberForProject,
  useProjectInvites,
  useCreateProjectInvite,
  useRevokeProjectInvite,
  consoleKeys,
} from '../hooks/use-console-queries';
import {
  useConsoleProjectFilter,
  useConsoleEnvironmentFilter,
  useConsoleEnvironmentSelectionMap,
} from '../hooks/use-console-filters';
import type { ProjectEnvironment, ProjectSummary, ProjectMember } from '../hooks/use-console-queries';
import { EnvironmentCard } from '../components/environment-card';
import { EnvironmentModal, type EnvironmentSubmitData } from '../components/environment-modal';
import { CITokenModal } from '../components/ci-token-modal';
import { CITokenRevealModal } from '../components/ci-token-reveal-modal';
import { CITokensTable } from '../components/ci-tokens-table';
import { ConfirmDialog } from '../components/confirm-dialog';
import { Button } from '../components/ui';
import { apiGet } from '@/lib/api';

// Helper type for All Projects view
interface EnvWithProject extends ProjectEnvironment {
  projectName: string;
}

// Count leaf values in a nested object (actual config vars users reference)
function countConfigVarLeaves(obj: Record<string, unknown>): number {
  let count = 0;
  for (const value of Object.values(obj)) {
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      count += countConfigVarLeaves(value as Record<string, unknown>);
    } else {
      count += 1;
    }
  }
  return count;
}

export function Environments() {
  // Global project filter - derive effective project from it
  const { selectedProjectIds } = useConsoleProjectFilter();
  // If exactly one project is selected, use single-project mode; otherwise All Projects
  const selectedProjectId = selectedProjectIds.length === 1 ? selectedProjectIds[0] : '';
  const [showModal, setShowModal] = useState(false);
  const [editingEnv, setEditingEnv] = useState<EnvWithProject | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<{ envId: string; projectId: string } | null>(null);

  // CI Token state
  const [showCITokenModal, setShowCITokenModal] = useState(false);
  const [showTokenReveal, setShowTokenReveal] = useState(false);
  const [revealedToken, setRevealedToken] = useState({ value: '', name: '' });
  const [revokingTokenId, setRevokingTokenId] = useState<string | null>(null);
  const [revokeConfirmTokenId, setRevokeConfirmTokenId] = useState<string | null>(null);

  // Access Control state
  const [memberSearchTerm, setMemberSearchTerm] = useState('');
  const [showInviteModal, setShowInviteModal] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteProjects, setInviteProjects] = useState<{ projectId: string; role: 'read' | 'write' }[]>([]);
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [removeMemberConfirm, setRemoveMemberConfirm] = useState<{ projectId: string; userId: string; username: string } | null>(null);
  const [revokeInviteConfirm, setRevokeInviteConfirm] = useState<string | null>(null);

  // Get all projects for "All Projects" mode
  const { data: projects = [] } = useProjects();

  // Apply the global project filter to this page's data.
  // Empty selection means "All Projects"; otherwise restrict to the selected subset.
  const filteredProjects = useMemo(() => {
    if (selectedProjectIds.length === 0) return projects;
    const selected = new Set(selectedProjectIds);
    return projects.filter((p) => selected.has(p.id));
  }, [projects, selectedProjectIds]);

  // Single project mode queries
  const { data: environments = [], isLoading: envsLoading } = useProjectEnvironments(selectedProjectId);

  // Local environment selection (from localStorage)
  const { selectedEnvironmentId } = useConsoleEnvironmentFilter(selectedProjectId);

  // For "All Projects" view, get the full selection map
  const environmentSelectionMap = useConsoleEnvironmentSelectionMap();

  // Get the selected project name for single project view
  const selectedProject = projects.find(p => p.id === selectedProjectId);

  // Mutations for single project mode
  const createMutation = useCreateEnvironment(selectedProjectId);
  const updateMutation = useUpdateEnvironment(selectedProjectId);
  const deleteMutation = useDeleteEnvironment(selectedProjectId);

  // Mutations for "All Projects" mode (project-aware)
  const createForProjectMutation = useCreateEnvironmentForProject();
  const updateForProjectMutation = useUpdateEnvironmentForProject();
  const deleteForProjectMutation = useDeleteEnvironmentForProject();

  // CI Token queries and mutations
  const { data: ciTokens = [], isLoading: ciTokensLoading, error: ciTokensError } = useCITokens();
  const createCITokenMutation = useCreateCIToken();
  const revokeCITokenMutation = useRevokeCIToken();

  // Access Control queries and mutations
  const { data: profile } = useProfile();
  const orgId = profile?.organization?.id ?? '';
  const isOrgOwner = profile?.organization?.role === 'admin';

  // Compute which projects the current user has write access to
  const writeProjectIds = useMemo(() => {
    if (!profile?.project_permissions) return new Set<string>();
    return new Set(
      profile.project_permissions
        .filter((p) => p.permissions.includes('write'))
        .map((p) => p.project_id)
    );
  }, [profile?.project_permissions]);

  // Non-owners with write access to at least one project can invite to those projects
  const canInvite = isOrgOwner || writeProjectIds.size > 0;

  const { data: orgOwners = [], isLoading: ownersLoading } = useOrgOwners(orgId);
  // For owners: use org-wide endpoint; for non-owners in single project mode: use per-project endpoint
  // Only call owner-only endpoints when user is an org owner to avoid 403 errors
  const { data: allProjectMembers = [], isLoading: allMembersLoading } = useAllProjectMembers(orgId, { enabled: isOrgOwner });
  const { data: projectMembers = [], isLoading: projectMembersLoading } = useProjectMembers(selectedProjectId);
  // Allow non-owners with write access to see their sent invites
  const { data: projectInvites = [], isLoading: invitesLoading, error: invitesError } = useProjectInvites({ enabled: canInvite });

  // For non-owners in "All Projects" mode: fetch members from each accessible project in parallel
  const allProjectMemberQueries = useQueries({
    queries: !isOrgOwner && !selectedProjectId && projects.length > 0
      ? filteredProjects.map((project) => ({
          queryKey: [...consoleKeys.all, 'project', project.id, 'members'] as const,
          queryFn: () => apiGet<ProjectMember[]>(`/api/projects/${project.id}/members`),
        }))
      : [],
  });

  // Determine which members data to use based on user role and selection
  const membersData = useMemo(() => {
    if (isOrgOwner) {
      // Owners use org-wide endpoint - apply project filter if set.
      if (selectedProjectId) {
        return allProjectMembers.filter((m) => m.project_id === selectedProjectId);
      }
      if (selectedProjectIds.length > 0) {
        const selected = new Set(selectedProjectIds);
        return allProjectMembers.filter((m) => selected.has(m.project_id));
      }
      return allProjectMembers;
    }
    // Non-owners in single project mode: use per-project endpoint
    if (selectedProjectId) {
      return projectMembers.map((m) => ({
        ...m,
        project_id: selectedProjectId,
        project_name: selectedProject?.name ?? '',
      }));
    }
    // Non-owners in all projects mode: aggregate members from all accessible projects
    const aggregated: { project_id: string; project_name: string; user_id: string; username: string; email: string; name: string; role: 'read' | 'write' }[] = [];
    filteredProjects.forEach((project, index) => {
      const query = allProjectMemberQueries[index];
      if (query?.data) {
        query.data.forEach((m) => {
          aggregated.push({
            ...m,
            project_id: project.id,
            project_name: project.name,
          });
        });
      }
    });
    return aggregated;
  }, [isOrgOwner, selectedProjectId, selectedProjectIds, allProjectMembers, projectMembers, selectedProject, filteredProjects, allProjectMemberQueries]);

  // Loading state: for non-owners in All Projects mode, check if any aggregated query is loading
  const aggregatedMembersLoading = !isOrgOwner && !selectedProjectId && allProjectMemberQueries.some((q) => q.isLoading);
  const membersLoading = isOrgOwner ? allMembersLoading : (selectedProjectId ? projectMembersLoading : aggregatedMembersLoading);
  // Track if any aggregated member queries failed (for error banner)
  const aggregatedMembersErrors = !isOrgOwner && !selectedProjectId
    ? allProjectMemberQueries.filter((q) => q.isError).length
    : 0;
  const createInviteMutation = useCreateProjectInvite();
  const revokeInviteMutation = useRevokeProjectInvite();
  const updateRoleMutation = useUpdateProjectMemberRoleForProject();
  const removeMemberMutation = useRemoveProjectMemberForProject();

  // "All Projects" mode: fetch environments for all projects in parallel
  const allProjectEnvQueries = useQueries({
    queries: !selectedProjectId
      ? filteredProjects.map((project) => ({
          queryKey: consoleKeys.projectEnvironments(project.id),
          queryFn: () => apiGet<ProjectEnvironment[]>(`/api/projects/${project.id}/environments`),
        }))
      : [],
  });

  // Combine all environments with project names for "All Projects" view
  const allProjectsData = useMemo(() => {
    if (selectedProjectId || filteredProjects.length === 0) {
      return { environments: [], projectMap: new Map<string, ProjectSummary>() };
    }

    const envs: EnvWithProject[] = [];
    const projectMap = new Map<string, ProjectSummary>();

    filteredProjects.forEach((project, index) => {
      projectMap.set(project.id, project);
      const envQuery = allProjectEnvQueries[index];

      if (envQuery?.data) {
        envQuery.data.forEach((env) => {
          envs.push({ ...env, projectName: project.name });
        });
      }
    });

    return { environments: envs, projectMap };
  }, [selectedProjectId, filteredProjects, allProjectEnvQueries]);

  const isAllProjectsLoading = !selectedProjectId && allProjectEnvQueries.some((q) => q.isLoading);

  const openCreateModal = () => {
    setEditingEnv(null);
    setShowModal(true);
  };

  const openEditModal = (env: EnvWithProject) => {
    setEditingEnv(env);
    setShowModal(true);
  };

  const closeModal = () => {
    setShowModal(false);
    setEditingEnv(null);
  };

  const handleSubmit = async (data: EnvironmentSubmitData) => {
    const envData = {
      name: data.name,
      slug: data.slug,
      config_vars: data.configVars,
      env_secrets: Object.keys(data.secrets).length > 0 ? data.secrets : undefined,
    };

    if (editingEnv) {
      // Update: use project-aware mutation for All Projects mode
      if (selectedProjectId) {
        await updateMutation.mutateAsync({
          envId: editingEnv.id,
          data: envData,
        });
      } else {
        await updateForProjectMutation.mutateAsync({
          projectId: data.projectId,
          envId: editingEnv.id,
          data: envData,
        });
      }
    } else {
      // Create: use project-aware mutation for All Projects mode
      if (selectedProjectId) {
        await createMutation.mutateAsync({
          name: data.name,
          slug: data.slug,
          config_vars: data.configVars,
          env_secrets: data.secrets,
        });
      } else {
        await createForProjectMutation.mutateAsync({
          projectId: data.projectId,
          data: {
            name: data.name,
            slug: data.slug,
            config_vars: data.configVars,
            env_secrets: data.secrets,
          },
        });
      }
    }
  };

  const handleDelete = async () => {
    if (!deleteConfirm) return;
    try {
      if (selectedProjectId) {
        await deleteMutation.mutateAsync(deleteConfirm.envId);
      } else {
        await deleteForProjectMutation.mutateAsync({
          projectId: deleteConfirm.projectId,
          envId: deleteConfirm.envId,
        });
      }
      setDeleteConfirm(null);
    } catch (err) {
      console.error('Failed to delete environment:', err);
    }
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

  // Opens the revoke confirmation dialog
  const handleRevokeCIToken = (tokenId: string) => {
    setRevokeConfirmTokenId(tokenId);
  };

  // Actually performs the revoke after confirmation
  const confirmRevokeCIToken = async () => {
    if (!revokeConfirmTokenId) return;
    setRevokingTokenId(revokeConfirmTokenId);
    try {
      await revokeCITokenMutation.mutateAsync(revokeConfirmTokenId);
      setRevokeConfirmTokenId(null);
    } finally {
      setRevokingTokenId(null);
    }
  };

  // Access Control handlers
  const filteredMembers = useMemo(() => {
    // Use the determined members data (already filtered by project if needed)
    const members = membersData;
    // Apply search filter
    if (!memberSearchTerm.trim()) return members;
    const term = memberSearchTerm.toLowerCase();
    return members.filter(
      (m) =>
        m.username.toLowerCase().includes(term) ||
        m.email.toLowerCase().includes(term) ||
        m.name.toLowerCase().includes(term) ||
        m.project_name.toLowerCase().includes(term)
    );
  }, [membersData, memberSearchTerm]);

  // Pending invites (not accepted, not revoked, not expired) - filtered by project if single project mode
  const pendingInvites = useMemo(() => {
    const pending = projectInvites.filter((inv) => inv.status === 'pending');
    if (selectedProjectIds.length === 0) return pending;
    if (selectedProjectId) {
      return pending.filter((inv) => inv.projects.some((p) => p.project_id === selectedProjectId));
    }
    const selected = new Set(selectedProjectIds);
    return pending.filter((inv) => inv.projects.some((p) => selected.has(p.project_id)));
  }, [projectInvites, selectedProjectId, selectedProjectIds]);

  // Projects the current user can invite to: org owners can invite to all, others only to projects with write access
  const invitableProjects = useMemo(() => {
    if (isOrgOwner) return projects;
    return projects.filter((p) => writeProjectIds.has(p.id));
  }, [isOrgOwner, projects, writeProjectIds]);

  const handleInviteMember = async () => {
    if (!inviteEmail.trim() || inviteProjects.length === 0) {
      setInviteError('Email and at least one project are required');
      return;
    }
    if (!inviteEmail.includes('@')) {
      setInviteError('Please enter a valid email address');
      return;
    }
    setInviteError(null);
    try {
      await createInviteMutation.mutateAsync({
        email: inviteEmail.trim(),
        projects: inviteProjects.map((p) => ({ project_id: p.projectId, role: p.role })),
      });
      // Reset form and close modal
      setInviteEmail('');
      setInviteProjects([]);
      setShowInviteModal(false);
    } catch (err) {
      if (err instanceof Error) {
        setInviteError(err.message);
      } else {
        setInviteError('Failed to send invite');
      }
    }
  };

  const handleRevokeInvite = async () => {
    if (!revokeInviteConfirm) return;
    try {
      await revokeInviteMutation.mutateAsync(revokeInviteConfirm);
      setRevokeInviteConfirm(null);
    } catch (err) {
      console.error('Failed to revoke invite:', err);
    }
  };

  const toggleInviteProject = (projectId: string) => {
    setInviteProjects((prev) => {
      const existing = prev.find((p) => p.projectId === projectId);
      if (existing) {
        return prev.filter((p) => p.projectId !== projectId);
      }
      return [...prev, { projectId, role: 'read' as const }];
    });
  };

  const setInviteProjectRole = (projectId: string, role: 'read' | 'write') => {
    setInviteProjects((prev) =>
      prev.map((p) => (p.projectId === projectId ? { ...p, role } : p))
    );
  };

  const handleUpdateRole = async (projectId: string, userId: string, role: 'read' | 'write') => {
    try {
      await updateRoleMutation.mutateAsync({ projectId, userId, role });
    } catch (err) {
      console.error('Failed to update role:', err);
    }
  };

  const handleRemoveMember = async () => {
    if (!removeMemberConfirm) return;
    try {
      await removeMemberMutation.mutateAsync({
        projectId: removeMemberConfirm.projectId,
        userId: removeMemberConfirm.userId,
      });
      setRemoveMemberConfirm(null);
    } catch (err) {
      console.error('Failed to remove member:', err);
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
          <p className="text-sm text-[#666666] mb-4">
            Create your first environment to get started.
          </p>
          <button
            onClick={openCreateModal}
            className="text-sm text-black hover:underline"
          >
            Create your first environment
          </button>
        </div>
      );
    }

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
              configVarCount={countConfigVarLeaves(env.config_vars)}
              onEdit={() => openEditModal(env)}
              onDelete={() => setDeleteConfirm({ envId: env.id, projectId: env.project_id })}
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
              <Button
                onClick={openCreateModal}
                disabled={projects.length === 0}
                leftIcon={<Plus className="w-4 h-4" />}
                title={projects.length === 0 ? 'Create a project first' : undefined}
              >
                New Environment
              </Button>
            </div>
            {renderAllProjectsView()}
          </div>
        )}

        {/* Single Project View */}
        {selectedProjectId && (
          <div>
            <div className="flex items-center justify-between mb-4">
              <h2>Environments</h2>
              <Button
                onClick={openCreateModal}
                leftIcon={<Plus className="w-4 h-4" />}
              >
                New Environment
              </Button>
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
                    configVarCount={countConfigVarLeaves(env.config_vars)}
                    onEdit={() => openEditModal({ ...env, projectName: selectedProject?.name ?? '' })}
                    onDelete={() => setDeleteConfirm({ envId: env.id, projectId: env.project_id })}
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
            <Button
              onClick={() => setShowCITokenModal(true)}
              disabled={projects.length === 0}
              leftIcon={<Plus className="w-4 h-4" />}
              title={projects.length === 0 ? 'Create a project first' : undefined}
            >
              New Token
            </Button>
          </div>
          <CITokensTable
            tokens={ciTokens}
            isLoading={ciTokensLoading}
            error={ciTokensError instanceof Error ? ciTokensError.message : null}
            onRevoke={handleRevokeCIToken}
            isRevoking={revokeCITokenMutation.isPending}
            revokingTokenId={revokingTokenId}
          />
        </div>

        {/* Access Control Section */}
        <div className="space-y-6">
          {/* Organization Owners */}
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm">
            <div className="flex items-center gap-2 p-4 border-b border-[#e5e5e5]">
              <Crown className="w-5 h-5 text-amber-500" />
              <h3>Organization Owners</h3>
            </div>
            <div className="p-4">
              {ownersLoading ? (
                <div className="flex items-center justify-center py-4">
                  <Loader2 className="w-5 h-5 animate-spin text-[#666666]" />
                  <span className="ml-2 text-sm text-[#666666]">Loading owners...</span>
                </div>
              ) : orgOwners.length === 0 ? (
                <p className="text-sm text-[#666666] text-center py-4">No owners found.</p>
              ) : (
                <div className="space-y-2">
                  {orgOwners.map((owner) => (
                    <div
                      key={owner.user_id}
                      className="flex items-center justify-between py-2 px-3 bg-[#fafafa] rounded-md"
                    >
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-full bg-amber-100 flex items-center justify-center">
                          <span className="text-amber-700 text-sm font-medium">
                            {(owner.name || owner.username || owner.email)[0].toUpperCase()}
                          </span>
                        </div>
                        <div>
                          <p className="text-sm font-medium">{owner.name || owner.username}</p>
                          <p className="text-xs text-[#666666]">{owner.email}</p>
                        </div>
                      </div>
                      <span className="text-xs text-amber-600 bg-amber-50 px-2 py-1 rounded-full">
                        Owner
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Project Members */}
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm">
            <div className="flex items-center justify-between p-4 border-b border-[#e5e5e5]">
              <div className="flex items-center gap-2">
                <Users className="w-5 h-5 text-[#666666]" />
                <h3>Project Members</h3>
              </div>
              {canInvite && (
                <Button
                  onClick={() => {
                    setShowInviteModal(true);
                    setInviteError(null);
                    setInviteEmail('');
                    setInviteProjects([]);
                  }}
                  disabled={projects.length === 0}
                  leftIcon={<Mail className="w-4 h-4" />}
                >
                  Invite Member
                </Button>
              )}
            </div>

            {/* Pending Invites - visible to anyone who can invite */}
            {canInvite && invitesError ? (
              <div className="p-4 border-b border-[#e5e5e5] bg-red-50">
                <div className="flex items-center gap-2">
                  <X className="w-4 h-4 text-red-600" />
                  <span className="text-sm text-red-700">
                    Failed to load invites: {invitesError instanceof Error ? invitesError.message : 'Unknown error'}
                  </span>
                </div>
              </div>
            ) : canInvite && invitesLoading ? (
              <div className="p-4 border-b border-[#e5e5e5] bg-amber-50">
                <div className="flex items-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin text-amber-600" />
                  <span className="text-sm text-amber-700">Loading pending invites...</span>
                </div>
              </div>
            ) : canInvite && pendingInvites.length > 0 ? (
              <div className="p-4 border-b border-[#e5e5e5] bg-amber-50">
                <div className="flex items-center gap-2 mb-3">
                  <Clock className="w-4 h-4 text-amber-600" />
                  <span className="text-sm font-medium text-amber-700">
                    {pendingInvites.length} Pending Invite{pendingInvites.length > 1 ? 's' : ''}
                  </span>
                </div>
                <div className="space-y-2">
                  {pendingInvites.map((invite) => (
                    <div
                      key={invite.id}
                      className="flex items-center justify-between p-3 bg-white rounded-md border border-amber-200"
                    >
                      <div>
                        <p className="text-sm font-medium">{invite.email}</p>
                        <p className="text-xs text-[#666666]">
                          Invited by {invite.inviter_name} · Expires{' '}
                          {new Date(invite.expires_at).toLocaleDateString()}
                        </p>
                        <div className="flex flex-wrap gap-1 mt-1">
                          {invite.projects.map((p) => (
                            <span
                              key={p.project_id}
                              className={`text-xs px-2 py-0.5 rounded-full ${
                                p.role === 'write'
                                  ? 'bg-green-50 text-green-700'
                                  : 'bg-blue-50 text-blue-700'
                              }`}
                            >
                              {p.project_name} ({p.role})
                            </span>
                          ))}
                        </div>
                      </div>
                      <button
                        onClick={() => setRevokeInviteConfirm(invite.id)}
                        className="text-xs text-red-600 hover:text-red-700 hover:underline"
                      >
                        Revoke
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}

            {/* Search */}
            <div className="p-4 border-b border-[#e5e5e5]">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#999999]" />
                <input
                  type="text"
                  value={memberSearchTerm}
                  onChange={(e) => setMemberSearchTerm(e.target.value)}
                  placeholder="Search by username, email, or project..."
                  className="w-full pl-10 pr-4 py-2 text-sm border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-1 focus:ring-black"
                />
                {memberSearchTerm && (
                  <button
                    onClick={() => setMemberSearchTerm('')}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-[#999999] hover:text-[#666666]"
                  >
                    <X className="w-4 h-4" />
                  </button>
                )}
              </div>
            </div>

            {/* Error banner for aggregated member query failures */}
            {aggregatedMembersErrors > 0 && (
              <div className="p-4 border-b border-[#e5e5e5] bg-red-50">
                <div className="flex items-center gap-2">
                  <X className="w-4 h-4 text-red-600" />
                  <span className="text-sm text-red-700">
                    Failed to load members for {aggregatedMembersErrors} project{aggregatedMembersErrors > 1 ? 's' : ''}.
                  </span>
                </div>
              </div>
            )}

            {/* Members Table */}
            <div className="overflow-x-auto">
              {membersLoading ? (
                <div className="flex items-center justify-center py-8">
                  <Loader2 className="w-5 h-5 animate-spin text-[#666666]" />
                  <span className="ml-2 text-sm text-[#666666]">Loading members...</span>
                </div>
              ) : filteredMembers.length === 0 ? (
                <p className="text-sm text-[#666666] text-center py-8">
                  {memberSearchTerm ? 'No members match your search.' : 'No project members yet.'}
                </p>
              ) : (
                <table className="w-full">
                  <thead>
                    <tr className="text-xs text-[#666666] border-b border-[#e5e5e5]">
                      <th className="text-left py-3 px-4 font-medium">User</th>
                      <th className="text-left py-3 px-4 font-medium">Project</th>
                      <th className="text-left py-3 px-4 font-medium">Role</th>
                      {isOrgOwner && (
                        <th className="text-right py-3 px-4 font-medium">Actions</th>
                      )}
                    </tr>
                  </thead>
                  <tbody>
                    {filteredMembers.map((member) => (
                      <tr
                        key={`${member.project_id}-${member.user_id}`}
                        className="border-b border-[#e5e5e5] last:border-b-0 hover:bg-[#fafafa]"
                      >
                        <td className="py-3 px-4">
                          <div className="flex items-center gap-3">
                            <div className="w-8 h-8 rounded-full bg-[#e5e5e5] flex items-center justify-center">
                              <span className="text-[#666666] text-sm font-medium">
                                {(member.name || member.username || member.email)[0].toUpperCase()}
                              </span>
                            </div>
                            <div>
                              <p className="text-sm font-medium">{member.name || member.username}</p>
                              <p className="text-xs text-[#666666]">{member.email}</p>
                            </div>
                          </div>
                        </td>
                        <td className="py-3 px-4">
                          <span className="text-sm">{member.project_name}</span>
                        </td>
                        <td className="py-3 px-4">
                          {isOrgOwner ? (
                            <div className="relative inline-block">
                              <select
                                value={member.role}
                                onChange={(e) =>
                                  handleUpdateRole(
                                    member.project_id,
                                    member.user_id,
                                    e.target.value as 'read' | 'write'
                                  )
                                }
                                disabled={updateRoleMutation.isPending}
                                className={`appearance-none text-sm px-2 py-1 pr-7 border rounded-md focus:outline-none focus:ring-1 focus:ring-black ${
                                  member.role === 'write'
                                    ? 'bg-green-50 border-green-200 text-green-700'
                                    : 'bg-blue-50 border-blue-200 text-blue-700'
                                }`}
                              >
                                <option value="read">Read</option>
                                <option value="write">Write</option>
                              </select>
                              <ChevronDown className="absolute right-2 top-1/2 -translate-y-1/2 w-3 h-3 pointer-events-none text-[#666666]" />
                            </div>
                          ) : (
                            <span
                              className={`text-sm px-2 py-1 rounded-full ${
                                member.role === 'write'
                                  ? 'bg-green-50 text-green-700'
                                  : 'bg-blue-50 text-blue-700'
                              }`}
                            >
                              {member.role === 'write' ? 'Write' : 'Read'}
                            </span>
                          )}
                        </td>
                        <td className="py-3 px-4 text-right">
                          {isOrgOwner && (
                            <button
                              onClick={() =>
                                setRemoveMemberConfirm({
                                  projectId: member.project_id,
                                  userId: member.user_id,
                                  username: member.username || member.email,
                                })
                              }
                              className="text-xs text-red-600 hover:text-red-700 hover:underline"
                            >
                              Remove
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Environment Modal (Create/Edit) */}
      <EnvironmentModal
        key={editingEnv?.id ?? 'create'}
        isOpen={showModal}
        onClose={closeModal}
        onSubmit={handleSubmit}
        isSubmitting={
          createMutation.isPending ||
          updateMutation.isPending ||
          createForProjectMutation.isPending ||
          updateForProjectMutation.isPending
        }
        projects={projects.map((p) => ({ id: p.id, name: p.name }))}
        selectedProjectId={selectedProjectId || undefined}
        existingEnvironment={editingEnv ? {
          name: editingEnv.name,
          slug: editingEnv.slug,
          projectId: editingEnv.project_id,
          projectName: editingEnv.projectName,
          configVars: editingEnv.config_vars,
          secretKeys: editingEnv.env_secrets_keys,
        } : undefined}
      />

      {/* Delete Confirmation Modal */}
      <ConfirmDialog
        isOpen={!!deleteConfirm}
        title="Delete Environment"
        message="Are you sure you want to delete this environment? This action cannot be undone. All secrets and configuration will be permanently removed."
        confirmLabel="Delete Environment"
        confirmVariant="danger"
        isConfirming={deleteMutation.isPending || deleteForProjectMutation.isPending}
        onCancel={() => setDeleteConfirm(null)}
        onConfirm={handleDelete}
      />

      {/* Revoke CI Token Confirmation Modal */}
      <ConfirmDialog
        isOpen={!!revokeConfirmTokenId}
        title="Revoke CI Token"
        message="Are you sure you want to revoke this CI token? CI jobs using it will fail immediately."
        confirmLabel="Revoke Token"
        confirmVariant="danger"
        isConfirming={revokeCITokenMutation.isPending && revokingTokenId === revokeConfirmTokenId}
        onCancel={() => setRevokeConfirmTokenId(null)}
        onConfirm={confirmRevokeCIToken}
      />

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

      {/* Remove Project Member Confirmation Modal */}
      <ConfirmDialog
        isOpen={!!removeMemberConfirm}
        title="Remove Project Member"
        message={`Are you sure you want to remove ${removeMemberConfirm?.username} from this project? They will lose access immediately.`}
        confirmLabel="Remove Member"
        confirmVariant="danger"
        isConfirming={removeMemberMutation.isPending}
        onCancel={() => setRemoveMemberConfirm(null)}
        onConfirm={handleRemoveMember}
      />

      {/* Revoke Invite Confirmation Modal */}
      <ConfirmDialog
        isOpen={!!revokeInviteConfirm}
        title="Revoke Invite"
        message="Are you sure you want to revoke this invite? The recipient will no longer be able to join using this invite code."
        confirmLabel="Revoke Invite"
        confirmVariant="danger"
        isConfirming={revokeInviteMutation.isPending}
        onCancel={() => setRevokeInviteConfirm(null)}
        onConfirm={handleRevokeInvite}
      />

      {/* Invite Member Modal */}
      {showInviteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div
            className="absolute inset-0 bg-black/50"
            onClick={() => setShowInviteModal(false)}
          />
          <div className="relative bg-white rounded-lg shadow-xl w-full max-w-lg mx-4">
            <div className="flex items-center justify-between p-4 border-b border-[#e5e5e5]">
              <div className="flex items-center gap-2">
                <Send className="w-5 h-5 text-[#666666]" />
                <h3 className="font-semibold">Invite Member</h3>
              </div>
              <button
                onClick={() => setShowInviteModal(false)}
                className="text-[#666666] hover:text-black"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="p-4 space-y-4">
              {/* Email Input */}
              <div>
                <label className="block text-sm font-medium mb-1">Email Address</label>
                <input
                  type="email"
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  placeholder="colleague@example.com"
                  className="w-full px-3 py-2 border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/20"
                />
              </div>

              {/* Project Selection */}
              <div>
                <label className="block text-sm font-medium mb-2">
                  Select Projects & Roles
                </label>
                {!isOrgOwner && (
                  <p className="text-xs text-[#666666] mb-2">
                    You can only invite to projects where you have Write access.
                  </p>
                )}
                <div className="border border-[#e5e5e5] rounded-md max-h-48 overflow-y-auto">
                  {invitableProjects.map((project) => {
                    const selected = inviteProjects.find((p) => p.projectId === project.id);
                    return (
                      <div
                        key={project.id}
                        className={`flex items-center justify-between p-3 border-b border-[#e5e5e5] last:border-b-0 ${
                          selected ? 'bg-blue-50' : 'hover:bg-[#fafafa]'
                        }`}
                      >
                        <label className="flex items-center gap-2 cursor-pointer flex-1">
                          <input
                            type="checkbox"
                            checked={!!selected}
                            onChange={() => toggleInviteProject(project.id)}
                            className="w-4 h-4 rounded border-[#e5e5e5] text-black focus:ring-black"
                          />
                          <span className="text-sm">{project.name}</span>
                        </label>
                        {selected && (
                          <select
                            value={selected.role}
                            onChange={(e) =>
                              setInviteProjectRole(project.id, e.target.value as 'read' | 'write')
                            }
                            className="text-sm px-2 py-1 border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-1 focus:ring-black"
                          >
                            <option value="read">Read</option>
                            <option value="write">Write</option>
                          </select>
                        )}
                      </div>
                    );
                  })}
                </div>
                {invitableProjects.length === 0 && (
                  <p className="text-sm text-[#666666] text-center py-4">
                    {isOrgOwner
                      ? 'No projects available. Create a project first.'
                      : 'You don\'t have Write access to any projects.'}
                  </p>
                )}
              </div>

              {inviteError && (
                <div className="text-sm text-red-600 bg-red-50 px-3 py-2 rounded-md">
                  {inviteError}
                </div>
              )}
            </div>

            <div className="flex justify-end gap-3 p-4 border-t border-[#e5e5e5]">
              <button
                onClick={() => setShowInviteModal(false)}
                className="px-4 py-2 text-sm border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleInviteMember}
                disabled={createInviteMutation.isPending || !inviteEmail.trim() || inviteProjects.length === 0}
                className="flex items-center gap-2 px-4 py-2 text-sm bg-black text-white rounded-md hover:bg-black/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {createInviteMutation.isPending ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Sending...
                  </>
                ) : (
                  <>
                    <Send className="w-4 h-4" />
                    Send Invite
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
