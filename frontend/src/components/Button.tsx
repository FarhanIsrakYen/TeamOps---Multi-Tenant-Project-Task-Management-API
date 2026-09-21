import type { ButtonHTMLAttributes, ReactNode } from "react";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "danger" | "ghost";
  size?: "default" | "compact";
  busy?: boolean;
  children: ReactNode;
}

export function Button({
  variant = "primary",
  size = "default",
  busy = false,
  className = "",
  disabled,
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      className={`button ${variant} ${size} ${className}`.trim()}
      disabled={disabled || busy}
      aria-busy={busy}
      {...props}
    >
      {busy ? "Please wait…" : children}
    </button>
  );
}
