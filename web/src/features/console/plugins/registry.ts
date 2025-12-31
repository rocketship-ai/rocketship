/**
 * Plugin Metadata Registry
 *
 * Centralized registry for plugin icons, labels, and styling.
 * Use this throughout the UI when rendering plugin information.
 */

import type { LucideIcon } from 'lucide-react';
import {
  Globe,
  Clock,
  Database,
  Bot,
  Terminal,
  FileText,
  Code,
  Sparkles,
  Play,
  MessageSquare,
} from 'lucide-react';

// =============================================================================
// Types
// =============================================================================

export type PluginId = string;

export interface PluginMeta {
  /** Plugin identifier (as used in YAML configs) */
  id: PluginId;
  /** Human-readable label */
  label: string;
  /** Lucide icon component */
  Icon: LucideIcon;
  /** Optional badge class name for colored styling */
  badgeClassName?: string;
  /** Description of what this plugin does */
  description?: string;
}

// =============================================================================
// Plugin Definitions
// =============================================================================

const PLUGIN_DEFINITIONS: Record<string, PluginMeta> = {
  http: {
    id: 'http',
    label: 'HTTP',
    Icon: Globe,
    badgeClassName: 'bg-blue-100 text-blue-700',
    description: 'Make HTTP requests to APIs',
  },
  delay: {
    id: 'delay',
    label: 'Delay',
    Icon: Clock,
    badgeClassName: 'bg-gray-100 text-gray-700',
    description: 'Wait for a specified duration',
  },
  sql: {
    id: 'sql',
    label: 'SQL',
    Icon: Database,
    badgeClassName: 'bg-purple-100 text-purple-700',
    description: 'Execute SQL queries',
  },
  supabase: {
    id: 'supabase',
    label: 'Supabase',
    Icon: Database,
    badgeClassName: 'bg-emerald-100 text-emerald-700',
    description: 'Interact with Supabase services',
  },
  script: {
    id: 'script',
    label: 'Script',
    Icon: Terminal,
    badgeClassName: 'bg-amber-100 text-amber-700',
    description: 'Execute custom scripts',
  },
  agent: {
    id: 'agent',
    label: 'Agent',
    Icon: Bot,
    badgeClassName: 'bg-violet-100 text-violet-700',
    description: 'AI-powered test automation',
  },
  playwright: {
    id: 'playwright',
    label: 'Playwright',
    Icon: Play,
    badgeClassName: 'bg-green-100 text-green-700',
    description: 'Browser automation with Playwright',
  },
  browser_use: {
    id: 'browser_use',
    label: 'Browser',
    Icon: Code,
    badgeClassName: 'bg-cyan-100 text-cyan-700',
    description: 'AI-driven browser automation',
  },
  log: {
    id: 'log',
    label: 'Log',
    Icon: FileText,
    badgeClassName: 'bg-slate-100 text-slate-700',
    description: 'Log messages during test execution',
  },
  llm: {
    id: 'llm',
    label: 'LLM',
    Icon: Sparkles,
    badgeClassName: 'bg-pink-100 text-pink-700',
    description: 'Large language model interactions',
  },
  mcp: {
    id: 'mcp',
    label: 'MCP',
    Icon: MessageSquare,
    badgeClassName: 'bg-indigo-100 text-indigo-700',
    description: 'Model Context Protocol',
  },
};

// Fallback for unknown plugins
const UNKNOWN_PLUGIN: PluginMeta = {
  id: 'unknown',
  label: 'Unknown',
  Icon: Code,
  badgeClassName: 'bg-gray-100 text-gray-500',
  description: 'Unknown plugin type',
};

// =============================================================================
// Public API
// =============================================================================

/**
 * Get metadata for a plugin by its ID.
 * Returns a fallback for unknown plugins.
 */
export function getPluginMeta(plugin: string): PluginMeta {
  const normalized = plugin.toLowerCase();
  return PLUGIN_DEFINITIONS[normalized] || {
    ...UNKNOWN_PLUGIN,
    id: plugin,
    label: plugin.toUpperCase(),
  };
}

/**
 * Get just the icon component for a plugin.
 */
export function getPluginIcon(plugin: string): LucideIcon {
  return getPluginMeta(plugin).Icon;
}

/**
 * Get all registered plugins.
 */
export function getAllPlugins(): PluginMeta[] {
  return Object.values(PLUGIN_DEFINITIONS);
}

/**
 * Check if a plugin is registered.
 */
export function isKnownPlugin(plugin: string): boolean {
  return plugin.toLowerCase() in PLUGIN_DEFINITIONS;
}

// =============================================================================
// Component Helpers (for direct use in JSX)
// =============================================================================

/**
 * Plugin icon mapping for use with Record<string, ComponentType>
 * Useful for places that expect a simple icon map.
 */
export const pluginIconMap: Record<string, LucideIcon> = Object.fromEntries(
  Object.entries(PLUGIN_DEFINITIONS).map(([key, meta]) => [
    key.charAt(0).toUpperCase() + key.slice(1), // Capitalize for legacy compatibility
    meta.Icon,
  ])
);

// Also include lowercase versions
Object.entries(PLUGIN_DEFINITIONS).forEach(([key, meta]) => {
  pluginIconMap[key] = meta.Icon;
});

// Legacy aliases for backward compatibility
pluginIconMap['HTTP'] = Globe;
pluginIconMap['Playwright'] = Play;
pluginIconMap['Supabase'] = Database;
pluginIconMap['Agent'] = Bot;
pluginIconMap['SQL'] = Database;
pluginIconMap['Script'] = Terminal;
pluginIconMap['Delay'] = Clock;
pluginIconMap['Log'] = FileText;
