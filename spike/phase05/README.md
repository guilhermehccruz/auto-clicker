# Phase 0.5 spike

Runnable harness for the blocking spike described in
[`../../DESIGN.md`](../../DESIGN.md) §8 Phase 0.5. It exercises the exact robotgo
sub-packages the app will ship: `robotgo/x11` and `robotgo/libei`, in one binary.

**Results so far** are recorded in Appendix C of `DESIGN.md`. The automated part was
read-only; the checks below marked _interactive_ need a human at the portal consent
dialog and a human to verify where a click actually landed.

## Build

```sh
CGO_ENABLED=0 go build -o phase05 .
```

Expected: a ~5 MB statically linked binary, no cgo.

## Read-only (safe)

```sh
./phase05 read
```

Prints `x11` screen size, per-display rectangles (Xinerama), scale, and the **real**
pointer position. Injects nothing and opens no portal session.

## Interactive (injects input / opens the portal)

Every command below **moves the real pointer, clicks, or opens a portal consent
dialog**, and requires `-yes`. There is a 2–3 s countdown before injection. Point at
something harmless first (e.g. an empty text editor).

### Q1 — `ClickHere` lands at the real pointer, no motion

Put the pointer somewhere identifiable, then:

```sh
./phase05 -yes x11-clickhere -count 3 -hold 20
./phase05 -yes -link-screencast libei-clickhere -count 3 -hold 20
```

Verify the click landed **where the pointer already was**, not at `(0, 0)`.

### Q2 — per-button press/release (already confirmed at source)

`-button left|right|middle` selects the button; `-hold MS` sets the down time.

### Q3 / Q6 — geometry and capabilities through libei

```sh
./phase05 -yes -link-screencast libei-info   # expects a 'share screen' step in the dialog
./phase05 -yes                    libei-info # no stream -> geometry is 0x0
```

Compare `DisplaysNum` / `GetScreenSize` between the two. Without `-link-screencast`,
libei returns zero geometry; the app therefore reads geometry from the `x11` path.

### Q4 — mixed-DPI / multi-monitor round-trip

On a desktop with a scaled monitor, pick a point on it, then:

```sh
./phase05 -yes -link-screencast libei-move <x> <y>
./phase05 -yes x11-click <x> <y>
```

Confirm the pointer lands on the same physical pixel under both backends.

## Notes

- The restore token lives at `$XDG_STATE_HOME/robotgo/portal_token`
  (fallback `~/.local/state/robotgo/portal_token`). Delete it to force a fresh consent
  dialog.
- This file is shared with any other robotgo app on the machine.
- `libei` without a ScreenCast stream moves by relative deltas and, on first move, parks
  the pointer in the top-left corner. Do not use it for `absolute` targets.
