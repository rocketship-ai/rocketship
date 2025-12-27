import { Play } from 'lucide-react';
import { useState } from 'react';
import { MultiSelectDropdown } from './multi-select-dropdown';

interface HeaderProps {
  title: string;
  activePage: string;
  isDetailView?: boolean;
  detailViewType?: 'suite' | 'test' | 'suite-run' | 'test-run' | 'project' | null;
  primaryAction?: {
    label: string;
    onClick: () => void;
  };
}

export function Header({ title, activePage, isDetailView = false, detailViewType, primaryAction }: HeaderProps) {
  const [selectedProjects, setSelectedProjects] = useState<string[]>([]);
  const [selectedEnvironments, setSelectedEnvironments] = useState<string[]>([]);
  const [showProjectDropdown, setShowProjectDropdown] = useState(false);
  const [showEnvironmentDropdown, setShowEnvironmentDropdown] = useState(false);

  // Pages that should show filters - but not if we're in a detail view
  const showProjectFilter = !isDetailView && ['overview', 'test-health', 'suite-activity'].includes(activePage);
  const showEnvironmentFilter = (!isDetailView && ['overview', 'test-health'].includes(activePage)) || 
                                 (isDetailView && (detailViewType === 'suite' || detailViewType === 'test'));
  
  const projects = ['Frontend', 'Backend'];
  
  const environments = ['Production', 'Staging', 'Development'];

  return (
    <div className="bg-white border-b border-[#e5e5e5] px-8 py-4 flex items-center justify-between">
      <div className="flex items-center gap-6">
        <h1>{title}</h1>
        
        {/* Project and Environment Selectors */}
        {(showProjectFilter || showEnvironmentFilter) && (
          <div className="flex items-center gap-2">
            {showProjectFilter && (
              <div className="min-w-[180px]">
                <MultiSelectDropdown
                  label="Projects"
                  items={projects}
                  selectedItems={selectedProjects}
                  onSelectionChange={setSelectedProjects}
                  isOpen={showProjectDropdown}
                  onToggle={() => {
                    setShowProjectDropdown(!showProjectDropdown);
                    setShowEnvironmentDropdown(false);
                  }}
                />
              </div>
            )}
            {showProjectFilter && showEnvironmentFilter && (
              <span className="text-[#999999]">/</span>
            )}
            {showEnvironmentFilter && (
              <div className="min-w-[180px]">
                <MultiSelectDropdown
                  label="Environments"
                  items={environments}
                  selectedItems={selectedEnvironments}
                  onSelectionChange={setSelectedEnvironments}
                  isOpen={showEnvironmentDropdown}
                  onToggle={() => {
                    setShowEnvironmentDropdown(!showEnvironmentDropdown);
                    setShowProjectDropdown(false);
                  }}
                />
              </div>
            )}
          </div>
        )}
      </div>

      <div className="flex items-center gap-3">
        {primaryAction && (
          <button
            onClick={primaryAction.onClick}
            className="flex items-center gap-2 px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors"
          >
            <Play className="w-4 h-4" />
            <span>{primaryAction.label}</span>
          </button>
        )}
      </div>
    </div>
  );
}