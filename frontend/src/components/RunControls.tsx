import type { RunState } from "../api/types";

// RunControls mirrors the global hotkeys, labelled with their key (DESIGN §5.2).
export function RunControls({
  state,
  canRun,
  blockReason,
  onRun,
  onStop,
  onPause,
  onResume,
  onPanic,
}: {
  state: RunState;
  canRun: boolean;
  blockReason?: string;
  onRun: () => void;
  onStop: () => void;
  onPause: () => void;
  onResume: () => void;
  onPanic: () => void;
}) {
  const idle = state === "idle";
  const paused = state === "paused";
  return (
    <div className="flex items-center gap-2">
      {idle ? (
        <button
          type="button"
          onClick={onRun}
          disabled={!canRun}
          title={canRun ? "Run (F8)" : blockReason}
          className="rounded bg-emerald-500 px-3 py-1.5 font-medium text-neutral-950 enabled:hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          ▶ Run <span className="mono text-[11px] opacity-70">F8</span>
        </button>
      ) : (
        <button
          type="button"
          onClick={onStop}
          className="rounded bg-rose-600 px-3 py-1.5 font-medium text-white hover:bg-rose-500 focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          ⏹ Stop <span className="mono text-[11px] opacity-70">F8</span>
        </button>
      )}
      <button
        type="button"
        onClick={paused ? onResume : onPause}
        disabled={idle}
        className="rounded border border-neutral-700 px-3 py-1.5 text-neutral-200 enabled:hover:bg-neutral-800 disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-indigo-500"
      >
        {paused ? "▶ Resume" : "⏸ Pause"} <span className="mono text-[11px] opacity-70">F9</span>
      </button>
      <button
        type="button"
        onClick={onPanic}
        disabled={idle}
        title="Panic (Ctrl+Shift+F12)"
        className="rounded border border-rose-600/50 px-2 py-1.5 text-rose-400 enabled:hover:bg-rose-600/10 disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-indigo-500"
      >
        Panic
      </button>
    </div>
  );
}
