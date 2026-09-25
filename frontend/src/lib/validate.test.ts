import { describe, expect, it } from "vitest";
import type { Profile } from "../api/types";
import { hasBlocking, validateProfile } from "./validate";

function profile(modules: Profile["modules"]): Profile {
  return {
    schemaVersion: 1, name: "t", display: { width: 1920, height: 1080 },
    loop: { mode: "once", count: 1, loopDelay: { min: 250, max: 250 } },
    failsafe: { maxRuntimeSec: 3600, mouseToCornerStops: false },
    modules,
    groups: [],
  };
}

const base = {
  kind: "click" as const, id: "m1", name: "", enabled: true,
  target: { kind: "cursor" as const }, button: "left" as const,
  count: 1, clickInterval: 0, holdMs: 10, delayAfter: 250,
};

describe("validateProfile", () => {
  it("blocks when no module is enabled", () => {
    expect(hasBlocking(validateProfile(profile([{ ...base, enabled: false }]), { width: 1920, height: 1080 }))).toBe(true);
  });

  it("blocks out-of-range fields", () => {
    const issues = validateProfile(profile([{ ...base, count: 0 }]), { width: 1920, height: 1080 });
    expect(issues.some((i) => i.field === "count" && i.level === "blocking")).toBe(true);
  });

  it("warns but does not block off-screen coordinates", () => {
    const issues = validateProfile(profile([{ ...base, target: { kind: "absolute", x: 5000, y: 10 } }]), { width: 1920, height: 1080 });
    expect(hasBlocking(issues)).toBe(false);
    expect(issues.some((i) => i.level === "warning" && i.moduleId === "m1")).toBe(true);
  });

  it("warns on a display-geometry mismatch", () => {
    const p = profile([base]);
    p.display = { width: 5360, height: 1440 };
    const issues = validateProfile(p, { width: 1920, height: 1080 });
    expect(issues.some((i) => i.field === "display" && i.level === "warning")).toBe(true);
  });

  it("blocks a count loop with count < 1", () => {
    const p = profile([base]);
    p.loop = { mode: "count", count: 0, loopDelay: { min: 250, max: 250 } };
    expect(hasBlocking(validateProfile(p, { width: 1920, height: 1080 }))).toBe(true);
  });
});
