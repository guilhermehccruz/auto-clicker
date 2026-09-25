package input

import (
	"errors"
	"time"
)

// Call records one driver invocation for assertions.
type Call struct {
	Op     string
	X, Y   int
	Button Button
	Hold   time.Duration
}

// FakeDriver is a Driver for tests. It records intended calls and lets tests
// assert the "no motion before ClickHere" contract.
type FakeDriver struct {
	Calls []Call

	// MoveBeforeClickHere is set if a Move immediately precedes a ClickHere,
	// which would break the cursor-target contract.
	MoveBeforeClickHere bool

	ReleaseAllCount int
	Closed          bool

	// Injectable behaviour.
	InjectErr  error
	CursorX    int
	CursorY    int
	CursorOK   bool
	ScreenW    int
	ScreenH    int
	DisplaysN  []Display
	Capability Capabilities

	// OnHold, when set, is called with the hold duration during ClickAt /
	// ClickHere. Tests use it to advance a fake clock, since the real backends
	// sleep for the hold internally.
	OnHold func(time.Duration)

	lastOp string
}

// NewFake returns a FakeDriver with a single 1920x1080 display and a readable
// cursor at the origin.
func NewFake() *FakeDriver {
	return &FakeDriver{
		CursorOK:   true,
		ScreenW:    1920,
		ScreenH:    1080,
		DisplaysN:  []Display{{X: 0, Y: 0, W: 1920, H: 1080, Scale: 1, Primary: true}},
		Capability: Capabilities{Backend: "fake", CanReadCursor: true, CanPickPoint: true, CanEnumerateDisplays: true, CanMoveAbsolute: true},
	}
}

func (f *FakeDriver) record(c Call) {
	f.Calls = append(f.Calls, c)
	f.lastOp = c.Op
}

func (f *FakeDriver) ClickAt(x, y int, b Button, hold time.Duration) error {
	if f.InjectErr != nil {
		return f.InjectErr
	}
	f.record(Call{Op: "ClickAt", X: x, Y: y, Button: b, Hold: hold})
	if f.OnHold != nil {
		f.OnHold(hold)
	}
	return nil
}

func (f *FakeDriver) ClickHere(b Button, hold time.Duration) error {
	if f.InjectErr != nil {
		return f.InjectErr
	}
	if f.lastOp == "Move" {
		f.MoveBeforeClickHere = true
	}
	f.record(Call{Op: "ClickHere", Button: b, Hold: hold})
	if f.OnHold != nil {
		f.OnHold(hold)
	}
	return nil
}

func (f *FakeDriver) Move(x, y int) error {
	if f.InjectErr != nil {
		return f.InjectErr
	}
	f.record(Call{Op: "Move", X: x, Y: y})
	return nil
}

func (f *FakeDriver) ButtonDown(b Button) error {
	if f.InjectErr != nil {
		return f.InjectErr
	}
	f.record(Call{Op: "ButtonDown", Button: b})
	return nil
}

func (f *FakeDriver) ButtonUp(b Button) error {
	if f.InjectErr != nil {
		return f.InjectErr
	}
	f.record(Call{Op: "ButtonUp", Button: b})
	return nil
}

func (f *FakeDriver) CursorPos() (int, int, bool) {
	return f.CursorX, f.CursorY, f.CursorOK
}

func (f *FakeDriver) ScreenSize() (int, int, error) {
	if f.ScreenW == 0 {
		return 0, 0, errors.New("no screen")
	}
	return f.ScreenW, f.ScreenH, nil
}

func (f *FakeDriver) Displays() ([]Display, error) {
	if len(f.DisplaysN) == 0 {
		return []Display{{W: f.ScreenW, H: f.ScreenH, Scale: 1, Primary: true}}, nil
	}
	return f.DisplaysN, nil
}

func (f *FakeDriver) ReleaseAll() error {
	f.ReleaseAllCount++
	f.record(Call{Op: "ReleaseAll"})
	return nil
}

func (f *FakeDriver) Capabilities() Capabilities { return f.Capability }

func (f *FakeDriver) Close() error {
	f.Closed = true
	f.record(Call{Op: "Close"})
	return nil
}

// Clicks returns only the click calls, in order.
func (f *FakeDriver) Clicks() []Call {
	var out []Call
	for _, c := range f.Calls {
		if c.Op == "ClickAt" || c.Op == "ClickHere" {
			out = append(out, c)
		}
	}
	return out
}
