import { ChevronRight, ChevronDown, CheckCircle2, XCircle, Clock, Circle } from 'lucide-react';
import { useState, useEffect } from 'react';
import { CopyButton } from './step-ui';

// ============================================================================
// TYPE DEFINITIONS
// ============================================================================

type StepStatus = 'success' | 'failed' | 'pending' | 'running' | 'skipped';
type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'HEAD' | 'OPTIONS';

interface BaseStepConfig {
  plugin: string;
  [key: string]: any;
}

interface HttpStepConfig extends BaseStepConfig {
  plugin: 'http';
  method: HttpMethod;
  url: string;
  headers?: Record<string, string>;
  body?: any;
  form?: Record<string, string>;
}

interface Assertion {
  type: string;
  field?: string;
  operator?: string;
  expected?: any;
  actual?: any;
  passed?: boolean;
  message?: string;
}

interface SaveRule {
  name: string;
  path: string;
  value?: any; // Resolved value if run context available
}

interface StepExecutionResult {
  status: StepStatus;
  duration?: number;
  response?: {
    status: number;
    statusText: string;
    headers: Record<string, string>;
    body: any;
  };
  logs?: string[];
  artifacts?: {
    type: string;
    url: string;
    name: string;
  }[];
}

export interface TestStep {
  stepNumber: number;
  name: string;
  config: BaseStepConfig;
  assertions?: Assertion[];
  saves?: SaveRule[];
  result?: StepExecutionResult;
}

// ============================================================================
// PLUGIN REGISTRY (Extensibility Interface)
// ============================================================================

interface PluginRenderer {
  // Collapsed one-line summary
  renderSummary: (config: BaseStepConfig) => React.ReactNode;
  // Expanded tabs (return null to hide tab)
  getTabs: (step: TestStep) => TabDefinition[];
}

interface TabDefinition {
  id: string;
  label: string;
  content: React.ReactNode;
  badge?: string | number;
}

// HTTP Plugin Renderer
const httpPluginRenderer: PluginRenderer = {
  renderSummary: (config: BaseStepConfig) => {
    const httpConfig = config as HttpStepConfig;
    const url = new URL(httpConfig.url, 'http://example.com');
    const pathAndQuery = url.pathname + url.search;
    
    return (
      <span className="text-sm text-[#666666] font-mono truncate">
        <span className="text-black mr-2">{httpConfig.method}</span>
        <span className="text-[#999999]">{url.hostname}</span>
        {pathAndQuery}
      </span>
    );
  },
  
  getTabs: (step: TestStep) => {
    const httpConfig = step.config as HttpStepConfig;
    const tabs: TabDefinition[] = [];
    
    // Request tab (always present for HTTP)
    tabs.push({
      id: 'request',
      label: 'Request',
      content: <HttpRequestPanel config={httpConfig} />
    });
    
    // Response tab (only if run result available)
    tabs.push({
      id: 'response',
      label: 'Response',
      badge: step.result?.response?.status.toString(),
      content: step.result?.response ? (
        <HttpResponsePanel response={step.result.response} />
      ) : (
        <EmptyState message="Run this test to see the response" />
      )
    });
    
    // Assertions tab (only if assertions exist)
    if (step.assertions && step.assertions.length > 0) {
      const passedCount = step.assertions.filter(a => a.passed === true).length;
      const badge = step.result 
        ? `${passedCount}/${step.assertions.length}`
        : step.assertions.length.toString();
      
      tabs.push({
        id: 'assertions',
        label: 'Assertions',
        badge,
        content: <AssertionsPanel assertions={step.assertions} />
      });
    }
    
    // Variables tab (only if saves exist)
    if (step.saves && step.saves.length > 0) {
      tabs.push({
        id: 'variables',
        label: 'Variables',
        badge: step.saves.length.toString(),
        content: <VariablesPanel saves={step.saves} />
      });
    }
    
    // Logs tab (only if logs available)
    if (step.result?.logs && step.result.logs.length > 0) {
      tabs.push({
        id: 'logs',
        label: 'Logs',
        content: <LogsPanel logs={step.result.logs} />
      });
    }
    
    return tabs;
  }
};

// Generic/Unknown Plugin Renderer (fallback)
const genericPluginRenderer: PluginRenderer = {
  renderSummary: (_config: BaseStepConfig) => {
    return (
      <span className="text-sm text-[#666666]">
        Plugin configuration
      </span>
    );
  },
  
  getTabs: (step: TestStep) => {
    const tabs: TabDefinition[] = [];
    
    // Details tab (show raw config)
    tabs.push({
      id: 'details',
      label: 'Details',
      content: <DetailsPanel config={step.config} />
    });
    
    // Only show other tabs if data exists
    if (step.assertions && step.assertions.length > 0) {
      tabs.push({
        id: 'assertions',
        label: 'Assertions',
        badge: step.assertions.length.toString(),
        content: <AssertionsPanel assertions={step.assertions} />
      });
    }
    
    if (step.saves && step.saves.length > 0) {
      tabs.push({
        id: 'variables',
        label: 'Variables',
        badge: step.saves.length.toString(),
        content: <VariablesPanel saves={step.saves} />
      });
    }
    
    return tabs;
  }
};

// Plugin Registry
const PLUGIN_REGISTRY: Record<string, PluginRenderer> = {
  'http': httpPluginRenderer,
  // Future: 'supabase': supabasePluginRenderer,
  // Future: 'playwright': playwrightPluginRenderer,
  // Future: 'agent': agentPluginRenderer,
};

function getPluginRenderer(plugin: string): PluginRenderer {
  return PLUGIN_REGISTRY[plugin] || genericPluginRenderer;
}

// ============================================================================
// SUB-COMPONENTS (Tab Content Panels)
// ============================================================================

function HttpRequestPanel({ config }: { config: HttpStepConfig }) {
  return (
    <div className="space-y-4">
      {/* URL */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <label className="text-xs text-[#999999]">URL</label>
          <CopyButton text={config.url} variant="small" />
        </div>
        <code className="block text-sm font-mono bg-[#fafafa] border border-[#e5e5e5] rounded px-3 py-2 break-all">
          {config.url}
        </code>
      </div>
      
      {/* Headers */}
      {config.headers && Object.keys(config.headers).length > 0 && (
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="text-xs text-[#999999]">Headers</label>
            <CopyButton text={JSON.stringify(config.headers, null, 2)} variant="small" />
          </div>
          <div className="border border-[#e5e5e5] rounded overflow-hidden">
            <table className="w-full text-sm">
              <tbody className="divide-y divide-[#e5e5e5]">
                {Object.entries(config.headers).map(([key, value]) => (
                  <tr key={key} className="bg-white">
                    <td className="px-3 py-2 font-mono text-[#666666] w-1/3">{key}</td>
                    <td className="px-3 py-2 font-mono text-black break-all">{value}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
      
      {/* Body */}
      {config.body && (
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="text-xs text-[#999999]">Body</label>
            <CopyButton text={typeof config.body === 'string' ? config.body : JSON.stringify(config.body, null, 2)} variant="small" />
          </div>
          <CodeBlock code={typeof config.body === 'string' ? config.body : JSON.stringify(config.body, null, 2)} language="json" />
        </div>
      )}
      
      {/* Form Data */}
      {config.form && Object.keys(config.form).length > 0 && (
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="text-xs text-[#999999]">Form Data</label>
          </div>
          <div className="border border-[#e5e5e5] rounded overflow-hidden">
            <table className="w-full text-sm">
              <tbody className="divide-y divide-[#e5e5e5]">
                {Object.entries(config.form).map(([key, value]) => (
                  <tr key={key} className="bg-white">
                    <td className="px-3 py-2 font-mono text-[#666666] w-1/3">{key}</td>
                    <td className="px-3 py-2 font-mono text-black">{value}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

function HttpResponsePanel({ response }: { response: NonNullable<StepExecutionResult['response']> }) {
  const isJson = typeof response.body === 'object';
  const bodyString = isJson ? JSON.stringify(response.body, null, 2) : String(response.body);
  
  return (
    <div className="space-y-4">
      {/* Status */}
      <div>
        <label className="text-xs text-[#999999] mb-2 block">Status</label>
        <div className="flex items-center gap-2">
          <span className={`text-sm font-mono px-2 py-1 rounded ${
            response.status >= 200 && response.status < 300 
              ? 'bg-[#4CBB17]/10 text-[#4CBB17]' 
              : response.status >= 400 
              ? 'bg-[#ef0000]/10 text-[#ef0000]'
              : 'bg-[#f6a724]/10 text-[#f6a724]'
          }`}>
            {response.status}
          </span>
          <span className="text-sm text-[#666666]">{response.statusText}</span>
        </div>
      </div>
      
      {/* Headers */}
      {response.headers && Object.keys(response.headers).length > 0 && (
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="text-xs text-[#999999]">Headers</label>
            <CopyButton text={JSON.stringify(response.headers, null, 2)} variant="small" />
          </div>
          <div className="border border-[#e5e5e5] rounded overflow-hidden max-h-[200px] overflow-y-auto">
            <table className="w-full text-sm">
              <tbody className="divide-y divide-[#e5e5e5]">
                {Object.entries(response.headers).map(([key, value]) => (
                  <tr key={key} className="bg-white">
                    <td className="px-3 py-2 font-mono text-[#666666] w-1/3">{key}</td>
                    <td className="px-3 py-2 font-mono text-black break-all">{value}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
      
      {/* Body */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <label className="text-xs text-[#999999]">Body</label>
          <CopyButton text={bodyString} variant="small" />
        </div>
        <CodeBlock code={bodyString} language={isJson ? 'json' : 'text'} />
      </div>
    </div>
  );
}

function AssertionsPanel({ assertions }: { assertions: Assertion[] }) {
  return (
    <div className="space-y-2">
      {assertions.map((assertion, idx) => {
        const hasPassed = assertion.passed === true;
        const hasFailed = assertion.passed === false;
        const isPending = assertion.passed === undefined;
        
        return (
          <div 
            key={idx} 
            className={`border rounded p-3 ${
              hasFailed ? 'border-[#ef0000] bg-[#ef0000]/5' : 'border-[#e5e5e5] bg-white'
            }`}
          >
            <div className="flex items-start gap-2">
              {hasPassed && <CheckCircle2 className="w-4 h-4 text-[#4CBB17] flex-shrink-0 mt-0.5" />}
              {hasFailed && <XCircle className="w-4 h-4 text-[#ef0000] flex-shrink-0 mt-0.5" />}
              {isPending && <Circle className="w-4 h-4 text-[#999999] flex-shrink-0 mt-0.5" />}
              
              <div className="flex-1 min-w-0">
                <p className="text-sm">
                  {assertion.message || `${assertion.field} ${assertion.operator} ${assertion.expected}`}
                </p>
                {hasFailed && assertion.actual !== undefined && (
                  <p className="text-xs text-[#ef0000] mt-1 font-mono">
                    Expected: {JSON.stringify(assertion.expected)} • Got: {JSON.stringify(assertion.actual)}
                  </p>
                )}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function VariablesPanel({ saves }: { saves: SaveRule[] }) {
  return (
    <div className="border border-[#e5e5e5] rounded overflow-hidden">
      <table className="w-full text-sm">
        <thead className="bg-[#fafafa] border-b border-[#e5e5e5]">
          <tr>
            <th className="px-3 py-2 text-left text-xs text-[#999999]">Variable Name</th>
            <th className="px-3 py-2 text-left text-xs text-[#999999]">Path</th>
            <th className="px-3 py-2 text-left text-xs text-[#999999]">Value</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-[#e5e5e5]">
          {saves.map((save, idx) => (
            <tr key={idx} className="bg-white">
              <td className="px-3 py-2 font-mono text-black">{save.name}</td>
              <td className="px-3 py-2 font-mono text-[#666666]">{save.path}</td>
              <td className="px-3 py-2 font-mono text-[#999999]">
                {save.value !== undefined ? JSON.stringify(save.value) : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function LogsPanel({ logs }: { logs: string[] }) {
  return (
    <div className="bg-[#fafafa] border border-[#e5e5e5] rounded p-3 max-h-[300px] overflow-y-auto">
      <pre className="text-xs font-mono text-[#666666] whitespace-pre-wrap">
        {logs.join('\n')}
      </pre>
    </div>
  );
}

function DetailsPanel({ config }: { config: BaseStepConfig }) {
  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <label className="text-xs text-[#999999]">Configuration</label>
        <CopyButton text={JSON.stringify(config, null, 2)} variant="small" />
      </div>
      <CodeBlock code={JSON.stringify(config, null, 2)} language="json" />
    </div>
  );
}

function CodeBlock({ code, language: _language }: { code: string; language?: string }) {
  return (
    <div className="bg-[#fafafa] border border-[#e5e5e5] rounded overflow-hidden">
      <pre className="p-3 text-xs font-mono overflow-x-auto max-h-[400px] overflow-y-auto">
        <code>{code}</code>
      </pre>
    </div>
  );
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="text-center py-8 text-sm text-[#999999]">
      {message}
    </div>
  );
}

// ============================================================================
// MAIN COMPONENT
// ============================================================================

export function TestStepCard({ step }: { step: TestStep }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [activeTab, setActiveTab] = useState<string>('');
  
  const renderer = getPluginRenderer(step.config.plugin);
  const pluginTabs = renderer.getTabs(step);
  
  // Always include Code tab at the end
  const codeTab: TabDefinition = {
    id: 'code',
    label: 'Code',
    content: <CodePanel step={step} />
  };
  
  const allTabs = [...pluginTabs, codeTab];

  // Set default active tab when expanding (using useEffect to avoid render-time state updates)
  useEffect(() => {
    if (isExpanded && !activeTab && allTabs.length > 0) {
      setActiveTab(allTabs[0].id);
    }
  }, [isExpanded, activeTab, allTabs]);

  const status = step.result?.status;
  const hasFailed = status === 'failed';
  const hasPassed = status === 'success';
  const isSkipped = status === 'skipped';
  
  // Count badges for collapsed view
  const assertionsCount = step.assertions?.length || 0;
  const passedAssertions = step.assertions?.filter(a => a.passed === true).length || 0;
  const failedAssertions = step.assertions?.filter(a => a.passed === false).length || 0;
  const savesCount = step.saves?.length || 0;
  
  // Determine border style based on status
  const getBorderClass = () => {
    if (hasFailed) return 'border-l-4 border-l-[#ef0000]';
    if (hasPassed) return 'border-l-4 border-l-[#4CBB17]';
    if (isSkipped) return 'border-l-4 border-l-[#999999]';
    return 'border-[#e5e5e5]';
  };
  
  return (
    <div className={`bg-white border rounded-lg shadow-sm overflow-hidden ${getBorderClass()}`}>
      {/* Collapsed Header */}
      <div 
        onClick={() => setIsExpanded(!isExpanded)}
        className="p-10 cursor-pointer hover:bg-[#fafafa] transition-colors"
      >
        <div className="flex items-center gap-3">
          {/* Status Icon */}
          <StatusIcon status={status} />
          
          {/* Step Number & Name */}
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <span className="text-xs text-[#999999]">Step {step.stepNumber}</span>
              <PluginBadge plugin={step.config.plugin} />
            </div>
            <h4 className="mb-1">{step.name}</h4>
            <div className="flex items-center gap-2">
              {renderer.renderSummary(step.config)}
            </div>
          </div>
          
          {/* Meta Badges */}
          <div className="flex items-center gap-2">
            {assertionsCount > 0 && step.result && (
              <span className={`text-xs px-2 py-1 rounded ${
                failedAssertions > 0 
                  ? 'bg-[#ef0000]/10 text-[#ef0000]'
                  : passedAssertions > 0
                  ? 'bg-[#4CBB17]/10 text-[#4CBB17]'
                  : 'bg-[#fafafa] text-[#999999]'
              }`}>
                {passedAssertions}/{assertionsCount} assertions
              </span>
            )}
            
            {savesCount > 0 && (
              <span className="text-xs px-2 py-1 rounded bg-[#fafafa] text-[#999999]">
                {savesCount} {savesCount === 1 ? 'variable' : 'variables'}
              </span>
            )}
            
            {step.result?.duration && (
              <span className="text-xs text-[#999999]">
                {step.result.duration}ms
              </span>
            )}
          </div>
          
          {/* Expand Icon */}
          {isExpanded ? (
            <ChevronDown className="w-4 h-4 text-[#666666]" />
          ) : (
            <ChevronRight className="w-4 h-4 text-[#666666]" />
          )}
        </div>
      </div>
      
      {/* Expanded Content */}
      {isExpanded && (
        <div className="border-t border-[#e5e5e5]">
          {/* Tab Bar */}
          <div className="flex items-center gap-1 px-4 pt-3 border-b border-[#e5e5e5]">
            {allTabs.map(tab => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`px-3 py-2 text-sm transition-colors relative ${
                  activeTab === tab.id
                    ? 'text-black'
                    : 'text-[#999999] hover:text-[#666666]'
                }`}
              >
                <div className="flex items-center gap-2">
                  {tab.label}
                  {tab.badge && (
                    <span className="text-xs px-1.5 py-0.5 rounded bg-[#fafafa] text-[#666666]">
                      {tab.badge}
                    </span>
                  )}
                </div>
                {activeTab === tab.id && (
                  <div className="absolute bottom-0 left-0 right-0 h-[2px] bg-black" />
                )}
              </button>
            ))}
          </div>
          
          {/* Tab Content */}
          <div className="p-4">
            {allTabs.find(tab => tab.id === activeTab)?.content}
          </div>
        </div>
      )}
    </div>
  );
}

function StatusIcon({ status }: { status?: StepStatus }) {
  if (!status) {
    return <Circle className="w-5 h-5 text-[#e5e5e5] flex-shrink-0" />;
  }
  
  switch (status) {
    case 'success':
      return <CheckCircle2 className="w-5 h-5 text-[#4CBB17] flex-shrink-0" />;
    case 'failed':
      return <XCircle className="w-5 h-5 text-[#ef0000] flex-shrink-0" />;
    case 'running':
      return <Clock className="w-5 h-5 text-[#f6a724] flex-shrink-0 animate-spin" />;
    case 'pending':
    case 'skipped':
      return <Circle className="w-5 h-5 text-[#999999] flex-shrink-0" />;
  }
}

function PluginBadge({ plugin }: { plugin: string }) {
  return (
    <span className="text-xs px-2 py-0.5 rounded bg-black text-white font-mono">
      {plugin}
    </span>
  );
}

function CodePanel({ step }: { step: TestStep }) {
  // Generate YAML-like representation
  const yamlLines: string[] = [];
  
  yamlLines.push(`- name: ${step.name}`);
  yamlLines.push(`  plugin: ${step.config.plugin}`);
  
  // Plugin-specific config
  if (step.config.plugin === 'http') {
    const httpConfig = step.config as HttpStepConfig;
    yamlLines.push(`  method: ${httpConfig.method}`);
    yamlLines.push(`  url: ${httpConfig.url}`);
    
    if (httpConfig.headers && Object.keys(httpConfig.headers).length > 0) {
      yamlLines.push(`  headers:`);
      Object.entries(httpConfig.headers).forEach(([key, value]) => {
        yamlLines.push(`    ${key}: ${value}`);
      });
    }
    
    if (httpConfig.body) {
      yamlLines.push(`  body: ${typeof httpConfig.body === 'string' ? httpConfig.body : JSON.stringify(httpConfig.body)}`);
    }
  } else {
    // Generic config dump
    Object.entries(step.config).forEach(([key, value]) => {
      if (key !== 'plugin') {
        yamlLines.push(`  ${key}: ${typeof value === 'object' ? JSON.stringify(value) : value}`);
      }
    });
  }
  
  // Assertions
  if (step.assertions && step.assertions.length > 0) {
    yamlLines.push(`  assertions:`);
    step.assertions.forEach(assertion => {
      yamlLines.push(`    - ${assertion.message || `${assertion.field} ${assertion.operator} ${assertion.expected}`}`);
    });
  }
  
  // Saves
  if (step.saves && step.saves.length > 0) {
    yamlLines.push(`  saves:`);
    step.saves.forEach(save => {
      yamlLines.push(`    - name: ${save.name}`);
      yamlLines.push(`      path: ${save.path}`);
    });
  }
  
  const yamlCode = yamlLines.join('\n');
  
  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <label className="text-xs text-[#999999]">YAML Configuration</label>
        <CopyButton text={yamlCode} variant="small" />
      </div>
      <CodeBlock code={yamlCode} language="yaml" />
    </div>
  );
}

// ============================================================================
// ADAPTER - Maps old data structure to new component interface
// ============================================================================

interface OldStepFormat {
  id: string;
  name: string;
  plugin: string;
  method: string;
  url: string;
  status?: 'success' | 'failed' | 'pending' | 'running' | 'skipped';
  duration?: string;
  assertions: (string | { text: string; passed: boolean | null })[];
  saves: string[];
  details?: {
    request: {
      headers: Record<string, string | undefined>;
      body: unknown;
    };
    response: {
      headers: Record<string, string | undefined>;
      body: unknown;
      statusCode: number;
      latency: number;
    };
  };
}

// Helper to filter out undefined values from headers
function filterHeaders(headers?: Record<string, string | undefined>): Record<string, string> | undefined {
  if (!headers) return undefined;
  const filtered: Record<string, string> = {};
  for (const [key, value] of Object.entries(headers)) {
    if (value !== undefined) {
      filtered[key] = value;
    }
  }
  return Object.keys(filtered).length > 0 ? filtered : undefined;
}

function convertOldStepToNew(oldStep: OldStepFormat, stepNumber: number): TestStep {
  // Convert assertions
  const assertions: Assertion[] = oldStep.assertions.map(assertion => {
    if (typeof assertion === 'string') {
      return {
        type: 'custom',
        message: assertion
      };
    } else {
      return {
        type: 'custom',
        message: assertion.text,
        passed: assertion.passed === null ? undefined : assertion.passed
      };
    }
  });

  // Convert saves
  const saves: SaveRule[] = oldStep.saves.map(saveName => ({
    name: saveName,
    path: 'response.body' // Default path since old format doesn't specify
  }));

  // Build config
  const config: HttpStepConfig = {
    plugin: 'http',
    method: oldStep.method as HttpMethod,
    url: oldStep.url,
    headers: filterHeaders(oldStep.details?.request.headers),
    body: oldStep.details?.request.body
  };

  // Build result if details exist
  let result: StepExecutionResult | undefined;
  if (oldStep.details && oldStep.status) {
    result = {
      status: oldStep.status as StepStatus,
      duration: oldStep.duration ? parseInt(oldStep.duration) : undefined,
      response: {
        status: oldStep.details.response.statusCode,
        statusText: oldStep.details.response.statusCode >= 200 && oldStep.details.response.statusCode < 300 
          ? 'OK' 
          : oldStep.details.response.statusCode >= 400 
          ? 'Error' 
          : 'Unknown',
        headers: filterHeaders(oldStep.details.response.headers) || {},
        body: oldStep.details.response.body
      }
    };
  }

  return {
    stepNumber,
    name: oldStep.name,
    config,
    assertions: assertions.length > 0 ? assertions : undefined,
    saves: saves.length > 0 ? saves : undefined,
    result
  };
}

// Wrapper component for backward compatibility
export function TestStepCardAdapter({ 
  step, 
  stepNumber,
  isExpanded,
  onToggle,
  children
}: { 
  step: OldStepFormat; 
  stepNumber: number;
  isExpanded?: boolean;
  onToggle?: (stepId: string) => void;
  children?: React.ReactNode;
}) {
  const newStep = convertOldStepToNew(step, stepNumber);
  
  // If children are provided, we need to handle the old expand/collapse pattern
  if (children) {
    return (
      <div>
        <div onClick={() => onToggle?.(step.id)}>
          <TestStepCard step={newStep} />
        </div>
        {isExpanded && (
          <div className="px-4 pb-4">
            {children}
          </div>
        )}
      </div>
    );
  }
  
  return <TestStepCard step={newStep} />;
}

// ============================================================================