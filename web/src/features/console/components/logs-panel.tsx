import { Loader2 } from 'lucide-react';
import { CopyButton } from './step-ui';

interface LogsPanelProps {
  logs: string;
  isLoading?: boolean;
}

export function LogsPanel({ logs, isLoading = false }: LogsPanelProps) {
  return (
    <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
      <div className="flex justify-end mb-4">
        <CopyButton text={logs} />
      </div>
      {isLoading ? (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="w-5 h-5 animate-spin text-[#666666]" />
          <span className="ml-2 text-[#666666]">Loading logs...</span>
        </div>
      ) : (
        <pre className="bg-black rounded p-4 font-mono text-xs text-[#00ff00] overflow-x-auto max-h-96 overflow-y-auto whitespace-pre-wrap">
          {logs}
        </pre>
      )}
    </div>
  );
}
