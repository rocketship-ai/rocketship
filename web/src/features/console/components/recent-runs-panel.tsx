import { Loader2 } from 'lucide-react';
import { useState } from 'react';
import { MultiSelectDropdown } from './multi-select-dropdown';
import { TestRunRow, type TestRunRowData } from './test-run-row';

interface RecentRunsPanelProps {
  runs: TestRunRowData[];
  totalRuns: number;
  currentPage: number;
  runsPerPage: number;
  onPageChange: (page: number) => void;
  isLoading: boolean;
  error: Error | null;
  selectedTriggers: string[];
  onTriggerChange: (triggers: string[]) => void;
  hasFiltersApplied: boolean;
  onViewRun: (runId: string) => void;
  onRetry: () => void;
}

/**
 * Recent Runs sidebar panel for the Test Detail page.
 * Shows a filterable, server-paginated list of test runs.
 */
export function RecentRunsPanel({
  runs,
  totalRuns,
  currentPage,
  runsPerPage,
  onPageChange,
  isLoading,
  error,
  selectedTriggers,
  onTriggerChange,
  hasFiltersApplied,
  onViewRun,
  onRetry,
}: RecentRunsPanelProps) {
  const [showTriggerDropdown, setShowTriggerDropdown] = useState(false);

  // Calculate total pages from server total
  const totalPages = Math.max(1, Math.ceil(totalRuns / runsPerPage));

  return (
    <div className="w-80 flex-shrink-0">
      <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm sticky top-8">
        <div className="p-4 border-b border-[#e5e5e5]">
          <h3 className="mb-3">Recent Runs</h3>

          {/* Trigger Filter */}
          <div>
            <label className="text-xs text-[#999999] mb-1 block">Trigger</label>
            <MultiSelectDropdown
              label="Triggers"
              items={['ci', 'manual', 'schedule']}
              selectedItems={selectedTriggers}
              onSelectionChange={onTriggerChange}
              isOpen={showTriggerDropdown}
              onToggle={() => setShowTriggerDropdown(!showTriggerDropdown)}
              showAllOption={true}
              placeholder="All triggers"
            />
          </div>
        </div>

        {/* Recent runs list - data is already server-paginated */}
        <div>
          {error ? (
            <div className="p-8 text-center">
              <p className="text-sm text-[#ef4444]">Failed to load runs</p>
              <p className="text-xs text-[#999999] mt-1">
                {error.message || 'An error occurred'}
              </p>
              <button
                onClick={onRetry}
                className="mt-3 text-xs px-3 py-1.5 bg-white border border-[#e5e5e5] rounded hover:bg-[#fafafa] transition-colors"
              >
                Retry
              </button>
            </div>
          ) : isLoading && runs.length === 0 ? (
            <div className="p-8 text-center">
              <Loader2 className="w-5 h-5 animate-spin mx-auto text-[#666666]" />
              <p className="text-sm text-[#666666] mt-2">Loading runs...</p>
            </div>
          ) : runs.length === 0 ? (
            <div className="p-8 text-center">
              <p className="text-sm text-[#666666]">No runs found</p>
              <p className="text-xs text-[#999999] mt-1">
                {hasFiltersApplied
                  ? 'Try adjusting your filters'
                  : 'This test has not been run yet'}
              </p>
            </div>
          ) : (
            runs.map((run) => (
              <TestRunRow
                key={run.id}
                run={run}
                onClick={onViewRun}
              />
            ))
          )}
        </div>

        {/* Pagination - based on server total */}
        {totalRuns > runsPerPage && (
          <div className="p-4 border-t border-[#e5e5e5]">
            <div className="flex items-center justify-between">
              <button
                onClick={() => onPageChange(Math.max(1, currentPage - 1))}
                disabled={currentPage === 1}
                className="text-xs px-3 py-1.5 bg-white border border-[#e5e5e5] rounded hover:bg-[#fafafa] disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Prev
              </button>
              <span className="text-xs text-[#666666]">
                Page {currentPage} of {totalPages}
              </span>
              <button
                onClick={() => onPageChange(Math.min(totalPages, currentPage + 1))}
                disabled={currentPage >= totalPages}
                className="text-xs px-3 py-1.5 bg-white border border-[#e5e5e5] rounded hover:bg-[#fafafa] disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
