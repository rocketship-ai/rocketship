import { AlertCircle } from 'lucide-react';

interface InfoLabelProps {
  children: React.ReactNode;
}

export function InfoLabel({ children }: InfoLabelProps) {
  return (
    <div className="bg-[#fafafa] border border-[#e5e5e5] rounded-lg p-4">
      <div className="flex items-start gap-3">
        <AlertCircle className="w-5 h-5 text-[#666666] flex-shrink-0 mt-0.5" />
        <div>
          <p className="text-sm text-[#666666]">
            {children}
          </p>
        </div>
      </div>
    </div>
  );
}
