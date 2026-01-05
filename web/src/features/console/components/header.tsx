import { Play } from 'lucide-react';
import { useState, useEffect, useMemo } from 'react';
import {
  useProjects,
  useSuite,
  useProjectEnvironments,
  useTestDetail,
} from '../hooks/use-console-queries';
import { useConsoleProjectFilter, useConsoleEnvironmentFilter } from '../hooks/use-console-filters';
import { MultiSelectDropdown } from './multi-select-dropdown';

interface HeaderProps {
  title: string;
  activePage: string;
  isDetailView?: boolean;
  detailViewType?: 'suite' | 'test' | 'suite-run' | 'test-run' | 'project' | null;
  suiteId?: string; // For suite detail view to fetch project environments
  testId?: string; // For test detail view to fetch project environments
  projectIdForEnvs?: string; // Explicit project ID for env filter (used by test-health)
  primaryAction?: {
    label: string;
    onClick: () => void;
  };
  // For environment filtering on suite detail
  onEnvironmentChange?: (envId: string | undefined) => void;
}

export function Header({
  title,
  activePage,
  isDetailView = false,
  detailViewType,
  suiteId,
  testId,
  projectIdForEnvs: explicitProjectIdForEnvs,
  primaryAction,
  onEnvironmentChange,
}: HeaderProps) {
  const [showProjectDropdown, setShowProjectDropdown] = useState(false);
  const [showEnvironmentDropdown, setShowEnvironmentDropdown] = useState(false);

  // Global project filter (sticky across pages via localStorage)
  const { selectedProjectIds, setSelectedProjectIds } = useConsoleProjectFilter();

  // Fetch real projects data
  const { data: projects = [] } = useProjects();

  // For suite detail view: fetch suite to get project ID, then fetch environments
  // If explicitProjectIdForEnvs is provided (from test-health), use that instead
  const { data: suite } = useSuite(suiteId || '');

  // For test detail view: fetch test to get project ID
  const { data: testDetail } = useTestDetail(testId || '');

  // Compute projectIdForEnvs with fallback for pages without explicit project context
  // Priority: explicit prop > suite's project > test's project > first selected project > first accessible project
  const projectIdForEnvs = useMemo(() => {
    if (explicitProjectIdForEnvs) return explicitProjectIdForEnvs;
    if (suite?.project?.id) return suite.project.id;
    if (testDetail?.project_id) return testDetail.project_id;
    // For test-health and overview pages, fall back to first selected or first project
    if (selectedProjectIds.length > 0) return selectedProjectIds[0];
    if (projects.length > 0) return projects[0].id;
    return '';
  }, [explicitProjectIdForEnvs, suite?.project?.id, testDetail?.project_id, selectedProjectIds, projects]);

  const { data: projectEnvironments = [] } = useProjectEnvironments(projectIdForEnvs);

  // Local environment selection for suite's project (sticky per project via localStorage)
  const {
    selectedEnvironmentId,
    setSelectedEnvironmentId,
    clearSelectedEnvironmentId,
  } = useConsoleEnvironmentFilter(projectIdForEnvs);

  // Convert env ID to selected environment name for dropdown
  // If the environment ID doesn't exist in current project, treat as "All Environments"
  const selectedEnvName = useMemo(() => {
    if (!selectedEnvironmentId || projectEnvironments.length === 0) return [];
    const env = projectEnvironments.find(e => e.id === selectedEnvironmentId);
    return env ? [env.name] : [];
  }, [selectedEnvironmentId, projectEnvironments]);

  // Clear stale environment selection if it doesn't exist in current project
  // This ensures UI and API requests stay consistent
  useEffect(() => {
    if (selectedEnvironmentId && projectEnvironments.length > 0) {
      const envExists = projectEnvironments.some(e => e.id === selectedEnvironmentId);
      if (!envExists) {
        clearSelectedEnvironmentId();
      }
    }
  }, [selectedEnvironmentId, projectEnvironments, clearSelectedEnvironmentId]);

  // Notify parent when environment changes (for backward compatibility)
  // Use effectiveEnvironmentId (validated) rather than raw selectedEnvironmentId
  const effectiveEnvironmentId = useMemo(() => {
    if (!selectedEnvironmentId || projectEnvironments.length === 0) return undefined;
    const envExists = projectEnvironments.some(e => e.id === selectedEnvironmentId);
    return envExists ? selectedEnvironmentId : undefined;
  }, [selectedEnvironmentId, projectEnvironments]);

  useEffect(() => {
    if (onEnvironmentChange) {
      onEnvironmentChange(effectiveEnvironmentId);
    }
  }, [effectiveEnvironmentId, onEnvironmentChange]);

  // Pages that should show filters - but not if we're in a detail view
  // All list pages including environments use the global multi-select project filter
  const showProjectFilter = !isDetailView && ['overview', 'test-health', 'suite-activity', 'environments'].includes(activePage);
  const showEnvironmentFilter = (!isDetailView && ['overview', 'test-health'].includes(activePage)) ||
                                 (isDetailView && (detailViewType === 'suite' || detailViewType === 'test'));

  // Disable environment filter on list pages when not exactly 1 project is selected
  // (environment filter only makes sense with a single project context)
  const shouldDisableEnvFilter = !isDetailView &&
    ['overview', 'test-health'].includes(activePage) &&
    selectedProjectIds.length !== 1;

  // Get project names for dropdowns
  const projectNames = projects.map(p => p.name);

  // For global multi-select filter: convert IDs to names for display
  const selectedProjectNamesForFilter = useMemo(() => {
    return selectedProjectIds
      .map(id => projects.find(p => p.id === id)?.name)
      .filter((name): name is string => !!name);
  }, [selectedProjectIds, projects]);

  // Get environment names for suite detail dropdown
  const environmentNames = projectEnvironments.map(e => e.name);

  // Handle global project filter selection (multi-select, converts names to IDs)
  const handleGlobalProjectSelect = (names: string[]) => {
    const ids = names
      .map(name => projects.find(p => p.name === name)?.id)
      .filter((id): id is string => !!id);
    setSelectedProjectIds(ids);
  };

  // Handle environment selection (updates local storage filter)
  const handleEnvironmentSelect = (names: string[]) => {
    if (names.length > 0) {
      // Find the env ID from name
      const envName = names[names.length - 1];
      const env = projectEnvironments.find(e => e.name === envName);
      if (env) {
        // Update local selection
        setSelectedEnvironmentId(env.id);
      }
    } else {
      // Clear local selection
      clearSelectedEnvironmentId();
    }
  };

  return (
    <div className="bg-white border-b border-[#e5e5e5] px-8 py-4 flex items-center justify-between">
      <div className="flex items-center gap-6">
        <h1>{title}</h1>

        {/* Project and Environment Selectors */}
        {(showProjectFilter || showEnvironmentFilter) && (
          <div className="flex items-center gap-2">
            {/* Global multi-select project dropdown for all list pages */}
            {showProjectFilter && (
              <div className="min-w-[180px]">
                <MultiSelectDropdown
                  label="Projects"
                  items={projectNames}
                  selectedItems={selectedProjectNamesForFilter}
                  onSelectionChange={handleGlobalProjectSelect}
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
                  items={environmentNames}
                  selectedItems={selectedEnvName}
                  onSelectionChange={handleEnvironmentSelect}
                  isOpen={showEnvironmentDropdown}
                  onToggle={() => {
                    setShowEnvironmentDropdown(!showEnvironmentDropdown);
                    setShowProjectDropdown(false);
                  }}
                  singleSelect={true}
                  showAllOption={true}
                  placeholder="All Environments"
                  disabled={shouldDisableEnvFilter}
                  disabledTooltip="Select a single project to filter by environment"
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
