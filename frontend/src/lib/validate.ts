import type { Display, Group, Issue, Module, Profile } from "../api/types";
import { RANGES } from "../api/types";
import { summary } from "./format";

// validateProfile mirrors Go model.Validate so the UI renders the same issues
// the runner would refuse on (DESIGN §5.6). Keep the two in sync.
export function validateProfile(p: Profile, current: Display): Issue[] {
  const issues: Issue[] = [];
  const groups: Group[] = p.groups ?? [];
  const modules: Module[] = p.modules ?? [];

  const enabledCount =
    modules.filter((m) => m.enabled).length +
    groups.filter((g) => g.enabled).reduce((n, g) => n + g.modules.filter((m) => m.enabled).length, 0);
  if (enabledCount === 0) {
    issues.push({ level: "blocking", message: "No modules enabled — enable at least one to run" });
  }

  if (!["once", "count", "forever"].includes(p.loop.mode)) {
    issues.push({ level: "blocking", field: "loop.mode", message: `Unknown loop mode "${p.loop.mode}"` });
  }
  if (p.loop.mode === "count" && p.loop.count < 1) {
    issues.push({ level: "blocking", field: "loop.count", message: "Loop count must be at least 1" });
  }
  if (p.loop.loopDelay.min > p.loop.loopDelay.max) {
    issues.push({ level: "blocking", field: "loop.loopDelay", message: "Loop delay min is greater than max" });
  }

  const checkModule = (m: Module, effectiveEnabled: boolean) => {
    const name = m.name.trim() !== "" ? m.name : summary(m);
    const range = (field: keyof typeof RANGES, value: number, unit = "") => {
      const [lo, hi] = RANGES[field];
      if (value < lo || value > hi) {
        issues.push({ level: "blocking", moduleId: m.id, field, message: `${name}: ${field} must be ${lo}–${hi}${unit}` });
      }
    };
    range("count", m.count);
    range("clickInterval", m.clickInterval, " ms");
    range("holdMs", m.holdMs, " ms");
    range("delayAfter", m.delayAfter, " ms");

    if (m.target.kind === "absolute") {
      const { x, y } = m.target;
      if (effectiveEnabled && (x === undefined || y === undefined)) {
        issues.push({ level: "blocking", moduleId: m.id, field: "target", message: `${name}: absolute target has no coordinates` });
      } else if (effectiveEnabled && current.width > 0 && (x! < 0 || y! < 0 || x! >= current.width || y! >= current.height)) {
        issues.push({ level: "warning", moduleId: m.id, field: "target", message: `${name}: (${x}, ${y}) is outside the current desktop` });
      }
    }
  };

  for (const m of modules) checkModule(m, m.enabled);
  for (const g of groups) {
    for (const m of g.modules) checkModule(m, g.enabled && m.enabled);
  }

  if (current.width > 0 && p.display.width > 0 && (p.display.width !== current.width || p.display.height !== current.height)) {
    issues.push({
      level: "warning",
      field: "display",
      message: `This profile was recorded for ${p.display.width}×${p.display.height}; the current desktop is ${current.width}×${current.height} — verify your coordinates`,
    });
  }
  return issues;
}

export function hasBlocking(issues: Issue[]): boolean {
  return issues.some((i) => i.level === "blocking");
}
