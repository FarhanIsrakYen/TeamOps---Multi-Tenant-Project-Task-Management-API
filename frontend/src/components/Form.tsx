import type { FormHTMLAttributes, ReactNode } from "react";

export function Form({
  error,
  children,
  ...props
}: FormHTMLAttributes<HTMLFormElement> & {
  error?: string;
  children: ReactNode;
}) {
  return (
    <form className="form" {...props}>
      {error && (
        <div className="error-banner" role="alert">
          {error}
        </div>
      )}
      {children}
    </form>
  );
}
