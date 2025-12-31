import { ExternalLink, FolderOpen, GitBranch } from 'lucide-react';
import { InfoLabel } from '../components/info-label';
import { SourceRefBadge } from '../components/SourceRefBadge';
import { useProjects } from '../hooks/use-console-queries';
import { QueryBoundary } from '../components/query-boundary';
import { Card, EmptyState } from '../components/ui';

interface ProjectsProps {
  onSelectProject: (projectId: string) => void;
}

export function Projects({ onSelectProject }: ProjectsProps) {
  const query = useProjects();

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto">
        <QueryBoundary
          query={query}
          loadingMessage="Loading projects..."
          errorTitle="Failed to load projects"
        >
          {(projects) => (
            <>
              {/* Info Label */}
              <div className="mb-4">
                <InfoLabel>
                  A project directly corresponds to a <code className="px-1 py-0.5 bg-white rounded text-xs font-mono">.rocketship</code> directory inside of a repo.
                </InfoLabel>
              </div>

              {projects.length === 0 ? (
                <EmptyState
                  icon={<FolderOpen className="w-12 h-12" />}
                  title="No projects yet"
                  description={
                    <>
                      Connect a repository with a <code className="px-1 py-0.5 bg-[#f5f5f5] rounded text-xs font-mono">.rocketship</code> directory to get started.
                    </>
                  }
                />
              ) : (
                <div className="grid grid-cols-1 gap-4">
                  {projects.map((project) => (
                    <Card
                      key={project.id}
                      onClick={() => onSelectProject(project.id)}
                      className="hover:shadow-md transition-shadow cursor-pointer"
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex-1">
                          <div className="flex items-center gap-3 mb-3">
                            <FolderOpen className="w-5 h-5 text-[#666666]" />
                            <h3>{project.name}</h3>
                            <SourceRefBadge sourceRef={project.source_ref} defaultBranch={project.default_branch} />
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
                    </Card>
                  ))}
                </div>
              )}
            </>
          )}
        </QueryBoundary>
      </div>
    </div>
  );
}
