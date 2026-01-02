import { useState } from 'react';
import { X, Copy, Check, AlertTriangle } from 'lucide-react';

interface CITokenRevealModalProps {
  isOpen: boolean;
  onClose: () => void;
  tokenValue: string;
  tokenName: string;
}

export function CITokenRevealModal({
  isOpen,
  onClose,
  tokenValue,
  tokenName,
}: CITokenRevealModalProps) {
  const [copied, setCopied] = useState(false);

  if (!isOpen) return null;

  const copyToClipboard = async (text: string) => {
    // Prefer the modern clipboard API when available and permitted.
    if (navigator.clipboard?.writeText && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return;
    }

    // Fallback for non-secure contexts (e.g. http:// in local dev).
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', 'true');
    textarea.style.position = 'fixed';
    textarea.style.top = '0';
    textarea.style.left = '0';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(textarea);
    if (!ok) {
      throw new Error('copy command failed');
    }
  };

  const handleCopy = async () => {
    try {
      await copyToClipboard(tokenValue);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy token:', err);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-lg w-full max-w-lg mx-4">
        <div className="flex items-center justify-between p-4 border-b border-[#e5e5e5]">
          <h2 className="text-lg font-semibold">CI Token Created</h2>
          <button
            onClick={onClose}
            className="p-1 hover:bg-[#f5f5f5] rounded-md transition-colors"
          >
            <X className="w-5 h-5 text-[#666666]" />
          </button>
        </div>

        <div className="p-4 space-y-4">
          <div className="flex items-start gap-3 p-3 bg-[#fff8e6] border border-[#f5d180] rounded-md">
            <AlertTriangle className="w-5 h-5 text-[#b8860b] flex-shrink-0 mt-0.5" />
            <div className="text-sm">
              <p className="font-medium text-[#8b6914]">Copy this token now</p>
              <p className="text-[#8b6914]/80 mt-1">
                This is the only time you'll see this token. Store it securely.
              </p>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium mb-1">Token Name</label>
            <p className="text-sm text-[#666666]">{tokenName}</p>
          </div>

          <div>
            <label className="block text-sm font-medium mb-1">Token Value</label>
            <div className="flex gap-2">
              <code className="flex-1 px-3 py-2 bg-[#f5f5f5] border border-[#e5e5e5] rounded-md text-sm font-mono break-all">
                {tokenValue}
              </code>
              <button
                onClick={handleCopy}
                className="px-3 py-2 bg-[#f5f5f5] border border-[#e5e5e5] rounded-md hover:bg-[#e5e5e5] transition-colors flex-shrink-0"
                title="Copy to clipboard"
              >
                {copied ? (
                  <Check className="w-4 h-4 text-green-600" />
                ) : (
                  <Copy className="w-4 h-4 text-[#666666]" />
                )}
              </button>
            </div>
          </div>

          <div className="pt-4 border-t border-[#e5e5e5]">
            <button
              onClick={onClose}
              className="w-full px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors"
            >
              Done
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
