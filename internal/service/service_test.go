package service

import (
	"context"
	"testing"
	"time"

	"auto-clicker/internal/engine"
	"auto-clicker/internal/hotkey"
	"auto-clicker/internal/input"
	"auto-clicker/internal/model"
	"auto-clicker/internal/store"
)

type instClock struct{}

func (instClock) Now() time.Time { return time.Unix(0, 0) }
func (instClock) Sleep(ctx context.Context, d time.Duration) error {
	return ctx.Err()
}

func newTestService(t *testing.T, drv *input.FakeDriver) *Service {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Options{
		ConfigDir: t.TempDir(),
		Store:     st,
		Driver:    drv,
		Hotkeys:   hotkey.Unavailable{Reason: "test"},
		Version:   "test",
		Clock:     instClock{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func waitState(t *testing.T, s *Service, want engine.State) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s.Status().State == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("state never became %s (got %s)", want, s.Status().State)
}

func TestCreateSaveAndRun(t *testing.T) {
	drv := input.NewFake()
	s := newTestService(t, drv)

	id, err := s.CreateProfile("Farm")
	if err != nil {
		t.Fatal(err)
	}
	if id != "farm" {
		t.Fatalf("id = %q", id)
	}
	sums, problems, err := s.ListProfiles()
	if err != nil || len(sums) != 1 || len(problems) != 0 {
		t.Fatalf("list = %d/%d err=%v", len(sums), len(problems), err)
	}

	// A fresh profile has no modules -> not runnable.
	p, _ := s.LoadProfile(id)
	if issues := s.Validate(p); !model.HasBlocking(issues) {
		t.Fatal("empty profile should not be runnable")
	}
	if err := s.Run(id); err == nil {
		t.Fatal("Run on an empty profile should fail")
	}

	// Add a module and run.
	m := model.NewModule()
	m.Target = model.NewAbsoluteTarget(940, 520)
	p.Modules = []model.Module{m}
	if err := s.SaveProfile(id, p); err != nil {
		t.Fatal(err)
	}
	if issues := s.Validate(p); model.HasBlocking(issues) {
		t.Fatalf("should be runnable: %+v", issues)
	}
	if err := s.Run(id); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, engine.StateIdle)
	if got := len(drv.Clicks()); got != 1 {
		t.Errorf("clicks = %d, want 1", got)
	}
}

func TestCreateProfileSnapshotAndDiagnostics(t *testing.T) {
	drv := input.NewFake()
	s := newTestService(t, drv)
	id, _ := s.CreateProfile("p")
	p, _ := s.LoadProfile(id)
	if p.Display.Width != 1920 || p.Display.Height != 1080 {
		t.Errorf("display snapshot = %+v", p.Display)
	}
	d := s.Diagnostics()
	if d.Backend != "fake" || d.AppID != AppID || d.ProfilesDir == "" {
		t.Errorf("diagnostics = %+v", d)
	}
	if s.Version() != "test" {
		t.Errorf("version = %q", s.Version())
	}
}

func TestRenamePersistsSelection(t *testing.T) {
	s := newTestService(t, input.NewFake())
	id, _ := s.CreateProfile("Farm")
	newID, err := s.RenameProfile(id, "Idle Tycoon")
	if err != nil {
		t.Fatal(err)
	}
	if newID != "idle-tycoon" {
		t.Fatalf("newID = %q", newID)
	}
	if got := s.GetSettings().SelectedProfile; got != "idle-tycoon" {
		t.Errorf("selected = %q", got)
	}
}
