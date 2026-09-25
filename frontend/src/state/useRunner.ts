import { useCallback, useEffect, useState } from "react";
import type { Api } from "../api/bindings";
import type { Progress } from "../api/types";

export interface RunnerState {
  progress: Progress;
  running: boolean;
  paused: boolean;
  run: (id: string) => Promise<void>;
  stop: () => Promise<void>;
  pause: () => Promise<void>;
  resume: () => Promise<void>;
  panic: () => Promise<void>;
}

const idle: Progress = {
  state: "idle", profileName: "", moduleId: "", moduleLabel: "",
  moduleIndex: 0, moduleTotal: 0, clickIndex: 0, clickTotal: 0,
  loopIteration: 0, clicks: 0, elapsedMs: 0,
};

export function useRunner(api: Api): RunnerState {
  const [progress, setProgress] = useState<Progress>(idle);

  useEffect(() => api.onProgress(setProgress), [api]);
  useEffect(() => {
    void api.status().then(setProgress);
  }, [api]);

  const run = useCallback(
    async (id: string) => {
      await api.run(id);
    },
    [api],
  );

  return {
    progress,
    running: progress.state === "running" || progress.state === "stopping",
    paused: progress.state === "paused",
    run,
    stop: () => api.stop(),
    pause: () => api.pause(),
    resume: () => api.resume(),
    panic: () => api.panic(),
  };
}
