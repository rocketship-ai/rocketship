import { CopyButton } from './CopyButton';

interface CodeBlockProps {
  code: string;
  /** Optional label above the code block */
  label?: string;
  /** Whether to show a copy button (default: true) */
  showCopy?: boolean;
  /** Max height before scrolling (default: no limit) */
  maxHeight?: string;
  className?: string;
}

export function CodeBlock({
  code,
  label,
  showCopy = true,
  maxHeight,
  className = ''
}: CodeBlockProps) {
  const style = maxHeight ? { maxHeight } : undefined;

  return (
    <div className={className}>
      {(label || showCopy) && (
        <div className="flex items-center justify-between mb-1.5">
          {label && <span className="text-xs text-[#888888]">{label}</span>}
          {showCopy && <CopyButton text={code} />}
        </div>
      )}
      <pre
        className="bg-[#f8f8f8] rounded border border-[#e8e8e8] px-3 py-2.5 font-mono text-sm text-[#1a1a1a] overflow-x-auto whitespace-pre-wrap overflow-y-auto"
        style={style}
      >
        {code}
      </pre>
    </div>
  );
}

/** Inline code display with copy button - for URLs, single values, etc. */
interface InlineCodeProps {
  code: string;
  label?: string;
  showCopy?: boolean;
  className?: string;
}

export function InlineCode({ code, label, showCopy = true, className = '' }: InlineCodeProps) {
  return (
    <div className={className}>
      {(label || showCopy) && (
        <div className="flex items-center justify-between mb-1.5">
          {label && <span className="text-xs text-[#888888]">{label}</span>}
          {showCopy && <CopyButton text={code} />}
        </div>
      )}
      <code className="block bg-[#f8f8f8] rounded border border-[#e8e8e8] px-3 py-2.5 font-mono text-sm text-[#1a1a1a]">
        {code}
      </code>
    </div>
  );
}
