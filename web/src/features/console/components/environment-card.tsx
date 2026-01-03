import { Edit2, Trash2, Lock, Settings, Star } from 'lucide-react';

interface EnvironmentCardProps {
  id: string;
  name: string;
  slug: string;
  projectName?: string;
  isSelected: boolean;
  secretCount: number;
  configVarCount: number;
  onEdit?: () => void;
  onDelete?: () => void;
  editDisabled?: boolean;
  editDisabledReason?: string;
}

export function EnvironmentCard({
  name,
  slug,
  projectName,
  isSelected,
  secretCount,
  configVarCount,
  onEdit,
  onDelete,
  editDisabled = false,
  editDisabledReason,
}: EnvironmentCardProps) {
  return (
    <div
      className={`bg-white rounded-lg border shadow-sm p-6 ${isSelected ? 'border-black' : 'border-[#e5e5e5]'}`}
    >
      <div className="flex items-start justify-between mb-4">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <h3 className="font-medium">{name}</h3>
            {isSelected && (
              <Star className="w-4 h-4 fill-amber-400 text-amber-400" />
            )}
          </div>
          <p className="text-sm text-[#666666]">slug: {slug}</p>
          {projectName && (
            <p className="text-xs text-[#999999] mt-1">{projectName}</p>
          )}
        </div>
        <div className="flex items-center gap-1">
          {onEdit && (
            <button
              onClick={onEdit}
              disabled={editDisabled}
              className={`p-1.5 rounded transition-colors ${editDisabled ? 'opacity-50 cursor-not-allowed' : 'hover:bg-[#f5f5f5]'}`}
              title={editDisabledReason || 'Edit'}
            >
              <Edit2 className="w-4 h-4 text-[#666666]" />
            </button>
          )}
          {onDelete && (
            <button
              onClick={onDelete}
              disabled={editDisabled}
              className={`p-1.5 rounded transition-colors ${editDisabled ? 'opacity-50 cursor-not-allowed' : 'hover:bg-[#f5f5f5]'}`}
              title={editDisabledReason || 'Delete'}
            >
              <Trash2 className="w-4 h-4 text-[#666666]" />
            </button>
          )}
        </div>
      </div>

      <div className="space-y-2 text-sm">
        <div className="flex items-center gap-2">
          <Lock className="w-4 h-4 text-[#999999]" />
          <span className="text-[#666666]">
            {secretCount} secret{secretCount !== 1 ? 's' : ''}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <Settings className="w-4 h-4 text-[#999999]" />
          <span className="text-[#666666]">
            {configVarCount} config var{configVarCount !== 1 ? 's' : ''}
          </span>
        </div>
      </div>
    </div>
  );
}
