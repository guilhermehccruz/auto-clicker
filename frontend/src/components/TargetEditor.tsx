import type { Target } from "../api/types";

// TargetEditor edits a module's target kind and coordinates (DESIGN §5.3).
export function TargetEditor({
  target,
  disabled,
  onChange,
  onPick,
}: {
  target: Target;
  disabled: boolean;
  onChange: (t: Target) => void;
  onPick: () => void;
}) {
  const absolute = target.kind === "absolute";
  return (
    <div className="flex flex-wrap items-center gap-3 text-[12px]">
      <label className="flex items-center gap-1.5">
        <input
          type="radio"
          checked={absolute}
          disabled={disabled}
          onChange={() => onChange({ kind: "absolute", x: target.x ?? 0, y: target.y ?? 0 })}
        />
        absolute
      </label>
      <label className="flex items-center gap-1.5">
        <input
          type="radio"
          checked={!absolute}
          disabled={disabled}
          onChange={() => onChange({ kind: "cursor" })}
        />
        cursor
      </label>

      {absolute && (
        <>
          <span className="mono flex items-center gap-1 text-neutral-400">
            x
            <input
              type="number"
              value={target.x ?? 0}
              disabled={disabled}
              onChange={(e) => onChange({ kind: "absolute", x: Number(e.target.value), y: target.y ?? 0 })}
              className="w-20 rounded border border-neutral-700 bg-neutral-800 px-2 py-0.5 text-neutral-100 disabled:opacity-50"
            />
            y
            <input
              type="number"
              value={target.y ?? 0}
              disabled={disabled}
              onChange={(e) => onChange({ kind: "absolute", x: target.x ?? 0, y: Number(e.target.value) })}
              className="w-20 rounded border border-neutral-700 bg-neutral-800 px-2 py-0.5 text-neutral-100 disabled:opacity-50"
            />
          </span>
          <button
            type="button"
            onClick={onPick}
            disabled={disabled}
            className="rounded border border-neutral-700 px-2 py-0.5 text-neutral-200 enabled:hover:bg-neutral-800 disabled:opacity-40"
          >
            pick position ⌖
          </button>
        </>
      )}
    </div>
  );
}
