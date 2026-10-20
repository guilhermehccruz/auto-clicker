import { useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type MouseEvent } from "react";
import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { ButtonName, Issue, Module, Target } from "../api/types";
import { summary, targetChip, numberInput } from "../lib/format";
import { Dropdown, MenuItem } from "./Dropdown";
import { Select } from "./Select";
import { TargetEditor } from "./TargetEditor";

const BUTTONS: readonly ButtonName[] = ["left", "right", "middle", "none"];

export function ModuleCard({
  module: m,
  order,
  expanded,
  locked,
  issue,
  desktop,
  onUpdate,
  onDelete,
  onDuplicate,
  onMoveUp,
  onMoveDown,
  onExpand,
  onPick,
}: {
  module: Module;
  order?: number;
  expanded: boolean;
  locked: boolean;
  issue?: Issue;
  desktop: { width: number; height: number };
  onUpdate: (patch: Partial<Module>) => void;
  onDelete: () => void;
  onDuplicate: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onExpand: () => void;
  onPick: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: m.id,
    disabled: locked,
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  const setTarget = (t: Target) => onUpdate({ target: t });
  const keyMode = m.kind === "key";

  // Clicking the card body expands it for editing; interactive controls and the
  // menu are excluded so they keep working.
  const onRowClick = (e: MouseEvent) => {
    const el = e.target as HTMLElement;
    if (el.closest("button, input, select, textarea, [role='switch'], [role='menu']")) return;
    onExpand();
  };

  // Rename lives on the title text only: double-click it to edit. A single
  // click on the text does not toggle (no delay); the rest of the card does.
  const [editingLabel, setEditingLabel] = useState(false);

  return (
    <li
      ref={setNodeRef}
      style={style}
      className={`rounded-lg border border-neutral-700 bg-neutral-800 p-3 ${
        m.enabled ? "" : "opacity-60 grayscale"
      }`}
    >
      <div className="flex cursor-pointer items-center gap-2" onClick={onRowClick}>
        <button
          type="button"
          role="switch"
          aria-checked={m.enabled}
          aria-label={m.enabled ? "Disable module" : "Enable module"}
          disabled={locked}
          onClick={() => onUpdate({ enabled: !m.enabled })}
          className={`h-4 w-4 shrink-0 rounded border ${
            m.enabled ? "border-emerald-500 bg-emerald-500" : "border-neutral-600 bg-transparent"
          } disabled:opacity-50`}
        />
        <span className="mono w-5 shrink-0 text-center text-[11px] text-neutral-500">
          {m.enabled ? order : "–"}
        </span>
        <span className="mono w-28 shrink-0 text-left text-neutral-200">
          {targetChip(m, desktop.width, desktop.height)}
        </span>
        {editingLabel ? (
          <input
            autoFocus
            value={m.name}
            maxLength={80}
            placeholder={summary(m)}
            onClick={(e) => e.stopPropagation()}
            onChange={(e) => onUpdate({ name: e.target.value })}
            onBlur={() => setEditingLabel(false)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === "Escape") (e.target as HTMLInputElement).blur();
            }}
            className="w-40 shrink-0 rounded border border-neutral-600 bg-neutral-900 px-1.5 py-0.5 text-[12px] text-neutral-100"
          />
        ) : (
          <span
            onClick={(e) => e.stopPropagation()}
            onDoubleClick={(e) => {
              e.stopPropagation();
              setEditingLabel(true);
            }}
            title="Double-click to rename"
            className="max-w-[20rem] shrink-0 truncate text-[12px] text-neutral-400"
          >
            {m.name.trim() !== "" ? m.name : summary(m)}
          </span>
        )}
        {/* Free space belongs to the card (click to expand), not the rename-only
            title text. */}
        <div className="min-w-0 flex-1" />

        <button
          type="button"
          {...attributes}
          {...listeners}
          disabled={locked}
          aria-label="Drag to reorder"
          title="Drag to reorder"
          className={`cursor-grab px-1 text-neutral-500 hover:text-neutral-300 ${locked ? "invisible" : ""}`}
        >
          ≡
        </button>

        <Dropdown label="⋮" ariaLabel="Module menu" className="rounded px-1 text-neutral-400 hover:text-neutral-200">
          <MenuItem onClick={onDuplicate} disabled={locked}>Duplicate</MenuItem>
          <MenuItem onClick={onMoveUp} disabled={locked}>Move up</MenuItem>
          <MenuItem onClick={onMoveDown} disabled={locked}>Move down</MenuItem>
          <MenuItem onClick={() => onUpdate({ enabled: !m.enabled })} disabled={locked}>{m.enabled ? "Disable" : "Enable"}</MenuItem>
          <MenuItem onClick={onDelete} disabled={locked} danger>Delete</MenuItem>
        </Dropdown>
      </div>

      {issue && (
        <p className={`mt-2 text-[11px] ${issue.level === "blocking" ? "text-rose-400" : "text-amber-400"}`}>
          {issue.message}
        </p>
      )}

      {expanded && (
        <div className="mt-3 flex flex-col gap-2 border-t border-neutral-700 pt-3">
          {keyMode ? (
            <KeyComboInput value={m.keys ?? ""} disabled={locked} onChange={(keys) => onUpdate({ keys })} />
          ) : (
            <>
              <TargetEditor target={m.target} disabled={locked} onChange={setTarget} onPick={onPick} />
              <KeyComboInput
                label="held keys"
                hint="optional: hold keys during the click"
                value={m.keys ?? ""}
                disabled={locked}
                onChange={(keys) => onUpdate({ keys })}
              />
            </>
          )}

          <div className="flex flex-wrap items-center gap-3 text-[12px] text-neutral-400">
            {!keyMode && (
              <div className="flex items-center gap-1.5">
                button
                <Select
                  ariaLabel="Mouse button"
                  value={m.button}
                  options={BUTTONS}
                  disabled={locked}
                  onChange={(button) => onUpdate({ button })}
                />
              </div>
            )}
            <NumberField label="count" value={m.count} min={1} max={1000} disabled={locked} onChange={(v) => onUpdate({ count: v })} />
            <NumberField label="delay" value={m.delayAfter} min={0} max={3600000} unit="ms" disabled={locked} onChange={(v) => onUpdate({ delayAfter: v })} />
            {(keyMode || m.button !== "none") && (
              <NumberField label="hold" value={m.holdMs} min={0} max={1000} unit="ms" disabled={locked} onChange={(v) => onUpdate({ holdMs: v })} />
            )}
          </div>
          <label className="flex items-center gap-2 text-[12px] text-neutral-400">
            <input
              type="checkbox"
              checked={m.holdUntilStop ?? false}
              disabled={locked}
              onChange={(e) =>
                onUpdate(e.target.checked ? { holdUntilStop: true, count: 1 } : { holdUntilStop: false })
              }
            />
            Hold until Stop (last enabled step; needs loop <span className="mono">once</span>)
          </label>
          {(keyMode || m.button !== "none") && m.holdMs < 5 && (
            <p className="text-[11px] text-amber-400">
              Hold under 5 ms may be ignored by some targets; the period is hold + delay.
            </p>
          )}
        </div>
      )}
    </li>
  );
}

function KeyComboInput({
  value,
  disabled,
  label = "keys",
  hint = "focus and press",
  onChange,
}: {
  value: string;
  disabled: boolean;
  label?: string;
  hint?: string;
  onChange: (keys: string) => void;
}) {
  const [recording, setRecording] = useState(false);
  const [draft, setDraft] = useState(value);
  const inputRef = useRef<HTMLInputElement>(null);
  return (
    <div className="flex items-center gap-2 text-[12px] text-neutral-400">
      {label}
      <input
        ref={inputRef}
        value={recording ? draft : value}
        readOnly
        disabled={disabled}
        placeholder="focus and press a combo"
        onFocus={() => {
          setDraft(value);
          setRecording(true);
        }}
        onBlur={() => setRecording(false)}
        onKeyDown={(e) => {
          if (!recording) return;
          e.preventDefault();
          e.stopPropagation();
          if (e.key === "Escape") {
            setDraft(value);
            setRecording(false);
            e.currentTarget.blur();
            return;
          }
          const modOnly = modifierKeyName(e.key);
          if (modOnly) {
            // A lone modifier is a valid action ("press shift").
            setDraft(modOnly);
            return;
          }
          const mods: string[] = [];
          if (e.ctrlKey) mods.push("ctrl");
          if (e.shiftKey) mods.push("shift");
          if (e.altKey) mods.push("alt");
          if (e.metaKey) mods.push("cmd");
          const key = comboKeyName(e);
          if (!key) return;
          setDraft([...mods, key].join("+"));
        }}
        className="mono w-56 rounded border border-neutral-700 bg-neutral-800 px-2 py-0.5 text-neutral-100 disabled:opacity-50"
      />
      {recording ? (
        <>
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => {
              onChange(draft);
              setRecording(false);
              inputRef.current?.blur();
            }}
            disabled={disabled}
            className="rounded border border-emerald-600/60 px-2 py-0.5 text-[11px] text-emerald-300 enabled:hover:bg-emerald-600/10 disabled:opacity-40"
          >
            OK
          </button>
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => {
              setDraft(value);
              setRecording(false);
              inputRef.current?.blur();
            }}
            disabled={disabled}
            className="rounded border border-neutral-700 px-2 py-0.5 text-[11px] enabled:hover:bg-neutral-800 disabled:opacity-40"
          >
            Cancel
          </button>
        </>
      ) : (
        <button
          type="button"
          onClick={() => onChange("")}
          disabled={disabled}
          className="rounded border border-neutral-700 px-2 py-0.5 text-[11px] enabled:hover:bg-neutral-800 disabled:opacity-40"
        >
          clear
        </button>
      )}
      <span className="text-[11px] text-neutral-500">
        {recording ? "press the combo, then OK" : hint}
      </span>
    </div>
  );
}

function modifierKeyName(key: string): string | null {
  switch (key) {
    case "Control":
      return "ctrl";
    case "Shift":
      return "shift";
    case "Alt":
      return "alt";
    case "Meta":
      return "cmd";
    default:
      return null;
  }
}

// comboKeyName maps a keydown to the robotgo key name used by the backend.
function comboKeyName(e: ReactKeyboardEvent<HTMLInputElement>): string | null {
  const named: Record<string, string> = {
    " ": "space", Escape: "esc", ArrowUp: "up", ArrowDown: "down",
    ArrowLeft: "left", ArrowRight: "right", Enter: "enter", Tab: "tab",
    Backspace: "backspace", Delete: "delete", Insert: "insert", Home: "home",
    End: "end", PageUp: "pageup", PageDown: "pagedown", CapsLock: "capslock",
  };
  const n = named[e.key];
  if (n) return n;
  if (/^F([1-9]|1[0-9]|2[0-4])$/.test(e.key)) return e.key.toLowerCase();
  if (e.key.length === 1) {
    const c = e.key.toLowerCase();
    if (/^[a-z0-9]$/.test(c) || "`-=[]\\;',./".includes(c)) return c;
  }
  return null;
}

function NumberField({
  label,
  value,
  min,
  max,
  unit,
  disabled,
  onChange,
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  unit?: string;
  disabled: boolean;
  onChange: (v: number) => void;
}) {
  return (
    <label className="flex items-center gap-1.5">
      {label}
      <input
        type="number"
        value={value}
        min={min}
        max={max}
        disabled={disabled}
        onChange={(e) => onChange(numberInput(e))}
        className="mono w-20 rounded border border-neutral-700 bg-neutral-900 px-1.5 py-0.5 text-neutral-100 disabled:opacity-50"
      />
      {unit}
    </label>
  );
}
