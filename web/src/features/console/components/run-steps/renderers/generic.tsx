import yaml from 'js-yaml';
import { Check, X } from 'lucide-react';
import type { RunStep } from '../../../hooks/use-console-queries';
import { CopyButton, CodeBlock } from '../../step-ui';

interface GenericRendererProps {
  step: RunStep;
  activeTab: string;
}

/** Generic fallback renderer for non-HTTP plugins */
export function GenericRenderer({ step, activeTab }: GenericRendererProps) {
  return (
    <div className="py-4 px-4 ml-8">
      {activeTab === 'details' && (
        <DetailsTab step={step} />
      )}
      {activeTab === 'assertions' && (
        <AssertionsTab step={step} />
      )}
      {activeTab === 'variables' && (
        <VariablesTab step={step} />
      )}
      {activeTab === 'code' && (
        <CodeTab step={step} />
      )}
    </div>
  );
}

function DetailsTab({ step }: { step: RunStep }) {
  // Show any available data from request/response
  const details: Record<string, unknown> = {};

  if (step.request_data) {
    details.request = step.request_data;
  }

  if (step.response_data) {
    details.response = step.response_data;
  }

  if (Object.keys(details).length === 0) {
    return <p className="text-sm text-[#888888]">No details available for this step.</p>;
  }

  return (
    <CodeBlock
      code={JSON.stringify(details, null, 2)}
      label="Step Details"
      maxHeight="24rem"
    />
  );
}

function AssertionsTab({ step }: { step: RunStep }) {
  if (!step.assertions_data || step.assertions_data.length === 0) {
    return <p className="text-sm text-[#888888]">No assertion data available.</p>;
  }

  return (
    <div className="rounded border border-[#e8e8e8] overflow-hidden">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-[#f8f8f8] border-b border-[#e8e8e8]">
            <th className="px-3 py-2 text-left text-xs font-medium text-[#888888] w-10"></th>
            <th className="px-3 py-2 text-left text-xs font-medium text-[#888888]">Assertion</th>
            <th className="px-3 py-2 text-left text-xs font-medium text-[#888888]">Expected</th>
            <th className="px-3 py-2 text-left text-xs font-medium text-[#888888]">Actual</th>
          </tr>
        </thead>
        <tbody>
          {step.assertions_data.map((assertion, idx) => (
            <tr key={idx} className={idx % 2 === 0 ? 'bg-white' : 'bg-[#f8f8f8]'}>
              <td className="px-3 py-2">
                {assertion.passed ? (
                  <Check className="w-4 h-4 text-[#4CBB17]" />
                ) : (
                  <X className="w-4 h-4 text-[#ef0000]" />
                )}
              </td>
              <td className="px-3 py-2">
                <span className="font-mono text-[#1a1a1a]">{assertion.type}</span>
                {assertion.path && (
                  <span className="font-mono text-[#888888] ml-2">{assertion.path}</span>
                )}
              </td>
              <td className="px-3 py-2 font-mono text-[#666666]">
                {assertion.expected !== undefined
                  ? (typeof assertion.expected === 'object'
                      ? JSON.stringify(assertion.expected)
                      : String(assertion.expected))
                  : '—'}
              </td>
              <td className={`px-3 py-2 font-mono ${assertion.passed ? 'text-[#666666]' : 'text-[#ef0000]'}`}>
                {assertion.actual !== undefined
                  ? (typeof assertion.actual === 'object'
                      ? JSON.stringify(assertion.actual)
                      : String(assertion.actual))
                  : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function VariablesTab({ step }: { step: RunStep }) {
  const variables = step.variables_data ?? [];

  if (variables.length === 0) {
    return <p className="text-sm text-[#888888]">No variables associated with this step.</p>;
  }

  return (
    <div className="rounded border border-[#e8e8e8] overflow-hidden">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-[#f8f8f8] border-b border-[#e8e8e8]">
            <th className="px-3 py-2 text-left text-xs font-medium text-[#888888]">Name</th>
            <th className="px-3 py-2 text-left text-xs font-medium text-[#888888]">Value</th>
            <th className="px-3 py-2 text-left text-xs font-medium text-[#888888]">Source</th>
            <th className="px-3 py-2 text-left text-xs font-medium text-[#888888] w-10"></th>
          </tr>
        </thead>
        <tbody>
          {variables.map((variable, idx) => (
            <tr key={idx} className={idx % 2 === 0 ? 'bg-white' : 'bg-[#f8f8f8]'}>
              <td className="px-3 py-2 font-mono text-[#1a1a1a] whitespace-nowrap">
                {variable.name}
              </td>
              <td className="px-3 py-2 font-mono text-[#666666] break-all max-w-md">
                {variable.value.length > 100
                  ? variable.value.substring(0, 100) + '...'
                  : variable.value}
              </td>
              <td className="px-3 py-2 text-[#888888] whitespace-nowrap">
                {variable.source_type && (
                  <span className="font-mono text-xs px-1.5 py-0.5 bg-[#f0f0f0] rounded">
                    {variable.source_type}
                  </span>
                )}
              </td>
              <td className="px-3 py-2">
                <CopyButton text={variable.value} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function CodeTab({ step }: { step: RunStep }) {
  if (!step.step_config) {
    return <p className="text-sm text-[#888888]">No configuration available.</p>;
  }

  const yamlCode = yaml.dump(step.step_config, { indent: 2, lineWidth: -1 });

  return (
    <CodeBlock
      code={yamlCode}
      label="Step Configuration"
      maxHeight="24rem"
    />
  );
}

/** Get tabs for generic plugin */
export function getGenericTabs(step: RunStep) {
  const totalAssertions = step.assertions_passed + step.assertions_failed;
  const totalVariables = step.variables_data?.length ?? 0;
  const tabs = [];

  // Details tab (always show if there's any data)
  if (step.request_data || step.response_data) {
    tabs.push({ id: 'details', label: 'Details' });
  }

  if (totalAssertions > 0) {
    tabs.push({
      id: 'assertions',
      label: 'Assertions',
      badge: `${step.assertions_passed}/${totalAssertions}`,
    });
  }

  if (totalVariables > 0) {
    tabs.push({
      id: 'variables',
      label: 'Variables',
      badge: String(totalVariables),
    });
  }

  if (step.step_config) {
    tabs.push({ id: 'code', label: 'Code' });
  }

  return tabs;
}
