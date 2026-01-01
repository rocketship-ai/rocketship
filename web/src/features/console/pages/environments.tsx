import { Plus, Check, Edit2, Trash2, Lock, Settings } from 'lucide-react';
import { useState } from 'react';
import {
  useProjects,
  useProjectEnvironments,
  useCreateEnvironment,
  useUpdateEnvironment,
  useDeleteEnvironment,
  useProjectEnvironmentSelection,
  useSetProjectEnvironmentSelection,
} from '../hooks/use-console-queries';
import type { ProjectEnvironment } from '../hooks/use-console-queries';

interface NavigationParams {
  env?: string;
  [key: string]: string | undefined;
}

interface EnvironmentsProps {
  onNavigate: (page: string, params?: NavigationParams) => void;
}

interface EnvironmentFormData {
  name: string;
  slug: string;
  description: string;
  configVarsJson: string;
  secrets: { key: string; value: string }[];
}

const emptyFormData: EnvironmentFormData = {
  name: '',
  slug: '',
  description: '',
  configVarsJson: '{}',
  secrets: [],
};

export function Environments({ onNavigate }: EnvironmentsProps) {
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');
  const [showModal, setShowModal] = useState(false);
  const [editingEnv, setEditingEnv] = useState<ProjectEnvironment | null>(null);
  const [formData, setFormData] = useState<EnvironmentFormData>(emptyFormData);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  // Queries
  const { data: projects = [], isLoading: projectsLoading } = useProjects();
  const { data: environments = [], isLoading: envsLoading } = useProjectEnvironments(selectedProjectId);

  // Queries
  const { data: selection } = useProjectEnvironmentSelection(selectedProjectId);

  // Mutations
  const createMutation = useCreateEnvironment(selectedProjectId);
  const updateMutation = useUpdateEnvironment(selectedProjectId);
  const deleteMutation = useDeleteEnvironment(selectedProjectId);
  const selectMutation = useSetProjectEnvironmentSelection(selectedProjectId);

  const openCreateModal = () => {
    setEditingEnv(null);
    setFormData(emptyFormData);
    setFormError(null);
    setShowModal(true);
  };

  const openEditModal = (env: ProjectEnvironment) => {
    setEditingEnv(env);
    setFormData({
      name: env.name,
      slug: env.slug,
      description: env.description || '',
      configVarsJson: JSON.stringify(env.config_vars, null, 2),
      secrets: env.env_secrets_keys.map(key => ({ key, value: '' })),
    });
    setFormError(null);
    setShowModal(true);
  };

  const closeModal = () => {
    setShowModal(false);
    setEditingEnv(null);
    setFormData(emptyFormData);
    setFormError(null);
  };

  const handleSubmit = async () => {
    setFormError(null);

    // Validate
    if (!formData.name.trim()) {
      setFormError('Name is required');
      return;
    }
    if (!formData.slug.trim()) {
      setFormError('Slug is required');
      return;
    }

    // Parse config vars JSON
    let configVars: Record<string, unknown>;
    try {
      configVars = JSON.parse(formData.configVarsJson);
      if (typeof configVars !== 'object' || Array.isArray(configVars)) {
        setFormError('Config vars must be a JSON object');
        return;
      }
    } catch {
      setFormError('Invalid JSON in config vars');
      return;
    }

    // Build env_secrets from form (only include non-empty values)
    const envSecrets: Record<string, string> = {};
    for (const s of formData.secrets) {
      if (s.key.trim() && s.value.trim()) {
        envSecrets[s.key.trim()] = s.value;
      }
    }

    try {
      if (editingEnv) {
        await updateMutation.mutateAsync({
          envId: editingEnv.id,
          data: {
            name: formData.name.trim(),
            slug: formData.slug.trim(),
            description: formData.description.trim() || undefined,
            config_vars: configVars,
            env_secrets: Object.keys(envSecrets).length > 0 ? envSecrets : undefined,
          },
        });
      } else {
        await createMutation.mutateAsync({
          name: formData.name.trim(),
          slug: formData.slug.trim(),
          description: formData.description.trim() || undefined,
          config_vars: configVars,
          env_secrets: envSecrets,
        });
      }
      closeModal();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : 'Failed to save environment');
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

  const handleSelectEnvironment = async (envId: string) => {
    try {
      await selectMutation.mutateAsync(envId);
    } catch (err) {
      console.error('Failed to select environment:', err);
    }
  };

  const addSecretField = () => {
    setFormData(prev => ({
      ...prev,
      secrets: [...prev.secrets, { key: '', value: '' }],
    }));
  };

  const updateSecret = (index: number, field: 'key' | 'value', value: string) => {
    setFormData(prev => ({
      ...prev,
      secrets: prev.secrets.map((s, i) => (i === index ? { ...s, [field]: value } : s)),
    }));
  };

  const removeSecret = (index: number) => {
    setFormData(prev => ({
      ...prev,
      secrets: prev.secrets.filter((_, i) => i !== index),
    }));
  };

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto space-y-8">
        {/* Project Selector */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <label className="text-sm text-[#666666]">Project:</label>
            <select
              value={selectedProjectId}
              onChange={(e) => setSelectedProjectId(e.target.value)}
              className="px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5 min-w-[200px]"
              disabled={projectsLoading}
            >
              <option value="">Select a project...</option>
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
          </div>

          {selectedProjectId && (
            <button
              onClick={openCreateModal}
              className="flex items-center gap-2 px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors"
            >
              <Plus className="w-4 h-4" />
              <span>New Environment</span>
            </button>
          )}
        </div>

        {/* Empty state when no project selected */}
        {!selectedProjectId && (
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
            <Settings className="w-12 h-12 text-[#999999] mx-auto mb-4" />
            <h3 className="text-lg font-medium mb-2">Select a project to manage environments</h3>
            <p className="text-sm text-[#666666]">
              Environments allow you to configure different settings for staging, production, and other deployment targets.
            </p>
          </div>
        )}

        {/* Environments List */}
        {selectedProjectId && (
          <div>
            <h2 className="mb-4">Environments</h2>
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
                {environments.map((env) => {
                  const isSelected = selection?.environment?.id === env.id;
                  return (
                    <div
                      key={env.id}
                      className={`bg-white rounded-lg border shadow-sm p-6 ${isSelected ? 'border-black' : 'border-[#e5e5e5]'}`}
                    >
                      <div className="flex items-start justify-between mb-4">
                        <div>
                          <div className="flex items-center gap-2 mb-1">
                            <h3 className="font-medium">{env.name}</h3>
                            {isSelected && (
                              <span className="text-xs px-2 py-0.5 bg-green-100 text-green-800 rounded flex items-center gap-1">
                                <Check className="w-3 h-3" />
                                Current
                              </span>
                            )}
                          </div>
                          <p className="text-sm text-[#666666]">slug: {env.slug}</p>
                        </div>
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => openEditModal(env)}
                            className="p-1.5 hover:bg-[#f5f5f5] rounded transition-colors"
                            title="Edit"
                          >
                            <Edit2 className="w-4 h-4 text-[#666666]" />
                          </button>
                          <button
                            onClick={() => setDeleteConfirmId(env.id)}
                            className="p-1.5 hover:bg-[#f5f5f5] rounded transition-colors"
                            title="Delete"
                          >
                            <Trash2 className="w-4 h-4 text-[#666666]" />
                          </button>
                        </div>
                      </div>

                      <div className="space-y-2 text-sm mb-4">
                        <div className="flex items-center gap-2">
                          <Lock className="w-4 h-4 text-[#999999]" />
                          <span className="text-[#666666]">
                            {env.env_secrets_keys.length} secret{env.env_secrets_keys.length !== 1 ? 's' : ''}
                          </span>
                        </div>
                        <div className="flex items-center gap-2">
                          <Settings className="w-4 h-4 text-[#999999]" />
                          <span className="text-[#666666]">
                            {Object.keys(env.config_vars).length} config var{Object.keys(env.config_vars).length !== 1 ? 's' : ''}
                          </span>
                        </div>
                      </div>

                      <div className="flex items-center justify-between pt-4 border-t border-[#e5e5e5]">
                        {!isSelected && (
                          <button
                            onClick={() => handleSelectEnvironment(env.id)}
                            className="text-sm text-black hover:underline"
                            disabled={selectMutation.isPending}
                          >
                            Use this environment
                          </button>
                        )}
                        <button
                          onClick={() => onNavigate('suite-activity', { env: env.slug })}
                          className="text-sm text-black hover:underline ml-auto"
                        >
                          View runs
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}

        {/* Coming Soon Sections */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 opacity-60">
            <h3 className="mb-2">CI Tokens</h3>
            <p className="text-sm text-[#666666]">
              Coming soon: Create and manage CI tokens for automated test runs.
            </p>
          </div>
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 opacity-60">
            <h3 className="mb-2">Access Control</h3>
            <p className="text-sm text-[#666666]">
              Coming soon: Manage team member access and permissions.
            </p>
          </div>
        </div>
      </div>

      {/* Create/Edit Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-lg p-6 max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
            <h3 className="mb-6">{editingEnv ? 'Edit Environment' : 'Create Environment'}</h3>

            {formError && (
              <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded text-sm text-red-600">
                {formError}
              </div>
            )}

            <div className="space-y-4">
              {/* Name */}
              <div>
                <label className="block text-sm text-[#666666] mb-2">Name *</label>
                <input
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData(prev => ({ ...prev, name: e.target.value }))}
                  placeholder="e.g., Production"
                  className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
                />
              </div>

              {/* Slug */}
              <div>
                <label className="block text-sm text-[#666666] mb-2">Slug * (lowercase, used in CLI)</label>
                <input
                  type="text"
                  value={formData.slug}
                  onChange={(e) => setFormData(prev => ({ ...prev, slug: e.target.value.toLowerCase().replace(/[^a-z0-9-_]/g, '') }))}
                  placeholder="e.g., production"
                  className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
                />
              </div>

              {/* Description */}
              <div>
                <label className="block text-sm text-[#666666] mb-2">Description</label>
                <input
                  type="text"
                  value={formData.description}
                  onChange={(e) => setFormData(prev => ({ ...prev, description: e.target.value }))}
                  placeholder="Optional description"
                  className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
                />
              </div>

              {/* Config Vars (JSON) */}
              <div>
                <label className="block text-sm text-[#666666] mb-2">
                  Config Variables (JSON) - accessed via {'{{ .vars.* }}'}
                </label>
                <textarea
                  value={formData.configVarsJson}
                  onChange={(e) => setFormData(prev => ({ ...prev, configVarsJson: e.target.value }))}
                  rows={6}
                  className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5 font-mono text-sm"
                  placeholder='{"base_url": "https://api.example.com"}'
                />
              </div>

              {/* Env Secrets */}
              <div>
                <label className="block text-sm text-[#666666] mb-2">
                  Secrets - accessed via {'{{ .env.* }}'}
                </label>
                <div className="space-y-2">
                  {formData.secrets.map((secret, idx) => (
                    <div key={idx} className="flex gap-2">
                      <input
                        type="text"
                        value={secret.key}
                        onChange={(e) => updateSecret(idx, 'key', e.target.value)}
                        placeholder="KEY_NAME"
                        className="flex-1 px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5 font-mono text-sm"
                      />
                      <input
                        type="password"
                        value={secret.value}
                        onChange={(e) => updateSecret(idx, 'value', e.target.value)}
                        placeholder={editingEnv ? '(unchanged)' : 'secret value'}
                        className="flex-1 px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
                      />
                      <button
                        onClick={() => removeSecret(idx)}
                        className="p-2 hover:bg-[#f5f5f5] rounded transition-colors"
                      >
                        <Trash2 className="w-4 h-4 text-[#666666]" />
                      </button>
                    </div>
                  ))}
                  <button
                    onClick={addSecretField}
                    className="text-sm text-black hover:underline flex items-center gap-1"
                  >
                    <Plus className="w-4 h-4" />
                    Add secret
                  </button>
                </div>
                {editingEnv && (
                  <p className="text-xs text-[#999999] mt-2">
                    Existing secret values are not displayed. Enter a new value to update a secret.
                  </p>
                )}
              </div>
            </div>

            <div className="flex gap-2 justify-end mt-6 pt-4 border-t border-[#e5e5e5]">
              <button
                onClick={closeModal}
                className="px-4 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleSubmit}
                disabled={createMutation.isPending || updateMutation.isPending}
                className="px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors disabled:opacity-50"
              >
                {createMutation.isPending || updateMutation.isPending
                  ? 'Saving...'
                  : editingEnv
                  ? 'Update Environment'
                  : 'Create Environment'}
              </button>
            </div>
          </div>
        </div>
      )}

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
    </div>
  );
}
