// Package cursor wraps the driver's cursor query with a one-time capability
// probe, so the UI can gate the picker / "use current cursor" / corner failsafe
// on whether the read actually works.
package cursor

import "auto-clicker/internal/input"

// Prober answers whether the real cursor can be read, and reads it.
type Prober struct {
	drv     input.Driver
	checked bool
	ok      bool
}

// New returns a Prober over drv.
func New(drv input.Driver) *Prober { return &Prober{drv: drv} }

// Available probes once and caches the result. On libei without an x11 query
// path this is false, and callers must degrade rather than guess.
func (p *Prober) Available() bool {
	if p == nil || p.drv == nil {
		return false
	}
	if !p.checked {
		_, _, ok := p.drv.CursorPos()
		p.ok = ok
		p.checked = true
	}
	return p.ok
}

// Pos returns the real cursor position when available.
func (p *Prober) Pos() (x, y int, ok bool) {
	if !p.Available() {
		return 0, 0, false
	}
	return p.drv.CursorPos()
}
