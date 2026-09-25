import { useCallback, useEffect, useRef, useState } from "react";
import type { Api } from "../api/bindings";
import type { Group, Issue, Module, Profile, ProfileSummary, ProblemFile } from "../api/types";

// UNGROUPED is the container id for the top-level (ungrouped) module list.
export const UNGROUPED = "";

export interface ProfilesState {
  profiles: ProfileSummary[];
  problems: ProblemFile[];
  selectedId: string | null;
  profile: Profile | null;
  issues: Issue[];
  dirty: boolean;
  saved: boolean;
  error: string | null;
  select: (id: string) => Promise<void>;
  create: (name: string) => Promise<void>;
  rename: (id: string, name: string) => Promise<void>;
  duplicate: (id: string) => Promise<void>;
  remove: (id: string) => Promise<void>;
  update: (mutate: (p: Profile) => Profile) => void;

  addModule: (containerId?: string) => string;
  removeModule: (id: string) => void;
  restoreModule: (m: Module, containerId: string, index: number) => void;
  moveModule: (id: string, toContainer: string, toIndex: number) => void;
  duplicateModule: (id: string) => void;
  updateModule: (id: string, patch: Partial<Module>) => void;

  addGroup: (name: string) => void;
  renameGroup: (id: string, name: string) => void;
  setGroupEnabled: (id: string, enabled: boolean) => void;
  removeGroup: (id: string) => void;
  moveGroup: (from: number, to: number) => void;
}

const AUTOSAVE_MS = 500;

function normalize(p: Profile): Profile {
  p.modules = p.modules ?? [];
  p.groups = (p.groups ?? []).map((g) => ({ ...g, modules: g.modules ?? [] }));
  return p;
}

function listOf(p: Profile, containerId: string): Module[] | null {
  if (containerId === UNGROUPED) return p.modules;
  return p.groups.find((g) => g.id === containerId)?.modules ?? null;
}

function takeModule(p: Profile, id: string): Module | null {
  const i = p.modules.findIndex((m) => m.id === id);
  if (i >= 0) return p.modules.splice(i, 1)[0];
  for (const g of p.groups) {
    const j = g.modules.findIndex((m) => m.id === id);
    if (j >= 0) return g.modules.splice(j, 1)[0];
  }
  return null;
}

function insertModule(p: Profile, containerId: string, index: number, m: Module) {
  const list = listOf(p, containerId) ?? p.modules;
  list.splice(Math.max(0, Math.min(index, list.length)), 0, m);
}

function locate(p: Profile, id: string): { containerId: string; index: number } | null {
  const i = p.modules.findIndex((m) => m.id === id);
  if (i >= 0) return { containerId: UNGROUPED, index: i };
  for (const g of p.groups) {
    const j = g.modules.findIndex((m) => m.id === id);
    if (j >= 0) return { containerId: g.id, index: j };
  }
  return null;
}

export function useProfiles(api: Api): ProfilesState {
  const [profiles, setProfiles] = useState<ProfileSummary[]>([]);
  const [problems, setProblems] = useState<ProblemFile[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [profile, setProfile] = useState<Profile | null>(null);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [dirty, setDirty] = useState(false);
  const [saved, setSaved] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const profileRef = useRef<Profile | null>(null);
  profileRef.current = profile;

  const refresh = useCallback(async () => {
    const { profiles, problems } = await api.listProfiles();
    setProfiles(profiles);
    setProblems(problems);
    return profiles;
  }, [api]);

  const select = useCallback(
    async (id: string) => {
      const p = normalize(await api.loadProfile(id));
      setSelectedId(id);
      setProfile(p);
      setDirty(false);
      setSaved(true);
      setIssues(await api.validate(p));
    },
    [api],
  );

  useEffect(() => {
    (async () => {
      try {
        const list = await refresh();
        if (list.length > 0) await select(list[0].id);
      } catch (e) {
        setError(String(e));
      }
    })();
  }, [refresh, select]);

  const persist = useCallback(
    async (p: Profile) => {
      if (!selectedId) return;
      try {
        await api.saveProfile(selectedId, p);
        setSaved(true);
        setError(null);
        await refresh();
      } catch (e) {
        setError(String(e));
      }
    },
    [api, selectedId, refresh],
  );

  const update = useCallback(
    (mutate: (p: Profile) => Profile) => {
      setProfile((prev) => {
        if (!prev) return prev;
        const next = normalize(mutate(structuredClone(prev)));
        setDirty(true);
        setSaved(false);
        void api.validate(next).then(setIssues);
        if (saveTimer.current) clearTimeout(saveTimer.current);
        saveTimer.current = setTimeout(() => void persist(next), AUTOSAVE_MS);
        return next;
      });
    },
    [api, persist],
  );

  const flush = useCallback(() => {
    if (saveTimer.current) {
      clearTimeout(saveTimer.current);
      saveTimer.current = null;
    }
    if (profileRef.current && dirty) void persist(profileRef.current);
  }, [dirty, persist]);

  useEffect(() => {
    const onBlur = () => flush();
    window.addEventListener("blur", onBlur);
    window.addEventListener("beforeunload", onBlur);
    return () => {
      window.removeEventListener("blur", onBlur);
      window.removeEventListener("beforeunload", onBlur);
    };
  }, [flush]);

  const create = useCallback(
    async (name: string) => {
      const id = await api.createProfile(name);
      await refresh();
      await select(id);
    },
    [api, refresh, select],
  );

  const rename = useCallback(
    async (id: string, name: string) => {
      const nid = await api.renameProfile(id, name);
      await refresh();
      if (id === selectedId) await select(nid);
    },
    [api, refresh, select, selectedId],
  );

  const duplicate = useCallback(
    async (id: string) => {
      await api.duplicateProfile(id);
      await refresh();
    },
    [api, refresh],
  );

  const remove = useCallback(
    async (id: string) => {
      await api.deleteProfile(id);
      const list = await refresh();
      if (id === selectedId) {
        if (list.length > 0) await select(list[0].id);
        else {
          setSelectedId(null);
          setProfile(null);
        }
      }
    },
    [api, refresh, select, selectedId],
  );

  // addModule returns the new module id so the caller can expand it.
  const addModule = useCallback(
    (containerId: string = UNGROUPED) => {
      const m = newModule();
      update((p) => {
        insertModule(p, containerId, Number.MAX_SAFE_INTEGER, m);
        return p;
      });
      return m.id;
    },
    [update],
  );

  const removeModule = useCallback((id: string) => update((p) => (takeModule(p, id), p)), [update]);

  const restoreModule = useCallback(
    (m: Module, containerId: string, index: number) =>
      update((p) => {
        insertModule(p, containerId, index, m);
        return p;
      }),
    [update],
  );

  const moveModule = useCallback(
    (id: string, toContainer: string, toIndex: number) =>
      update((p) => {
        const m = takeModule(p, id);
        if (m) insertModule(p, toContainer, toIndex, m);
        return p;
      }),
    [update],
  );

  const duplicateModule = useCallback(
    (id: string) =>
      update((p) => {
        const loc = locate(p, id);
        if (!loc) return p;
        const src = listOf(p, loc.containerId);
        const copy = { ...structuredClone(src![loc.index]), id: newModule().id };
        src!.splice(loc.index + 1, 0, copy);
        return p;
      }),
    [update],
  );

  const updateModule = useCallback(
    (id: string, patch: Partial<Module>) =>
      update((p) => {
        const loc = locate(p, id);
        if (loc) {
          const list = listOf(p, loc.containerId)!;
          list[loc.index] = { ...list[loc.index], ...patch };
        }
        return p;
      }),
    [update],
  );

  const addGroup = useCallback(
    (name: string) =>
      update((p) => {
        p.groups.push(newGroup(name));
        return p;
      }),
    [update],
  );

  const renameGroup = useCallback(
    (id: string, name: string) =>
      update((p) => {
        const g = p.groups.find((x) => x.id === id);
        if (g) g.name = name;
        return p;
      }),
    [update],
  );

  const setGroupEnabled = useCallback(
    (id: string, enabled: boolean) =>
      update((p) => {
        const g = p.groups.find((x) => x.id === id);
        if (g) g.enabled = enabled;
        return p;
      }),
    [update],
  );

  const removeGroup = useCallback(
    (id: string) =>
      update((p) => {
        const idx = p.groups.findIndex((g) => g.id === id);
        if (idx < 0) return p;
        // Keep the modules: move them to the ungrouped list rather than deleting.
        p.modules.push(...p.groups[idx].modules);
        p.groups.splice(idx, 1);
        return p;
      }),
    [update],
  );

  const moveGroup = useCallback(
    (from: number, to: number) =>
      update((p) => {
        if (from < 0 || from >= p.groups.length || to < 0 || to >= p.groups.length) return p;
        const [g] = p.groups.splice(from, 1);
        p.groups.splice(to, 0, g);
        return p;
      }),
    [update],
  );

  return {
    profiles, problems, selectedId, profile, issues, dirty, saved, error,
    select, create, rename, duplicate, remove, update,
    addModule, removeModule, restoreModule, moveModule, duplicateModule, updateModule,
    addGroup, renameGroup, setGroupEnabled, removeGroup, moveGroup,
  };
}

let idCounter = 0;

function rand(): string {
  idCounter += 1;
  return Math.random().toString(16).slice(2, 10) + idCounter.toString(16);
}

export function newModule(): Module {
  return {
    kind: "click",
    id: `m_${rand()}`,
    name: "",
    enabled: true,
    target: { kind: "cursor" },
    button: "left",
    count: 1,
    clickInterval: 0,
    holdMs: 10,
    delayAfter: 250,
  };
}

export function newGroup(name: string): Group {
  return { id: `g_${rand()}`, name: name || "New group", enabled: true, modules: [] };
}
