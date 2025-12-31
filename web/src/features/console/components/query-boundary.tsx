import type { ReactNode } from 'react';
import type { UseQueryResult } from '@tanstack/react-query';
import { LoadingState, ErrorState } from './ui';

interface QueryBoundaryProps<TData> {
  query: Pick<UseQueryResult<TData>, 'data' | 'isLoading' | 'error' | 'refetch'>;
  children: (data: TData) => ReactNode;
  loadingMessage?: string;
  errorTitle?: string;
  /** Optional: Custom loading state component */
  loadingComponent?: ReactNode;
}

/**
 * QueryBoundary handles common loading/error states for React Query hooks.
 *
 * Usage:
 * ```tsx
 * const query = useProjects();
 *
 * return (
 *   <QueryBoundary query={query} loadingMessage="Loading projects...">
 *     {(projects) => (
 *       <ProjectsList projects={projects} />
 *     )}
 *   </QueryBoundary>
 * );
 * ```
 */
export function QueryBoundary<TData>({
  query,
  children,
  loadingMessage = 'Loading...',
  errorTitle = 'Something went wrong',
  loadingComponent,
}: QueryBoundaryProps<TData>) {
  const { data, isLoading, error, refetch } = query;

  if (isLoading) {
    return loadingComponent ?? <LoadingState message={loadingMessage} />;
  }

  if (error) {
    const errorMessage = error instanceof Error
      ? error.message
      : 'An unexpected error occurred';

    return (
      <ErrorState
        title={errorTitle}
        message={errorMessage}
        onRetry={() => refetch()}
      />
    );
  }

  if (data === undefined || data === null) {
    return null;
  }

  return <>{children(data)}</>;
}
