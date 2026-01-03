import { CheckCircle2, XCircle, Loader2, GitBranch, Hash } from 'lucide-react';
import { EnvBadge, TriggerBadge, UsernameBadge, BadgeDot, ConfigSourceBadge } from './status-badge';
import { formatRelativeTime, formatDuration, mapRunStatus } from '../lib/format';

export interface SuiteRunRowData {
  id: string;
  status: 'RUNNING' | 'PASSED' | 'FAILED' | 'CANCELLED' | 'PENDING';
  branch: string;
  commit_sha?: string;
  commit_message?: string;
  config_source: 'repo_commit' | 'uncommitted';
  environment?: string;
  created_at: string;
  duration_ms?: number;
  initiator_type: 'ci' | 'manual' | 'schedule';
  initiator_name?: string;
}

interface SuiteRunRowProps {
  /** The run data to display */
  run: SuiteRunRowData;
  /** Callback when the row is clicked */
  onClick?: (runId: string) => void;
  /** Optional className for additional styling */
  className?: string;
}

/**
 * A clickable row displaying a suite run with status, branch, commit, and metadata.
 * Used in the Activity tab of suite detail pages.
 */
export function SuiteRunRow({ run, onClick, className = '' }: SuiteRunRowProps) {
  const status = mapRunStatus(run.status);

  // Prefer commit message, then "Commit <sha>", then "Manual run"
  const title = run.commit_message
    || (run.commit_sha ? `Commit ${run.commit_sha.slice(0, 7)}` : 'Manual run');

  const handleClick = () => {
    onClick?.(run.id);
  };

  return (
    <div
      onClick={handleClick}
      className={`p-4 hover:bg-[#fafafa] transition-colors cursor-pointer ${className}`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4 flex-1">
          {/* Status icon */}
          <RunStatusIcon status={status} />

          {/* Run info */}
          <div className="flex-1">
            <p className="text-sm mb-1 truncate max-w-lg">{title}</p>
            <div className="flex items-center gap-3 flex-wrap">
              <BranchDisplay branch={run.branch} />
              {/* For uncommitted runs: show Uncommitted badge, no commit SHA */}
              {/* For repo_commit runs: show commit SHA, no badge */}
              {run.config_source === 'uncommitted' ? (
                <ConfigSourceBadge type="uncommitted" />
              ) : (
                run.commit_sha && (
                  <span className="inline-flex items-center gap-1 text-xs text-[#666666] font-mono">
                    <Hash className="w-3 h-3" />
                    {run.commit_sha.slice(0, 7)}
                  </span>
                )
              )}
              <BadgeDot />
              {run.environment && <EnvBadge env={run.environment} />}
              <TriggerBadge trigger={run.initiator_type} />
              {run.initiator_type === 'manual' && run.initiator_name && (
                <UsernameBadge username={run.initiator_name} />
              )}
            </div>
          </div>

          {/* Metadata */}
          <div className="flex items-center gap-4 text-sm text-[#666666]">
            <span>{formatRelativeTime(run.created_at)}</span>
            {run.duration_ms !== undefined && (
              <span>{formatDuration(run.duration_ms)}</span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

/**
 * Status icon component for the run row
 */
function RunStatusIcon({ status }: { status: 'success' | 'failed' | 'running' }) {
  switch (status) {
    case 'success':
      return <CheckCircle2 className="w-5 h-5 text-[#4CBB17]" />;
    case 'failed':
      return <XCircle className="w-5 h-5 text-[#ef0000]" />;
    case 'running':
      return <Loader2 className="w-5 h-5 text-[#4CBB17] animate-spin" />;
  }
}

/**
 * Branch display with icon
 */
function BranchDisplay({ branch }: { branch: string }) {
  return (
    <span className="inline-flex items-center gap-1 text-xs text-[#666666]">
      <GitBranch className="w-3 h-3" />
      {branch}
    </span>
  );
}
