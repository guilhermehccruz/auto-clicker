import type { Group, Module } from "../api/types";

// summary mirrors Go model.Module.Summary so the card and the engine agree.
export function summary(m: Module): string {
  if (m.kind === "key") {
    let out = `${m.keys || "(no keys)"} ×${m.count} hold ${m.holdMs}ms`;
    if (m.delayAfter > 0) out += ` delay ${m.delayAfter}ms`;
    return out;
  }
  const target =
    m.target.kind === "absolute"
      ? m.target.x !== undefined && m.target.y !== undefined
        ? `(${m.target.x}, ${m.target.y})`
        : "(no coords)"
      : "at cursor";
  if (m.button === "none") {
    return m.count > 1 ? `${target} move ×${m.count}` : `${target} move only`;
  }
  let out = `${target} ${m.button} ×${m.count}`;
  out += ` hold ${m.holdMs}ms`;
  if (m.delayAfter > 0) out += ` delay ${m.delayAfter}ms`;
  return out;
}

export function label(m: Module): string {
  return m.name.trim() !== "" ? m.name : summary(m);
}

export function targetChip(m: Module, desktopW: number, desktopH: number): string {
  if (m.kind === "key") return `⌨ ${m.keys || "no keys"}`;
  if (m.target.kind === "cursor") return "⌖ at cursor";
  const { x, y } = m.target;
  if (x === undefined || y === undefined) return "⚠ no coords";
  const off = x < 0 || y < 0 || (desktopW > 0 && x >= desktopW) || (desktopH > 0 && y >= desktopH);
  return off ? `⚠ (${x}, ${y})` : `(${x}, ${y})`;
}

// numberInput normalizes a numeric <input> as the user types: it strips leading
// zeros (so typing into a field showing "0" yields "100", not "0100") and
// returns the parsed value.
export function numberInput(e: { currentTarget: HTMLInputElement }): number {
  const el = e.currentTarget;
  const raw = el.value.replace(/^0+(?=\d)/, "");
  if (el.value !== raw) el.value = raw;
  return raw === "" ? 0 : Number(raw);
}

export function formatElapsed(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${Math.round(s % 60)}s`;
}

// executionOrders numbers enabled modules in execution order: ungrouped first,
// then each enabled group (DESIGN §5.3). Disabled modules/groups get no number.
export function executionOrders(modules: Module[], groups: Group[]): Map<string, number> {
  const orders = new Map<string, number>();
  let n = 0;
  for (const m of modules) if (m.enabled) orders.set(m.id, ++n);
  for (const g of groups) {
    if (!g.enabled) continue;
    for (const m of g.modules) if (m.enabled) orders.set(m.id, ++n);
  }
  return orders;
}
