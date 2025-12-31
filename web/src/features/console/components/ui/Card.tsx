import type { ReactNode, HTMLAttributes } from 'react';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  padding?: 'sm' | 'md' | 'lg';
}

const paddingStyles = {
  sm: 'p-4',
  md: 'p-6',
  lg: 'p-8',
};

export function Card({ children, padding = 'md', className = '', ...props }: CardProps) {
  return (
    <div
      className={`bg-white rounded-lg border border-[#e5e5e5] shadow-sm ${paddingStyles[padding]} ${className}`}
      {...props}
    >
      {children}
    </div>
  );
}
