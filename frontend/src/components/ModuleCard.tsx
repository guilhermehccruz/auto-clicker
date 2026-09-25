import { useState, type MouseEvent } from "react";
import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { ButtonName, Issue, Module, Target } from "../api/types";
import { summary, targetChip } from "../lib/format";
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
      <div className="flex cursor-pointer items-center gap-3" onClick={onRowClick}>
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
        <span className="mono shrink-0 text-neutral-200">{targetChip(m, desktop.width, desktop.height)}</span>
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
        <label className="flex items-center gap-1 text-[11px] text-neutral-500">
          delay
          <input
            type="number"
            value={m.delayAfter}
            disabled={locked}
            onChange={(e) => onUpdate({ delayAfter: Number(e.target.value) })}
            className="mono w-16 rounded border border-neutral-700 bg-neutral-900 px-1 py-0.5 text-neutral-200 disabled:opacity-50"
          />
          ms
        </label>

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
          <TargetEditor target={m.target} disabled={locked} onChange={setTarget} onPick={onPick} />

          <div className="flex flex-wrap items-center gap-3 text-[12px] text-neutral-400">
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
            <NumberField label="count" value={m.count} min={1} max={1000} disabled={locked} onChange={(v) => onUpdate({ count: v })} />
            <NumberField label="interval" value={m.clickInterval} min={0} max={60000} unit="ms" disabled={locked} onChange={(v) => onUpdate({ clickInterval: v })} />
            {m.button !== "none" && (
              <NumberField label="hold" value={m.holdMs} min={0} max={1000} unit="ms" disabled={locked} onChange={(v) => onUpdate({ holdMs: v })} />
            )}
          </div>
          {m.button !== "none" && m.holdMs < 5 && (
            <p className="text-[11px] text-amber-400">
              Hold under 5 ms may be ignored by some targets; the period is hold + interval.
            </p>
          )}
        </div>
      )}
    </li>
  );
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
        onChange={(e) => onChange(Number(e.target.value))}
        className="mono w-20 rounded border border-neutral-700 bg-neutral-900 px-1.5 py-0.5 text-neutral-100 disabled:opacity-50"
      />
      {unit}
    </label>
  );
}
