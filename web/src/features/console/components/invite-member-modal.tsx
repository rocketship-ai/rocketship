import { useState } from 'react';
import { X, Send, Loader2 } from 'lucide-react';
import type { ProjectSummary } from '../hooks/use-console-queries';

export interface InviteMemberFormData {
  email: string;
  projects: { project_id: string; role: 'read' | 'write' }[];
}

interface ProjectSelection {
  projectId: string;
  role: 'read' | 'write';
}

interface InviteMemberModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (data: InviteMemberFormData) => Promise<void>;
  isSubmitting: boolean;
  /** Projects the user can invite to */
  projects: ProjectSummary[];
  /** Whether the current user is an org owner (affects messaging) */
  isOrgOwner?: boolean;
}

export function InviteMemberModal({
  isOpen,
  onClose,
  onSubmit,
  isSubmitting,
  projects,
  isOrgOwner = false,
}: InviteMemberModalProps) {
  const [email, setEmail] = useState('');
  const [selectedProjects, setSelectedProjects] = useState<ProjectSelection[]>([]);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const toggleProject = (projectId: string) => {
    setSelectedProjects((prev) => {
      const existing = prev.find((p) => p.projectId === projectId);
      if (existing) {
        return prev.filter((p) => p.projectId !== projectId);
      }
      return [...prev, { projectId, role: 'read' as const }];
    });
  };

  const setProjectRole = (projectId: string, role: 'read' | 'write') => {
    setSelectedProjects((prev) =>
      prev.map((p) => (p.projectId === projectId ? { ...p, role } : p))
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!email.trim()) {
      setError('Email address is required');
      return;
    }
    if (!email.includes('@')) {
      setError('Please enter a valid email address');
      return;
    }
    if (selectedProjects.length === 0) {
      setError('Please select at least one project');
      return;
    }

    setError(null);
    try {
      await onSubmit({
        email: email.trim(),
        projects: selectedProjects.map((p) => ({
          project_id: p.projectId,
          role: p.role,
        })),
      });
      // Reset form only on success
      setEmail('');
      setSelectedProjects([]);
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to send invite');
      }
    }
  };

  const handleClose = () => {
    setError(null);
    setEmail('');
    setSelectedProjects([]);
    onClose();
  };

  const isValid = email.trim() && email.includes('@') && selectedProjects.length > 0;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="absolute inset-0 bg-black/50"
        onClick={handleClose}
      />
      <div className="relative bg-white rounded-lg shadow-xl w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-4 border-b border-[#e5e5e5]">
          <div className="flex items-center gap-2">
            <Send className="w-5 h-5 text-[#666666]" />
            <h3 className="font-semibold">Invite Member</h3>
          </div>
          <button
            onClick={handleClose}
            className="text-[#666666] hover:text-black transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-4 space-y-4">
          {error && (
            <div className="text-sm text-red-600 bg-red-50 px-3 py-2 rounded-md">
              {error}
            </div>
          )}

          {/* Email Input */}
          <div>
            <label className="block text-sm font-medium mb-1">Email Address</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
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
              {projects.length > 0 ? (
                projects.map((project) => {
                  const selected = selectedProjects.find((p) => p.projectId === project.id);
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
                          onChange={() => toggleProject(project.id)}
                          className="w-4 h-4 rounded border-[#e5e5e5] text-black focus:ring-black"
                        />
                        <span className="text-sm">{project.name}</span>
                      </label>
                      {selected && (
                        <select
                          value={selected.role}
                          onChange={(e) =>
                            setProjectRole(project.id, e.target.value as 'read' | 'write')
                          }
                          className="text-sm px-2 py-1 border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-1 focus:ring-black"
                        >
                          <option value="read">Read</option>
                          <option value="write">Write</option>
                        </select>
                      )}
                    </div>
                  );
                })
              ) : (
                <p className="text-sm text-[#666666] text-center py-4">
                  {isOrgOwner
                    ? 'No projects available. Create a project first.'
                    : "You don't have Write access to any projects."}
                </p>
              )}
            </div>
          </div>
        </form>

        <div className="flex justify-end gap-3 p-4 border-t border-[#e5e5e5]">
          <button
            type="button"
            onClick={handleClose}
            className="px-4 py-2 text-sm border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            disabled={isSubmitting || !isValid}
            className="flex items-center gap-2 px-4 py-2 text-sm bg-black text-white rounded-md hover:bg-black/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isSubmitting ? (
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
  );
}
