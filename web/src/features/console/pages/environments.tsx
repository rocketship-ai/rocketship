import { Plus, ExternalLink, Key as KeyIcon } from 'lucide-react';
import { useState } from 'react';
import { environments, accessKeys, projects } from '../data/mock-data';
import { InfoLabel } from '../components/info-label';
import { MultiSelectDropdown } from '../components/multi-select-dropdown';

interface EnvironmentsProps {
  onNavigate: (page: string, params?: any) => void;
}

export function Environments({ onNavigate }: EnvironmentsProps) {
  const [showTokenForm, setShowTokenForm] = useState(false);
  const [selectedProjectFilter, setSelectedProjectFilter] = useState<string[]>([]);
  const [isProjectDropdownOpen, setIsProjectDropdownOpen] = useState(false);
  const [revokeTokenId, setRevokeTokenId] = useState<string | null>(null);

  // Transform environments for UI display
  const transformedEnvironments = environments.map(env => ({
    id: env.id,
    name: env.name,
    isDefault: env.type === 'production',
    lastRun: env.lastDeployed,
  }));

  // Transform CI tokens - parse scopes properly
  const ciTokens = accessKeys.map(key => {
    const hasRead = key.permissions.some(p => p.startsWith('read:'));
    const hasWrite = key.permissions.some(p => p.startsWith('write:'));
    const scopes = [];
    if (hasRead) scopes.push('Read');
    if (hasWrite) scopes.push('Write');
    
    return {
      id: key.id,
      name: key.name,
      projectId: key.projectId,
      scopes,
      expires: 'Never',
      lastUsed: key.lastUsed,
      status: 'active' as const,
    };
  });

  const orgAdmins = [
    { name: 'Sarah Chen', email: 'sarah@acme.com' },
    { name: 'Mike Rodriguez', email: 'mike@acme.com' },
  ];

  const projectMembers = [
    { name: 'Alex Kim', email: 'alex@acme.com', role: 'Write', project: 'Backend' },
    { name: 'Jordan Lee', email: 'jordan@acme.com', role: 'Read', project: 'Backend' },
    { name: 'Taylor Swift', email: 'taylor@acme.com', role: 'Write', project: 'Frontend' },
    { name: 'Jamie Diaz', email: 'jamie@acme.com', role: 'Read', project: 'Frontend' },
  ];

  const filteredProjectMembers = selectedProjectFilter.length === 0 
    ? projectMembers 
    : projectMembers.filter(m => selectedProjectFilter.includes(m.project));

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto space-y-8">
        {/* Environments Section */}
        <div>
          <h2 className="mb-3">Environments</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {transformedEnvironments.map((env) => (
              <div
                key={env.id}
                className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6"
              >
                <div className="flex items-start justify-between mb-4">
                  <div>
                    <div className="flex items-center gap-2 mb-2">
                      <h3>{env.name}</h3>
                    </div>
                    <p className="text-sm text-[#666666]">Last run: {env.lastRun}</p>
                  </div>
                </div>

                <div className="flex gap-2">
                  <button
                    onClick={() => onNavigate('suite-activity', { env: env.name })}
                    className="flex items-center gap-1 text-sm text-black hover:underline ml-auto"
                  >
                    <span>View runs</span>
                    <ExternalLink className="w-3 h-3" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* CI Tokens Section */}
        <div>
          <div className="flex items-center justify-between mb-4">
            <h2>CI Tokens</h2>
            <button
              onClick={() => setShowTokenForm(!showTokenForm)}
              className="flex items-center gap-2 px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors"
            >
              <Plus className="w-4 h-4" />
              <span>Issue new token</span>
            </button>
          </div>

          {showTokenForm && (
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 mb-4">
              <h3 className="mb-4">Create New Token</h3>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm text-[#666666] mb-2">Token Name</label>
                  <input
                    type="text"
                    placeholder="e.g., GitHub Actions Production"
                    className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
                  />
                </div>

                <div>
                  <label className="block text-sm text-[#666666] mb-2">Project</label>
                  <select className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5">
                    {projects.map((project) => (
                      <option key={project.id} value={project.id}>
                        {project.name}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-sm text-[#666666] mb-2">Scopes</label>
                  <div className="space-y-2">
                    <label className="flex items-start gap-2">
                      <input type="checkbox" defaultChecked className="mt-1" />
                      <span className="text-sm">
                        <strong>Read</strong> <span className="text-[#666666]">(view runs, artifacts, logs, configs at ref; cannot run/modify; cannot view tokens.)</span>
                      </span>
                    </label>
                    <label className="flex items-start gap-2">
                      <input type="checkbox" defaultChecked className="mt-1" />
                      <span className="text-sm">
                        <strong>Write</strong> <span className="text-[#666666]">(run tests in any env, edit tests in UI and run edited tests, edit schedules, ask Rocketship to create PR or commit, mint tokens; includes Read)</span>
                      </span>
                    </label>
                  </div>
                </div>

                <div>
                  <label className="block text-sm text-[#666666] mb-2">Expiration</label>
                  <select className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5">
                    <option value="never">Never</option>
                    <option value="30">30 days</option>
                    <option value="90">90 days</option>
                    <option value="365">1 year</option>
                  </select>
                </div>

                <div className="flex gap-2 pt-2">
                  <button className="px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors">
                    Generate token
                  </button>
                  <button
                    onClick={() => setShowTokenForm(false)}
                    className="px-4 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            </div>
          )}

          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm overflow-hidden">
            <table className="w-full">
              <thead className="border-b border-[#e5e5e5] bg-[#fafafa]">
                <tr>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider">
                    Name
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider">
                    Project
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider">
                    Scopes
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider">
                    Expires
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider">
                    Last Used
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider">
                    Status
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#e5e5e5]">
                {ciTokens.map((token) => {
                  const project = projects.find(p => p.id === token.projectId);
                  return (
                    <tr key={token.id}>
                      <td className="px-6 py-4">
                        <div className="flex items-center gap-2">
                          <KeyIcon className="w-4 h-4 text-[#666666]" />
                          <span className="text-sm">{token.name}</span>
                        </div>
                      </td>
                      <td className="px-6 py-4 text-sm">{project?.name || 'Unknown'}</td>
                      <td className="px-6 py-4">
                        <div className="flex gap-1">
                          {token.scopes.map((scope) => (
                            <span
                              key={scope}
                              className="text-xs px-2 py-0.5 bg-[#fafafa] border border-[#e5e5e5] rounded"
                            >
                              {scope}
                            </span>
                          ))}
                        </div>
                      </td>
                      <td className="px-6 py-4 text-sm text-[#666666]">{token.expires}</td>
                      <td className="px-6 py-4 text-sm text-[#666666]">{token.lastUsed}</td>
                      <td className="px-6 py-4">
                        <span
                          className={`text-xs px-2 py-1 rounded ${
                            token.status === 'active'
                              ? 'bg-[#228b22]/10 text-[#228b22]'
                              : 'bg-[#999999]/10 text-[#999999]'
                          }`}
                        >
                          {token.status}
                        </span>
                      </td>
                      <td className="px-6 py-4">
                        {token.status === 'active' ? (
                          <button
                            onClick={() => setRevokeTokenId(token.id)}
                            className="text-sm text-[#ef0000] hover:underline"
                          >
                            Revoke
                          </button>
                        ) : (
                          <span className="text-sm text-[#999999]">-</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>

        {/* Access Section */}
        <div>
          <h2 className="mb-3">Access Control</h2>

          <div className="mb-6">
            <InfoLabel>
              Organization admins automatically have Write access to all projects. Project members can be granted Read or Write access.
            </InfoLabel>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Org Admins */}
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
              <h3 className="mb-4">Organization Admins</h3>
              <div className="space-y-3">
                {orgAdmins.map((admin, idx) => (
                  <div
                    key={idx}
                    className="flex items-center justify-between pb-3 border-b border-[#e5e5e5] last:border-0 last:pb-0"
                  >
                    <div>
                      <p className="text-sm">{admin.name}</p>
                      <p className="text-xs text-[#999999]">{admin.email}</p>
                    </div>
                    <span className="text-xs px-2 py-1 bg-[#fafafa] border border-[#e5e5e5] rounded">
                      Admin
                    </span>
                  </div>
                ))}
              </div>
            </div>

            {/* Project Members */}
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
              <div className="flex items-center justify-between mb-4">
                <h3>Project Members</h3>
                <button className="flex items-center gap-2 text-sm text-black hover:underline">
                  <Plus className="w-4 h-4" />
                  <span>Add member</span>
                </button>
              </div>

              {/* Project Filter */}
              <div className="mb-4">
                <MultiSelectDropdown
                  label="Projects"
                  items={projects.map(project => project.name)}
                  selectedItems={selectedProjectFilter}
                  onSelectionChange={setSelectedProjectFilter}
                  isOpen={isProjectDropdownOpen}
                  onToggle={() => setIsProjectDropdownOpen(!isProjectDropdownOpen)}
                />
              </div>

              <div className="space-y-3">
                {filteredProjectMembers.map((member, idx) => (
                  <div
                    key={idx}
                    className="flex items-center justify-between pb-3 border-b border-[#e5e5e5] last:border-0 last:pb-0"
                  >
                    <div className="flex-1">
                      <p className="text-sm">{member.name}</p>
                      <p className="text-xs text-[#999999]">{member.email} • {member.project}</p>
                    </div>
                    <select
                      defaultValue={member.role}
                      className="text-xs px-2 py-1 bg-white border border-[#e5e5e5] rounded focus:outline-none focus:ring-2 focus:ring-black/5"
                    >
                      <option value="Read">Read</option>
                      <option value="Write">Write</option>
                    </select>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Revoke Token Confirmation Modal */}
      {revokeTokenId && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-lg p-6 max-w-md w-full mx-4">
            <h3 className="mb-4">Revoke Token</h3>
            <p className="text-sm text-[#666666] mb-6">
              Are you sure you want to revoke this token? This action cannot be undone and any services using this token will immediately lose access.
            </p>
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setRevokeTokenId(null)}
                className="px-4 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => {
                  // Handle revoke logic here
                  console.log('Revoking token:', revokeTokenId);
                  setRevokeTokenId(null);
                }}
                className="px-4 py-2 bg-[#ef0000] text-white rounded-md hover:bg-[#ef0000]/90 transition-colors"
              >
                Revoke Token
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}