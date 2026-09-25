// Package store owns profile and settings files: path resolution, atomic
// writes, listing (including unreadable "problem" files) and CRUD.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"auto-clicker/internal/model"
)

// AppSlug is the folder name used under Documents / config / state.
const AppSlug = "auto-clicker"

// ErrNotFound is returned when a profile id has no file.
var ErrNotFound = errors.New("store: profile not found")

// ErrNewerSchema is returned when a file was written by a newer build.
var ErrNewerSchema = errors.New("store: profile schema is newer than this app")

// Summary describes a profile without loading all of it.
type Summary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Modules int    `json:"modules"`
	Enabled int    `json:"enabled"`
}

// ProblemFile is a .json file that could not be parsed.
type ProblemFile struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// Store reads and writes profiles in Dir.
type Store struct {
	Dir string
}

// ConfigDir returns the machine-specific config directory.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppSlug), nil
}

// StateDir returns the state directory used for logs.
func StateDir() (string, error) {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, AppSlug, "logs"), nil
		}
	}
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, AppSlug), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", AppSlug), nil
}

// DocumentsDir resolves the user's Documents folder without hardcoding it.
func DocumentsDir() (string, error) {
	if runtime.GOOS == "linux" {
		if d := os.Getenv("XDG_DOCUMENTS_DIR"); d != "" {
			return d, nil
		}
		if out, err := exec.Command("xdg-user-dir", "DOCUMENTS").Output(); err == nil {
			if p := strings.TrimSpace(string(out)); p != "" {
				return p, nil
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Documents"), nil
}

// DefaultProfilesDir is <Documents>/auto-clicker.
func DefaultProfilesDir() (string, error) {
	docs, err := DocumentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(docs, AppSlug), nil
}

// New opens a store rooted at dir, creating it if needed.
func New(dir string) (*Store, error) {
	if dir == "" {
		d, err := DefaultProfilesDir()
		if err != nil {
			return nil, err
		}
		dir = d
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

func (s *Store) path(id string) string { return filepath.Join(s.Dir, id+".json") }

func (s *Store) exists(id string) bool {
	_, err := os.Stat(s.path(id))
	return err == nil
}

// List returns every readable profile and every unparseable file.
func (s *Store) List() ([]Summary, []ProblemFile, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, nil, err
	}
	var out []Summary
	var problems []ProblemFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		p, err := s.Load(id)
		if err != nil {
			problems = append(problems, ProblemFile{ID: id, Error: err.Error()})
			continue
		}
		out = append(out, Summary{ID: id, Name: p.Name, Modules: len(p.Modules), Enabled: p.EnabledCount()})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	sort.Slice(problems, func(i, j int) bool { return problems[i].ID < problems[j].ID })
	return out, problems, nil
}

// Load reads and validates the schema version of a profile.
func (s *Store) Load(id string) (*model.Profile, error) {
	b, err := os.ReadFile(s.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var p model.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	if p.SchemaVersion > model.SchemaVersion {
		return nil, fmt.Errorf("%w (file %d, app %d)", ErrNewerSchema, p.SchemaVersion, model.SchemaVersion)
	}
	return &p, nil
}

// Save atomically writes a profile under an existing id.
func (s *Store) Save(id string, p *model.Profile) error {
	if id == "" {
		return errors.New("store: empty profile id")
	}
	if p.SchemaVersion == 0 {
		p.SchemaVersion = model.SchemaVersion
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(s.path(id), b)
}

// Create writes a new profile and returns its id.
func (s *Store) Create(name string, display model.Display, defaultMaxRuntimeSec int) (string, error) {
	id := model.UniqueSlug(model.Slugify(name), s.exists)
	p := model.NewProfile(name, display, defaultMaxRuntimeSec)
	if err := s.Save(id, p); err != nil {
		return "", err
	}
	return id, nil
}

// Rename gives a profile a new name and filename, returning the new id.
func (s *Store) Rename(id, newName string) (string, error) {
	p, err := s.Load(id)
	if err != nil {
		return "", err
	}
	p.Name = newName
	newID := model.UniqueSlug(model.Slugify(newName), s.exists)
	if newID == id {
		return id, s.Save(id, p)
	}
	if err := s.Save(newID, p); err != nil {
		return "", err
	}
	if err := os.Remove(s.path(id)); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return newID, nil
}

// Duplicate copies a profile under a "copy" name.
func (s *Store) Duplicate(id string) (string, error) {
	p, err := s.Load(id)
	if err != nil {
		return "", err
	}
	// taken matches either an existing name or a slug collision.
	takenName := func(name string) bool {
		ids, _, _ := s.List()
		for _, sum := range ids {
			if strings.EqualFold(sum.Name, name) {
				return true
			}
		}
		return false
	}
	p.Name = model.DuplicateName(p.Name, takenName)
	newID := model.UniqueSlug(model.Slugify(p.Name), s.exists)
	p.SchemaVersion = model.SchemaVersion
	if err := s.Save(newID, p); err != nil {
		return "", err
	}
	return newID, nil
}

// Delete removes a profile file.
func (s *Store) Delete(id string) error {
	err := os.Remove(s.path(id))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

// UpdateDisplaySnapshot refreshes a profile's display geometry without touching
// anything else (DESIGN §3.5).
func (s *Store) UpdateDisplaySnapshot(id string, display model.Display) error {
	p, err := s.Load(id)
	if err != nil {
		return err
	}
	p.Display = display
	return s.Save(id, p)
}

// OpenPath opens a file or folder in the OS file manager. Non-blocking.
func OpenPath(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// WriteFileAtomic writes data to a temp file in the same directory, fsyncs, and
// renames over the target.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
