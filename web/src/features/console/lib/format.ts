/**
 * Shared formatting utilities for the console UI
 */

// =============================================================================
// URL Formatting
// =============================================================================

/** Extract domain and path from a URL for display */
export function getUrlParts(url: string): { domain: string; path: string } {
  try {
    const parsed = new URL(url);
    return {
      domain: parsed.host,
      path: parsed.pathname + parsed.search,
    };
  } catch {
    return { domain: url, path: '' };
  }
}

/** Try to format a string as JSON, returns original if not valid JSON */
export function tryFormatJSON(str: string): string {
  try {
    const parsed = JSON.parse(str);
    return JSON.stringify(parsed, null, 2);
  } catch {
    return str;
  }
}

// =============================================================================
// Duration & Time Formatting
// =============================================================================

/** Format duration from milliseconds to human-readable string */
export function formatDuration(ms?: number): string {
  if (!ms) return '—';
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
}

/** Alias for formatDuration - clearer naming */
export const formatDurationMs = formatDuration;

/** Format ISO date string to localized date/time */
export function formatDateTime(isoString?: string): string {
  if (!isoString) return '—';
  const date = new Date(isoString);
  return date.toLocaleString('en-US', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
}

/** Format ISO date string to relative time (e.g., "2h ago", "3d ago") */
export function formatRelativeTime(dateStr?: string): string {
  if (!dateStr) return '—';

  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / (1000 * 60));
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

// =============================================================================
// HTTP Status Formatting
// =============================================================================

/** HTTP status code color mapping */
export function getStatusCodeColor(statusCode: number): string {
  if (statusCode >= 200 && statusCode < 300) {
    return 'text-[#4CBB17]';
  }
  if (statusCode >= 400) {
    return 'text-[#ef0000]';
  }
  return 'text-[#f6a724]'; // 3xx redirects, 1xx informational
}

/** HTTP status code background color mapping */
export function getStatusCodeBgColor(statusCode: number): string {
  if (statusCode >= 200 && statusCode < 300) {
    return 'bg-[#4CBB17]/10 text-[#4CBB17]';
  }
  if (statusCode >= 400) {
    return 'bg-[#ef0000]/10 text-[#ef0000]';
  }
  return 'bg-[#f6a724]/10 text-[#f6a724]';
}

// =============================================================================
// Status Mapping Utilities
// =============================================================================

/** Map API status to UI status for run display */
export function mapRunStatus(status: string): 'success' | 'failed' | 'running' {
  switch (status.toUpperCase()) {
    case 'PASSED':
      return 'success';
    case 'RUNNING':
    case 'PENDING':
      return 'running';
    case 'FAILED':
    case 'CANCELLED':
    default:
      return 'failed';
  }
}

/** Map API status to TestItem status (pending, not running) */
export function mapTestStatus(status: string): 'success' | 'failed' | 'pending' {
  switch (status.toUpperCase()) {
    case 'PASSED':
      return 'success';
    case 'FAILED':
    case 'CANCELLED':
      return 'failed';
    default:
      return 'pending';
  }
}

/** Map API status to TestItem status with live running state */
export function mapTestStatusLive(status: string): 'success' | 'failed' | 'pending' | 'running' {
  switch (status.toUpperCase()) {
    case 'PASSED':
      return 'success';
    case 'FAILED':
    case 'CANCELLED':
    case 'TIMEOUT':
      return 'failed';
    case 'RUNNING':
      return 'running';
    default:
      return 'pending';
  }
}

/**
 * Map step status for summary display (e.g., test item chips).
 * Returns only success/failed/pending - no running state.
 * For full step lifecycle status including running, use mapStepStatus from run-steps/types.ts
 */
export function mapStepStatusForSummary(status: string): 'success' | 'failed' | 'pending' {
  switch (status.toUpperCase()) {
    case 'PASSED':
      return 'success';
    case 'FAILED':
      return 'failed';
    case 'RUNNING':
    case 'PENDING':
    default:
      return 'pending';
  }
}

// =============================================================================
// Live Status Helpers (for polling logic)
// =============================================================================

/**
 * Returns true if a run status indicates the run is still active (should poll).
 * RUNNING and PENDING are live states.
 */
export function isLiveRunStatus(status: string): boolean {
  const upper = status.toUpperCase();
  return upper === 'RUNNING' || upper === 'PENDING';
}

/**
 * Returns true if a test status indicates the test is still active (should poll).
 * PENDING and RUNNING are live states for tests.
 */
export function isLiveTestStatus(status: string): boolean {
  const upper = status.toUpperCase();
  return upper === 'PENDING' || upper === 'RUNNING';
}

/**
 * Returns true if a step status indicates the step is still active (should poll).
 * PENDING and RUNNING are live states for steps.
 */
export function isLiveStepStatus(status: string): boolean {
  const upper = status.toUpperCase();
  return upper === 'PENDING' || upper === 'RUNNING';
}

