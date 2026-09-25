import type { Loop, LoopMode, Progress, RunState } from "../api/types";
import { RunControls } from "./RunControls";
import { Select } from "./Select";
import { Telemetry } from "./Telemetry";

const LOOP_MODES: readonly LoopMode[] = ["once", "count", "forever"];

// BottomBar: loop-mode selector (idle only), telemetry and run controls.
export function BottomBar({
  loop,
  onLoopChange,
  locked,
  progress,
  canRun,
  blockReason,
  onRun,
  onStop,
  onPause,
  onResume,
  onPanic,
}: {
  loop: Loop;
  onLoopChange: (l: Loop) => void;
  locked: boolean;
  progress: Progress;
  canRun: boolean;
  blockReason?: string;
  onRun: () => void;
  onStop: () => void;
  onPause: () => void;
  onResume: () => void;
  onPanic: () => void;
}) {
  return (
    <footer className="flex items-center justify-between gap-3 border-t border-neutral-800 bg-neutral-900 px-3 py-2">
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2 text-[12px] text-neutral-400">
          Loop
          <Select
            up
            ariaLabel="Loop mode"
            value={loop.mode}
            options={LOOP_MODES}
            disabled={locked}
            onChange={(mode) => onLoopChange({ ...loop, mode })}
          />
        </div>
        {loop.mode === "count" && (
          <label className="flex items-center gap-1 text-[12px] text-neutral-400">
            passes
            <input
              type="number"
              min={1}
              value={loop.count}
              disabled={locked}
              onChange={(e) => onLoopChange({ ...loop, count: Number(e.target.value) })}
              className="mono w-16 rounded border border-neutral-700 bg-neutral-800 px-2 py-1 text-neutral-100 disabled:opacity-50"
            />
          </label>
        )}
        {loop.mode !== "once" && (
          <label className="flex items-center gap-1 text-[12px] text-neutral-400" title="Random delay between passes (min–max, inclusive)">
            loop delay
            <input
              type="number"
              min={0}
              value={loop.loopDelay.min}
              disabled={locked}
              onChange={(e) =>
                onLoopChange({ ...loop, loopDelay: { ...loop.loopDelay, min: Number(e.target.value) } })
              }
              className="mono w-16 rounded border border-neutral-700 bg-neutral-800 px-1.5 py-1 text-neutral-100 disabled:opacity-50"
            />
            –
            <input
              type="number"
              min={0}
              value={loop.loopDelay.max}
              disabled={locked}
              onChange={(e) =>
                onLoopChange({ ...loop, loopDelay: { ...loop.loopDelay, max: Number(e.target.value) } })
              }
              className="mono w-16 rounded border border-neutral-700 bg-neutral-800 px-1.5 py-1 text-neutral-100 disabled:opacity-50"
            />
            ms
          </label>
        )}
      </div>

      <Telemetry progress={progress} />

      <RunControls
        state={progress.state as RunState}
        canRun={canRun}
        blockReason={blockReason}
        onRun={onRun}
        onStop={onStop}
        onPause={onPause}
        onResume={onResume}
        onPanic={onPanic}
      />
    </footer>
  );
}
