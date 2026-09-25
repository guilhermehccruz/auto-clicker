import type { Progress } from "../api/types";
import { formatElapsed } from "../lib/format";

// Telemetry is the live status segment of the bottom bar (DESIGN §5.2).
export function Telemetry({ progress }: { progress: Progress }) {
  const active = progress.state !== "idle";
  const dot = progress.state === "running" ? "bg-emerald-500" : progress.state === "paused" ? "bg-amber-400" : "bg-neutral-600";
  return (
    <div className="mono flex min-w-0 items-center gap-2 text-[12px] text-neutral-400">
      <span className={`inline-block h-2 w-2 rounded-full ${dot}`} />
      <span className="uppercase text-neutral-200">{progress.state}</span>
      {active && (
        <>
          <span className="truncate text-neutral-300">{progress.profileName}</span>
          <span>
            {progress.moduleIndex}/{progress.moduleTotal}
          </span>
          <span>loop {progress.loopIteration}</span>
          <span>{formatElapsed(progress.elapsedMs)}</span>
          <span>clicks {progress.clicks}</span>
          {progress.stopReason && <span className="text-amber-400">{progress.stopReason}</span>}
        </>
      )}
    </div>
  );
}
