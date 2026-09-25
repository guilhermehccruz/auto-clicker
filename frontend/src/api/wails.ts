import { Events } from "@wailsio/runtime";
import * as Svc from "../../bindings/auto-clicker/internal/service";
import type { Api } from "./bindings";
import type {
  Capabilities,
  Diagnostics,
  DisplayInfo,
  Issue,
  Profile,
  ProfileSummary,
  ProblemFile,
  Progress,
  Settings,
} from "./types";

// WailsApi adapts the generated bindings to the UI's Api interface. It is used
// when running inside the Wails webview; `vite dev` without the shell uses the
// mock instead (see createApi).
export class WailsApi implements Api {
  async listProfiles(): Promise<{ profiles: ProfileSummary[]; problems: ProblemFile[] }> {
    const [profiles, problems] = await Svc.Service.ListProfiles();
    return {
      profiles: (profiles ?? []) as unknown as ProfileSummary[],
      problems: (problems ?? []) as unknown as ProblemFile[],
    };
  }

  async loadProfile(id: string): Promise<Profile> {
    return (await Svc.Service.LoadProfile(id)) as unknown as Profile;
  }

  async saveProfile(id: string, p: Profile): Promise<void> {
    await Svc.Service.SaveProfile(id, p as never);
  }

  async createProfile(name: string): Promise<string> {
    return Svc.Service.CreateProfile(name);
  }

  async renameProfile(id: string, name: string): Promise<string> {
    return Svc.Service.RenameProfile(id, name);
  }

  async duplicateProfile(id: string): Promise<string> {
    return Svc.Service.DuplicateProfile(id);
  }

  async deleteProfile(id: string): Promise<void> {
    await Svc.Service.DeleteProfile(id);
  }

  async updateDisplaySnapshot(id: string): Promise<void> {
    await Svc.Service.UpdateDisplaySnapshot(id);
  }

  async validate(p: Profile): Promise<Issue[]> {
    return ((await Svc.Service.Validate(p as never)) ?? []) as unknown as Issue[];
  }

  async capabilities(): Promise<Capabilities> {
    return (await Svc.Service.Capabilities()) as unknown as Capabilities;
  }

  async diagnostics(): Promise<Diagnostics> {
    return (await Svc.Service.Diagnostics()) as unknown as Diagnostics;
  }

  async displays(): Promise<DisplayInfo[]> {
    return ((await Svc.Service.Displays()) ?? []) as unknown as DisplayInfo[];
  }

  async cursorPos(): Promise<{ x: number; y: number; ok: boolean }> {
    const [x, y, ok] = await Svc.Service.CursorPos();
    return { x, y, ok };
  }

  async startPick(): Promise<void> {
    await Svc.Service.StartPick();
  }

  async confirmPick(): Promise<void> {
    await Svc.Service.ConfirmPick();
  }

  async cancelPick(): Promise<void> {
    await Svc.Service.CancelPick();
  }

  onPickDone(cb: (r: { x: number; y: number; ok: boolean }) => void): () => void {
    return Events.On("pick:done", (e) => cb(e.data as { x: number; y: number; ok: boolean }));
  }

  async run(id: string): Promise<void> {
    await Svc.Service.Run(id);
  }

  async stop(): Promise<void> {
    await Svc.Service.Stop();
  }

  async pause(): Promise<void> {
    await Svc.Service.Pause();
  }

  async resume(): Promise<void> {
    await Svc.Service.Resume();
  }

  async panic(): Promise<void> {
    await Svc.Service.Panic();
  }

  async status(): Promise<Progress> {
    return (await Svc.Service.Status()) as unknown as Progress;
  }

  async getSettings(): Promise<Settings> {
    return (await Svc.Service.GetSettings()) as unknown as Settings;
  }

  async saveSettings(s: Settings): Promise<void> {
    await Svc.Service.SaveSettings(s as never);
  }

  async profilesDir(): Promise<string> {
    return Svc.Service.ProfilesDir();
  }

  async openProfilesDir(): Promise<void> {
    await Svc.Service.OpenProfilesDir();
  }

  async openLogDir(): Promise<void> {
    await Svc.Service.OpenLogDir();
  }

  async version(): Promise<string> {
    return Svc.Service.Version();
  }

  onProgress(cb: (p: Progress) => void): () => void {
    return Events.On("run:progress", (e) => cb(e.data as Progress));
  }
}
