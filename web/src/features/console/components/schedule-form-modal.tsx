import { useState, useEffect } from 'react';
import { X, ToggleRight, ToggleLeft, Loader2 } from 'lucide-react';

export interface ScheduleFormData {
  name: string;
  environment_id: string;
  cron_expression: string;
  timezone: string;
  enabled: boolean;
}

export interface ScheduleFormEnvironment {
  id: string;
  name: string;
  slug: string;
}

interface ScheduleFormModalProps {
  /** Whether the modal is open */
  isOpen: boolean;
  /** Modal mode - determines title, button text, and whether environment is editable */
  mode: 'create' | 'edit';
  /** Available environments to select from */
  environments: ScheduleFormEnvironment[];
  /** Initial form values (required for edit mode) */
  initialValues?: Partial<ScheduleFormData>;
  /** Whether the form is currently submitting */
  isSubmitting?: boolean;
  /** Error message to display */
  error?: string | null;
  /** Callback when form is submitted with valid data */
  onSubmit: (data: ScheduleFormData) => void;
  /** Callback when modal is closed */
  onClose: () => void;
  /** Callback to clear error state */
  onErrorClear?: () => void;
}

const TIMEZONES = [
  'UTC',
  'America/New_York',
  'America/Los_Angeles',
  'America/Chicago',
  'Europe/London',
  'Europe/Paris',
  'Asia/Tokyo',
  'Asia/Shanghai',
  'Australia/Sydney',
];

/**
 * A reusable modal component for creating or editing project schedules.
 * Used in both suite detail and project detail pages.
 */
export function ScheduleFormModal({
  isOpen,
  mode,
  environments,
  initialValues,
  isSubmitting = false,
  error,
  onSubmit,
  onClose,
  onErrorClear,
}: ScheduleFormModalProps) {
  // Form state
  const [name, setName] = useState('');
  const [environmentId, setEnvironmentId] = useState('');
  const [cronExpression, setCronExpression] = useState('');
  const [timezone, setTimezone] = useState('UTC');
  const [enabled, setEnabled] = useState(true);
  const [localError, setLocalError] = useState<string | null>(null);

  // Reset form when modal opens or initialValues change
  useEffect(() => {
    if (isOpen) {
      setName(initialValues?.name || '');
      setEnvironmentId(initialValues?.environment_id || environments[0]?.id || '');
      setCronExpression(initialValues?.cron_expression || '');
      setTimezone(initialValues?.timezone || 'UTC');
      setEnabled(initialValues?.enabled ?? true);
      setLocalError(null);
    }
  }, [isOpen, initialValues, environments]);

  // Display either external error or local validation error
  const displayError = error || localError;

  const handleSubmit = () => {
    // Validation
    if (!name.trim()) {
      setLocalError('Schedule name is required');
      return;
    }
    if (!environmentId) {
      setLocalError('Environment is required');
      return;
    }
    if (!cronExpression.trim()) {
      setLocalError('Cron expression is required');
      return;
    }

    setLocalError(null);
    onErrorClear?.();

    onSubmit({
      name: name.trim(),
      environment_id: environmentId,
      cron_expression: cronExpression.trim(),
      timezone,
      enabled,
    });
  };

  const handleClose = () => {
    setLocalError(null);
    onClose();
  };

  if (!isOpen) return null;

  const title = mode === 'create' ? 'Add Schedule' : 'Edit Schedule';
  const submitText = mode === 'create' ? 'Add Schedule' : 'Save Changes';
  const isEnvironmentDisabled = mode === 'edit';

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-xl w-full max-w-md mx-4">
        {/* Modal Header */}
        <div className="flex items-center justify-between p-6 border-b border-[#e5e5e5]">
          <h3>{title}</h3>
          <button
            onClick={handleClose}
            className="text-[#666666] hover:text-black transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Modal Body */}
        <div className="p-6 space-y-4">
          {/* Error message */}
          {displayError && (
            <div className="bg-red-50 border border-red-200 rounded-md p-3 text-sm text-red-700">
              {displayError}
            </div>
          )}

          {/* Schedule Name */}
          <div>
            <label className="text-sm mb-2 block">Schedule Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g., Daily Staging Tests"
              className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-black/5"
            />
          </div>

          {/* Environment */}
          <div>
            <label className="text-sm mb-2 block">Environment</label>
            <select
              value={environmentId}
              onChange={(e) => setEnvironmentId(e.target.value)}
              disabled={isEnvironmentDisabled}
              className={`w-full px-3 py-2 border border-[#e5e5e5] rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-black/5 ${
                isEnvironmentDisabled
                  ? 'bg-[#fafafa] text-[#666666] cursor-not-allowed'
                  : 'bg-white'
              }`}
            >
              {environments.map((env) => (
                <option key={env.id} value={env.id}>
                  {env.name}
                </option>
              ))}
            </select>
            {isEnvironmentDisabled && (
              <p className="text-xs text-[#999999] mt-1">
                Environment cannot be changed. Delete and recreate to use a different environment.
              </p>
            )}
          </div>

          {/* Cron Expression */}
          <div>
            <label className="text-sm mb-2 block">Cron Expression</label>
            <input
              type="text"
              value={cronExpression}
              onChange={(e) => setCronExpression(e.target.value)}
              placeholder="*/30 * * * *"
              className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md text-sm font-mono focus:outline-none focus:ring-2 focus:ring-black/5"
            />
            <p className="text-xs text-[#999999] mt-1">
              Example: */30 * * * * (every 30 minutes)
            </p>
          </div>

          {/* Timezone */}
          <div>
            <label className="text-sm mb-2 block">Timezone</label>
            <select
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
              className="w-full px-3 py-2 bg-white border border-[#e5e5e5] rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-black/5"
            >
              {TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </select>
          </div>

          {/* Enabled Toggle */}
          <div className="flex items-center justify-between">
            <label className="text-sm">Enabled</label>
            <button
              onClick={() => setEnabled(!enabled)}
              className={`p-2 rounded transition-colors ${
                enabled ? 'text-[#4CBB17]' : 'text-[#999999]'
              }`}
            >
              {enabled ? (
                <ToggleRight className="w-6 h-6" />
              ) : (
                <ToggleLeft className="w-6 h-6" />
              )}
            </button>
          </div>
        </div>

        {/* Modal Footer */}
        <div className="flex items-center justify-end gap-3 p-6 border-t border-[#e5e5e5]">
          <button
            onClick={handleClose}
            className="px-4 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors"
          >
            Cancel
          </button>
          <button
            disabled={isSubmitting}
            onClick={handleSubmit}
            className="px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {isSubmitting && <Loader2 className="w-4 h-4 animate-spin" />}
            {submitText}
          </button>
        </div>
      </div>
    </div>
  );
}
