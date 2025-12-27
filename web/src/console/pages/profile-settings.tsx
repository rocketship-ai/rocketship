import { useState } from 'react';
import { User, Mail, Github, Shield, Check, X, Edit2, LogOut } from 'lucide-react';

interface ProfileSettingsProps {
  onLogout?: () => void;
}

export function ProfileSettings({ onLogout }: ProfileSettingsProps) {
  const [isEditingName, setIsEditingName] = useState(false);
  const [name, setName] = useState('Austin Rath');
  const [tempName, setTempName] = useState(name);

  const handleSaveName = () => {
    setName(tempName);
    setIsEditingName(false);
  };

  const handleCancelEdit = () => {
    setTempName(name);
    setIsEditingName(false);
  };

  // Mock user data
  const userEmail = 'austin.rath@globalbank.com';
  const githubUsername = 'arath36';
  const organization = 'Global Bank';

  // Mock TBAC permissions for projects
  const projectPermissions = [
    {
      projectId: 'project-1',
      projectName: 'Backend',
      permissions: ['read', 'write']
    },
    {
      projectId: 'project-2',
      projectName: 'Frontend',
      permissions: ['read', 'write']
    },
  ];

  return (
    <div className="p-8">
      <div className="max-w-4xl mx-auto space-y-6">
        {/* Profile Information Card */}
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
          <h2 className="mb-6 flex items-center gap-2">
            <User className="w-5 h-5" />
            Profile Information
          </h2>

          <div className="space-y-6">
            {/* Name Field */}
            <div>
              <label className="block text-sm text-[#666666] mb-2">Name</label>
              {!isEditingName ? (
                <div className="flex items-center gap-3">
                  <span className="text-[#000000]">{name}</span>
                  <button
                    onClick={() => {
                      setTempName(name);
                      setIsEditingName(true);
                    }}
                    className="text-[#666666] hover:text-black transition-colors"
                  >
                    <Edit2 className="w-4 h-4" />
                  </button>
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    value={tempName}
                    onChange={(e) => setTempName(e.target.value)}
                    className="flex-1 px-3 py-2 border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
                    autoFocus
                  />
                  <button
                    onClick={handleSaveName}
                    className="p-2 text-[#4CBB17] hover:bg-[#4CBB17]/5 rounded-md transition-colors"
                    title="Save"
                  >
                    <Check className="w-4 h-4" />
                  </button>
                  <button
                    onClick={handleCancelEdit}
                    className="p-2 text-[#999999] hover:bg-black/5 rounded-md transition-colors"
                    title="Cancel"
                  >
                    <X className="w-4 h-4" />
                  </button>
                </div>
              )}
            </div>

            {/* Email */}
            <div>
              <label className="block text-sm text-[#666666] mb-2">Email</label>
              <div className="flex items-center gap-2 text-[#000000]">
                <Mail className="w-4 h-4 text-[#666666]" />
                <span>{userEmail}</span>
              </div>
            </div>

            {/* Organization */}
            <div>
              <label className="block text-sm text-[#666666] mb-2">Organization</label>
              <span className="text-[#000000]">{organization}</span>
            </div>
          </div>
        </div>

        {/* GitHub Integration Card */}
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
          <h2 className="mb-6 flex items-center gap-2">
            <Github className="w-5 h-5" />
            GitHub Integration
          </h2>

          <div className="flex items-start gap-4">
            <img
              src={`https://github.com/${githubUsername}.png`}
              alt="GitHub Avatar"
              className="w-16 h-16 rounded-full border border-[#e5e5e5]"
            />
            <div className="flex-1">
              <div className="mb-2">
                <label className="block text-sm text-[#666666] mb-1">GitHub Username</label>
                <a
                  href={`https://github.com/${githubUsername}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[#000000] hover:underline font-mono"
                >
                  @{githubUsername}
                </a>
              </div>
              <div className="inline-flex items-center gap-2 px-3 py-1 bg-[#4CBB17]/10 text-[#4CBB17] rounded-full text-sm border border-[#4CBB17]/20">
                <Check className="w-3 h-3" />
                Connected
              </div>
            </div>
          </div>
        </div>

        {/* Project Permissions Card */}
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
          <h2 className="mb-6 flex items-center gap-2">
            <Shield className="w-5 h-5" />
            Project Permissions
          </h2>

          <div className="space-y-4">
            {projectPermissions.map((project) => (
              <div
                key={project.projectId}
                className="border border-[#e5e5e5] rounded-lg p-4"
              >
                <h3 className="text-[#000000] mb-3">{project.projectName}</h3>
                
                <div>
                  <label className="block text-sm text-[#666666] mb-2">Permissions</label>
                  <div className="flex flex-wrap gap-2">
                    {project.permissions.map((permission) => (
                      <span
                        key={permission}
                        className="px-2 py-1 bg-[#fafafa] border border-[#e5e5e5] rounded text-xs font-mono text-[#666666]"
                      >
                        {permission}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Sign Out */}
        {onLogout && (
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <h2 className="mb-4 flex items-center gap-2">
              <LogOut className="w-5 h-5" />
              Sign Out
            </h2>
            <p className="text-sm text-[#666666] mb-4">
              Sign out of your Rocketship Cloud account on this device.
            </p>
            <button
              onClick={onLogout}
              className="px-4 py-2 bg-[#ef0000] text-white rounded-md hover:bg-[#ef0000]/90 transition-colors"
            >
              Sign out
            </button>
          </div>
        )}
      </div>
    </div>
  );
}