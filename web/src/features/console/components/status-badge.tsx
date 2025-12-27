interface StatusBadgeProps {
  status: 'success' | 'running' | 'failed' | 'pending';
  showLabel?: boolean;
  size?: 'sm' | 'md';
}

export function StatusBadge({ status, showLabel = false, size = 'md' }: StatusBadgeProps) {
  const dotSize = size === 'sm' ? 'w-2 h-2' : 'w-2.5 h-2.5';
  
  const config = {
    success: {
      color: '#4CBB17',
      label: 'Passed',
    },
    running: {
      color: '#f6a724',
      label: 'Running',
    },
    failed: {
      color: '#ef0000',
      label: 'Failed',
    },
    pending: {
      color: '#999999',
      label: 'Pending',
    },
  };

  const { color, label } = config[status];

  if (showLabel) {
    return (
      <div className="inline-flex items-center gap-2 px-2 py-1 rounded-full bg-[#fafafa] border border-[#e5e5e5]">
        <div className={`${dotSize} rounded-full`} style={{ backgroundColor: color }} />
        <span className="text-sm text-[#666666]">{label}</span>
      </div>
    );
  }

  return (
    <div
      className={`${dotSize} rounded-full`}
      style={{ backgroundColor: color }}
      title={label}
    />
  );
}

interface EnvBadgeProps {
  env: string;
}

export function EnvBadge({ env }: EnvBadgeProps) {
  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded text-xs bg-[#fafafa] border border-[#e5e5e5] text-[#666666]">
      {env}
    </span>
  );
}

interface InitiatorBadgeProps {
  initiator: 'ci' | 'manual' | 'schedule';
  name?: string;
}

export function InitiatorBadge({ initiator, name }: InitiatorBadgeProps) {
  const getInitiatorColor = (initiator: string) => {
    switch (initiator) {
      case 'ci': return 'bg-blue-50 text-blue-700 border-blue-200';
      case 'manual': return 'bg-green-50 text-green-700 border-green-200';
      case 'schedule': return 'bg-purple-50 text-purple-700 border-purple-200';
      default: return 'bg-gray-50 text-gray-700 border-gray-200';
    }
  };

  return (
    <span className={`text-xs px-2 py-0.5 rounded border ${getInitiatorColor(initiator)}`}>
      {name ? `${initiator} (${name})` : initiator}
    </span>
  );
}

interface ConfigSourceBadgeProps {
  type: 'uncommitted' | 'repo';
  sha?: string;
}

export function ConfigSourceBadge({ type, sha }: ConfigSourceBadgeProps) {
  if (type === 'uncommitted') {
    return (
      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs bg-[#f6a724]/10 border border-[#f6a724]/30 text-[#f6a724]">
        Uncommitted
      </span>
    );
  }

  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded text-xs bg-[#fafafa] border border-[#e5e5e5] text-[#666666]">
      Repo@{sha?.slice(0, 7)}
    </span>
  );
}