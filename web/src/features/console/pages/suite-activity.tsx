import { Search, Loader2, AlertCircle, RefreshCw, FileCode } from 'lucide-react';
import { useState } from 'react';
import { useSuiteActivity } from '../hooks/use-console-queries';

interface SuiteActivityProps {
  onSelectSuite: (suiteId: string) => void;
}

function SourceRefBadge({ sourceRef }: { sourceRef: string }) {
  const isPR = sourceRef.startsWith('pr/');
  const displayText = isPR ? `#${sourceRef.slice(3)}` : sourceRef;
  return (
    <span className={`text-xs px-2 py-0.5 rounded border ${
      isPR
        ? 'bg-amber-50 text-amber-700 border-amber-200'
        : 'bg-gray-50 text-gray-700 border-gray-200'
    }`}>
      {displayText}
    </span>
  );
}

export function SuiteActivity({ onSelectSuite }: SuiteActivityProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const { data: suites, isLoading, error, refetch } = useSuiteActivity();

  if (isLoading) {
    return (
      <div className="p-8">
        <div className="max-w-5xl mx-auto flex items-center justify-center py-12">
          <Loader2 className="w-6 h-6 animate-spin text-[#666666]" />
          <span className="ml-3 text-[#666666]">Loading suite activity...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="max-w-5xl mx-auto">
          <div className="bg-red-50 border border-red-200 rounded-lg p-6 flex items-start gap-3">
            <AlertCircle className="w-5 h-5 text-red-500 flex-shrink-0 mt-0.5" />
            <div className="flex-1">
              <p className="text-red-700 font-medium">Failed to load suite activity</p>
              <p className="text-red-600 text-sm mt-1">
                {error instanceof Error ? error.message : 'An unexpected error occurred'}
              </p>
            </div>
            <button
              onClick={() => refetch()}
              className="flex items-center gap-2 px-3 py-1.5 text-sm text-red-700 hover:bg-red-100 rounded transition-colors"
            >
              <RefreshCw className="w-4 h-4" />
              Retry
            </button>
          </div>
        </div>
      </div>
    );
  }

  const allSuites = suites || [];

  const filteredSuites = allSuites.filter((suite) => {
    if (searchQuery && !(
      suite.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      (suite.description?.toLowerCase().includes(searchQuery.toLowerCase()))
    )) {
      return false;
    }
    return true;
  });

  if (allSuites.length === 0) {
    return (
      <div className="p-8">
        <div className="max-w-5xl mx-auto">
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
            <FileCode className="w-12 h-12 text-[#999999] mx-auto mb-4" />
            <h3 className="text-lg font-medium mb-2">No suites yet</h3>
            <p className="text-[#666666] text-sm">
              Connect a repository with test suites to see activity here.
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8">
      <div className="max-w-5xl mx-auto">
        {/* Search and Filters */}
        <div className="flex items-center gap-3 mb-6">
          <div className="flex-1 relative">
            <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-[#999999]" />
            <input
              type="text"
              placeholder="Search suites..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-10 pr-4 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
            />
          </div>
        </div>

        {/* Suites List */}
        <div className="grid grid-cols-1 gap-4">
          {filteredSuites.map((suite) => (
            <div
              key={suite.suite_id}
              onClick={() => onSelectSuite(suite.suite_id)}
              className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 hover:shadow-md transition-shadow cursor-pointer"
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

                  {/* Metrics - placeholders until run aggregation exists */}
                  <div className="flex items-center gap-8 mt-6">
                    <div className="flex items-center gap-2">
                      <div>
                        <p className="text-xs text-[#999999]">Speed</p>
                        <p className="text-sm text-[#999999]">—</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <div>
                        <p className="text-xs text-[#999999]">Reliability</p>
                        <p className="text-sm text-[#999999]">—</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <div>
                        <p className="text-xs text-[#999999]">Runs</p>
                        <p className="text-sm text-[#999999]">—</p>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Activity placeholder - no run history yet */}
                <div className="flex flex-col items-end gap-2 flex-shrink-0 ml-auto">
                  <p className="text-xs text-[#999999]">{suite.test_count} tests</p>
                  <div className="flex items-center text-xs text-[#999999]">
                    {suite.last_run.status ? (
                      <span>Last run: {suite.last_run.status}</span>
                    ) : (
                      <span>No runs yet</span>
                    )}
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}