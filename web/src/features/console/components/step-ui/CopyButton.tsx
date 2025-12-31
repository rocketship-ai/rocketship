import { useState } from 'react';
import { Copy, Check, CheckCircle2 } from 'lucide-react';

interface CopyButtonProps {
  text: string;
  className?: string;
  /** Use 'small' for inline/table contexts, 'default' for standalone buttons */
  variant?: 'default' | 'small';
}

export function CopyButton({ text, className = '', variant = 'default' }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback for older browsers or non-HTTPS contexts
      const textArea = document.createElement('textarea');
      textArea.value = text;
      textArea.style.position = 'fixed';
      textArea.style.left = '-999999px';
      document.body.appendChild(textArea);
      textArea.select();
      try {
        document.execCommand('copy');
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      } catch {
        console.error('Failed to copy to clipboard');
      }
      document.body.removeChild(textArea);
    }
  };

  if (variant === 'small') {
    return (
      <button
        onClick={handleCopy}
        className={`text-[#999999] hover:text-black transition-colors p-1 ${className}`}
        title="Copy to clipboard"
      >
        {copied ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
      </button>
    );
  }

  return (
    <button
      onClick={handleCopy}
      className={`p-1 text-[#999999] hover:text-[#666666] transition-colors ${className}`}
      title="Copy"
    >
      {copied ? (
        <CheckCircle2 className="w-4 h-4 text-[#4CBB17]" />
      ) : (
        <Copy className="w-4 h-4" />
      )}
    </button>
  );
}
