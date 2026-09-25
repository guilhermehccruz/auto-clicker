import { useState } from "react";
import type { ProfileSummary } from "../api/types";
import { Dropdown, MenuItem } from "./Dropdown";

// ProfileRow: select, inline rename, duplicate, delete (confirm for delete).
export function ProfileRow({
  profile,
  selected,
  disabled,
  onSelect,
  onRename,
  onDuplicate,
  onDelete,
}: {
  profile: ProfileSummary;
  selected: boolean;
  disabled: boolean;
  onSelect: () => void;
  onRename: (name: string) => void;
  onDuplicate: () => void;
  onDelete: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(profile.name);

  const commit = () => {
    setEditing(false);
    if (name.trim() && name !== profile.name) onRename(name.trim());
  };

  return (
    <div
      className={`group flex items-center gap-2 rounded px-2 py-1.5 ${
        selected ? "bg-neutral-800" : "hover:bg-neutral-800/50"
      }`}
    >
      {editing ? (
        <input
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => e.key === "Enter" && commit()}
          className="min-w-0 flex-1 rounded border border-neutral-600 bg-neutral-900 px-1.5 py-0.5 text-neutral-100"
        />
      ) : (
        <button
          type="button"
          onClick={onSelect}
          onDoubleClick={() => !disabled && setEditing(true)}
          className="min-w-0 flex-1 text-left"
        >
          <span className="block truncate text-neutral-100">{profile.name}</span>
          <span className="block text-[11px] text-neutral-500">
            {profile.modules} modules · {profile.enabled} on
          </span>
        </button>
      )}
      <Dropdown
        label="⋮"
        ariaLabel="Profile menu"
        disabled={disabled}
        className="rounded px-1 text-neutral-500 opacity-60 hover:opacity-100 focus-visible:opacity-100"
      >
        <MenuItem onClick={() => setEditing(true)} disabled={disabled}>Rename</MenuItem>
        <MenuItem onClick={onDuplicate} disabled={disabled}>Duplicate</MenuItem>
        <MenuItem
          disabled={disabled}
          danger
          onClick={() => {
            if (confirm(`Delete "${profile.name}"? This removes the file on disk.`)) onDelete();
          }}
        >
          Delete…
        </MenuItem>
      </Dropdown>
    </div>
  );
}
