import { ArrowLeft, FolderOpen, ExternalLink, GitBranch, FileCode, Layers } from 'lucide-react';
import { useProject, useProjectSuites, useProjectEnvironments } from '../hooks/use-console-queries';
import { SourceRefBadge } from '../components/SourceRefBadge';
import { LoadingState, ErrorState, Card } from '../components/ui';

interface ProjectDetailProps {
  projectId: string;
  onBack: () => void;
  onViewSuite?: (suiteId: string) => void;
}

export function ProjectDetail({ projectId, onBack, onViewSuite }: ProjectDetailProps) {
  const { data: project, isLoading: projectLoading, error: projectError, refetch: refetchProject } = useProject(projectId);
  const { data: suites, isLoading: suitesLoading, error: suitesError, refetch: refetchSuites } = useProjectSuites(projectId);
  const { data: environments } = useProjectEnvironments(projectId);

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

        {/* Suites Section */}
        <div className="mb-6">
          <h3 className="mb-4">Suites</h3>

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

      </div>
    </div>
  );
}
