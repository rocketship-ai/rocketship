interface SparklineProps {
  results: readonly ('success' | 'failed' | 'pending' | 'running')[];
  size?: 'sm' | 'md' | 'lg';
  shape?: 'circle' | 'pill';
  maxItems?: number; // Maximum number of items to display, will fill with gray if fewer results
}

export function Sparkline({ results, size = 'md', shape = 'circle', maxItems }: SparklineProps) {
  const dotSize = size === 'sm' ? 'w-1.5 h-1.5' : size === 'lg' ? 'w-2.5 h-2.5' : 'w-2 h-2';
  const pillSize = size === 'sm' ? 'w-1.5 h-4' : size === 'lg' ? 'w-2.5 h-6' : 'w-2 h-5';
  const gap = size === 'lg' ? 'gap-1' : 'gap-0.5';

  const colorMap = {
    success: '#4CBB17',
    failed: '#ef0000',
    pending: '#999999',
    running: '#fbbf24', // Amber/yellow for running
    empty: '#e5e5e5', // Gray for empty data
  };

  // If maxItems is specified, pad the results with empty items
  const displayResults = maxItems
    ? [...Array(Math.max(0, maxItems - results.length)).fill('empty' as const), ...results]
    : results;

  return (
    <div className={`flex items-center ${gap} w-full`}>
      {displayResults.map((result, index) => (
        <div
          key={index}
          className={`${shape === 'pill' ? pillSize : dotSize} ${shape === 'pill' ? 'rounded-full' : 'rounded-full'}`}
          style={{ backgroundColor: colorMap[result as keyof typeof colorMap] }}
          title={result === 'empty' ? 'No data' : result}
        />
      ))}
    </div>
  );
}