interface SourceRefBadgeProps {
  sourceRef: string;
  defaultBranch?: string;
}

export function SourceRefBadge({ sourceRef, defaultBranch }: SourceRefBadgeProps) {
  // Determine if this is the default branch
  // Use provided defaultBranch, or fall back to common defaults
  const isDefaultBranch = defaultBranch
    ? sourceRef === defaultBranch
    : sourceRef === 'main' || sourceRef === 'master';

  // Don't show badge for default branch
  if (isDefaultBranch) {
    return null;
  }

  // Show amber badge for feature branches
  return (
    <span className="text-xs px-2 py-0.5 rounded border bg-amber-50 text-amber-700 border-amber-200">
      {sourceRef}
    </span>
  );
}
