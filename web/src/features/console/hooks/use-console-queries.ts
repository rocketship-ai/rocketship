import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiPut, apiDelete } from '@/lib/api'

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
  status: 'RUNNING' | 'PASSED' | 'FAILED' | 'PENDING'
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
  role: 'admin' | 'member'
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
  // Overview / Setup
  overviewSetup: () => [...consoleKeys.all, 'overview', 'setup'] as const,
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

export function useSuiteRuns(suiteId: string, environmentId?: string) {
  return useQuery({
    queryKey: [...consoleKeys.suiteRuns(suiteId), environmentId] as const,
    queryFn: () => {
      const url = environmentId
        ? `/api/suites/${suiteId}/runs?environment_id=${environmentId}`
        : `/api/suites/${suiteId}/runs`
      return apiGet<SuiteRunSummary[]>(url)
    },
    enabled: !!suiteId,
  })
}

export function useProfile() {
  return useQuery({
    queryKey: consoleKeys.profile(),
    queryFn: () => apiGet<ProfileData>('/api/profile'),
  })
}

// Run detail hooks

export function useRun(runId: string) {
  return useQuery({
    queryKey: consoleKeys.run(runId),
    queryFn: () => apiGet<RunDetail>(`/api/runs/${runId}`),
    enabled: !!runId,
  })
}

export function useRunTests(runId: string) {
  return useQuery({
    queryKey: consoleKeys.runTests(runId),
    queryFn: () => apiGet<RunTest[]>(`/api/runs/${runId}/tests`),
    enabled: !!runId,
  })
}

export function useRunLogs(runId: string, limit = 500) {
  return useQuery({
    queryKey: consoleKeys.runLogs(runId),
    queryFn: () => apiGet<RunLog[]>(`/api/runs/${runId}/logs?limit=${limit}`),
    enabled: !!runId,
  })
}

export function useTestRun(testRunId: string) {
  return useQuery({
    queryKey: consoleKeys.testRun(testRunId),
    queryFn: () => apiGet<TestRunDetail>(`/api/test-runs/${testRunId}`),
    enabled: !!testRunId,
  })
}

export function useTestRunLogs(testRunId: string, limit = 500) {
  return useQuery({
    queryKey: consoleKeys.testRunLogs(testRunId),
    queryFn: () => apiGet<RunLog[]>(`/api/test-runs/${testRunId}/logs?limit=${limit}`),
    enabled: !!testRunId,
  })
}

export function useTestRunSteps(testRunId: string) {
  return useQuery({
    queryKey: consoleKeys.testRunSteps(testRunId),
    queryFn: () => apiGet<RunStep[]>(`/api/test-runs/${testRunId}/steps`),
    enabled: !!testRunId,
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

export function useCITokens() {
  return useQuery({
    queryKey: [...consoleKeys.all, 'ci-tokens'] as const,
    queryFn: () => apiGet<CIToken[]>('/api/ci-tokens'),
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
