// Shared TS types mirroring internal/model and internal/engine.
// Keep in sync with the Go structs; the Go side is the source of truth.

export type TargetKind = "absolute" | "cursor";
// "none" = move only (no button event; absolute targets only).
export type ButtonName = "left" | "right" | "middle" | "none";
export type LoopMode = "once" | "count" | "forever";
export type IssueLevel = "blocking" | "warning";
export type RunState = "idle" | "running" | "paused" | "stopping";

export interface Target {
  kind: TargetKind;
  x?: number;
  y?: number;
}

export interface Module {
  kind?: "click";
  id: string;
  name: string;
  enabled: boolean;
  target: Target;
  button: ButtonName;
  count: number;
  clickInterval: number;
  holdMs: number;
  delayAfter: number;
}

export interface DelayRange {
  min: number;
  max: number;
}

export interface Loop {
  mode: LoopMode;
  count: number;
  loopDelay: DelayRange;
}

export interface Failsafe {
  maxRuntimeSec: number;
  mouseToCornerStops: boolean;
}

export interface Display {
  width: number;
  height: number;
}

export interface Group {
  id: string;
  name: string;
  enabled: boolean;
  modules: Module[];
}

export interface Profile {
  schemaVersion: number;
  name: string;
  display: Display;
  loop: Loop;
  failsafe: Failsafe;
  modules: Module[]; // ungrouped
  groups: Group[];
}

export interface Issue {
  level: IssueLevel;
  moduleId?: string;
  field?: string;
  message: string;
}

export interface ProfileSummary {
  id: string;
  name: string;
  modules: number;
  enabled: number;
}

export interface ProblemFile {
  id: string;
  error: string;
}

export interface DisplayInfo {
  X: number;
  Y: number;
  W: number;
  H: number;
  Scale: number;
  Primary: boolean;
}

export interface Capabilities {
  Backend: string;
  FallbackReason?: string;
  CanReadCursor: boolean;
  CanPickPoint: boolean;
  CanEnumerateDisplays: boolean;
  CanMoveAbsolute: boolean;
}

export interface Diagnostics {
  version: string;
  robotgoPin: string;
  appId: string;
  backend: string;
  fallbackReason?: string;
  canReadCursor: boolean;
  canPickPoint: boolean;
  canMoveAbsolute: boolean;
  desktopWidth: number;
  desktopHeight: number;
  displays: DisplayInfo[];
  hotkeySource: string;
  hotkeyError?: string;
  profilesDir: string;
  logDir: string;
}

export interface Progress {
  state: RunState;
  profileName: string;
  moduleId: string;
  moduleLabel: string;
  moduleIndex: number;
  moduleTotal: number;
  clickIndex: number;
  clickTotal: number;
  loopIteration: number;
  clicks: number;
  elapsedMs: number;
  stopReason?: string;
  error?: string;
}

export interface Hotkeys {
  run: string;
  pause: string;
  panic: string;
}

export interface WindowGeometry {
  x: number;
  y: number;
  width: number;
  height: number;
  maximized: boolean;
}

export interface Settings {
  schemaVersion: number;
  selectedProfile: string;
  profilesDir: string;
  window: WindowGeometry;
  hotkeys: Hotkeys;
  defaultMaxRuntimeSec: number;
  theme: string;
}

export const MODULE_DEFAULTS = {
  count: 1,
  clickInterval: 0,
  holdMs: 10,
  delayAfter: 250,
} as const;

export const RANGES = {
  count: [1, 1000],
  clickInterval: [0, 60000],
  holdMs: [0, 1000],
  delayAfter: [0, 3600000],
} as const;
