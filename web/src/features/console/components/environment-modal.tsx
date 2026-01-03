import { Plus, Trash2 } from 'lucide-react';
import { useState, useMemo } from 'react';
import * as yaml from 'js-yaml';

interface SecretField {
  key: string;
  value: string;
  isExisting: boolean; // true if this secret already exists in the environment
}

interface ProjectOption {
  id: string;
  name: string;
}

interface EnvironmentModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (data: EnvironmentSubmitData) => Promise<void>;
  isSubmitting: boolean;
  // Projects list for project selection (All Projects mode)
  projects?: ProjectOption[];
  // Pre-selected project ID (locked when exactly 1 project is globally selected, or for edit mode)
  selectedProjectId?: string;
  // For edit mode - pass the existing environment data
  existingEnvironment?: {
    name: string;
    slug: string;
    projectId: string;
    projectName?: string;
    configVars: Record<string, unknown>;
    secretKeys: string[];
  };
}

export interface EnvironmentSubmitData {
  name: string;
  slug: string;
  projectId: string;
  configVars: Record<string, unknown>;
  secrets: Record<string, string>;
}

// Helper to convert name to slug: lowercase, spaces to dashes
function nameToSlug(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9-_]/g, '');
}

// Convert config vars object to YAML string
function configVarsToYaml(vars: Record<string, unknown>): string {
  if (!vars || Object.keys(vars).length === 0) return '';
  try {
    return yaml.dump(vars, { indent: 2, lineWidth: -1 });
  } catch {
    return JSON.stringify(vars, null, 2);
  }
}

export function EnvironmentModal({
  isOpen,
  onClose,
  onSubmit,
  isSubmitting,
  projects = [],
  selectedProjectId,
  existingEnvironment,
}: EnvironmentModalProps) {
  const isEditMode = !!existingEnvironment;

  // Form state
  const [name, setName] = useState(existingEnvironment?.name ?? '');
  const [configVarsYaml, setConfigVarsYaml] = useState(
    existingEnvironment ? configVarsToYaml(existingEnvironment.configVars) : ''
  );
  const [secrets, setSecrets] = useState<SecretField[]>(
    existingEnvironment?.secretKeys.map(key => ({ key, value: '', isExisting: true })) ?? []
  );
  const [error, setError] = useState<string | null>(null);

  // Project selection state - defaults to locked project ID or existing env's project, or empty
  const [projectId, setProjectId] = useState(
    selectedProjectId ?? existingEnvironment?.projectId ?? ''
  );

  // Determine if project selection should be locked
  // Locked when: single project selected globally, or in edit mode (can't move env between projects)
  const isProjectLocked = !!selectedProjectId || isEditMode;

  // Compute slug from name in real-time
  const slug = useMemo(() => nameToSlug(name), [name]);

  // Get the display name for the locked project
  const lockedProjectName = useMemo(() => {
    if (!isProjectLocked) return undefined;
    if (existingEnvironment?.projectName) return existingEnvironment.projectName;
    const effectiveProjectId = selectedProjectId ?? existingEnvironment?.projectId;
    return projects.find(p => p.id === effectiveProjectId)?.name;
  }, [isProjectLocked, selectedProjectId, existingEnvironment, projects]);

  // Reset form when modal opens/closes or environment changes
  const resetForm = () => {
    setName(existingEnvironment?.name ?? '');
    setConfigVarsYaml(existingEnvironment ? configVarsToYaml(existingEnvironment.configVars) : '');
    setSecrets(existingEnvironment?.secretKeys.map(key => ({ key, value: '', isExisting: true })) ?? []);
    setProjectId(selectedProjectId ?? existingEnvironment?.projectId ?? '');
    setError(null);
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleSubmit = async () => {
    setError(null);

    // Validate project selection
    const effectiveProjectId = isProjectLocked
      ? (selectedProjectId ?? existingEnvironment?.projectId ?? '')
      : projectId;
    if (!effectiveProjectId) {
      setError('Please select a project');
      return;
    }

    // Validate name
    if (!name.trim()) {
      setError('Name is required');
      return;
    }

    // Validate slug
    if (!slug) {
      setError('Name must contain valid characters for a slug');
      return;
    }

    // Parse config vars YAML
    let configVars: Record<string, unknown> = {};
    if (configVarsYaml.trim()) {
      try {
        const parsed = yaml.load(configVarsYaml);
        if (parsed === null || parsed === undefined) {
          configVars = {};
        } else if (typeof parsed !== 'object' || Array.isArray(parsed)) {
          setError('Config vars must be a YAML object (key: value pairs)');
          return;
        } else {
          configVars = parsed as Record<string, unknown>;
        }
      } catch (e) {
        setError(`Invalid YAML: ${e instanceof Error ? e.message : 'parse error'}`);
        return;
      }
    }

    // Build secrets (only include non-empty values)
    const secretsMap: Record<string, string> = {};
    for (const s of secrets) {
      if (s.key.trim() && s.value.trim()) {
        secretsMap[s.key.trim()] = s.value;
      }
    }

    try {
      await onSubmit({
        name: name.trim(),
        slug,
        projectId: effectiveProjectId,
        configVars,
        secrets: secretsMap,
      });
      handleClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save environment');
    }
  };

  const addSecretField = () => {
    setSecrets(prev => [...prev, { key: '', value: '', isExisting: false }]);
  };

  const updateSecret = (index: number, field: 'key' | 'value', value: string) => {
    setSecrets(prev => prev.map((s, i) => (i === index ? { ...s, [field]: value } : s)));
  };

  const removeSecret = (index: number) => {
    setSecrets(prev => prev.filter((_, i) => i !== index));
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-lg p-6 max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <h3 className="mb-6">{isEditMode ? 'Edit Environment' : 'Create Environment'}</h3>

        {error && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded text-sm text-red-600">
            {error}
          </div>
        )}

        <div className="space-y-4">
          {/* Project Selection (shown only in All Projects mode) */}
          {projects.length > 0 && (
            <div>
              <label className="block text-sm text-[#666666] mb-2">Project *</label>
              {isProjectLocked ? (
                <div className="flex items-center gap-2 px-3 py-2 bg-[#f5f5f5] border border-[#e5e5e5] rounded-md">
                  <span className="text-sm text-[#333333]">
                    {lockedProjectName || 'Loading...'}
                  </span>
                </div>
              ) : (
                <select
                  value={projectId}
                  onChange={(e) => setProjectId(e.target.value)}
                  className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
                >
                  <option value="">Select a project...</option>
                  {projects.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              )}
            </div>
          )}

          {/* Name */}
          <div>
            <label className="block text-sm text-[#666666] mb-2">Name *</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g., Production"
              className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
            />
          </div>

          {/* Slug - auto-generated from name */}
          <div>
            <label className="block text-sm text-[#666666] mb-2">Slug (used in CLI)</label>
            <div className="flex items-center gap-2 px-3 py-2 bg-[#f5f5f5] border border-[#e5e5e5] rounded-md">
              <code className="font-mono text-sm text-[#333333] flex-1">
                {slug || <span className="text-[#999999] italic">type a name above...</span>}
              </code>
            </div>
          </div>

          {/* Config Vars (YAML) */}
          <div>
            <label className="block text-sm text-[#666666] mb-2">
              Config Variables (YAML) - accessed via {'{{ .vars.* }}'}
            </label>
            <textarea
              value={configVarsYaml}
              onChange={(e) => setConfigVarsYaml(e.target.value)}
              rows={6}
              className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5 font-mono text-sm"
              placeholder={'base_url: https://api.example.com\napi_version: v2'}
            />
          </div>

          {/* Secrets */}
          <div>
            <label className="block text-sm text-[#666666] mb-2">
              Secrets - accessed via {'{{ .env.* }}'}
            </label>
            <div className="space-y-2">
              {secrets.map((secret, idx) => (
                <div key={idx} className="flex gap-2">
                  <input
                    type="text"
                    value={secret.key}
                    onChange={(e) => updateSecret(idx, 'key', e.target.value)}
                    placeholder="KEY_NAME"
                    className="flex-1 px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5 font-mono text-sm disabled:bg-[#f5f5f5] disabled:text-[#666666]"
                    disabled={secret.isExisting}
                  />
                  {secret.isExisting && !secret.value ? (
                    <input
                      type="text"
                      value="••••••••"
                      onClick={() => updateSecret(idx, 'value', '')}
                      onFocus={() => updateSecret(idx, 'value', '')}
                      readOnly
                      className="flex-1 px-3 py-2 bg-[#f5f5f5] border border-[#e5e5e5] rounded-md text-[#999999] cursor-pointer"
                      title="Click to enter new value"
                    />
                  ) : (
                    <input
                      type="password"
                      value={secret.value}
                      onChange={(e) => updateSecret(idx, 'value', e.target.value)}
                      placeholder={secret.isExisting ? 'Enter new value to update' : 'secret value'}
                      className="flex-1 px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
                    />
                  )}
                  <button
                    onClick={() => removeSecret(idx)}
                    className="p-2 hover:bg-[#f5f5f5] rounded transition-colors"
                    type="button"
                  >
                    <Trash2 className="w-4 h-4 text-[#666666]" />
                  </button>
                </div>
              ))}
              <button
                onClick={addSecretField}
                className="text-sm text-black hover:underline flex items-center gap-1"
                type="button"
              >
                <Plus className="w-4 h-4" />
                Add secret
              </button>
            </div>
            {isEditMode && secrets.length > 0 && (
              <p className="text-xs text-[#999999] mt-2">
                Click on masked values to enter a new value. Leave blank to keep existing.
              </p>
            )}
          </div>
        </div>

        <div className="flex gap-2 justify-end mt-6 pt-4 border-t border-[#e5e5e5]">
          <button
            onClick={handleClose}
            className="px-4 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
            type="button"
          >
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            disabled={isSubmitting}
            className="px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors disabled:opacity-50"
            type="button"
          >
            {isSubmitting
              ? 'Saving...'
              : isEditMode
              ? 'Update Environment'
              : 'Create Environment'}
          </button>
        </div>
      </div>
    </div>
  );
}
