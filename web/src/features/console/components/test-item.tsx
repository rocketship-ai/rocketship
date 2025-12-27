import { ArrowRight } from 'lucide-react';

interface TestItemProps {
  test: {
    id: string;
    name: string;
    status?: 'success' | 'failed' | 'pending';
    duration?: string;
    steps: Array<{
      name: string;
      method?: string;
      plugin?: string;
      status?: 'success' | 'failed' | 'pending';
    }>;
  };
  onClick?: () => void;
}

export function TestItem({ test, onClick }: TestItemProps) {
  const displaySteps = test.steps.slice(0, 5);
  const remainingSteps = test.steps.length - 5;
  
  // Find the first failed step for test runs
  const firstFailedStepIndex = test.steps.findIndex(s => s.status === 'failed');
  const failedStepNumber = firstFailedStepIndex !== -1 ? firstFailedStepIndex + 1 : null;
  
  // Determine border color based on status (for test runs) or default gray (for test definitions)
  const borderColorClass = 
    test.status === 'success' 
      ? 'border-l-[#4CBB17]' 
      : test.status === 'failed' 
      ? 'border-l-[#ef0000]' 
      : test.status === 'pending'
      ? 'border-l-[#999999]'
      : 'border-l-[#666666]'; // default for test definitions

  return (
    <div
      onClick={onClick}
      className={`bg-white rounded-lg border border-[#e5e5e5] ${borderColorClass} border-l-4 shadow-sm p-4 hover:shadow-md hover:bg-[#fafafa] transition-all cursor-pointer`}
    >
      <div className="flex flex-col gap-3">
        {/* Test name and metadata */}
        <div>
          <p className="text-base mb-1">{test.name}</p>
          <div className="flex items-center gap-3 text-sm text-[#666666]">
            {test.duration && (
              <>
                <span>{test.duration}</span>
                <span>•</span>
              </>
            )}
            <span>{test.steps.length} {test.steps.length === 1 ? 'step' : 'steps'}</span>
            
            {/* Show which step failed for test runs */}
            {failedStepNumber && (
              <>
                <span>•</span>
                <span className="text-[#ef0000]">step {failedStepNumber} failed</span>
              </>
            )}
          </div>
        </div>

        {/* HTTP Steps flow */}
        <div className="flex items-center gap-2 flex-wrap">
          {displaySteps.map((step, idx) => {
            const pluginName = step.plugin || 'HTTP';
            
            return (
              <div key={idx} className="flex items-center gap-2">
                <div className="flex items-center gap-1.5 px-1.5 py-0.5 bg-[#f5f5f5] rounded border border-[#e5e5e5]">
                  <span className="text-xs font-mono text-[#666666]">{pluginName}</span>
                  <span className="text-xs text-[#999999]">{step.name}</span>
                </div>
                {idx < displaySteps.length - 1 && (
                  <ArrowRight className="w-3 h-3 text-[#999999]" />
                )}
              </div>
            );
          })}
          {remainingSteps > 0 && (
            <>
              <ArrowRight className="w-3 h-3 text-[#999999]" />
              <span className="text-xs text-[#999999] px-1.5 py-0.5 bg-[#f5f5f5] rounded border border-[#e5e5e5]">
                +{remainingSteps} more
              </span>
            </>
          )}
        </div>
      </div>
    </div>
  );
}