import yaml from 'js-yaml';
import { Check, X } from 'lucide-react';
import type { RunStep } from '../../../hooks/use-console-queries';
import { CopyButton, KeyValueTable, headersToRows, CodeBlock, InlineCode } from '../../step-ui';
import { tryFormatJSON, getUrlParts } from '../../../lib/format';

interface HttpRendererProps {
  step: RunStep;
  activeTab: string;
}

/** HTTP plugin-specific renderer for expanded content */
export function HttpRenderer({ step, activeTab }: HttpRendererProps) {
  return (
    <div className="py-4 px-4 ml-8">
      {activeTab === 'request' && step.request_data && (
        <RequestTab step={step} />
      )}
      {activeTab === 'response' && step.response_data && (
        <ResponseTab step={step} />
      )}
      {activeTab === 'assertions' && (
        <AssertionsTab step={step} />
      )}
      {activeTab === 'variables' && (
        <VariablesTab step={step} />
      )}
      {activeTab === 'code' && step.step_config && (
        <CodeTab step={step} />
      )}
    </div>
  );
}

function RequestTab({ step }: { step: RunStep }) {
  const { request_data } = step;
  if (!request_data) return null;

  const headerRows = headersToRows(request_data.headers, { maskSensitive: true });

  return (
    <div className="space-y-4">
      <InlineCode code={request_data.url} label="URL" />

      {headerRows.length > 0 && (
        <KeyValueTable
          rows={headerRows}
          label="Headers"
          copyAllText={JSON.stringify(request_data.headers, null, 2)}
        />
      )}

      {request_data.body && (
        <CodeBlock
          code={tryFormatJSON(request_data.body)}
          label="Body"
        />
      )}
    </div>
  );
}

function ResponseTab({ step }: { step: RunStep }) {
  const { response_data } = step;
  if (!response_data) return null;

  const headerRows = headersToRows(response_data.headers);

  return (
    <div className="space-y-4">
      {headerRows.length > 0 && (
        <KeyValueTable
          rows={headerRows}
          label="Headers"
          copyAllText={JSON.stringify(response_data.headers, null, 2)}
        />
      )}

      {response_data.body && (
        <CodeBlock
          code={tryFormatJSON(response_data.body)}
          label="Body"
          maxHeight="24rem"
        />
      )}
    </div>
  );
}

function AssertionsTab({ step }: { step: RunStep }) {
  if (!step.assertions_data || step.assertions_data.length === 0) {
    return <p className="text-sm text-[#888888]">No detailed assertion data available.</p>;
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
                {assertion.name && (
                  <span className="text-[#888888] ml-2">{assertion.name}</span>
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
  const configVariables = step.variables_data?.filter(v => v.source_type === 'config') ?? [];
  const runtimeVariables = step.variables_data?.filter(v => v.source_type === 'runtime') ?? [];
  const savedVariables = step.variables_data?.filter(v =>
    v.source_type !== 'config' && v.source_type !== 'runtime'
  ) ?? [];

  return (
    <div className="space-y-6">
      {/* Available Variables Section */}
      <div>
        <h4 className="text-sm font-medium text-[#1a1a1a] mb-2">
          Available Variables ({configVariables.length + runtimeVariables.length})
        </h4>
        <p className="text-xs text-[#888888] mb-3">
          Variables available for template substitution in this step
        </p>

        {/* Config Variables */}
        <div className="mb-4">
          <h5 className="text-xs font-medium text-[#666666] mb-2 flex items-center gap-2">
            <span className="font-mono px-1.5 py-0.5 bg-[#f0f0f0] rounded text-[10px]">config</span>
            Config Variables ({configVariables.length})
            <span className="text-[#a1a1aa] font-normal">— accessed via {"{{ .vars.* }}"}</span>
          </h5>
          {configVariables.length > 0 ? (
            <VariableTable variables={configVariables} showSource={false} />
          ) : (
            <div className="rounded border border-[#e8e8e8] bg-[#f8f8f8] px-4 py-2">
              <p className="text-xs text-[#888888]">No config variables defined in this suite.</p>
            </div>
          )}
        </div>

        {/* Runtime Variables */}
        <div>
          <h5 className="text-xs font-medium text-[#666666] mb-2 flex items-center gap-2">
            <span className="font-mono px-1.5 py-0.5 bg-[#f0f0f0] rounded text-[10px]">runtime</span>
            Runtime Variables ({runtimeVariables.length})
            <span className="text-[#a1a1aa] font-normal">— accessed via {"{{ var_name }}"}</span>
          </h5>
          {runtimeVariables.length > 0 ? (
            <VariableTable variables={runtimeVariables} showSource={false} />
          ) : (
            <div className="rounded border border-[#e8e8e8] bg-[#f8f8f8] px-4 py-2">
              <p className="text-xs text-[#888888]">No runtime variables saved by previous steps.</p>
            </div>
          )}
        </div>
      </div>

      {/* Saved Variables Section */}
      <div>
        <h4 className="text-sm font-medium text-[#1a1a1a] mb-2">
          Saved Variables ({savedVariables.length})
        </h4>
        <p className="text-xs text-[#888888] mb-3">
          Variables saved by this step for use in subsequent steps
        </p>
        {savedVariables.length > 0 ? (
          <VariableTable variables={savedVariables} showSource={true} />
        ) : (
          <div className="rounded border border-[#e8e8e8] bg-[#f8f8f8] px-4 py-3">
            <p className="text-sm text-[#888888]">This step does not save any variables.</p>
          </div>
        )}
      </div>
    </div>
  );
}

function VariableTable({
  variables,
  showSource
}: {
  variables: NonNullable<RunStep['variables_data']>;
  showSource: boolean;
}) {
  return (
    <div className="rounded border border-[#e8e8e8] overflow-hidden">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-[#f8f8f8] border-b border-[#e8e8e8]">
            <th className="px-3 py-2 text-left text-xs font-medium text-[#888888]">Name</th>
            <th className="px-3 py-2 text-left text-xs font-medium text-[#888888]">Value</th>
            {showSource && (
              <th className="px-3 py-2 text-left text-xs font-medium text-[#888888]">Source</th>
            )}
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
              {showSource && (
                <td className="px-3 py-2 text-[#888888] whitespace-nowrap">
                  {variable.source_type && (
                    <span className="font-mono text-xs px-1.5 py-0.5 bg-[#f0f0f0] rounded">
                      {variable.source_type}
                    </span>
                  )}
                  {variable.source && (
                    <span className="font-mono text-xs text-[#a1a1aa] ml-2">
                      {variable.source}
                    </span>
                  )}
                </td>
              )}
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
  if (!step.step_config) return null;

  const yamlCode = yaml.dump(step.step_config, { indent: 2, lineWidth: -1 });

  return (
    <CodeBlock
      code={yamlCode}
      label="Step Configuration"
      maxHeight="24rem"
    />
  );
}

/** Get URL display parts for collapsed view */
export function getHttpSummary(step: RunStep): { method: string; domain: string; path: string } | null {
  if (step.plugin !== 'http' || !step.request_data?.url) {
    return null;
  }

  const parts = getUrlParts(step.request_data.url);
  return {
    method: step.request_data.method || '',
    domain: parts.domain,
    path: parts.path,
  };
}

/** Get tabs for HTTP plugin */
export function getHttpTabs(step: RunStep) {
  const totalAssertions = step.assertions_passed + step.assertions_failed;
  const tabs = [];

  if (step.request_data) {
    tabs.push({ id: 'request', label: 'Request' });
  }

  if (step.response_data) {
    tabs.push({ id: 'response', label: 'Response' });
  }

  if (totalAssertions > 0) {
    tabs.push({
      id: 'assertions',
      label: 'Assertions',
      badge: `${step.assertions_passed}/${totalAssertions}`,
    });
  }

  tabs.push({ id: 'variables', label: 'Variables' });

  if (step.step_config) {
    tabs.push({ id: 'code', label: 'Code' });
  }

  return tabs;
}
