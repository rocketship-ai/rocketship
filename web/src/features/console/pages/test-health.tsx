import { Search, Code, Globe, Database, Bot, Play, FileText, Clock, Terminal } from 'lucide-react';
import { Sparkline } from '../components/sparkline';
import { MultiSelectDropdown } from '../components/multi-select-dropdown';
import { useState } from 'react';
import { tests, suites, availablePlugins, getSuiteById } from '../data/mock-data';

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

  const allPlugins = availablePlugins;
  const allSuites = suites.map(s => s.name);

  const filteredTests = tests.filter((test) => {
    if (searchQuery && !test.name.toLowerCase().includes(searchQuery.toLowerCase())) {
      return false;
    }
    if (selectedPlugins.length > 0 && !selectedPlugins.some(plugin => test.plugins.includes(plugin))) {
      return false;
    }
    if (selectedSuites.length > 0) {
      const suite = getSuiteById(test.suiteId);
      if (!suite || !selectedSuites.includes(suite.name)) {
        return false;
      }
    }
    return true;
  });

  const pluginIcons: Record<string, any> = {
    HTTP: Globe,
    Playwright: Code,
    Supabase: Database,
    Agent: Bot,
    SQL: Database,
    Script: Terminal,
    Delay: Clock,
    Log: FileText,
  };

  return (
    <div className="p-8">
      <div className="max-w-[1600px] mx-auto">
        {/* Subtitle */}
        <p className="text-sm text-[#666666] mb-6">Scheduled test results over time</p>

        {/* Filters */}
        <div className="flex items-center gap-3 mb-6">
          <div className="flex-1 relative">
            <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-[#999999]" />
            <input
              type="text"
              placeholder="Search tests..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-10 pr-4 py-2 bg-white border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/5"
            />
          </div>

          <MultiSelectDropdown
            label="Plugins"
            items={allPlugins}
            selectedItems={selectedPlugins}
            onSelectionChange={setSelectedPlugins}
            isOpen={showPluginDropdown}
            onToggle={() => {
              setShowPluginDropdown(!showPluginDropdown);
              setShowSuiteDropdown(false);
            }}
            renderIcon={(item) => {
              const Icon = pluginIcons[item];
              return Icon ? <Icon className="w-4 h-4 text-[#666666]" /> : null;
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
        </div>

        {/* Table */}
        {filteredTests.length > 0 ? (
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm overflow-hidden">
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
                  const suite = getSuiteById(test.suiteId);
                  const suiteName = suite?.name || 'Unknown';
                  
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
                            const Icon = pluginIcons[plugin] || Code;
                            return (
                              <Icon 
                                key={plugin} 
                                className="w-4 h-4 text-[#666666] flex-shrink-0"
                                title={plugin}
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
                              onSelectSuite(test.suiteId);
                            }
                          }}
                          className="text-sm text-black hover:underline truncate block text-left w-full"
                        >
                          {suiteName}
                        </button>
                      </td>
                      <td className="px-6 h-14 align-middle">
                        <Sparkline results={test.recentResults} size="lg" shape="pill" />
                      </td>
                      <td className="px-6 h-14 align-middle">
                        <span className="text-sm">{test.successRate}</span>
                      </td>
                      <td className="px-6 h-14 align-middle">
                        <span className="text-sm text-[#666666]">{test.lastScheduledRun}</span>
                      </td>
                      <td className="px-6 h-14 align-middle">
                        <span className="text-sm text-[#666666]">{test.nextRun}</span>
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
          </div>
        ) : (
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
            <p className="text-[#666666] mb-4">No tests found matching your filters</p>
            <button
              onClick={() => {
                setSelectedPlugins([]);
                setSelectedSuites([]);
                setSearchQuery('');
              }}
              className="text-sm text-black hover:underline"
            >
              Clear filters
            </button>
          </div>
        )}
      </div>
    </div>
  );
}