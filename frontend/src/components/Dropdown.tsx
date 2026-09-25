import { useEffect, useRef, useState, type ReactNode } from "react";

// Dropdown is a controlled menu: it closes when an item is chosen, when the
// user clicks outside, and on Escape.
export function Dropdown({
  label,
  ariaLabel,
  disabled,
  className,
  children,
}: {
  label: ReactNode;
  ariaLabel: string;
  disabled?: boolean;
  className?: string;
  children: ReactNode;
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
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className={className ?? "rounded px-1 text-neutral-400 enabled:hover:text-neutral-200 disabled:opacity-40"}
      >
        {label}
      </button>
      {open && (
        // Any click inside the panel (i.e. choosing an item) closes it.
        <div
          role="menu"
          onClick={() => setOpen(false)}
          className="absolute right-0 z-20 mt-1 w-40 rounded border border-neutral-700 bg-neutral-800 py-1 text-[12px] shadow-lg"
        >
          {children}
        </div>
      )}
    </div>
  );
}

// MenuItem is a single row inside a Dropdown.
export function MenuItem({
  children,
  onClick,
  disabled,
  danger,
}: {
  children: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      role="menuitem"
      disabled={disabled}
      onClick={onClick}
      className={`block w-full px-3 py-1 text-left enabled:hover:bg-neutral-700 disabled:opacity-40 ${
        danger ? "text-rose-400" : "text-neutral-200"
      }`}
    >
      {children}
    </button>
  );
}
