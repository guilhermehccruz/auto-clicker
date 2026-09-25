# Auto Clicker

A single-purpose autoclicker for repetitive clicking in clicker/idle games and any other
"click this spot N times" chore.

This is a personal project. I built it out of a need to automate a few repetitive actions
in offline idle games, and I am sharing it in case it is useful to someone with the same
itch. It is not a general-purpose product and it is not trying to become one.

## What it does

- Compose a run from ordered, toggleable **click modules** that execute strictly
  sequentially.
- Each module clicks at fixed screen coordinates **or** at the current cursor position.
- Run once, N times, or forever, with drift-free timing.
- Profiles with autosave, optional groups, a global panic hotkey, and a runtime cap.
- Ships as one desktop binary per OS (Linux + Windows), no installer and no runtime
  dependencies beyond the OS webview.

## Stack

A Go engine plus a React/TypeScript UI, packaged as a single binary with
[Wails v3](https://v3.wails.io). Linux uses the GTK3/WebKit2GTK-4.1 backend; Windows is
cross-compiled from Linux with CGO disabled because the input backends are pure Go.

## Development

Prerequisites: Go 1.26+, Node 22+, the Wails v3 CLI, and on Linux the GTK3/WebKit2GTK-4.1
development packages (`build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev`).

```sh
wails3 task dev     # backend + Vite dev server together
wails3 task build   # production build -> bin/auto-clicker
```

Run the frontend and backend separately:

```sh
# terminal 1
wails3 task common:dev:frontend

# terminal 2
wails3 task build DEV=true
FRONTEND_DEVSERVER_URL=http://127.0.0.1:9245 wails3 task run
```

Tests:

```sh
go test -tags gtk3 ./internal/...
cd frontend && npm test
```

## Status

The MVP is implemented and covered by unit tests. See [`DESIGN.md`](./DESIGN.md) for the
full specification, architecture, and roadmap. CI and release workflows live in
[`.github/workflows/`](./.github/workflows/); every push to `main` publishes a GitHub
Release with Linux and Windows builds ready to download.
