import { ExternalLink, FolderOpen, GitBranch, Loader2, AlertCircle, RefreshCw } from 'lucide-react';
import { InfoLabel } from '../components/info-label';
import { SourceRefBadge } from '../components/SourceRefBadge';
import { useProjects } from '../hooks/use-console-queries';

interface ProjectsProps {
  onSelectProject: (projectId: string) => void;
}

export function Projects({ onSelectProject }: ProjectsProps) {
  const { data: projects, isLoading, error, refetch } = useProjects();

  if (isLoading) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto flex items-center justify-center py-12">
          <Loader2 className="w-6 h-6 animate-spin text-[#666666]" />
          <span className="ml-3 text-[#666666]">Loading projects...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <div className="bg-red-50 border border-red-200 rounded-lg p-6 flex items-start gap-3">
            <AlertCircle className="w-5 h-5 text-red-500 flex-shrink-0 mt-0.5" />
            <div className="flex-1">
              <p className="text-red-700 font-medium">Failed to load projects</p>
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

  if (!projects || projects.length === 0) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <div className="mb-4">
            <InfoLabel>
              A project directly corresponds to a <code className="px-1 py-0.5 bg-white rounded text-xs font-mono">.rocketship</code> directory inside of a repo.
            </InfoLabel>
          </div>
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
            <FolderOpen className="w-12 h-12 text-[#999999] mx-auto mb-4" />
            <h3 className="text-lg font-medium mb-2">No projects yet</h3>
            <p className="text-[#666666] text-sm">
              Connect a repository with a <code className="px-1 py-0.5 bg-[#f5f5f5] rounded text-xs font-mono">.rocketship</code> directory to get started.
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto">
        {/* Info Label */}
        <div className="mb-4">
          <InfoLabel>
            A project directly corresponds to a <code className="px-1 py-0.5 bg-white rounded text-xs font-mono">.rocketship</code> directory inside of a repo.
          </InfoLabel>
        </div>

        {/* Projects Grid */}
        <div className="grid grid-cols-1 gap-4">
          {projects.map((project) => (
            <div
              key={project.id}
              onClick={() => onSelectProject(project.id)}
              className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 hover:shadow-md transition-shadow cursor-pointer"
            >
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center gap-3 mb-3">
                    <FolderOpen className="w-5 h-5 text-[#666666]" />
                    <h3>{project.name}</h3>
                    <SourceRefBadge sourceRef={project.source_ref} />
                  </div>

                  <div className="space-y-2 mb-4">
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-[#999999] w-32">Repository:</span>
                      <a
                        href={project.repo_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-[#666666] hover:text-black font-mono flex items-center gap-1"
                        onClick={(e) => e.stopPropagation()}
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
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-[#999999] w-32">Default branch:</span>
                      <span className="inline-flex items-center gap-1 text-[#666666]">
                        <GitBranch className="w-3 h-3" />
                        {project.default_branch}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-[#999999] w-32">Suites / Tests:</span>
                      <span className="text-[#666666]">{project.suite_count} / {project.test_count}</span>
                    </div>
                  </div>

                  {project.last_scan ? (
                    <p className="text-xs text-[#999999]">
                      Last scanned {new Date(project.last_scan.created_at).toLocaleString()}
                      {project.last_scan.status === 'error' && ` - scan failed: ${project.last_scan.error_message}`}
                    </p>
                  ) : (
                    <p className="text-xs text-[#999999]">Not yet scanned</p>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
