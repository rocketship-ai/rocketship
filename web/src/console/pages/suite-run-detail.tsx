import { ArrowLeft, RotateCw, Download, GitBranch, Hash, CheckCircle2, XCircle, Clock } from 'lucide-react';
import { EnvBadge, InitiatorBadge, ConfigSourceBadge } from '../components/status-badge';
import { TestItem } from '../components/test-item';
import { useState } from 'react';

interface SuiteRunDetailProps {
  suiteRunId: string;
  onBack: () => void;
  onViewTestRun: (testRunId: string) => void;
}

export function SuiteRunDetail({ suiteRunId, onBack, onViewTestRun }: SuiteRunDetailProps) {
  const [activeTab, setActiveTab] = useState<'test-runs' | 'logs' | 'artifacts'>('test-runs');

  const suiteRun = {
    id: suiteRunId,
    suiteName: 'API regression suite',
    status: 'failed' as 'success' | 'failed' | 'running',
    env: 'staging',
    initiator: 'ci' as const,
    configSource: { type: 'repo' as const, sha: 'def4567890123' },
    duration: '1m 57s',
    started: '2024-11-30 14:23:45',
    ended: '2024-11-30 14:25:42',
    branch: 'feature/payment-v2',
    commit: 'def4567',
    passed: 2,
    failed: 1,
    skipped: 0,
  };

  // Test runs within this suite run - only 3 tests, all HTTP
  const testRuns = [
    {
      id: 'testrun-1',
      name: 'User authentication',
      status: 'success' as const,
      duration: '1m 12s',
      steps: [
        {
          id: 'step-1',
          plugin: 'HTTP',
          name: 'Health check',
          status: 'success' as const,
          duration: '89ms',
          input: {
            method: 'GET',
            url: 'https://api.staging.example.com/health',
            headers: {},
            body: null,
          },
          output: {
            statusCode: 200,
            headers: { 'Content-Type': 'application/json' },
            body: { status: 'ok' },
            duration: '89ms',
          },
        },
        {
          id: 'step-2',
          plugin: 'HTTP',
          name: 'Login request',
          status: 'success' as const,
          duration: '234ms',
          input: {
            method: 'POST',
            url: 'https://api.staging.example.com/api/auth/login',
            headers: {
              'Content-Type': 'application/json',
            },
            body: { email: 'test@example.com', password: '***' },
          },
          output: {
            statusCode: 200,
            headers: {
              'Content-Type': 'application/json',
              'X-Request-Id': 'req_abc123',
            },
            body: { token: 'eyJhbGc...', user: { id: 123, email: 'test@example.com' } },
            duration: '234ms',
          },
        },
        {
          id: 'step-3',
          plugin: 'HTTP',
          name: 'Verify token',
          status: 'success' as const,
          duration: '145ms',
          input: {
            method: 'GET',
            url: 'https://api.staging.example.com/api/auth/verify',
            headers: {
              'Authorization': 'Bearer eyJhbGc...',
            },
            body: null,
          },
          output: {
            statusCode: 200,
            headers: {
              'Content-Type': 'application/json',
            },
            body: { valid: true, userId: 123 },
            duration: '145ms',
          },
        },
        {
          id: 'step-4',
          plugin: 'HTTP',
          name: 'Get user profile',
          status: 'success' as const,
          duration: '178ms',
          input: {
            method: 'GET',
            url: 'https://api.staging.example.com/api/users/123',
            headers: {
              'Authorization': 'Bearer eyJhbGc...',
            },
            body: null,
          },
          output: {
            statusCode: 200,
            headers: {
              'Content-Type': 'application/json',
            },
            body: { id: 123, email: 'test@example.com', name: 'Test User' },
            duration: '178ms',
          },
        },
      ],
    },
    {
      id: 'testrun-2',
      name: 'Payment processing',
      status: 'failed' as const,
      duration: '2m 34s',
      steps: [
        {
          id: 'step-5',
          plugin: 'HTTP',
          name: 'Get customer',
          status: 'success' as const,
          duration: '198ms',
          input: {
            method: 'GET',
            url: 'https://api.staging.example.com/api/customers/cus_123',
            headers: {
              'Authorization': 'Bearer eyJhbGc...',
            },
            body: null,
          },
          output: {
            statusCode: 200,
            headers: {
              'Content-Type': 'application/json',
            },
            body: { id: 'cus_123', email: 'customer@example.com' },
            duration: '198ms',
          },
        },
        {
          id: 'step-6',
          plugin: 'HTTP',
          name: 'Create payment',
          status: 'success' as const,
          duration: '312ms',
          input: {
            method: 'POST',
            url: 'https://api.staging.example.com/api/payments/create',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer eyJhbGc...',
            },
            body: { amount: 1000, currency: 'USD', customerId: 'cus_123' },
          },
          output: {
            statusCode: 200,
            headers: {
              'Content-Type': 'application/json',
            },
            body: { paymentId: 'pay_456', status: 'pending' },
            duration: '312ms',
          },
        },
        {
          id: 'step-7',
          plugin: 'HTTP',
          name: 'Confirm payment',
          status: 'failed' as const,
          duration: '5234ms',
          input: {
            method: 'POST',
            url: 'https://api.staging.example.com/api/payments/confirm',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer eyJhbGc...',
            },
            body: { paymentId: 'pay_456' },
          },
          output: {
            statusCode: 500,
            headers: {
              'Content-Type': 'application/json',
            },
            body: { error: 'Payment gateway timeout', code: 'GATEWAY_TIMEOUT' },
            duration: '5234ms',
          },
        },
        {
          id: 'step-8',
          plugin: 'HTTP',
          name: 'Retry payment',
          status: 'failed' as const,
          duration: '5123ms',
          input: {
            method: 'POST',
            url: 'https://api.staging.example.com/api/payments/retry',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer eyJhbGc...',
            },
            body: { paymentId: 'pay_456' },
          },
          output: {
            statusCode: 500,
            headers: {
              'Content-Type': 'application/json',
            },
            body: { error: 'Payment gateway timeout', code: 'GATEWAY_TIMEOUT' },
            duration: '5123ms',
          },
        },
        {
          id: 'step-9',
          plugin: 'HTTP',
          name: 'Send failure notification',
          status: 'success' as const,
          duration: '256ms',
          input: {
            method: 'POST',
            url: 'https://api.staging.example.com/api/notifications/send',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer eyJhbGc...',
            },
            body: { type: 'payment_failed', paymentId: 'pay_456' },
          },
          output: {
            statusCode: 200,
            headers: {
              'Content-Type': 'application/json',
            },
            body: { notificationId: 'notif_789', sent: true },
            duration: '256ms',
          },
        },
      ],
    },
    {
      id: 'testrun-3',
      name: 'Webhook endpoints',
      status: 'success' as const,
      duration: '45s',
      steps: [
        {
          id: 'step-10',
          plugin: 'HTTP',
          name: 'Stripe webhook',
          status: 'success' as const,
          duration: '189ms',
          input: {
            method: 'POST',
            url: 'https://api.staging.example.com/api/webhooks/stripe',
            headers: {
              'Content-Type': 'application/json',
              'Stripe-Signature': 't=1234567890,v1=...',
            },
            body: { type: 'payment.succeeded', data: { id: 'pay_456' } },
          },
          output: {
            statusCode: 200,
            headers: {
              'Content-Type': 'application/json',
            },
            body: { received: true },
            duration: '189ms',
          },
        },
        {
          id: 'step-11',
          plugin: 'HTTP',
          name: 'Verify webhook signature',
          status: 'success' as const,
          duration: '23ms',
          input: {
            method: 'POST',
            url: 'https://api.staging.example.com/api/webhooks/verify',
            headers: {
              'Content-Type': 'application/json',
            },
            body: { signature: 't=1234567890,v1=...', payload: '...' },
          },
          output: {
            statusCode: 200,
            headers: {
              'Content-Type': 'application/json',
            },
            body: { valid: true },
            duration: '23ms',
          },
        },
        {
          id: 'step-12',
          plugin: 'HTTP',
          name: 'Process webhook event',
          status: 'success' as const,
          duration: '345ms',
          input: {
            method: 'POST',
            url: 'https://api.staging.example.com/api/events/process',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer eyJhbGc...',
            },
            body: { eventType: 'payment.succeeded', paymentId: 'pay_456' },
          },
          output: {
            statusCode: 200,
            headers: {
              'Content-Type': 'application/json',
            },
            body: { processed: true, eventId: 'evt_123' },
            duration: '345ms',
          },
        },
      ],
    },
  ];

  const logs = `[14:23:45] Starting suite run: API regression suite
[14:23:45] Environment: staging
[14:23:45] Initiator: GitHub Actions (CI)
[14:23:45] Config source: repo@def4567890123
[14:23:46] Running test 1/10: User authentication
[14:24:31] ✓ User authentication passed (45s)
[14:24:31] Running test 2/10: Payment processing
[14:25:43] ✗ Payment processing failed (1m 12s)
[14:25:43] Running test 3/10: Database migrations
[14:26:05] ✓ Database migrations passed (22s)
[14:26:05] Running test 4/10: Email notifications
[14:26:43] ✓ Email notifications passed (38s)
[14:26:43] Running test 5/10: Webhook endpoints
[14:27:48] ✗ Webhook endpoints failed (1m 5s)
[14:27:48] Running test 6/10: User profile updates
[14:28:19] ✓ User profile updates passed (31s)
[14:28:19] Suite run completed
[14:28:20] Total duration: 4m 35s
[14:28:20] Results: 8 passed, 2 failed, 0 skipped`;

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-6">
          <button
            onClick={onBack}
            className="flex items-center gap-2 text-[#666666] hover:text-black mb-4 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to suite</span>
          </button>

          <div className="flex items-start justify-between mb-4">
            <div>
              <h1 className="mb-2">{suiteRun.suiteName}</h1>
              <div className="flex items-center gap-3 flex-wrap">
                <div>
                  {suiteRun.status === 'success' && (
                    <CheckCircle2 className="w-5 h-5 text-[#4CBB17]" />
                  )}
                  {suiteRun.status === 'failed' && (
                    <XCircle className="w-5 h-5 text-[#ef0000]" />
                  )}
                  {suiteRun.status === 'running' && (
                    <Clock className="w-5 h-5 text-[#4CBB17] animate-spin" />
                  )}
                </div>
                <EnvBadge env={suiteRun.env} />
                <InitiatorBadge initiator={suiteRun.initiator} />
                <ConfigSourceBadge type={suiteRun.configSource.type} sha={suiteRun.configSource.sha} />
              </div>
            </div>

            <div className="flex items-center gap-2">
              <button disabled className="flex items-center gap-2 px-4 py-2 bg-white border border-[#e5e5e5] rounded-md text-[#cccccc] cursor-not-allowed">
                <RotateCw className="w-4 h-4" />
                <span>Rerun</span>
              </button>
              <button disabled className="flex items-center gap-2 px-4 py-2 bg-white border border-[#e5e5e5] rounded-md text-[#cccccc] cursor-not-allowed">
                <Download className="w-4 h-4" />
                <span>Export</span>
              </button>
            </div>
          </div>

          {/* Stats */}
          <div className="grid grid-cols-4 gap-4">
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Duration</p>
              <p className="text-xl">{suiteRun.duration}</p>
            </div>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Passed</p>
              <p className="text-xl text-[#228b22]">{suiteRun.passed}</p>
            </div>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Failed</p>
              <p className="text-xl text-[#ef0000]">{suiteRun.failed}</p>
            </div>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Skipped</p>
              <p className="text-xl text-[#999999]">{suiteRun.skipped}</p>
            </div>
          </div>
        </div>

        {/* Metadata */}
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6 mb-6">
          <div className="grid grid-cols-2 gap-6">
            <div>
              <p className="text-xs text-[#999999] mb-1">Started</p>
              <p className="text-sm">{suiteRun.started}</p>
            </div>
            <div>
              <p className="text-xs text-[#999999] mb-1">Ended</p>
              <p className="text-sm">{suiteRun.ended}</p>
            </div>
            <div>
              <p className="text-xs text-[#999999] mb-1">Branch</p>
              <p className="text-sm flex items-center gap-2">
                <GitBranch className="w-3 h-3" />
                {suiteRun.branch}
              </p>
            </div>
            <div>
              <p className="text-xs text-[#999999] mb-1">Commit</p>
              <p className="text-sm flex items-center gap-2">
                <Hash className="w-3 h-3" />
                {suiteRun.commit}
              </p>
            </div>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 border-b border-[#e5e5e5]">
          {(['test-runs', 'logs', 'artifacts'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => tab !== 'artifacts' && setActiveTab(tab)}
              disabled={tab === 'artifacts'}
              className={`px-4 py-2 capitalize transition-colors ${
                tab === 'artifacts'
                  ? 'text-[#cccccc] cursor-not-allowed'
                  : activeTab === tab
                  ? 'border-b-2 border-black text-black'
                  : 'text-[#666666] hover:text-black'
              }`}
            >
              {tab === 'test-runs' ? 'Test Runs' : tab}
            </button>
          ))}
        </div>

        {/* Tab Content */}
        {activeTab === 'test-runs' && (
          <div className="space-y-3">
            {testRuns.map((testRun) => (
              <TestItem
                key={testRun.id}
                test={testRun}
                onClick={() => onViewTestRun(testRun.id)}
              />
            ))}
          </div>
        )}

        {activeTab === 'logs' && (
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <div className="flex justify-end gap-2 mb-4">
              <button className="text-sm text-[#666666] hover:text-black transition-colors">
                Copy
              </button>
              <button className="text-sm text-[#666666] hover:text-black transition-colors">
                Download
              </button>
            </div>
            <pre className="bg-black rounded p-4 font-mono text-xs text-[#00ff00] overflow-x-auto max-h-96 overflow-y-auto whitespace-pre-wrap">
              {logs}
            </pre>
          </div>
        )}

        {activeTab === 'artifacts' && (
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-12 text-center">
            <p className="text-[#666666]">No artifacts available</p>
          </div>
        )}
      </div>
    </div>
  );
}