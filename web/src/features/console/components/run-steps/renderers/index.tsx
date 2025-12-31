import type { RunStep } from '../../../hooks/use-console-queries';
import { HttpRenderer, getHttpTabs, getHttpSummary } from './http';
import { GenericRenderer, getGenericTabs } from './generic';

export interface StepTab {
  id: string;
  label: string;
  badge?: string;
}

export interface PluginRenderer {
  /** Render the expanded tab content */
  render: (step: RunStep, activeTab: string) => React.ReactNode;
  /** Get tabs for this plugin */
  getTabs: (step: RunStep) => StepTab[];
  /** Get summary info for collapsed view (e.g., HTTP method/URL) */
  getSummary?: (step: RunStep) => { method?: string; domain?: string; path?: string } | null;
}

const httpRenderer: PluginRenderer = {
  render: (step, activeTab) => <HttpRenderer step={step} activeTab={activeTab} />,
  getTabs: getHttpTabs,
  getSummary: getHttpSummary,
};

const genericRenderer: PluginRenderer = {
  render: (step, activeTab) => <GenericRenderer step={step} activeTab={activeTab} />,
  getTabs: getGenericTabs,
};

/** Plugin registry - add new plugin renderers here */
const PLUGIN_REGISTRY: Record<string, PluginRenderer> = {
  http: httpRenderer,
  // Future plugins:
  // delay: delayRenderer,
  // sql: sqlRenderer,
  // supabase: supabaseRenderer,
  // script: scriptRenderer,
  // agent: agentRenderer,
  // playwright: playwrightRenderer,
};

/** Get the renderer for a plugin, falls back to generic renderer */
export function getPluginRenderer(plugin: string): PluginRenderer {
  return PLUGIN_REGISTRY[plugin] ?? genericRenderer;
}

/** Check if a plugin has a custom renderer */
export function hasCustomRenderer(plugin: string): boolean {
  return plugin in PLUGIN_REGISTRY;
}

export { HttpRenderer, getHttpTabs, getHttpSummary } from './http';
export { GenericRenderer, getGenericTabs } from './generic';
