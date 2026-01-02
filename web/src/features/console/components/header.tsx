import { Play, ChevronDown, Check } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';
import { useProjects } from '../hooks/use-console-queries';
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
  // For pages that need single project selection (e.g., environments)
  selectedProjectId?: string;
  onProjectSelect?: (projectId: string) => void;
}

export function Header({
  title,
  activePage,
  isDetailView = false,
  detailViewType,
  primaryAction,
  selectedProjectId,
  onProjectSelect,
}: HeaderProps) {
  const [selectedProjects, setSelectedProjects] = useState<string[]>([]);
  const [selectedEnvironments, setSelectedEnvironments] = useState<string[]>([]);
  const [showProjectDropdown, setShowProjectDropdown] = useState(false);
  const [showEnvironmentDropdown, setShowEnvironmentDropdown] = useState(false);
  const [showSingleProjectDropdown, setShowSingleProjectDropdown] = useState(false);
  const singleProjectDropdownRef = useRef<HTMLDivElement>(null);

  // Fetch real projects data
  const { data: projects = [], isLoading: projectsLoading } = useProjects();

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (singleProjectDropdownRef.current && !singleProjectDropdownRef.current.contains(event.target as Node)) {
        setShowSingleProjectDropdown(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Pages that should show filters - but not if we're in a detail view
  // Environments page uses single-select project dropdown
  const showMultiProjectFilter = !isDetailView && ['overview', 'test-health', 'suite-activity'].includes(activePage);
  const showSingleProjectFilter = !isDetailView && activePage === 'environments';
  const showEnvironmentFilter = (!isDetailView && ['overview', 'test-health'].includes(activePage)) ||
                                 (isDetailView && (detailViewType === 'suite' || detailViewType === 'test'));

  // Get project names for multi-select dropdown
  const projectNames = projects.map(p => p.name);

  // Get selected project for single-select
  const selectedProject = projects.find(p => p.id === selectedProjectId);

  const environments = ['Production', 'Staging', 'Development'];

  return (
    <div className="bg-white border-b border-[#e5e5e5] px-8 py-4 flex items-center justify-between">
      <div className="flex items-center gap-6">
        <h1>{title}</h1>

        {/* Project and Environment Selectors */}
        {(showMultiProjectFilter || showSingleProjectFilter || showEnvironmentFilter) && (
          <div className="flex items-center gap-2">
            {/* Multi-select project dropdown for overview, test-health, suite-activity */}
            {showMultiProjectFilter && (
              <div className="min-w-[180px]">
                <MultiSelectDropdown
                  label="Projects"
                  items={projectNames}
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

            {/* Single-select project dropdown for environments page */}
            {showSingleProjectFilter && (
              <div className="relative min-w-[200px]" ref={singleProjectDropdownRef}>
                <button
                  onClick={() => setShowSingleProjectDropdown(!showSingleProjectDropdown)}
                  disabled={projectsLoading}
                  className="w-full flex items-center justify-between px-3 py-2 bg-white border border-[#e5e5e5] rounded-md text-sm hover:border-[#999999] transition-colors focus:outline-none focus:ring-2 focus:ring-black/5"
                >
                  <span className={selectedProject ? 'text-black' : 'text-[#999999]'}>
                    {projectsLoading ? 'Loading...' : selectedProject ? selectedProject.name : 'Select project...'}
                  </span>
                  <ChevronDown className={`w-4 h-4 text-[#666666] transition-transform ${showSingleProjectDropdown ? 'rotate-180' : ''}`} />
                </button>

                {showSingleProjectDropdown && !projectsLoading && (
                  <div className="absolute top-full left-0 right-0 mt-1 bg-white border border-[#e5e5e5] rounded-md shadow-lg z-50 max-h-[300px] overflow-y-auto">
                    {projects.length === 0 ? (
                      <div className="px-3 py-2 text-sm text-[#999999]">No projects available</div>
                    ) : (
                      projects.map((project) => (
                        <button
                          key={project.id}
                          onClick={() => {
                            onProjectSelect?.(project.id);
                            setShowSingleProjectDropdown(false);
                          }}
                          className="w-full flex items-center justify-between px-3 py-2 text-sm text-left hover:bg-[#fafafa] transition-colors"
                        >
                          <span>{project.name}</span>
                          {project.id === selectedProjectId && (
                            <Check className="w-4 h-4 text-[#4CBB17]" />
                          )}
                        </button>
                      ))
                    )}
                  </div>
                )}
              </div>
            )}

            {(showMultiProjectFilter || showSingleProjectFilter) && showEnvironmentFilter && (
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