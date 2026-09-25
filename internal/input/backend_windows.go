//go:build windows

package input

import (
	"errors"
	"log"
	"time"

	"github.com/go-vgo/robotgo/win"
	tw "github.com/tailscale/win"
)

// windowsDriver injects through the pure-Go SendInput backend. It can read the
// real cursor and enumerate displays, so absolute targets are always available.
type windowsDriver struct{}

// New returns the Windows driver.
func New() (Driver, error) {
	d := &windowsDriver{}
	x, y, w, h := d.virtualRect()
	log.Printf("input/win: virtual desktop origin=(%d,%d) size=%dx%d monitors=%d",
		x, y, w, h, win.DisplaysNum())
	return d, nil
}

func (d *windowsDriver) ClickAt(x, y int, b Button, hold time.Duration) error {
	win.Move(x, y)
	if err := win.MouseDown(b.String()); err != nil {
		return err
	}
	time.Sleep(hold)
	return win.MouseUp(b.String())
}

func (d *windowsDriver) ClickHere(b Button, hold time.Duration) error {
	if err := win.MouseDown(b.String()); err != nil {
		return err
	}
	time.Sleep(hold)
	return win.MouseUp(b.String())
}

// virtualRect returns the virtual-desktop bounding box: its top-left origin in
// OS coordinates (may be negative when a monitor is left/above the primary) and
// its size. This is the canonical coordinate space the app stores.
func (d *windowsDriver) virtualRect() (x, y, w, h int) {
	// Use the canonical virtual-screen metrics directly; robotgo/win only
	// exposes them via GetScreenRect(>0), which requires DisplaysNum()>1.
	x = int(tw.GetSystemMetrics(tw.SM_XVIRTUALSCREEN))
	y = int(tw.GetSystemMetrics(tw.SM_YVIRTUALSCREEN))
	w = int(tw.GetSystemMetrics(tw.SM_CXVIRTUALSCREEN))
	h = int(tw.GetSystemMetrics(tw.SM_CYVIRTUALSCREEN))
	if w <= 0 || h <= 0 {
		pw, ph := win.GetScreenSize()
		return 0, 0, pw, ph
	}
	return x, y, w, h
}

func (d *windowsDriver) Move(x, y int) error {
	// Windows coordinates are already primary-relative (the primary monitor's
	// top-left is (0,0)), which is the canonical space.
	win.Move(x, y)
	return nil
}

func (d *windowsDriver) ButtonDown(b Button) error { return win.MouseDown(b.String()) }
func (d *windowsDriver) ButtonUp(b Button) error   { return win.MouseUp(b.String()) }

func (d *windowsDriver) CursorPos() (int, int, bool) {
	// Already primary-relative.
	x, y := win.Location()
	return x, y, true
}

func (d *windowsDriver) ScreenSize() (int, int, error) {
	// robotgo/win.GetScreenSize reports the primary monitor only; report the
	// whole virtual desktop size instead.
	_, _, w, h := d.virtualRect()
	if w == 0 && h == 0 {
		return 0, 0, errors.New("no display geometry available")
	}
	return w, h, nil
}

func (d *windowsDriver) Displays() ([]Display, error) {
	// Per-monitor enumeration is not available through robotgo/win, so report
	// the virtual desktop rectangle as a single display. Its origin is
	// primary-relative (negative when a monitor sits left/above the primary).
	x, y, w, h := d.virtualRect()
	if w == 0 && h == 0 {
		return nil, errors.New("no display geometry available")
	}
	scale := 1.0
	if s := win.ScaleF(0); s > 0 {
		scale = s
	}
	return []Display{{X: x, Y: y, W: w, H: h, Scale: scale, Primary: true}}, nil
}

func (d *windowsDriver) ReleaseAll() error {
	var first error
	for _, b := range []Button{Left, Right, Middle} {
		if err := win.MouseUp(b.String()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (d *windowsDriver) Capabilities() Capabilities {
	return Capabilities{
		Backend:              "win",
		CanReadCursor:        true,
		CanPickPoint:         true,
		CanEnumerateDisplays: true,
		CanMoveAbsolute:      true,
	}
}

func (d *windowsDriver) Close() error { return nil }
