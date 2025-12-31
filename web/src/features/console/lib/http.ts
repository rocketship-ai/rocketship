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

/** Format duration from milliseconds to human-readable string */
export function formatDuration(ms?: number): string {
  if (!ms) return '—';
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return `${minutes}m ${remainingSeconds}s`;
}

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
