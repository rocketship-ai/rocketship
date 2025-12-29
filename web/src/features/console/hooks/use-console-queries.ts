import { useQuery } from '@tanstack/react-query'
import { apiGet } from '@/lib/api'

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

export interface TestSummary {
  id: string
  name: string
  description?: string
  source_ref: string
  step_count: number
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

// Query key factories
export const consoleKeys = {
  all: ['console'] as const,
  projects: () => [...consoleKeys.all, 'projects'] as const,
  project: (id: string) => [...consoleKeys.all, 'project', id] as const,
  projectSuites: (id: string) => [...consoleKeys.all, 'project', id, 'suites'] as const,
  suiteActivity: () => [...consoleKeys.all, 'suites', 'activity'] as const,
  suite: (id: string) => [...consoleKeys.all, 'suite', id] as const,
  suiteRuns: (id: string) => [...consoleKeys.all, 'suite', id, 'runs'] as const,
  profile: () => [...consoleKeys.all, 'profile'] as const,
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

export function useSuiteRuns(suiteId: string) {
  return useQuery({
    queryKey: consoleKeys.suiteRuns(suiteId),
    queryFn: () => apiGet<SuiteRunSummary[]>(`/api/suites/${suiteId}/runs`),
    enabled: !!suiteId,
  })
}

export function useProfile() {
  return useQuery({
    queryKey: consoleKeys.profile(),
    queryFn: () => apiGet<ProfileData>('/api/profile'),
  })
}
