import { ArrowRight, Loader2 } from 'lucide-react';

interface TestItemProps {
  test: {
    id: string;
    name: string;
    status?: 'success' | 'failed' | 'pending' | 'running';
    duration?: string;
    steps: Array<{
      name: string;
      method?: string;
      plugin?: string;
      status?: 'success' | 'failed' | 'pending';
    }>;
    /** Expected total steps from YAML (may differ from steps.length while test is running) */
    expectedStepCount?: number;
  };
  /** Whether the test is currently running/pending (enables placeholder steps) */
  isLive?: boolean;
  onClick?: () => void;
}

export function TestItem({ test, isLive = false, onClick }: TestItemProps) {
  // Use expected step count if available, otherwise fall back to reported steps length
  const expectedStepCount = test.expectedStepCount ?? test.steps.length;
  const reportedStepCount = test.steps.length;

  // Create combined steps array: real steps + placeholders for pending steps
  const allSteps = [...test.steps];
  if (isLive && reportedStepCount < expectedStepCount) {
    // Add placeholder steps for steps not yet reported
    for (let i = reportedStepCount; i < expectedStepCount; i++) {
      allSteps.push({
        name: `Step ${i + 1}`,
        plugin: 'pending',
        status: 'pending' as const,
      });
    }
  }

  const displaySteps = allSteps.slice(0, 5);
  const remainingSteps = allSteps.length - 5;

  // Find the first failed step for test runs
  const firstFailedStepIndex = test.steps.findIndex(s => s.status === 'failed');
  const failedStepNumber = firstFailedStepIndex !== -1 ? firstFailedStepIndex + 1 : null;

  // Determine border color based on status (for test runs) or default gray (for test definitions)
  // Running tests get light gray border (like pending) - the pulsating dot indicates activity
  const borderColorClass =
    test.status === 'success'
      ? 'border-l-[#4CBB17]'
      : test.status === 'failed'
      ? 'border-l-[#ef0000]'
      : test.status === 'running'
      ? 'border-l-[#d4d4d4]'
      : test.status === 'pending'
      ? 'border-l-[#d4d4d4]'
      : 'border-l-[#666666]'; // default for test definitions

  const isRunning = test.status === 'running';

  return (
    <div
      onClick={onClick}
      className={`bg-white rounded-lg border border-[#e5e5e5] ${borderColorClass} border-l-4 shadow-sm p-4 hover:shadow-md hover:bg-[#fafafa] transition-all cursor-pointer`}
    >
      <div className="flex flex-col gap-3">
        {/* Test name and metadata */}
        <div>
          <div className="flex items-center gap-2 mb-1">
            {isRunning && (
              <span className="relative flex h-2.5 w-2.5">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-[#4CBB17] opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-[#4CBB17]"></span>
              </span>
            )}
            <p className="text-base">{test.name}</p>
          </div>
          <div className="flex items-center gap-3 text-sm text-[#666666]">
            {test.duration && (
              <>
                <span>{test.duration}</span>
                <span>•</span>
              </>
            )}
            <span>{expectedStepCount} {expectedStepCount === 1 ? 'step' : 'steps'}</span>
            
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
            const isPending = step.plugin === 'pending';
            const pluginName = step.plugin || 'HTTP';

            return (
              <div key={idx} className="flex items-center gap-2">
                {isPending ? (
                  // Placeholder step chip for steps not yet reported
                  <div className="flex items-center gap-1.5 px-1.5 py-0.5 bg-[#fafafa] rounded border border-dashed border-[#d5d5d5]">
                    <Loader2 className="w-3 h-3 animate-spin text-[#999999]" />
                    <span className="text-xs text-[#999999]">{step.name}</span>
                  </div>
                ) : (
                  // Normal step chip
                  <div className="flex items-center gap-1.5 px-1.5 py-0.5 bg-[#f5f5f5] rounded border border-[#e5e5e5]">
                    <span className="text-xs font-mono text-[#666666]">{pluginName}</span>
                    <span className="text-xs text-[#999999]">{step.name}</span>
                  </div>
                )}
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