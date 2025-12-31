import { Loader2 } from 'lucide-react';
import type { ReactNode, ButtonHTMLAttributes } from 'react';

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  loading?: boolean;
  leftIcon?: ReactNode;
  children: ReactNode;
}

const variantStyles: Record<ButtonVariant, string> = {
  primary: 'bg-black text-white hover:bg-black/90 disabled:bg-black/50',
  secondary: 'bg-white border border-[#e5e5e5] text-[#666666] hover:border-black hover:text-black disabled:text-[#cccccc] disabled:hover:border-[#e5e5e5]',
  ghost: 'bg-transparent text-[#666666] hover:bg-[#fafafa] hover:text-black disabled:text-[#cccccc]',
  danger: 'bg-[#ef0000] text-white hover:bg-[#ef0000]/90 disabled:bg-[#ef0000]/50',
};

export function Button({
  variant = 'primary',
  loading = false,
  leftIcon,
  children,
  disabled,
  className = '',
  ...props
}: ButtonProps) {
  const isDisabled = disabled || loading;

  return (
    <button
      className={`
        inline-flex items-center justify-center gap-2 px-4 py-2 rounded-md
        transition-colors cursor-pointer
        disabled:cursor-not-allowed
        ${variantStyles[variant]}
        ${className}
      `.trim().replace(/\s+/g, ' ')}
      disabled={isDisabled}
      {...props}
    >
      {loading ? (
        <Loader2 className="w-4 h-4 animate-spin" />
      ) : leftIcon ? (
        leftIcon
      ) : null}
      {children}
    </button>
  );
}
