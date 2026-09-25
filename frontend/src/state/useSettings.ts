import { useCallback, useEffect, useState } from "react";
import type { Api } from "../api/bindings";
import type { Capabilities, Diagnostics, Settings } from "../api/types";

export interface SettingsState {
  settings: Settings | null;
  capabilities: Capabilities | null;
  diagnostics: Diagnostics | null;
  save: (s: Settings) => Promise<void>;
  refreshDiagnostics: () => Promise<void>;
}

export function useSettings(api: Api): SettingsState {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [capabilities, setCapabilities] = useState<Capabilities | null>(null);
  const [diagnostics, setDiagnostics] = useState<Diagnostics | null>(null);

  const refreshDiagnostics = useCallback(async () => {
    setDiagnostics(await api.diagnostics());
    setCapabilities(await api.capabilities());
  }, [api]);

  useEffect(() => {
    (async () => {
      setSettings(await api.getSettings());
      await refreshDiagnostics();
    })();
  }, [api, refreshDiagnostics]);

  const save = useCallback(
    async (s: Settings) => {
      await api.saveSettings(s);
      setSettings(s);
    },
    [api],
  );

  return { settings, capabilities, diagnostics, save, refreshDiagnostics };
}
