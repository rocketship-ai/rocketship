import { useQueryClient, useQuery } from '@tanstack/react-query'
import { useCallback, useMemo } from 'react'

// Types for console filters
export interface ConsoleFilters {
  selectedProjectIds: string[]
  // Map of projectId -> environmentId for per-project environment selection
  selectedEnvironmentIdByProjectId: Record<string, string>
}

// Storage key for localStorage persistence
const STORAGE_KEY = 'rocketship.console.filters.v1'

// Query key for TanStack Query state
const FILTERS_QUERY_KEY = ['consoleFilters'] as const

// Default empty filters
const DEFAULT_FILTERS: ConsoleFilters = {
  selectedProjectIds: [],
  selectedEnvironmentIdByProjectId: {},
}

// Load filters from localStorage
function loadFiltersFromStorage(): ConsoleFilters {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      const parsed = JSON.parse(stored)
      return {
        selectedProjectIds: Array.isArray(parsed.selectedProjectIds) ? parsed.selectedProjectIds : [],
        // Safely handle old stored data that only had selectedProjectIds
        selectedEnvironmentIdByProjectId:
          typeof parsed.selectedEnvironmentIdByProjectId === 'object' &&
          parsed.selectedEnvironmentIdByProjectId !== null
            ? parsed.selectedEnvironmentIdByProjectId
            : {},
      }
    }
  } catch {
    // Ignore parse errors
  }
  return DEFAULT_FILTERS
}

// Save filters to localStorage
function saveFiltersToStorage(filters: ConsoleFilters): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(filters))
  } catch {
    // Ignore storage errors
  }
}

/**
 * Hook to manage the global project filter across the console.
 * Uses TanStack Query as the state container with localStorage persistence.
 */
export function useConsoleProjectFilter() {
  const queryClient = useQueryClient()

  // Use TanStack Query to manage the filter state
  const { data: filters } = useQuery({
    queryKey: FILTERS_QUERY_KEY,
    queryFn: loadFiltersFromStorage,
    initialData: loadFiltersFromStorage,
    staleTime: Infinity, // Never refetch automatically
  })

  const selectedProjectIds = filters?.selectedProjectIds ?? []

  const setSelectedProjectIds = useCallback(
    (ids: string[]) => {
      const currentFilters = queryClient.getQueryData<ConsoleFilters>(FILTERS_QUERY_KEY) ?? DEFAULT_FILTERS
      const newFilters: ConsoleFilters = {
        ...currentFilters,
        selectedProjectIds: ids,
      }
      saveFiltersToStorage(newFilters)
      queryClient.setQueryData(FILTERS_QUERY_KEY, newFilters)
    },
    [queryClient]
  )

  const clearSelectedProjectIds = useCallback(() => {
    setSelectedProjectIds([])
  }, [setSelectedProjectIds])

  return {
    selectedProjectIds,
    setSelectedProjectIds,
    clearSelectedProjectIds,
  }
}

/**
 * Hook to manage environment selection for a specific project.
 * Uses TanStack Query as the state container with localStorage persistence.
 * Environment selection is sticky per project across the UI.
 */
export function useConsoleEnvironmentFilter(projectId: string) {
  const queryClient = useQueryClient()

  // Use TanStack Query to manage the filter state
  const { data: filters } = useQuery({
    queryKey: FILTERS_QUERY_KEY,
    queryFn: loadFiltersFromStorage,
    initialData: loadFiltersFromStorage,
    staleTime: Infinity, // Never refetch automatically
  })

  const selectedEnvironmentId = useMemo(() => {
    if (!projectId) return undefined
    return filters?.selectedEnvironmentIdByProjectId?.[projectId]
  }, [filters, projectId])

  const setSelectedEnvironmentId = useCallback(
    (environmentId: string) => {
      if (!projectId) return
      const currentFilters = queryClient.getQueryData<ConsoleFilters>(FILTERS_QUERY_KEY) ?? DEFAULT_FILTERS
      const newFilters: ConsoleFilters = {
        ...currentFilters,
        selectedEnvironmentIdByProjectId: {
          ...currentFilters.selectedEnvironmentIdByProjectId,
          [projectId]: environmentId,
        },
      }
      saveFiltersToStorage(newFilters)
      queryClient.setQueryData(FILTERS_QUERY_KEY, newFilters)
    },
    [queryClient, projectId]
  )

  const clearSelectedEnvironmentId = useCallback(() => {
    if (!projectId) return
    const currentFilters = queryClient.getQueryData<ConsoleFilters>(FILTERS_QUERY_KEY) ?? DEFAULT_FILTERS
    const { [projectId]: _, ...rest } = currentFilters.selectedEnvironmentIdByProjectId
    const newFilters: ConsoleFilters = {
      ...currentFilters,
      selectedEnvironmentIdByProjectId: rest,
    }
    saveFiltersToStorage(newFilters)
    queryClient.setQueryData(FILTERS_QUERY_KEY, newFilters)
  }, [queryClient, projectId])

  return {
    selectedEnvironmentId,
    setSelectedEnvironmentId,
    clearSelectedEnvironmentId,
  }
}

/**
 * Helper hook to get the full environment selection map.
 * Useful for pages that need to show selections across multiple projects.
 */
export function useConsoleEnvironmentSelectionMap() {
  const { data: filters } = useQuery({
    queryKey: FILTERS_QUERY_KEY,
    queryFn: loadFiltersFromStorage,
    initialData: loadFiltersFromStorage,
    staleTime: Infinity,
  })

  return filters?.selectedEnvironmentIdByProjectId ?? {}
}
