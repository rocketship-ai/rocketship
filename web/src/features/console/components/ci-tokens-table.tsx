import { Loader2, Key } from 'lucide-react';
import type { CIToken } from '../hooks/use-console-queries';

interface CITokensTableProps {
  tokens: CIToken[];
  isLoading: boolean;
  onRevoke: (tokenId: string) => void;
  isRevoking: boolean;
  revokingTokenId: string | null;
}

function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '—';
  const date = new Date(dateStr);
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

function formatRelativeTime(dateStr: string | undefined): string {
  if (!dateStr) return '—';
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / (1000 * 60));
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return formatDate(dateStr);
}

function getStatusBadge(status: CIToken['status']) {
  switch (status) {
    case 'active':
      return (
        <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-green-100 text-green-800">
          Active
        </span>
      );
    case 'revoked':
      return (
        <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-red-100 text-red-800">
          Revoked
        </span>
      );
    case 'expired':
      return (
        <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-yellow-100 text-yellow-800">
          Expired
        </span>
      );
    default:
      return null;
  }
}

export function CITokensTable({
  tokens,
  isLoading,
  onRevoke,
  isRevoking,
  revokingTokenId,
}: CITokensTableProps) {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-6 h-6 animate-spin text-[#666666]" />
        <span className="ml-3 text-[#666666]">Loading CI tokens...</span>
      </div>
    );
  }

  if (tokens.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <Key className="w-12 h-12 text-[#999999] mb-4" />
        <h3 className="text-lg font-medium mb-2">No CI tokens</h3>
        <p className="text-sm text-[#666666] max-w-md">
          Create a CI token to authenticate automated test runs from your CI/CD pipelines.
        </p>
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead>
          <tr className="border-b border-[#e5e5e5]">
            <th className="text-left py-3 px-4 text-sm font-medium text-[#666666]">Name</th>
            <th className="text-left py-3 px-4 text-sm font-medium text-[#666666]">Projects</th>
            <th className="text-left py-3 px-4 text-sm font-medium text-[#666666]">Expires</th>
            <th className="text-left py-3 px-4 text-sm font-medium text-[#666666]">Last Used</th>
            <th className="text-left py-3 px-4 text-sm font-medium text-[#666666]">Status</th>
            <th className="text-right py-3 px-4 text-sm font-medium text-[#666666]">Actions</th>
          </tr>
        </thead>
        <tbody>
          {tokens.map((token) => (
            <tr key={token.id} className="border-b border-[#e5e5e5] hover:bg-[#fafafa]">
              <td className="py-3 px-4">
                <div>
                  <div className="font-medium text-sm">{token.name}</div>
                  {token.description && (
                    <div className="text-xs text-[#666666] mt-0.5">{token.description}</div>
                  )}
                </div>
              </td>
              <td className="py-3 px-4">
                <div className="flex flex-wrap gap-1">
                  {token.projects.map((proj) => (
                    <span
                      key={proj.project_id}
                      className={`px-2 py-0.5 text-xs rounded-full ${
                        proj.scope === 'write'
                          ? 'bg-blue-100 text-blue-800'
                          : 'bg-gray-100 text-gray-700'
                      }`}
                    >
                      {proj.project_name}: {proj.scope === 'write' ? 'Write' : 'Read'}
                    </span>
                  ))}
                </div>
              </td>
              <td className="py-3 px-4 text-sm">
                {token.never_expires ? (
                  <span className="text-[#666666]">Never</span>
                ) : (
                  formatDate(token.expires_at)
                )}
              </td>
              <td className="py-3 px-4 text-sm text-[#666666]">
                {formatRelativeTime(token.last_used_at)}
              </td>
              <td className="py-3 px-4">{getStatusBadge(token.status)}</td>
              <td className="py-3 px-4 text-right">
                {token.status === 'active' ? (
                  <button
                    onClick={() => onRevoke(token.id)}
                    disabled={isRevoking && revokingTokenId === token.id}
                    className="text-sm text-red-600 hover:text-red-800 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isRevoking && revokingTokenId === token.id ? 'Revoking...' : 'Revoke'}
                  </button>
                ) : (
                  <span className="text-sm text-[#999999]">—</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
