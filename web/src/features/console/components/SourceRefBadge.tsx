interface SourceRefBadgeProps {
  sourceRef: string;
}

export function SourceRefBadge({ sourceRef }: SourceRefBadgeProps) {
  const isPR = sourceRef.startsWith('pr/');
  const displayText = isPR ? `#${sourceRef.slice(3)}` : sourceRef;
  return (
    <span className={`text-xs px-2 py-0.5 rounded border ${
      isPR
        ? 'bg-amber-50 text-amber-700 border-amber-200'
        : 'bg-gray-50 text-gray-700 border-gray-200'
    }`}>
      {displayText}
    </span>
  );
}
