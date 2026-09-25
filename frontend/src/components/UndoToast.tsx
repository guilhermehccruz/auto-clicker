import { useEffect } from "react";

// UndoToast shows a 5s undo affordance for a deleted module (DESIGN §5.3).
export function UndoToast({
  message,
  onUndo,
  onDismiss,
}: {
  message: string;
  onUndo: () => void;
  onDismiss: () => void;
}) {
  useEffect(() => {
    const t = setTimeout(onDismiss, 5000);
    return () => clearTimeout(t);
  }, [onDismiss]);

  return (
    <div className="fixed bottom-4 left-1/2 z-50 -translate-x-1/2 rounded-lg border border-neutral-700 bg-neutral-800 px-4 py-2 text-[12px] shadow-lg">
      <span className="text-neutral-200">{message}</span>
      <button type="button" onClick={onUndo} className="ml-3 font-medium text-indigo-400 hover:text-indigo-300">
        Undo
      </button>
    </div>
  );
}
