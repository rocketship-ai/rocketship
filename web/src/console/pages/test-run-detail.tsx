import { ArrowLeft, Play, AlertCircle, Edit3 } from 'lucide-react';
import { StatusBadge, EnvBadge, InitiatorBadge, ConfigSourceBadge } from '../components/status-badge';
import { TestStepCardAdapter } from '../components/test-step-card';
import { useState } from 'react';

interface TestRunDetailProps {
  testRunId: string;
  onBack: () => void;
}

export function TestRunDetail({ testRunId, onBack }: TestRunDetailProps) {
  const [activeTab, setActiveTab] = useState<'steps' | 'logs' | 'artifacts'>('steps');

  const testRun = {
    id: testRunId,
    testName: 'Payment processing',
    status: 'failed' as const,
    env: 'staging',
    initiator: { type: 'ci' as const, name: 'GitHub Actions' },
    configSource: { type: 'repo' as const, sha: 'def4567890123' },
    duration: '1m 12s',
    started: '2024-11-30 14:23:45',
    ended: '2024-11-30 14:24:57',
    branch: 'feature/payment-v2',
    commit: 'def4567',
  };

  const steps = [
    {
      id: 'step-1',
      name: 'Create payment intent',
      plugin: 'HTTP',
      method: 'POST',
      url: 'https://api.stripe.com/v1/payment_intents',
      status: 'success' as const,
      duration: '2s',
      assertions: [
        { text: 'Status is 200', passed: true },
        { text: 'Response has id', passed: true },
        { text: 'Amount equals 2000', passed: true }
      ],
      saves: ['paymentIntentId'],
      details: {
        request: {
          headers: {
            'Authorization': 'Bearer sk_test_*********************',
            'Content-Type': 'application/json',
            'User-Agent': 'RocketshipCloud/1.0'
          },
          body: {
            amount: 2000,
            currency: 'usd',
            payment_method_types: ['card']
          }
        },
        response: {
          headers: {
            'content-type': 'application/json',
            'request-id': 'req_abc123xyz',
            'stripe-version': '2023-10-16'
          },
          body: {
            id: 'pi_abc123',
            object: 'payment_intent',
            amount: 2000,
            currency: 'usd',
            status: 'requires_payment_method'
          },
          statusCode: 200,
          latency: 342
        }
      },
    },
    {
      id: 'step-2',
      name: 'Confirm payment',
      plugin: 'HTTP',
      method: 'POST',
      url: 'https://api.stripe.com/v1/payment_intents/pi_abc123/confirm',
      status: 'success' as const,
      duration: '8s',
      assertions: [
        { text: 'Status is 200', passed: true },
        { text: 'Payment status is succeeded', passed: true }
      ],
      saves: ['confirmationId'],
      details: {
        request: {
          headers: {
            'Authorization': 'Bearer sk_test_*********************',
            'Content-Type': 'application/json',
            'User-Agent': 'RocketshipCloud/1.0'
          },
          body: {
            payment_method: 'pm_card_visa'
          }
        },
        response: {
          headers: {
            'content-type': 'application/json',
            'request-id': 'req_def456xyz',
            'stripe-version': '2023-10-16'
          },
          body: {
            id: 'pi_abc123',
            object: 'payment_intent',
            amount: 2000,
            status: 'succeeded',
            charges: {
              data: [{ id: 'ch_xyz789', amount: 2000, status: 'succeeded' }]
            }
          },
          statusCode: 200,
          latency: 1248
        }
      },
    },
    {
      id: 'step-3',
      name: 'Retrieve payment details',
      plugin: 'HTTP',
      method: 'GET',
      url: 'https://api.stripe.com/v1/payment_intents/pi_abc123',
      status: 'failed' as const,
      duration: '1s',
      assertions: [
        { text: 'Status is 200', passed: true },
        { text: 'Amount captured equals 2000', passed: false },
        { text: 'Status is succeeded', passed: true }
      ],
      saves: [],
      details: {
        request: {
          headers: {
            'Authorization': 'Bearer sk_test_*********************',
            'Content-Type': 'application/json',
            'User-Agent': 'RocketshipCloud/1.0'
          },
          body: null
        },
        response: {
          headers: {
            'content-type': 'application/json',
            'request-id': 'req_ghi789xyz',
            'stripe-version': '2023-10-16'
          },
          body: {
            id: 'pi_abc123',
            object: 'payment_intent',
            amount: 2000,
            amount_capturable: 0,
            amount_received: 1500,
            status: 'succeeded'
          },
          statusCode: 200,
          latency: 198
        }
      },
      error: 'Assertion failed: Amount captured equals 2000 (expected: 2000, actual: 1500)',
    },
    {
      id: 'step-4',
      name: 'Create refund',
      plugin: 'HTTP',
      method: 'POST',
      url: 'https://api.stripe.com/v1/refunds',
      status: 'skipped' as const,
      duration: '0s',
      assertions: [
        { text: 'Status is 200', passed: null },
        { text: 'Refund status is succeeded', passed: null }
      ],
      saves: ['refundId'],
      details: {
        request: {
          headers: {
            'Authorization': 'Bearer sk_test_*********************',
            'Content-Type': 'application/json',
            'User-Agent': 'RocketshipCloud/1.0'
          },
          body: null
        },
        response: {
          headers: {},
          body: null,
          statusCode: 0,
          latency: 0
        }
      },
      error: 'Step skipped due to previous failure',
    },
  ];

  const logs = `[14:23:45] Starting test: Payment processing
[14:23:46] Loading test configuration from def4567
[14:23:47] Environment: staging
[14:23:47] Step 1/4: Create payment intent - PASSED (2s)
[14:23:55] Step 2/4: Confirm payment - PASSED (8s)
[14:23:56] Step 3/4: Retrieve payment details - FAILED
[14:23:56] Step 4/4: Create refund - SKIPPED
[14:23:57] Test run completed - 1 passed, 1 failed, 2 skipped`;

  const failingStep = steps.find((s) => s.status === 'failed');
  const passedCount = steps.filter(s => s.status === 'success').length;
  const failedCount = steps.filter(s => s.status === 'failed').length;

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
            <span>Back to test</span>
          </button>

          <div className="flex items-start justify-between mb-4">
            <div>
              <h1 className="mb-2">{testRun.testName}</h1>
              <div className="flex items-center gap-3 flex-wrap">
                <StatusBadge status={testRun.status} />
                <EnvBadge env={testRun.env} />
                <InitiatorBadge initiator={testRun.initiator.type} name={testRun.initiator.name} />
                <ConfigSourceBadge type={testRun.configSource.type} sha={testRun.configSource.sha} />
              </div>
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

          {/* Stats */}
          <div className="grid grid-cols-3 gap-4">
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Duration</p>
              <p className="text-xl">{testRun.duration}</p>
            </div>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Passed</p>
              <p className="text-xl text-[#228b22]">{passedCount} steps</p>
            </div>
            <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-4">
              <p className="text-sm text-[#666666] mb-1">Failed</p>
              <p className="text-xl text-[#ef0000]">{failedCount} steps</p>
            </div>
          </div>
        </div>

        {/* Failing Step Alert */}
        {failingStep && (
          <div className="bg-[#ef0000]/5 border border-[#ef0000]/20 rounded-lg p-4 mb-6">
            <div className="flex items-start gap-3">
              <AlertCircle className="w-5 h-5 text-[#ef0000] flex-shrink-0 mt-0.5" />
              <div className="flex-1">
                <p className="text-sm mb-1">
                  <strong>{failingStep.name}</strong> failed
                </p>
                {failingStep.error && (
                  <p className="text-sm text-[#666666]">{failingStep.error}</p>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Tabs */}
        <div className="flex gap-1 mb-6 border-b border-[#e5e5e5]">
          <button
            onClick={() => setActiveTab('steps')}
            className={`px-4 py-2 capitalize transition-colors ${
              activeTab === 'steps'
                ? 'border-b-2 border-black text-black'
                : 'text-[#666666] hover:text-black'
            }`}
          >
            steps
          </button>
          <button
            onClick={() => setActiveTab('logs')}
            className={`px-4 py-2 capitalize transition-colors ${
              activeTab === 'logs'
                ? 'border-b-2 border-black text-black'
                : 'text-[#666666] hover:text-black'
            }`}
          >
            logs
          </button>
          <button
            disabled
            className="px-4 py-2 text-[#999999] cursor-not-allowed opacity-50"
          >
            Artifacts
          </button>
        </div>

        {/* Tab Content */}
        {activeTab === 'steps' && (
          <div className="space-y-3">
            {steps.map((step, idx) => (
              <div key={step.id}>
                <TestStepCardAdapter
                  step={step}
                  stepNumber={idx + 1}
                />
              </div>
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
          <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-6">
            <p className="text-sm text-[#666666]">Artifacts tab is disabled</p>
          </div>
        )}
      </div>
    </div>
  );
}