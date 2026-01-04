interface SparklineProps {
  results: readonly ('success' | 'failed' | 'pending' | 'running')[];
  size?: 'sm' | 'md' | 'lg';
  maxItems?: number; // Maximum number of items to display (default 20)
  isLive?: boolean; // Show live indicator (flashing rightmost pill)
}

export function Sparkline({ results, size = 'md', maxItems = 20, isLive = false }: SparklineProps) {
  // Size classes for vertical pills - match north-star mockup sizing
  // Pills are tall and narrow with fully rounded corners
  const pillSize = size === 'sm'
    ? 'w-[5px] h-[16px]'
    : size === 'lg'
    ? 'w-[8px] h-[24px]'
    : 'w-[6px] h-[20px]';
  const gap = size === 'lg' ? 'gap-[3px]' : 'gap-[2px]';

  // Only green (success/live) and red (failed) - no gray/yellow
  const colorMap = {
    success: '#22c55e', // Tailwind green-500
    failed: '#ef4444',  // Tailwind red-500
    pending: '#22c55e', // Live runs show as green (flashing)
    running: '#22c55e', // Live runs show as green (flashing)
  };

  // Limit to maxItems, then reverse so newest is on the right
  // API returns newest-first, we want oldest-left newest-right
  const displayResults = [...results].slice(0, maxItems).reverse();

  // Compute how many contiguous RUNNING results are at the right edge (the "live tail")
  // Walk from the end backward counting running items until the first non-running result
  // Note: PENDING does NOT flash - only RUNNING pills animate
  let liveTailCount = 0;
  if (isLive) {
    for (let i = displayResults.length - 1; i >= 0; i--) {
      const r = displayResults[i];
      if (r === 'running') {
        liveTailCount++;
      } else {
        break;
      }
    }
  }

  return (
    <div className={`flex items-center ${gap}`}>
      {displayResults.map((result, index) => {
        // Animate if this pill is within the live tail (rightmost contiguous live results)
        const isLivePill = liveTailCount > 0 && index >= displayResults.length - liveTailCount;

        return (
          <div
            key={index}
            className={`${pillSize} rounded-full flex-shrink-0 ${isLivePill ? 'animate-blink' : ''}`}
            style={{ backgroundColor: colorMap[result] }}
            title={result}
          />
        );
      })}
    </div>
  );
}
