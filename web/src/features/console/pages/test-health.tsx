import { Play } from 'lucide-react';
import { Sparkline } from '../components/sparkline';
import { MultiSelectDropdown } from '../components/multi-select-dropdown';
import { useState, useMemo, useEffect } from 'react';
import { EmptyState, Card } from '../components/ui';
import { FilterBar, SearchInput } from '../components/filter-bar';
import { getPluginIcon } from '../plugins';
import { useTestHealth, useProjects, useProjectEnvironments } from '../hooks/use-console-queries';
import { useConsoleProjectFilter, useConsoleEnvironmentFilter } from '../hooks/use-console-filters';
import { formatRelativeTime, formatFutureRelativeTime } from '../lib/format';

// Available plugins for filter dropdown
const availablePlugins = ['http', 'playwright', 'supabase', 'agent', 'sql', 'script', 'delay', 'log', 'browser_use'];

interface TestHealthProps {
  onSelectTest: (testId: string) => void;
  onSelectSuite?: (suiteId: string) => void;
}

export function TestHealth({ onSelectTest, onSelectSuite }: TestHealthProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedPlugins, setSelectedPlugins] = useState<string[]>([]);
  const [selectedSuites, setSelectedSuites] = useState<string[]>([]);
  const [showPluginDropdown, setShowPluginDropdown] = useState(false);
  const [showSuiteDropdown, setShowSuiteDropdown] = useState(false);

  // Get the global project filter from localStorage
  const { selectedProjectIds } = useConsoleProjectFilter();

  // Get projects for determining projectIdForEnvs
  const { data: projects = [] } = useProjects();

  // Determine the project ID for environment filtering
  // Use first selected project, or first accessible project
  const projectIdForEnvs = useMemo(() => {
    if (selectedProjectIds.length > 0) {
      return selectedProjectIds[0];
    }
    if (projects.length > 0) {
      return projects[0].id;
    }
    return undefined;
  }, [selectedProjectIds, projects]);

  // Get environment filter and fetch project environments for validation
  const { selectedEnvironmentId, clearSelectedEnvironmentId } = useConsoleEnvironmentFilter(projectIdForEnvs || '');
  const { data: projectEnvironments = [] } = useProjectEnvironments(projectIdForEnvs || '');

  // Validate selectedEnvironmentId against actual project environments
  // This prevents stale/invalid environment IDs from being sent to the API
  const effectiveEnvironmentId = useMemo(() => {
    if (!selectedEnvironmentId) return undefined;
    if (projectEnvironments.length === 0) return undefined; // Still loading
    const envExists = projectEnvironments.some(e => e.id === selectedEnvironmentId);
    return envExists ? selectedEnvironmentId : undefined;
  }, [selectedEnvironmentId, projectEnvironments]);

  // Clear stale environment selection if it doesn't exist in current project
  useEffect(() => {
    if (selectedEnvironmentId && projectEnvironments.length > 0) {
      const envExists = projectEnvironments.some(e => e.id === selectedEnvironmentId);
      if (!envExists) {
        clearSelectedEnvironmentId();
      }
    }
  }, [selectedEnvironmentId, projectEnvironments, clearSelectedEnvironmentId]);

  // Fetch test health data from API
  const { data, isLoading, error } = useTestHealth({
    projectIds: selectedProjectIds.length > 0 ? selectedProjectIds : undefined,
    environmentId: effectiveEnvironmentId,
    plugins: selectedPlugins.length > 0 ? selectedPlugins : undefined,
    search: searchQuery || undefined,
    suiteIds: undefined, // We filter by suite name in the UI, not IDs
  });

  // Get available suites from API response
  const allSuites = useMemo(() => {
    if (!data?.suites) return [];
    return data.suites.map(s => s.name);
  }, [data?.suites]);

  // Filter tests by selected suites (client-side since we have the data)
  const filteredTests = useMemo(() => {
    if (!data?.tests) return [];
    if (selectedSuites.length === 0) return data.tests;
    return data.tests.filter(test => selectedSuites.includes(test.suite_name));
  }, [data?.tests, selectedSuites]);

  const handleClearFilters = () => {
    setSelectedPlugins([]);
    setSelectedSuites([]);
    setSearchQuery('');
  };

  // Loading state
  if (isLoading) {
    return (
      <div className="p-8">
        <div className="max-w-[1600px] mx-auto">
          <p className="text-sm text-[#666666] mb-6">Loading test health data...</p>
        </div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="p-8">
        <div className="max-w-[1600px] mx-auto">
          <EmptyState
            title="Failed to load test health data"
            action={
              <button
                onClick={() => window.location.reload()}
                className="text-sm text-black hover:underline"
              >
                Retry
              </button>
            }
          />
        </div>
      </div>
    );
  }

  return (
    <div className="p-8">
      <div className="max-w-[1600px] mx-auto">
        {/* Subtitle */}
        <p className="text-sm text-[#666666] mb-6">Scheduled test results over time</p>

        {/* Filters */}
        <FilterBar>
          <SearchInput
            value={searchQuery}
            onChange={setSearchQuery}
            placeholder="Search tests..."
            className="flex-1"
          />

          <MultiSelectDropdown
            label="Plugins"
            items={availablePlugins}
            selectedItems={selectedPlugins}
            onSelectionChange={setSelectedPlugins}
            isOpen={showPluginDropdown}
            onToggle={() => {
              setShowPluginDropdown(!showPluginDropdown);
              setShowSuiteDropdown(false);
            }}
            renderIcon={(item) => {
              const Icon = getPluginIcon(item);
              return <Icon className="w-4 h-4 text-[#666666]" />;
            }}
          />

          <MultiSelectDropdown
            label="Suites"
            items={allSuites}
            selectedItems={selectedSuites}
            onSelectionChange={setSelectedSuites}
            isOpen={showSuiteDropdown}
            onToggle={() => {
              setShowSuiteDropdown(!showSuiteDropdown);
              setShowPluginDropdown(false);
            }}
            align="right"
          />
        </FilterBar>

        {/* Table */}
        {filteredTests.length > 0 ? (
          <Card padding="none" className="overflow-hidden">
            <table className="w-full table-fixed">
              <thead className="border-b border-[#e5e5e5] bg-[#fafafa]">
                <tr>
                  <th className="text-left px-6 py-3 text-[11px] font-medium text-[#666666] uppercase tracking-wider" style={{ width: '280px' }}>
                    Name
                  </th>
                  <th className="text-left px-4 py-3 text-[11px] font-medium text-[#666666] uppercase tracking-wider" style={{ width: '100px' }}>
                    Plugins
                  </th>
                  <th className="text-left px-4 py-3 text-[11px] font-medium text-[#666666] uppercase tracking-wider" style={{ width: '160px' }}>
                    Suite
                  </th>
                  <th className="text-left px-4 py-3 text-[11px] font-medium text-[#666666] uppercase tracking-wider" style={{ width: '240px' }}>
                    Latest Schedule Results
                  </th>
                  <th className="text-left px-4 py-3 text-[11px] font-medium text-[#666666] uppercase tracking-wider" style={{ width: '80px' }}>
                    Success
                  </th>
                  <th className="text-left px-4 py-3 text-[11px] font-medium text-[#666666] uppercase tracking-wider" style={{ width: '100px' }}>
                    Last Run
                  </th>
                  <th className="text-left px-4 py-3 text-[11px] font-medium text-[#666666] uppercase tracking-wider" style={{ width: '100px' }}>
                    Next Run
                  </th>
                  <th className="text-center px-4 py-3 text-[11px] font-medium text-[#666666] uppercase tracking-wider" style={{ width: '70px' }}>
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#f0f0f0]">
                {filteredTests.map((test) => {
                  return (
                    <tr
                      key={test.id}
                      className="h-[60px] hover:bg-[#fafafa] transition-colors cursor-pointer"
                      onClick={() => onSelectTest(test.id)}
                    >
                      <td className="px-6 align-middle">
                        <span className="text-sm text-[#111111] truncate block" title={test.name}>
                          {test.name}
                        </span>
                      </td>
                      <td className="px-4 align-middle">
                        {/* Plugin icons */}
                        {test.plugins && test.plugins.length > 0 ? (
                          <div className="flex items-center gap-1.5">
                            {test.plugins.slice(0, 3).map((plugin, idx) => {
                              const Icon = getPluginIcon(plugin);
                              return (
                                <span key={idx} title={plugin}>
                                  <Icon className="w-4 h-4 text-[#666666]" />
                                </span>
                              );
                            })}
                            {test.plugins.length > 3 && (
                              <span className="text-xs text-[#999999] ml-0.5">
                                +{test.plugins.length - 3}
                              </span>
                            )}
                          </div>
                        ) : (
                          <span className="text-sm text-[#999999]">—</span>
                        )}
                      </td>
                      <td className="px-4 align-middle">
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            if (onSelectSuite) {
                              onSelectSuite(test.suite_id);
                            }
                          }}
                          className="text-sm text-[#111111] hover:underline truncate block text-left max-w-full"
                          title={test.suite_name}
                        >
                          {test.suite_name}
                        </button>
                      </td>
                      <td className="px-4 align-middle">
                        <Sparkline results={test.recent_results} size="lg" maxItems={20} isLive={test.is_live} />
                      </td>
                      <td className="px-4 align-middle">
                        <span className="text-sm text-[#111111] whitespace-nowrap">{test.success_rate || '—'}</span>
                      </td>
                      <td className="px-4 align-middle">
                        <span className="text-sm text-[#666666] whitespace-nowrap">
                          {formatRelativeTime(test.last_run_at || undefined)}
                        </span>
                      </td>
                      <td className="px-4 align-middle">
                        <span className="text-sm text-[#666666] whitespace-nowrap">
                          {formatFutureRelativeTime(test.next_run_at)}
                        </span>
                      </td>
                      <td className="px-4 align-middle text-center">
                        <button
                          disabled
                          className="p-1.5 rounded inline-flex items-center justify-center opacity-30 cursor-not-allowed"
                          title="Run test (coming soon)"
                        >
                          <Play className="w-4 h-4 text-[#666666]" />
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </Card>
        ) : (
          <EmptyState
            title={data?.tests?.length === 0 ? "No scheduled tests found" : "No tests found matching your filters"}
            action={
              selectedPlugins.length > 0 || selectedSuites.length > 0 || searchQuery ? (
                <button
                  onClick={handleClearFilters}
                  className="text-sm text-black hover:underline"
                >
                  Clear filters
                </button>
              ) : undefined
            }
          />
        )}
      </div>
    </div>
  );
}

// Export a helper to get projectIdForEnvs for the Header component
export function useTestHealthProjectIdForEnvs(): string | undefined {
  const { selectedProjectIds } = useConsoleProjectFilter();
  const { data: projects = [] } = useProjects();

  return useMemo(() => {
    if (selectedProjectIds.length > 0) {
      return selectedProjectIds[0];
    }
    if (projects.length > 0) {
      return projects[0].id;
    }
    return undefined;
  }, [selectedProjectIds, projects]);
}
