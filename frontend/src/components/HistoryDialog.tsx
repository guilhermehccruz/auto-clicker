import { useEffect, useState } from "react";
import type { RunRecord, Settings } from "../api/types";
import { formatElapsed, numberInput } from "../lib/format";

// HistoryDialog is the run-history modal (DESIGN §5.5). It owns the Clear
// action and the retention setting, keeping them out of Settings.
export function HistoryDialog({
  settings,
  onSave,
  onLoadHistory,
  onClearHistory,
  onClose,
}: {
  settings: Settings;
  onSave: (s: Settings) => void;
  onLoadHistory: () => Promise<RunRecord[]>;
  onClearHistory: () => Promise<void>;
  onClose: () => void;
}) {
  const [runs, setRuns] = useState<RunRecord[] | null>(null);
  useEffect(() => {
    let alive = true;
    onLoadHistory()
      .then((r) => alive && setRuns(r))
      .catch(() => alive && setRuns([]));
    return () => {
      alive = false;
    };
  }, [onLoadHistory]);

  const clear = () => {
    void onClearHistory().then(() => setRuns([]));
  };

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/50" onMouseDown={onClose}>
      <div
        className="flex max-h-[80vh] w-[42rem] flex-col rounded-lg border border-neutral-700 bg-neutral-900 p-4"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-neutral-100">History</h2>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={clear}
              disabled={!runs || runs.length === 0}
              className="rounded border border-neutral-700 px-2 py-1 text-[12px] enabled:hover:bg-neutral-800 disabled:opacity-40"
            >
              Clear
            </button>
            <button type="button" onClick={onClose} className="rounded px-2 py-1 text-neutral-400 hover:bg-neutral-800">Esc</button>
          </div>
        </div>

        <label className="mb-3 flex items-center gap-2 text-[12px] text-neutral-400">
          Keep last
          <input
            type="number"
            min={1}
            value={settings.runHistoryLimit}
            onChange={(e) => onSave({ ...settings, runHistoryLimit: numberInput(e) })}
            className="mono w-20 rounded border border-neutral-700 bg-neutral-800 px-2 py-0.5"
          />
          runs
        </label>

        <div className="min-h-0 flex-1 overflow-y-auto">
          {runs && runs.length > 0 ? (
            <ul className="flex flex-col divide-y divide-neutral-800 text-[12px]">
              {runs.map((r, i) => (
                <li
                  key={i}
                  className="grid grid-cols-[10.5rem_1fr_5rem_6rem] items-center gap-2 py-1"
                  title={r.error || undefined}
                >
                  <span className="mono text-[11px] text-neutral-500">
                    {new Date(r.startedAt).toLocaleString()}
                  </span>
                  <span className="truncate text-neutral-300">{r.profile || "—"}</span>
                  <span className={r.stopReason === "error" ? "text-rose-400" : "text-neutral-400"}>
                    {r.stopReason}
                  </span>
                  <span className="mono text-right text-neutral-400">
                    {r.clicks} · {formatElapsed(r.elapsedMs)}
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="py-6 text-center text-[12px] text-neutral-500">No runs recorded yet.</p>
          )}
        </div>
      </div>
    </div>
  );
}
