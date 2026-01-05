import type { RunStep } from '../../hooks/use-console-queries';

/** Re-export RunStep from queries for convenience */
export type { RunStep, AssertionResult, SavedVariable, StepConfig } from '../../hooks/use-console-queries';

/** UI status for step display */
export type StepUIStatus = 'success' | 'failed' | 'pending' | 'running' | 'definition';

/** Map API status to UI status */
export function mapStepStatus(status: string): StepUIStatus {
  switch (status.toUpperCase()) {
    case 'PASSED':
      return 'success';
    case 'FAILED':
      return 'failed';
    case 'RUNNING':
      return 'running';
    case 'DEFINITION':
      return 'definition';
    case 'PENDING':
    default:
      return 'pending';
  }
}

/** Tab configuration for step card */
export interface StepTabConfig {
  id: string;
  label: string;
  badge?: string;
  hidden?: boolean;
}

/** Props passed to plugin renderers */
export interface PluginRendererProps {
  step: RunStep;
}
