package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"auto-clicker/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCRUDAndSlugCollision(t *testing.T) {
	s := newTestStore(t)

	id1, err := s.Create("Farm", model.Display{Width: 1920, Height: 1080}, 3600)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != "farm" {
		t.Fatalf("id = %q, want farm", id1)
	}
	id2, err := s.Create("Farm", model.Display{}, 3600)
	if err != nil {
		t.Fatal(err)
	}
	if id2 != "farm-2" {
		t.Fatalf("second id = %q, want farm-2", id2)
	}

	p, err := s.Load(id1)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Farm" || p.SchemaVersion != model.SchemaVersion {
		t.Fatalf("loaded profile wrong: %+v", p)
	}

	newID, err := s.Rename(id1, "Idle Tycoon")
	if err != nil {
		t.Fatal(err)
	}
	if newID != "idle-tycoon" {
		t.Fatalf("renamed id = %q", newID)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "farm.json")); !os.IsNotExist(err) {
		t.Error("old file should be gone after rename")
	}

	dupID, err := s.Duplicate(newID)
	if err != nil {
		t.Fatal(err)
	}
	dup, err := s.Load(dupID)
	if err != nil {
		t.Fatal(err)
	}
	if dup.Name != "Idle Tycoon copy" {
		t.Fatalf("duplicate name = %q", dup.Name)
	}

	sums, problems, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("unexpected problem files: %+v", problems)
	}
	if len(sums) != 3 {
		t.Errorf("summaries = %d, want 3", len(sums))
	}

	if err := s.Delete(dupID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(dupID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete err = %v, want ErrNotFound", err)
	}
}

func TestProblemFileSurfaced(t *testing.T) {
	s := newTestStore(t)
	if err := os.WriteFile(filepath.Join(s.Dir, "broken.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	sums, problems, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 0 || len(problems) != 1 {
		t.Fatalf("want 1 problem, 0 profiles; got %d/%d", len(sums), len(problems))
	}
	if problems[0].ID != "broken" {
		t.Errorf("problem id = %q", problems[0].ID)
	}
	// The broken file must not have been rewritten.
	b, _ := os.ReadFile(filepath.Join(s.Dir, "broken.json"))
	if string(b) != "{not json" {
		t.Error("unreadable file was modified")
	}
}

func TestNewerSchemaRefused(t *testing.T) {
	s := newTestStore(t)
	body := `{"schemaVersion": 99, "name": "future", "modules": []}`
	if err := os.WriteFile(filepath.Join(s.Dir, "future.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := s.Load("future")
	if !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("err = %v, want ErrNewerSchema", err)
	}
}

func TestUpdateDisplaySnapshot(t *testing.T) {
	s := newTestStore(t)
	id, _ := s.Create("p", model.Display{Width: 1, Height: 1}, 3600)
	if err := s.UpdateDisplaySnapshot(id, model.Display{Width: 5360, Height: 1440}); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Load(id)
	if p.Display.Width != 5360 || p.Display.Height != 1440 {
		t.Errorf("display not updated: %+v", p.Display)
	}
}

func TestSettingsRoundTripAndDefaults(t *testing.T) {
	dir := t.TempDir()
	st, err := LoadSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Hotkeys.Run != "F8" || st.DefaultMaxRuntimeSec != 3600 || st.Theme != "dark" {
		t.Fatalf("defaults wrong: %+v", st)
	}
	st.SelectedProfile = "farm"
	st.ProfilesDir = "/tmp/x"
	if err := SaveSettings(dir, &st); err != nil {
		t.Fatal(err)
	}
	back, err := LoadSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if back.SelectedProfile != "farm" || back.ProfilesDir != "/tmp/x" {
		t.Fatalf("round trip lost data: %+v", back)
	}

	// Unknown keys preserved, absent keys defaulted.
	raw := `{"selectedProfile":"a","futureSetting":true}`
	var s2 Settings
	if err := json.Unmarshal([]byte(raw), &s2); err != nil {
		t.Fatal(err)
	}
	if s2.Hotkeys.Run != "F8" {
		t.Errorf("absent hotkeys not defaulted: %+v", s2.Hotkeys)
	}
	out, _ := json.Marshal(&s2)
	if !strings.Contains(string(out), "futureSetting") {
		t.Errorf("unknown settings key lost: %s", out)
	}
}

func TestSingleInstanceLock(t *testing.T) {
	dir := t.TempDir()
	l1, err := AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLock(dir); !errors.Is(err, ErrLocked) {
		t.Fatalf("second lock err = %v, want ErrLocked", err)
	}
	if err := l1.Release(); err != nil {
		t.Fatal(err)
	}
	l2, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("reacquire failed: %v", err)
	}
	_ = l2.Release()
}
