import type {
  Capabilities,
  Diagnostics,
  DisplayInfo,
  Profile,
  ProfileSummary,
  ProblemFile,
  Progress,
  Settings,
} from "./types";
import { MockApi } from "./mock";
import { WailsApi } from "./wails";

// Api is the §7.2 operation surface. The Wails v3 generated bindings should be
// wrapped in a WailsApi adapter here (import from the generated `bindings/`
// folder and forward each call + subscribe to `run:progress`).
export interface Api {
  listProfiles(): Promise<{ profiles: ProfileSummary[]; problems: ProblemFile[] }>;
  loadProfile(id: string): Promise<Profile>;
  saveProfile(id: string, p: Profile): Promise<void>;
  createProfile(name: string): Promise<string>;
  renameProfile(id: string, name: string): Promise<string>;
  duplicateProfile(id: string): Promise<string>;
  deleteProfile(id: string): Promise<void>;
  updateDisplaySnapshot(id: string): Promise<void>;

  validate(p: Profile): Promise<import("./types").Issue[]>;
  capabilities(): Promise<Capabilities>;
  diagnostics(): Promise<Diagnostics>;
  displays(): Promise<DisplayInfo[]>;
  cursorPos(): Promise<{ x: number; y: number; ok: boolean }>;
  startPick(): Promise<void>;
  confirmPick(): Promise<void>;
  cancelPick(): Promise<void>;
  onPickDone(cb: (r: { x: number; y: number; ok: boolean }) => void): () => void;

  run(id: string): Promise<void>;
  stop(): Promise<void>;
  pause(): Promise<void>;
  resume(): Promise<void>;
  panic(): Promise<void>;
  status(): Promise<Progress>;

  getSettings(): Promise<Settings>;
  saveSettings(s: Settings): Promise<void>;
  profilesDir(): Promise<string>;
  openProfilesDir(): Promise<void>;
  openLogDir(): Promise<void>;
  version(): Promise<string>;

  onProgress(cb: (p: Progress) => void): () => void;
}

declare global {
  interface Window {
    // Installed by the Wails shell when running packaged; absent in `vite dev`.
    autoClicker?: Api;
  }
}

// createApi returns the live bridge when running inside the Wails webview
// (marked by window._wails), else an in-memory mock so the UI is developable
// with `vite dev` and testable without the Go side.
export function createApi(): Api {
  if (typeof window !== "undefined") {
    if ((window as unknown as { _wails?: unknown })._wails) {
      return new WailsApi();
    }
    if (window.autoClicker) {
      return window.autoClicker;
    }
  }
  return new MockApi();
}
