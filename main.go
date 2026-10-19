// Command auto-clicker is the Wails v3 desktop shell (DESIGN §7, §4.5).
//
// Build with `wails3 build` (or `wails3 dev`). The default build embeds the
// compiled frontend from frontend/dist, so run the frontend build first — the
// Taskfile does this automatically.
package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"

	"auto-clicker/internal/desktop"
	"auto-clicker/internal/engine"
	"auto-clicker/internal/logging"
	"auto-clicker/internal/service"
	"auto-clicker/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is injected at build time with
// -ldflags "-X main.version=…" (DESIGN §10).
var version = "dev"

func main() {
	appDir, err := store.DefaultAppDir()
	if err != nil {
		log.Fatalf("app dir: %v", err)
	}
	// Log to a file in the app folder first: the Windows GUI binary has no console.
	if logPath, err := logging.Setup(appDir); err != nil {
		log.Printf("logging setup failed: %v", err)
	} else {
		log.Printf("auto-clicker %s starting; log file: %s", version, logPath)
	}

	configDir, err := store.ConfigDir()
	if err != nil {
		log.Fatalf("config dir: %v", err)
	}

	// Register a launcher entry on Linux so the app appears in the desktop's
	// application search (and the GlobalShortcuts portal can find its .desktop
	// file). Best-effort: never block startup on it.
	if err := desktop.Ensure(); err != nil {
		log.Printf("desktop integration: %v", err)
	}

	// Single-instance lock before any service starts (DESIGN §4.5).
	lock, lockErr := store.AcquireLock(configDir)
	if lockErr != nil && lockErr != store.ErrLocked {
		log.Fatalf("lock: %v", lockErr)
	}

	// app and the main window are assigned below; the closures only run once the
	// app is running.
	var app *application.App
	var mainWin *application.WebviewWindow
	// saveGeometry persists the window geometry; assigned once the window
	// exists, and called from OnShutdown.
	var saveGeometry func()
	var geomTimer *time.Timer
	// updaterReady gates the update hooks until the updater is configured.
	var updaterReady bool
	svc, err := service.New(service.Options{
		Version:   version,
		ConfigDir: configDir,
		AppDir:    appDir,
		Emitter: func(p engine.Progress) {
			if app != nil {
				app.Event.Emit("run:progress", p)
			}
		},
		PickDone: func(r service.PickResult) {
			if app != nil {
				app.Event.Emit("pick:done", r)
			}
		},
		FocusMain: func() {
			if mainWin != nil {
				mainWin.Show()
				mainWin.Focus()
			}
		},
		CheckUpdate: func() (service.UpdateInfo, error) {
			if !updaterReady {
				return service.UpdateInfo{}, nil
			}
			rel, err := app.Updater.Check(context.Background())
			if err != nil {
				return service.UpdateInfo{}, err
			}
			if rel == nil {
				return service.UpdateInfo{}, nil
			}
			return service.UpdateInfo{
				Available: true,
				Version:   rel.Version,
				Notes:     rel.Notes,
				URL:       "https://github.com/guilhermehccruz/auto-clicker/releases/tag/v" + rel.Version,
			}, nil
		},
		InstallUpdate: func() error {
			if !updaterReady {
				return errors.New("update: not available")
			}
			// Keep the updater's staging file on the binary's filesystem so the
			// Unix rename-based swap cannot fail with EXDEV (see samefs_linux.go).
			restore := sameFSTempDir()
			defer restore()
			if err := app.Updater.DownloadAndInstall(context.Background()); err != nil {
				return err
			}
			return app.Updater.Restart(context.Background())
		},
	})
	if err != nil {
		log.Fatalf("service: %v", err)
	}

	app = application.New(application.Options{
		Name:        "Auto Clicker",
		Description: "Repetitive-clicking autoclicker",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		OnShutdown: func() {
			// Persist the window geometry, stop the run (releases inputs),
			// unregister hotkeys, drop lock.
			if geomTimer != nil {
				geomTimer.Stop()
			}
			if saveGeometry != nil {
				saveGeometry()
			}
			_ = svc.Stop()
			_ = svc.CloseHotkeys()
			if lock != nil {
				_ = lock.Release()
			}
		},
	})

	// Self-update against GitHub Releases. Skipped for non-semver builds (dev).
	if v := strings.TrimPrefix(version, "v"); v != "" && v != "dev" {
		provider, perr := github.New(github.Config{
			Repository:    "guilhermehccruz/auto-clicker",
			ChecksumAsset: "checksums.txt",
		})
		if perr != nil {
			log.Printf("updater: provider: %v", perr)
		} else if ierr := app.Updater.Init(updater.Config{
			CurrentVersion: v,
			Providers:      []updater.Provider{provider},
			Window:         updater.WindowNone,
		}); ierr != nil {
			log.Printf("updater: init: %v", ierr)
		} else {
			updaterReady = true
		}
	}

	if lockErr == store.ErrLocked {
		// A second launch starts no services and just reports (DESIGN §6).
		app.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:  "Auto Clicker",
			Width:  360,
			Height: 140,
			HTML:   `<body style="font:13px sans-serif;background:#0a0a0a;color:#e5e5e5;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">Auto Clicker is already running</body>`,
		})
		if err := app.Run(); err != nil {
			log.Fatal(err)
		}
		return
	}

	// The picker has no window of its own: the countdown runs inside this
	// window and the service captures the real cursor after PickCountdown
	// (DESIGN §5.7). This avoids Wayland's window positioning/focus limits.
	//
	// Restore the saved geometry, falling back to 1100×720 centered.
	winOpts := application.WebviewWindowOptions{
		Title:  "Auto Clicker",
		Width:  1100,
		Height: 720,
		URL:    "/",
	}
	if saved := svc.GetSettings().Window; saved.W > 0 && saved.H > 0 {
		winOpts.Width, winOpts.Height = saved.W, saved.H
		if saved.X != 0 || saved.Y != 0 {
			winOpts.InitialPosition = application.WindowXY
			winOpts.X, winOpts.Y = saved.X, saved.Y
		}
		if saved.Maximized {
			winOpts.StartState = application.WindowStateMaximised
		}
	}
	mainWin = app.Window.NewWithOptions(winOpts)

	// Persist geometry on move/resize/maximise, debounced so a drag does not
	// hammer the disk. OnShutdown saves once more.
	saveGeometry = func() {
		w, h := mainWin.Size()
		if w <= 0 || h <= 0 {
			return
		}
		x, y := mainWin.Position()
		st := svc.GetSettings()
		st.Window = store.Window{X: x, Y: y, W: w, H: h, Maximized: mainWin.IsMaximised()}
		if err := svc.SaveSettings(st); err != nil {
			log.Printf("save window geometry: %v", err)
		}
	}
	scheduleSave := func() {
		if geomTimer != nil {
			geomTimer.Stop()
		}
		geomTimer = time.AfterFunc(500*time.Millisecond, saveGeometry)
	}
	for _, ev := range []events.WindowEventType{
		events.Common.WindowDidMove,
		events.Common.WindowDidResize,
		events.Common.WindowMaximise,
		events.Common.WindowUnMaximise,
	} {
		mainWin.OnWindowEvent(ev, func(*application.WindowEvent) { scheduleSave() })
	}

	// Register global hotkeys off the UI thread: the portal may show a consent
	// dialog and the D-Bus reply is deferred until it is answered.
	go func() {
		log.Printf("hotkey: source=%s", svc.HotkeySourceName())
		granted, err := svc.RegisterHotkeys()
		if err != nil {
			log.Printf("global hotkeys unavailable: %v", err)
			return
		}
		log.Printf("global hotkeys registered: %v", granted)
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
