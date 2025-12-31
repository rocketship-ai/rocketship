export interface TabDefinition {
  id: string;
  label: string;
  badge?: string | number;
  hidden?: boolean;
}

interface TabsProps {
  tabs: TabDefinition[];
  activeId: string;
  onChange: (id: string) => void;
  className?: string;
}

export function Tabs({ tabs, activeId, onChange, className = '' }: TabsProps) {
  const visibleTabs = tabs.filter(tab => !tab.hidden);

  return (
    <div className={`flex items-center border-b border-[#e5e5e5] ${className}`}>
      {visibleTabs.map(tab => (
        <button
          key={tab.id}
          onClick={(e) => {
            e.stopPropagation();
            onChange(tab.id);
          }}
          className={`px-3 py-2.5 text-sm transition-colors border-b-2 -mb-px ${
            activeId === tab.id
              ? 'text-[#1a1a1a] border-[#1a1a1a] font-medium'
              : 'text-[#888888] border-transparent hover:text-[#1a1a1a]'
          }`}
        >
          {tab.label}
          {tab.badge !== undefined && (
            <span className="text-[#888888] ml-1">{tab.badge}</span>
          )}
        </button>
      ))}
    </div>
  );
}
