import { Loader2 } from 'lucide-react';
import { useState, useMemo } from 'react';
import { MultiSelectDropdown } from './multi-select-dropdown';
import { TestRunRow, type TestRunRowData } from './test-run-row';

interface RecentRunsPanelProps {
  runs: TestRunRowData[];
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
 * Shows a filterable, paginated list of test runs.
 */
export function RecentRunsPanel({
  runs,
  isLoading,
  error,
  selectedTriggers,
  onTriggerChange,
  hasFiltersApplied,
  onViewRun,
  onRetry,
}: RecentRunsPanelProps) {
  const [showTriggerDropdown, setShowTriggerDropdown] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const runsPerPage = 10;

  // Reset to first page when filters change
  const handleTriggerChange = (triggers: string[]) => {
    onTriggerChange(triggers);
    setCurrentPage(1);
  };

  // Paginate runs
  const paginatedRuns = useMemo(() => {
    const start = (currentPage - 1) * runsPerPage;
    return runs.slice(start, start + runsPerPage);
  }, [runs, currentPage, runsPerPage]);

  const totalPages = Math.max(1, Math.ceil(runs.length / runsPerPage));

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
              onSelectionChange={handleTriggerChange}
              isOpen={showTriggerDropdown}
              onToggle={() => setShowTriggerDropdown(!showTriggerDropdown)}
              showAllOption={true}
              placeholder="All triggers"
            />
          </div>
        </div>

        {/* Recent runs list */}
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
            paginatedRuns.map((run) => (
              <TestRunRow
                key={run.id}
                run={run}
                onClick={onViewRun}
              />
            ))
          )}
        </div>

        {/* Pagination */}
        {runs.length > runsPerPage && (
          <div className="p-4 border-t border-[#e5e5e5]">
            <div className="flex items-center justify-between">
              <button
                onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
                disabled={currentPage === 1}
                className="text-xs px-3 py-1.5 bg-white border border-[#e5e5e5] rounded hover:bg-[#fafafa] disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Previous
              </button>
              <span className="text-xs text-[#666666]">
                Page {currentPage} of {totalPages}
              </span>
              <button
                onClick={() => setCurrentPage(Math.min(totalPages, currentPage + 1))}
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
