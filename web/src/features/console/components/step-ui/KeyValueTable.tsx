import { CopyButton } from './CopyButton';

export interface KeyValueRow {
  key: string;
  value: string;
  /** If provided, allows copying a different value than displayed */
  copyText?: string;
  /** If true, masks the value display (for sensitive headers) */
  masked?: boolean;
}

interface KeyValueTableProps {
  rows: KeyValueRow[];
  /** Optional header for the copy button to copy all headers */
  copyAllText?: string;
  /** Optional label above the table */
  label?: string;
  className?: string;
}

export function KeyValueTable({ rows, copyAllText, label, className = '' }: KeyValueTableProps) {
  if (rows.length === 0) return null;

  return (
    <div className={className}>
      {(label || copyAllText) && (
        <div className="flex items-center justify-between mb-1.5">
          {label && <span className="text-xs text-[#888888]">{label}</span>}
          {copyAllText && <CopyButton text={copyAllText} />}
        </div>
      )}
      <div className="rounded border border-[#e8e8e8] overflow-hidden">
        <table className="w-full text-sm">
          <tbody>
            {rows.map((row, index) => (
              <tr key={row.key} className={index % 2 === 0 ? 'bg-[#f8f8f8]' : 'bg-white'}>
                <td className="px-3 py-2 font-mono text-[#1a1a1a] whitespace-nowrap w-1/3">{row.key}</td>
                <td className="px-3 py-2 font-mono text-[#666666] break-all">{row.value}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/** Helper to convert Record<string, string> to KeyValueRow[] */
export function headersToRows(
  headers: Record<string, string> | undefined,
  options?: { maskSensitive?: boolean }
): KeyValueRow[] {
  if (!headers) return [];

  return Object.entries(headers).map(([key, value]) => ({
    key,
    value: options?.maskSensitive ? maskSensitiveValue(key, value) : value,
    copyText: value, // Always allow copying the real value
  }));
}

/** Masks sensitive header values like Authorization */
function maskSensitiveValue(key: string, value: string): string {
  const sensitiveKeys = ['authorization', 'x-api-key', 'api-key', 'apikey', 'token', 'secret'];
  if (sensitiveKeys.some(k => key.toLowerCase().includes(k))) {
    if (value.length > 20) {
      return value.substring(0, 15) + '*'.repeat(20);
    }
    return value.substring(0, 5) + '*'.repeat(Math.max(0, value.length - 5));
  }
  return value;
}
