export function EmptyState({ onAdd }: { onAdd: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-neutral-700 p-10 text-center">
      <p className="text-neutral-100">No clicks yet.</p>
      <button
        type="button"
        onClick={onAdd}
        className="rounded bg-emerald-500 px-3 py-1.5 font-medium text-neutral-950 hover:bg-emerald-400 focus-visible:ring-2 focus-visible:ring-indigo-500"
      >
        + Add click
      </button>
      <p className="text-[12px] text-neutral-500">
        Each click is one step; enabled steps run top to bottom.
      </p>
    </div>
  );
}
