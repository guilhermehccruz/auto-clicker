# Auto Clicker — Design Document

A **single-purpose autoclicker** for repetitive clicking in clicker/idle games and any
other "click this spot N times" chore. A Go engine plus a React/TS UI, packaged as one
desktop binary per OS (Wails).

The unit of composition is an ordered, toggleable **click module**. Enabled modules run
**strictly sequentially**, each clicking at fixed screen coordinates **or at the current
cursor position**. A run requires at least one enabled module — the app never clicks
implicitly.

**Status:** design phase. The stack below is not speculative: it was validated by a spike
on Kubuntu/Plasma Wayland (results in [Appendix C](#appendix-c-stack-validation-results)),
with the Windows runtime check still pending. Sections marked **[unproven]** rest on
assumptions that a short spike must confirm before implementation
(see [§8 Phase 0](#8-phases)).

**Self-contained by design.** This document is the full specification for this app; it
does not require reading any other repository. When the project gets its own repo, this
file becomes that repo's `DESIGN.md`. Appendix B carries the platform protocol notes
verbatim because they are hard-won, silent-failure knowledge that would otherwise be
lost.

---

## 1. Goals & Non-Goals

### Goals
- Compose a run from ordered, toggleable **click modules**; enabled modules execute
  **strictly sequentially**, in list order.
- Each module clicks either at **absolute screen coordinates** or **at the current cursor
  position** (without moving the pointer).
- Run once, N times, or forever, with drift-free timing.
- Fast, drag-and-drop UI: profiles sidebar + flat module list + telemetry bar.
- Ships as **one executable per OS** (Linux + Windows), no installer, no runtime deps
  beyond the OS webview.
- Safe: global stop hotkey, runtime cap, no data loss, no single-instance conflicts.

### Non-Goals
- **Implicit / fallback runs.** There is no "if nothing is enabled, just click" mode.
  Clicking at the cursor is available *explicitly*, as a module target kind. A pipeline
  with no enabled module **blocks Run** ([§3.1](#31-execution-semantics)).
- **Keyboard, typing, scroll, hotkey actions.** This is *just* an autoclicker. A general
  macro builder is a different product.
- Input recorder, image/vision triggers, OCR, window binding.
- **A global CPS knob.** Rate is expressed by module fields (`holdMs`, `delayAfter`)
  and the profile's `loopDelay`. Nothing else sets the rate.
- Parallel/concurrent lanes — strictly sequential.
- Mobile/ADB automation, browser/DOM automation.
- macOS, and Linux desktops other than KDE/Wayland + X11 sessions (see
  [§3.6](#36-platform-scope)).

---

## 2. Stack

| Layer | Choice | Notes |
|---|---|---|
| Engine | Go 1.26 | monotonic-clock scheduler, single binary, cross-compile |
| Input | `github.com/go-vgo/robotgo` @ pinned master | `CGO_ENABLED=0`, pure-Go backends |
| Shell / IPC | Wails v3 (`v3.0.0-beta.25`) | native WebView2 (Win) / WebKitGTK (Linux), auto TS bindings |
| UI | React + TypeScript + Tailwind + `@dnd-kit` | drag-reorder, component forms |
| Storage | JSON, one file per profile | human-readable, git-diffable, copyable |
| Packaging | Wails build | `linux/amd64`, `windows/amd64` from one machine |

**Rejected:** Electron (robotjs unmaintained, nut.js went closed-source/EULA),
Python (weaker Wayland input, heavier packaging, harder single-binary story).

### 2.1 robotgo version pin (important)

The pure-Go, cgo-free backends exist **only on robotgo `master`**. No tagged release is
consumable: `@latest` resolves to `v1.0.2` (cgo-only), and the `v2.0.0-beta*` tags are
unusable because their `go.mod` lacks the required `/v2` module path, so `go get` rejects
them. We therefore pin an exact pseudo-version:

```
github.com/go-vgo/robotgo v1.0.3-0.20260921150940-12f16b7c5d82  // = master @ 2026-09-21
```

(`@master` resolves as a `v1.0.3` pre-release pseudo-version, sidestepping the `/v2` rule.)

Trade-off acknowledged: this is upstream's *experimental* code (their own commit messages
say so) and is not tagged or stable. Mitigations:

- the exact commit is pinned in `go.sum`, so builds are reproducible;
- the spike exercises every backend we ship;
- the `Driver` interface ([§7](#7-code-structure)) contains the blast radius if robotgo
  must be swapped for a hand-rolled backend later.

### 2.2 Backend selection

robotgo's *root package* backends are mutually exclusive at compile time (build tags
`win` / `x11` / `libei`; combining tags fails because they declare overlapping symbols).
The underlying **sub-packages are not exclusive**: `robotgo/x11`, `robotgo/libei`
(`//go:build linux`) and `robotgo/win` (`//go:build windows`) expose the same API
(`Move`, `Click`, `KeyTap`, `Location`, `Type`, `Scroll`, `GetScreenSize`, …).

**Decision: import the sub-packages directly, never the root package.**

```
internal/input/backend_linux.go    //go:build linux   → x11 + libei, picked at runtime
internal/input/backend_windows.go  //go:build windows → win
```

Result: **one binary per OS**, and on Linux a *runtime* choice:

| Backend | Reaches | Role |
|---|---|---|
| `libei` (portal `RemoteDesktop`) | all windows, native-Wayland and XWayland | **preferred** on Wayland |
| `x11` (XTEST via XWayland) | XWayland/X11 clients only | X11 sessions; fallback when libei is unavailable/denied |
| `win` (SendInput) | all Windows apps | Windows 10 |

Backend selection on Linux happens at startup ([§4.5](#45-startup-and-shutdown)): an
**X11 session** uses `x11` directly. On a **Wayland session** the app starts on `x11`
(usable immediately, no dialog) and then tries `libei` — automatically when a cached
consent token lets initialization resolve without user interaction, otherwise through an
explicit **"Enable native Wayland input"** action in the UI, so the portal consent dialog
is never a launch-time surprise. Once `libei` is ready it becomes the active backend for
subsequent runs; if it fails, `x11` stays and the reason is recorded for the diagnostics
panel ([§5.5](#55-settings--diagnostics)). The token is a plain file, so the app checks
it directly rather than probing with a dialog (see below).

`libei` limitations that shape this design:
- **It cannot read the physical cursor position.** `Location()` returns the last
  *injected* position only. This is *fine* for the core "click at cursor" feature (we
  inject a button event **with no preceding motion**, so we never need to know the
  coordinates) but it breaks anything that must *know* the pointer: the coordinate
  picker, plus cursor telemetry. Those go
  through a separate, backend-specific `CursorPos()` query path.
- Screen capture / window helpers return `ErrNotSupported` — irrelevant here (no vision).
- First run shows a portal consent dialog; the grant is cached (robotgo stores a restore
  token under `$XDG_STATE_HOME/robotgo`).
- **Absolute motion and screen geometry require a linked ScreenCast stream.** robotgo's
  libei backend sends `NotifyPointerMotionAbsolute` only on a stream negotiated through
  `org.freedesktop.portal.ScreenCast` (its `LinkScreenCast`, default true). Without a
  stream, `Move` falls back to relative deltas seeded by a large "corner reset" that
  **parks the pointer in the top-left corner** — and because libei cannot read the real
  cursor, any physical mouse movement desynchronizes the tracked position, so `absolute`
  targets land wrong. Consequence: on Wayland the app requests the ScreenCast source (one
  extra "share screen" step inside the consent flow) so `absolute` targets and display
  geometry work. `cursor` targets need none of it. If the user declines ScreenCast,
  `DisplaysNum()` is `0`, `cursor`/relative input still works, and the app
  capability-gates `absolute` ([§4.4](#44-error-handling), [§3.4](#34-targets)).
- **The restore token is a plain file**, `$XDG_STATE_HOME/robotgo/portal_token`
  (fallback `~/.local/state/robotgo/portal_token`). The app checks it directly to decide
  automatic versus explicit upgrade at startup ([§4.5](#45-startup-and-shutdown)) instead
  of probing with a dialog. A stale token can still re-prompt; that is handled as a normal
  failure. Note this file is **shared by any robotgo app**, not per-application.

**Deliberately dropped:** robotgo's `wayland` (wlroots `zwlr_*`) backend — KWin does not
implement those protocols, and on Kubuntu it reported bogus geometry (3440×1440 instead
of 5360×1440). Sway/Hyprland are out of scope.

### 2.3 Global hotkeys

robotgo **removed its hotkey/event API** (`AddHotkey` / `EventStart` are gone; the
`event/` package is mobile-only stubs). Global hotkeys therefore need their own layer, a
`HotkeySource` interface with three per-platform implementations:

| Platform | Mechanism | Notes |
|---|---|---|
| Windows | `RegisterHotKey` | pure-Go syscall, no dependencies |
| Linux X11 session | `XGrabKey` on the root window | works while any XWayland/X11 client is focused, which covers Steam/Proton games |
| Linux Wayland | `org.freedesktop.portal.GlobalShortcuts` | works on Kubuntu/Plasma 6; requires an app id and one consent dialog |

Selection mirrors libei/x11: try the portal on Wayland, fall back to `XGrabKey` (which
only fires while an XWayland client has focus), and if neither is available, fall back to
**window-focused key handling only** and show a persistent warning
([§4](#4-runner-state-machine), [§6](#6-safety)). The protocol has several
**silent-failure** traps that must be carried over verbatim — see
[Appendix B](#appendix-b-global-shortcuts-portal-gotchas).

> **Implementation note.** Wails v3 also ships a native `app.GlobalShortcut` manager. The
> hand-rolled layer above is kept because it is already written and validated against the
> real portal, and because it makes the Wayland→XGrabKey fallback chain explicit. If the
> Wails manager turns out to cover the same ground, it is a candidate simplification.

### 2.4 Module path and naming

```
module auto-clicker          // no URL prefix, matching the sibling project's convention
```

Working name: display **"Auto Clicker"**, slug **`auto-clicker`** used for the binary
name, the profiles+settings folder (`<Documents>/auto-clicker/`) and the runtime config
folder (`~/.config/auto-clicker/`, `%APPDATA%\auto-clicker\`, holding the single-instance
lock). Renaming later touches paths and
CI artifact names, so it is worth confirming before the first release — but it is cheap
now and nothing in this document depends on the brand.

The **application id** presented to the desktop portal is **`auto-clicker`** (matches the
slug, non-empty as [Appendix B.4](#appendix-b-global-shortcuts-portal-gotchas) requires).
It must be **stable**: KDE keys global shortcuts per component id, so changing it orphans
the previously granted consent and re-prompts. If a portal build rejects the non-reverse-DNS
form, the fallback id is `io.github.autoclicker.AutoClicker`.

### 2.5 Pinned toolchain

The Go side is pinned by `go.mod` / `go.sum` ([§2.1](#21-robotgo-version-pin-important)).
The UI side is pinned by `package-lock.json` at these majors, and the choice of Tailwind
**v4** (rather than v3) is deliberate — it is CSS-first, has no `tailwind.config.js`, and
adopting it now avoids a config rewrite later:

| Tool | Pin |
|---|---|
| Node | 22 LTS (`>=22 <23`) |
| React | 19 |
| TypeScript | 5.7 |
| Vite | 6 |
| Vitest | 3 |
| Tailwind CSS | 4 |
| `@dnd-kit` | current stable |

Exact patch versions live in the lockfile; CI uses the pinned Node major
([§10](#10-ci--release-pipeline-github-actions)). No floating `latest` anywhere.

### 2.6 Project metadata & licensing

- **License: MIT.** A small single-purpose desktop utility gains nothing from a copyleft
  license and loses distribution reach.
- **README** carries the support-load-bearing notes in [§6](#6-safety) (anti-cheat,
  fullscreen games, Wayland consent, physical-pixel coordinates) plus download, first-run
  and "which backend am I on" guidance.
- **App icon**: one PNG (`build/appicon.png`). Wails derives `build/windows/icon.ico`
  from it at build time (macOS icons are out of scope), and a 256×256 copy
  (`internal/desktop/icon.png`) is embedded in the binary for the Linux launcher icon
  ([§3.6](#36-platform-scope)). The window and taskbar icon ship from the derived `.ico`;
  there is no separate branding pass in v1.

---

## 3. Core Concepts & Data Model

```
Profile  (one .json file — e.g. "idle-tycoon.json"; the filename IS the profile id)
 ├── loop, failsafe, display snapshot
 └── modules[]   ordered; each = ONE click step
      ├── kind (click|key), id, name, enabled
      ├── target: absolute{x,y} | cursor
      └── button, count, holdMs, delayAfter
```

A module is a **single click step**, not a container of mixed actions. Waits are
*fields*, not steps: `holdMs` inside a click, `delayAfter` after **every** click (so it is
both the inter-click gap and the gap to the next module), `loopDelay` between passes, and
the group's `delayAfter` between groups. To insert a pause, raise the preceding module's
`delayAfter` — there is no standalone "wait" module.

### 3.1 Execution semantics

1. **Validate before anything else.** A run requires **at least one enabled module**. If
   none is enabled (empty list, or all toggled off), Run is **blocked** with an inline
   reason ([§5.6](#56-validation)). The global Run hotkey is a no-op in that state; the
   UI already shows why, so no toast is needed while the app is unfocused.
2. On Run, the runner takes a **snapshot** (deep copy) of the profile. The runner never
   reads state the UI could be mutating.
3. Enabled modules are **flattened in list order** into an execution plan: ungrouped modules
   first, then each enabled group's modules in order ([§3.12](#312-groups)). Disabled
   modules and modules in disabled groups are skipped entirely but keep their position.
   Each group runs its block **`repeat` times** (its own loop) before the next group, and
   its `delayAfter` (ms) spaces it from the following group.
4. The plan executes per the profile's loop mode:
   - `once` — one pass, then stop.
   - `count` — N passes, `loopDelay` between passes.
   - `forever` — passes until Stop.
5. **The UI is locked while running**: toggles, reordering and editing are disabled; only
   Stop / Pause are live. No mid-run mutation logic exists at all.

A **pass** walks the plan; within it, a group is its own loop:

```
for pass in loop:
    for block in plan:                 # ungrouped first, then each enabled group
        for r in 0 .. block.repeat-1:  # ungrouped repeat is always 1
            for m in block.modules:
                if m.kind is key:
                    KeyDown(m.keys); sleep(m.holdMs); KeyUp(m.keys)
                else:
                    resolve target
                    if absolute: Move(x, y)      # instant, no animation
                    if cursor:   (no motion)
                    ButtonDown(m.button); sleep(m.holdMs); ButtonUp(m.button)
                sleep(m.delayAfter)              # after EVERY action
        if block is a group and not the last block: sleep(block.delayAfter)
    if more passes: sleep(loopDelay)
```

Between consecutive passes: `sleep(loopDelay)` (a fixed value or a random `{min,max}`
window). `loopDelay` is **not** applied after the final pass of a `count` run, and the
last group's `delayAfter` is skipped because the loop delay already covers the gap.

**Hold (`holdUntilStop`)**: instead of press/sleep/release, the module presses the
button/key and keeps it down. It is terminal — the run stays active until Stop (or the
runtime cap), then `ReleaseAll` releases it. Validation therefore requires it to be the
**last enabled action** and the loop mode to be **`once`**.

### 3.2 Timing

- The scheduler uses a **monotonic clock with deadline accumulation**:
  `next = next + interval; sleepUntil(next)` — never `sleep(interval)` — so drift does
  not accumulate over thousands of clicks. Measured in the spike: p95 lateness ≈ 1 ms at
  10–20 CPS, drift −39 ms over 2.5 s.
- Every sleep is **interruptible** via context, so Stop halts **before** the next
  click is injected rather than after the current sleep. Measured stop latency: ~9 ms at
  50 CPS.
- `delayAfter` is a pause **after every click**, including the last one. So it sets both
  the inter-click period (`holdMs + delayAfter`) and the gap to the next module/group/pass.
  One knob for the whole cadence. `delayAfter = 0` means "as soon as the release lands".
- **`humanize`** (profile-level, default off). When on, every **fixed** delay —
  `delayAfter` (per click, which also spaces modules), a group's `delayAfter`, and a fixed
  `loopDelay` — is drawn uniformly in ±25% of its value, so the cadence is not perfectly
  periodic. A `{min,max}` `loopDelay` is already random and keeps its own range.
- On any exit path, held buttons are released ([§6](#6-safety)).
- **Pause gates at click boundaries** — an in-flight click (down → hold → up) always
  completes. While paused, *all* engine delays are frozen (click spacing, `delayAfter`,
  `loopDelay`), and so is the wall clock that `maxRuntimeSec` and `Progress.ElapsedMs`
  measure against. Resuming restarts the current delay from zero instead of firing
  immediately for a deadline that passed while paused.
- **`loopDelay` randomness.** A `{min,max}` window is drawn as a **uniform integer,
  inclusive of both ends** (`min == max` is therefore the fixed case — no special path).
  The draw happens **once, when the inter-pass delay starts**, and is *not* re-rolled on
  resume, so pausing cannot be used to fish for a shorter pause. The engine takes an
  injectable `Rng` (an `io.Reader`), seeded from OS entropy in production and seeded
  deterministically in tests ([§9](#9-testing)).
- **Latency budget.** Injection is sub-millisecond per call on all three backends
  ([Appendix C](#appendix-c-stack-validation-results) measured p95 scheduling lateness ≈1 ms
  at 10–20 CPS, stop within ~9 ms at 50 CPS). `holdMs` is a real sleep between down and up,
  so the effective press ≈ `holdMs` + a fraction of a ms. **Presses below ~5 ms are ignored
  by some targets**, which is why the default is 10 ms; `delayAfter` is additive to
  `holdMs` ([§3.2](#32-timing) above), so the click period is `holdMs + delayAfter`.
- **`Progress.ElapsedMs`** counts *active* run time (wall clock minus paused time) and
  resets to `0` on each Run; `maxRuntimeSec` is measured on the same active clock, so time
  spent paused never counts against the cap. `Clicks` likewise resets per run.

### 3.3 Module fields (the complete vocabulary)

| Field | Type | Default | Range | Meaning |
|---|---|---|---|---|
| `kind` | enum | `"click"` | `click`/`key` | `click` = mouse action; `key` = press a keyboard combo (target/button unused) |
| `id` | string | `m_` + 8 hex | — | stable id for DnD/React keys; unique within the profile |
| `name` | string | `""` | ≤ 80 chars | optional user label; when empty the UI shows the generated summary (`(940, 520) ×3`) |
| `enabled` | bool | `true` | — | participates in the run; keeps its list slot when off |
| `target` | object | `{"kind":"absolute"}` | — | `absolute {x,y}` or `cursor` ([§3.4](#34-targets)); a new module has no coordinates yet, so validation asks for them |
| `button` | enum | `left` | `left`/`right`/`middle`/`none` | mouse button; `none` = **move only** (absolute target: moves without clicking; `holdMs` is ignored) |
| `keys` | string | `""` | combo | key combo for `kind: key`, lowercase robotgo names (`ctrl+shift+c`, `f5`, `space`); validation blocks an empty combo |
| `holdUntilStop` | bool | `false` | — | press the button/key and **keep it down until Stop**. Must be the last enabled module, with loop `once` ([§3.1](#31-execution-semantics)) |
| `count` | int | `1` | 1–1000 | actions per pass (burst size, *not* a rate) |
| `holdMs` | int (ms) | `10` | 0–1000 | how long the button/key stays down per action |
| `delayAfter` | int (ms) | `50` | 0–3600000 | pause after **every** action (period = `holdMs + delayAfter`; also the gap to the next module) |
| `clickInterval` | int (ms) | `0` | 0–60000 | **deprecated**; folded into `delayAfter` on load, no longer edited or run |

Design notes:
- **Safe defaults.** A freshly added click is `absolute`-targeted **with no coordinates**,
  so validation blocks the run until the user picks a point (nothing is clicked at 0,0 by
  accident); a freshly added key module has an empty combo and is blocked until recorded.
  `delayAfter: 50` keeps a configured run at a sane rate. `count`
  caps at 1000 because continuous clicking is what `loop: forever` is for; the card shows
  a hint to that effect when the user types a large value.
- `holdMs` defaults to 10 ms rather than 0 because some targets ignore presses shorter
  than a few milliseconds. Setting it to 0 clicks as fast as the backend allows.
- Everything is expressed in **milliseconds**, integers, to keep JSON diff-friendly.
- **`id` generation:** `m_` + 8 lowercase hex chars from `crypto/rand`, re-drawn until
  it is unique within the profile. It is opaque and never reused, so reordering and
  duplicate-then-edit never alias two cards.
- **`name` is user-editable** (an optional "label" field on the expanded card). It is a
  *label only* — it never affects execution, validation or summaries, which are always
  derived from the fields.

### 3.4 Targets

| Kind | Fields | Behavior | Phase |
|---|---|---|---|
| `absolute` | `x`, `y` (canonical physical px, [§3.5](#35-coordinate-space-and-display-changes)) | move the pointer instantly, then click | v1 |
| `cursor` | — | click **without moving**; the pointer stays where the user left it | v1 |
| `relativeToCursor` | `dx`, `dy` | move by offset from the real cursor | deferred |
| `windowRelative` | `dx`, `dy` + window binding | resolution-independent | deferred |
| `image` | template + region | vision-anchored | out of scope |

A module with `button: "none"` is a **move step**: for an `absolute` target it just moves the
pointer to `(x, y)` (no button event, `holdMs` ignored) — handy to reposition the pointer
before a later `cursor` module. With a `cursor` target it does nothing (validation warns).

- **Why `cursor` needs no cursor read:** the injection is a button event with no
  preceding motion, so the OS/compositor clicks wherever the pointer already is. We never
  have to *know* the coordinates. This is precisely what makes `cursor` work on libei,
  which cannot read the real cursor ([§2.2](#22-backend-selection)) — and it is the first
  thing the blocking spike must confirm **[unproven]** ([§8](#8-phases), [Appendix C](#appendix-c-stack-validation-results)).
- **`absolute` on Wayland/libei needs the ScreenCast stream** ([§2.2](#22-backend-selection)):
  it is the only path that sends true absolute motion. Without it the backend degrades to
  relative moves and clicks can land wrong after physical mouse movement, so the app gates
  `absolute` on `Capabilities.CanMoveAbsolute`. On X11 and Windows `absolute` is always
  available.
- `relativeToCursor` is **deferred**, unlike a naive "click at cursor" implementation:
  it requires *reading* the real cursor (a hard problem on Wayland), while `cursor` gives
  the same practical value with no read at all. Revisit only after `CursorPos()` is solid.
- Coordinates are **physical pixels in the canonical space anchored at the primary
  monitor's top-left** ([§3.5](#35-coordinate-space-and-display-changes)); monitors left
  of/above the primary give **negative** coordinates, so a profile is portable across
  machines as long as the point is on the primary monitor.
- v1 is honest about being absolute-coordinate based and says so in the UI.

### 3.5 Coordinate space and display changes

Decision: **store physical pixels in one canonical space**, plus a snapshot of the
desktop geometry the coordinates were captured against:

> **Canonical origin = the primary monitor's top-left**, X right and Y down. This is the
> native Windows convention, so on Windows coordinates are stored as-is. On Linux/X11 the
> root origin is the bounding box's top-left, which depends on how monitors are arranged;
> the backend translates in both directions by the primary monitor's root origin
> (`canonical = root − primaryOrigin` when reading the cursor, `root = canonical +
> primaryOrigin` when injecting). A canonical space anchored at the **bounding box** would
> instead give the same physical point different numbers on two machines whose monitor
> arrangements differ; anchoring at the **primary** makes a point on the primary monitor
> mean the same thing everywhere (which is the point of a shareable profile).
>
> Consequence: a monitor **left of / above** the primary yields **negative** coordinates
> (legal), and a monitor right/below yields coordinates beyond the primary's size. Bounds
> for validation are the union of the display rectangles in this space.

```jsonc
"display": { "width": 5360, "height": 1440 }
```

Rationale: physical pixels are what all three backends inject in, and they are stable
under OS zoom/scaling changes. On load, if the current desktop geometry differs from the
snapshot, the app shows a **non-blocking warning banner** ("this profile was recorded for
5360×1440; the current desktop is 2560×1440 — verify your coordinates") with a shortcut to
re-pick. This turns the classic "my clicker clicks the wrong place on the other machine"
bug into a visible, fixable notice.

The snapshot is **refreshed only when coordinates change** — on profile creation, and
whenever a coordinate is picked or typed — plus an
explicit **"Update to current desktop"** action available from the warning banner.
Ordinary edits (`button`, `count`, `delayAfter`, enable/disable) never touch it, so a
profile opened and edited on machine B keeps the geometry it was actually authored
against and the warning keeps meaning something. The banner carries both a "Re-pick"
shortcut and "Update to current desktop".

Coordinates outside the current desktop produce a **warning chip on the module card, not
a blocking validation error** — a profile may legitimately be used on a different
machine, and we prefer a runnable profile with a visible warning over an unrunnable one.

Mixed-DPI correctness is a spike item **[unproven]** (see
[§8 Phase 0.5](#8-phases)): it must be verified that a coordinate picked on a 150 %-scaled
monitor lands on the same physical pixel at injection time, on both X11 and Wayland. If
the backends turn out to disagree, the mitigation is to record the coordinate *and* the
monitor layout (origin + size per output) so the picker can convert; the schema has room
for a `displays[]` array later.

### 3.6 Platform scope

| Platform | Support |
|---|---|
| Windows 10 / 11 | target (runtime test pending) |
| Kubuntu / KDE Plasma 6, Wayland | primary development target, validated |
| Other Wayland compositors | unsupported (no libei portal guarantee, no tested grab) |
| X11 sessions | supported via `x11` + `XGrabKey` |
| macOS | out of scope |

On Linux the app registers itself in the desktop's application search on first launch:
it writes `~/.local/share/applications/auto-clicker.desktop` and the hicolor icon
`~/.local/share/icons/hicolor/256x256/apps/auto-clicker.png` (idempotent, best-effort,
never fatal). This is also what lets the GlobalShortcuts portal resolve the app
([Appendix B.4](#appendix-b-global-shortcuts-portal-gotchas)); the launcher entry and the
portal registration intentionally share the `auto-clicker` id.

### 3.7 Example profile

`idle-tycoon.json`:

```jsonc
{
  "schemaVersion": 1,
  "name": "Idle Tycoon",
  "display": { "width": 5360, "height": 1440 },
  "loop": { "mode": "forever", "count": 10, "loopDelay": { "min": 500, "max": 1500 } },
  "failsafe": { "maxRuntimeSec": 3600 },
  "modules": [
    { "kind": "click", "id": "m1", "enabled": true, "delayAfter": 90,
      "target": { "kind": "absolute", "x": 940, "y": 520 },
      "button": "left", "count": 3, "holdMs": 10 },
    { "kind": "click", "id": "m2", "enabled": true, "delayAfter": 400,
      "target": { "kind": "cursor" },
      "button": "left", "count": 1, "holdMs": 10 },
    { "kind": "click", "id": "m3", "enabled": false, "delayAfter": 250,
      "target": { "kind": "absolute", "x": 100, "y": 100 },
      "button": "right", "count": 1, "holdMs": 10 }
  ]
}
```

Run of the above: click `(940,520)` ×3, 90 ms after each (including the last) → click at
the cursor once → 400 ms → end of pass → wait a random 500–1500 ms → repeat forever. `m3`
never fires.

### 3.8 Where profiles are stored

Profiles live in a user-visible folder, one JSON file per profile, alongside the
app's files:

```text
Documents/auto-clicker/        # app folder
  app.log
  settings.json
  runs.jsonl
  profiles/                    # one <slug>.json per profile
    idle-tycoon.json
```

| OS | Path resolution |
|---|---|
| Windows | `%USERPROFILE%\Documents\auto-clicker\` — resolved with `SHGetKnownFolderPath(FOLDERID_Documents)` |
| Linux | `$XDG_DOCUMENTS_DIR/auto-clicker/` (via `xdg-user-dir DOCUMENTS`), fallback `~/Documents/auto-clicker/` |

Rules:
- **Never hardcode the path string.** `Documents` may be redirected (OneDrive on
  Windows, XDG config on Linux) or localized. Resolve through the OS API, fall back to
  `$HOME`. Settings shows the resolved app folder with a single **Open app folder** button.
- **One-time layout migration.** On startup, profile `.json` files found in the app root
  are moved into `profiles/` (settings.json and runs.jsonl stay in the root). If both
  copies exist, `profiles/` wins and the root copy is left untouched, never deleted.
- **The filename is the profile id**: `slug(name).json`, e.g. `idle-tycoon.json`.
  Slugify is deterministic: lowercase → NFKD-fold and strip diacritics → replace every run
  of non-`[a-z0-9]` with `-` → trim leading/trailing `-` → truncate to 64 chars. An empty
  result (blank name, or a name with no ASCII letters/digits — e.g. `"日本語"`) falls back
  to the literal slug `profile`. On collision (create, rename, or duplicate) append
  `-2`, `-3`, … until free; the name itself keeps whatever the user typed and is never
  auto-suffixed in the UI. Duplicating `Idle Tycoon` creates the name `Idle Tycoon copy`
  (then `… copy 2`, …) and its file is the slug of that. Renaming a profile renames the
  file atomically and updates the app setting that points at the selected profile.
- **Unparseable files are surfaced, not hidden.** A `.json` file that fails to parse
  appears in the sidebar under "Problem files" with its error, and is never silently
  deleted or overwritten. The app never rewrites a file it could not read.
- **Atomic writes:** write to a temp file in the same directory, `fsync`, then
  `os.Rename` over the target, so a crash mid-save cannot corrupt a profile.
- **Unknown JSON fields are preserved.** Decoding keeps a raw map of unrecognized keys
  per object and re-merges it on encode (known keys win on collision). A profile saved by
  a future version survives a round-trip through this one.
- **`schemaVersion` gates migrations.** This build writes `1`. Loading a *newer* version
  is refused with a clear message rather than mangled; loading an older one runs a
  migration chain (there is nothing to migrate yet, but the hook exists from day one).
- **The folder is re-read, not watched.** v1 has no continuous filesystem watcher (per-OS
  watcher code is not worth it yet). The app re-lists the folder on startup, on window
  focus, and via an explicit **Refresh** button; `profiles:changed` fires for our own CRUD.
  If a profile that is open with unsaved in-memory edits changed on disk, the app asks
  before discarding the edit rather than silently clobbering either side. A real watcher
  is a Phase 2 candidate.
- `settings.json` lives in the **app folder** (`<Documents>/auto-clicker/settings.json`),
  next to `runs.jsonl` and the `profiles/` subfolder. It is skipped when
  listing profiles, and a profile named "Settings" gets a suffixed id so it can never
  overwrite it. Its schema is [§3.10](#310-app-settings-schema). The single-instance lock
  stays in the OS config dir ([§6](#6-safety)).
- Profiles are plain JSON: back up, diff, or move them between machines by copying the
  folder.

### 3.9 Schema evolution

Two cheap conventions now avoid a rewrite later:

- Every module carries a `kind`: `click` or `key`. If drag or scroll modules are ever
  added, they arrive as new kinds (`"drag"`, `"scroll"`) — a schema addition, not a
  rewrite. Absent `kind` means `"click"`.
  with `kind` **absent meaning `"click"`**. Reserving the discriminator costs one line in
  the docs and keeps old files valid.
- Unknown fields are preserved on round-trip ([§3.8](#38-where-profiles-are-stored)), so
  a future build's extra fields survive a visit to an older one.

### 3.10 App settings schema

`settings.json` holds machine-specific state. It follows the same rules as profiles:
atomic write, unknown-field preservation, and a `schemaVersion` gate
([§3.8](#38-where-profiles-are-stored)). Missing keys take the defaults below; the app
always writes every key so the file is self-describing.

```jsonc
{
  "schemaVersion": 1,
  "selectedProfile": "idle-tycoon",   // profile id (filename stem); "" = none
  "profilesDir": "",                  // "" = resolve the OS default; else absolute override
  "window": { "x": 120, "y": 80, "width": 1100, "height": 720, "maximized": false },
  "hotkeys": { "run": "F8", "pause": "F9" },
  "defaultMaxRuntimeSec": 3600,       // seed for NEW profiles; 0 = unlimited
  "runHistoryLimit": 200,             // runs.jsonl keeps at most this many runs
  "theme": "dark"                     // "dark" | "light" | "system" (light/system = Phase 2)
}
```

Rules:
- **`selectedProfile`** is restored on launch; if the id no longer exists the app selects
  the first profile instead (no error).
- **`profilesDir`** is validated on load: if it cannot be created or is not writable, the
  app falls back to the OS default and surfaces a warning. An override lets users who
  prefer app-data storage move profiles without a symlink.
- **`window`** stores the last *normal* geometry plus the maximized flag, so restoring a
  maximized window still remembers where it was un-maximized.
- **`hotkeys`** are the *desired* bindings (editable in Settings; defaults are v1). They
  use the portal's uppercase-modifier notation ([Appendix B.3](#appendix-b-global-shortcuts-portal-gotchas))
  and are normalized per platform at registration time. Whether registration succeeded is
  runtime-only and is **not** persisted — it is recomputed every launch and shown in
  diagnostics.
- **`defaultMaxRuntimeSec`** only seeds new profiles. The cap that actually applies to a
  run lives in the profile (`failsafe.maxRuntimeSec`, [§6](#6-safety)) so it travels with
  the file; the bottom bar's "This profile" row edits the open profile, not this seed.

### 3.11 First run and profile creation

- **First launch.** The app folder (`Documents/auto-clicker/`) and its `profiles/`
  subfolder are created; `settings.json` and `runs.jsonl` live in the app root. If
  `profiles/` holds no profiles, the app
  creates one named **"New profile"** (file `profile.json`) with `modules: []`,
  `loop: { mode: "once", count: 1, loopDelay: { min: 250, max: 250 } }`,
  `failsafe` seeded from `defaultMaxRuntimeSec`, and the current display snapshot. There
  is **no onboarding modal**; the sidebar simply shows the selected profile and the
  [§5.4](#54-empty-and-blocked-states) empty state invites the first click.
- **`CreateProfile(name)` contract.** An empty/blank name becomes `"New profile"`; the
  slug/collision rules of [§3.8](#38-where-profiles-are-stored) pick the filename; the
  function returns the new profile id. The body is exactly the first-run template above
  (0 modules, `once`), because "start from nothing and add a click" is the safe default.
- A **0-module profile is valid to save and open**; it is only *unrunnable*
  ([§5.4](#54-empty-and-blocked-states)). This keeps "new profile" cheap and never blocks
  the editor.

### 3.12 Groups

Modules can be organised into **groups** — an ordered, toggleable container. This is the
"Adventure vs City" case: toggle a whole group off instead of each click.

```jsonc
"modules": [ /* ungrouped clicks */ ],
"groups": [
  { "id": "g_ab12cd34", "name": "Adventure", "enabled": true,  "repeat": 3, "delayAfter": 500, "modules": [ /* … */ ] },
  { "id": "g_99ff0011", "name": "City",      "enabled": false, "repeat": 1, "delayAfter": 0,   "modules": [ /* … */ ] }
]
```

Rules:
- **Two containers.** Ungrouped modules live in `modules`; grouped modules live in
  `groups[].modules`. A module is in exactly one place.
- **Execution order is ungrouped first, then each group in list order.** Within a container,
  list order applies. This keeps the flat "top → bottom" model for profiles that use no
  groups.
- **Each group is its own loop.** `repeat` (default `1`, range `1–1000`) runs the group's
  block that many times before the next group, inside every pass of the profile loop.
  `delayAfter` (ms, default `0`) waits after the group's repeats before the next group; it
  is skipped for the last group, which the profile's `loopDelay` already covers.
- **Effective enabled = `module.enabled && group.enabled`.** A disabled group skips all of
  its modules but keeps them, their order and their settings; the group can be re-enabled
  later. `EnabledCount`, validation and the execution-order badges all use the effective
  flag.
- **Groups are gated, not flattened away.** The runner still takes a snapshot, but keeps
  the ungrouped block and each enabled group as a distinct block so the group loop and
  delay survive ([§3.1](#31-execution-semantics)).
- **Deleting a group deletes its clicks too** (after a confirmation that names the click
  count). Nothing is ever silently dropped; the confirm is the guard.
- **Backward compatible.** `groups` is absent in older files and defaults to empty, so a
  pre-groups profile is all-ungrouped. `groupId` is *not* stored on the module — membership
  is structural (which array it lives in), which keeps the JSON unambiguous.
- **DnD across containers**: a click can be dragged between groups (or into/out of the
  ungrouped list) and reordered within a container. The engine is unaffected because the
  plan's order is what it consumes.

---

## 4. Runner State Machine

```
        run                       stop / done / error
 idle ────────► running ─────────────────────────────────────► idle
                  │  ▲
           pause  │  │ resume  (F9)
                  ▼  │
                paused
```

| Transition | Trigger | Effect |
|---|---|---|
| `idle → running` | Run button / F8 | validate (**≥ 1 enabled module**) → snapshot → build plan → start scheduler goroutine |
| `running → paused` | Pause button / F9 | gate between clicks; in-flight click completes; all delays and the active-time clock frozen ([§3.2](#32-timing)) |
| `paused → running` | Resume button / F9 | current delay restarts from zero |
| `running → idle` | Stop button / F8 | cancel immediately, release all buttons, log the reason |
| any → `idle` | done / max runtime | natural end of the plan, or the runtime cap reached |
| any → `idle` | window closed / app quit | same as Stop (there is no tray in v1) |

**Closing the window stops the run and releases inputs.** In v1 there is no tray icon,
so the window is the app; minimizing keeps the run alive, closing does not.

### 4.1 Hotkeys (global — they work while the game has focus)

| Key | Action |
|---|---|
| `F8` | Run / Stop toggle (Stop aborts immediately and releases all inputs) |
| `F9` | Pause / Resume |

Defaults are chosen to be unlikely to collide with games. Triggers are editable in
Settings: the value is canonicalized (uppercase modifiers, fixed order) and the source is
rebuilt, so the GlobalShortcuts portal shows a fresh consent dialog. **If no global source
is available the app shows a visible warning at startup** rather than a log line, because
while running unfocused the hotkeys may be the only practical stopper.

### 4.2 Progress events

The engine emits to the UI:

```go
type RunState string // "idle" | "running" | "paused" | "stopping"

type Progress struct {
    State         RunState `json:"state"`
    ProfileName   string   `json:"profileName"`
    ModuleID      string   `json:"moduleId"`
    ModuleLabel   string   `json:"moduleLabel"`   // generated summary for the card
    ModuleIndex   int      `json:"moduleIndex"`   // 1-based, among module executions this pass
    ModuleTotal   int      `json:"moduleTotal"`   // executions per pass (group repeats included)
    ClickIndex    int      `json:"clickIndex"`    // 1-based, within the module
    ClickTotal    int      `json:"clickTotal"`
    LoopIteration int      `json:"loopIteration"` // 1-based pass counter
    Clicks        int64    `json:"clicks"`        // cumulative, monotonic within a run
    ElapsedMs     int64    `json:"elapsedMs"`     // active run time; paused time excluded
    StopReason    string   `json:"stopReason,omitempty"` // "done"|"user"|"maxRuntime"|"error"
    Error         string   `json:"error,omitempty"`
}
```

**Throttling matters.** Emitting one event per click at 50 CPS would flood the webview.
The engine therefore emits:
- **immediately** on any state change, module change, and run end;
- **coalesced at ~10 Hz** otherwise (latest values win — the UI only ever needs "now").

`Clicks` is monotonic within a run, so the UI never sees it go backwards even when events
are dropped.

### 4.3 Threading and cancellation

- One scheduler goroutine owns the run; input calls are serialized through it (no
  concurrent injection, ever).
- Cancellation via `context.Context`. `Stop` sets the stop flag *and* cancels the context,
  which interrupts the interruptible sleeps immediately.
- `ReleaseAll()` runs on every exit path via `defer`, so no code path can leave a button
  held.

### 4.4 Error handling

| Failure | Behavior |
|---|---|
| Input injection fails (portal revoked, permission lost, backend error) | **abort the run**, release all, show an error banner with the backend message and the diagnostics panel shortcut |
| `CursorPos()` unavailable | disable the picker / failsafe controls with an explanatory tooltip; runs still work |
| Hotkey registration fails | warn at startup, keep running with in-window shortcuts |
| Profile file unreadable | list it under "Problem files"; never overwrite it |
| Save fails (disk full, permission) | keep the edit in memory, show a persistent "unsaved changes" indicator, retry on next change |
| Coordinate off-screen | non-blocking warning chip on the card |

### 4.5 Startup and shutdown

Order matters; this is the sequence `main.go` follows.

On Linux, before this sequence the app installs/refreshes its launcher entry
([§3.6](#36-platform-scope)) as a best-effort step. It starts no service and is never
fatal, so a failure only logs.

**Startup**
1. **Acquire the single-instance lock** ([§6](#6-safety)) before any service starts. If it
   is already held, this process starts **no engine, no driver and no hotkeys**; it shows
   a minimal window with "Auto Clicker is already running" and a Close button, then exits.
   (A native OS dialog would be platform-specific; reusing the Wails shell is uniform.)
2. Resolve the app folder and run the one-time layout migration (profile JSONs from the
   root into `profiles/`), then load `settings.json` ([§3.10](#310-app-settings-schema));
   write defaults on first run.
3. List profiles from `profiles/`, restore `selectedProfile` (falling back to the first),
   and create the first-run template if the folder is empty
   ([§3.11](#311-first-run-and-profile-creation)).
4. Show the main window with the restored geometry. The window position, size and
   maximised state come from `settings.window`, falling back to **1100×720 centered** on
   first run. They are written back on move/resize/maximise (debounced) and once more on
   shutdown ([§3.10](#310-app-settings-schema)).
5. **Initialize the input backend and probe capabilities on a background goroutine.** The
   UI renders immediately with Run disabled and a "preparing input…" note; it enables Run
   when the driver is ready. On Wayland the order is the one in
   [§2.2](#22-backend-selection): `x11` first, `libei` offered/upgraded after. While on
   the `x11` fallback on a Wayland session, a persistent note warns that **native-Wayland
   windows may not receive clicks** until native input is enabled; Run stays available
   because XWayland/X11 games (the Steam/Proton case) work. The upgrade is **automatic**
   when `$XDG_STATE_HOME/robotgo/portal_token` exists and is non-empty, and user-initiated
   otherwise ([§2.2](#22-backend-selection)). The `CursorPos()` probe runs here and feeds
   [§4.4](#44-error-handling) capability gating.
6. **Register global hotkeys asynchronously.** Portal consent is an explicit action, not a
   launch-time dialog; failure produces the visible startup warning in
   [§4.1](#41-hotkeys-global--they-work-while-the-game-has-focus).

**Shutdown** (window closed / app quit / OS logout), in this order:
1. Persist the window geometry (position, size, maximised).
2. If a run is active, cancel it with **Stop** semantics and `ReleaseAll()` — inputs are
   released *before* the driver is closed ([§6](#6-safety)).
3. Flush any pending debounced autosave ([§5.3](#53-module-card)).
4. Unregister hotkeys, close the driver, release the lock.

A crash cannot skip step 1 in a way that strands a held button, because the engine holds
buttons only within a single click and `ReleaseAll` is deferred on every exit path; the OS
also releases injected button state when the process dies.

---

## 5. UI Specification

### 5.1 Visual language

Dark-first, dense, built on the same component vocabulary a Tailwind app would produce —
this is the "same visual" requirement, written down so the spec stands alone:

| Token | Value |
|---|---|
| App background | `neutral-950` |
| Panels / sidebar | `neutral-900` |
| Cards | `neutral-800`, `rounded-lg`, `border border-neutral-700`, `p-3` |
| Card (disabled module) | 60 % opacity, `grayscale`, draggable but visually inert |
| Text | `neutral-100` primary, `neutral-400` secondary, `neutral-500` hints |
| Mono | `ui-monospace` for coordinates, counts and shortcuts |
| Accent — Run | `emerald-500` |
| Accent — Pause/warning | `amber-400` |
| Accent — Stop | `rose-600` |
| Focus | `ring-2 ring-indigo-500 focus-visible` on every interactive element |
| Type scale | 13 px base, 12 px secondary, 11 px badges |
| Spacing | 8 px base; card gap `2`; section padding `3` |

Interaction rules:
- **No native `<select>`**: WebKitGTK renders it light regardless of CSS, so selects and
  menus use the app's own `Select` / `Dropdown` components. `color-scheme: dark` is set for
  any remaining native control.
- Every icon-only button has a tooltip **and** an `aria-label`.
- Controls that are unavailable (not merely disabled-while-running) are visibly
  unavailable and explain why: reduced opacity + tooltip stating the reason
  (e.g. "reading the cursor needs the Wayland query path, unavailable with libei").
- Motion is subtle; drag animations respect `prefers-reduced-motion`.

### 5.2 Layout

```
┌────────────┬──────────────────────────────────────────────────────────┐
│ PROFILES   │  ▸ Click pipeline  (enabled modules run top → bottom)    │
│            │  ┌──────────────────────────────────────────────────────┐│
│ ▸ Tycoon   │  │ ☑ 1  (940, 520) left ×3 hold 10ms delay 90ms      ≡ ⋮ ││
│ ▸ Farm     │  │ ☑ 2  ⌖ at cursor left ×1 hold 10ms delay 400ms     ≡ ⋮ ││
│ + New      │  │ ☐ –  (100, 100) right ×1 hold 10ms delay 250ms     ≡ ⋮ ││
│            │  └──────────────────────────────────────────────────────┘│
│ ⚙ Settings │  [+ Add click]                                          │
├────────────┴──────────────────────────────────────────────────────────┤
│ Loop: ▼ forever │ ● RUNNING · Tycoon 1/2 · loop 14 · 8.2s · clicks 84│
│                  │ [⏸ F9] [⏹ F8]                                     │
└───────────────────────────────────────────────────────────────────────┘
```

- **Left sidebar** — profiles: select, inline rename, duplicate, delete, plus **History**
  (opens the run-history modal: `runs.jsonl`, newest first, with Clear and the retention
  count) and Settings.
  Profile rows show module count and enabled count (`3 modules · 2 on`).
- **Center** — the click pipeline: **ungrouped clicks**, then **groups** (each a titled,
  collapsible-looking section with an enable switch, its own click list, a **repeat** input
  (×N) and a **delay** input (ms) for the gap to the next group). Groups are one
  level deep — not nested action rows, which keeps the flat-list simplicity
  ([§3.12](#312-groups)). `+ Add group` creates one; `+ Add click` / `+ Add key` inside a
  group adds to
  that group. Drag a click between groups (or the ungrouped list) and reorder within a
  container, and **groups are reordered by dragging the group handle (≡)** — no arrow
  buttons. Each section can be **collapsed** (▸/▾) to hide its list — UI-only state, not
  persisted, and independent of the group's enable switch. **Clicking the group header
  toggles it**, mirroring how clicking a module card expands it (interactive controls
  excluded); **double-clicking the group name renames it**. The name text is a rename-only
  zone sized to its content (no delay); the header's free space toggles.
- **Bottom bar** — two rows. The **top row** is **this profile**: `max runtime` (minutes,
  `0` = unlimited) and the **Humanize delays** toggle ([§3.2](#32-timing)); disabled when
  no profile is open. The **bottom row** is the pipeline: loop-mode selector plus its
  parameters (enabled only while idle) — `passes` for `count`, and a `loop delay min–max`
  pair (ms) for `count`/`forever` — live telemetry, and Run/Stop/Pause mirroring the
  hotkeys (labeled with their key).

Drag-and-drop uses `@dnd-kit` with a visible drop indicator line, autoscroll near the
edges, and **keyboard-accessible reordering** (lift with Space, move with arrows, drop
with Space) plus explicit *Move up* / *Move down* items in the card menu for users who
prefer the mouse or cannot drag.

### 5.3 Module card

```
┌──────────────────────────────────────────────────────────────────────┐
│ ☑  1   (940, 520)  left ×3  hold 10ms  delay 90ms             ≡   ⋮  │
│ ☑  2   ⌨ ctrl+c   ×1  hold 10ms  delay 50ms                    ≡   ⋮  │
└──────────────────────────────────────────────────────────────────────┘
```

| Element | Behavior |
|---|---|
| Enable switch | toggles `enabled`; card dims when off |
| Order badge | **execution order** (`1..n`) among enabled modules; `–` when disabled, so the real sequence is readable at a glance |
| Target / key chip | `(940, 520)`, `⌖ at cursor`, `⚠ off-screen`, or for key modules `⌨ ctrl+c` |
| Summary | `left ×3 hold 10ms delay 90ms` or `ctrl+c ×1 hold 10ms delay 50ms` — generated from fields, never stale |
| Drag handle `≡` | hidden while running |
| Menu `⋮` | Duplicate · Move up · Move down · Enable/Disable · Delete |

Expanding a card (or its menu → Expand) reveals the edit row:

```
click: label [            ]  target ( ▪ absolute  ○ cursor )  pick ⌖  x [940] y [520]
       button [ left ▼ ]   count [ 3 ]   delay [ 90 ] ms   hold [ 10 ] ms

key:   label [            ]  keys [ ctrl+c ]  OK / Cancel  (focus, press the combo, confirm)
       count [ 1 ]   delay [ 50 ] ms   hold [ 10 ] ms
       [ ] Hold until Stop (last enabled step; needs loop once)
```

A **key module** is added with `+ Add key` (section footer or group menu) and opens
expanded with an empty combo, so the run stays blocked until one is recorded.

- **Clicking a card expands it** for editing (click again to collapse); the card menu no
  longer carries an "Expand/edit" item. **A newly added click opens expanded**, and adding
  one to a collapsed group expands that group. Menus (module and profile) close when an item is
  chosen, on an outside click, and on `Esc`.
- **Label** (the module's `name`, [§3.3](#33-module-fields-the-complete-vocabulary)) is
  optional free text, ≤ 80 chars. When set it replaces the generated summary wherever the
  module is named — card title, undo toast, and the telemetry bar's `ModuleLabel`; clearing
  it restores the generated summary. It is never parsed and never affects execution.
  **Edited by double-clicking the card title** — the title text is a rename-only zone sized
  to its content (clicking it does not toggle), while the card's free space toggles; no
  delay, and no label field in the expanded row.
- **Delete requires a confirm dialog** (click or group), because a group also destroys the
  clicks inside it. Deleting a click still shows the undo toast (5 s) after the confirm.
  **Profile delete** requires a confirm too, since it destroys a file on disk.
- **Autosave**, debounced ~500 ms, with a subtle "Saved" indicator; also flushed on card
  blur and on app close. There is no explicit Save button.

### 5.4 Empty and blocked states

Three distinct states, deliberately worded differently:

| State | Presentation |
|---|---|
| Profile has **0 modules** | Empty-state panel in the list area: "No clicks yet." + primary `+ Add click` button + one-line explainer ("Each click is one step; enabled steps run top to bottom.") |
| Profile has modules but **0 enabled** | Muted `amber` row: `⚠ No modules enabled — enable at least one to run`. **Run is disabled**, with that sentence as its tooltip. The hotkey no-ops. |
| Profile is **invalid** (field out of range, missing coordinates) | Same amber row pattern, listing each issue next to the offending card; Run disabled |

The warning is always visible *before* the user presses Run — never a surprise toast
after the fact. That is why the hotkey can safely be a silent no-op.

### 5.5 Settings & diagnostics

| Group | Contents |
|---|---|
| Hotkeys | Editable **Run / Stop** and **Pause / Resume** triggers (canonicalized on Apply; the portal re-prompts consent), the active source (`Portal (KDE)`, `X11 grab`, `Windows RegisterHotKey`, or ⚠ unavailable), plus the **"Enable native Wayland input"** action when the app is on the `x11` fallback and `libei` is available ([§2.2](#22-backend-selection)). |
| App folder | Resolved app folder, **Open app folder** (profiles live in `profiles/` inside it) |
| Advanced | `defaultMaxRuntimeSec` (seed for new profiles) |
| Updates | Current version, a **Check for updates** button (manual trigger) and the Update action when a newer release is found ([§5.9](#59-self-update)) |
| Diagnostics | App version, robotgo pin, application id, selected backend (+ fallback reason), `CursorPos` availability, virtual desktop geometry, hotkey registration result, app folder / profiles / log paths, and a `Copy diagnostics` button for bug reports |

The diagnostics panel is not a nicety: this app's failure modes are environment-specific
(portal consent, session type, scaling), and the first support question will always be
"which backend is it using?".

**Logging.** The app writes a structured log to the OS state dir —
`$XDG_STATE_HOME/auto-clicker/app.log` (fallback `~/.local/state/auto-clicker/`) on Linux,
`%LOCALAPPDATA%\auto-clicker\logs\app.log` on Windows — rotated at 1 MB with 3 files kept.
Startup problems (lock held, profiles dir unwritable, backend init failure, hotkey
failure) are both logged and surfaced in the UI; an error that prevents the window from
opening at all is also written to stderr. **The Windows build is a GUI binary
(`-H windowsgui`) with no console**, so the file log is the only place to see runtime
output there; Settings → Diagnostics shows the resolved path, and the **Open app folder**
button opens the folder that contains it. `Copy diagnostics` includes the log tail. This
is the *diagnostic* log, distinct from the append-only per-run history in
[§6](#6-safety).

### 5.6 Validation

Run is blocked, with the reason shown inline next to the offending control:

| Rule | Kind |
|---|---|
| At least one module is **effectively** enabled (module **and** its group) | blocking |
| A `kind: key` module has a valid non-empty combo (`ctrl+shift+c`, `f5`, `space`) | blocking |
| A hold module (`holdUntilStop`) has `count = 1`, is the **last** enabled action, and `loop.mode = once`; only one hold is allowed | blocking |
| Every enabled `absolute` module has `x` and `y` | blocking |
| Move-only (`button: none`) with a `cursor` target | warning |
| Group ids are unique; each group has an id | blocking |
| A group has a non-empty name | warning |
| Group `repeat` ∈ 1–1000, group `delayAfter` ∈ 0–3600000 | blocking |
| `count` ∈ 1–1000, `holdMs` ∈ 0–1000, `delayAfter` ∈ 0–3600000 | blocking |
| `loop.mode = count` ⇒ `count ≥ 1` | blocking |
| `loopDelay.min ≤ loopDelay.max`, both ≥ 0 | blocking |
| Coordinates inside the canonical desktop bounds ([§3.5](#35-coordinate-space-and-display-changes)) | **warning only** |
| Profile's `display` snapshot matches the current desktop | **warning only** |

Validation lives in Go (`internal/model`) and is exposed to the UI through a binding, so
the UI and the runner can never disagree — the UI renders the same issues the runner
would refuse on.

### 5.7 Picking coordinates

A **"pick position"** mode. On success it writes `x`/`y` into the module.

**As implemented (v1): in-app countdown, cursor captured by the engine side.** There is **no
second window**. The UI calls `StartPick()`, which starts a **Go-side timer**
(`PickCountdown`, 3 s) and returns. A banner inside the main window shows the countdown
(with tenths) and a Capture/Cancel button. The user moves the pointer over the target; when
the timer fires, the service reads the **real** cursor (`CursorPos()`, the x11 query path,
[§2.2](#22-backend-selection)) and delivers it via the `pick:done` event (`{x, y, ok}`).
`Enter` captures immediately and `Esc` cancels while the app has focus.

Why no window:
- On Wayland a window cannot be positioned, and a second window's focus/hide behaviour was
  unreliable; a fullscreen transparent click-overlay was tried and reverted (the shell
  appeared maximized/on top and the page handlers did not fire on KDE/GTK3).
- The countdown is **authoritative in Go**, so it still captures when the app is unfocused
  or behind a fullscreen game (a JS countdown alone would be throttled).
- Because the capture is cursor-based, it needs no coordinate translation and works on any
  monitor. Trade-off: clicking the screen is *not* the capture gesture — the target is
  captured where the pointer is when the timer fires (or on `Enter`).

Implementation notes that matter:
- **Non-blocking flow.** `StartPick()` returns immediately; the outcome arrives as
  `pick:done`. A blocking `PickPosition()` call would deadlock the picker's own
  `ConfirmPick`/`CancelPick` binding calls.
- **Idempotent finish**, and `finishPick` runs its callback off the calling goroutine.

Also in v1:
- **Manual entry** of `x`/`y` remains first-class and is what the picker writes into.

> **Removed:** the "use current cursor" button. Reading the cursor at the moment the button
> is clicked always returns the position *over the app itself*, never the target, so it was
> misleading; the picker covers the real need.

### 5.8 In-app keyboard shortcuts

| Key | Action |
|---|---|
| `F8` | Run / Stop (also global) |
| `F9` | Pause / Resume (also global) |
| `Esc` | close the picker, popovers, dialogs |
| `Space` | toggle the focused module's enable switch; lift/drop during DnD |
| `Delete` | delete the selected click or group (with a confirmation; clicks also get an undo toast) |
| `↑` / `↓` with `Alt` | move the focused module up/down |

**Shortcut scoping.** F8/F9 always act, from anywhere. The *editing*
shortcuts — `Space`, `Delete`, and `Alt+↑/↓` — are **suppressed while focus is in a text
field, number input, textarea, select or `contenteditable`**, and while a dialog, popover
or the picker overlay owns focus. Without this, typing a label or a coordinate would
toggle or delete modules. `Esc` closes the topmost overlay/dialog and does nothing when
none is open. During a run the UI is locked ([§3.1](#31-execution-semantics)), so only
F8/F9 are live.

### 5.9 Self-update

On startup the app checks the **latest GitHub Release** of `guilhermehccruz/auto-clicker`
through the Wails v3 updater (`pkg/updater` + the `github` provider). It runs silently:
nothing opens unless a newer version exists, in which case the header shows a
`⬆ vX.Y.Z available` banner with an **Update** button.

- **Version source.** The binary embeds the release tag (`-X main.version=…`); the leading
  `v` is stripped before comparison. Non-semver builds (`dev`) skip the updater entirely.
- **Install.** Update downloads the platform asset (`.tar.gz` on Linux, `.zip` on
  Windows), verifies its **SHA-256** against the release's `checksums.txt`, swaps the
  binary via the updater's helper, and relaunches. Requires the binary to live in a
  user-writable location (the portable download is); an installed-to-`/usr/local/bin`
  build would need elevation.
- **Release assets.** Each archive contains **exactly one top-level entry** (the binary) —
  the updater rejects multi-entry archives. Run instructions live in the release notes.
  CI publishes a `checksums.txt` sidecar alongside the archives.
- **Trust.** Verification is checksum-only today: it detects corruption and a
  partially-replaced asset, but the digest comes from the same release, so it is **not** an
  authenticity guarantee. A pinned signing key (`updater.Config.PublicKey`) is the planned
  hardening step.
- **Linux filesystem caveat.** The Unix swap helper replaces the binary with `os.Rename`,
  which fails with `EXDEV` across filesystems — and `$TMPDIR` (`/tmp`, often tmpfs) is
  usually a different mount than `$HOME`. For the duration of an update the app therefore
  points `TMPDIR` at the binary's own directory, so staging and target share a device. The
  Windows helper already falls back to a copy when the staging file is on another drive.
- **Channel.** Every push to `main` publishes a normal (non-prerelease) release, so the
  app updates to the newest build. Tagging `v*` still publishes a semantic version.

---

## 6. Safety

| Mechanism | Behavior | Phase |
|---|---|---|
| Stop (`F8`) | abort immediately (even mid-click), release all buttons | v1 |
| Max runtime | profile-level `maxRuntimeSec` (default 3600, `0` = unlimited), measured on **active** run time so pausing never counts against it ([§3.2](#32-timing)); auto-stops with a notice | v1 |
| **Single-instance lock** | a second launch refuses to run and shows "Auto Clicker is already running" | v1 |
| **UI locked while running** | destructive controls are inert, so even a `cursor` module clicking over our own window cannot delete or retarget anything | v1 |
| Held-input audit | `ReleaseAll()` on every exit path (`defer`), including window close and app quit | v1 |
| Hotkey failure warning | visible warning at startup when no global source registered | v1 |
| Run log | append-only `runs.jsonl` history (start/stop/reason/clicks), capped | v1 |

**Single-instance lock** is not optional for an input injector: two instances would
fight over the same pointer, double-click at different coordinates, and each would have
its own global hotkeys. Implementation: an exclusive lock file in the config dir
(`auto-clicker.lock`, `flock` on Linux) / a named mutex on Windows. The lock is held for
the whole process lifetime and released by the OS if the process dies, so a stale lock
cannot wedge the app. There is no tray to restore, so the second instance starts no
services, shows "Auto Clicker is already running" and exits
([§4.5](#45-startup-and-shutdown)).

**Why the locked UI is a safety feature:** with `cursor` targets the user can
legitimately click anywhere, including over this app's own window. Because every
mutating control is disabled during a run, a stray click on our UI can at worst press
Stop or Pause — which is what the user would want anyway.

**README notes** (surface these, they are support-load-bearing):
- Some online games' anti-cheat prohibits input automation. Single-player clicker/idle
  titles are generally unaffected.
- Exclusive-fullscreen games may block injection or hide the pointer; borderless
  windowed is the workaround.
- First run on Wayland shows a screen-share/remote-desktop consent dialog from the
  compositor, and the permission is remembered afterwards.
- Coordinates are recorded in physical pixels for the desktop geometry shown in
  diagnostics; changing monitor layout may require re-picking.

---

## 7. Code Structure

```
go.mod                       module auto-clicker
main.go                      Wails bootstrap, startup/shutdown sequence (§4.5),
                             single-instance lock, hotkey registration, service bindings
Taskfile.yml                 Wails task runner (build/dev/package) [scaffold]
build/                       Wails build config: config.yml, appicon, per-OS Taskfiles
internal/model/              Profile, Module, Target, Loop, Failsafe; validation;
                             JSON with unknown-field preservation; schema migrations
internal/engine/             Runner: snapshot → plan (group blocks) → schedule →
                             execute; state machine, pause/stop, progress events
internal/input/              Driver interface; backend_linux.go / backend_windows.go;
                             fake.go for tests
internal/cursor/             CursorPos query path per platform + capability probe
internal/store/              profile/settings paths, atomic writes, dir listing,
                             settings schema, single-instance lock
internal/logging/            structured log + rotation in the OS state dir;
                             diagnostics tail
internal/hotkey/             HotkeySource: portal_linux.go / x11_linux.go / win_windows.go
internal/desktop/            Linux launcher entry (.desktop + hicolor icon) written on
                             startup; no-op elsewhere
internal/service/            operation surface the UI binds to (§7.2); no Wails import
frontend/                    React + TS app (see below); built to frontend/dist and
                             embedded by main.go
frontend/bindings/           Wails-generated TS bindings (do not edit)
.github/workflows/           ci.yml, release.yml
```

`frontend/src` layout:

```
main.tsx  App.tsx
api/          Api interface + WailsApi adapter (bindings.ts, wails.ts) + MockApi
              for `vite dev`, and shared TS types mirroring internal/model
components/   Sidebar, ProfileRow, ModuleList, ModuleCard, TargetEditor, Dropdown,
              Select, BottomBar, Telemetry, RunControls, EmptyState, PickBanner,
              WarningBanner, SettingsDialog, DiagnosticsPanel, UndoToast
state/        useProfiles.ts, useRunner.ts, useSettings.ts
lib/          format.ts (card summaries), validate.ts (mirrors Go issues)
```

`createApi()` returns `WailsApi` when `window._wails` is present (running in the
Wails webview) and `MockApi` otherwise, so the UI runs standalone under `vite dev`.

### 7.1 The Driver interface

This is the seam that keeps the engine testable (a `FakeDriver` records intended calls)
and the one place where the "no motion" contract is enforced:

```go
package input

type Button int // Left, Right, Middle

type Driver interface {
    // ClickAt moves the pointer to (x,y) and clicks — target kind "absolute".
    ClickAt(x, y int, button Button, hold time.Duration) error

    // ClickHere presses and releases |button| at the pointer's CURRENT position
    // without issuing any motion event — target kind "cursor".
    ClickHere(button Button, hold time.Duration) error

    // Move moves the pointer without clicking. Reserved: no v1 module uses it.
    Move(x, y int) error

    ButtonDown(button Button) error
    ButtonUp(button Button) error

    // CursorPos reports the real pointer position. ok == false when the active
    // backend cannot read it (libei). Callers must degrade, never guess.
    CursorPos() (x, y int, ok bool)

    // ScreenSize returns the virtual desktop size in physical pixels. Equal to the
    // bounding box of Displays(); kept for the common case and for validation.
    ScreenSize() (w, h int, err error)

    // Displays enumerates monitors (origin + size in physical pixels, scale factor,
    // primary flag). Used by the coordinate picker to place one overlay per monitor and
    // to translate a captured local point to virtual-desktop coordinates ([§5.7]).
    // May return a single synthetic display when the backend cannot enumerate; callers
    // must not assume more than one entry.
    Displays() ([]Display, error)

    // ReleaseAll releases every button/key we may hold. Called on every exit path.
    ReleaseAll() error

    Close() error
}

type Display struct {
    X, Y   int     // top-left origin in virtual-desktop physical pixels
    W, H   int     // size in physical pixels
    Scale  float64 // compositor scale factor (1.0 when unknown)
    Primary bool
}

// New picks the backend. Windows → win. Linux: an X11 session → x11. A Wayland session
// starts on x11 and may upgrade to libei once consent is resolved ([§2.2], [§4.5]); the
// driver is therefore initialized and re-initialized during startup, not just once.
func New() (Driver, error)

// Capabilities reports what the active backend can do, for the UI.
type Capabilities struct {
    Backend              string // "libei" | "x11" | "win"
    FallbackReason       string
    CanReadCursor        bool
    CanPickPoint         bool
    CanEnumerateDisplays bool
    CanMoveAbsolute      bool // false on libei without a linked ScreenCast stream
}
```

`ClickHere` is a distinct method (rather than `ClickAt(*Point)`) so the "no motion"
guarantee is structural, not a convention — and `FakeDriver.Clicks` asserts that no
`Move` precedes a `ClickHere` in unit tests.

**Confirmed at source level (Phase 0.5).** All three sub-packages expose per-button
press/release primitives — `MouseDown(button)`, `MouseUp(button)` and `Toggle` — so
`holdMs` is implemented as `MouseDown → sleep(holdMs) → MouseUp`. This is deliberate: the
convenience `Click()` hardcodes a 10 ms hold on both `x11` and `libei`, which would make
`holdMs` advisory, so the engine must never call it. `holdMs = 0` therefore means the
backend's minimum down/up gap, not zero wall time. The `CursorPos()` caveat stands:
`libei.Location()` returns the last *injected* position ([§2.2](#22-backend-selection)),
so `CursorPos()` must be sourced from the `x11` query path when XWayland is present
(confirmed working — it read the real cursor in the spike) and report `ok == false`
otherwise.

### 7.2 UI ↔ Go bindings

```go
type Service struct{ /* engine, store, hotkeys, driver */ }

// Profiles
func (s *Service) ListProfiles() ([]ProfileSummary, error)   // incl. problem files
func (s *Service) LoadProfile(id string) (*model.Profile, error)
func (s *Service) SaveProfile(p *model.Profile) error
func (s *Service) CreateProfile(name string) (string, error)
func (s *Service) RenameProfile(id, newName string) (string, error)
func (s *Service) DuplicateProfile(id string) (string, error)
func (s *Service) DeleteProfile(id string) error
func (s *Service) ReloadProfiles() ([]ProfileSummary, error)  // Refresh / window focus
func (s *Service) UpdateDisplaySnapshot(profileID string) error // banner action (§3.5)

// Validation / capabilities
func (s *Service) Validate(p *model.Profile) []model.Issue
func (s *Service) Capabilities() input.Capabilities
func (s *Service) Diagnostics() Diagnostics

// Runner
func (s *Service) Run(profileID string) error
func (s *Service) Stop() error
func (s *Service) Pause() error
func (s *Service) Resume() error
func (s *Service) Status() Progress

// Picking
func (s *Service) CursorPos() (x, y int, ok bool)
func (s *Service) StartPick() error       // shows the picker; result via `pick:done`
func (s *Service) ConfirmPick() error     // called by the picker toast
func (s *Service) CancelPick() error      // called by the picker toast on Esc
func (s *Service) Displays() ([]input.Display, error)           // picker overlay placement

// Settings / paths
func (s *Service) GetSettings() (Settings, error)
func (s *Service) SaveSettings(st Settings) error
func (s *Service) ProfilesDir() string
func (s *Service) OpenProfilesDir() error
func (s *Service) OpenLogDir() error
func (s *Service) EnableWaylandInput() error  // explicit libei upgrade (§2.2, §4.5)
func (s *Service) Version() string
```

Events pushed to the UI: `run:progress` ([§4.2](#42-progress-events)),
`run:state`, `profiles:changed`, `capabilities:changed`, and `pick:done` ([§5.7](#57-picking-coordinates)).

---

## 8. Phases

### Phase 0 — stack spike ✅ (already done)
Toolchain and robotgo pure-Go builds for Linux and Windows; precise interruptible click
loop (sub-millisecond p95 lateness, ~9 ms stop latency at 50 CPS); `x11` + `libei` +
`win` backends; all three global hotkeys registering and firing through the
`GlobalShortcuts` portal. Full results in
[Appendix C](#appendix-c-stack-validation-results).

### Phase 0.5 — blocking spike for this app **[unproven]**
Seven questions, all cheap to test, any of which could change the design. A runnable
harness lives at **`spike/phase05/`** (read-only by default; injection and portal access
require `-yes`). Source inspection already answers several of these; the ones that need a
human at the consent dialog are marked **pending**. Results are recorded in
[Appendix C](#appendix-c-stack-validation-results).

1. **`ClickHere` on all three backends.** Does a button injection with *no* preceding
   motion land at the real pointer? Especially libei: confirm the portal's remote-desktop
   button event carries no implicit position (e.g. moving to origin). **If this fails,
   `cursor` targets break on Wayland** and the feature needs a cursor read first, which
   drags the whole `CursorPos` problem into the critical path.
   *Source: ✅ both `x11` (`XTEST FakeInput` ButtonPress/Release) and `libei`
   (`NotifyPointerButton`) send a button event with no motion; `libei.MouseDown` does not
   touch `Move`. Runtime confirmation still pending a human run.*
2. **Press/release primitives** with per-button control in the pure-Go sub-packages
   (needed for `holdMs`, [§7.1](#71-the-driver-interface)).
   *✅ answered — `MouseDown`/`MouseUp`/`Toggle` exist in `x11`, `libei` and `win`; the
   engine must avoid `Click()`, which hardcodes a 10 ms hold.*
3. **`CursorPos()` on Wayland.** X11 `XQueryPointer` when an XWayland client exists; an
   overlay-derived path otherwise. Determines whether the picker
   and the picker work on the primary target platform.
   *Partial: ✅ the `x11` query path reads the real cursor on this Wayland session
   (returned the true pointer position); `libei.Location()` returns injected-only, as
   documented. The overlay-derived path (no XWayland client) is still open.*
4. **Mixed-DPI / multi-monitor coordinates.** Pick a point on a scaled monitor, then
   inject it and verify the same physical pixel lights up ([§3.5](#35-coordinate-space-and-display-changes)).
   *Pending a human run.*
5. **Fullscreen overlay placement on Wayland** for the picker ([§5.7](#57-picking-coordinates)).
   *Pending — needs a Wails window, out of scope for this CLI harness.*
6. **`ScreenSize` / `Displays` on libei.** The spike measured geometry only on `x11`
   ([Appendix C](#appendix-c-stack-validation-results)). Confirm the virtual desktop size
   is readable on the preferred Wayland backend, and whether per-monitor origins are
   available. If not, validation, the display warning and per-monitor picking all depend
   on the `x11` fallback, and `Capabilities.CanEnumerateDisplays` must gate them.
   *✅ answered with a design consequence: `x11` enumerates monitors via Xinerama
   (2 displays, correct origins, runtime-verified). `libei` geometry is **only** available
   when a ScreenCast stream is linked — otherwise all helpers return zero. So geometry is
   sourced from the `x11` query path even when injecting through `libei`, and
   `absolute` targets require the ScreenCast stream ([§2.2](#22-backend-selection)).*
7. **Consent probe without a dialog.** Can libei initialization report "cached token
   present" versus "would prompt" *without* showing the portal dialog? This decides whether
   the Wayland upgrade in [§4.5](#45-startup-and-shutdown) can ever be automatic, or must
   always be user-initiated to avoid a launch-time surprise.
   *✅ answered — the token is a plain file at `$XDG_STATE_HOME/robotgo/portal_token`;
   the app checks it before init. No probe dialog needed.*

### Phase 1 — MVP
Profiles CRUD (create/rename/duplicate/delete, autosave, atomic writes); flat module list
with toggle, drag-reorder (+ keyboard/ Move up/down), add, duplicate, delete with undo;
inline editing; coordinate picker; plan runner; three loop
modes; Run/Pause/Stop; validation with the three empty/blocked states; locked UI;
telemetry bar; settings + diagnostics; single-instance lock; global hotkeys.

**Status (in progress).** The Go core is implemented and tested (`go test ./internal/...`):

| Area | State |
|---|---|
| `internal/model` — types, JSON + unknown-field preservation, defaults, validation, slug | ✅ done, tested |
| `internal/store` — paths, atomic writes, CRUD, settings, single-instance lock | ✅ done, tested (Linux + Windows build) |
| `internal/engine` — plan (group repeats + delays), deadline accumulation, loop modes, pause/stop, progress | ✅ done, tested (fake clock + fake driver) |
| `internal/input` — `Driver`, `FakeDriver`, x11/libei and win backends | ✅ done; Linux + Windows cross-build |
| `internal/cursor` — capability probe | ✅ done, tested |
| `internal/hotkey` — trigger normalization | ✅ done, tested |
| `internal/service` — the §7.2 operation surface | ✅ done, tested |
| `internal/hotkey` — portal / `XGrabKey` / `RegisterHotKey` sources | ✅ done, builds on Linux + Windows (runtime consent/grabs untested here) |
| `main.go` + Wails scaffold — bootstrap, bindings, events, Taskfile | ✅ `wails3 build` produces a working Linux binary |
| `frontend/` — React/TS app (sidebar, DnD list, cards, telemetry, picker, settings, diagnostics) | ✅ typechecks, `vite build` and 10 vitest cases pass; `WailsApi` adapter wired to the generated bindings |
| **Groups** ([§3.12](#312-groups)) — model/engine gating + UI multi-container DnD | ✅ done, tested (Go + UI) |
| `internal/logging` — file log + 1 MiB×3 rotation | ✅ done |
| CI workflows (`.github/workflows/ci.yml`, `release.yml`) | ✅ added |

Linux uses Wails' **GTK3/WebKit2GTK-4.1** backend (`-tags gtk3`), not the default
GTK4/WebKitGTK-6.0, so the runtime deps match the platform scope in
[§3.6](#36-platform-scope).

### Phase 2 — polish

The remaining work is the backlog below. The product shape is stable: one dark theme,
portable archives, mouse and keyboard operation.

### Backlog (living list)

| Item | State | Notes |
|---|---|---|
| "Check for updates" in Settings | ✅ done | manual trigger alongside the silent startup check |
| Update channel | **decided** | every push to `main` publishes a new version; no prerelease/stable split |
| Tray icon / minimize-to-tray | **dropped** | closing the window keeps stopping the run |
| Hotkey rebinding | ✅ done | edit the F8/F9 triggers in Settings (portal notation) |
| Run history | ✅ done | append-only `runs.jsonl`, capped by a Settings count (`runHistoryLimit`) |
| Delay humanization | ✅ done | profile `humanize` scales every fixed delay by ±25% |
| Docs / lint | ✅ done | DESIGN §10 packaging row fixed; ESLint wired into CI |
| Keyboard actions (key modules) | ✅ done | `kind: key` presses a combo (target/button unused), same count/hold/delay; capture UI, validation and x11/libei/win injection |
| App keyboard navigation | open | select and operate modules/groups without the mouse; needs a focus/selection model and a shortcut map |
| Hold action | ✅ done | `holdUntilStop`: mouse or key stays down until Stop; validated as the last enabled action with loop `once` |
| Single app folder | ✅ done | `app.log`, `settings.json`, `runs.jsonl` in the app root; profiles in `profiles/`; startup migration; one Open app folder button |
| History modal | ✅ done | opened from the sidebar; lists `runs.jsonl` and owns Clear + the retention count |
| Profile controls on the main screen | ✅ done | max runtime + humanize sit in the bottom bar's second row |
| Light theme | **dropped** | the dark theme is the only theme |

Removed from the backlog (decided / validated): mixed-DPI, fullscreen overlay,
`ClickHere` at runtime, `XGrabKey`, portal consent, the manual release matrix, Windows
runtime, import/export, packaged installers, update signing.

**Process:** pushes are manual. The agent never runs `git push`; it commits locally and
the user pushes.

### Deferred / maybe never
`relativeToCursor` and `windowRelative` targets; scroll and drag (down → move → up)
module kinds; input recorder; vision triggers. Each is a schema addition, not a rewrite —
see [§3.9](#39-schema-evolution).

---

## 9. Testing

| Layer | Approach |
|---|---|
| `internal/model` | table-driven validation tests (every blocking and warning rule), JSON round-trip for **profiles and settings**, **unknown-field preservation** (encode → decode → encode is byte-stable), schema-version refusal, slug edge cases (blank, all-non-ASCII → `profile`, truncation) and collision/duplicate naming, first-run template contents, **group gating** (effective enabled, `EnabledCount`) and group round-trip/unknown-field preservation |
| `internal/engine` | `FakeDriver` + a **fake clock** + a **seeded `Rng`**: plan order (disabled skipped, order preserved; ungrouped then groups; a disabled group skips its modules), key modules press the combo `count` times with `holdMs`, group `repeat` and `delayAfter` (last group skipped), per-click `delayAfter` spacing (`holdMs + delayAfter`), loop modes incl. `loopDelay` not applied after the last pass, uniform inclusive `loopDelay` draws, **resume does not re-roll** a drawn delay, `ElapsedMs`/`maxRuntime` exclude paused time, cancel-before-next-click, pause gating and resume-from-zero, stop releases without a trailing click, `ReleaseAll` on every exit path |
| `internal/input` | `FakeDriver` asserts no `Move` before `ClickHere`; capability degradation when `CursorPos` is unavailable; `Displays()` bounding box matches `ScreenSize()`; single-synthetic-display fallback |
| `internal/store` | atomic write (temp+rename), path resolution via mocks/injection, `profilesDir` override validation and fallback, unreadable file listing without overwrite, single-instance lock acquire/release |
| `internal/hotkey` | trigger-string parsing (the uppercase-modifier trap, [Appendix B](#appendix-b-global-shortcuts-portal-gotchas)) unit-testable in isolation; real grab is a manual check |
| UI | `vitest` for card summary formatting, validation rendering, empty/blocked states; manual pass for DnD and the picker overlay |

The engine tests are the ones that matter: with a fake clock and a fake driver, "clicked
`(940,520)` three times 90 ms apart, then 50 ms of nothing, then stopped within one
click of the signal" is a completely deterministic assertion.

### Manual release test matrix

1. **Linux/Wayland**: extract, run → verify `~/Documents/auto-clicker/` is created; add a
   module, pick coordinates, run once (`count: 3`) and confirm 3 clicks; toggle all
   modules off → Run blocked with the inline reason; enable, run `forever`, then press
   `F8` → instant stop; quit while running → no button held; launch a second
   instance → refused with a message.
2. **Linux/X11 session**: same, confirming the `x11` backend is selected and hotkeys work
   through `XGrabKey`.
3. **Windows 10/11**: same, plus confirm clicks land in a real game window and that
   `RegisterHotKey` fires while the game has focus.
4. **Regression**: display-geometry warning appears when a profile made at 5360×1440 is
   loaded on a different desktop.

---

## 10. CI / Release Pipeline (GitHub Actions)

Goal: push a tag → a **GitHub Release** with installable Linux and Windows binaries
attached, visible on the repo's Releases page.

### `ci.yml` — on push/PR
Two jobs. **Go**: install the Linux build deps (below) → `gofmt -l` (must be empty) →
`go vet -tags gtk3 ./...` → `go build -tags gtk3 ./...` → `go test -tags gtk3 ./...`.
`main.go` embeds `frontend/dist`, so the job drops a `.gitkeep` placeholder there to
satisfy the embed without compiling the frontend. **Frontend**: `npm ci` →
`tsc --noEmit` → `eslint .` → `vitest` → `vite build`. Never publishes. The Go build
includes `main.go`, so the CI runner needs the GTK3/WebKit2GTK-4.1 dev packages even for
`go test`, and `-tags gtk3` selects the right Wails backend. ESLint runs the classic hooks
rules (`rules-of-hooks`, `exhaustive-deps`) plus `typescript-eslint` and
`react-refresh`; the React Compiler rules are off because they flag patterns the app uses
on purpose.

**Linux build deps** (Ubuntu): `build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev`.
The project targets Wails' GTK3 backend via `-tags gtk3`; GTK4/WebKitGTK-6.0 is not needed.

**Local builds** (from the repo root, Wails CLI + Go + Node installed):
```sh
wails3 build                 # Linux  → bin/auto-clicker
wails3 build GOOS=windows    # Windows → bin/auto-clicker.exe (cross-compiled from Linux)
```
Both compile the frontend, regenerate bindings, embed `frontend/dist`, and print the output
path. `wails3 build GOOS=linux` is the explicit Linux form.

### `release.yml` — on push to `main`, tag `v*`, or `workflow_dispatch`

| Job | Runner | Steps |
|---|---|---|
| `version` | `ubuntu-24.04` | resolve the release tag: a pushed tag is used verbatim (`v1.2.3`); a branch/dispatch build gets `v0.1.<run_number>` |
| `build-linux` | `ubuntu-24.04` | install `build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev` → setup Go + Node → `wails3 build` (`GOOS=linux`) → upload artifact |
| `build-windows` | `ubuntu-24.04` | cross-compile `wails3 build GOOS=windows` with `CGO_ENABLED=0` — possible *because* the robotgo backends are pure Go and Wails needs no CGO on Windows → upload artifact |
| `package` | `ubuntu-24.04` | needs both → name them `auto-clicker_<version>_linux_amd64.tar.gz` and `auto-clicker_<version>_windows_amd64.zip` (binary only, one top-level entry) plus a `checksums.txt` sidecar → `softprops/action-gh-release` **published** (`draft: false`, `make_latest: true`) |

Design choices:
- **Publish directly.** Every push to `main` builds both OS binaries and publishes a
  GitHub Release, so they are downloadable from the Releases page without a tag step.
  Pushing a tag (`v*`) publishes a semantic version instead of the auto-increment.
- **Workflow artifacts too.** Both jobs upload their binary as a workflow artifact, so a
  run page also offers a download (artifacts expire after 90 days; releases do not).
- **Windows is cross-compiled from Linux**, so no Windows runner is needed. (macOS would
  require a macOS runner — another reason it is out of scope.)
- The Wails CLI is installed with `-tags gtk3` (and both jobs install the GTK3/WebKit2GTK
  dev packages) because the CLI and the host-side binding generator type-check the app on
  Linux, where the default backend would demand GTK4/WebKitGTK-6.0.
- Both build jobs keep their artifacts even if the other fails, so a Linux failure does
  not hide the Windows binary.
- `package` depends on both, so the release only appears complete or not at all.
- **Pinned to Ubuntu 24.04.** `ubuntu-latest` migrates to Ubuntu 26 in October 2026; the
  release jobs stay on 24.04 so the GTK3/WebKit2GTK-4.1 dev packages are stable.
- **The Wails CLI install retries.** `go install` occasionally fails while verifying a
  module against `sum.golang.org` (transient stream errors); the step retries three times
  before failing.
- Version is exported as the `VERSION` environment variable. The platform Taskfiles append
  `-X main.version=$VERSION` to `-ldflags` when `VERSION` is set, so the
  About/diagnostics panel reports it and plain local builds keep the `dev` default.

---

## 11. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| `ClickHere` has an implicit motion on libei | `cursor` targets unusable on Wayland | Phase 0.5.1 is blocking; if it fails, ship `cursor` for x11/Windows only and gate it per-capability in the UI |
| `CursorPos()` unworkable on Wayland | no picker | manual coordinate entry + coordinate display in diagnostics |
| libei cannot report screen geometry / displays | validation, the display warning and per-monitor picking depend on the `x11` fallback | Phase 0.5.6; `CanEnumerateDisplays` gate; manual coordinate entry always available |
| libei init always prompts (no cached-token probe) | the Wayland upgrade can never be automatic | resolved: token is a plain file, checked before init; explicit action remains the fallback |
| User declines the libei ScreenCast source | `absolute` targets unreliable on Wayland (relative fallback drifts) | `CanMoveAbsolute` gates the target kind; diagnostics shows stream state; app stays usable on `x11`/`cursor` |
| Mixed-DPI coordinate mismatch | clicks land on the wrong pixel | Phase 0.5.4; schema has room for a `displays[]` layout snapshot |
| robotgo `master` pin regresses | build or runtime breakage | exact commit pinned in `go.sum`; `Driver` interface isolates the swap |
| Wayland overlay cannot cover all monitors | picker limited to one monitor at a time | per-monitor overlay plus origin offset; manual entry always available |
| Hotkey source unavailable | user cannot stop a run while unfocused | startup warning; in-window shortcuts; max runtime cap |
| Anti-cheat / fullscreen games block injection | feature appears broken | README guidance; diagnostics panel shows backend + geometry |

---

## 12. Open Questions

1. **Corner failsafe.** Removed from the profile. Stopping is the global hotkey, the
   in-window shortcuts and the runtime cap; there is no "drag the pointer into a corner"
   gesture.
2. **`delayAfter` humanization.** Resolved without a new range field: the profile has a
   `humanize` toggle that draws every fixed delay in ±25% of its value ([§3.2](#32-timing)).
   Fixed per-click, per-group and per-pass delays all become irregular; explicit
   `{min,max}` loop ranges keep their own range.
3. **Burst vs rate ergonomics.** `count` is capped at 1000 and continuous clicking is
   expressed with `loop: forever`. Is that discoverable enough, or should the UI offer a
   "repeat" helper that *writes* that combination for the user? Worth a usability pass
   after the first build.
4. **Tray icon.** Dropped: closing the window keeps stopping the run, and the app stays a
   single window with no per-OS tray code. (Revisit only if running while closed becomes a
   real need.)
5. **Coordinate re-mapping on display change.** The warning banner tells the user to
   re-pick. A future version could offer a one-click proportional re-map
   (old geometry → new geometry) with a preview. Nice, but only if the warning turns out
   to be noisy in practice.
6. **Name.** Working title `auto-clicker` / "Auto Clicker", application id `auto-clicker`
   ([§2.4](#24-module-path-and-naming)). The app id is fixed now (KDE consent is keyed to
   it); only the human-facing name still needs a confirm before the first release.

---

## Appendix A — JSON Schema Reference

```jsonc
{
  "schemaVersion": 1,              // int; newer than this build → refuse to load
  "name": "Idle Tycoon",           // string; also determines the filename slug
  "display": { "width": 5360, "height": 1440 },   // physical px, geometry coords were captured against
  "loop": {
    "mode": "once" | "count" | "forever",
    "count": 10,                   // used when mode == "count"; ≥ 1
    "loopDelay": { "min": 500, "max": 1500 }      // ms; min == max → fixed delay
  },
  "failsafe": {
    "maxRuntimeSec": 3600          // 0 = unlimited
  },
  "humanize": false,               // true → every fixed delay drawn in ±25%
  "modules": [                     // ungrouped clicks (§3.12)
    {
      "kind": "click",             // "click" | "key"; absent == "click" (§3.9)
      // "keys": "ctrl+c",         // required for kind: key; target/button unused
      "id": "m1",                  // string, stable; m_ + 8 hex, unique in the profile
      "name": "",                  // optional label; empty → UI generates a summary
      "enabled": true,
      "target": { "kind": "absolute", "x": 940, "y": 520 },   // or { "kind": "cursor" }
      "button": "left",            // "left" | "right" | "middle" | "none" (none = move only)
      "count": 3,                  // 1..1000
      "holdMs": 10,                // ms, 0..1000, button-down duration
      "holdUntilStop": false,      // true = keep pressed until Stop (last enabled module, loop once)
      "delayAfter": 250,           // ms, 0..3600000, after EVERY click (period = holdMs + delayAfter)
      "clickInterval": 0           // deprecated; folded into delayAfter on load
    }
  ],
  "groups": [                      // optional; absent == [] (§3.12)
    {
      "id": "g_ab12cd34",
      "name": "Adventure",
      "enabled": true,
      "repeat": 3,                 // 1..1000; times this group runs before the next
      "delayAfter": 500,           // ms, 0..3600000, after the group's repeats (skipped for the last group)
      "modules": [ /* … */ ]
    }
  ]
}
```

Notes:
- Absent numeric fields take the defaults in [§3.3](#33-module-fields-the-complete-vocabulary);
  the app always writes them explicitly so files are self-describing.
- `"kind"` is written explicitly as `"click"`; **absent still means `"click"`**. A
  `"kind": "key"` module adds `"keys": "ctrl+shift+c"` and ignores `target`/`button`
  (same `count`, `holdMs`, `delayAfter`). See [§3.9](#39-schema-evolution).
- **Groups** ([§3.12](#312-groups)): ungrouped `modules` run first, then each enabled
  `groups[]` entry in order; a module runs only if both it and its group are enabled. Each
  group repeats `repeat` times per pass, with `delayAfter` before the next group. Group
  ids are unique within the profile; deleting a group also deletes its modules (with a
  confirmation).
- Unknown keys at any level are preserved on round-trip.
- `settings.json` sits in the app folder but is not a profile, specified in
  [§3.10](#310-app-settings-schema); profiles live in its `profiles/` subfolder and it is
  skipped when listing them.

---

## Appendix B — Global Shortcuts Portal Gotchas

Carried verbatim because every one of these fails **silently** (or catastrophically) and
cost a debugging session each. Verified against `xdg-desktop-portal` 1.21.1 source.

1. **`CreateSession` must pass `session_handle_token`** — distinct from `handle_token`,
   which only names the Request path. Missing it triggers `g_assert(token != NULL)` →
   abort → **core dump of `xdg-desktop-portal.service`**.
2. **The `session_handle` in the response arrives as D-Bus type `s`** (string), not `o`,
   even though later methods take it as an object path.
3. **`preferred_trigger` uses UPPERCASE modifier names** — `"CTRL+SHIFT+F12"`, not the
   Qt-notation `"Ctrl+Shift+F12"`. In `xdgshortcut.cpp`, `XdgShortcut::parse` looks
   modifiers up in a case-sensitive `QHash` keyed `CTRL`/`SHIFT`/`ALT`/`LOGO`/`CAPS`/`NUM`.
   A wrong-case modifier logs `Unknown modifier "Ctrl"` and produces an **empty binding**:
   the shortcut registers but never fires. Bare key names (`F8`) go through
   `xkb_keysym_from_name` and *are* case-insensitive, which is why they appeared to work
   and masked the bug.
4. **`CreateSession` refuses callers with an empty app id** ("An app id is required") — a
   risk for unsandboxed binaries; confirmed acceptable in the spike by setting an app id.
   Two further traps, hit in practice:
   - **`org.freedesktop.host.portal.Registry.Register` needs a matching `.desktop` file.**
     For an unsandboxed app it fails with `App info not found for '<id>'` unless
     `~/.local/share/applications/<id>.desktop` exists; with it, `Register` succeeds and the
     shortcut component is named `<id>`. The app writes that file itself on startup
     ([§3.6](#36-platform-scope)) rather than relying on the user to install one.
   - **Launching from a snap terminal poisons the app id.** The child process inherits the
     snap cgroup (`/proc/<pid>/cgroup` → `snap.<name>.scope`), so the portal treats the app
     as sandboxed: `Registry.Register` is refused (`Can't manually register a
     io.snapcraft application`) and the shortcuts land under the *terminal's* component
     (e.g. `[code_code]`). Activation is then routed to that component and the app never
     receives `Activated`. Launch from a native terminal, a `.desktop` launcher, or
     `systemd-run --user --scope <binary>`.
5. **Subscribe to `Activated` without an object-path match.** The signal's path is not
   reliably the session handle; matching on `WithMatchObjectPath(session)` silently drops
   every activation. Match interface + member only and validate the session in the body.
6. **`BindShortcuts` must register every shortcut in a single call.** In
   `GlobalShortcutsSession::setActions`, each call registers only the shortcuts passed to
   *that* call and then deletes every shortcut the component already has that is absent
   from it ("We can forget the shortcuts that aren't around anymore" →
   `removeAllShortcuts`). One call per shortcut therefore makes each call wipe the
   previous ones: only the last id survives, silently. The response still reports a valid
   `trigger_description` for every id (it reads the session's own `m_shortcuts`, not the
   global grab state), but only one is actually grabbed. Symptom: exactly one line in
   `kglobalshortcutsrc`, and only that key ever emitting `Activated`.
7. **One `BindShortcuts` call with new shortcuts = one KDE consent dialog**, and the
   D-Bus reply is deferred until the user answers it (`delayReply`). The app must not
   block as if it hung; an unanswered dialog stalls registration indefinitely. Shortcuts
   already present in the component are "returning" and skip the dialog. A returning
   `(component, id)` also **keeps its old trigger** even when `preferred_trigger` changes,
   so the app derives the shortcut id from the trigger (`run-<hash>`, `pause-<hash>`):
   rebinding is a new id (one dialog), while restarts with the same trigger hit the
   returning path (no dialog).
   **`BindShortcuts` is asynchronous**: it returns `o request_handle`, and the granted
   shortcuts arrive in the `Response` signal's `shortcuts` result (`a(sa{sv})`) — *not* as
   the method's return value. Storing the method return directly fails with
   `cannot convert dbus.ObjectPath to []…`. `CreateSession`, `BindShortcuts` and
   `ListShortcuts` all follow this Request/Response pattern. The method arguments must
   also use a concrete struct for `a(sa{sv})`; `[]interface{}` is marshalled as `av` and
   rejected with a signature mismatch.
8. **Shortcuts are session-scoped**: closing the session removes them from
   `kglobalshortcutsrc`. Rebinding must happen on every start — the returning path makes
   this dialog-free once the ids exist.
9. **On Plasma 6 Wayland, `org.kde.kglobalaccel` is owned by `kwin_wayland` itself**;
   `kglobalacceld` starts and immediately exits (the name is already taken). That is
   normal — KWin is the grab daemon, so no extra service needs to be running.
10. **`XGrabKey` (X11 fallback)** only fires while an XWayland/X11 client has focus, which
   does cover Steam/Proton games. Its end-to-end behavior was implemented but not yet
   exercised in the spike.
11. **`RegisterHotKey` (Windows)** cross-builds cleanly. Implementation note: register with
    a **NULL hwnd on a dedicated thread** and pump `WM_HOTKEY` from that thread's message
    queue — no window is needed. `RegisterHotKey(NULL, …)` associates the hotkey with the
    *calling* thread, so registration must be marshalled onto the pumping thread, and the
    thread's message queue must exist first (a `PeekMessage` call forces it). A message-only
    window is unnecessary and was replaced by this.
12. **Windows `GetSystemMetrics(SM_CXSCREEN)` is the primary monitor only.** The virtual
    desktop size must be the union of the enumerated display rectangles, otherwise a
    multi-monitor profile's display snapshot is wrong.

Three bugs this produced in the spike, all silent:
- case-sensitive modifiers → empty binding for anything with a modifier;
- one `BindShortcuts` call per shortcut → only the last survived;
- missing `session_handle_token` → crashed `xdg-desktop-portal` itself.

---

## Appendix C — Stack Validation Results

Measured on Kubuntu / Plasma 6 Wayland, 2026-09-22, dual-monitor desktop of 5360×1440.

| Test | Backend | Result |
|---|---|---|
| Build with `CGO_ENABLED=0`, no system headers | all build tags | ✅ 4.3–5.8 MB binaries, `go vet` clean per tag |
| Cursor read + 1 px move injection | `x11` | ✅ screen size read correctly (5360×1440) |
| Click timing, 50 clicks @ 20 CPS | `x11` | ✅ p95 lateness **920 µs**, drift −39 ms over 2.5 s |
| Click timing, 50 clicks @ 10 CPS | `libei` | ✅ p95 lateness **1.0 ms**, drift −88 ms over 5 s |
| Keyboard taps, 20 @ 5 CPS | `x11` | ✅ p95 lateness 937 µs |
| Stop latency at 50 CPS | `x11` | ✅ ~9 ms from interrupt to halt; no click after the signal |
| Portal consent, first run | `libei` | ✅ dialog shown, granted, token cached; later runs non-interactive |
| `wayland` (wlroots) backend | `wayland` | ❌ bogus geometry on KWin → dropped |
| **x11 + libei coexisting in one binary** | both | ✅ same process, both inject; dual-backend binary 5.2 MB |
| Windows cross-build | `win` | ✅ compiles (3.4 MB); runtime test pending |
| Global hotkeys via `GlobalShortcuts` | portal | ✅ `F8`/`F9`/`CTRL+SHIFT+F12` all registered **and firing** (`Activated` → dispatch), single consent dialog |
| Global hotkeys via `XGrabKey` | `x11` | ⏳ implemented, not exercised end-to-end |
| Global hotkeys via `RegisterHotKey` | `win` | ⏳ cross-builds clean (2.6 MB); runtime test pending |

These numbers are why the design can promise drift-free timing and a stop that lands
within one click: the scheduler and the cancellation path are already known to behave.
Everything still marked **[unproven]** in this document is listed in
[§8 Phase 0.5](#8-phases).

### Phase 0.5 spike (this app)

Measured 2026-09-22 on the same Kubuntu / Plasma 6 Wayland desktop, with the pinned
robotgo commit and `CGO_ENABLED=0`. Harness: `spike/phase05/`.

| Test | Backend | Result |
|---|---|---|
| `x11` + `libei` sub-packages in one binary | both | ✅ 5.2 MB static binary, no cgo |
| `win` sub-package cross-build | `win` | ✅ `GOOS=windows` builds clean |
| Screen size, display enumeration, real cursor read | `x11` | ✅ 5360×1440; 2 displays via Xinerama with origins; real pointer position read |
| Per-button press/release primitives | all three | ✅ `MouseDown`/`MouseUp`/`Toggle`; `Click()` hardcodes a 10 ms hold |
| `ClickHere` sends a button event with no motion | `x11`, `libei` | ✅ by source inspection (`XTEST` / `NotifyPointerButton`); runtime pending |
| Absolute motion + geometry | `libei` | ⚠️ only with a linked ScreenCast stream; otherwise relative fallback parks the pointer top-left and drifts after physical mouse movement |
| Restore-token probe without a dialog | `libei` | ✅ plain file `$XDG_STATE_HOME/robotgo/portal_token` |
| Mixed-DPI coordinate round-trip | both | ⏳ pending a human run |
| Fullscreen overlay placement | Wails | ⏳ pending (needs a window) |

No input was injected and no portal dialog was opened by the automated part of this run;
the interactive checks are left for a manual pass with `spike/phase05`.
