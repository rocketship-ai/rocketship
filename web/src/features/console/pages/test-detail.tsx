import { ArrowLeft, Play, Edit3, AlertCircle } from 'lucide-react';
import { StatusBadge, EnvBadge, TriggerBadge } from '../components/status-badge';
import { MultiSelectDropdown } from '../components/multi-select-dropdown';
import { TestStepCardAdapter } from '../components/test-step-card';
import { useState } from 'react';

interface TestDetailProps {
  testId: string;
  onBack: () => void;
  onViewRun: (runId: string) => void;
  onViewSuite?: (suiteId: string) => void;
}

export function TestDetail({ testId, onBack, onViewRun, onViewSuite }: TestDetailProps) {
  const [activeTab, setActiveTab] = useState<'runHistory' | 'steps' | 'schedules'>('steps');
  const [selectedTriggers, setSelectedTriggers] = useState<string[]>(['schedule']);
  const [showTriggerDropdown, setShowTriggerDropdown] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const runsPerPage = 10;

  const test = {
    id: testId,
    name: 'Payment processing',
    suiteId: 'suite-1',
    suiteName: 'E-commerce Checkout Flow',
    branch: 'main',
    sha: 'abc1234567890',
    defaultBranchSha: 'def4567890123',
    isOutdated: false,
  };

  const steps = [
    {
      id: 'step-1',
      name: 'Create payment intent',
      plugin: 'HTTP',
      method: 'POST',
      url: 'https://api.stripe.com/v1/payment_intents',
      assertions: ['Status is 200', 'Response has id', 'Amount equals 2000'],
      saves: ['paymentIntentId'],
    },
    {
      id: 'step-2',
      name: 'Confirm payment',
      plugin: 'HTTP',
      method: 'POST',
      url: 'https://api.stripe.com/v1/payment_intents/{{paymentIntentId}}/confirm',
      assertions: ['Status is 200', 'Payment status is succeeded'],
      saves: ['confirmationId'],
    },
    {
      id: 'step-3',
      name: 'Retrieve payment details',
      plugin: 'HTTP',
      method: 'GET',
      url: 'https://api.stripe.com/v1/payment_intents/{{paymentIntentId}}',
      assertions: ['Status is 200', 'Amount captured equals 2000', 'Status is succeeded'],
      saves: [],
    },
    {
      id: 'step-4',
      name: 'Create refund',
      plugin: 'HTTP',
      method: 'POST',
      url: 'https://api.stripe.com/v1/refunds',
      assertions: ['Status is 200', 'Refund status is succeeded'],
      saves: ['refundId'],
    },
  ];

  const schedules = [
    {
      id: 'sched-1',
      env: 'staging',
      cron: '*/30 * * * *',
      enabled: true,
      lastRun: '15 minutes ago',
      nextRun: 'in 15 minutes',
    },
    {
      id: 'sched-2',
      env: 'production',
      cron: '0 0 * * *',
      enabled: true,
      lastRun: '6 hours ago',
      nextRun: 'in 18 hours',
    },
  ];

  // Activity: Test run activity in descending execution time order
  const activity = [
    {
      id: 'testrun-1',
      status: 'failed' as const,
      env: 'staging',
      initiator: 'schedule' as const,
      duration: '1m 12s',
      executed: '15 minutes ago',
      runId: 'run_abc123',
      branch: 'main',
      commit: 'abc1234',
    },
    {
      id: 'testrun-2',
      status: 'success' as const,
      env: 'staging',
      initiator: 'manual' as const,
      duration: '1m 34s',
      executed: '2 hours ago',
      runId: 'run_def456',
      branch: 'main',
      commit: 'def4567',
    },
    {
      id: 'testrun-3',
      status: 'success' as const,
      env: 'production',
      initiator: 'ci' as const,
      duration: '2m 10s',
      executed: '4 hours ago',
      runId: 'run_ghi789',
      branch: 'feature/payments',
      commit: 'ghi7890',
    },
    {
      id: 'testrun-4',
      status: 'success' as const,
      env: 'staging',
      initiator: 'schedule' as const,
      duration: '1m 45s',
      executed: '6 hours ago',
      runId: 'run_jkl012',
      branch: 'main',
      commit: 'jkl0123',
    },
    {
      id: 'testrun-5',
      status: 'failed' as const,
      env: 'production',
      initiator: 'ci' as const,
      duration: '2m 22s',
      executed: '8 hours ago',
      runId: 'run_mno345',
      branch: 'main',
      commit: 'mno3456',
    },
    {
      id: 'testrun-6',
      status: 'success' as const,
      env: 'staging',
      initiator: 'manual' as const,
      duration: '1m 55s',
      executed: '10 hours ago',
      runId: 'run_pqr678',
      branch: 'main',
      commit: 'pqr6789',
    },
    {
      id: 'testrun-7',
      status: 'success' as const,
      env: 'production',
      initiator: 'schedule' as const,
      duration: '2m 05s',
      executed: '12 hours ago',
      runId: 'run_stu901',
      branch: 'main',
      commit: 'stu9012',
    },
    {
      id: 'testrun-8',
      status: 'success' as const,
      env: 'staging',
      initiator: 'ci' as const,
      duration: '1m 48s',
      executed: '14 hours ago',
      runId: 'run_vwx234',
      branch: 'feature/refunds',
      commit: 'vwx2345',
    },
    {
      id: 'testrun-9',
      status: 'failed' as const,
      env: 'production',
      initiator: 'manual' as const,
      duration: '2m 30s',
      executed: '16 hours ago',
      runId: 'run_yza567',
      branch: 'main',
      commit: 'yza5678',
    },
    {
      id: 'testrun-10',
      status: 'success' as const,
      env: 'staging',
      initiator: 'schedule' as const,
      duration: '1m 25s',
      executed: '18 hours ago',
      runId: 'run_bcd890',
      branch: 'main',
      commit: 'bcd8901',
    },
    {
      id: 'testrun-11',
      status: 'success' as const,
      env: 'production',
      initiator: 'ci' as const,
      duration: '2m 15s',
      executed: '20 hours ago',
      runId: 'run_efg123',
      branch: 'main',
      commit: 'efg1234',
    },
    {
      id: 'testrun-12',
      status: 'success' as const,
      env: 'staging',
      initiator: 'manual' as const,
      duration: '1m 40s',
      executed: '22 hours ago',
      runId: 'run_hij456',
      branch: 'feature/api',
      commit: 'hij4567',
    },
    {
      id: 'testrun-13',
      status: 'failed' as const,
      env: 'production',
      initiator: 'schedule' as const,
      duration: '2m 50s',
      executed: '1 day ago',
      runId: 'run_klm789',
      branch: 'main',
      commit: 'klm7890',
    },
    {
      id: 'testrun-14',
      status: 'success' as const,
      env: 'staging',
      initiator: 'ci' as const,
      duration: '1m 30s',
      executed: '1 day ago',
      runId: 'run_nop012',
      branch: 'main',
      commit: 'nop0123',
    },
    {
      id: 'testrun-15',
      status: 'success' as const,
      env: 'production',
      initiator: 'manual' as const,
      duration: '2m 20s',
      executed: '1 day ago',
      runId: 'run_qrs345',
      branch: 'main',
      commit: 'qrs3456',
    },
    {
      id: 'testrun-16',
      status: 'success' as const,
      env: 'staging',
      initiator: 'schedule' as const,
      duration: '1m 50s',
      executed: '2 days ago',
      runId: 'run_tuv678',
      branch: 'main',
      commit: 'tuv6789',
    },
    {
      id: 'testrun-17',
      status: 'failed' as const,
      env: 'production',
      initiator: 'ci' as const,
      duration: '3m 10s',
      executed: '2 days ago',
      runId: 'run_wxy901',
      branch: 'feature/checkout',
      commit: 'wxy9012',
    },
    {
      id: 'testrun-18',
      status: 'success' as const,
      env: 'staging',
      initiator: 'manual' as const,
      duration: '1m 35s',
      executed: '2 days ago',
      runId: 'run_zab234',
      branch: 'main',
      commit: 'zab2345',
    },
    {
      id: 'testrun-19',
      status: 'success' as const,
      env: 'production',
      initiator: 'schedule' as const,
      duration: '2m 00s',
      executed: '2 days ago',
      runId: 'run_cde567',
      branch: 'main',
      commit: 'cde5678',
    },
    {
      id: 'testrun-20',
      status: 'success' as const,
      env: 'staging',
      initiator: 'ci' as const,
      duration: '1m 42s',
      executed: '3 days ago',
      runId: 'run_fgh890',
      branch: 'main',
      commit: 'fgh8901',
    },
    {
      id: 'testrun-21',
      status: 'failed' as const,
      env: 'production',
      initiator: 'manual' as const,
      duration: '2m 55s',
      executed: '3 days ago',
      runId: 'run_ijk123',
      branch: 'main',
      commit: 'ijk1234',
    },
    {
      id: 'testrun-22',
      status: 'success' as const,
      env: 'staging',
      initiator: 'schedule' as const,
      duration: '1m 28s',
      executed: '3 days ago',
      runId: 'run_lmn456',
      branch: 'main',
      commit: 'lmn4567',
    },
    {
      id: 'testrun-23',
      status: 'success' as const,
      env: 'production',
      initiator: 'ci' as const,
      duration: '2m 12s',
      executed: '3 days ago',
      runId: 'run_opq789',
      branch: 'feature/validation',
      commit: 'opq7890',
    },
    {
      id: 'testrun-24',
      status: 'success' as const,
      env: 'staging',
      initiator: 'manual' as const,
      duration: '1m 52s',
      executed: '4 days ago',
      runId: 'run_rst012',
      branch: 'main',
      commit: 'rst0123',
    },
    {
      id: 'testrun-25',
      status: 'success' as const,
      env: 'production',
      initiator: 'schedule' as const,
      duration: '2m 08s',
      executed: '4 days ago',
      runId: 'run_uvw345',
      branch: 'main',
      commit: 'uvw3456',
    },
  ];

  return (
    <div className="p-8 flex gap-6">
      {/* Left Sidebar - Recent Test Runs */}
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
                onSelectionChange={setSelectedTriggers}
                isOpen={showTriggerDropdown}
                onToggle={() => setShowTriggerDropdown(!showTriggerDropdown)}
              />
            </div>
          </div>

          {/* Recent runs list */}
          <div>
            {activity
              .filter(run => selectedTriggers.length === 0 || selectedTriggers.includes(run.initiator))
              .slice((currentPage - 1) * runsPerPage, currentPage * runsPerPage)
              .map((run) => (
                <div
                  key={run.id}
                  onClick={() => onViewRun(run.id)}
                  className="p-4 hover:bg-[#fafafa] transition-colors cursor-pointer"
                >
                  {/* Top row: Status with text on left, time on right */}
                  <div className="flex items-start justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <StatusBadge status={run.status} />
                      <span className="text-sm">
                        {run.status === 'success' ? 'Passed' : run.status === 'failed' ? 'Failed' : 'Running'}
                      </span>
                    </div>
                    <span className="text-xs text-[#999999]">{run.executed}</span>
                  </div>
                  
                  {/* Middle row: Badges */}
                  <div className="flex items-center gap-2 mb-2">
                    <EnvBadge env={run.env} />
                    <TriggerBadge trigger={run.initiator} />
                  </div>
                  
                  {/* Bottom row: Author (optional) and duration */}
                  <div className="flex items-center gap-2 text-xs text-[#666666]">
                    {run.initiator === 'manual' && (
                      <>
                        <span>Austin Rath</span>
                        <span className="text-[#999999]">•</span>
                      </>
                    )}
                    <span>{run.duration}</span>
                  </div>
                </div>
              ))}
          </div>

          {/* Pagination */}
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
                Page {currentPage} of {Math.ceil(activity.filter(run => selectedTriggers.length === 0 || selectedTriggers.includes(run.initiator)).length / runsPerPage)}
              </span>
              <button
                onClick={() => setCurrentPage(Math.min(Math.ceil(activity.filter(run => selectedTriggers.length === 0 || selectedTriggers.includes(run.initiator)).length / runsPerPage), currentPage + 1))}
                disabled={currentPage >= Math.ceil(activity.filter(run => selectedTriggers.length === 0 || selectedTriggers.includes(run.initiator)).length / runsPerPage)}
                className="text-xs px-3 py-1.5 bg-white border border-[#e5e5e5] rounded hover:bg-[#fafafa] disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Next
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 min-w-0">
        {/* Header */}
        <div className="mb-6">
          <button
            onClick={onBack}
            className="flex items-center gap-2 text-[#666666] hover:text-black mb-4 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to tests</span>
          </button>

          {test.isOutdated && (
            <div className="bg-[#f6a724]/10 border border-[#f6a724]/30 rounded-lg p-4 mb-4">
              <div className="flex items-start gap-3">
                <AlertCircle className="w-5 h-5 text-[#f6a724] flex-shrink-0 mt-0.5" />
                <div>
                  <p className="text-sm mb-1">
                    <strong>Default branch has advanced</strong>
                  </p>
                  <p className="text-sm text-[#666666] mb-2">
                    Test is at <code className="px-1 bg-white rounded text-xs">{test.sha.slice(0, 7)}</code>, but{' '}
                    {test.branch} is now at{' '}
                    <code className="px-1 bg-white rounded text-xs">{test.defaultBranchSha.slice(0, 7)}</code>
                  </p>
                  <button className="text-sm text-black hover:underline">
                    View diff →
                  </button>
                </div>
              </div>
            </div>
          )}

          <div className="flex items-start justify-between mb-6">
            <div>
              <h1 className="mb-0">{test.name}</h1>
              <p className="text-sm text-[#666666] mt-2">
                Part of{' '}
                <button
                  onClick={() => onViewSuite?.(test.suiteId)}
                  className="text-black hover:underline"
                >
                  {test.suiteName}
                </button>
              </p>
            </div>

            <div className="flex items-center gap-2">
              <button 
                disabled
                className="flex items-center gap-2 px-4 py-2 bg-white border border-[#e5e5e5] rounded-md text-[#999999] cursor-not-allowed opacity-60"
              >
                <Edit3 className="w-4 h-4" />
                <span>Edit</span>
              </button>
              <button className="flex items-center gap-2 px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors">
                <Play className="w-4 h-4" />
                <span>Run now</span>
              </button>
            </div>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 border-b border-[#e5e5e5]">
          <button
            onClick={() => setActiveTab('steps')}
            className={`px-4 py-2 transition-colors ${
              activeTab === 'steps'
                ? 'border-b-2 border-black text-black'
                : 'text-[#666666] hover:text-black'
            }`}
          >
            Steps
          </button>
          <button
            disabled
            className="px-4 py-2 text-[#999999] cursor-not-allowed opacity-50"
          >
            Schedules
          </button>
        </div>

        {/* Tab Content */}
        {activeTab === 'steps' && (
          <div className="space-y-3">
            {steps.map((step, idx) => (
              <div key={step.id}>
                <div className="bg-white rounded-lg overflow-hidden">
                  <TestStepCardAdapter step={step} stepNumber={idx + 1} />
                </div>
              </div>
            ))}
          </div>
        )}

        {activeTab === 'schedules' && (
          <div className="space-y-4">
            {schedules.map((schedule) => (
              <div
                key={schedule.id}
                className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6"
              >
                <div className="flex items-start justify-between mb-4">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-2">
                      <EnvBadge env={schedule.env} />
                    </div>
                    <p className="text-sm mb-2">
                      Schedule: {schedule.cron}
                    </p>
                    <code className="text-xs px-2 py-1 bg-[#fafafa] rounded border border-[#e5e5e5] text-[#666666] break-all">
                      {schedule.enabled ? 'Enabled' : 'Disabled'}
                    </code>
                  </div>
                </div>

                <div className="mt-3 pt-3 border-t border-[#e5e5e5]">
                  <p className="text-xs text-[#999999] mb-2">Last Run:</p>
                  <div className="flex flex-wrap gap-2">
                    <code
                      className="text-xs px-2 py-1 bg-[#fafafa] rounded border border-[#e5e5e5]"
                    >
                      {schedule.lastRun}
                    </code>
                  </div>
                </div>

                <div className="mt-3 pt-3 border-t border-[#e5e5e5]">
                  <p className="text-xs text-[#999999] mb-2">Next Run:</p>
                  <div className="flex flex-wrap gap-2">
                    <code
                      className="text-xs px-2 py-1 bg-[#fafafa] rounded border border-[#e5e5e5]"
                    >
                      {schedule.nextRun}
                    </code>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}