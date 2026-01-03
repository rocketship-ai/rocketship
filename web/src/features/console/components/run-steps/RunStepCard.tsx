import { useState } from 'react';
import { CheckCircle2, XCircle, Clock, Loader2, ChevronRight, ChevronDown } from 'lucide-react';
import type { RunStep } from '../../hooks/use-console-queries';
import { Tabs } from '../step-ui';
import { formatDuration } from '../../lib/format';
import { getPluginRenderer } from './renderers';
import { mapStepStatus, type StepUIStatus } from './types';

interface RunStepCardProps {
  step: RunStep;
  stepNumber: number;
}

export function RunStepCard({ step, stepNumber }: RunStepCardProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [activeTab, setActiveTab] = useState<string>('');

  const status = mapStepStatus(step.status);
  const renderer = getPluginRenderer(step.plugin);
  const tabs = renderer.getTabs(step);
  const summary = renderer.getSummary?.(step);

  // Set default active tab when expanding
  const handleToggle = () => {
    const newExpanded = !isExpanded;
    setIsExpanded(newExpanded);
    if (newExpanded && !activeTab && tabs.length > 0) {
      setActiveTab(tabs[0].id);
    }
  };

  // Calculate assertion counts for badge
  const executedAssertions = step.assertions_passed + step.assertions_failed;
  const plannedAssertionsCount = Array.isArray(step.step_config?.assertions) ? step.step_config.assertions.length : 0;
  const totalAssertions = executedAssertions > 0 ? executedAssertions : plannedAssertionsCount;
  const assertionsPassed = executedAssertions > 0 && step.assertions_failed === 0;
  const isPlannedOnly = executedAssertions === 0 && plannedAssertionsCount > 0;

  // Variables for pill (only saved variables, not config/runtime)
  const savedVariablesCount = step.variables_data?.filter(v =>
    v.source_type !== 'config' && v.source_type !== 'runtime'
  ).length ?? 0;

  // Check if there's content to show
  const hasExpandedContent = tabs.length > 0;

  return (
    <div className={`bg-white rounded-lg border border-[#e5e5e5] ${getBorderColor(status)} border-l-4 shadow-sm`}>
      {/* Collapsed Header */}
      <div
        className="px-6 py-7 cursor-pointer hover:bg-[#fafafa] transition-colors"
        onClick={handleToggle}
      >
        <div className="flex items-center">
          {/* Left: Status Icon */}
          <div className="flex-shrink-0 mr-4">
            <StatusIcon status={status} />
          </div>

          {/* Middle: Content */}
          <div className="flex-1 min-w-0">
            {/* Step label + plugin badge */}
            <div className="flex items-center gap-2 mb-1">
              <span className="text-[13px] text-[#71717a]">Step {stepNumber}</span>
              <span className="text-[11px] font-mono px-1.5 py-0.5 bg-[#18181b] text-white rounded">
                {step.plugin}
              </span>
            </div>

            {/* Step name */}
            <h4 className="text-[15px] font-medium text-[#18181b] leading-snug">{step.name}</h4>

            {/* HTTP-specific summary (method + URL) */}
            {summary && (
              <div className="flex items-center mt-2">
                <span className="text-[13px] font-mono font-semibold text-[#18181b]">{summary.method}</span>
                <span className="text-[13px] font-mono text-[#71717a] ml-1">{summary.domain}</span>
                <span className="text-[13px] font-mono text-[#a1a1aa]">{summary.path}</span>
              </div>
            )}

            {/* Error message */}
            {step.error_message && (
              <p className="text-xs text-[#ef4444] mt-2">{step.error_message}</p>
            )}
          </div>

          {/* Right side: badges, duration, chevron */}
          <div className="flex items-center gap-4 ml-auto pl-6 flex-shrink-0">
            {/* Assertions badge */}
            {totalAssertions > 0 && (
              <span className={`text-xs font-normal px-2 py-1 rounded ${
                isPlannedOnly
                  ? 'text-[#71717a] bg-[#f4f4f5]'
                  : assertionsPassed
                    ? 'text-[#4CBB17] bg-[#4CBB17]/10'
                    : 'text-[#ef4444] bg-[#ef4444]/10'
              }`}>
                {isPlannedOnly
                  ? `${totalAssertions} assertion${totalAssertions !== 1 ? 's' : ''}`
                  : `${step.assertions_passed}/${executedAssertions} assertions`}
              </span>
            )}

            {/* Variables count */}
            {savedVariablesCount > 0 && (
              <span className="text-xs text-[#a1a1aa]">
                {savedVariablesCount} variable{savedVariablesCount !== 1 ? 's' : ''}
              </span>
            )}

            {/* Duration */}
            {step.duration_ms !== undefined && (
              <span className="text-xs text-[#a1a1aa]">{formatDuration(step.duration_ms)}</span>
            )}

            {/* Expand/collapse chevron */}
            {isExpanded ? (
              <ChevronDown className="w-4 h-4 text-[#d4d4d8]" />
            ) : (
              <ChevronRight className="w-4 h-4 text-[#d4d4d8]" />
            )}
          </div>
        </div>
      </div>

      {/* Expanded Content */}
      {isExpanded && hasExpandedContent && (
        <div className="border-t border-[#e5e5e5]">
          {/* Tabs */}
          <Tabs
            tabs={tabs}
            activeId={activeTab}
            onChange={setActiveTab}
            className="ml-12 mr-4"
          />

          {/* Tab Content - rendered by plugin */}
          {renderer.render(step, activeTab)}
        </div>
      )}
    </div>
  );
}

function StatusIcon({ status }: { status: StepUIStatus }) {
  switch (status) {
    case 'success':
      return <CheckCircle2 className="w-5 h-5 text-[#4CBB17]" />;
    case 'failed':
      return <XCircle className="w-5 h-5 text-[#ef0000]" />;
    case 'running':
      return <Loader2 className="w-5 h-5 text-[#4CBB17] animate-spin" />;
    case 'pending':
    default:
      return <Clock className="w-5 h-5 text-[#999999]" />;
  }
}

function getBorderColor(status: StepUIStatus): string {
  switch (status) {
    case 'success':
      return 'border-l-[#4CBB17]';
    case 'failed':
      return 'border-l-[#ef0000]';
    case 'running':
      return 'border-l-[#4CBB17]';
    case 'pending':
    default:
      return 'border-l-[#999999]';
  }
}
