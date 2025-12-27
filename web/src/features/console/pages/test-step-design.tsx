import { TestStepCard, type TestStep } from '../components/test-step-card';

// ============================================================================
// MOCK DATA - Demonstrates different states and plugin types
// ============================================================================

const mockSteps: TestStep[] = [
  // HTTP Step - Success with full response
  {
    stepNumber: 1,
    name: 'Fetch user profile',
    config: {
      plugin: 'http',
      method: 'GET',
      url: 'https://api.example.com/v1/users/12345',
      headers: {
        'Authorization': 'Bearer ${API_TOKEN}',
        'Accept': 'application/json',
        'User-Agent': 'RocketshipCloud/1.0'
      }
    },
    assertions: [
      {
        type: 'status',
        operator: 'equals',
        expected: 200,
        actual: 200,
        passed: true,
        message: 'Response status is 200'
      },
      {
        type: 'json',
        field: 'data.email',
        operator: 'contains',
        expected: '@example.com',
        actual: 'john@example.com',
        passed: true,
        message: 'Email contains @example.com'
      },
      {
        type: 'json',
        field: 'data.role',
        operator: 'equals',
        expected: 'admin',
        actual: 'admin',
        passed: true,
        message: 'User has admin role'
      }
    ],
    saves: [
      {
        name: 'userId',
        path: 'data.id',
        value: '12345'
      },
      {
        name: 'userEmail',
        path: 'data.email',
        value: 'john@example.com'
      }
    ],
    result: {
      status: 'success',
      duration: 342,
      response: {
        status: 200,
        statusText: 'OK',
        headers: {
          'content-type': 'application/json',
          'x-ratelimit-remaining': '98',
          'cache-control': 'no-cache'
        },
        body: {
          data: {
            id: '12345',
            email: 'john@example.com',
            name: 'John Doe',
            role: 'admin',
            created_at: '2024-01-15T10:30:00Z'
          }
        }
      },
      logs: [
        '[INFO] Resolving DNS for api.example.com',
        '[INFO] Connecting to 104.26.10.123:443',
        '[INFO] TLS handshake completed',
        '[INFO] Sent GET request',
        '[INFO] Received 200 OK response (342ms)'
      ]
    }
  },
  
  // HTTP Step - Failed with assertion errors
  {
    stepNumber: 2,
    name: 'Create new project',
    config: {
      plugin: 'http',
      method: 'POST',
      url: 'https://api.example.com/v1/projects',
      headers: {
        'Authorization': 'Bearer ${API_TOKEN}',
        'Content-Type': 'application/json'
      },
      body: {
        name: 'My New Project',
        description: 'Test project for demo',
        settings: {
          public: false,
          region: 'us-west-2'
        }
      }
    },
    assertions: [
      {
        type: 'status',
        operator: 'equals',
        expected: 201,
        actual: 200,
        passed: false,
        message: 'Response status is 201'
      },
      {
        type: 'json',
        field: 'data.id',
        operator: 'exists',
        expected: true,
        actual: true,
        passed: true,
        message: 'Project ID exists in response'
      },
      {
        type: 'json',
        field: 'data.name',
        operator: 'equals',
        expected: 'My New Project',
        actual: 'My New Project',
        passed: true,
        message: 'Project name matches'
      }
    ],
    saves: [
      {
        name: 'projectId',
        path: 'data.id',
        value: 'proj_9876'
      }
    ],
    result: {
      status: 'failed',
      duration: 523,
      response: {
        status: 200,
        statusText: 'OK',
        headers: {
          'content-type': 'application/json'
        },
        body: {
          data: {
            id: 'proj_9876',
            name: 'My New Project',
            description: 'Test project for demo'
          }
        }
      },
      logs: [
        '[INFO] Sent POST request',
        '[WARN] Expected 201 but received 200',
        '[ERROR] Assertion failed: Response status is 201'
      ]
    }
  },
  
  // HTTP Step - Not yet run (no result)
  {
    stepNumber: 3,
    name: 'Delete project',
    config: {
      plugin: 'http',
      method: 'DELETE',
      url: 'https://api.example.com/v1/projects/${projectId}',
      headers: {
        'Authorization': 'Bearer ${API_TOKEN}'
      }
    },
    assertions: [
      {
        type: 'status',
        operator: 'equals',
        expected: 204,
        message: 'Response status is 204 No Content'
      }
    ]
  },
  
  // HTTP Step - Form data submission
  {
    stepNumber: 4,
    name: 'Upload avatar image',
    config: {
      plugin: 'http',
      method: 'POST',
      url: 'https://api.example.com/v1/users/12345/avatar',
      headers: {
        'Authorization': 'Bearer ${API_TOKEN}'
      },
      form: {
        'file': '@avatar.png',
        'visibility': 'public',
        'format': 'png'
      }
    },
    assertions: [
      {
        type: 'status',
        operator: 'equals',
        expected: 200,
        message: 'Upload successful'
      }
    ]
  },
  
  // Generic plugin (not HTTP) - Shows fallback renderer
  {
    stepNumber: 5,
    name: 'Query active users',
    config: {
      plugin: 'supabase',
      table: 'users',
      operation: 'select',
      filters: {
        status: 'active',
        created_at: { gte: '2024-01-01' }
      },
      columns: ['id', 'email', 'name', 'created_at']
    },
    assertions: [
      {
        type: 'count',
        operator: 'greaterThan',
        expected: 0,
        message: 'Returns at least one user'
      }
    ],
    saves: [
      {
        name: 'activeUserCount',
        path: 'count'
      }
    ]
  },
  
  // Generic plugin - Another example
  {
    stepNumber: 6,
    name: 'Wait for processing',
    config: {
      plugin: 'delay',
      duration: 5000,
      unit: 'milliseconds'
    }
  }
];

// ============================================================================
// DEMO PAGE - Interactive Wireframes/Mockups
// ============================================================================

export function TestStepDesign() {
  return (
    <div className="min-h-screen bg-[#fafafa]">
      {/* Header */}
      <div className="bg-white border-b border-[#e5e5e5] px-8 py-6">
        <div className="max-w-5xl mx-auto">
          <h1 className="mb-2">Test Step Component Design</h1>
          <p className="text-[#666666]">
            Interactive wireframes and mockups demonstrating the extensible step card component
          </p>
        </div>
      </div>
      
      <div className="p-8">
        <div className="max-w-5xl mx-auto space-y-8">
          
          {/* Section 1: HTTP Steps with Various States */}
          <section>
            <div className="mb-4">
              <h2 className="mb-2">HTTP Plugin - Various States</h2>
              <p className="text-sm text-[#666666]">
                Demonstrates HTTP steps with success, failure, and not-yet-run states. Click to expand and explore tabs.
              </p>
            </div>
            
            <div className="space-y-3">
              {mockSteps.slice(0, 4).map(step => (
                <TestStepCard key={step.stepNumber} step={step} />
              ))}
            </div>
          </section>
          
          {/* Section 2: Generic/Unknown Plugin Fallback */}
          <section>
            <div className="mb-4">
              <h2 className="mb-2">Generic Plugin Renderer (Extensibility)</h2>
              <p className="text-sm text-[#666666]">
                Shows how the component handles unknown plugins with a generic fallback renderer. 
                Future plugins (Supabase, Playwright, Agent) will have custom renderers following the same interface.
              </p>
            </div>
            
            <div className="space-y-3">
              {mockSteps.slice(4).map(step => (
                <TestStepCard key={step.stepNumber} step={step} />
              ))}
            </div>
          </section>
          
          {/* Component Specification */}
          <section className="bg-white border border-[#e5e5e5] rounded-lg p-6">
            <h2 className="mb-4">Component Specification</h2>
            
            <div className="space-y-6 text-sm">
              {/* Collapsed State */}
              <div>
                <h3 className="mb-2">Collapsed State (Summary)</h3>
                <ul className="space-y-1 text-[#666666] list-disc list-inside">
                  <li>Status icon (colored dot/check/x based on execution result)</li>
                  <li>Step number label + plugin badge (small black pill)</li>
                  <li>Step name (heading)</li>
                  <li>Plugin-specific one-line summary (e.g., "GET hostname/path")</li>
                  <li>Optional meta badges: assertion pass/fail count, variables count</li>
                  <li>Duration (if available)</li>
                  <li>Expand/collapse chevron icon</li>
                  <li>Failed steps show red left border accent</li>
                </ul>
              </div>
              
              {/* Expanded State */}
              <div>
                <h3 className="mb-2">Expanded State (Tabs)</h3>
                <ul className="space-y-1 text-[#666666] list-disc list-inside">
                  <li>Header remains visible (same as collapsed)</li>
                  <li>Tab bar with dynamic tabs based on plugin and available data</li>
                  <li>Tab badges show counts or status codes where relevant</li>
                  <li>"Code" tab always present (shows YAML representation)</li>
                  <li>Active tab indicated by bottom border + black text</li>
                </ul>
              </div>
              
              {/* HTTP Plugin Tabs */}
              <div>
                <h3 className="mb-2">HTTP Plugin Tabs (v1)</h3>
                <ol className="space-y-1 text-[#666666] list-decimal list-inside">
                  <li><strong>Request:</strong> URL (with copy), headers table, body (JSON/text), form data table</li>
                  <li><strong>Response:</strong> Status badge, headers table, body (formatted JSON or raw text, with copy). Empty state if not run yet.</li>
                  <li><strong>Assertions:</strong> List of assertions with pass/fail icons. Failed assertions highlighted in red with expected vs actual values.</li>
                  <li><strong>Variables:</strong> Table showing variable name, path, and resolved value (if available).</li>
                  <li><strong>Logs:</strong> Plain text log output (only if available).</li>
                  <li><strong>Code:</strong> YAML representation with copy button.</li>
                </ol>
              </div>
              
              {/* Plugin Extensibility */}
              <div>
                <h3 className="mb-2">Plugin Registry Interface (Extensibility)</h3>
                <div className="bg-[#fafafa] border border-[#e5e5e5] rounded p-4 font-mono text-xs space-y-2">
                  <div className="text-[#666666]">// Plugin renderers follow this interface:</div>
                  <div>interface PluginRenderer {'{'}</div>
                  <div className="pl-4">renderSummary: (config) =&gt; ReactNode;</div>
                  <div className="pl-4">getTabs: (step) =&gt; TabDefinition[];</div>
                  <div>{'}'}</div>
                  <div className="text-[#666666] mt-2">// Register new plugins like:</div>
                  <div>PLUGIN_REGISTRY['playwright'] = playwrightRenderer;</div>
                  <div>PLUGIN_REGISTRY['agent'] = agentRenderer;</div>
                </div>
                <p className="text-[#666666] mt-2">
                  This allows adding new plugin types without modifying the base TestStepCard component. 
                  Each plugin defines its own summary format and tabs.
                </p>
              </div>
              
              {/* Future Plugin Examples */}
              <div>
                <h3 className="mb-2">Future Plugin Tab Examples</h3>
                <ul className="space-y-2 text-[#666666]">
                  <li>
                    <strong>Supabase:</strong> Tabs: Query, Filters, Results, Variables, Code
                  </li>
                  <li>
                    <strong>Playwright/Browser:</strong> Tabs: Script, Screenshots, Video Replay, Assertions, Logs, Code
                  </li>
                  <li>
                    <strong>Agent/LLM:</strong> Tabs: Prompt, Transcript, Response, Tools Used, Logs, Code
                  </li>
                  <li>
                    <strong>SQL:</strong> Tabs: Query, Parameters, Results Table, Variables, Code
                  </li>
                </ul>
              </div>
              
              {/* Visual Style */}
              <div>
                <h3 className="mb-2">Visual Style Rules</h3>
                <ul className="space-y-1 text-[#666666] list-disc list-inside">
                  <li>White card background with subtle border (#e5e5e5)</li>
                  <li>Hover state: light gray background (#fafafa)</li>
                  <li>Status colors: green #4CBB17, yellow #f6a724, red #ef0000</li>
                  <li>Failed steps: red left border (4px) + red-tinted assertion panels</li>
                  <li>Monospace font for code, URLs, headers, paths</li>
                  <li>Copy buttons on all copyable content (subtle, hover to darken)</li>
                  <li>Tables with alternating rows for readability</li>
                  <li>Max heights with scroll on long content (headers, logs, response body)</li>
                </ul>
              </div>
              
              {/* Empty States */}
              <div>
                <h3 className="mb-2">Empty States</h3>
                <ul className="space-y-1 text-[#666666] list-disc list-inside">
                  <li>Response tab (not run): "Run this test to see the response"</li>
                  <li>Logs tab (no logs): "No logs available"</li>
                  <li>Artifacts tab (future): "No artifacts captured"</li>
                  <li>Tabs with no content are hidden from tab bar</li>
                </ul>
              </div>
              
              {/* Layout Rules */}
              <div>
                <h3 className="mb-2">Layout Rules</h3>
                <ul className="space-y-1 text-[#666666] list-disc list-inside">
                  <li>Steps stack vertically with 12px gap between cards</li>
                  <li>Collapsed header: 16px padding, flexbox with status + content + badges + chevron</li>
                  <li>Expanded content: tab bar at top (no padding), content area with 16px padding</li>
                  <li>Tab bar border-bottom, active tab has 2px bottom border in black</li>
                  <li>All content should be responsive and readable on smaller screens</li>
                </ul>
              </div>
            </div>
          </section>
          
          {/* Design Notes */}
          <section className="bg-[#fffbeb] border border-[#f6a724]/30 rounded-lg p-6">
            <h2 className="mb-3">Design Notes</h2>
            <div className="space-y-3 text-sm text-[#666666]">
              <p>
                <strong>Scalability:</strong> The plugin registry pattern allows adding 100+ plugin types without 
                touching the base component. Each plugin defines its own summary format and tab structure.
              </p>
              <p>
                <strong>Runscope Inspiration:</strong> HTTP request/response viewers with clean headers tables, 
                formatted JSON bodies, status code badges, and copy buttons everywhere.
              </p>
              <p>
                <strong>Progressive Disclosure:</strong> Collapsed state shows minimal info for quick scanning. 
                Expanded state reveals full details organized in logical tabs.
              </p>
              <p>
                <strong>Data Flexibility:</strong> Component works with or without execution results. Shows placeholders 
                when data isn't available yet, and enriches display when run data exists.
              </p>
              <p>
                <strong>Future Extensions:</strong> Easy to add Artifacts tab for videos/screenshots, Replay tab for 
                browser recordings, Transcript tab for Agent conversations, etc.
              </p>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

export default TestStepDesign;
