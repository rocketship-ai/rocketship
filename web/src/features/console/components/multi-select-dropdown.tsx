import { ChevronDown, X } from 'lucide-react';
import { useState } from 'react';

interface MultiSelectDropdownProps {
  label: string; // e.g., "Plugins", "Suites", "Projects"
  items: string[];
  selectedItems: string[];
  onSelectionChange: (items: string[]) => void;
  isOpen: boolean;
  onToggle: () => void;
  renderIcon?: (item: string) => React.ReactNode;
  /** When true, only allows single selection and closes dropdown on select */
  singleSelect?: boolean;
  /** Placeholder text when nothing is selected (for singleSelect mode) */
  placeholder?: string;
  /** Show "All X" option even in singleSelect mode. Defaults to true for multi-select, false for single-select */
  showAllOption?: boolean;
}

export function MultiSelectDropdown({
  label,
  items,
  selectedItems,
  onSelectionChange,
  isOpen,
  onToggle,
  renderIcon,
  singleSelect = false,
  placeholder,
  showAllOption,
}: MultiSelectDropdownProps) {
  // Default showAllOption: true for multi-select, false for single-select (unless explicitly set)
  const shouldShowAllOption = showAllOption !== undefined ? showAllOption : !singleSelect;
  const [searchQuery, setSearchQuery] = useState('');

  const toggleItem = (item: string) => {
    if (singleSelect) {
      // In single-select mode, replace selection and close dropdown
      onSelectionChange([item]);
      onToggle(); // Close dropdown
    } else {
      const newSelection = selectedItems.includes(item)
        ? selectedItems.filter((i) => i !== item)
        : [...selectedItems, item];
      onSelectionChange(newSelection);
    }
  };

  const clearAll = () => {
    onSelectionChange([]);
  };

  const filteredItems = items.filter((item) =>
    item.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const buttonLabel =
    selectedItems.length === 0
      ? (singleSelect ? (placeholder || `Select ${label.toLowerCase()}...`) : `All ${label}`)
      : selectedItems.length === 1
      ? selectedItems[0]
      : `${selectedItems.length} ${label.toLowerCase()}`;

  return (
    <div className="relative">
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between gap-2 px-3 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
      >
        <span className="text-sm">{buttonLabel}</span>
        <ChevronDown className="w-4 h-4 text-[#666666]" />
      </button>

      {isOpen && (
        <>
          <div className="fixed inset-0 z-10" onClick={onToggle} />
          <div className="absolute top-full left-0 mt-1 w-full min-w-[240px] bg-white border border-[#e5e5e5] rounded-md shadow-lg z-20">
            <div className="p-2 border-b border-[#e5e5e5]">
              <div className="relative">
                <input
                  type="text"
                  placeholder={`Search ${label.toLowerCase()}...`}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="w-full pl-3 pr-8 py-2 bg-white border border-[#e5e5e5] rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-black/5"
                  onClick={(e) => e.stopPropagation()}
                />
                {selectedItems.length > 0 && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      clearAll();
                    }}
                    className="absolute right-2 top-1/2 -translate-y-1/2 p-1 hover:bg-[#e5e5e5] rounded transition-colors"
                    title="Clear all"
                  >
                    <X className="w-3 h-3 text-[#666666]" />
                  </button>
                )}
              </div>
            </div>

            {/* "All" option - show when showAllOption is true */}
            {shouldShowAllOption && (
              <label className="flex items-center gap-2 px-3 py-2 hover:bg-[#fafafa] cursor-pointer border-b border-[#e5e5e5]">
                <input
                  type={singleSelect ? 'radio' : 'checkbox'}
                  name={singleSelect ? `${label}-select` : undefined}
                  checked={selectedItems.length === 0}
                  onChange={() => {
                    clearAll();
                    if (singleSelect) {
                      onToggle(); // Close dropdown when selecting "All" in single-select mode
                    }
                  }}
                  className="rounded border-[#e5e5e5] accent-black"
                  style={{ colorScheme: 'light' }}
                />
                <span className="text-sm">All {label}</span>
              </label>
            )}

            {/* Individual items */}
            <div className="max-h-64 overflow-y-scroll [scrollbar-width:thin] [scrollbar-color:#d4d4d4_white]">
              {filteredItems.map((item) => (
                <label
                  key={item}
                  className="flex items-center gap-2 px-3 py-2 hover:bg-[#fafafa] cursor-pointer"
                >
                  <input
                    type={singleSelect ? 'radio' : 'checkbox'}
                    name={singleSelect ? `${label}-select` : undefined}
                    checked={selectedItems.includes(item)}
                    onChange={() => toggleItem(item)}
                    className="rounded border-[#e5e5e5] accent-black"
                    style={{ colorScheme: 'light' }}
                  />
                  {renderIcon && renderIcon(item)}
                  <span className="text-sm">{item}</span>
                </label>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  );
}