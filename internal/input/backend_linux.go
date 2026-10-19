//go:build linux

package input

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/go-vgo/robotgo/libei"
	"github.com/go-vgo/robotgo/x11"

	"auto-clicker/internal/model"
)

type linuxBackend int

const (
	backendX11 linuxBackend = iota
	backendLibei
)

// linuxDriver injects through x11 or libei and answers cursor/geometry queries
// through the x11 path when XWayland is available (libei cannot read the real
// cursor and only reports geometry with a linked ScreenCast stream).
type linuxDriver struct {
	be       linuxBackend
	fallback string
	// queryOK is true when the x11 query path works (real cursor + displays).
	queryOK bool
	// libeiAbsolute is true when libei has a linked ScreenCast stream.
	libeiAbsolute bool

	// primary monitor origin in root (bounding-box) coordinates, cached.
	ox, oy    int
	originSet bool
	// heldKeys holds combos pressed with KeyDown and not released yet.
	heldKeys []string
}

// primaryOrigin returns the primary monitor's top-left in X11 root coordinates.
// Coordinates are exposed primary-relative so profiles are portable across OSes
// (Windows uses the primary monitor's top-left as (0,0)).
func (d *linuxDriver) primaryOrigin() (int, int) {
	if !d.queryOK {
		return 0, 0
	}
	if !d.originSet {
		r := x11.GetScreenRect(x11.MainDisplayID())
		d.ox, d.oy = r.X, r.Y
		d.originSet = true
	}
	return d.ox, d.oy
}

// New picks the backend per DESIGN §2.2 / §4.5: X11 sessions use x11; a Wayland
// session starts on x11 (no dialog) and upgrades to libei automatically only
// when a cached portal token exists.
func New() (Driver, error) {
	d := &linuxDriver{be: backendX11}
	d.queryOK = x11Usable()

	if os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		if hasPortalToken() {
			if err := d.enableLibei(); err != nil {
				d.fallback = err.Error()
			}
		} else {
			d.fallback = "native Wayland input not enabled yet"
		}
	}
	if d.be == backendX11 && !d.queryOK {
		d.fallback = "x11 unavailable (no DISPLAY / XWayland)"
	}
	return d, nil
}

// NewLibei explicitly upgrades to libei (the UI's "Enable native Wayland input"
// action). It may show the portal consent dialog.
func NewLibei() (Driver, error) {
	d := &linuxDriver{be: backendX11}
	d.queryOK = x11Usable()
	if err := d.enableLibei(); err != nil {
		return d, err
	}
	return d, nil
}

func (d *linuxDriver) enableLibei() error {
	libei.LinkScreenCast = true
	// Touching the backend establishes the portal session (possibly prompting).
	libei.GetScreenSize()
	if libei.DisplaysNum() == 0 {
		// No ScreenCast stream: cursor/relative input may still work, but
		// absolute targets are unreliable.
		d.be = backendLibei
		d.libeiAbsolute = false
		return errors.New("libei active without a ScreenCast stream: absolute targets unavailable")
	}
	d.be = backendLibei
	d.libeiAbsolute = true
	return nil
}

func x11Usable() bool {
	if os.Getenv("DISPLAY") == "" {
		return false
	}
	w, h := x11.GetScreenSize()
	return w > 0 && h > 0
}

func hasPortalToken() bool {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		dir = filepath.Join(home, ".local", "state")
	}
	b, err := os.ReadFile(filepath.Join(dir, "robotgo", "portal_token"))
	return err == nil && len(b) > 0
}

func (d *linuxDriver) ClickAt(x, y int, b Button, hold time.Duration) error {
	if err := d.Move(x, y); err != nil {
		return err
	}
	if err := d.ButtonDown(b); err != nil {
		return err
	}
	time.Sleep(hold)
	return d.ButtonUp(b)
}

func (d *linuxDriver) ClickHere(b Button, hold time.Duration) error {
	// No motion: a button event at the pointer's current position.
	if err := d.ButtonDown(b); err != nil {
		return err
	}
	time.Sleep(hold)
	return d.ButtonUp(b)
}

func (d *linuxDriver) Move(x, y int) error {
	// Canonical (primary-relative) → root coordinates.
	ox, oy := d.primaryOrigin()
	if d.be == backendLibei {
		libei.Move(x+ox, y+oy)
		return nil
	}
	x11.Move(x+ox, y+oy)
	return nil
}

func (d *linuxDriver) ButtonDown(b Button) error {
	if d.be == backendLibei {
		return libei.MouseDown(b.String())
	}
	return x11.MouseDown(b.String())
}

func (d *linuxDriver) ButtonUp(b Button) error {
	if d.be == backendLibei {
		return libei.MouseUp(b.String())
	}
	return x11.MouseUp(b.String())
}

func (d *linuxDriver) keyDownOne(k string) error {
	if d.be == backendLibei {
		return libei.KeyDown(k)
	}
	return x11.KeyDown(k)
}

func (d *linuxDriver) keyUpOne(k string) error {
	if d.be == backendLibei {
		return libei.KeyUp(k)
	}
	return x11.KeyUp(k)
}

func (d *linuxDriver) KeyTap(combo string, hold time.Duration) error {
	if err := d.KeyDown(combo); err != nil {
		return err
	}
	time.Sleep(hold)
	return d.KeyUp(combo)
}

func (d *linuxDriver) KeyDown(combo string) error {
	mods, key, err := model.ParseKeyCombo(combo)
	if err != nil {
		return err
	}
	var pressed []string
	release := func() {
		for i := len(pressed) - 1; i >= 0; i-- {
			_ = d.keyUpOne(pressed[i])
		}
	}
	for _, m := range mods {
		if err := d.keyDownOne(m); err != nil {
			release()
			return err
		}
		pressed = append(pressed, m)
	}
	if err := d.keyDownOne(key); err != nil {
		release()
		return err
	}
	d.heldKeys = append(d.heldKeys, combo)
	return nil
}

// releaseKeyCombo releases the key and its modifiers (reverse order).
func (d *linuxDriver) releaseKeyCombo(combo string) error {
	mods, key, err := model.ParseKeyCombo(combo)
	if err != nil {
		return err
	}
	var errs []error
	errs = append(errs, d.keyUpOne(key))
	for i := len(mods) - 1; i >= 0; i-- {
		errs = append(errs, d.keyUpOne(mods[i]))
	}
	return errors.Join(errs...)
}

func (d *linuxDriver) KeyUp(combo string) error {
	err := d.releaseKeyCombo(combo)
	for i, h := range d.heldKeys {
		if h == combo {
			d.heldKeys = append(d.heldKeys[:i], d.heldKeys[i+1:]...)
			break
		}
	}
	return err
}

func (d *linuxDriver) CursorPos() (int, int, bool) {
	if !d.queryOK {
		return 0, 0, false
	}
	// Root coordinates → canonical (primary-relative).
	x, y := x11.Location()
	ox, oy := d.primaryOrigin()
	return x - ox, y - oy, true
}

func (d *linuxDriver) ScreenSize() (int, int, error) {
	if d.queryOK {
		w, h := x11.GetScreenSize()
		return w, h, nil
	}
	w, h := libei.GetScreenSize()
	if w == 0 && h == 0 {
		return 0, 0, errors.New("no display geometry available")
	}
	return w, h, nil
}

func (d *linuxDriver) Displays() ([]Display, error) {
	if d.queryOK {
		n := x11.DisplaysNum()
		if n <= 0 {
			n = 1
		}
		ox, oy := d.primaryOrigin()
		main := x11.MainDisplayID()
		out := make([]Display, 0, n)
		for i := 0; i < n; i++ {
			r := x11.GetScreenRect(i)
			// Primary-relative (canonical) coordinates.
			out = append(out, Display{X: r.X - ox, Y: r.Y - oy, W: r.W, H: r.H, Scale: 1, Primary: i == main})
		}
		return out, nil
	}
	w, h := libei.GetScreenSize()
	if w == 0 && h == 0 {
		return nil, errors.New("no display geometry available")
	}
	return []Display{{W: w, H: h, Scale: 1, Primary: true}}, nil
}

func (d *linuxDriver) ReleaseAll() error {
	var first error
	held := append([]string(nil), d.heldKeys...)
	d.heldKeys = nil
	for i := len(held) - 1; i >= 0; i-- {
		if err := d.releaseKeyCombo(held[i]); err != nil && first == nil {
			first = err
		}
	}
	for _, b := range []Button{Left, Right, Middle} {
		if err := d.ButtonUp(b); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (d *linuxDriver) Capabilities() Capabilities {
	backend := "x11"
	if d.be == backendLibei {
		backend = "libei"
	}
	return Capabilities{
		Backend:              backend,
		FallbackReason:       d.fallback,
		CanReadCursor:        d.queryOK,
		CanPickPoint:         d.queryOK,
		CanEnumerateDisplays: d.queryOK || d.libeiAbsolute,
		CanMoveAbsolute:      d.be == backendX11 || d.libeiAbsolute,
	}
}

func (d *linuxDriver) Close() error {
	if d.be == backendLibei {
		libei.Close()
		return nil
	}
	x11.Close()
	return nil
}
