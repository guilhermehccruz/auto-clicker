import type { Diagnostics } from "../api/types";

// DiagnosticsPanel is support-load-bearing: the first question is always
// "which backend is it using?" (DESIGN §5.5).
export function DiagnosticsPanel({
  diagnostics,
  onOpenLog,
}: {
  diagnostics: Diagnostics | null;
  onOpenLog?: () => void;
}) {
  if (!diagnostics) return null;
  const d = diagnostics;
  const rows: [string, string][] = [
    ["Version", d.version],
    ["robotgo pin", d.robotgoPin],
    ["App id", d.appId],
    ["Backend", d.backend + (d.fallbackReason ? ` (fallback: ${d.fallbackReason})` : "")],
    ["Cursor read", d.canReadCursor ? "yes" : "no"],
    ["Absolute move", d.canMoveAbsolute ? "yes" : "no"],
    ["Desktop", `${d.desktopWidth}×${d.desktopHeight}`],
    ["Displays", d.displays.map((x) => `${x.W}×${x.H}@${x.X},${x.Y}`).join(", ") || "—"],
    ["Hotkeys", d.hotkeySource + (d.hotkeyError ? ` (${d.hotkeyError})` : "")],
    ["Profiles", d.profilesDir],
    ["Logs", d.logDir],
  ];
  const copy = () => {
    const text = rows.map(([k, v]) => `${k}: ${v}`).join("\n");
    void navigator.clipboard?.writeText(text);
  };
  return (
    <section className="rounded border border-neutral-700 bg-neutral-900 p-3">
      <div className="mb-2 flex items-center justify-between">
        <h3 className="text-[12px] font-medium uppercase tracking-wide text-neutral-400">Diagnostics</h3>
        <span className="flex gap-2">
          {onOpenLog && (
            <button type="button" onClick={onOpenLog} className="rounded border border-neutral-700 px-2 py-0.5 text-[11px] hover:bg-neutral-800">
              Open log folder
            </button>
          )}
          <button type="button" onClick={copy} className="rounded border border-neutral-700 px-2 py-0.5 text-[11px] hover:bg-neutral-800">
            Copy diagnostics
          </button>
        </span>
      </div>
      <dl className="grid grid-cols-[8rem_1fr] gap-x-3 gap-y-1 text-[12px]">
        {rows.map(([k, v]) => (
          <div key={k} className="contents">
            <dt className="text-neutral-500">{k}</dt>
            <dd className="mono truncate text-neutral-300" title={v}>{v}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
