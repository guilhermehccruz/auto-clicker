import { useEffect, useMemo, useRef, useState } from "react";
import { createApi } from "./api/bindings";
import type { Failsafe, Module } from "./api/types";
import { BottomBar } from "./components/BottomBar";
import { EmptyState } from "./components/EmptyState";
import { ModuleList } from "./components/ModuleList";
import { PickBanner } from "./components/PickBanner";
import { SettingsDialog } from "./components/SettingsDialog";
import { Sidebar } from "./components/Sidebar";
import { UndoToast } from "./components/UndoToast";
import { WarningBanner } from "./components/WarningBanner";
import { hasBlocking } from "./lib/validate";
import { label } from "./lib/format";
import { UNGROUPED, useProfiles } from "./state/useProfiles";
import { useRunner } from "./state/useRunner";
import { useSettings } from "./state/useSettings";

export default function App() {
  const api = useMemo(createApi, []);
  const profiles = useProfiles(api);
  const runner = useRunner(api);
  const { settings, capabilities, diagnostics, save: saveSettings } = useSettings(api);

  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [pickActive, setPickActive] = useState(false);
  const [undo, setUndo] = useState<{ module: Module; containerId: string; index: number; name: string } | null>(null);

  const profile = profiles.profile;
  const locked = runner.running || runner.paused;
  const blocking = profiles.issues.find((i) => i.level === "blocking");
  const canRun = !!profile && !hasBlocking(profiles.issues);
  const desktop = { width: profile?.display.width ?? 0, height: profile?.display.height ?? 0 };

  const runOrStop = () => {
    if (runner.progress.state === "idle") {
      if (canRun && profiles.selectedId) void runner.run(profiles.selectedId);
    } else {
      void runner.stop();
    }
  };

  // In-window shortcuts (DESIGN §5.8). Global hotkeys arrive through the Go side.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement;
      const typing = ["INPUT", "TEXTAREA", "SELECT"].includes(el.tagName) || el.isContentEditable;
      if (pickActive) {
        // While picking, Enter captures now and Esc cancels.
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          void api.confirmPick();
        } else if (e.key === "Escape") {
          e.preventDefault();
          void api.cancelPick();
        }
        return;
      }
      if (e.key === "F8") {
        e.preventDefault();
        runOrStop();
      } else if (e.key === "F9") {
        e.preventDefault();
        if (runner.paused) void runner.resume();
        else if (runner.running) void runner.pause();
      } else if (e.key === "Escape") {
        setShowSettings(false);
      } else if (!typing && e.key === "Delete" && expandedId && !locked) {
        deleteModule(expandedId);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const deleteModule = (id: string) => {
    if (!profile) return;
    let containerId = UNGROUPED;
    let index = profile.modules.findIndex((m) => m.id === id);
    let mod = index >= 0 ? profile.modules[index] : undefined;
    if (!mod) {
      for (const g of profile.groups) {
        const j = g.modules.findIndex((m) => m.id === id);
        if (j >= 0) {
          containerId = g.id;
          index = j;
          mod = g.modules[j];
          break;
        }
      }
    }
    if (!mod) return;
    profiles.removeModule(id);
    setUndo({ module: mod, containerId, index, name: label(mod) });
    if (expandedId === id) setExpandedId(null);
  };

  // onPick opens the shell's picker (a small always-on-top toast that reads the
  // real cursor). The result arrives asynchronously via `pick:done`.
  const pendingPick = useRef<string | null>(null);
  const diagnosticsRef = useRef(diagnostics);
  diagnosticsRef.current = diagnostics;

  // Refresh the profile's display snapshot to the current desktop (DESIGN §3.5).
  const snapshotDisplay = () => {
    const d = diagnosticsRef.current;
    if (!d || d.desktopWidth <= 0) return;
    profiles.update((p) => ({ ...p, display: { width: d.desktopWidth, height: d.desktopHeight } }));
  };

  useEffect(
    () =>
      api.onPickDone((r) => {
        const id = pendingPick.current;
        pendingPick.current = null;
        setPickActive(false);
        if (r.ok && id) {
          profiles.updateModule(id, { target: { kind: "absolute", x: r.x, y: r.y } });
          snapshotDisplay();
        }
      }),
    [api, profiles.updateModule],
  );

  // addModule creates a click and expands it right away.
  const addModule = (containerId: string = UNGROUPED) => {
    const id = profiles.addModule(containerId);
    setExpandedId(id);
  };

  const onPick = (id: string) => {
    pendingPick.current = id;
    setPickActive(true);
    void api.startPick();
  };

  return (
    <div className="flex h-screen w-screen flex-col">
      <div className="flex min-h-0 flex-1">
        <Sidebar
          profiles={profiles.profiles}
          problems={profiles.problems}
          selectedId={profiles.selectedId}
          locked={locked}
          onSelect={(id) => void profiles.select(id)}
          onCreate={() => {
            const name = prompt("Profile name", "New profile");
            if (name) void profiles.create(name);
          }}
          onRename={(id, name) => void profiles.rename(id, name)}
          onDuplicate={(id) => void profiles.duplicate(id)}
          onDelete={(id) => void profiles.remove(id)}
          onOpenSettings={() => setShowSettings(true)}
        />

        <main className="flex min-w-0 flex-1 flex-col">
          <header className="flex items-center justify-between border-b border-neutral-800 px-3 py-2">
            <h1 className="text-neutral-100">
              ▸ Click pipeline{" "}
              <span className="text-[12px] font-normal text-neutral-500">
                (enabled modules run top → bottom)
              </span>
            </h1>
            {profiles.error && <span className="text-[12px] text-rose-400">{profiles.error}</span>}
            <span className="text-[11px] text-neutral-500">{profiles.saved ? "Saved" : "Saving…"}</span>
          </header>

          <div className="min-h-0 flex-1 overflow-y-auto p-3">
            {profile && <WarningBanner issues={profiles.issues} onUpdateSnapshot={snapshotDisplay} />}
            {profile && profile.modules.length === 0 && profile.groups.length === 0 ? (
              <EmptyState onAdd={() => addModule()} />
            ) : profile ? (
              <>
                {blocking && (
                  <div className="mb-2 rounded border border-amber-400/40 bg-amber-400/10 px-3 py-2 text-[12px] text-amber-200">
                    ⚠ {blocking.message}
                  </div>
                )}
                <ModuleList
                  modules={profile.modules}
                  groups={profile.groups}
                  issues={profiles.issues}
                  locked={locked}
                  desktop={desktop}
                  expandedId={expandedId}
                  onUpdate={profiles.updateModule}
                  onDelete={deleteModule}
                  onDuplicate={profiles.duplicateModule}
                  onMove={profiles.moveModule}
                  onExpand={(id) => setExpandedId((cur) => (cur === id ? null : id))}
                  onPick={onPick}
                  onAddGroup={() => {
                    const name = prompt("Group name", "New group");
                    if (name) profiles.addGroup(name);
                  }}
                  onRenameGroup={profiles.renameGroup}
                  onSetGroupEnabled={profiles.setGroupEnabled}
                  onRemoveGroup={profiles.removeGroup}
                  onMoveGroup={profiles.moveGroup}
                  onAddModule={addModule}
                />
              </>
            ) : (
              <p className="p-6 text-neutral-500">No profile selected.</p>
            )}
          </div>
        </main>
      </div>

      <BottomBar
        loop={profile?.loop ?? { mode: "once", count: 1, loopDelay: { min: 250, max: 250 } }}
        onLoopChange={(loop) => profiles.update((p) => ({ ...p, loop }))}
        locked={locked}
        progress={runner.progress}
        canRun={canRun}
        blockReason={blocking?.message}
        onRun={runOrStop}
        onStop={() => void runner.stop()}
        onPause={() => void runner.pause()}
        onResume={() => void runner.resume()}
        onPanic={() => void runner.panic()}
      />

      {pickActive && (
        <PickBanner seconds={5} onCancel={() => void api.cancelPick()} />
      )}

      {showSettings && settings && (
        <SettingsDialog
          settings={settings}
          diagnostics={diagnostics}
          capabilities={capabilities}
          profile={profile}
          onSave={(s) => void saveSettings(s)}
          onChangeFailsafe={(f: Failsafe) => profiles.update((p) => ({ ...p, failsafe: f }))}
          onOpenFolder={() => void api.openProfilesDir()}
          onOpenLog={() => void api.openLogDir()}
          onClose={() => setShowSettings(false)}
        />
      )}

      {undo && (
        <UndoToast
          message={`Deleted “${undo.name}”`}
          onUndo={() => {
            profiles.restoreModule(undo.module, undo.containerId, undo.index);
            setUndo(null);
          }}
          onDismiss={() => setUndo(null)}
        />
      )}
    </div>
  );
}
