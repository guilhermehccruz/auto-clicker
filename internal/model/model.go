// Package model holds the profile data model, its JSON representation
// (including unknown-field preservation) and validation. It is deliberately
// dependency-free so both the store and the engine can share it, and so the UI
// binding and the runner validate identically.
package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// SchemaVersion is the version this build writes. Loading a newer file is
// refused; loading an older one runs the migration chain (empty today).
const SchemaVersion = 1

// Target kinds.
const (
	TargetAbsolute = "absolute"
	TargetCursor   = "cursor"
)

// Loop modes.
const (
	LoopOnce    = "once"
	LoopCount   = "count"
	LoopForever = "forever"
)

// Module kinds. Absent means click.
const (
	KindClick = "click"
)

// Field ranges (see DESIGN §3.3 / §5.6).
const (
	MinCount         = 1
	MaxCount         = 1000
	MinClickInterval = 0
	MaxClickInterval = 60000
	MinHoldMs        = 0
	MaxHoldMs        = 1000
	MinDelayAfter    = 0
	MaxDelayAfter    = 3600000
	MaxNameLen       = 80
)

// Profile is one .json file; the filename (without extension) is the profile id.
type Profile struct {
	SchemaVersion int      `json:"schemaVersion"`
	Name          string   `json:"name"`
	Display       Display  `json:"display"`
	Loop          Loop     `json:"loop"`
	Failsafe      Failsafe `json:"failsafe"`
	Modules       []Module `json:"modules"`
	Groups        []Group  `json:"groups"`

	extras map[string]json.RawMessage
}

// Display is a snapshot of the desktop geometry the coordinates were captured
// against.
type Display struct {
	Width  int `json:"width"`
	Height int `json:"height"`

	extras map[string]json.RawMessage
}

// Loop describes how many passes a run makes.
type Loop struct {
	Mode      string     `json:"mode"`
	Count     int        `json:"count"`
	LoopDelay DelayRange `json:"loopDelay"`

	extras map[string]json.RawMessage
}

// DelayRange is a uniform inclusive millisecond window; min == max is a fixed
// delay.
type DelayRange struct {
	Min int `json:"min"`
	Max int `json:"max"`

	extras map[string]json.RawMessage
}

// Failsafe carries the per-profile safety settings.
type Failsafe struct {
	MaxRuntimeSec      int  `json:"maxRuntimeSec"`
	MouseToCornerStops bool `json:"mouseToCornerStops"`

	extras map[string]json.RawMessage
}

// Group is an ordered, toggleable container of modules. Ungrouped modules live
// in Profile.Modules; group modules live in Group.Modules. Execution runs the
// ungrouped modules first, then each enabled group in order.
type Group struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Enabled bool     `json:"enabled"`
	Modules []Module `json:"modules"`

	extras map[string]json.RawMessage
}

// Module is a single click step.
type Module struct {
	Kind          string `json:"kind"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Target        Target `json:"target"`
	Button        string `json:"button"`
	Count         int    `json:"count"`
	ClickInterval int    `json:"clickInterval"`
	HoldMs        int    `json:"holdMs"`
	DelayAfter    int    `json:"delayAfter"`

	extras map[string]json.RawMessage
}

// Target is either an absolute point or the current cursor position.
type Target struct {
	Kind string `json:"kind"`
	X    int    `json:"x,omitempty"`
	Y    int    `json:"y,omitempty"`

	extras map[string]json.RawMessage
	// coordsSet records whether x/y were present in the JSON (or set via
	// SetAbsolute), so validation can distinguish a missing coordinate from a
	// legitimate (0, 0).
	coordsSet bool
}

// NewAbsoluteTarget returns an absolute target with its coordinates marked as
// present.
func NewAbsoluteTarget(x, y int) Target {
	return Target{Kind: TargetAbsolute, X: x, Y: y, coordsSet: true}
}

// SetAbsolute marks t as an absolute target at (x, y).
func (t *Target) SetAbsolute(x, y int) {
	t.Kind = TargetAbsolute
	t.X, t.Y = x, y
	t.coordsSet = true
}

// HasCoords reports whether an absolute target carries coordinates.
func (t Target) HasCoords() bool { return t.coordsSet }

// --- unknown-field preservation -------------------------------------------

func knownKeys(v any) map[string]struct{} {
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
		name := strings.Split(tag, ",")[0]
		if name != "" {
			m[name] = struct{}{}
		}
	}
	return m
}

// decodePreserve decodes b into into (an aux struct with no custom
// UnmarshalJSON) and returns the raw unknown keys.
func decodePreserve(b []byte, into any) (map[string]json.RawMessage, error) {
	if err := json.Unmarshal(b, into); err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k := range knownKeys(into) {
		delete(m, k)
	}
	if len(m) == 0 {
		return nil, nil
	}
	return m, nil
}

// marshalPreserve marshals v (an aux struct) and re-merges extras, with known
// keys winning on collision.
func marshalPreserve(v any, extras map[string]json.RawMessage) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(extras) == 0 {
		return b, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k, val := range extras {
		if _, ok := m[k]; !ok {
			m[k] = val
		}
	}
	return json.Marshal(m)
}

// has reports whether the raw object contains key.
func has(raw map[string]json.RawMessage, key string) bool {
	_, ok := raw[key]
	return ok
}

// UnmarshalJSON implements json.Unmarshaler with unknown-field preservation and
// default application for absent fields.
func (p *Profile) UnmarshalJSON(b []byte) error {
	type aux Profile
	var a aux
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	ex, err := decodePreserve(b, &a)
	if err != nil {
		return err
	}
	*p = Profile(a)
	p.extras = ex
	if !has(raw, "loop") {
		p.Loop = DefaultLoop()
	}
	if !has(raw, "failsafe") {
		p.Failsafe = DefaultFailsafe()
	}
	if p.SchemaVersion == 0 {
		p.SchemaVersion = SchemaVersion
	}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (p Profile) MarshalJSON() ([]byte, error) {
	type aux Profile
	return marshalPreserve(aux(p), p.extras)
}

func (d *Display) UnmarshalJSON(b []byte) error {
	type aux Display
	var a aux
	ex, err := decodePreserve(b, &a)
	if err != nil {
		return err
	}
	*d = Display(a)
	d.extras = ex
	return nil
}

func (d Display) MarshalJSON() ([]byte, error) {
	type aux Display
	return marshalPreserve(aux(d), d.extras)
}

func (l *Loop) UnmarshalJSON(b []byte) error {
	type aux Loop
	var a aux
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	ex, err := decodePreserve(b, &a)
	if err != nil {
		return err
	}
	*l = Loop(a)
	l.extras = ex
	if !has(raw, "mode") || l.Mode == "" {
		l.Mode = LoopOnce
	}
	if !has(raw, "count") {
		l.Count = 1
	}
	if !has(raw, "loopDelay") {
		l.LoopDelay = DefaultLoopDelay()
	}
	return nil
}

func (l Loop) MarshalJSON() ([]byte, error) {
	type aux Loop
	return marshalPreserve(aux(l), l.extras)
}

func (d *DelayRange) UnmarshalJSON(b []byte) error {
	type aux DelayRange
	var a aux
	ex, err := decodePreserve(b, &a)
	if err != nil {
		return err
	}
	*d = DelayRange(a)
	d.extras = ex
	return nil
}

func (d DelayRange) MarshalJSON() ([]byte, error) {
	type aux DelayRange
	return marshalPreserve(aux(d), d.extras)
}

func (f *Failsafe) UnmarshalJSON(b []byte) error {
	type aux Failsafe
	var a aux
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	ex, err := decodePreserve(b, &a)
	if err != nil {
		return err
	}
	*f = Failsafe(a)
	f.extras = ex
	if !has(raw, "maxRuntimeSec") {
		f.MaxRuntimeSec = DefaultMaxRuntimeSec
	}
	return nil
}

func (f Failsafe) MarshalJSON() ([]byte, error) {
	type aux Failsafe
	return marshalPreserve(aux(f), f.extras)
}

func (g *Group) UnmarshalJSON(b []byte) error {
	type aux Group
	var a aux
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	ex, err := decodePreserve(b, &a)
	if err != nil {
		return err
	}
	*g = Group(a)
	g.extras = ex
	if !has(raw, "enabled") {
		g.Enabled = true
	}
	return nil
}

func (g Group) MarshalJSON() ([]byte, error) {
	type aux Group
	return marshalPreserve(aux(g), g.extras)
}

func (m *Module) UnmarshalJSON(b []byte) error {
	type aux Module
	var a aux
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	ex, err := decodePreserve(b, &a)
	if err != nil {
		return err
	}
	*m = Module(a)
	m.extras = ex

	if m.Kind == "" {
		m.Kind = KindClick
	}
	if !has(raw, "enabled") {
		m.Enabled = true
	}
	if !has(raw, "button") || m.Button == "" {
		m.Button = ButtonLeft
	}
	if !has(raw, "target") {
		m.Target = Target{Kind: TargetCursor}
	}
	if m.Target.Kind == "" {
		m.Target.Kind = TargetCursor
	}
	if !has(raw, "count") {
		m.Count = 1
	}
	if !has(raw, "holdMs") {
		m.HoldMs = DefaultHoldMs
	}
	if !has(raw, "delayAfter") {
		m.DelayAfter = DefaultDelayAfter
	}
	return nil
}

func (m Module) MarshalJSON() ([]byte, error) {
	type aux Module
	return marshalPreserve(aux(m), m.extras)
}

func (t *Target) UnmarshalJSON(b []byte) error {
	type aux Target
	var a aux
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	ex, err := decodePreserve(b, &a)
	if err != nil {
		return err
	}
	*t = Target(a)
	t.extras = ex
	if t.Kind == "" {
		t.Kind = TargetCursor
	}
	t.coordsSet = has(raw, "x") || has(raw, "y")
	return nil
}

func (t Target) MarshalJSON() ([]byte, error) {
	m := map[string]json.RawMessage{}
	kind, _ := json.Marshal(t.Kind)
	m["kind"] = kind
	if t.Kind == TargetAbsolute {
		x, _ := json.Marshal(t.X)
		y, _ := json.Marshal(t.Y)
		m["x"], m["y"] = x, y
	}
	for k, v := range t.extras {
		if _, ok := m[k]; !ok {
			m[k] = v
		}
	}
	return json.Marshal(m)
}

// String is a human summary used in errors and logs.
func (t Target) String() string {
	if t.Kind == TargetAbsolute {
		return fmt.Sprintf("(%d, %d)", t.X, t.Y)
	}
	return "at cursor"
}
