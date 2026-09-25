import { useEffect, useRef, useState } from "react";

// Select is a dark-styled replacement for the native <select>, which WebKitGTK
// renders light regardless of CSS. It closes on choose, outside click and Esc.
export function Select<T extends string>({
  value,
  options,
  onChange,
  disabled,
  ariaLabel,
  up,
}: {
  value: T;
  options: readonly T[];
  onChange: (v: T) => void;
  disabled?: boolean;
  ariaLabel: string;
  up?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        aria-label={ariaLabel}
        aria-haspopup="listbox"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className="flex min-w-24 items-center justify-between gap-2 rounded border border-neutral-700 bg-neutral-800 px-2 py-1 text-neutral-100 disabled:opacity-50 focus-visible:ring-2 focus-visible:ring-indigo-500"
      >
        <span>{value}</span>
        <span className="text-neutral-400">▾</span>
      </button>
      {open && (
        <ul
          role="listbox"
          className={`absolute left-0 z-20 min-w-full rounded border border-neutral-700 bg-neutral-800 py-1 text-[12px] shadow-lg ${
            up ? "bottom-full mb-1" : "top-full mt-1"
          }`}
        >
          {options.map((o) => (
            <li key={o}>
              <button
                type="button"
                role="option"
                aria-selected={o === value}
                onClick={() => {
                  onChange(o);
                  setOpen(false);
                }}
                className={`block w-full px-3 py-1 text-left hover:bg-neutral-700 ${
                  o === value ? "bg-neutral-700 text-white" : "text-neutral-200"
                }`}
              >
                {o}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
