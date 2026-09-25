import type { ProfileSummary, ProblemFile } from "../api/types";
import { ProfileRow } from "./ProfileRow";

export function Sidebar({
  profiles,
  problems,
  selectedId,
  locked,
  onSelect,
  onCreate,
  onRename,
  onDuplicate,
  onDelete,
  onOpenSettings,
}: {
  profiles: ProfileSummary[];
  problems: ProblemFile[];
  selectedId: string | null;
  locked: boolean;
  onSelect: (id: string) => void;
  onCreate: () => void;
  onRename: (id: string, name: string) => void;
  onDuplicate: (id: string) => void;
  onDelete: (id: string) => void;
  onOpenSettings: () => void;
}) {
  return (
    <aside className="flex w-60 shrink-0 flex-col border-r border-neutral-800 bg-neutral-900">
      <div className="px-3 py-2 text-[11px] uppercase tracking-wide text-neutral-500">Profiles</div>
      <div className="min-h-0 flex-1 overflow-y-auto px-1">
        {profiles.map((p) => (
          <ProfileRow
            key={p.id}
            profile={p}
            selected={p.id === selectedId}
            disabled={locked}
            onSelect={() => onSelect(p.id)}
            onRename={(name) => onRename(p.id, name)}
            onDuplicate={() => onDuplicate(p.id)}
            onDelete={() => onDelete(p.id)}
          />
        ))}
        {problems.length > 0 && (
          <div className="mt-2 border-t border-neutral-800 pt-2">
            <div className="px-2 text-[11px] uppercase tracking-wide text-amber-400">Problem files</div>
            {problems.map((p) => (
              <div key={p.id} className="px-2 py-1 text-[11px] text-neutral-500" title={p.error}>
                ⚠ {p.id}.json
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="flex items-center justify-between border-t border-neutral-800 p-2">
        <button
          type="button"
          disabled={locked}
          onClick={onCreate}
          className="rounded px-2 py-1 text-[12px] text-neutral-200 enabled:hover:bg-neutral-800 disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          + New
        </button>
        <button
          type="button"
          onClick={onOpenSettings}
          className="rounded px-2 py-1 text-[12px] text-neutral-300 hover:bg-neutral-800 focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          ⚙ Settings
        </button>
      </div>
    </aside>
  );
}
