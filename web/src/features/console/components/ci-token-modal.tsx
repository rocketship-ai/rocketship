import { useState } from 'react';
import { X, Plus, Trash2 } from 'lucide-react';
import type { ProjectSummary } from '../hooks/use-console-queries';

interface ProjectScopeItem {
  projectId: string;
  projectName: string;
  scope: 'read' | 'write';
}

interface CITokenModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (data: {
    name: string;
    description: string;
    neverExpires: boolean;
    expiresAt: string | null;
    projects: { project_id: string; scope: 'read' | 'write' }[];
  }) => Promise<void>;
  isSubmitting: boolean;
  projects: ProjectSummary[];
}

export function CITokenModal({
  isOpen,
  onClose,
  onSubmit,
  isSubmitting,
  projects,
}: CITokenModalProps) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [neverExpires, setNeverExpires] = useState(true);
  const [expiresAt, setExpiresAt] = useState('');
  const [selectedProjects, setSelectedProjects] = useState<ProjectScopeItem[]>([]);
  const [selectedProjectToAdd, setSelectedProjectToAdd] = useState('');

  if (!isOpen) return null;

  const availableProjects = projects.filter(
    (p) => !selectedProjects.some((sp) => sp.projectId === p.id)
  );

  const handleAddProject = () => {
    const project = projects.find((p) => p.id === selectedProjectToAdd);
    if (project) {
      setSelectedProjects([
        ...selectedProjects,
        { projectId: project.id, projectName: project.name, scope: 'write' },
      ]);
      setSelectedProjectToAdd('');
    }
  };

  const handleRemoveProject = (projectId: string) => {
    setSelectedProjects(selectedProjects.filter((p) => p.projectId !== projectId));
  };

  const handleScopeChange = (projectId: string, scope: 'read' | 'write') => {
    setSelectedProjects(
      selectedProjects.map((p) =>
        p.projectId === projectId ? { ...p, scope } : p
      )
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await onSubmit({
      name: name.trim(),
      description: description.trim(),
      neverExpires,
      expiresAt: neverExpires ? null : expiresAt || null,
      projects: selectedProjects.map((p) => ({
        project_id: p.projectId,
        scope: p.scope,
      })),
    });
    // Reset form
    setName('');
    setDescription('');
    setNeverExpires(true);
    setExpiresAt('');
    setSelectedProjects([]);
  };

  const isValid = name.trim() && selectedProjects.length > 0;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-lg w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-4 border-b border-[#e5e5e5]">
          <h2 className="text-lg font-semibold">Issue New CI Token</h2>
          <button
            onClick={onClose}
            className="p-1 hover:bg-[#f5f5f5] rounded-md transition-colors"
          >
            <X className="w-5 h-5 text-[#666666]" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-4 space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">Name *</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g., GitHub Actions CI"
              className="w-full px-3 py-2 border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/10"
            />
          </div>

          <div>
            <label className="block text-sm font-medium mb-1">Description</label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description"
              className="w-full px-3 py-2 border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/10"
            />
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Expiration</label>
            <div className="flex items-center gap-4">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  checked={neverExpires}
                  onChange={() => setNeverExpires(true)}
                  className="w-4 h-4"
                />
                <span className="text-sm">Never expires</span>
              </label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  checked={!neverExpires}
                  onChange={() => setNeverExpires(false)}
                  className="w-4 h-4"
                />
                <span className="text-sm">Set expiration date</span>
              </label>
            </div>
            {!neverExpires && (
              <input
                type="datetime-local"
                value={expiresAt}
                onChange={(e) => setExpiresAt(e.target.value)}
                min={new Date().toISOString().slice(0, 16)}
                className="mt-2 w-full px-3 py-2 border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/10"
              />
            )}
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Projects *</label>
            <p className="text-xs text-[#666666] mb-2">
              Select which projects this token can access and their permission level.
            </p>

            {selectedProjects.length > 0 && (
              <div className="space-y-2 mb-3">
                {selectedProjects.map((sp) => (
                  <div
                    key={sp.projectId}
                    className="flex items-center gap-2 p-2 bg-[#f9f9f9] rounded-md"
                  >
                    <span className="flex-1 text-sm font-medium truncate">
                      {sp.projectName}
                    </span>
                    <select
                      value={sp.scope}
                      onChange={(e) =>
                        handleScopeChange(sp.projectId, e.target.value as 'read' | 'write')
                      }
                      className="px-2 py-1 text-sm border border-[#e5e5e5] rounded bg-white"
                    >
                      <option value="write">Write</option>
                      <option value="read">Read</option>
                    </select>
                    <button
                      type="button"
                      onClick={() => handleRemoveProject(sp.projectId)}
                      className="p-1 hover:bg-[#e5e5e5] rounded transition-colors"
                    >
                      <Trash2 className="w-4 h-4 text-[#666666]" />
                    </button>
                  </div>
                ))}
              </div>
            )}

            {availableProjects.length > 0 && (
              <div className="flex gap-2">
                <select
                  value={selectedProjectToAdd}
                  onChange={(e) => setSelectedProjectToAdd(e.target.value)}
                  className="flex-1 px-3 py-2 border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/10"
                >
                  <option value="">Select a project...</option>
                  {availableProjects.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  onClick={handleAddProject}
                  disabled={!selectedProjectToAdd}
                  className="px-3 py-2 bg-[#f5f5f5] text-[#333333] rounded-md hover:bg-[#e5e5e5] transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <Plus className="w-4 h-4" />
                </button>
              </div>
            )}
          </div>

          <div className="flex gap-2 pt-4 border-t border-[#e5e5e5]">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!isValid || isSubmitting}
              className="flex-1 px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isSubmitting ? 'Creating...' : 'Create Token'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
