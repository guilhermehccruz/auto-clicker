import type { Capabilities, Diagnostics, Failsafe, Profile, Settings } from "../api/types";
import { DiagnosticsPanel } from "./DiagnosticsPanel";

// SettingsDialog: hotkeys (read-only in v1), safety for the open profile,
// profiles folder, advanced, diagnostics (DESIGN §5.5).
export function SettingsDialog({
  settings,
  diagnostics,
  capabilities,
  profile,
  onSave,
  onChangeFailsafe,
  onOpenFolder,
  onOpenLog,
  onClose,
}: {
  settings: Settings;
  diagnostics: Diagnostics | null;
  capabilities: Capabilities | null;
  profile: Profile | null;
  onSave: (s: Settings) => void;
  onChangeFailsafe: (f: Failsafe) => void;
  onOpenFolder: () => void;
  onOpenLog: () => void;
  onClose: () => void;
}) {
  const canReadCursor = capabilities?.CanReadCursor ?? false;
  const failsafe = profile?.failsafe;

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/50" onMouseDown={onClose}>
      <div
        className="max-h-[85vh] w-[36rem] overflow-y-auto rounded-lg border border-neutral-700 bg-neutral-900 p-4"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-neutral-100">Settings</h2>
          <button type="button" onClick={onClose} className="rounded px-2 py-1 text-neutral-400 hover:bg-neutral-800">Esc</button>
        </div>

        <section className="mb-4">
          <h3 className="mb-1 text-[12px] uppercase tracking-wide text-neutral-500">Hotkeys</h3>
          <ul className="text-[12px] text-neutral-300">
            <li className="mono">F8 · Run / Stop</li>
            <li className="mono">F9 · Pause / Resume</li>
            <li className="mono">Ctrl+Shift+F12 · Panic</li>
          </ul>
          <p className="mt-1 text-[11px] text-neutral-500">
            Source: {diagnostics?.hotkeySource ?? "—"}. Rebinding is Phase 2.
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
                  onChange={(e) => onChangeFailsafe({ ...failsafe, maxRuntimeSec: Number(e.target.value) * 60 })}
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
              onChange={(e) => onSave({ ...settings, defaultMaxRuntimeSec: Number(e.target.value) * 60 })}
              className="mono w-20 rounded border border-neutral-700 bg-neutral-800 px-2 py-0.5"
            />
          </label>
        </section>

        <DiagnosticsPanel diagnostics={diagnostics} onOpenLog={onOpenLog} />
      </div>
    </div>
  );
}
