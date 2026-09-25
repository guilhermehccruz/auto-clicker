// Command auto-clicker is the Wails v3 desktop shell (DESIGN §7, §4.5).
//
// Build with `wails3 build` (or `wails3 dev`). The default build embeds the
// compiled frontend from frontend/dist, so run the frontend build first — the
// Taskfile does this automatically.
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

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
	// Log to a file first: the Windows GUI binary has no console.
	if logPath, err := logging.Setup(); err != nil {
		log.Printf("logging setup failed: %v", err)
	} else {
		log.Printf("auto-clicker %s starting; log file: %s", version, logPath)
	}

	configDir, err := store.ConfigDir()
	if err != nil {
		log.Fatalf("config dir: %v", err)
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
	svc, err := service.New(service.Options{
		Version:   version,
		ConfigDir: configDir,
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
			// Release inputs, flush autosave, unregister hotkeys, drop lock.
			_ = svc.Panic()
			_ = svc.CloseHotkeys()
			if lock != nil {
				_ = lock.Release()
			}
		},
	})

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
	mainWin = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Auto Clicker",
		Width:  1100,
		Height: 720,
		URL:    "/",
	})

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
