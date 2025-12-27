import { ExternalLink, FolderOpen, GitBranch } from 'lucide-react';
import { projects, getTestCountByProject } from '../data/mock-data';
import { InfoLabel } from '../components/info-label';

interface ProjectsProps {
  onSelectProject: (projectId: string) => void;
}

export function Projects({ onSelectProject }: ProjectsProps) {
  const testCounts = getTestCountByProject();

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
                  </div>

                  <div className="space-y-2 mb-4">
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-[#999999] w-32">Repository:</span>
                      <a 
                        href={project.repoUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-[#666666] hover:text-black font-mono flex items-center gap-1"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {project.repoUrl}
                        <ExternalLink className="w-3 h-3" />
                      </a>
                    </div>
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-[#999999] w-32">Path scope:</span>
                      <span className="text-[#666666] font-mono">{project.pathScope}</span>
                    </div>
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-[#999999] w-32">Default branch:</span>
                      <span className="inline-flex items-center gap-1 text-[#666666]">
                        <GitBranch className="w-3 h-3" />
                        {project.defaultBranch}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-[#999999] w-32">Tests:</span>
                      <span className="text-[#666666]">{testCounts[project.id] || 0}</span>
                    </div>
                  </div>

                  <p className="text-xs text-[#999999]">Last updated {project.lastUpdated}</p>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}