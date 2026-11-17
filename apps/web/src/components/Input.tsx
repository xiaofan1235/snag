import { useId } from "react";
import type { InputHTMLAttributes, TextareaHTMLAttributes, ReactNode } from "react";
import { cx } from "./cx";

interface FieldShell {
  label?: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  id?: string;
}

export interface InputProps extends InputHTMLAttributes<HTMLInputElement>, FieldShell {}

// Input（D2.2）：label（上）+ control + hint/error（下）；error 用 aria-invalid + aria-describedby。
// 安全：Secret 类输入由调用方设 type=password 且不进本地草稿（前端工程规则）。
export function Input({ label, hint, error, id, className, ...rest }: InputProps) {
  const auto = useId();
  const fid = id ?? auto;
  const descId = error ? `${fid}-err` : hint ? `${fid}-hint` : undefined;
  return (
    <div className="sn-field">
      {label && (
        <label className="sn-label" htmlFor={fid}>
          {label}
        </label>
      )}
      <input
        id={fid}
        className={cx("sn-input", "sn-focus", className)}
        aria-invalid={error ? true : undefined}
        aria-describedby={descId}
        {...rest}
      />
      {error ? (
        <span className="sn-error" id={descId} role="alert">
          {error}
        </span>
      ) : (
        hint && (
          <span className="sn-hint" id={descId}>
            {hint}
          </span>
        )
      )}
    </div>
  );
}

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement>, FieldShell {}

// Textarea（D2.2）：纵向 resize；其余同 Input。
export function Textarea({ label, hint, error, id, className, ...rest }: TextareaProps) {
  const auto = useId();
  const fid = id ?? auto;
  const descId = error ? `${fid}-err` : hint ? `${fid}-hint` : undefined;
  return (
    <div className="sn-field">
      {label && (
        <label className="sn-label" htmlFor={fid}>
          {label}
        </label>
      )}
      <textarea
        id={fid}
        className={cx("sn-input", "sn-focus", className)}
        aria-invalid={error ? true : undefined}
        aria-describedby={descId}
        {...rest}
      />
      {error ? (
        <span className="sn-error" id={descId} role="alert">
          {error}
        </span>
      ) : (
        hint && (
          <span className="sn-hint" id={descId}>
            {hint}
          </span>
        )
      )}
    </div>
  );
}
