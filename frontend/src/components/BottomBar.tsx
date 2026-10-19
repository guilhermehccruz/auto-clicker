import type { Hotkeys, Loop, LoopMode, Profile, Progress, RunState } from "../api/types";
import { numberInput } from "../lib/format";
import { RunControls } from "./RunControls";
import { Select } from "./Select";
import { Telemetry } from "./Telemetry";

const LOOP_MODES: readonly LoopMode[] = ["once", "count", "forever"];

// BottomBar: loop + profile controls (idle only), telemetry and run controls.
export function BottomBar({
  loop,
  onLoopChange,
  profile,
  onChangeMaxRuntime,
  onChangeHumanize,
  locked,
  progress,
  canRun,
  blockReason,
  hotkeys,
  onRun,
  onStop,
  onPause,
  onResume,
}: {
  loop: Loop;
  onLoopChange: (l: Loop) => void;
  profile: Profile | null;
  onChangeMaxRuntime: (sec: number) => void;
  onChangeHumanize: (v: boolean) => void;
  locked: boolean;
  progress: Progress;
  canRun: boolean;
  blockReason?: string;
  hotkeys?: Hotkeys;
  onRun: () => void;
  onStop: () => void;
  onPause: () => void;
  onResume: () => void;
}) {
  const maxMinutes = profile ? Math.round(profile.failsafe.maxRuntimeSec / 60) : 0;
  return (
    <footer className="flex flex-col border-t border-neutral-800 bg-neutral-900">
      <div className="flex items-center justify-between gap-3 px-3 py-2">
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
              onChange={(e) => onLoopChange({ ...loop, count: numberInput(e) })}
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
                onLoopChange({ ...loop, loopDelay: { ...loop.loopDelay, min: numberInput(e) } })
              }
              className="mono w-20 rounded border border-neutral-700 bg-neutral-800 px-1.5 py-1 text-neutral-100 disabled:opacity-50"
            />
            –
            <input
              type="number"
              min={0}
              value={loop.loopDelay.max}
              disabled={locked}
              onChange={(e) =>
                onLoopChange({ ...loop, loopDelay: { ...loop.loopDelay, max: numberInput(e) } })
              }
              className="mono w-20 rounded border border-neutral-700 bg-neutral-800 px-1.5 py-1 text-neutral-100 disabled:opacity-50"
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
        hotkeys={hotkeys}
        onRun={onRun}
        onStop={onStop}
        onPause={onPause}
        onResume={onResume}
      />
      </div>

      <div className="flex flex-wrap items-center gap-4 border-t border-neutral-800 px-3 py-1.5 text-[12px] text-neutral-400">
        <span className="text-neutral-500">This profile</span>
        <label className="flex items-center gap-1" title="Max runtime for the open profile (0 = unlimited)">
          max runtime
          <input
            type="number"
            min={0}
            value={maxMinutes}
            disabled={!profile || locked}
            onChange={(e) => onChangeMaxRuntime(numberInput(e) * 60)}
            className="mono w-20 rounded border border-neutral-700 bg-neutral-800 px-1.5 py-0.5 text-neutral-100 disabled:opacity-50"
          />
          min
        </label>
        <label className="flex items-center gap-2" title="Randomize every fixed delay by ±25%">
          <input
            type="checkbox"
            checked={profile?.humanize ?? false}
            disabled={!profile || locked}
            onChange={(e) => onChangeHumanize(e.target.checked)}
          />
          Humanize delays (±25%)
        </label>
        {!profile && <span className="text-neutral-600">no profile open</span>}
      </div>
    </footer>
  );
}
