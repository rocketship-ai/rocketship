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

  return (
    <div className={`flex items-center ${gap}`}>
      {displayResults.map((result, index) => {
        const isLastPill = index === displayResults.length - 1;
        const isLivePill = isLastPill && isLive && (result === 'pending' || result === 'running');

        return (
          <div
            key={index}
            className={`${pillSize} rounded-full flex-shrink-0 ${isLivePill ? 'animate-pulse' : ''}`}
            style={{ backgroundColor: colorMap[result] }}
            title={result}
          />
        );
      })}
    </div>
  );
}
