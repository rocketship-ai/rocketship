import { Play } from 'lucide-react';
import { Sparkline } from '../components/sparkline';
import { MultiSelectDropdown } from '../components/multi-select-dropdown';
import { useState, useMemo } from 'react';
import { EmptyState, Card } from '../components/ui';
import { FilterBar, SearchInput } from '../components/filter-bar';
import { getPluginIcon } from '../plugins';
import { useTestHealth, useProjects } from '../hooks/use-console-queries';
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

  // Get environment filter
  const { selectedEnvironmentId } = useConsoleEnvironmentFilter(projectIdForEnvs || '');

  // Fetch test health data from API
  const { data, isLoading, error } = useTestHealth({
    projectIds: selectedProjectIds.length > 0 ? selectedProjectIds : undefined,
    environmentId: selectedEnvironmentId || undefined,
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
          />
        </FilterBar>

        {/* Table */}
        {filteredTests.length > 0 ? (
          <Card padding="sm" className="overflow-hidden">
            <table className="w-full">
              <thead className="border-b border-[#e5e5e5] bg-[#fafafa]">
                <tr>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider w-64">
                    Name
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider w-32">
                    Plugins
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider w-48">
                    Suite
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider w-64">
                    Latest Schedule Results
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider w-20">
                    Success
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider w-28">
                    Last Run
                  </th>
                  <th className="text-left px-6 py-3 text-xs text-[#666666] uppercase tracking-wider w-28">
                    Next Run
                  </th>
                  <th className="text-center px-6 py-3 text-xs text-[#666666] uppercase tracking-wider w-24">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#e5e5e5]">
                {filteredTests.map((test) => {
                  return (
                    <tr
                      key={test.id}
                      className="hover:bg-[#fafafa] transition-colors cursor-pointer"
                      onClick={() => onSelectTest(test.id)}
                    >
                      <td className="px-6 h-14 align-middle max-w-0">
                        <span className="text-sm truncate block">{test.name}</span>
                      </td>
                      <td className="px-6 h-14 align-middle">
                        <div className="flex items-center gap-2">
                          {test.plugins.map((plugin) => {
                            const Icon = getPluginIcon(plugin);
                            return (
                              <Icon
                                key={plugin}
                                className="w-4 h-4 text-[#666666] flex-shrink-0"
                              />
                            );
                          })}
                        </div>
                      </td>
                      <td className="px-6 h-14 align-middle max-w-0">
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            if (onSelectSuite) {
                              onSelectSuite(test.suite_id);
                            }
                          }}
                          className="text-sm text-black hover:underline truncate block text-left w-full"
                        >
                          {test.suite_name}
                        </button>
                      </td>
                      <td className="px-6 h-14 align-middle">
                        <Sparkline results={test.recent_results} size="lg" shape="pill" />
                      </td>
                      <td className="px-6 h-14 align-middle">
                        <span className="text-sm">{test.success_rate || '—'}</span>
                      </td>
                      <td className="px-6 h-14 align-middle">
                        <span className="text-sm text-[#666666]">
                          {formatRelativeTime(test.last_run_at || undefined)}
                        </span>
                      </td>
                      <td className="px-6 h-14 align-middle">
                        <span className="text-sm text-[#666666]">
                          {formatFutureRelativeTime(test.next_run_at)}
                        </span>
                      </td>
                      <td className="px-6 h-14 align-middle text-center">
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            console.log('Run test:', test.id);
                          }}
                          className="p-1 hover:bg-[#e5e5e5] rounded transition-colors"
                          title="Run test"
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
