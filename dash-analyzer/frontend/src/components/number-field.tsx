"use client";
import { useEffect, useState } from "react";
export function NumberField({
  label,
  value,
  min,
  max,
  step,
  help,
  onCommit,
  disabled = false,
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  help?: string;
  disabled?: boolean;
  onCommit: (n: number) => void;
}) {
  const [draft, setDraft] = useState(String(value));
  const [error, setError] = useState("");
  useEffect(() => {
    setDraft(String(value));
    setError("");
  }, [value]);
  function commit() {
    const n = Number(draft);
    if (!draft.trim() || !Number.isFinite(n) || n < min || n > max) {
      setError(`Enter a number between ${min} and ${max}.`);
      return;
    }
    setError("");
    onCommit(n);
  }
  return (
    <label className="field">
      {label}
      <input
        type="number"
        aria-label={label}
        disabled={disabled}
        value={draft}
        min={min}
        max={max}
        step={step}
        aria-invalid={!!error}
        onChange={(e) => {
          setDraft(e.target.value);
          setError("");
        }}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur();
        }}
      />
      {error ? (
        <small className="source-warning" role="alert">
          {error}
        </small>
      ) : (
        help && <small>{help}</small>
      )}
    </label>
  );
}
