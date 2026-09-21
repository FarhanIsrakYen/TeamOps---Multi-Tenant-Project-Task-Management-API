import {
  useId,
  type InputHTMLAttributes,
  type TextareaHTMLAttributes,
} from "react";

interface CommonProps {
  label: string;
  error?: string;
  hint?: string;
}

export function Input({
  label,
  error,
  hint,
  id,
  ...props
}: CommonProps & InputHTMLAttributes<HTMLInputElement>) {
  const generatedID = useId();
  const inputID = id ?? generatedID;
  return (
    <label className="field" htmlFor={inputID}>
      <span>{label}</span>
      <input id={inputID} aria-invalid={Boolean(error)} {...props} />
      {error ? (
        <small className="field-error">{error}</small>
      ) : (
        hint && <small>{hint}</small>
      )}
    </label>
  );
}

export function TextArea({
  label,
  error,
  hint,
  id,
  ...props
}: CommonProps & TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const generatedID = useId();
  const inputID = id ?? generatedID;
  return (
    <label className="field" htmlFor={inputID}>
      <span>{label}</span>
      <textarea id={inputID} aria-invalid={Boolean(error)} {...props} />
      {error ? (
        <small className="field-error">{error}</small>
      ) : (
        hint && <small>{hint}</small>
      )}
    </label>
  );
}
