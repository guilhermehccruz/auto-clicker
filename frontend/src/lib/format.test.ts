import { describe, expect, it } from "vitest";
import type { Module } from "../api/types";
import { executionOrders, formatElapsed, summary, targetChip } from "./format";

function mod(partial: Partial<Module>): Module {
  return {
    kind: "click", id: "m1", name: "", enabled: true,
    target: { kind: "cursor" }, button: "left",
    count: 1, clickInterval: 0, holdMs: 10, delayAfter: 250,
    ...partial,
  };
}

describe("summary", () => {
  it("renders an absolute module", () => {
    const m = mod({ target: { kind: "absolute", x: 940, y: 520 }, count: 3, clickInterval: 90 });
    expect(summary(m)).toBe("(940, 520) left ×3 @90ms hold 10ms");
  });
  it("renders a cursor module", () => {
    expect(summary(mod({}))).toBe("at cursor left ×1 hold 10ms");
  });
});

describe("targetChip", () => {
  it("flags off-screen coordinates", () => {
    expect(targetChip(mod({ target: { kind: "absolute", x: 5000, y: 10 } }), 1920, 1080)).toContain("⚠");
    expect(targetChip(mod({ target: { kind: "absolute", x: 10, y: 10 } }), 1920, 1080)).toBe("(10, 10)");
  });
});

describe("executionOrders", () => {
  it("numbers only enabled modules and preserves order", () => {
    const orders = executionOrders(
      [mod({ id: "a", enabled: true }), mod({ id: "b", enabled: false }), mod({ id: "c", enabled: true })],
      [],
    );
    expect(orders.get("a")).toBe(1);
    expect(orders.get("b")).toBeUndefined();
    expect(orders.get("c")).toBe(2);
  });
});

describe("formatElapsed", () => {
  it("formats units", () => {
    expect(formatElapsed(250)).toBe("250ms");
    expect(formatElapsed(1500)).toBe("1.5s");
    expect(formatElapsed(65000)).toBe("1m 5s");
  });
});
