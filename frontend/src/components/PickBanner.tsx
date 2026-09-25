import { useEffect, useState } from "react";

// PickBanner is the in-app pick countdown (DESIGN §5.7). The actual capture is
// driven by the Go timer; this is the visible countdown and the manual controls.
export function PickBanner({
  seconds,
  onCancel,
}: {
  seconds: number;
  onCancel: () => void;
}) {
  const [remaining, setRemaining] = useState(seconds);

  useEffect(() => {
    const start = performance.now();
    const id = setInterval(() => {
      setRemaining(Math.max(0, seconds - (performance.now() - start) / 1000));
    }, 100);
    return () => clearInterval(id);
  }, [seconds]);

  return (
    <div className="fixed left-1/2 top-3 z-50 -translate-x-1/2 rounded-lg border border-indigo-500/50 bg-neutral-900/95 px-4 py-3 shadow-lg">
      <div className="flex items-center gap-4">
        <div className="mono w-12 text-center text-2xl text-indigo-300">{remaining.toFixed(1)}</div>
        <div>
          <div className="text-[13px] text-neutral-100">
            Move the pointer to the target — capturing automatically.
          </div>
          <div className="text-[11px] text-neutral-400">
            Enter captures now · Esc cancels
          </div>
        </div>
        <button
          type="button"
          onClick={onCancel}
          className="rounded border border-neutral-600 px-3 py-1 text-[12px] text-neutral-200 hover:bg-neutral-800"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
