package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"auto-clicker/internal/model"
)

// Window is the persisted main-window geometry.
type Window struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	W         int  `json:"width"`
	H         int  `json:"height"`
	Maximized bool `json:"maximized"`
}

// Hotkeys holds the desired global bindings (portal uppercase-modifier form).
type Hotkeys struct {
	Run   string `json:"run"`
	Pause string `json:"pause"`
}

// Settings is the machine-specific app configuration (DESIGN §3.10).
type Settings struct {
	SchemaVersion        int     `json:"schemaVersion"`
	SelectedProfile      string  `json:"selectedProfile"`
	ProfilesDir          string  `json:"profilesDir"`
	Window               Window  `json:"window"`
	Hotkeys              Hotkeys `json:"hotkeys"`
	DefaultMaxRuntimeSec int     `json:"defaultMaxRuntimeSec"`
	Theme                string  `json:"theme"`

	extras map[string]json.RawMessage
}

// DefaultSettings returns the documented defaults.
func DefaultSettings() Settings {
	return Settings{
		SchemaVersion:        model.SchemaVersion,
		Window:               Window{W: 1100, H: 720},
		Hotkeys:              Hotkeys{Run: "F8", Pause: "F9"},
		DefaultMaxRuntimeSec: model.DefaultMaxRuntimeSec,
		Theme:                "dark",
	}
}

// SettingsFile is the settings filename. It lives in the profiles directory so
// a profile folder is self-contained (DESIGN §3.8).
const SettingsFile = "settings.json"

// SettingsPath returns the settings file path inside dir (the profiles dir).
func SettingsPath(dir string) string { return filepath.Join(dir, SettingsFile) }

// LoadSettings reads settings from dir, creating defaults on first run. Legacy
// locations are checked in order when dir has no settings yet, and a settings
// file found there is migrated into dir.
func LoadSettings(dir string, legacyDirs ...string) (Settings, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Settings{}, err
	}
	b, err := os.ReadFile(SettingsPath(dir))
	if os.IsNotExist(err) {
		for _, legacy := range legacyDirs {
			if legacy == "" || legacy == dir {
				continue
			}
			lb, lerr := os.ReadFile(SettingsPath(legacy))
			if lerr != nil {
				continue
			}
			var st Settings
			if json.Unmarshal(lb, &st) != nil {
				continue
			}
			return st, SaveSettings(dir, &st)
		}
		st := DefaultSettings()
		return st, SaveSettings(dir, &st)
	}
	if err != nil {
		return Settings{}, err
	}
	st := DefaultSettings()
	if err := json.Unmarshal(b, &st); err != nil {
		return Settings{}, err
	}
	return st, nil
}

// SaveSettings atomically writes settings into dir.
func SaveSettings(dir string, st *Settings) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(SettingsPath(dir), b)
}

func knownJSONKeys(v any) map[string]struct{} {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	m := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if name := strings.Split(tag, ",")[0]; name != "" {
			m[name] = struct{}{}
		}
	}
	return m
}

func decodeWithExtras(b []byte, into any) (map[string]json.RawMessage, error) {
	if err := json.Unmarshal(b, into); err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k := range knownJSONKeys(into) {
		delete(m, k)
	}
	if len(m) == 0 {
		return nil, nil
	}
	return m, nil
}

// UnmarshalJSON preserves unknown keys and applies defaults for absent ones.
func (s *Settings) UnmarshalJSON(b []byte) error {
	type aux Settings
	var a aux
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	ex, err := decodeWithExtras(b, &a)
	if err != nil {
		return err
	}
	*s = Settings(a)
	s.extras = ex

	def := DefaultSettings()
	if !hasKey(raw, "schemaVersion") || s.SchemaVersion == 0 {
		s.SchemaVersion = model.SchemaVersion
	}
	if !hasKey(raw, "window") {
		s.Window = def.Window
	}
	if !hasKey(raw, "hotkeys") {
		s.Hotkeys = def.Hotkeys
	}
	if !hasKey(raw, "defaultMaxRuntimeSec") {
		s.DefaultMaxRuntimeSec = def.DefaultMaxRuntimeSec
	}
	if !hasKey(raw, "theme") || s.Theme == "" {
		s.Theme = def.Theme
	}
	return nil
}

// MarshalJSON re-merges unknown keys (known keys win).
func (s Settings) MarshalJSON() ([]byte, error) {
	type aux Settings
	b, err := json.Marshal(aux(s))
	if err != nil {
		return nil, err
	}
	if len(s.extras) == 0 {
		return b, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k, v := range s.extras {
		if _, ok := m[k]; !ok {
			m[k] = v
		}
	}
	return json.Marshal(m)
}

func hasKey(raw map[string]json.RawMessage, key string) bool {
	_, ok := raw[key]
	return ok
}
