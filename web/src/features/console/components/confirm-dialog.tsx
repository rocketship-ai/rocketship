interface ConfirmDialogProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  cancelLabel?: string;
  confirmVariant?: 'danger' | 'primary';
  isConfirming?: boolean;
  onCancel: () => void;
  onConfirm: () => void | Promise<void>;
}

export function ConfirmDialog({
  isOpen,
  title,
  message,
  confirmLabel,
  cancelLabel = 'Cancel',
  confirmVariant = 'danger',
  isConfirming = false,
  onCancel,
  onConfirm,
}: ConfirmDialogProps) {
  if (!isOpen) return null;

  const confirmButtonClasses =
    confirmVariant === 'danger'
      ? 'bg-[#ef0000] text-white hover:bg-[#ef0000]/90'
      : 'bg-black text-white hover:bg-black/90';

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-lg p-6 max-w-md w-full mx-4">
        <h3 className="mb-4">{title}</h3>
        <p className="text-sm text-[#666666] mb-6">{message}</p>
        <div className="flex gap-2 justify-end">
          <button
            onClick={onCancel}
            disabled={isConfirming}
            className="px-4 py-2 bg-white border border-[#e5e5e5] rounded-md hover:bg-[#fafafa] transition-colors disabled:opacity-50"
          >
            {cancelLabel}
          </button>
          <button
            onClick={onConfirm}
            disabled={isConfirming}
            className={`px-4 py-2 rounded-md transition-colors disabled:opacity-50 ${confirmButtonClasses}`}
          >
            {isConfirming ? `${confirmLabel.split(' ')[0]}...` : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
