import type { ReactNode } from 'react';

interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className = '',
}: EmptyStateProps) {
  return (
    <div className={`bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center ${className}`}>
      {icon && (
        <div className="mx-auto mb-4 text-[#999999]">
          {icon}
        </div>
      )}
      <h3 className="text-lg font-medium mb-2">{title}</h3>
      {description && (
        <p className="text-[#666666] text-sm mb-4">{description}</p>
      )}
      {action}
    </div>
  );
}
