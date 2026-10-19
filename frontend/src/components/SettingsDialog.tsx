import { useEffect, useState } from "react";
import type {
  Capabilities,
  Diagnostics,
  Failsafe,
  Profile,
  RunRecord,
  Settings,
  UpdateInfo,
} from "../api/types";
import { formatElapsed, numberInput } from "../lib/format";
import { DiagnosticsPanel } from "./DiagnosticsPanel";

// SettingsDialog: hotkeys (read-only in v1), safety for the open profile,
// profiles folder, advanced, updates, run history, diagnostics (DESIGN §5.5).
export function SettingsDialog({
  settings,
  diagnostics,
  capabilities,
  profile,
  updating,
  onSave,
  onChangeFailsafe,
  onOpenFolder,
  onOpenLog,
  onCheckUpdate,
  onInstallUpdate,
  onLoadHistory,
  onClearHistory,
  onClose,
}: {
  settings: Settings;
  diagnostics: Diagnostics | null;
  capabilities: Capabilities | null;
  profile: Profile | null;
  updating: boolean;
  onSave: (s: Settings) => void;
  onChangeFailsafe: (f: Failsafe) => void;
  onOpenFolder: () => void;
  onOpenLog: () => void;
  onCheckUpdate: () => Promise<UpdateInfo>;
  onInstallUpdate: () => void;
  onLoadHistory: () => Promise<RunRecord[]>;
  onClearHistory: () => Promise<void>;
  onClose: () => void;
}) {
  const canReadCursor = capabilities?.CanReadCursor ?? false;
  const failsafe = profile?.failsafe;

  // Manual update check.
  const [checking, setChecking] = useState(false);
  const [checkResult, setCheckResult] = useState<UpdateInfo | null>(null);
  const [checkError, setCheckError] = useState<string | null>(null);
  const check = () => {
    setChecking(true);
    setCheckError(null);
    setCheckResult(null);
    onCheckUpdate()
      .then(setCheckResult)
      .catch((e) => setCheckError(String(e)))
      .finally(() => setChecking(false));
  };

  // Run history, loaded once when the dialog opens.
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
  const clearHistory = () => {
    void onClearHistory().then(() => setRuns([]));
  };

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/50" onMouseDown={onClose}>
      <div
        className="max-h-[85vh] w-[36rem] overflow-y-auto rounded-lg border border-neutral-700 bg-neutral-900 p-4"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-neutral-100">
            Settings{" "}
            <span className="text-[11px] font-normal text-neutral-500">
              {diagnostics?.version ?? "…"}
            </span>
          </h2>
          <button type="button" onClick={onClose} className="rounded px-2 py-1 text-neutral-400 hover:bg-neutral-800">Esc</button>
        </div>

        <section className="mb-4">
          <h3 className="mb-1 text-[12px] uppercase tracking-wide text-neutral-500">Hotkeys</h3>
          <ul className="text-[12px] text-neutral-300">
            <li className="mono">F8 · Run / Stop</li>
            <li className="mono">F9 · Pause / Resume</li>
          </ul>
          <p className="mt-1 text-[11px] text-neutral-500">
            Source: {diagnostics?.hotkeySource ?? "—"}. Rebinding is planned.
          </p>
        </section>

        <section className="mb-4">
          <h3 className="mb-1 text-[12px] uppercase tracking-wide text-neutral-500">Safety (this profile)</h3>
          {failsafe ? (
            <div className="flex flex-col gap-2 text-[12px] text-neutral-300">
              <label className="flex items-center gap-2">
                Max runtime (minutes, 0 = unlimited)
                <input
                  type="number"
                  min={0}
                  value={Math.round(failsafe.maxRuntimeSec / 60)}
                  onChange={(e) => onChangeFailsafe({ ...failsafe, maxRuntimeSec: numberInput(e) * 60 })}
                  className="mono w-20 rounded border border-neutral-700 bg-neutral-800 px-2 py-0.5"
                />
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={failsafe.mouseToCornerStops}
                  disabled={!canReadCursor}
                  title={canReadCursor ? "" : "needs a cursor read, unavailable with libei"}
                  onChange={(e) => onChangeFailsafe({ ...failsafe, mouseToCornerStops: e.target.checked })}
                />
                Stop when the pointer is dragged into a screen corner
              </label>
            </div>
          ) : (
            <p className="text-[12px] text-neutral-500">No profile open.</p>
          )}
        </section>

        <section className="mb-4">
          <h3 className="mb-1 text-[12px] uppercase tracking-wide text-neutral-500">Profiles</h3>
          <p className="mono mb-2 truncate text-[12px] text-neutral-300" title={settings.profilesDir}>
            {settings.profilesDir || diagnostics?.profilesDir}
          </p>
          <button type="button" onClick={onOpenFolder} className="rounded border border-neutral-700 px-2 py-1 text-[12px] hover:bg-neutral-800">
            Open folder
          </button>
        </section>

        <section className="mb-4">
          <h3 className="mb-1 text-[12px] uppercase tracking-wide text-neutral-500">Advanced</h3>
          <label className="flex items-center gap-2 text-[12px] text-neutral-300">
            Default max runtime for new profiles (minutes)
            <input
              type="number"
              min={0}
              value={Math.round(settings.defaultMaxRuntimeSec / 60)}
              onChange={(e) => onSave({ ...settings, defaultMaxRuntimeSec: numberInput(e) * 60 })}
              className="mono w-20 rounded border border-neutral-700 bg-neutral-800 px-2 py-0.5"
            />
          </label>
        </section>

        <section className="mb-4">
          <h3 className="mb-1 text-[12px] uppercase tracking-wide text-neutral-500">Updates</h3>
          <div className="flex flex-wrap items-center gap-2 text-[12px] text-neutral-300">
            <button
              type="button"
              onClick={check}
              disabled={checking}
              className="rounded border border-neutral-700 px-2 py-1 enabled:hover:bg-neutral-800 disabled:opacity-50"
            >
              {checking ? "Checking…" : "Check for updates"}
            </button>
            {checkResult?.available && (
              <>
                <span className="text-indigo-200">v{checkResult.version} available</span>
                <button
                  type="button"
                  onClick={onInstallUpdate}
                  disabled={updating}
                  className="rounded bg-indigo-500 px-2 py-1 font-medium text-neutral-950 enabled:hover:bg-indigo-400 disabled:opacity-60"
                >
                  {updating ? "Updating…" : "Update"}
                </button>
              </>
            )}
            {checkResult && !checkResult.available && <span className="text-neutral-500">Up to date</span>}
            {checkError && <span className="text-rose-400">{checkError}</span>}
          </div>
        </section>

        <section className="mb-4">
          <div className="mb-1 flex items-center justify-between">
            <h3 className="text-[12px] uppercase tracking-wide text-neutral-500">History</h3>
            <button type="button" onClick={clearHistory} className="rounded border border-neutral-700 px-2 py-0.5 text-[11px] hover:bg-neutral-800">
              Clear
            </button>
          </div>
          <label className="mb-2 flex items-center gap-2 text-[12px] text-neutral-400">
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
          {runs && runs.length > 0 ? (
            <ul className="flex flex-col gap-1 text-[12px]">
              {runs.slice(0, 20).map((r, i) => (
                <li key={i} className="flex items-center justify-between gap-3" title={r.startedAt}>
                  <span className="truncate text-neutral-300">{r.profile || "—"}</span>
                  <span className="mono shrink-0 text-[11px] text-neutral-500">
                    {r.stopReason} · {r.clicks} clicks · {formatElapsed(r.elapsedMs)}
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-[12px] text-neutral-500">No runs recorded yet.</p>
          )}
        </section>

        <DiagnosticsPanel diagnostics={diagnostics} onOpenLog={onOpenLog} />
      </div>
    </div>
  );
}
