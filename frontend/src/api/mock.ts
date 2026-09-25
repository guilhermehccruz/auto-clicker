import type {
  Capabilities,
  Diagnostics,
  DisplayInfo,
  Issue,
  Profile,
  ProfileSummary,
  ProblemFile,
  Progress,
  Settings,
} from "./types";
import type { Api } from "./bindings";
import { validateProfile } from "../lib/validate";
import { label } from "../lib/format";

// MockApi is an in-memory stand-in used by `vite dev` and UI tests.
export class MockApi implements Api {
  private profiles = new Map<string, Profile>();
  private settings: Settings = {
    schemaVersion: 1,
    selectedProfile: "idle-tycoon",
    profilesDir: "~/Documents/auto-clicker",
    window: { x: 120, y: 80, width: 1100, height: 720, maximized: false },
    hotkeys: { run: "F8", pause: "F9", panic: "CTRL+SHIFT+F12" },
    defaultMaxRuntimeSec: 3600,
    theme: "dark",
  };
  private progress: Progress = emptyProgress();
  private listeners = new Set<(p: Progress) => void>();
  private timer: ReturnType<typeof setTimeout> | null = null;
  private paused = false;

  constructor() {
    this.profiles.set("idle-tycoon", {
      schemaVersion: 1,
      name: "Idle Tycoon",
      display: { width: 5360, height: 1440 },
      loop: { mode: "forever", count: 10, loopDelay: { min: 500, max: 1500 } },
      failsafe: { maxRuntimeSec: 3600, mouseToCornerStops: false },
      groups: [],
      modules: [
        { kind: "click", id: "m1", name: "", enabled: true, target: { kind: "absolute", x: 940, y: 520 }, button: "left", count: 3, clickInterval: 90, holdMs: 10, delayAfter: 250 },
        { kind: "click", id: "m2", name: "", enabled: true, target: { kind: "cursor" }, button: "left", count: 1, clickInterval: 0, holdMs: 10, delayAfter: 400 },
        { kind: "click", id: "m3", name: "", enabled: false, target: { kind: "absolute", x: 100, y: 100 }, button: "right", count: 1, clickInterval: 0, holdMs: 10, delayAfter: 250 },
      ],
    });
  }

  private emit(p: Progress) {
    this.progress = p;
    this.listeners.forEach((l) => l(p));
  }

  async listProfiles(): Promise<{ profiles: ProfileSummary[]; problems: ProblemFile[] }> {
    const profiles = [...this.profiles.entries()].map(([id, p]) => ({
      id,
      name: p.name,
      modules: p.modules.length,
      enabled: p.modules.filter((m) => m.enabled).length,
    }));
    return { profiles, problems: [] };
  }

  async loadProfile(id: string): Promise<Profile> {
    const p = this.profiles.get(id);
    if (!p) throw new Error("not found");
    return structuredClone(p);
  }

  async saveProfile(id: string, p: Profile): Promise<void> {
    this.profiles.set(id, structuredClone(p));
  }

  async createProfile(name: string): Promise<string> {
    const id = slug(name || "New profile");
    this.profiles.set(id, {
      schemaVersion: 1,
      name: name || "New profile",
      display: { width: 1920, height: 1080 },
      loop: { mode: "once", count: 1, loopDelay: { min: 250, max: 250 } },
      failsafe: { maxRuntimeSec: this.settings.defaultMaxRuntimeSec, mouseToCornerStops: false },
      groups: [],
      modules: [],
    });
    return id;
  }

  async renameProfile(id: string, name: string): Promise<string> {
    const p = this.profiles.get(id);
    if (!p) throw new Error("not found");
    this.profiles.delete(id);
    const nid = slug(name);
    p.name = name;
    this.profiles.set(nid, p);
    return nid;
  }

  async duplicateProfile(id: string): Promise<string> {
    const p = this.profiles.get(id);
    if (!p) throw new Error("not found");
    const nid = slug(`${p.name} copy`);
    this.profiles.set(nid, { ...structuredClone(p), name: `${p.name} copy` });
    return nid;
  }

  async deleteProfile(id: string): Promise<void> {
    this.profiles.delete(id);
  }

  async updateDisplaySnapshot(id: string): Promise<void> {
    const p = this.profiles.get(id);
    if (p) p.display = { width: 1920, height: 1080 };
  }

  async validate(p: Profile): Promise<Issue[]> {
    return validateProfile(p, { width: 1920, height: 1080 });
  }

  async capabilities(): Promise<Capabilities> {
    return { Backend: "mock", CanReadCursor: true, CanPickPoint: true, CanEnumerateDisplays: true, CanMoveAbsolute: true };
  }

  async diagnostics(): Promise<Diagnostics> {
    return {
      version: "dev",
      robotgoPin: "v1.0.3-0.20260921150940-12f16b7c5d82",
      appId: "auto-clicker",
      backend: "mock",
      canReadCursor: true,
      canPickPoint: true,
      canMoveAbsolute: true,
      desktopWidth: 1920,
      desktopHeight: 1080,
      displays: [{ X: 0, Y: 0, W: 1920, H: 1080, Scale: 1, Primary: true }],
      hotkeySource: "unavailable",
      profilesDir: this.settings.profilesDir,
      logDir: "~/.local/state/auto-clicker",
    };
  }

  async displays(): Promise<DisplayInfo[]> {
    return [{ X: 0, Y: 0, W: 1920, H: 1080, Scale: 1, Primary: true }];
  }

  async cursorPos() {
    return { x: 960, y: 540, ok: true };
  }

  private pickListeners = new Set<(r: { x: number; y: number; ok: boolean }) => void>();

  private pickTimer: ReturnType<typeof setTimeout> | null = null;

  async startPick(): Promise<void> {
    // Dev stand-in: capture a point shortly after, mirroring the Go countdown.
    this.pickTimer = setTimeout(() => {
      this.pickListeners.forEach((l) => l({ x: 940, y: 520, ok: true }));
    }, 300);
  }

  async confirmPick(): Promise<void> {
    if (this.pickTimer) clearTimeout(this.pickTimer);
    this.pickListeners.forEach((l) => l({ x: 940, y: 520, ok: true }));
  }

  async cancelPick(): Promise<void> {
    if (this.pickTimer) clearTimeout(this.pickTimer);
    this.pickListeners.forEach((l) => l({ x: 0, y: 0, ok: false }));
  }

  onPickDone(cb: (r: { x: number; y: number; ok: boolean }) => void): () => void {
    this.pickListeners.add(cb);
    return () => this.pickListeners.delete(cb);
  }

  async run(id: string): Promise<void> {
    const p = this.profiles.get(id);
    if (!p) throw new Error("not found");
    const enabled = p.modules.filter((m) => m.enabled);
    this.paused = false;
    let clicks = 0;
    const start = Date.now();
    let mi = 0;
    let ci = 0;
    const step = () => {
      if (this.paused) {
        this.timer = setTimeout(step, 100);
        return;
      }
      if (mi >= enabled.length) {
        this.emit({ ...emptyProgress(), state: "idle", profileName: p.name, clicks, elapsedMs: Date.now() - start, stopReason: "done" });
        return;
      }
      const m = enabled[mi];
      ci++;
      clicks++;
      if (ci >= m.count) {
        mi++;
        ci = 0;
      }
      this.emit({
        state: "running",
        profileName: p.name,
        moduleId: m.id,
        moduleLabel: label(m),
        moduleIndex: mi + 1,
        moduleTotal: enabled.length,
        clickIndex: ci,
        clickTotal: m.count,
        loopIteration: 1,
        clicks,
        elapsedMs: Date.now() - start,
      });
      this.timer = setTimeout(step, Math.max(30, m.clickInterval || m.delayAfter || 120));
    };
    this.emit({ ...emptyProgress(), state: "running", profileName: p.name });
    this.timer = setTimeout(step, 150);
  }

  async stop(): Promise<void> {
    if (this.timer) clearTimeout(this.timer);
    this.timer = null;
    this.emit({ ...this.progress, state: "idle", stopReason: "user" });
  }

  async pause(): Promise<void> {
    this.paused = true;
    this.emit({ ...this.progress, state: "paused" });
  }

  async resume(): Promise<void> {
    this.paused = false;
    this.emit({ ...this.progress, state: "running" });
  }

  async panic(): Promise<void> {
    if (this.timer) clearTimeout(this.timer);
    this.timer = null;
    this.paused = false;
    this.emit({ ...this.progress, state: "idle", stopReason: "panic" });
  }

  async status(): Promise<Progress> {
    return this.progress;
  }

  async getSettings(): Promise<Settings> {
    return structuredClone(this.settings);
  }

  async saveSettings(s: Settings): Promise<void> {
    this.settings = structuredClone(s);
  }

  async profilesDir(): Promise<string> {
    return this.settings.profilesDir;
  }

  async openProfilesDir(): Promise<void> {}

  async openLogDir(): Promise<void> {}

  async version(): Promise<string> {
    return "dev";
  }

  onProgress(cb: (p: Progress) => void): () => void {
    this.listeners.add(cb);
    return () => this.listeners.delete(cb);
  }
}

function emptyProgress(): Progress {
  return {
    state: "idle",
    profileName: "",
    moduleId: "",
    moduleLabel: "",
    moduleIndex: 0,
    moduleTotal: 0,
    clickIndex: 0,
    clickTotal: 0,
    loopIteration: 0,
    clicks: 0,
    elapsedMs: 0,
  };
}

function slug(name: string): string {
  return name
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64) || "profile";
}
