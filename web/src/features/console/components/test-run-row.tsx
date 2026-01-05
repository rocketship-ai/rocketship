import { GitBranch } from 'lucide-react';
import { StatusBadge, EnvBadge, TriggerBadge, UsernameBadge, BadgeDot } from './status-badge';
import { formatDuration, formatRelativeTime } from '../lib/format';
import { useLiveDurationMs } from '../hooks/use-live-duration';

export interface TestRunRowData {
  id: string;
  run_id: string;
  status: 'RUNNING' | 'PASSED' | 'FAILED' | 'CANCELLED' | 'PENDING' | 'TIMEOUT';
  trigger: 'ci' | 'manual' | 'schedule';
  environment: string;
  branch: string;
  initiator: string;
  initiator_name?: string;
  commit_sha?: string;
  duration_ms?: number;
  created_at: string;
  started_at?: string;
  ended_at?: string;
}

interface TestRunRowProps {
  run: TestRunRowData;
  onClick?: (runId: string) => void;
}

// Map API status to UI status
function mapRunStatus(status: string): 'pending' | 'success' | 'failed' | 'running' {
  switch (status.toUpperCase()) {
    case 'PASSED':
      return 'success';
    case 'FAILED':
    case 'CANCELLED':
    case 'TIMEOUT':
      return 'failed';
    case 'RUNNING':
      return 'running';
    case 'PENDING':
    default:
      return 'pending';
  }
}

// Get status display text
function getStatusText(status: 'pending' | 'success' | 'failed' | 'running'): string {
  switch (status) {
    case 'success':
      return 'Passed';
    case 'failed':
      return 'Failed';
    case 'running':
      return 'Running';
    case 'pending':
    default:
      return 'Pending';
  }
}

/**
 * A clickable row displaying a test run with status, environment, and metadata.
 * Used in the Recent Runs panel on the Test Detail page.
 */
export function TestRunRow({ run, onClick }: TestRunRowProps) {
  const status = mapRunStatus(run.status);
  const isLive = run.status === 'RUNNING';
  const isManual = run.trigger === 'manual';
  const isSchedule = run.trigger === 'schedule';

  // Live duration for running scheduled tests - shows elapsed time ticking upward
  const liveDurationMs = useLiveDurationMs({
    startedAt: run.started_at,
    endedAt: run.ended_at,
    isLive: isLive && isSchedule, // Only tick for running schedule triggers
    durationMs: run.duration_ms,
    interval: 1000, // Update every second for smoother display
  });

  const handleClick = () => {
    onClick?.(run.id);
  };

  return (
    <div
      onClick={handleClick}
      className="p-4 hover:bg-[#fafafa] transition-colors cursor-pointer border-b border-[#e5e5e5] last:border-b-0"
    >
      {/* Row 1: Status + text on left, time on right */}
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-2">
          <StatusBadge status={status} isLive={isLive} />
          <span className="text-sm">{getStatusText(status)}</span>
        </div>
        <span className="text-xs text-[#999999]">
          {formatRelativeTime(run.started_at || run.created_at)}
        </span>
      </div>

      {/* Row 2: Env badge (left) + Trigger badge (right) */}
      <div className="flex items-center gap-2 mb-2">
        {run.environment && <EnvBadge env={run.environment} />}
        <TriggerBadge trigger={run.trigger} />
      </div>

      {/* Row 3: Branch/username (manual) or duration (non-manual) */}
      <div className="flex items-center gap-2 flex-wrap">
        {isManual ? (
          <>
            {run.branch && (
              <span className="inline-flex items-center gap-1 text-xs text-[#666666]">
                <GitBranch className="w-3 h-3" />
                {run.branch}
              </span>
            )}
            {run.initiator_name && (
              <>
                <BadgeDot />
                <UsernameBadge username={run.initiator_name} />
              </>
            )}
          </>
        ) : (
          <>
            {liveDurationMs !== undefined && (
              <span className="text-xs text-[#666666]">{formatDuration(liveDurationMs)}</span>
            )}
          </>
        )}
      </div>
    </div>
  );
}
