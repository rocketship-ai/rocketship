import { Loader2 } from 'lucide-react';

interface LoadingStateProps {
  message?: string;
  className?: string;
}

export function LoadingState({ message = 'Loading...', className = '' }: LoadingStateProps) {
  return (
    <div className={`flex items-center justify-center py-12 ${className}`}>
      <Loader2 className="w-6 h-6 animate-spin text-[#666666]" />
      <span className="ml-3 text-[#666666]">{message}</span>
    </div>
  );
}
