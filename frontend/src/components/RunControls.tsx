import type { Hotkeys, RunState } from "../api/types";

// RunControls mirrors the global hotkeys, labelled with their key (DESIGN §5.2).
export function RunControls({
  state,
  canRun,
  blockReason,
  hotkeys,
  onRun,
  onStop,
  onPause,
  onResume,
}: {
  state: RunState;
  canRun: boolean;
  blockReason?: string;
  hotkeys?: Hotkeys;
  onRun: () => void;
  onStop: () => void;
  onPause: () => void;
  onResume: () => void;
}) {
  const idle = state === "idle";
  const paused = state === "paused";
  const runKey = hotkeys?.run ?? "F8";
  const pauseKey = hotkeys?.pause ?? "F9";
  return (
    <div className="flex items-center gap-2">
      {idle ? (
        <button
          type="button"
          onClick={onRun}
          disabled={!canRun}
          title={canRun ? `Run (${runKey})` : blockReason}
          className="rounded bg-emerald-500 px-3 py-1.5 font-medium text-neutral-950 enabled:hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          ▶ Run <span className="mono text-[11px] opacity-70">{runKey}</span>
        </button>
      ) : (
        <button
          type="button"
          onClick={onStop}
          className="rounded bg-rose-600 px-3 py-1.5 font-medium text-white hover:bg-rose-500 focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          ⏹ Stop <span className="mono text-[11px] opacity-70">{runKey}</span>
        </button>
      )}
      <button
        type="button"
        onClick={paused ? onResume : onPause}
        disabled={idle}
        className="rounded border border-neutral-700 px-3 py-1.5 text-neutral-200 enabled:hover:bg-neutral-800 disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-indigo-500"
      >
        {paused ? "▶ Resume" : "⏸ Pause"} <span className="mono text-[11px] opacity-70">{pauseKey}</span>
      </button>
    </div>
  );
}
