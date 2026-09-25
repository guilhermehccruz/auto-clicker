import type { Issue } from "../api/types";

// WarningBanner renders the non-blocking advisories: display-geometry mismatch
// and off-screen coordinates (DESIGN §3.5, §5.4).
export function WarningBanner({
  issues,
  onRepick,
  onUpdateSnapshot,
}: {
  issues: Issue[];
  onRepick?: () => void;
  onUpdateSnapshot?: () => void;
}) {
  const display = issues.find((i) => i.field === "display");
  if (!display) return null;
  return (
    <div className="mb-2 flex items-center justify-between gap-3 rounded border border-amber-400/40 bg-amber-400/10 px-3 py-2 text-[12px] text-amber-200">
      <span>{display.message}</span>
      <span className="flex gap-2">
        {onUpdateSnapshot && (
          <button type="button" onClick={onUpdateSnapshot} className="underline hover:text-amber-100">
            Update to current desktop
          </button>
        )}
        {onRepick && (
          <button type="button" onClick={onRepick} className="underline hover:text-amber-100">
            Re-pick
          </button>
        )}
      </span>
    </div>
  );
}
