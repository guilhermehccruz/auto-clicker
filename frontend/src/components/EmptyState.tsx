export function EmptyState({ onAdd }: { onAdd: (kind: "click" | "key") => void }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-neutral-700 p-10 text-center">
      <p className="text-neutral-100">No steps yet.</p>
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => onAdd("click")}
          className="rounded bg-emerald-500 px-3 py-1.5 font-medium text-neutral-950 hover:bg-emerald-400 focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          + Add click
        </button>
        <button
          type="button"
          onClick={() => onAdd("key")}
          className="rounded border border-neutral-700 px-3 py-1.5 font-medium text-neutral-200 hover:bg-neutral-800 focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          + Add key
        </button>
      </div>
      <p className="text-[12px] text-neutral-500">
        Each step is one action; enabled steps run top to bottom.
      </p>
    </div>
  );
}
