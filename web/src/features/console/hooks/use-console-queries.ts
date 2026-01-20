import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiPut, apiPatch, apiDelete } from '@/lib/api'
import { isLiveRunStatus, isLiveTestStatus, isLiveStepStatus } from '../lib/format'

// Polling intervals (ms) for two-tier polling strategy
// Live polling: fast refresh when tests/runs are in progress
// Idle polling: slower background refresh to discover new runs
const TEST_HEALTH_POLL_LIVE_MS = 5000    // 5s when tests are running
const TEST_HEALTH_POLL_IDLE_MS = 30000   // 30s when idle (to discover new runs)
const SUITE_RUNS_POLL_LIVE_MS = 3000     // 3s when runs are in progress
const SUITE_RUNS_POLL_IDLE_MS = 15000    // 15s when idle (to discover new runs)

// Types matching the API response shapes

export interface LastScan {
  status: string
  created_at: string
  head_sha: string
  error_message: string
  suites_found: number
  tests_found: number
}

export interface ProjectSummary {
  id: string
  name: string
  repo_url: string
  default_branch: string
  path_scope: string[]
  source_ref: string
  suite_count: number
  test_count: number
  last_scan: LastScan | null
}

export interface ProjectDetail extends ProjectSummary {}

export interface SuiteSummary {
  id: string
  name: string
  description?: string
  file_path?: string
  source_ref: string
  test_count: number
  last_run_status?: string
  last_run_at?: string
}

export interface SuiteActivityItem {
  suite_id: string
  name: string
  description?: string
  file_path?: string
  source_ref: string
  test_count: number
  project: {
    id: string
    name: string
    repo_url: string
  }
  last_run: {
    status: string | null
    at: string | null
  }
  // Aggregate metrics from recent runs
  median_duration_ms?: number | null
  reliability_pct?: number | null
  runs_per_week?: number | null
}

// Step summary from test definition (YAML)
export interface TestStepSummary {
  step_index: number
  plugin: string
  name: string
}

export interface TestSummary {
  id: string
  name: string
  description?: string
  source_ref: string
  step_count: number
  step_summaries?: TestStepSummary[]
  last_run_status?: string
  last_run_at?: string
  pass_rate?: number
  avg_duration_ms?: number
}

export interface SuiteDetail {
  id: string
  name: string
  description?: string
  file_path?: string
  source_ref: string
  test_count: number
  last_run_status?: string
  last_run_at?: string
  project: {
    id: string
    name: string
    repo_url: string
  }
  tests: TestSummary[]
}

// Suite run summary for activity tab
export interface SuiteRunSummary {
  id: string
  status: 'RUNNING' | 'PASSED' | 'FAILED' | 'CANCELLED' | 'PENDING'
  branch: string
  commit_sha?: string
  commit_message?: string
  config_source: 'repo_commit' | 'uncommitted'
  environment: string
  created_at: string
  started_at?: string
  ended_at?: string
  duration_ms?: number
  initiator_type: 'ci' | 'manual' | 'schedule'
  initiator_name?: string
  total_tests: number
  passed_tests: number
  failed_tests: number
  timeout_tests: number
  skipped_tests: number
}

// Run detail from /api/runs/{runId}
export interface RunDetail {
  id: string
  project_id?: string
  status: 'RUNNING' | 'PASSED' | 'FAILED' | 'CANCELLED' | 'PENDING'
  suite_name: string
  initiator: string
  trigger: string
  schedule_name: string
  config_source: string
  source: string
  branch: string
  environment: string
  commit_sha?: string
  bundle_sha?: string
  commit_message?: string
  environment_id?: string
  schedule_id?: string
  total_tests: number
  passed_tests: number
  failed_tests: number
  timeout_tests: number
  skipped_tests: number
  created_at: string
  updated_at: string
  started_at?: string
  ended_at?: string
  duration_ms?: number
  initiator_type: 'ci' | 'manual' | 'schedule'
  initiator_name?: string
}

// Step summary for test runs (minimal data for step chips)
export interface StepSummary {
  step_index: number
  name: string
  plugin: string
  status: 'RUNNING' | 'PASSED' | 'FAILED' | 'PENDING'
}

// Run test from /api/runs/{runId}/tests
export interface RunTest {
  id: string
  run_id: string
  test_id?: string
  workflow_id: string
  name: string
  status: 'RUNNING' | 'PASSED' | 'FAILED' | 'CANCELLED' | 'PENDING'
  step_count: number
  passed_steps: number
  failed_steps: number
  error_message?: string
  created_at: string
  started_at?: string
  ended_at?: string
  duration_ms?: number
  steps?: StepSummary[]
}

// Run log from /api/runs/{runId}/logs
export interface RunLog {
  id: string
  run_id: string
  run_test_id?: string
  run_step_id?: string
  level: string
  message: string
  logged_at: string
  metadata?: Record<string, unknown>
}

// Test run detail from /api/test-runs/{testRunId}
export interface TestRunDetail {
  test: RunTest
  run: RunDetail
}

// Assertion result from the HTTP plugin
export interface AssertionResult {
  type: string // status_code, json_path, header
  name?: string // Header name for header assertions
  path?: string // JSONPath/jq expression for json_path assertions
  expected?: unknown // Expected value
  actual?: unknown // Actual value received
  passed: boolean
  message?: string // Error message if failed
}

// Saved variable from a step
export interface SavedVariable {
  name: string
  value: string
  source_type?: string // json_path, header, or auto
  source?: string // The expression used
}

// Step configuration snapshot
export interface StepConfig {
  name: string
  plugin: string
  config?: Record<string, unknown>
  assertions?: Array<Record<string, unknown>>
  save?: Array<Record<string, unknown>>
  retry?: {
    maximum_attempts?: number
    initial_interval?: string
    maximum_interval?: string
    backoff_coefficient?: number
  }
}

// Run step from /api/test-runs/{testRunId}/steps
export interface RunStep {
  id: string
  run_test_id: string
  step_index: number
  name: string
  plugin: string
  status: 'RUNNING' | 'PASSED' | 'FAILED' | 'PENDING' | 'DEFINITION'
  error_message?: string
  assertions_passed: number
  assertions_failed: number
  created_at: string
  started_at?: string
  ended_at?: string
  duration_ms?: number
  request_data?: {
    method: string
    url: string
    headers?: Record<string, string>
    body?: string
    body_truncated?: boolean
    body_bytes?: number
  }
  response_data?: {
    status_code: number
    headers?: Record<string, string>
    body?: string
    body_truncated?: boolean
    body_bytes?: number
  }
  assertions_data?: AssertionResult[]
  variables_data?: SavedVariable[]
  step_config?: StepConfig
}

// Profile types
export interface ProfileUser {
  id: string
  email: string
  name: string
  username: string
}

export interface ProfileOrganization {
  id: string
  name: string
  slug: string
  role: 'owner' | 'member'
}

export interface ProfileGitHub {
  username: string
  avatar_url: string
  app_installed: boolean
  app_account_login: string
  installation_id: number
}

export interface ProfileProjectPermission {
  project_id: string
  project_name: string
  source_ref: string
  permissions: string[]
}

export interface ProfileData {
  user: ProfileUser
  organization: ProfileOrganization
  github: ProfileGitHub
  project_permissions: ProfileProjectPermission[]
}

// Overview / Setup types
export interface SetupStep {
  id: string
  title: string
  complete: boolean
}

export interface SetupData {
  steps: SetupStep[]
  github_app_slug?: string
  github_install_url?: string
}

// Overview dashboard metrics types
export interface OverviewNowMetrics {
  failing_monitors: number
  failing_tests_24h: number
  runs_in_progress: number
  pass_rate_24h: number | null
  median_duration_ms_24h: number | null
}

export interface PassRateDataPoint {
  date: string // YYYY-MM-DD
  pass_rate: number // 0-100
  volume: number
}

export interface SuiteFailureData {
  suite: string
  passes: number
  failures: number
}

export interface OverviewMetricsResponse {
  now: OverviewNowMetrics
  pass_rate_over_time: PassRateDataPoint[]
  failures_by_suite_24h: SuiteFailureData[]
}

export interface GitHubRepo {
  id: number
  name: string
  full_name: string
  private: boolean
  default_branch: string
  html_url: string
}

export interface ConnectRepoResult {
  project_id: string
  name: string
  repo_url: string
}

export interface SyncResult {
  synced: boolean
  message?: string
}

// Environment types
export interface ProjectEnvironment {
  id: string
  project_id: string
  name: string
  slug: string
  env_secrets_keys: string[] // Only keys are returned, not values
  config_vars: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateEnvironmentRequest {
  name: string
  slug: string
  env_secrets?: Record<string, string>
  config_vars?: Record<string, unknown>
}

export interface UpdateEnvironmentRequest {
  name?: string
  slug?: string
  env_secrets?: Record<string, string> // Empty string value removes the key
  config_vars?: Record<string, unknown>
}

// Query key factories
export const consoleKeys = {
  all: ['console'] as const,
  projects: () => [...consoleKeys.all, 'projects'] as const,
  project: (id: string) => [...consoleKeys.all, 'project', id] as const,
  projectSuites: (id: string) => [...consoleKeys.all, 'project', id, 'suites'] as const,
  projectEnvironments: (id: string) => [...consoleKeys.all, 'project', id, 'environments'] as const,
  suiteActivity: () => [...consoleKeys.all, 'suites', 'activity'] as const,
  suite: (id: string) => [...consoleKeys.all, 'suite', id] as const,
  suiteRuns: (id: string) => [...consoleKeys.all, 'suite', id, 'runs'] as const,
  profile: () => [...consoleKeys.all, 'profile'] as const,
  run: (id: string) => [...consoleKeys.all, 'run', id] as const,
  runTests: (id: string) => [...consoleKeys.all, 'run', id, 'tests'] as const,
  runLogs: (id: string) => [...consoleKeys.all, 'run', id, 'logs'] as const,
  testRun: (id: string) => [...consoleKeys.all, 'testRun', id] as const,
  testRunLogs: (id: string) => [...consoleKeys.all, 'testRun', id, 'logs'] as const,
  testRunSteps: (id: string) => [...consoleKeys.all, 'testRun', id, 'steps'] as const,
  // Test Detail (test definition page)
  testDetail: (id: string) => [...consoleKeys.all, 'test', id] as const,
  testRuns: (id: string) => [...consoleKeys.all, 'test', id, 'runs'] as const,
  // Overview / Setup
  overviewSetup: () => [...consoleKeys.all, 'overview', 'setup'] as const,
  overviewMetrics: (projectIds?: string[], environmentId?: string, days?: number) =>
    [...consoleKeys.all, 'overview', 'metrics', { projectIds, environmentId, days }] as const,
  githubAppRepos: () => [...consoleKeys.all, 'github', 'repos'] as const,
}

// Query hooks

export function useProjects() {
  return useQuery({
    queryKey: consoleKeys.projects(),
    queryFn: () => apiGet<ProjectSummary[]>('/api/projects'),
  })
}

export function useProject(projectId: string) {
  return useQuery({
    queryKey: consoleKeys.project(projectId),
    queryFn: () => apiGet<ProjectDetail>(`/api/projects/${projectId}`),
    enabled: !!projectId,
  })
}

export function useProjectSuites(projectId: string) {
  return useQuery({
    queryKey: consoleKeys.projectSuites(projectId),
    queryFn: () => apiGet<SuiteSummary[]>(`/api/projects/${projectId}/suites`),
    enabled: !!projectId,
  })
}

export function useSuiteActivity() {
  return useQuery({
    queryKey: consoleKeys.suiteActivity(),
    queryFn: () => apiGet<SuiteActivityItem[]>('/api/suites/activity'),
  })
}

export function useSuite(suiteId: string) {
  return useQuery({
    queryKey: consoleKeys.suite(suiteId),
    queryFn: () => apiGet<SuiteDetail>(`/api/suites/${suiteId}`),
    enabled: !!suiteId,
  })
}

// Suite runs params for server-side filtering and pagination
export interface SuiteRunsParams {
  environmentId?: string
  branch?: string
  triggers?: string[]
  search?: string
  limit?: number
  offset?: number
  runsPerBranch?: number
}

// Suite runs response with mode indicator for summary vs branch drilldown
export interface SuiteRunsResponse {
  mode: 'summary' | 'branch'
  runs: SuiteRunSummary[]
  total?: number    // Only meaningful in branch mode
  limit: number
  offset: number
  branch?: string   // Only in branch mode
}

export function useSuiteRuns(suiteId: string, params: SuiteRunsParams = {}) {
  // Build query string from params
  const buildQueryString = () => {
    const queryParts: string[] = []
    if (params.environmentId) {
      queryParts.push(`environment_id=${encodeURIComponent(params.environmentId)}`)
    }
    if (params.branch) {
      queryParts.push(`branch=${encodeURIComponent(params.branch)}`)
    }
    if (params.triggers && params.triggers.length > 0) {
      queryParts.push(`triggers=${encodeURIComponent(params.triggers.join(','))}`)
    }
    if (params.search) {
      queryParts.push(`search=${encodeURIComponent(params.search)}`)
    }
    if (params.limit !== undefined) {
      queryParts.push(`limit=${params.limit}`)
    }
    if (params.offset !== undefined) {
      queryParts.push(`offset=${params.offset}`)
    }
    if (params.runsPerBranch !== undefined) {
      queryParts.push(`runs_per_branch=${params.runsPerBranch}`)
    }
    return queryParts.length > 0 ? `?${queryParts.join('&')}` : ''
  }

  return useQuery({
    queryKey: [...consoleKeys.suiteRuns(suiteId), params] as const,
    queryFn: () => {
      const url = `/api/suites/${suiteId}/runs${buildQueryString()}`
      return apiGet<SuiteRunsResponse>(url)
    },
    enabled: !!suiteId,
    refetchOnWindowFocus: true,
    refetchIntervalInBackground: false, // Don't poll when tab is backgrounded
    // Keep previous data while fetching (prevents UI flicker during refetch/search)
    placeholderData: (previousData) => previousData,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return SUITE_RUNS_POLL_IDLE_MS // Initial load: use idle polling
      // Guard against non-array runs (e.g., API version mismatch)
      const runs = Array.isArray(data.runs) ? data.runs : []
      // Fast polling when runs are live, idle polling otherwise (to discover new runs)
      const hasLiveRun = runs.some((run) => isLiveRunStatus(run.status))
      return hasLiveRun ? SUITE_RUNS_POLL_LIVE_MS : SUITE_RUNS_POLL_IDLE_MS
    },
  })
}

export function useProfile() {
  return useQuery({
    queryKey: consoleKeys.profile(),
    queryFn: () => apiGet<ProfileData>('/api/profile'),
  })
}

export function useUpdateProfileName() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (name: string) =>
      apiPatch<{ name: string }>('/api/profile/name', { name }),
    onSuccess: () => {
      // Invalidate profile to refresh the profile page
      queryClient.invalidateQueries({ queryKey: consoleKeys.profile() })
    },
  })
}

// Run detail hooks

export function useRun(runId: string) {
  return useQuery({
    queryKey: consoleKeys.run(runId),
    queryFn: () => apiGet<RunDetail>(`/api/runs/${runId}`),
    enabled: !!runId,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false
      // Poll every 2s while run is live
      return isLiveRunStatus(data.status) ? 2000 : false
    },
  })
}

export function useRunTests(runId: string) {
  return useQuery({
    queryKey: consoleKeys.runTests(runId),
    queryFn: () => apiGet<RunTest[]>(`/api/runs/${runId}/tests`),
    enabled: !!runId,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false
      // Poll every 2s if any test is live (PENDING or RUNNING)
      const hasLiveTest = data.some((test) => isLiveTestStatus(test.status))
      return hasLiveTest ? 2000 : false
    },
  })
}

export function useRunLogs(runId: string, options?: { limit?: number; isRunLive?: boolean }) {
  const limit = options?.limit ?? 500
  const isRunLive = options?.isRunLive ?? false
  return useQuery({
    queryKey: consoleKeys.runLogs(runId),
    queryFn: () => apiGet<RunLog[]>(`/api/runs/${runId}/logs?limit=${limit}`),
    enabled: !!runId,
    // Poll every 2s while run is live to stream logs
    refetchInterval: isRunLive ? 2000 : false,
  })
}

export function useTestRun(testRunId: string) {
  return useQuery({
    queryKey: consoleKeys.testRun(testRunId),
    queryFn: () => apiGet<TestRunDetail>(`/api/test-runs/${testRunId}`),
    enabled: !!testRunId,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false
      // Poll every 2s while test is live
      return isLiveTestStatus(data.test.status) ? 2000 : false
    },
  })
}

export function useTestRunLogs(testRunId: string, options?: { limit?: number; isTestLive?: boolean }) {
  const limit = options?.limit ?? 500
  const isTestLive = options?.isTestLive ?? false
  return useQuery({
    queryKey: consoleKeys.testRunLogs(testRunId),
    queryFn: () => apiGet<RunLog[]>(`/api/test-runs/${testRunId}/logs?limit=${limit}`),
    enabled: !!testRunId,
    // Poll every 2s while test is live to stream logs
    refetchInterval: isTestLive ? 2000 : false,
  })
}

export function useTestRunSteps(testRunId: string, options?: { isTestLive?: boolean }) {
  const isTestLive = options?.isTestLive ?? false
  return useQuery({
    queryKey: consoleKeys.testRunSteps(testRunId),
    queryFn: () => apiGet<RunStep[]>(`/api/test-runs/${testRunId}/steps`),
    enabled: !!testRunId,
    refetchInterval: (query) => {
      // Poll if test is live (passed from parent) OR if any step is still live
      if (isTestLive) return 2000
      const data = query.state.data
      if (!data) return false
      const hasLiveStep = data.some((step) => isLiveStepStatus(step.status))
      return hasLiveStep ? 2000 : false
    },
  })
}

// Overview / Setup hooks

export function useOverviewSetup() {
  return useQuery({
    queryKey: consoleKeys.overviewSetup(),
    queryFn: async () => {
      try {
        return await apiGet<SetupData>('/api/overview/setup')
      } catch (error) {
        // Return null for 403 (user not in org yet) instead of throwing
        if (error instanceof Error && 'status' in error && (error as { status: number }).status === 403) {
          return null
        }
        throw error
      }
    },
  })
}

// Polling intervals for overview metrics
const OVERVIEW_POLL_LIVE_MS = 5000   // 5s when runs are in progress
const OVERVIEW_POLL_IDLE_MS = 15000  // 15s when idle (frequent enough to discover new runs promptly)

export interface OverviewMetricsParams {
  projectIds?: string[]
  environmentId?: string
  days?: number
}

export function useOverviewMetrics(params: OverviewMetricsParams = {}) {
  const { projectIds, environmentId, days = 7 } = params

  return useQuery({
    queryKey: consoleKeys.overviewMetrics(projectIds, environmentId, days),
    queryFn: async () => {
      const queryParams = new URLSearchParams()
      if (projectIds && projectIds.length > 0) {
        queryParams.set('project_ids', projectIds.join(','))
      }
      if (environmentId) {
        queryParams.set('environment_id', environmentId)
      }
      if (days) {
        queryParams.set('days', days.toString())
      }
      const url = `/api/overview/metrics${queryParams.toString() ? `?${queryParams.toString()}` : ''}`
      return await apiGet<OverviewMetricsResponse>(url)
    },
    refetchOnWindowFocus: true,
    refetchIntervalInBackground: false, // Don't poll when browser tab is backgrounded
    // Keep previous data while fetching (prevents UI flicker during refetch)
    placeholderData: (previousData) => previousData,
    refetchInterval: (query) => {
      // Two-tier polling: faster when runs are in progress
      const data = query.state.data
      if (!data) return OVERVIEW_POLL_IDLE_MS
      return data.now.runs_in_progress > 0 ? OVERVIEW_POLL_LIVE_MS : OVERVIEW_POLL_IDLE_MS
    },
  })
}

export function useGithubAppRepos() {
  return useQuery({
    queryKey: consoleKeys.githubAppRepos(),
    queryFn: () => apiGet<GitHubRepo[]>('/api/github/app/repos'),
    enabled: false, // Only fetch when explicitly triggered
  })
}

export function useConnectGithubRepo() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (repoFullName: string) =>
      apiPost<ConnectRepoResult>('/api/github/app/repos/connect', {
        repo_full_name: repoFullName,
      }),
    onSuccess: () => {
      // Invalidate setup data to refresh the setup steps
      queryClient.invalidateQueries({ queryKey: consoleKeys.overviewSetup() })
    },
  })
}

export function useSyncGithubApp() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => apiPost<SyncResult>('/api/github/app/sync'),
    onSuccess: () => {
      // Invalidate setup data to refresh the setup steps
      queryClient.invalidateQueries({ queryKey: consoleKeys.overviewSetup() })
    },
  })
}

// Environment hooks

export function useProjectEnvironments(projectId: string) {
  return useQuery({
    queryKey: consoleKeys.projectEnvironments(projectId),
    queryFn: () => apiGet<ProjectEnvironment[]>(`/api/projects/${projectId}/environments`),
    enabled: !!projectId,
  })
}

export function useCreateEnvironment(projectId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateEnvironmentRequest) =>
      apiPost<ProjectEnvironment>(`/api/projects/${projectId}/environments`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: consoleKeys.projectEnvironments(projectId) })
    },
  })
}

export function useUpdateEnvironment(projectId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ envId, data }: { envId: string; data: UpdateEnvironmentRequest }) =>
      apiPut<ProjectEnvironment>(`/api/projects/${projectId}/environments/${envId}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: consoleKeys.projectEnvironments(projectId) })
    },
  })
}

export function useDeleteEnvironment(projectId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (envId: string) =>
      apiDelete(`/api/projects/${projectId}/environments/${envId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: consoleKeys.projectEnvironments(projectId) })
    },
  })
}

// Project-aware mutations for "All Projects" mode
// These accept projectId at call time rather than hook creation time

export function useCreateEnvironmentForProject() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ projectId, data }: { projectId: string; data: CreateEnvironmentRequest }) =>
      apiPost<ProjectEnvironment>(`/api/projects/${projectId}/environments`, data),
    onSuccess: (_, { projectId }) => {
      queryClient.invalidateQueries({ queryKey: consoleKeys.projectEnvironments(projectId) })
    },
  })
}

export function useUpdateEnvironmentForProject() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ projectId, envId, data }: { projectId: string; envId: string; data: UpdateEnvironmentRequest }) =>
      apiPut<ProjectEnvironment>(`/api/projects/${projectId}/environments/${envId}`, data),
    onSuccess: (_, { projectId }) => {
      queryClient.invalidateQueries({ queryKey: consoleKeys.projectEnvironments(projectId) })
    },
  })
}

export function useDeleteEnvironmentForProject() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ projectId, envId }: { projectId: string; envId: string }) =>
      apiDelete(`/api/projects/${projectId}/environments/${envId}`),
    onSuccess: (_, { projectId }) => {
      queryClient.invalidateQueries({ queryKey: consoleKeys.projectEnvironments(projectId) })
    },
  })
}

// CI Token types

export interface CITokenProjectScope {
  project_id: string
  project_name: string
  scope: 'read' | 'write'
}

export interface CIToken {
  id: string
  name: string
  description?: string
  status: 'active' | 'revoked' | 'expired'
  never_expires: boolean
  expires_at?: string
  last_used_at?: string
  created_at: string
  updated_at: string
  created_by?: string
  revoked_at?: string
  revoked_by?: string
  projects: CITokenProjectScope[]
}

export interface CreateCITokenRequest {
  name: string
  description?: string
  never_expires: boolean
  expires_at?: string
  projects: { project_id: string; scope: 'read' | 'write' }[]
}

export interface CreateCITokenResponse {
  token: string
  token_record: CIToken
}

// CI Token hooks

export function useCITokens(options?: { includeRevoked?: boolean }) {
  const includeRevoked = options?.includeRevoked ?? false
  return useQuery({
    queryKey: [...consoleKeys.all, 'ci-tokens', { includeRevoked }] as const,
    queryFn: () => {
      const url = includeRevoked ? '/api/ci-tokens?include_revoked=true' : '/api/ci-tokens'
      return apiGet<CIToken[]>(url)
    },
  })
}

export function useCreateCIToken() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateCITokenRequest) =>
      apiPost<CreateCITokenResponse>('/api/ci-tokens', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'ci-tokens'] })
    },
  })
}

export function useRevokeCIToken() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (tokenId: string) =>
      apiPost<void>(`/api/ci-tokens/${tokenId}/revoke`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'ci-tokens'] })
    },
  })
}

// Access Control types

export interface OrganizationOwner {
  user_id: string
  email: string
  name: string
  username: string
  added_at: string
}

export interface ProjectMember {
  user_id: string
  email: string
  name: string
  username: string
  role: 'read' | 'write'
  joined_at: string
  updated_at: string
}

export interface OrgProjectMember {
  project_id: string
  project_name: string
  user_id: string
  username: string
  email: string
  name: string
  role: 'read' | 'write'
}

// Access Control hooks

export function useOrgOwners(orgId: string) {
  return useQuery({
    queryKey: [...consoleKeys.all, 'org', orgId, 'owners'] as const,
    queryFn: () => apiGet<OrganizationOwner[]>(`/api/orgs/${orgId}/owners`),
    enabled: !!orgId,
  })
}

export function useProjectMembers(projectId: string) {
  return useQuery({
    queryKey: [...consoleKeys.all, 'project', projectId, 'members'] as const,
    queryFn: () => apiGet<ProjectMember[]>(`/api/projects/${projectId}/members`),
    enabled: !!projectId,
  })
}

export function useAllProjectMembers(orgId: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: [...consoleKeys.all, 'org', orgId, 'project-members'] as const,
    queryFn: () => apiGet<OrgProjectMember[]>(`/api/orgs/${orgId}/project-members`),
    enabled: (options?.enabled ?? true) && !!orgId,
  })
}

export function useAddProjectMember(projectId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: { username: string; role: 'read' | 'write' }) =>
      apiPost<ProjectMember>(`/api/projects/${projectId}/members`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'project', projectId, 'members'] })
      // Also invalidate org-wide members list
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'org'] })
    },
  })
}

export function useAddProjectMemberForProject() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ projectId, data }: { projectId: string; data: { username: string; role: 'read' | 'write' } }) =>
      apiPost<ProjectMember>(`/api/projects/${projectId}/members`, data),
    onSuccess: (_, { projectId }) => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'project', projectId, 'members'] })
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'org'] })
    },
  })
}

export function useUpdateProjectMemberRole(projectId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: 'read' | 'write' }) =>
      apiPut<void>(`/api/projects/${projectId}/members/${userId}`, { role }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'project', projectId, 'members'] })
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'org'] })
    },
  })
}

export function useUpdateProjectMemberRoleForProject() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ projectId, userId, role }: { projectId: string; userId: string; role: 'read' | 'write' }) =>
      apiPut<void>(`/api/projects/${projectId}/members/${userId}`, { role }),
    onSuccess: (_, { projectId }) => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'project', projectId, 'members'] })
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'org'] })
    },
  })
}

export function useRemoveProjectMember(projectId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (userId: string) =>
      apiDelete(`/api/projects/${projectId}/members/${userId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'project', projectId, 'members'] })
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'org'] })
    },
  })
}

export function useRemoveProjectMemberForProject() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ projectId, userId }: { projectId: string; userId: string }) =>
      apiDelete(`/api/projects/${projectId}/members/${userId}`),
    onSuccess: (_, { projectId }) => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'project', projectId, 'members'] })
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'org'] })
    },
  })
}

// Project Invite types

export interface ProjectInviteProject {
  project_id: string
  project_name: string
  role: 'read' | 'write'
}

export interface ProjectInvite {
  id: string
  email: string
  invited_by: string
  inviter_name: string
  status: 'pending' | 'accepted' | 'revoked' | 'expired'
  expires_at: string
  created_at: string
  projects: ProjectInviteProject[]
}

export interface PendingProjectInvite {
  id: string
  organization_id: string
  organization_name: string
  inviter_name: string
  expires_at: string
  created_at: string
  projects: ProjectInviteProject[]
}

export interface ProjectInvitePreview {
  id: string
  organization_id: string
  organization_name: string
  inviter_name: string
  expires_at: string
  created_at: string
  projects: ProjectInviteProject[]
}

export interface CreateProjectInviteRequest {
  email: string
  projects: { project_id: string; role: 'read' | 'write' }[]
}

// Project Invite hooks

export function useProjectInvites(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: [...consoleKeys.all, 'project-invites'] as const,
    queryFn: () => apiGet<ProjectInvite[]>('/api/project-invites'),
    enabled: options?.enabled ?? true,
  })
}

export function usePendingProjectInvites(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: [...consoleKeys.all, 'project-invites', 'pending'] as const,
    queryFn: () => apiGet<PendingProjectInvite[]>('/api/project-invites/pending'),
    enabled: options?.enabled ?? true,
  })
}

export function useProjectInvitePreview(inviteId: string | null, code: string | null, options?: { enabled?: boolean }) {
  const enabled = options?.enabled ?? true
  return useQuery({
    queryKey: [...consoleKeys.all, 'project-invites', 'preview', inviteId, code] as const,
    queryFn: () =>
      apiGet<ProjectInvitePreview>(
        `/api/project-invites/preview?invite=${encodeURIComponent(inviteId || '')}&code=${encodeURIComponent(code || '')}`
      ),
    enabled: enabled && !!inviteId && !!code,
  })
}

export function useCreateProjectInvite() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateProjectInviteRequest) =>
      apiPost<{ invite_id: string; email: string; expires_at: string }>('/api/project-invites', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'project-invites'] })
    },
  })
}

export function useAcceptProjectInvite() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: { invite_id: string; code: string }) =>
      apiPost<{ organization: { id: string; name: string }; projects: ProjectInviteProject[] }>(
        '/api/project-invites/accept',
        input
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'project-invites'] })
      queryClient.invalidateQueries({ queryKey: consoleKeys.profile() })
      queryClient.invalidateQueries({ queryKey: consoleKeys.projects() })
    },
  })
}

export function useRevokeProjectInvite() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (inviteId: string) =>
      apiPost<void>(`/api/project-invites/${inviteId}/revoke`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'project-invites'] })
    },
  })
}

// ============================================
// Project Schedule Types and Hooks
// ============================================

export interface ProjectSchedule {
  id: string
  project_id: string
  environment_id: string
  name: string
  cron_expression: string
  timezone: string
  enabled: boolean
  next_run_at: string | null
  last_run_at: string | null
  last_run_id: string | null
  last_run_status: string | null
  created_at: string
  updated_at: string
  environment: {
    name: string
    slug: string
  }
}

export interface CreateProjectScheduleRequest {
  environment_id: string
  name: string
  cron_expression: string
  timezone: string
  enabled: boolean
}

export interface UpdateProjectScheduleRequest {
  name?: string
  cron_expression?: string
  timezone?: string
  enabled?: boolean
}

export function useProjectSchedules(projectId: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: [...consoleKeys.all, 'project', projectId, 'schedules'] as const,
    queryFn: () => apiGet<ProjectSchedule[]>(`/api/projects/${projectId}/schedules`),
    enabled: (options?.enabled ?? true) && !!projectId,
  })
}

export function useCreateProjectSchedule(projectId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateProjectScheduleRequest) =>
      apiPost<ProjectSchedule>(`/api/projects/${projectId}/project-schedules`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'project', projectId, 'schedules'] })
    },
  })
}

export function useUpdateProjectSchedule() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ scheduleId, data }: { scheduleId: string; data: UpdateProjectScheduleRequest }) =>
      apiPut<ProjectSchedule>(`/api/project-schedules/${scheduleId}`, data),
    onSuccess: () => {
      // Invalidate all project schedule queries since we don't know the project ID here
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all] })
    },
  })
}

export function useDeleteProjectSchedule() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (scheduleId: string) =>
      apiDelete(`/api/project-schedules/${scheduleId}`),
    onSuccess: () => {
      // Invalidate all project schedule queries
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all] })
    },
  })
}

// ============================================
// Suite Schedule Types and Hooks (Overrides)
// ============================================

export interface SuiteSchedule {
  id: string
  suite_id: string
  project_id: string
  environment_id: string
  name: string
  cron_expression: string
  timezone: string
  enabled: boolean
  last_run_id?: string
  last_run_status?: string
  last_run_at?: string
  next_run_at?: string
  created_by: string
  created_at: string
  updated_at: string
  environment: {
    name: string
    slug: string
  }
}

export interface CreateSuiteScheduleRequest {
  environment_id: string
  name: string
  cron_expression: string
  timezone: string
  enabled: boolean
}

export interface UpdateSuiteScheduleRequest {
  name?: string
  cron_expression?: string
  timezone?: string
  enabled?: boolean
}

export function useSuiteSchedules(suiteId: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: [...consoleKeys.all, 'suite', suiteId, 'schedules'] as const,
    queryFn: () => apiGet<SuiteSchedule[]>(`/api/suites/${suiteId}/schedules`),
    enabled: (options?.enabled ?? true) && !!suiteId,
  })
}

export function useUpsertSuiteSchedule(suiteId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateSuiteScheduleRequest) =>
      apiPost<SuiteSchedule>(`/api/suites/${suiteId}/schedules`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all, 'suite', suiteId, 'schedules'] })
    },
  })
}

export function useUpdateSuiteSchedule() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ scheduleId, data }: { scheduleId: string; data: UpdateSuiteScheduleRequest }) =>
      apiPut<SuiteSchedule>(`/api/suite-schedules/${scheduleId}`, data),
    onSuccess: () => {
      // Invalidate all suite schedule queries since we don't know the suite ID here
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all] })
    },
  })
}

export function useDeleteSuiteSchedule() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (scheduleId: string) =>
      apiDelete(`/api/suite-schedules/${scheduleId}`),
    onSuccess: () => {
      // Invalidate all suite schedule queries
      queryClient.invalidateQueries({ queryKey: [...consoleKeys.all] })
    },
  })
}

// ============================================
// Test Health Types and Hooks
// ============================================

export interface TestHealthItem {
  id: string
  name: string
  step_count: number
  plugins: string[]
  suite_id: string
  suite_name: string
  project_id: string
  project_name: string
  recent_results: ('success' | 'failed' | 'pending' | 'running')[]
  success_rate: string | null
  last_run_at: string | null
  next_run_at: string | null
  is_live: boolean
}

export interface TestHealthSuiteOption {
  id: string
  name: string
}

export interface TestHealthResponse {
  tests: TestHealthItem[]
  suites: TestHealthSuiteOption[]
}

export interface TestHealthParams {
  projectIds?: string[]
  environmentId?: string
  suiteIds?: string[]
  plugins?: string[]
  search?: string
  limit?: number
}

export function useTestHealth(params: TestHealthParams = {}) {
  // Build query string
  const queryParts: string[] = []
  if (params.projectIds && params.projectIds.length > 0) {
    queryParts.push(`project_ids=${params.projectIds.join(',')}`)
  }
  if (params.environmentId) {
    queryParts.push(`environment_id=${params.environmentId}`)
  }
  if (params.suiteIds && params.suiteIds.length > 0) {
    queryParts.push(`suite_ids=${params.suiteIds.join(',')}`)
  }
  if (params.plugins && params.plugins.length > 0) {
    queryParts.push(`plugins=${params.plugins.join(',')}`)
  }
  if (params.search) {
    queryParts.push(`search=${encodeURIComponent(params.search)}`)
  }
  if (params.limit) {
    queryParts.push(`limit=${params.limit}`)
  }

  const queryString = queryParts.length > 0 ? `?${queryParts.join('&')}` : ''

  return useQuery({
    queryKey: [...consoleKeys.all, 'test-health', params] as const,
    queryFn: () => apiGet<TestHealthResponse>(`/api/test-health${queryString}`),
    refetchOnWindowFocus: true,
    refetchIntervalInBackground: false, // Don't poll when tab is backgrounded
    // Keep previous data while fetching new results (prevents UI flicker during search)
    placeholderData: (previousData) => previousData,
    // Two-tier polling: fast when tests are live, idle otherwise (to discover new runs)
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return TEST_HEALTH_POLL_IDLE_MS // Initial load: use idle polling
      const hasLiveTest = data.tests.some((test) => test.is_live)
      return hasLiveTest ? TEST_HEALTH_POLL_LIVE_MS : TEST_HEALTH_POLL_IDLE_MS
    },
  })
}

// ============================================
// Test Detail Types and Hooks
// ============================================

// Retry policy for step definitions
export interface StepRetryPolicy {
  initial_interval?: string
  maximum_interval?: string
  maximum_attempts?: number
  backoff_coefficient?: number
  non_retryable_errors?: string[]
}

// Enriched step definition from the test YAML
export interface TestDetailStep {
  step_index: number
  plugin: string
  name: string
  config?: Record<string, unknown>
  assertions?: Array<Record<string, unknown>>
  save?: Array<Record<string, unknown>>
  retry?: StepRetryPolicy
}

// Test detail response from GET /api/tests/:testId
export interface TestDetail {
  id: string
  name: string
  description?: string
  source_ref: string
  step_count: number
  suite_id: string
  suite_name: string
  project_id: string
  project_name: string
  project_default_branch: string
  steps: TestDetailStep[]
  created_at: string
  updated_at: string
}

// Run summary for test detail sidebar
export interface TestRunForTest {
  id: string
  run_id: string
  status: 'RUNNING' | 'PASSED' | 'FAILED' | 'CANCELLED' | 'PENDING' | 'TIMEOUT'
  trigger: 'ci' | 'manual' | 'schedule'
  environment: string
  branch: string
  initiator: string
  initiator_name?: string
  commit_sha?: string
  duration_ms?: number
  created_at: string
  started_at?: string
  ended_at?: string
}

// Server-side paginated response for test runs
export interface TestRunsResponse {
  runs: TestRunForTest[]
  total: number
  limit: number
  offset: number
}

export interface TestRunsParams {
  triggers?: string[]
  environmentId?: string
  limit?: number
  offset?: number
}

// Polling intervals for test detail page
const TEST_DETAIL_RUNS_POLL_LIVE_MS = 3000  // 3s when runs are in progress
const TEST_DETAIL_RUNS_POLL_IDLE_MS = 15000 // 15s when idle

export function useTestDetail(testId: string) {
  return useQuery({
    queryKey: consoleKeys.testDetail(testId),
    queryFn: () => apiGet<TestDetail>(`/api/tests/${testId}`),
    enabled: !!testId,
  })
}

export function useTestRuns(testId: string, params: TestRunsParams = {}) {
  // Build query string
  const queryParts: string[] = []
  if (params.triggers && params.triggers.length > 0) {
    queryParts.push(`triggers=${params.triggers.join(',')}`)
  }
  if (params.environmentId) {
    queryParts.push(`environment_id=${params.environmentId}`)
  }
  if (params.limit) {
    queryParts.push(`limit=${params.limit}`)
  }
  if (params.offset !== undefined) {
    queryParts.push(`offset=${params.offset}`)
  }

  const queryString = queryParts.length > 0 ? `?${queryParts.join('&')}` : ''

  return useQuery({
    queryKey: [...consoleKeys.testRuns(testId), params] as const,
    queryFn: () => apiGet<TestRunsResponse>(`/api/tests/${testId}/runs${queryString}`),
    enabled: !!testId,
    refetchOnWindowFocus: true,
    refetchIntervalInBackground: false,
    // Keep previous data while fetching new results (prevents UI flicker during page/filter changes)
    placeholderData: (previousData) => previousData,
    // Two-tier polling: fast when runs are in progress, idle otherwise
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data || !data.runs) return TEST_DETAIL_RUNS_POLL_IDLE_MS
      const hasLiveRun = data.runs.some((run) => isLiveTestStatus(run.status))
      return hasLiveRun ? TEST_DETAIL_RUNS_POLL_LIVE_MS : TEST_DETAIL_RUNS_POLL_IDLE_MS
    },
  })
}
