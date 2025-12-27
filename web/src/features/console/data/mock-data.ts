// Single Source of Truth for all mock data

export type Status = 'success' | 'failed' | 'pending';

export interface Project {
  id: string;
  name: string;
  repoUrl: string;
  pathScope: string;
  defaultBranch: string;
  lastUpdated: string;
}

export interface Suite {
  id: string;
  name: string;
  projectId: string;
  path: string;
  description: string;
  lastRunStatus: Status;
  recentActivity: readonly Status[];
  metrics: {
    speed: string;
    reliability: string;
    runsPerDay: string;
  };
}

export interface Test {
  id: string;
  name: string;
  suiteId: string;
  lastStatus: Status;
  lastScheduledRun: string;
  nextRun: string;
  plugins: string[];
  tags: string[];
  recentResults: readonly Status[];
  schedules: Array<{ env: string; cron: string }>;
  successRate: string;
}

export interface Run {
  id: string;
  suiteId: string;
  status: Status;
  startedAt: string;
  duration: string;
  triggeredBy: string;
  env: string;
  branch: string;
  commit: string;
  passed: number;
  failed: number;
  total: number;
}

export interface Environment {
  id: string;
  name: string;
  type: 'production' | 'staging' | 'development';
  url: string;
  lastDeployed: string;
  status: 'active' | 'inactive';
}

export interface AccessKey {
  id: string;
  name: string;
  projectId: string;
  createdAt: string;
  lastUsed: string;
  permissions: string[];
}

// Projects
export const projects: Project[] = [
  {
    id: 'project-1',
    name: 'Backend',
    repoUrl: 'https://github.com/acme/backend',
    pathScope: '.rocketship/',
    defaultBranch: 'main',
    lastUpdated: '2 hours ago',
  },
  {
    id: 'project-2',
    name: 'Frontend',
    repoUrl: 'https://github.com/acme/frontend',
    pathScope: '.rocketship/',
    defaultBranch: 'main',
    lastUpdated: '1 hour ago',
  },
];

// Suites
export const suites: Suite[] = [
  {
    id: 'suite-1',
    name: 'API regression suite',
    projectId: 'project-1',
    path: '.rocketship/api-regression.yaml',
    description: 'Core API endpoints and authentication flows',
    lastRunStatus: 'success',
    recentActivity: ['success', 'success', 'success', 'failed', 'success', 'success', 'success', 'success'],
    metrics: {
      speed: '4m 32s',
      reliability: '94%',
      runsPerDay: '12',
    },
  },
  {
    id: 'suite-2',
    name: 'E2E checkout flow',
    projectId: 'project-2',
    path: '.rocketship/checkout-flow.yaml',
    description: 'Complete user checkout journey',
    lastRunStatus: 'success',
    recentActivity: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    metrics: {
      speed: '8m 15s',
      reliability: '100%',
      runsPerDay: '6',
    },
  },
  {
    id: 'suite-3',
    name: 'Payment processing',
    projectId: 'project-1',
    path: '.rocketship/payment-processing.yaml',
    description: 'Stripe integration and payment flows',
    lastRunStatus: 'failed',
    recentActivity: ['success', 'failed', 'failed', 'success', 'success', 'failed', 'success', 'success'],
    metrics: {
      speed: '6m 18s',
      reliability: '75%',
      runsPerDay: '8',
    },
  },
  {
    id: 'suite-4',
    name: 'Auth flows',
    projectId: 'project-1',
    path: '.rocketship/auth-flows.yaml',
    description: 'OAuth and SSO authentication tests',
    lastRunStatus: 'success',
    recentActivity: ['success', 'success', 'failed', 'success', 'success', 'success', 'success', 'success'],
    metrics: {
      speed: '5m 12s',
      reliability: '88%',
      runsPerDay: '10',
    },
  },
  {
    id: 'suite-5',
    name: 'Database migrations',
    projectId: 'project-1',
    path: '.rocketship/database-migrations.yaml',
    description: 'Schema migrations and data integrity checks',
    lastRunStatus: 'success',
    recentActivity: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    metrics: {
      speed: '3m 45s',
      reliability: '100%',
      runsPerDay: '4',
    },
  },
  {
    id: 'suite-6',
    name: 'User dashboard',
    projectId: 'project-2',
    path: '.rocketship/user-dashboard.yaml',
    description: 'User dashboard functionality and components',
    lastRunStatus: 'success',
    recentActivity: ['success', 'success', 'success', 'success', 'failed', 'success', 'success', 'success'],
    metrics: {
      speed: '12m 30s',
      reliability: '92%',
      runsPerDay: '5',
    },
  },
  {
    id: 'suite-7',
    name: 'Webhook handlers',
    projectId: 'project-1',
    path: '.rocketship/webhook-handlers.yaml',
    description: 'External webhook processing and validation',
    lastRunStatus: 'failed',
    recentActivity: ['success', 'success', 'failed', 'success', 'success', 'success', 'failed', 'success'],
    metrics: {
      speed: '1m 50s',
      reliability: '82%',
      runsPerDay: '20',
    },
  },
  {
    id: 'suite-8',
    name: 'Admin panel',
    projectId: 'project-2',
    path: '.rocketship/admin-panel.yaml',
    description: 'Admin panel functionality and permissions',
    lastRunStatus: 'success',
    recentActivity: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    metrics: {
      speed: '6m 55s',
      reliability: '93%',
      runsPerDay: '4',
    },
  },
];

// Tests
export const tests: Test[] = [
  {
    id: 'test-1',
    name: 'Auth flow with OAuth',
    suiteId: 'suite-4',
    lastStatus: 'success',
    lastScheduledRun: '2 hours ago',
    nextRun: 'in 58 minutes',
    plugins: ['Playwright', 'HTTP'],
    tags: ['auth', 'critical'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '0 * * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-2',
    name: 'Payment processing',
    suiteId: 'suite-3',
    lastStatus: 'failed',
    lastScheduledRun: '15 minutes ago',
    nextRun: 'in 15 minutes',
    plugins: ['Playwright', 'HTTP', 'Supabase'],
    tags: ['payment', 'critical'],
    recentResults: ['success', 'success', 'failed', 'failed', 'success', 'success', 'success', 'success', 'success', 'failed', 'success', 'success', 'success', 'success', 'success', 'success', 'failed', 'success', 'success', 'success'],
    schedules: [
      { env: 'staging', cron: '*/30 * * * *' },
      { env: 'production', cron: '0 0 * * *' },
    ],
    successRate: '85%',
  },
  {
    id: 'test-3',
    name: 'E2E checkout flow',
    suiteId: 'suite-2',
    lastStatus: 'success',
    lastScheduledRun: '1 hour ago',
    nextRun: 'Not scheduled',
    plugins: ['Playwright', 'Delay'],
    tags: ['e2e', 'checkout'],
    recentResults: ['success', 'success', 'success', 'failed', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [],
    successRate: '95%',
  },
  {
    id: 'test-4',
    name: 'Agent conversation flow',
    suiteId: 'suite-1',
    lastStatus: 'success',
    lastScheduledRun: '3 hours ago',
    nextRun: 'in 3 hours',
    plugins: ['Agent', 'HTTP', 'Log'],
    tags: ['ai', 'agent'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'staging', cron: '0 */6 * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-5',
    name: 'Database migration validation',
    suiteId: 'suite-5',
    lastStatus: 'success',
    lastScheduledRun: '4 hours ago',
    nextRun: 'in 2 hours',
    plugins: ['Supabase', 'SQL'],
    tags: ['database', 'critical'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '0 */6 * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-6',
    name: 'API endpoint health check',
    suiteId: 'suite-1',
    lastStatus: 'success',
    lastScheduledRun: '30 minutes ago',
    nextRun: 'in 30 minutes',
    plugins: ['HTTP', 'Script'],
    tags: ['api', 'monitoring'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '*/15 * * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-7',
    name: 'Batch data processing',
    suiteId: 'suite-5',
    lastStatus: 'success',
    lastScheduledRun: '5 hours ago',
    nextRun: 'in 1 hour',
    plugins: ['Script', 'SQL', 'Delay'],
    tags: ['batch', 'data'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'failed', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'staging', cron: '0 0 * * *' }],
    successRate: '98%',
  },
  {
    id: 'test-8',
    name: 'Email notification flow',
    suiteId: 'suite-7',
    lastStatus: 'success',
    lastScheduledRun: '1 hour ago',
    nextRun: 'in 5 hours',
    plugins: ['HTTP', 'Log'],
    tags: ['notifications', 'email'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '0 */6 * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-9',
    name: 'Search indexing validation',
    suiteId: 'suite-1',
    lastStatus: 'success',
    lastScheduledRun: '2 hours ago',
    nextRun: 'in 4 hours',
    plugins: ['HTTP', 'Script'],
    tags: ['search', 'indexing'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '0 */6 * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-10',
    name: 'Cache invalidation check',
    suiteId: 'suite-1',
    lastStatus: 'failed',
    lastScheduledRun: '30 minutes ago',
    nextRun: 'in 30 minutes',
    plugins: ['HTTP', 'SQL'],
    tags: ['cache', 'performance'],
    recentResults: ['success', 'failed', 'success', 'success', 'success', 'success', 'failed', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'staging', cron: '*/15 * * * *' }],
    successRate: '90%',
  },
  {
    id: 'test-11',
    name: 'User session management',
    suiteId: 'suite-4',
    lastStatus: 'success',
    lastScheduledRun: '1 hour ago',
    nextRun: 'in 1 hour',
    plugins: ['Playwright', 'Supabase'],
    tags: ['auth', 'session'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '0 * * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-12',
    name: 'Third-party webhook processing',
    suiteId: 'suite-7',
    lastStatus: 'success',
    lastScheduledRun: '45 minutes ago',
    nextRun: 'in 15 minutes',
    plugins: ['HTTP', 'Log', 'Delay'],
    tags: ['webhooks', 'integration'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '*/30 * * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-13',
    name: 'File upload and validation',
    suiteId: 'suite-1',
    lastStatus: 'success',
    lastScheduledRun: '3 hours ago',
    nextRun: 'in 3 hours',
    plugins: ['Playwright', 'HTTP'],
    tags: ['upload', 'files'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'staging', cron: '0 */6 * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-14',
    name: 'Rate limiting enforcement',
    suiteId: 'suite-1',
    lastStatus: 'success',
    lastScheduledRun: '2 hours ago',
    nextRun: 'in 4 hours',
    plugins: ['HTTP', 'Script'],
    tags: ['security', 'rate-limit'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '0 */6 * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-15',
    name: 'Mobile API compatibility',
    suiteId: 'suite-1',
    lastStatus: 'success',
    lastScheduledRun: '1 hour ago',
    nextRun: 'in 5 hours',
    plugins: ['HTTP', 'Agent'],
    tags: ['mobile', 'api'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '0 */6 * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-16',
    name: 'Analytics data pipeline',
    suiteId: 'suite-5',
    lastStatus: 'success',
    lastScheduledRun: '6 hours ago',
    nextRun: 'in 6 hours',
    plugins: ['SQL', 'Script', 'Delay'],
    tags: ['analytics', 'data'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '0 */12 * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-17',
    name: 'Super long test name that should definitely be truncated with an ellipsis to prevent wrapping',
    suiteId: 'suite-6',
    lastStatus: 'success',
    lastScheduledRun: '1 hour ago',
    nextRun: 'in 2 hours',
    plugins: ['HTTP', 'Playwright'],
    tags: ['test', 'long'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'staging', cron: '0 * * * *' }],
    successRate: '100%',
  },
  {
    id: 'test-18',
    name: 'Another incredibly verbose and detailed test name example',
    suiteId: 'suite-8',
    lastStatus: 'failed',
    lastScheduledRun: '30 minutes ago',
    nextRun: 'in 90 minutes',
    plugins: ['Agent', 'SQL', 'Log'],
    tags: ['verbose', 'test'],
    recentResults: ['failed', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '*/30 * * * *' }],
    successRate: '95%',
  },
  {
    id: 'test-19',
    name: 'E2E comprehensive multi-step workflow validation with extensive retry logic and error handling',
    suiteId: 'suite-2',
    lastStatus: 'success',
    lastScheduledRun: '2 hours ago',
    nextRun: 'in 4 hours',
    plugins: ['Playwright', 'HTTP', 'Supabase', 'Delay'],
    tags: ['enterprise', 'critical'],
    recentResults: ['success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success', 'success'],
    schedules: [{ env: 'production', cron: '0 */6 * * *' }],
    successRate: '100%',
  },
];

// Runs
export const runs: Run[] = [
  {
    id: 'run-1',
    suiteId: 'suite-1',
    status: 'success',
    startedAt: '2 hours ago',
    duration: '4m 32s',
    triggeredBy: 'schedule',
    env: 'production',
    branch: 'main',
    commit: 'a3b4c5d',
    passed: 12,
    failed: 0,
    total: 12,
  },
  {
    id: 'run-2',
    suiteId: 'suite-2',
    status: 'success',
    startedAt: '1 hour ago',
    duration: '8m 15s',
    triggeredBy: 'manual',
    env: 'staging',
    branch: 'develop',
    commit: 'f7e8d9c',
    passed: 8,
    failed: 0,
    total: 8,
  },
  {
    id: 'run-3',
    suiteId: 'suite-3',
    status: 'failed',
    startedAt: '15 minutes ago',
    duration: '6m 18s',
    triggeredBy: 'schedule',
    env: 'production',
    branch: 'main',
    commit: 'b2c3d4e',
    passed: 6,
    failed: 2,
    total: 8,
  },
  {
    id: 'run-4',
    suiteId: 'suite-4',
    status: 'success',
    startedAt: '3 hours ago',
    duration: '5m 12s',
    triggeredBy: 'webhook',
    env: 'staging',
    branch: 'feature/oauth',
    commit: 'c4d5e6f',
    passed: 10,
    failed: 0,
    total: 10,
  },
  {
    id: 'run-5',
    suiteId: 'suite-5',
    status: 'success',
    startedAt: '4 hours ago',
    duration: '3m 45s',
    triggeredBy: 'schedule',
    env: 'production',
    branch: 'main',
    commit: 'd5e6f7a',
    passed: 5,
    failed: 0,
    total: 5,
  },
];

// Environments
export const environments: Environment[] = [
  {
    id: 'env-1',
    name: 'production',
    type: 'production',
    url: 'https://api.acme.com',
    lastDeployed: '2 hours ago',
    status: 'active',
  },
  {
    id: 'env-2',
    name: 'staging',
    type: 'staging',
    url: 'https://staging-api.acme.com',
    lastDeployed: '1 hour ago',
    status: 'active',
  },
  {
    id: 'env-3',
    name: 'development',
    type: 'development',
    url: 'https://dev-api.acme.com',
    lastDeployed: '30 minutes ago',
    status: 'active',
  },
];

// Access Keys
export const accessKeys: AccessKey[] = [
  {
    id: 'key-1',
    name: 'CI/CD Pipeline',
    projectId: 'project-1',
    createdAt: '2024-01-15',
    lastUsed: '2 hours ago',
    permissions: ['read:tests', 'write:runs'],
  },
  {
    id: 'key-2',
    name: 'Development Team',
    projectId: 'project-1',
    createdAt: '2024-02-01',
    lastUsed: '1 day ago',
    permissions: ['read:tests', 'read:runs'],
  },
  {
    id: 'key-3',
    name: 'Monitoring Service',
    projectId: 'project-2',
    createdAt: '2024-03-10',
    lastUsed: '5 minutes ago',
    permissions: ['read:tests', 'read:runs', 'read:metrics'],
  },
];

// Available plugins
export const availablePlugins = ['HTTP', 'Playwright', 'Supabase', 'Agent', 'SQL', 'Script', 'Delay', 'Log'];

// Helper functions to get related data
export function getProjectById(projectId: string): Project | undefined {
  return projects.find(p => p.id === projectId);
}

export function getSuiteById(suiteId: string): Suite | undefined {
  return suites.find(s => s.id === suiteId);
}

export function getTestById(testId: string): Test | undefined {
  return tests.find(t => t.id === testId);
}

export function getSuitesByProjectId(projectId: string): Suite[] {
  return suites.filter(s => s.projectId === projectId);
}

export function getTestsBySuiteId(suiteId: string): Test[] {
  return tests.filter(t => t.suiteId === suiteId);
}

export function getRunsBySuiteId(suiteId: string): Run[] {
  return runs.filter(r => r.suiteId === suiteId);
}

export function getTestCountByProject(): Record<string, number> {
  const counts: Record<string, number> = {};
  projects.forEach(project => {
    const projectSuites = getSuitesByProjectId(project.id);
    const testCount = projectSuites.reduce((acc, suite) => {
      return acc + getTestsBySuiteId(suite.id).length;
    }, 0);
    counts[project.id] = testCount;
  });
  return counts;
}

// Get project name for a suite
export function getProjectNameForSuite(suiteId: string): string {
  const suite = getSuiteById(suiteId);
  if (!suite) return 'Unknown';
  const project = getProjectById(suite.projectId);
  return project?.name || 'Unknown';
}

// Get suite name for a test
export function getSuiteNameForTest(testId: string): string {
  const test = getTestById(testId);
  if (!test) return 'Unknown';
  const suite = getSuiteById(test.suiteId);
  return suite?.name || 'Unknown';
}