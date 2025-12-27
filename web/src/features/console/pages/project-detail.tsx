import { ArrowLeft, FolderOpen, ExternalLink, GitBranch, FileCode, Key, Users } from 'lucide-react';
import { projects, accessKeys, getSuitesByProjectId, getTestsBySuiteId } from '../data/mock-data';

interface ProjectDetailProps {
  projectId: string;
  onBack: () => void;
  onViewSuite?: (suiteId: string) => void;
}

export function ProjectDetail({ projectId, onBack, onViewSuite }: ProjectDetailProps) {
  const project = projects.find((p) => p.id === projectId);
  
  if (!project) {
    return (
      <div className="p-8">
        <div className="max-w-7xl mx-auto">
          <p className="text-[#666666]">Project not found</p>
        </div>
      </div>
    );
  }

  const projectSuites = getSuitesByProjectId(projectId);
  const projectTests = projectSuites.flatMap(suite => getTestsBySuiteId(suite.id));
  const projectTokens = accessKeys.filter(token => token.projectId === projectId);
  
  // Mock access control users for this project
  const usersWithAccess = [
    { id: 'user-1', name: 'Alex Kim', email: 'alex@acme.com', role: 'Write' },
    { id: 'user-2', name: 'Jordan Lee', email: 'jordan@acme.com', role: 'Read' },
    { id: 'user-3', name: 'Taylor Swift', email: 'taylor@acme.com', role: 'Write' },
  ].filter(() => projectId === 'project-1' || Math.random() > 0.5); // Mock filtering

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
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm">
                <span className="text-[#999999] w-32">Repository:</span>
                <a 
                  href={project.repoUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[#666666] hover:text-black font-mono flex items-center gap-1"
                >
                  {project.repoUrl}
                  <ExternalLink className="w-3 h-3" />
                </a>
              </div>
              <div className="flex items-center gap-2 text-sm">
                <span className="text-[#999999] w-32">Path scope:</span>
                <span className="text-[#666666] font-mono">{project.pathScope}</span>
              </div>
            </div>

            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm">
                <span className="text-[#999999] w-32">Default branch:</span>
                <span className="inline-flex items-center gap-1 text-[#666666]">
                  <GitBranch className="w-3 h-3" />
                  {project.defaultBranch}
                </span>
              </div>
              <div className="flex items-center gap-2 text-sm">
                <span className="text-[#999999] w-32">Last updated:</span>
                <span className="text-[#666666]">{project.lastUpdated}</span>
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
            <div className="text-2xl">{projectSuites.length}</div>
          </div>

          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <div className="flex items-center gap-2 mb-2">
              <FileCode className="w-4 h-4 text-[#666666]" />
              <span className="text-sm text-[#999999]">Tests</span>
            </div>
            <div className="text-2xl">{projectTests.length}</div>
          </div>

          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <div className="flex items-center gap-2 mb-2">
              <Key className="w-4 h-4 text-[#666666]" />
              <span className="text-sm text-[#999999]">CI Tokens</span>
            </div>
            <div className="text-2xl">{projectTokens.length}</div>
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
              {projectSuites.map((suite) => {
                const testCount = getTestsBySuiteId(suite.id).length;
                
                return (
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
                        </div>
                        <p className="text-xs text-[#666666] font-mono ml-7">{suite.path}</p>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-[#999999]">{testCount} tests</span>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Access Section */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* CI Tokens */}
          <div>
            <h3 className="mb-4">CI Tokens</h3>
            
            {projectTokens.length === 0 ? (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 text-center">
                <p className="text-sm text-[#666666]">No CI tokens for this project</p>
              </div>
            ) : (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm">
                <div className="divide-y divide-[#e5e5e5]">
                  {projectTokens.map((token) => (
                    <div key={token.id} className="p-4">
                      <div className="flex items-start justify-between">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 mb-1">
                            <Key className="w-4 h-4 text-[#666666] flex-shrink-0" />
                            <span className="text-sm truncate">{token.name}</span>
                          </div>
                          <p className="text-xs text-[#999999] ml-6">
                            Created {token.createdAt}
                          </p>
                          <p className="text-xs text-[#999999] ml-6">
                            Last used: {token.lastUsed}
                          </p>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Users with Access */}
          <div>
            <h3 className="mb-4">Users with Access</h3>
            
            {usersWithAccess.length === 0 ? (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 text-center">
                <p className="text-sm text-[#666666]">No users have explicit access</p>
              </div>
            ) : (
              <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm">
                <div className="divide-y divide-[#e5e5e5]">
                  {usersWithAccess.map((user) => (
                    <div key={user.id} className="p-4">
                      <div className="flex items-start justify-between">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 mb-1">
                            <Users className="w-4 h-4 text-[#666666] flex-shrink-0" />
                            <span className="text-sm truncate">{user.name}</span>
                          </div>
                          <p className="text-xs text-[#999999] ml-6">{user.email}</p>
                          <p className="text-xs text-[#666666] ml-6 mt-1">{user.role}</p>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}