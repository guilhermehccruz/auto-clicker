package model

import (
	"crypto/rand"
	"encoding/hex"
)

// Mouse button names. ButtonNone is "move only": move to the target without a
// button event (absolute targets only; meaningless for cursor targets).
const (
	ButtonLeft   = "left"
	ButtonRight  = "right"
	ButtonMiddle = "middle"
	ButtonNone   = "none"
)

// Defaults (see DESIGN §3.3).
const (
	DefaultHoldMs        = 10
	DefaultDelayAfter    = 250
	DefaultMaxRuntimeSec = 3600
	DefaultLoopDelayMin  = 250
	DefaultLoopDelayMax  = 250
)

// newHex returns 8 lowercase hex chars.
func newHex() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is catastrophic; fall back to a fixed-shape id.
		return "00000000"
	}
	return hex.EncodeToString(b[:])
}

// NewID returns a fresh module id: "m_" + 8 lowercase hex chars.
func NewID() string { return "m_" + newHex() }

// DefaultLoopDelay returns the fixed 250 ms default.
func DefaultLoopDelay() DelayRange {
	return DelayRange{Min: DefaultLoopDelayMin, Max: DefaultLoopDelayMax}
}

// DefaultLoop returns a safe default loop (one pass).
func DefaultLoop() Loop { return Loop{Mode: LoopOnce, Count: 1, LoopDelay: DefaultLoopDelay()} }

// DefaultFailsafe returns the default safety settings.
func DefaultFailsafe() Failsafe { return Failsafe{MaxRuntimeSec: DefaultMaxRuntimeSec} }

// NewGroup returns a freshly added, enabled group with no modules.
func NewGroup(name string) Group {
	if name == "" {
		name = "New group"
	}
	return Group{ID: "g_" + newHex(), Name: name, Enabled: true, Modules: []Module{}}
}

// NewModule returns a freshly added module: cursor-targeted, left button, one
// click, 250 ms after — safe to run without configuration.
func NewModule() Module {
	return Module{
		Kind:       KindClick,
		ID:         NewID(),
		Enabled:    true,
		Target:     Target{Kind: TargetCursor},
		Button:     ButtonLeft,
		Count:      1,
		HoldMs:     DefaultHoldMs,
		DelayAfter: DefaultDelayAfter,
	}
}

// NewProfile builds the first-run / new-profile template: no modules, one pass,
// and the given display snapshot.
func NewProfile(name string, display Display, defaultMaxRuntimeSec int) *Profile {
	if name == "" {
		name = "New profile"
	}
	if defaultMaxRuntimeSec < 0 {
		defaultMaxRuntimeSec = 0
	}
	return &Profile{
		SchemaVersion: SchemaVersion,
		Name:          name,
		Display:       display,
		Loop:          DefaultLoop(),
		Failsafe:      Failsafe{MaxRuntimeSec: defaultMaxRuntimeSec},
		Modules:       []Module{},
	}
}

// GroupEnabled reports whether a group id is enabled. A missing group is
// treated as enabled (forward-compatible with dangling references).
func (p *Profile) GroupEnabled(id string) bool {
	for _, g := range p.Groups {
		if g.ID == id {
			return g.Enabled
		}
	}
	return true
}

// EnabledModules returns the modules that participate in a run, in execution
// order: ungrouped modules first, then each enabled group in order. A module in
// a disabled group is skipped regardless of its own flag.
func (p *Profile) EnabledModules() []Module {
	out := make([]Module, 0, len(p.Modules))
	for _, m := range p.Modules {
		if m.Enabled {
			out = append(out, m)
		}
	}
	for _, g := range p.Groups {
		if !g.Enabled {
			continue
		}
		for _, m := range g.Modules {
			if m.Enabled {
				out = append(out, m)
			}
		}
	}
	return out
}

// EnabledCount returns how many modules are effectively enabled (module flag
// and, for grouped modules, the group flag).
func (p *Profile) EnabledCount() int {
	return len(p.EnabledModules())
}

// TotalModules counts modules across ungrouped and grouped lists.
func (p *Profile) TotalModules() int {
	n := len(p.Modules)
	for _, g := range p.Groups {
		n += len(g.Modules)
	}
	return n
}
