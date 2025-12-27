import { Search } from 'lucide-react';
import { useState } from 'react';
import { suites } from '../data/mock-data';

interface SuiteActivityProps {
  onSelectSuite: (suiteId: string) => void;
}

export function SuiteActivity({ onSelectSuite }: SuiteActivityProps) {
  const [searchQuery, setSearchQuery] = useState('');

  const filteredSuites = suites.filter((suite) => {
    if (searchQuery && !(
      suite.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      suite.description.toLowerCase().includes(searchQuery.toLowerCase())
    )) {
      return false;
    }
    return true;
  });

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
              key={suite.id}
              onClick={() => onSelectSuite(suite.id)}
              className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 hover:shadow-md transition-shadow cursor-pointer"
            >
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <h3 className="mb-2">{suite.name}</h3>
                  <p className="text-sm text-[#666666] mb-1">{suite.description}</p>
                  <p className="text-xs text-[#999999] font-mono">{suite.path}</p>

                  {/* Metrics */}
                  <div className="flex items-center gap-8 mt-6">
                    <div className="flex items-center gap-2">
                      <div>
                        <p className="text-xs text-[#999999]">Speed</p>
                        <p className="text-sm">{suite.metrics.speed}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <div>
                        <p className="text-xs text-[#999999]">Reliability</p>
                        <p className="text-sm">{suite.metrics.reliability}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <div>
                        <p className="text-xs text-[#999999]">Runs</p>
                        <p className="text-sm">{suite.metrics.runsPerDay}/week</p>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Activity Bar Chart */}
                <div className="flex flex-col items-end gap-2 flex-shrink-0 ml-auto">
                  <p className="text-xs text-[#666666]">main</p>
                  <div className="flex items-end gap-[3px] h-[60px]">
                    {suite.recentActivity.map((status, index) => {
                      const heightPercentage = 40 + Math.random() * 60;
                      
                      let barColor;
                      if (status === 'success') {
                        barColor = 'bg-[#4CBB17]';
                      } else if (status === 'failed') {
                        barColor = 'bg-[#ef0000]';
                      } else {
                        barColor = 'bg-[#f6a724]';
                      }

                      return (
                        <div
                          key={index}
                          className={`w-[6px] rounded-sm ${barColor}`}
                          style={{ height: `${heightPercentage}%` }}
                        />
                      );
                    })}
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