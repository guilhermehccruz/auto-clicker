// Package service wires store, engine, driver, cursor and hotkeys into the
// operations the UI binds to (DESIGN §7.2). It is deliberately free of any
// Wails dependency so it can be tested directly.
package service

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"auto-clicker/internal/cursor"
	"auto-clicker/internal/engine"
	"auto-clicker/internal/hotkey"
	"auto-clicker/internal/input"
	"auto-clicker/internal/model"
	"auto-clicker/internal/store"
)

// AppID is the portal application id (DESIGN §2.4).
const AppID = "auto-clicker"

// Diagnostics is the support payload (DESIGN §5.5).
type Diagnostics struct {
	Version         string          `json:"version"`
	RobotgoPin      string          `json:"robotgoPin"`
	AppID           string          `json:"appId"`
	Backend         string          `json:"backend"`
	FallbackReason  string          `json:"fallbackReason,omitempty"`
	CanReadCursor   bool            `json:"canReadCursor"`
	CanPickPoint    bool            `json:"canPickPoint"`
	CanMoveAbsolute bool            `json:"canMoveAbsolute"`
	DesktopWidth    int             `json:"desktopWidth"`
	DesktopHeight   int             `json:"desktopHeight"`
	Displays        []input.Display `json:"displays"`
	HotkeySource    string          `json:"hotkeySource"`
	HotkeyError     string          `json:"hotkeyError,omitempty"`
	AppDir          string          `json:"appDir"`
	ProfilesDir     string          `json:"profilesDir"`
	LogDir          string          `json:"logDir"`
}

// Options configures a Service. Store and Driver are injectable for tests.
type Options struct {
	ConfigDir string
	// AppDir is the app folder (settings, runs, log, profiles/). Empty resolves
	// the default. Ignored when Store is injected.
	AppDir  string
	Store   *store.Store
	Driver  input.Driver
	Hotkeys hotkey.Source
	// NewHotkeys builds a fresh global-shortcut source; used on rebinding
	// (a closed portal session is not reusable). Defaults to hotkey.New.
	NewHotkeys func(hotkey.Handler) hotkey.Source
	Version    string
	Clock      engine.Clock
	Rng        engine.Rng
	// Emitter receives run progress. It is not a bound method so the generated
	// bindings stay free of function types.
	Emitter func(engine.Progress)
	// PickDone is called when a pick confirms, cancels or times out. It is not
	// a bound method, so the generated bindings stay free of function types.
	PickDone func(PickResult)
	// FocusMain brings the app window back to the front after a pick (the game
	// may have been fullscreen and focused).
	FocusMain func()
	// CheckUpdate reports whether a newer release is available. Wired by main
	// to the Wails updater; nil disables updates (e.g. dev builds).
	CheckUpdate func() (UpdateInfo, error)
	// InstallUpdate downloads and installs the pending update, then restarts.
	InstallUpdate func() error
}

// PickResult is the outcome of a pick; Ok is false on cancel.
type PickResult struct {
	X  int  `json:"x"`
	Y  int  `json:"y"`
	Ok bool `json:"ok"`
}

// UpdateInfo describes an available release.
type UpdateInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Notes     string `json:"notes"`
	URL       string `json:"url"`
}

// Service is the app's operation surface.
type Service struct {
	mu sync.Mutex

	store       *store.Store
	settingsDir string
	drv         input.Driver
	engine      *engine.Engine
	cursor      *cursor.Prober
	hotkeys     hotkey.Source
	settings    store.Settings
	version     string
	selected    string
	newHotkeys  func(hotkey.Handler) hotkey.Source

	pickDone      func(PickResult)
	focusMain     func()
	checkUpdate   func() (UpdateInfo, error)
	installUpdate func() error
	pickMu        sync.Mutex
	picking       bool
	pickCancel    chan struct{}

	// Run-history bookkeeping, guarded by mu.
	runActive  bool
	runStarted time.Time
	runProfile string
	runMode    string
}

// New builds a Service, loading settings and opening the store.
func New(opts Options) (*Service, error) {
	configDir := opts.ConfigDir
	if configDir == "" {
		d, err := store.ConfigDir()
		if err != nil {
			return nil, err
		}
		configDir = d
	}

	// Everything lives in the app folder (DESIGN §3.8): settings.json, runs.jsonl,
	// app.log and profiles/. With an injected store its Root is used, keeping
	// tests isolated. configDir is only the legacy settings fallback.
	st := opts.Store
	settingsDir := ""
	if st != nil {
		settingsDir = st.Root
	} else {
		appDir := opts.AppDir
		if appDir == "" {
			d, err := store.DefaultAppDir()
			if err != nil {
				return nil, err
			}
			appDir = d
		}
		if err := store.MigrateLayout(appDir); err != nil {
			log.Printf("store: layout migration: %v", err)
		}
		settingsDir = appDir
	}
	settings, err := store.LoadSettings(settingsDir, configDir)
	if err != nil {
		return nil, err
	}
	if st == nil {
		st, err = store.Open(settingsDir)
		if err != nil {
			return nil, err
		}
	}

	drv := opts.Driver
	if drv == nil {
		drv, err = input.New()
		if err != nil {
			return nil, err
		}
	}
	s := &Service{
		store:         st,
		settingsDir:   settingsDir,
		drv:           drv,
		cursor:        cursor.New(drv),
		settings:      settings,
		version:       opts.Version,
		selected:      settings.SelectedProfile,
		pickDone:      opts.PickDone,
		focusMain:     opts.FocusMain,
		checkUpdate:   opts.CheckUpdate,
		installUpdate: opts.InstallUpdate,
	}
	switch {
	case opts.Hotkeys != nil:
		// A fixed source (tests): rebinding reuses it.
		s.newHotkeys = func(hotkey.Handler) hotkey.Source { return opts.Hotkeys }
	case opts.NewHotkeys != nil:
		s.newHotkeys = opts.NewHotkeys
	default:
		s.newHotkeys = hotkey.New
	}
	hk := opts.Hotkeys
	if hk == nil {
		hk = s.newHotkeys(s.onHotkey)
	}
	s.hotkeys = hk
	engineOpts := []engine.Option{engine.WithEmitter(func(p engine.Progress) {
		s.recordRun(p)
		if opts.Emitter != nil {
			opts.Emitter(p)
		}
	})}
	if opts.Clock != nil {
		engineOpts = append(engineOpts, engine.WithClock(opts.Clock))
	}
	if opts.Rng != nil {
		engineOpts = append(engineOpts, engine.WithRng(opts.Rng))
	}
	s.engine = engine.New(drv, engineOpts...)
	return s, nil
}

// --- profiles -------------------------------------------------------------

// ListProfiles returns readable profiles and unparseable files.
func (s *Service) ListProfiles() ([]store.Summary, []store.ProblemFile, error) {
	return s.store.List()
}

// LoadProfile loads one profile.
func (s *Service) LoadProfile(id string) (*model.Profile, error) {
	return s.store.Load(id)
}

// SaveProfile writes a profile and remembers it as selected.
func (s *Service) SaveProfile(id string, p *model.Profile) error {
	if err := s.store.Save(id, p); err != nil {
		return err
	}
	return s.selectProfile(id)
}

// CreateProfile creates a profile from the first-run template.
func (s *Service) CreateProfile(name string) (string, error) {
	w, h := s.desktopSize()
	id, err := s.store.Create(name, model.Display{Width: w, Height: h}, s.settings.DefaultMaxRuntimeSec)
	if err != nil {
		return "", err
	}
	if err := s.selectProfile(id); err != nil {
		return "", err
	}
	return id, nil
}

// RenameProfile renames a profile and its file, returning the new id.
func (s *Service) RenameProfile(id, newName string) (string, error) {
	newID, err := s.store.Rename(id, newName)
	if err != nil {
		return "", err
	}
	if err := s.selectProfile(newID); err != nil {
		return "", err
	}
	return newID, nil
}

// DuplicateProfile copies a profile.
func (s *Service) DuplicateProfile(id string) (string, error) {
	newID, err := s.store.Duplicate(id)
	if err != nil {
		return "", err
	}
	return newID, s.selectProfile(newID)
}

// DeleteProfile removes a profile.
func (s *Service) DeleteProfile(id string) error {
	return s.store.Delete(id)
}

// UpdateDisplaySnapshot refreshes a profile's display geometry.
func (s *Service) UpdateDisplaySnapshot(id string) error {
	w, h := s.desktopSize()
	return s.store.UpdateDisplaySnapshot(id, model.Display{Width: w, Height: h})
}

// --- validation & capabilities --------------------------------------------

// Validate returns the same issues the runner would refuse on.
func (s *Service) Validate(p *model.Profile) []model.Issue {
	return model.Validate(p, s.desktopInfo())
}

// desktopInfo builds the canonical coordinate bounds (primary-relative) and the
// virtual desktop size for validation.
func (s *Service) desktopInfo() model.Desktop {
	var d model.Desktop
	if displays, err := s.drv.Displays(); err == nil && len(displays) > 0 {
		d.MinX, d.MinY = displays[0].X, displays[0].Y
		d.MaxX, d.MaxY = displays[0].X+displays[0].W, displays[0].Y+displays[0].H
		for _, dp := range displays[1:] {
			d.MinX, d.MinY = min(d.MinX, dp.X), min(d.MinY, dp.Y)
			d.MaxX, d.MaxY = max(d.MaxX, dp.X+dp.W), max(d.MaxY, dp.Y+dp.H)
		}
	}
	d.Width, d.Height = s.desktopSize()
	return d
}

// Capabilities reports the active backend's feature set.
func (s *Service) Capabilities() input.Capabilities { return s.drv.Capabilities() }

// CursorPos returns the real pointer position when available.
func (s *Service) CursorPos() (int, int, bool) { return s.cursor.Pos() }

// Displays enumerates monitors for the picker.
func (s *Service) Displays() ([]input.Display, error) { return s.drv.Displays() }

// PickCountdown is the delay before the cursor is captured. The countdown runs
// in the app UI; the capture itself is driven by this Go timer, which is
// authoritative (the app may be unfocused/behind a fullscreen game).
const PickCountdown = 3 * time.Second

// StartPick begins a countdown and returns immediately. After PickCountdown the
// real cursor position is captured and delivered via the PickDone callback
// (`pick:done`). There is no second window: the countdown is shown inside the
// app, which sidesteps Wayland's window positioning/focus/transparency limits.
func (s *Service) StartPick() error {
	s.pickMu.Lock()
	if s.picking {
		s.pickMu.Unlock()
		return errors.New("picker already running")
	}
	s.picking = true
	cancel := make(chan struct{})
	s.pickCancel = cancel
	s.pickMu.Unlock()
	log.Printf("pick: start (countdown %s)", PickCountdown)

	go func() {
		select {
		case <-time.After(PickCountdown):
			x, y, ok := s.cursor.Pos()
			log.Printf("pick: captured x=%d y=%d ok=%v", x, y, ok)
			_ = s.finishPick(PickResult{X: x, Y: y, Ok: ok})
		case <-cancel:
		}
	}()
	return nil
}

// ConfirmPick captures immediately (Enter in the app, or a "capture now" button).
func (s *Service) ConfirmPick() error {
	x, y, ok := s.cursor.Pos()
	log.Printf("pick: confirm x=%d y=%d ok=%v", x, y, ok)
	return s.finishPick(PickResult{X: x, Y: y, Ok: ok})
}

// CancelPick abandons the countdown (Esc / cancel button while the app is focused).
func (s *Service) CancelPick() error {
	log.Printf("pick: cancel")
	return s.finishPick(PickResult{Ok: false})
}

// finishPick is idempotent and runs its callback off the calling goroutine.
func (s *Service) finishPick(r PickResult) error {
	s.pickMu.Lock()
	if !s.picking {
		s.pickMu.Unlock()
		return nil
	}
	s.picking = false
	cancel := s.pickCancel
	s.pickCancel = nil
	done := s.pickDone
	focus := s.focusMain
	s.pickMu.Unlock()

	if cancel != nil {
		close(cancel)
	}
	go func() {
		if focus != nil {
			focus()
		}
		if done != nil {
			done(r)
		}
	}()
	return nil
}

// Diagnostics assembles the support payload.
func (s *Service) Diagnostics() Diagnostics {
	caps := s.drv.Capabilities()
	w, h := s.desktopSize()
	displays, _ := s.drv.Displays()
	d := Diagnostics{
		Version:         s.version,
		AppID:           AppID,
		Backend:         caps.Backend,
		FallbackReason:  caps.FallbackReason,
		CanReadCursor:   caps.CanReadCursor,
		CanPickPoint:    caps.CanPickPoint,
		CanMoveAbsolute: caps.CanMoveAbsolute,
		DesktopWidth:    w,
		DesktopHeight:   h,
		Displays:        displays,
		HotkeySource:    s.hotkeys.Name(),
		HotkeyError:     hotkey.FallbackReason,
		AppDir:          s.store.Root,
		ProfilesDir:     s.store.Dir,
		LogDir:          s.store.Root,
	}
	return d
}

// --- runner ---------------------------------------------------------------

// Run starts the given profile.
func (s *Service) Run(profileID string) error {
	p, err := s.store.Load(profileID)
	if err != nil {
		return err
	}
	if issues := s.Validate(p); model.HasBlocking(issues) {
		return errors.New(firstBlocking(issues))
	}
	s.mu.Lock()
	s.runActive = false
	s.runProfile = p.Name
	s.runMode = p.Loop.Mode
	s.mu.Unlock()
	return s.engine.Run(p)
}

// recordRun captures run-history entries from progress events. It starts a
// record when a run becomes active and appends one when it returns to idle.
func (s *Service) recordRun(p engine.Progress) {
	s.mu.Lock()
	switch p.State {
	case engine.StateRunning, engine.StatePaused:
		if !s.runActive {
			s.runActive = true
			s.runStarted = time.Now()
			s.runProfile = p.ProfileName
		}
		s.mu.Unlock()
		return
	case engine.StateIdle:
		if !s.runActive {
			s.mu.Unlock()
			return
		}
		rec := store.RunRecord{
			StartedAt:  s.runStarted.UTC().Format(time.RFC3339),
			Profile:    s.runProfile,
			Mode:       s.runMode,
			StopReason: p.StopReason,
			Clicks:     p.Clicks,
			ElapsedMs:  p.ElapsedMs,
			Error:      p.Error,
		}
		limit := s.settings.RunHistoryLimit
		dir := s.store.Root
		s.runActive = false
		s.mu.Unlock()
		if err := store.AppendRun(dir, rec, limit); err != nil {
			log.Printf("run history: %v", err)
		}
		return
	default:
		s.mu.Unlock()
	}
}

// GetRunHistory returns the recorded runs, newest first.
func (s *Service) GetRunHistory() ([]store.RunRecord, error) {
	s.mu.Lock()
	limit := s.settings.RunHistoryLimit
	s.mu.Unlock()
	return store.ListRuns(s.store.Root, limit)
}

// ClearRunHistory deletes the recorded runs.
func (s *Service) ClearRunHistory() error { return store.ClearRuns(s.store.Root) }

// Stop cancels the run immediately, releasing all inputs.
func (s *Service) Stop() error { return s.engine.Stop() }

// Pause pauses the run.
func (s *Service) Pause() error { return s.engine.Pause() }

// Resume resumes the run.
func (s *Service) Resume() error { return s.engine.Resume() }

// Status returns the latest progress.
func (s *Service) Status() engine.Progress { return s.engine.Status() }

// hotkeyBindings returns the configured global shortcuts. The binding id
// carries the action prefix plus a short hash of the trigger: KDE's portal
// treats a known (component, id) as a "returning" shortcut and keeps the old
// binding, ignoring preferred_trigger, so a new trigger must be a new id.
func (s *Service) hotkeyBindings() []hotkey.Binding {
	s.mu.Lock()
	hk := s.settings.Hotkeys
	s.mu.Unlock()
	return []hotkey.Binding{
		{ID: bindingID("run", hk.Run), Label: "Run / Stop", Trigger: hk.Run},
		{ID: bindingID("pause", hk.Pause), Label: "Pause / Resume", Trigger: hk.Pause},
	}
}

// bindingID keeps the action prefix stable while changing with the trigger.
func bindingID(action, trigger string) string {
	sum := sha1.Sum([]byte(trigger))
	return action + "-" + hex.EncodeToString(sum[:3])
}

// RegisterHotkeys binds the configured global shortcuts. It may block on a
// portal consent dialog, so callers should run it off the UI thread.
func (s *Service) RegisterHotkeys() (map[string]string, error) {
	s.mu.Lock()
	hk := s.hotkeys
	s.mu.Unlock()
	return hk.Register(s.hotkeyBindings())
}

// SetHotkeys canonicalizes, persists and applies new Run/Pause triggers. It
// rebuilds the source because a portal session is not reusable after Close.
func (s *Service) SetHotkeys(run, pause string) (store.Settings, error) {
	cr, err := hotkey.CanonicalTrigger(run)
	if err != nil {
		return store.Settings{}, err
	}
	cp, err := hotkey.CanonicalTrigger(pause)
	if err != nil {
		return store.Settings{}, err
	}
	if cr == cp {
		return store.Settings{}, errors.New("hotkeys: Run and Pause must differ")
	}
	log.Printf("hotkey: rebind requested run=%s pause=%s", cr, cp)

	s.mu.Lock()
	s.settings.Hotkeys = store.Hotkeys{Run: cr, Pause: cp}
	st := s.settings
	old := s.hotkeys
	s.mu.Unlock()
	if err := store.SaveSettings(s.settingsDir, &st); err != nil {
		return st, err
	}

	// Release the old grabs/session BEFORE binding the new ones: on X11 a
	// GrabKey while the old connection still holds the key fails silently, and
	// on the portal the old connection still owns the app id. Restarting is
	// not needed; the new source replaces the old one live.
	_ = old.Close()
	hk := s.newHotkeys(s.onHotkey)
	s.mu.Lock()
	s.hotkeys = hk
	s.mu.Unlock()
	if hk.Name() == "unavailable" {
		return st, errors.New("hotkeys: no global shortcut source available")
	}
	if _, err := hk.Register(s.hotkeyBindings()); err != nil {
		return st, err
	}
	return st, nil
}

// CloseHotkeys releases the global shortcuts.
func (s *Service) CloseHotkeys() error { return s.hotkeys.Close() }

// HotkeySourceName reports the active mechanism for diagnostics.
func (s *Service) HotkeySourceName() string { return s.hotkeys.Name() }

// onHotkey routes an activation to the runner.
func (s *Service) onHotkey(id string) {
	log.Printf("hotkey: activated %q (engine %s)", id, s.engine.State())
	action, _, _ := strings.Cut(id, "-")
	if action == "" {
		action = id
	}
	switch action {
	case "run":
		if s.engine.State() == engine.StateIdle {
			s.mu.Lock()
			pid := s.selected
			s.mu.Unlock()
			if pid != "" {
				_ = s.Run(pid)
			}
		} else {
			_ = s.engine.Stop()
		}
	case "pause":
		switch s.engine.State() {
		case engine.StateRunning:
			_ = s.engine.Pause()
		case engine.StatePaused:
			_ = s.engine.Resume()
		}
	}
}

// --- settings / paths -----------------------------------------------------

// GetSettings returns the current settings.
func (s *Service) GetSettings() store.Settings { return s.settings }

// SaveSettings persists settings.
func (s *Service) SaveSettings(st store.Settings) error {
	s.mu.Lock()
	s.settings = st
	s.mu.Unlock()
	return store.SaveSettings(s.settingsDir, &st)
}

// AppDir returns the app folder.
func (s *Service) AppDir() string { return s.store.Root }

// ProfilesDir returns the resolved profiles folder.
func (s *Service) ProfilesDir() string { return s.store.Dir }

// OpenAppDir opens the app folder in the OS file manager.
func (s *Service) OpenAppDir() error { return store.OpenPath(s.store.Root) }

// Version returns the embedded app version.
func (s *Service) Version() string { return s.version }

// CheckUpdate reports whether a newer release is available.
func (s *Service) CheckUpdate() (UpdateInfo, error) {
	if s.checkUpdate == nil {
		return UpdateInfo{}, nil
	}
	return s.checkUpdate()
}

// InstallUpdate downloads and installs the pending update, then restarts.
func (s *Service) InstallUpdate() error {
	if s.installUpdate == nil {
		return errors.New("update: not available")
	}
	return s.installUpdate()
}

// --- helpers --------------------------------------------------------------

func (s *Service) selectProfile(id string) error {
	s.mu.Lock()
	s.selected = id
	s.settings.SelectedProfile = id
	st := s.settings
	s.mu.Unlock()
	return store.SaveSettings(s.settingsDir, &st)
}

func (s *Service) desktopSize() (int, int) {
	w, h, err := s.drv.ScreenSize()
	if err != nil {
		return 0, 0
	}
	return w, h
}

func firstBlocking(issues []model.Issue) string {
	for _, i := range issues {
		if i.Level == model.Blocking {
			return i.Message
		}
	}
	return "profile is not runnable"
}
