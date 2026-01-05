import { FileCode } from 'lucide-react';
import { useState } from 'react';
import { useSuiteActivity } from '../hooks/use-console-queries';
import { useConsoleProjectFilter } from '../hooks/use-console-filters';
import { SourceRefBadge } from '../components/SourceRefBadge';
import { QueryBoundary } from '../components/query-boundary';
import { Card, EmptyState } from '../components/ui';
import { FilterBar, SearchInput } from '../components/filter-bar';
import { formatDuration } from '../lib/format';

interface SuiteActivityProps {
  onSelectSuite: (suiteId: string) => void;
}

export function SuiteActivity({ onSelectSuite }: SuiteActivityProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const query = useSuiteActivity();
  const { selectedProjectIds } = useConsoleProjectFilter();

  return (
    <div className="p-8">
      <div className="max-w-5xl mx-auto">
        <QueryBoundary
          query={query}
          loadingMessage="Loading suite activity..."
          errorTitle="Failed to load suite activity"
        >
          {(suites) => {
            if (suites.length === 0) {
              return (
                <EmptyState
                  icon={<FileCode className="w-12 h-12" />}
                  title="No suites yet"
                  description="Connect a repository with test suites to see activity here."
                />
              );
            }

            const filteredSuites = suites.filter((suite) => {
              // Project filter (global sticky filter)
              if (selectedProjectIds.length > 0 && !selectedProjectIds.includes(suite.project.id)) {
                return false;
              }
              // Search filter
              if (searchQuery && !(
                suite.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                (suite.description?.toLowerCase().includes(searchQuery.toLowerCase()))
              )) {
                return false;
              }
              return true;
            });

            return (
              <>
                {/* Search and Filters */}
                <FilterBar>
                  <SearchInput
                    value={searchQuery}
                    onChange={setSearchQuery}
                    placeholder="Search suites..."
                    className="flex-1"
                  />
                </FilterBar>

                {/* Suites List */}
                <div className="grid grid-cols-1 gap-4">
                  {filteredSuites.map((suite) => (
                    <Card
                      key={suite.suite_id}
                      onClick={() => onSelectSuite(suite.suite_id)}
                      className="hover:shadow-md transition-shadow cursor-pointer"
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-3 mb-2">
                            <h3>{suite.name}</h3>
                            <SourceRefBadge sourceRef={suite.source_ref} />
                          </div>
                          {suite.description && (
                            <p className="text-sm text-[#666666] mb-1">{suite.description}</p>
                          )}
                          {suite.file_path && (
                            <p className="text-xs text-[#999999] font-mono">{suite.file_path}</p>
                          )}
                          <p className="text-xs text-[#666666] mt-1">
                            {suite.project.name}
                          </p>

                          {/* Metrics from run aggregation */}
                          <div className="flex items-center gap-8 mt-6">
                            <div className="flex items-center gap-2">
                              <div>
                                <p className="text-xs text-[#999999]">Speed</p>
                                <p className={`text-sm ${suite.median_duration_ms != null ? 'text-black' : 'text-[#999999]'}`}>
                                  {suite.median_duration_ms != null
                                    ? formatDuration(suite.median_duration_ms)
                                    : '—'}
                                </p>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <div>
                                <p className="text-xs text-[#999999]">Reliability</p>
                                <p className={`text-sm ${suite.reliability_pct != null ? 'text-black' : 'text-[#999999]'}`}>
                                  {suite.reliability_pct != null
                                    ? `${Math.round(suite.reliability_pct)}%`
                                    : '—'}
                                </p>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <div>
                                <p className="text-xs text-[#999999]">Runs</p>
                                <p className={`text-sm ${suite.runs_per_week != null ? 'text-black' : 'text-[#999999]'}`}>
                                  {suite.runs_per_week != null
                                    ? `${suite.runs_per_week}/week`
                                    : '—'}
                                </p>
                              </div>
                            </div>
                          </div>
                        </div>

                        {/* Right side: test count */}
                        <div className="flex flex-col items-end gap-2 flex-shrink-0 ml-auto">
                          <p className="text-xs text-[#999999]">{suite.test_count} tests</p>
                        </div>
                      </div>
                    </Card>
                  ))}
                </div>
              </>
            );
          }}
        </QueryBoundary>
      </div>
    </div>
  );
}
