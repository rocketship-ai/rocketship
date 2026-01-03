import { Edit2, X } from 'lucide-react';
import { EnvBadge } from './status-badge';

export interface ScheduleCardData {
  id: string;
  name: string;
  /** Environment slug for display */
  env: string;
  /** Cron expression (e.g., "0 30 * * *") */
  cron: string;
  /** Timezone (e.g., "UTC", "America/New_York") */
  timezone: string;
  /** Whether the schedule is enabled */
  enabled: boolean;
  /** Formatted relative time of last run (e.g., "2h ago", "Never") */
  lastRun: string;
  /** Formatted relative time of next run (e.g., "in 30m", "Not scheduled") */
  nextRun: string;
  /** Last run status (e.g., "PASSED", "FAILED") */
  lastRunStatus?: string | null;
}

interface ScheduleCardProps {
  /** The schedule data to display */
  schedule: ScheduleCardData;
  /** Callback when the edit button is clicked */
  onEdit?: (scheduleId: string) => void;
  /** Callback when the delete button is clicked */
  onDelete?: (scheduleId: string) => void;
  /** Whether the card is in a loading/disabled state */
  disabled?: boolean;
  /** Optional className for additional styling */
  className?: string;
}

/**
 * A card component displaying schedule metadata with edit/delete actions.
 * Used in the Schedules tab of suite detail pages.
 */
export function ScheduleCard({
  schedule,
  onEdit,
  onDelete,
  disabled = false,
  className = '',
}: ScheduleCardProps) {
  const handleEdit = () => {
    if (!disabled) {
      onEdit?.(schedule.id);
    }
  };

  const handleDelete = () => {
    if (!disabled) {
      onDelete?.(schedule.id);
    }
  };

  return (
    <div
      className={`bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 ${className}`}
    >
      <div className="flex items-start justify-between">
        <div className="flex-1">
          {/* Header row: name, env, cron, timezone, enabled status */}
          <div className="flex items-center gap-3 mb-2 flex-wrap">
            <span className="font-medium">{schedule.name}</span>
            <EnvBadge env={schedule.env} />
            <code className="text-xs font-mono px-2 py-0.5 bg-[#fafafa] rounded border border-[#e5e5e5] text-[#666666]">
              {schedule.cron}
            </code>
            <span className="text-xs text-[#999999]">{schedule.timezone}</span>
            <EnabledBadge enabled={schedule.enabled} />
          </div>

          {/* Info row: last run, last run status, next run */}
          <div className="flex items-center gap-4 text-sm text-[#666666]">
            <span>Last run: {schedule.lastRun}</span>
            {schedule.lastRunStatus && (
              <>
                <span>•</span>
                <LastRunStatus status={schedule.lastRunStatus} />
              </>
            )}
            {schedule.enabled && (
              <>
                <span>•</span>
                <span>Next run: {schedule.nextRun}</span>
              </>
            )}
          </div>
        </div>

        {/* Action buttons */}
        {(onEdit || onDelete) && (
          <div className="flex items-center gap-2">
            {onEdit && (
              <button
                onClick={handleEdit}
                disabled={disabled}
                className="p-2 text-[#666666] hover:text-black transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                aria-label="Edit schedule"
              >
                <Edit2 className="w-4 h-4" />
              </button>
            )}
            {onDelete && (
              <button
                onClick={handleDelete}
                disabled={disabled}
                className="p-2 text-[#666666] hover:text-red-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                aria-label="Delete schedule"
              >
                <X className="w-4 h-4" />
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * Badge showing enabled/disabled status
 */
function EnabledBadge({ enabled }: { enabled: boolean }) {
  return (
    <span
      className={`text-xs px-2 py-0.5 rounded border ${
        enabled
          ? 'bg-green-50 text-green-700 border-green-200'
          : 'bg-gray-50 text-gray-700 border-gray-200'
      }`}
    >
      {enabled ? 'Enabled' : 'Disabled'}
    </span>
  );
}

/**
 * Status display for last run
 */
function LastRunStatus({ status }: { status: string }) {
  const getStatusClass = () => {
    switch (status.toUpperCase()) {
      case 'PASSED':
      case 'RUNNING':
        return 'text-green-600';
      case 'FAILED':
        return 'text-red-600';
      default:
        return 'text-[#666666]';
    }
  };

  return <span className={getStatusClass()}>{status}</span>;
}
