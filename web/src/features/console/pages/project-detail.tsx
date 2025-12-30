import { ArrowLeft, FolderOpen, ExternalLink, GitBranch, FileCode, Key, Users, Loader2, AlertCircle, RefreshCw } from 'lucide-react';
import { useProject, useProjectSuites } from '../hooks/use-console-queries';
import { SourceRefBadge } from '../components/SourceRefBadge';

interface ProjectDetailProps {
  projectId: string;
  onBack: () => void;
  onViewSuite?: (suiteId: string) => void;
}

export function ProjectDetail({ projectId, onBack, onViewSuite }: ProjectDetailProps) {
  const { data: project, isLoading: projectLoading, error: projectError, refetch: refetchProject } = useProject(projectId);
  const { data: suites, isLoading: suitesLoading, error: suitesError, refetch: refetchSuites } = useProjectSuites(projectId);

  const isLoading = projectLoading || suitesLoading;
  const error = projectError || suitesError;

  if (isLoading) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <button
            onClick={onBack}
            className="flex items-center gap-2 text-[#666666] hover:text-black mb-6 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to Projects
          </button>
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-6 h-6 animate-spin text-[#666666]" />
            <span className="ml-3 text-[#666666]">Loading project...</span>
          </div>
        </div>
      </div>
    );
  }

  if (error || !project) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <button
            onClick={onBack}
            className="flex items-center gap-2 text-[#666666] hover:text-black mb-6 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to Projects
          </button>
          <div className="bg-red-50 border border-red-200 rounded-lg p-6 flex items-start gap-3">
            <AlertCircle className="w-5 h-5 text-red-500 flex-shrink-0 mt-0.5" />
            <div className="flex-1">
              <p className="text-red-700 font-medium">
                {!project ? 'Project not found' : 'Failed to load project'}
              </p>
              <p className="text-red-600 text-sm mt-1">
                {error instanceof Error ? error.message : 'An unexpected error occurred'}
              </p>
            </div>
            <button
              onClick={() => { refetchProject(); refetchSuites(); }}
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

  const projectSuites = suites || [];

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto">
        {/* Back Button */}
        <button
          onClick={onBack}
          className="flex items-center gap-2 text-[#666666] hover:text-black mb-6 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Projects
        </button>

        {/* Project Header */}
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 mb-6">
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
        </div>

        {/* Stats Grid */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <div className="flex items-center gap-2 mb-2">
              <FileCode className="w-4 h-4 text-[#666666]" />
              <span className="text-sm text-[#999999]">Suites</span>
            </div>
            <div className="text-2xl">{project.suite_count}</div>
          </div>

          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <div className="flex items-center gap-2 mb-2">
              <FileCode className="w-4 h-4 text-[#666666]" />
              <span className="text-sm text-[#999999]">Tests</span>
            </div>
            <div className="text-2xl">{project.test_count}</div>
          </div>

          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <div className="flex items-center gap-2 mb-2">
              <Key className="w-4 h-4 text-[#666666]" />
              <span className="text-sm text-[#999999]">CI Tokens</span>
            </div>
            <div className="text-2xl text-[#999999]">-</div>
          </div>
        </div>

        {/* Suites Section */}
        <div className="mb-6">
          <h3 className="mb-4">Suites</h3>

          {projectSuites.length === 0 ? (
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 text-center">
              <p className="text-[#666666]">No suites found for this project</p>
            </div>
          ) : (
            <div className="space-y-3">
              {projectSuites.map((suite) => (
                <div
                  key={suite.id}
                  onClick={() => onViewSuite?.(suite.id)}
                  className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-5 hover:shadow-md transition-shadow cursor-pointer"
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
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Access Section */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* CI Tokens */}
          <div>
            <h3 className="mb-4">CI Tokens</h3>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 text-center">
              <Key className="w-8 h-8 text-[#999999] mx-auto mb-3" />
              <p className="text-sm text-[#666666]">No tokens linked yet</p>
            </div>
          </div>

          {/* Users with Access */}
          <div>
            <h3 className="mb-4">Users with Access</h3>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 text-center">
              <Users className="w-8 h-8 text-[#999999] mx-auto mb-3" />
              <p className="text-sm text-[#666666]">No users yet</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
