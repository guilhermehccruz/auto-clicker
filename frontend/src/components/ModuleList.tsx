import { useEffect, useMemo, useRef, useState, type DragEvent as ReactDragEvent, type MouseEvent } from "react";
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCorners,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
} from "@dnd-kit/core";
import { SortableContext, sortableKeyboardCoordinates, verticalListSortingStrategy } from "@dnd-kit/sortable";
import type { Group, Issue, Module } from "../api/types";
import { executionOrders } from "../lib/format";
import { UNGROUPED } from "../state/useProfiles";
import { Dropdown, MenuItem } from "./Dropdown";
import { ModuleCard } from "./ModuleCard";

const UNGROUPED_KEY = "__ungrouped__";

interface Container {
  key: string; // dnd id ("__ungrouped__" or group id)
  modelId: string; // "" or group id
  group?: Group;
  modules: Module[];
}

export function ModuleList({
  modules,
  groups,
  issues,
  locked,
  desktop,
  expandedId,
  onUpdate,
  onDelete,
  onDuplicate,
  onMove,
  onExpand,
  onPick,
  onAddGroup,
  onRenameGroup,
  onSetGroupEnabled,
  onRemoveGroup,
  onMoveGroup,
  onAddModule,
}: {
  modules: Module[];
  groups: Group[];
  issues: Issue[];
  locked: boolean;
  desktop: { width: number; height: number };
  expandedId: string | null;
  onUpdate: (id: string, patch: Partial<Module>) => void;
  onDelete: (id: string) => void;
  onDuplicate: (id: string) => void;
  onMove: (id: string, toContainer: string, toIndex: number) => void;
  onExpand: (id: string) => void;
  onPick: (id: string) => void;
  onAddGroup: () => void;
  onRenameGroup: (id: string, name: string) => void;
  onSetGroupEnabled: (id: string, enabled: boolean) => void;
  onRemoveGroup: (id: string) => void;
  onMoveGroup: (from: number, to: number) => void;
  onAddModule: (containerId: string) => void;
}) {
  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const containers: Container[] = useMemo(
    () => [
      { key: UNGROUPED_KEY, modelId: UNGROUPED, modules },
      ...groups.map((g) => ({ key: g.id, modelId: g.id, group: g, modules: g.modules })),
    ],
    [modules, groups],
  );

  const moduleContainer = useMemo(() => {
    const m = new Map<string, string>();
    for (const c of containers) for (const mod of c.modules) m.set(mod.id, c.key);
    return m;
  }, [containers]);

  const containerKeys = useMemo(() => new Set(containers.map((c) => c.key)), [containers]);
  const containerOf = (id: string): string | undefined =>
    containerKeys.has(id) ? id : moduleContainer.get(id);
  const modelIdOf = (key: string) => (key === UNGROUPED_KEY ? UNGROUPED : key);

  const orders = executionOrders(modules, groups);
  const showUngroupedHeader = groups.length > 0;

  // Collapsed sections are UI-only state (not persisted): they just hide the
  // module list to reduce clutter. Groups start collapsed when the app opens;
  // the ungrouped section stays expanded.
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set(groups.map((g) => g.id)));
  const toggleCollapse = (key: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  // Auto-expand a collapsed section when a click is added to it, so the new
  // card is visible. The first run only records the current counts: otherwise
  // the groups seeded as collapsed above would immediately expand again.
  const prevCounts = useRef<Map<string, number> | null>(null);
  useEffect(() => {
    if (prevCounts.current === null) {
      prevCounts.current = new Map(containers.map((c) => [c.key, c.modules.length]));
      return;
    }
    setCollapsed((prev) => {
      let changed = false;
      const next = new Set(prev);
      for (const c of containers) {
        const before = prevCounts.current?.get(c.key) ?? 0;
        if (c.modules.length > before && next.has(c.key)) {
          next.delete(c.key);
          changed = true;
        }
      }
      return changed ? next : prev;
    });
    prevCounts.current = new Map(containers.map((c) => [c.key, c.modules.length]));
  }, [containers]);

  const onDragOver = (e: DragOverEvent) => {
    const activeId = String(e.active.id);
    const overId = e.over ? String(e.over.id) : null;
    if (!overId) return;
    const from = moduleContainer.get(activeId);
    const to = containerOf(overId);
    if (!from || !to || from === to) return;
    const target = containers.find((c) => c.key === to);
    if (!target) return;
    const overIdx = target.modules.findIndex((m) => m.id === overId);
    onMove(activeId, modelIdOf(to), overIdx >= 0 ? overIdx : target.modules.length);
  };

  const onDragEnd = (e: DragEndEvent) => {
    const activeId = String(e.active.id);
    const overId = e.over ? String(e.over.id) : null;
    if (!overId) return;
    const from = moduleContainer.get(activeId);
    const to = containerOf(overId);
    if (!from || from !== to) return; // cross-container moves are handled in onDragOver
    const c = containers.find((x) => x.key === from);
    if (!c) return;
    const fromIdx = c.modules.findIndex((m) => m.id === activeId);
    const toIdx = c.modules.findIndex((m) => m.id === overId);
    if (fromIdx < 0 || toIdx < 0 || fromIdx === toIdx) return;
    onMove(activeId, modelIdOf(from), toIdx);
  };

  return (
    <DndContext sensors={sensors} collisionDetection={closestCorners} onDragOver={onDragOver} onDragEnd={onDragEnd}>
      <div className="flex flex-col gap-4">
        {containers.map((c, ci) => (
          <Section
            key={c.key}
            container={c}
            groupIndex={c.group ? ci - 1 : -1}
            showHeader={c.group !== undefined || showUngroupedHeader}
            collapsed={collapsed.has(c.key)}
            onToggleCollapse={() => toggleCollapse(c.key)}
            orders={orders}
            issues={issues}
            locked={locked}
            desktop={desktop}
            expandedId={expandedId}
            onUpdate={onUpdate}
            onDelete={onDelete}
            onDuplicate={onDuplicate}
            onExpand={onExpand}
            onPick={onPick}
            onMove={onMove}
            onRenameGroup={onRenameGroup}
            onSetGroupEnabled={onSetGroupEnabled}
            onRemoveGroup={onRemoveGroup}
            onMoveGroup={onMoveGroup}
            onAddModule={onAddModule}
          />
        ))}
        <button
          type="button"
          disabled={locked}
          onClick={onAddGroup}
          className="self-start rounded border border-neutral-700 px-3 py-1.5 text-[12px] text-neutral-200 enabled:hover:bg-neutral-800 disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          + Add group
        </button>
      </div>
    </DndContext>
  );
}

function Section({
  container,
  groupIndex,
  showHeader,
  collapsed,
  onToggleCollapse,
  orders,
  issues,
  locked,
  desktop,
  expandedId,
  onUpdate,
  onDelete,
  onDuplicate,
  onExpand,
  onPick,
  onMove,
  onRenameGroup,
  onSetGroupEnabled,
  onRemoveGroup,
  onMoveGroup,
  onAddModule,
}: {
  container: Container;
  groupIndex: number; // group index (0-based) or -1 for the ungrouped section
  showHeader: boolean;
  collapsed: boolean;
  onToggleCollapse: () => void;
  orders: Map<string, number>;
  issues: Issue[];
  locked: boolean;
  desktop: { width: number; height: number };
  expandedId: string | null;
  onUpdate: (id: string, patch: Partial<Module>) => void;
  onDelete: (id: string) => void;
  onDuplicate: (id: string) => void;
  onExpand: (id: string) => void;
  onPick: (id: string) => void;
  onMove: (id: string, toContainer: string, toIndex: number) => void;
  onRenameGroup: (id: string, name: string) => void;
  onSetGroupEnabled: (id: string, enabled: boolean) => void;
  onRemoveGroup: (id: string) => void;
  onMoveGroup: (from: number, to: number) => void;
  onAddModule: (containerId: string) => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: container.key });
  const g = container.group;
  const [dragOver, setDragOver] = useState(false);

  // Clicking the header toggles collapse, like clicking a module card expands
  // it; interactive controls are excluded.
  const onHeaderClick = (e: MouseEvent) => {
    const el = e.target as HTMLElement;
    if (el.closest("button, input, select, textarea, [role='menu']")) return;
    onToggleCollapse();
  };

  // Rename lives on the group name text only: double-click it to edit. A single
  // click on the name does not toggle (no delay); the rest of the header does.
  const [editingName, setEditingName] = useState(false);

  // Groups are reordered with native HTML5 drag (separate from dnd-kit, which
  // handles module drags), so the two never conflict.
  const onSectionDragOver = (e: ReactDragEvent) => {
    if (groupIndex < 0 || !e.dataTransfer.types.includes("text/plain")) return;
    e.preventDefault();
    setDragOver(true);
  };
  const onSectionDrop = (e: ReactDragEvent) => {
    if (groupIndex < 0) return;
    e.preventDefault();
    setDragOver(false);
    const from = Number(e.dataTransfer.getData("text/plain"));
    if (!Number.isNaN(from) && from !== groupIndex) onMoveGroup(from, groupIndex);
  };

  return (
    <section
      onDragOver={onSectionDragOver}
      onDragLeave={(e) => {
        if (e.currentTarget.contains(e.relatedTarget as Node)) return;
        setDragOver(false);
      }}
      onDrop={onSectionDrop}
      className={`flex flex-col gap-2 rounded-lg ${dragOver ? "ring-1 ring-indigo-500/60" : ""}`}
    >
      {showHeader && (
        <header
          onClick={onHeaderClick}
          className="flex cursor-pointer items-center gap-2 rounded border border-neutral-700 bg-neutral-900 px-3 py-1.5"
        >
          <button
            type="button"
            onClick={onToggleCollapse}
            aria-label={collapsed ? "Expand group" : "Collapse group"}
            aria-expanded={!collapsed}
            className="px-1 text-neutral-400 hover:text-neutral-200"
          >
            {collapsed ? "▸" : "▾"}
          </button>
          {g ? (
            <>
              <button
                type="button"
                role="switch"
                aria-checked={g.enabled}
                aria-label={g.enabled ? "Disable group" : "Enable group"}
                disabled={locked}
                onClick={() => onSetGroupEnabled(g.id, !g.enabled)}
                className={`h-4 w-4 shrink-0 rounded border ${
                  g.enabled ? "border-emerald-500 bg-emerald-500" : "border-neutral-600"
                } disabled:opacity-50`}
              />
              {editingName ? (
                <input
                  autoFocus
                  value={g.name}
                  maxLength={80}
                  onClick={(e) => e.stopPropagation()}
                  onChange={(e) => onRenameGroup(g.id, e.target.value)}
                  onBlur={() => setEditingName(false)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === "Escape") (e.target as HTMLInputElement).blur();
                  }}
                  className="w-40 shrink-0 rounded border border-neutral-600 bg-neutral-900 px-1.5 py-0.5 text-neutral-100"
                />
              ) : (
                <span
                  onClick={(e) => e.stopPropagation()}
                  onDoubleClick={(e) => {
                    e.stopPropagation();
                    setEditingName(true);
                  }}
                  title="Double-click to rename"
                  className="max-w-[16rem] shrink-0 truncate text-neutral-100"
                >
                  {g.name}
                </span>
              )}
              {/* Free space belongs to the header (click to collapse), not the
                  rename-only name text. */}
              <div className="min-w-0 flex-1" />
              <span className="text-[11px] text-neutral-500">
                {g.modules.filter((m) => m.enabled).length}/{g.modules.length} on
              </span>
              <span
                draggable
                onDragStart={(e) => {
                  e.dataTransfer.setData("text/plain", String(groupIndex));
                  e.dataTransfer.effectAllowed = "move";
                }}
                title="Drag to reorder group"
                aria-label="Drag to reorder group"
                className={`px-1 text-neutral-500 hover:text-neutral-300 ${locked ? "invisible" : "cursor-grab"}`}
              >
                ≡
              </span>
              <Dropdown label="⋮" ariaLabel="Group menu">
                <MenuItem onClick={() => onAddModule(g.id)} disabled={locked}>Add click</MenuItem>
                <MenuItem onClick={() => onSetGroupEnabled(g.id, !g.enabled)} disabled={locked}>
                  {g.enabled ? "Disable group" : "Enable group"}
                </MenuItem>
                <MenuItem
                  disabled={locked}
                  danger
                  onClick={() => {
                    if (confirm(`Delete group "${g.name}"? Its clicks move to the ungrouped list.`)) onRemoveGroup(g.id);
                  }}
                >
                  Delete group
                </MenuItem>
              </Dropdown>
            </>
          ) : (
            <span className="text-[11px] uppercase tracking-wide text-neutral-500">Ungrouped</span>
          )}
        </header>
      )}

      {collapsed ? null : (
      <div
        ref={setNodeRef}
        className={`flex flex-col gap-2 rounded-lg p-1 ${isOver ? "bg-indigo-500/10 ring-1 ring-indigo-500/40" : ""}`}
      >
        <SortableContext items={container.modules.map((m) => m.id)} strategy={verticalListSortingStrategy}>
          <ul className="flex flex-col gap-2">
            {container.modules.map((m) => (
              <ModuleCard
                key={m.id}
                module={m}
                order={orders.get(m.id)}
                expanded={expandedId === m.id}
                locked={locked}
                issue={issues.find((i) => i.moduleId === m.id)}
                desktop={desktop}
                onUpdate={(patch) => onUpdate(m.id, patch)}
                onDelete={() => onDelete(m.id)}
                onDuplicate={() => onDuplicate(m.id)}
                onMoveUp={() => {
                  const i = container.modules.findIndex((x) => x.id === m.id);
                  if (i > 0) onMove(m.id, container.modelId, i - 1);
                }}
                onMoveDown={() => {
                  const i = container.modules.findIndex((x) => x.id === m.id);
                  if (i < container.modules.length - 1) onMove(m.id, container.modelId, i + 1);
                }}
                onExpand={() => onExpand(m.id)}
                onPick={() => onPick(m.id)}
              />
            ))}
          </ul>
        </SortableContext>
        {container.modules.length === 0 && (
          <p className="px-2 py-3 text-[11px] text-neutral-600">
            Drop clicks here{g ? ` to add them to “${g.name}”` : ""}.
          </p>
        )}
        <button
          type="button"
          disabled={locked}
          onClick={() => onAddModule(container.modelId)}
          className="self-start rounded px-2 py-1 text-[12px] text-neutral-300 enabled:hover:bg-neutral-800 disabled:opacity-40"
        >
          + Add click
        </button>
      </div>
      )}
    </section>
  );
}
