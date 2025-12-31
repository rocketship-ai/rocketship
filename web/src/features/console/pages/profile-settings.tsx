import { User, Mail, Github, Shield, Check, LogOut } from 'lucide-react';
import { useProfile } from '../hooks/use-console-queries';
import { SourceRefBadge } from '../components/SourceRefBadge';
import { ApiError } from '@/lib/api';
import { LoadingState, ErrorState } from '../components/ui';

interface ProfileSettingsProps {
  onLogout?: () => void;
}

export function ProfileSettings({ onLogout }: ProfileSettingsProps) {
  const { data: profile, isLoading, error } = useProfile();

  if (isLoading) {
    return (
      <div className="p-8">
        <div className="max-w-4xl mx-auto">
          <LoadingState message="Loading profile..." />
        </div>
      </div>
    );
  }

  if (error) {
    const errorMessage = error instanceof ApiError ? error.message : 'Failed to load profile';
    return (
      <div className="p-8">
        <div className="max-w-4xl mx-auto">
          <ErrorState title="Failed to load profile" message={errorMessage} />
        </div>
      </div>
    );
  }

  if (!profile) {
    return null;
  }

  const { user, organization, github, project_permissions } = profile;

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
            {/* Name Field - Read-only for v1 */}
            <div>
              <label className="block text-sm text-[#666666] mb-2">Name</label>
              <span className="text-[#000000]">{user.name || 'Not set'}</span>
            </div>

            {/* Email */}
            <div>
              <label className="block text-sm text-[#666666] mb-2">Email</label>
              <div className="flex items-center gap-2 text-[#000000]">
                <Mail className="w-4 h-4 text-[#666666]" />
                <span>{user.email || 'Not set'}</span>
              </div>
            </div>

            {/* Organization */}
            <div>
              <label className="block text-sm text-[#666666] mb-2">Organization</label>
              <div className="flex items-center gap-2">
                <span className="text-[#000000]">{organization.name}</span>
                <span className="px-2 py-0.5 bg-[#fafafa] border border-[#e5e5e5] rounded text-xs font-mono text-[#666666]">
                  {organization.role}
                </span>
              </div>
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
              src={github.avatar_url}
              alt="GitHub Avatar"
              className="w-16 h-16 rounded-full border border-[#e5e5e5]"
              onError={(e) => {
                // Fallback to a placeholder if avatar fails to load
                e.currentTarget.src = `https://ui-avatars.com/api/?name=${encodeURIComponent(github.username)}&background=random`;
              }}
            />
            <div className="flex-1">
              <div className="mb-2">
                <label className="block text-sm text-[#666666] mb-1">GitHub Username</label>
                <a
                  href={`https://github.com/${github.username}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[#000000] hover:underline font-mono"
                >
                  @{github.username}
                </a>
              </div>
              <div className="inline-flex items-center gap-2 px-3 py-1 bg-[#4CBB17]/10 text-[#4CBB17] rounded-full text-sm border border-[#4CBB17]/20">
                <Check className="w-3 h-3" />
                Connected
              </div>
              {github.app_installed && github.app_account_login && (
                <div className="mt-2 text-sm text-[#666666]">
                  App installed on: <span className="font-mono">{github.app_account_login}</span>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Project Permissions Card */}
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
          <h2 className="mb-6 flex items-center gap-2">
            <Shield className="w-5 h-5" />
            Project Permissions
          </h2>

          {project_permissions.length === 0 ? (
            <p className="text-[#666666] text-sm">No project permissions found.</p>
          ) : (
            <div className="space-y-4">
              {project_permissions.map((project) => (
                <div
                  key={project.project_id}
                  className="border border-[#e5e5e5] rounded-lg p-4"
                >
                  <div className="flex items-center gap-2 mb-3">
                    <h3 className="text-[#000000]">{project.project_name}</h3>
                    <SourceRefBadge sourceRef={project.source_ref} />
                  </div>

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
          )}
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
