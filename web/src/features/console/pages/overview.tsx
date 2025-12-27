import { CheckCircle2, Circle, AlertCircle } from 'lucide-react';
import { Line, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ComposedChart } from 'recharts';
import { useState } from 'react';

interface OverviewProps {
  onNavigate: (page: string) => void;
}

export function Overview({ onNavigate }: OverviewProps) {
  const [_selectedProject, _setSelectedProject] = useState('all');
  const [_selectedEnv, _setSelectedEnv] = useState('all');
  const [_timeRange, _setTimeRange] = useState('7d');
  const [passRateToggle, setPassRateToggle] = useState<'all' | 'scheduled' | 'ci'>('all');

  // Setup checklist
  const setupComplete = false;
  const setupItems = [
    { done: true, label: 'Connect repository', action: 'Connect GitHub' },
    { done: false, label: 'Add CI environment variables', action: 'View docs' },
    { done: false, label: 'Configure first scheduled monitor', action: 'Create schedule' },
  ];

  // "Now" row data with sparklines
  const nowMetrics = [
    {
      label: 'Failing Monitors',
      value: 3,
      sparkline: [1, 2, 1, 3, 2, 3, 3],
      trend: 'up',
      color: '#ef0000',
      link: 'test-health?status=failed'
    },
    {
      label: 'Failing Tests (24h)',
      value: 12,
      sparkline: [8, 10, 9, 12, 11, 13, 12],
      trend: 'stable',
      color: '#ef0000',
      link: 'suite-activity?status=failed&timeRange=24h'
    },
    {
      label: 'Runs in Progress',
      value: 2,
      sparkline: [3, 2, 4, 3, 2, 3, 2],
      trend: 'down',
      color: '#f6a724',
      link: 'suite-activity?status=running'
    },
    {
      label: 'Pass Rate (24h)',
      value: '94.2%',
      sparkline: [92, 93, 95, 94, 93, 94, 94.2],
      trend: 'up',
      color: '#4CBB17',
      link: 'suite-activity?timeRange=24h'
    },
    {
      label: 'Median Duration (24h)',
      value: '3m 24s',
      sparkline: [180, 195, 210, 204, 198, 205, 204],
      trend: 'stable',
      color: '#666666',
      link: 'suite-activity?timeRange=24h&sort=duration'
    }
  ];

  // Pass rate over time with volume
  const passRateData = [
    { date: 'Dec 19', passRate: 92.3, volume: 145 },
    { date: 'Dec 20', passRate: 93.1, volume: 167 },
    { date: 'Dec 21', passRate: 94.8, volume: 189 },
    { date: 'Dec 22', passRate: 93.5, volume: 201 },
    { date: 'Dec 23', passRate: 92.9, volume: 178 },
    { date: 'Dec 24', passRate: 94.6, volume: 156 },
    { date: 'Dec 25', passRate: 94.2, volume: 142 },
  ];

  // Failures by suite (top 8)
  const failuresBySuite = [
    { suite: 'Auth Flow', passes: 142, failures: 8 },
    { suite: 'Payment API', passes: 156, failures: 5 },
    { suite: 'User Onboarding', passes: 178, failures: 4 },
    { suite: 'Search Service', passes: 189, failures: 3 },
    { suite: 'Notification System', passes: 164, failures: 3 },
    { suite: 'Analytics Pipeline', passes: 145, failures: 2 },
    { suite: 'Report Generation', passes: 167, failures: 2 },
    { suite: 'Data Sync', passes: 201, failures: 1 },
  ];

  return (
    <div className="flex-1 min-w-0 p-8">
      <div className="max-w-[1600px] mx-auto">
        {/* Setup Banner */}
        {!setupComplete ? (
          <div className="bg-[#f6a724]/10 border-2 border-[#f6a724] rounded-lg p-6 mb-6">
            <div className="flex items-start gap-4">
              <AlertCircle className="w-6 h-6 text-[#f6a724] flex-shrink-0 mt-1" />
              <div className="flex-1">
                <h2 className="mb-2">Finish setup to start monitoring</h2>
                <p className="text-sm text-[#666666] mb-4">
                  Complete these steps to unlock continuous monitoring and CI integration
                </p>
                <div className="space-y-3 mb-4">
                  {setupItems.map((item, idx) => (
                    <div key={idx} className="flex items-center gap-3">
                      {item.done ? (
                        <CheckCircle2 className="w-5 h-5 text-[#4CBB17]" />
                      ) : (
                        <Circle className="w-5 h-5 text-[#999999]" />
                      )}
                      <span className={`text-sm flex-1 ${item.done ? 'text-[#666666]' : 'text-black'}`}>
                        {item.label}
                      </span>
                      {!item.done && (
                        <button className="text-sm text-black hover:underline">
                          {item.action} →
                        </button>
                      )}
                    </div>
                  ))}
                </div>
                <button className="px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors">
                  Connect repository
                </button>
              </div>
            </div>
          </div>
        ) : (
          <div className="bg-[#4CBB17]/10 border border-[#4CBB17] rounded-md px-3 py-2 mb-6 inline-flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4 text-[#4CBB17]" />
            <span className="text-sm text-[#4CBB17]">Setup complete</span>
          </div>
        )}

        {/* "Now" Row - 4 Tiles */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4 mb-6">
          {nowMetrics.map((metric, idx) => (
            <button
              key={idx}
              onClick={() => onNavigate(metric.link.split('?')[0])}
              className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-5 hover:border-[#999999] transition-colors text-left"
            >
              <div className="flex items-start justify-between mb-3">
                <span className="text-sm text-[#666666]">{metric.label}</span>
              </div>
              <div className="text-3xl">
                {metric.value}
              </div>
            </button>
          ))}
        </div>

        {/* Main Charts Row */}
        <div className="grid grid-cols-1 lg:grid-cols-5 gap-6 mb-6">
          {/* Pass Rate Over Time - Takes 3 columns */}
          <div className="lg:col-span-3 bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <div className="flex items-center justify-between mb-6">
              <h3>Pass Rate Over Time</h3>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setPassRateToggle('all')}
                  className={`px-3 py-1 text-sm rounded transition-colors ${
                    passRateToggle === 'all'
                      ? 'bg-black text-white'
                      : 'bg-[#fafafa] text-[#666666] hover:bg-[#e5e5e5]'
                  }`}
                >
                  All
                </button>
                <button
                  onClick={() => setPassRateToggle('scheduled')}
                  className={`px-3 py-1 text-sm rounded transition-colors ${
                    passRateToggle === 'scheduled'
                      ? 'bg-black text-white'
                      : 'bg-[#fafafa] text-[#666666] hover:bg-[#e5e5e5]'
                  }`}
                >
                  Scheduled
                </button>
                <button
                  onClick={() => setPassRateToggle('ci')}
                  className={`px-3 py-1 text-sm rounded transition-colors ${
                    passRateToggle === 'ci'
                      ? 'bg-black text-white'
                      : 'bg-[#fafafa] text-[#666666] hover:bg-[#e5e5e5]'
                  }`}
                >
                  CI
                </button>
              </div>
            </div>
            <ResponsiveContainer width="100%" height={280}>
              <ComposedChart data={passRateData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e5e5e5" />
                <XAxis
                  dataKey="date"
                  tick={{ fill: '#666666', fontSize: 12 }}
                  stroke="#e5e5e5"
                />
                <YAxis
                  yAxisId="left"
                  tick={{ fill: '#666666', fontSize: 12 }}
                  stroke="#e5e5e5"
                  domain={[85, 100]}
                  label={{ value: 'Pass Rate (%)', angle: -90, position: 'insideLeft', style: { fill: '#666666', fontSize: 12 } }}
                />
                <YAxis
                  yAxisId="right"
                  orientation="right"
                  tick={{ fill: '#999999', fontSize: 12 }}
                  stroke="#e5e5e5"
                  label={{ value: 'Volume', angle: 90, position: 'insideRight', style: { fill: '#999999', fontSize: 12 } }}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: 'white',
                    border: '1px solid #e5e5e5',
                    borderRadius: '6px',
                    fontSize: '12px'
                  }}
                />
                <Bar
                  yAxisId="right"
                  dataKey="volume"
                  fill="#e5e5e5"
                  opacity={0.3}
                  name="Run Volume"
                />
                <Line
                  yAxisId="left"
                  type="monotone"
                  dataKey="passRate"
                  stroke="#4CBB17"
                  strokeWidth={3}
                  dot={{ fill: '#4CBB17', r: 4 }}
                  name="Pass Rate"
                />
              </ComposedChart>
            </ResponsiveContainer>
          </div>

          {/* Failures by Suite - Takes 2 columns */}
          <div className="lg:col-span-2 bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <h3 className="mb-6">Recent Failures by Suite (24h)</h3>
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={failuresBySuite} layout="vertical">
                <CartesianGrid strokeDasharray="3 3" stroke="#e5e5e5" />
                <XAxis
                  type="number"
                  tick={{ fill: '#666666', fontSize: 12 }}
                  stroke="#e5e5e5"
                />
                <YAxis
                  type="category"
                  dataKey="suite"
                  tick={{ fill: '#666666', fontSize: 11 }}
                  stroke="#e5e5e5"
                  width={120}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: 'white',
                    border: '1px solid #e5e5e5',
                    borderRadius: '6px',
                    fontSize: '12px'
                  }}
                />
                <Bar dataKey="passes" stackId="a" fill="#4CBB17" name="Passes" />
                <Bar dataKey="failures" stackId="a" fill="#ef0000" name="Failures" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </div>
  );
}