import type { ReactNode } from 'react';

interface FilterBarProps {
  children: ReactNode;
  className?: string;
}

/**
 * FilterBar is a layout container for search input and filter dropdowns.
 *
 * Usage:
 * ```tsx
 * <FilterBar>
 *   <SearchInput value={search} onChange={setSearch} placeholder="Search..." />
 *   <MultiSelectDropdown ... />
 * </FilterBar>
 * ```
 */
export function FilterBar({ children, className = '' }: FilterBarProps) {
  return (
    <div className={`flex items-center gap-3 mb-6 ${className}`}>
      {children}
    </div>
  );
}
