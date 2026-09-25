// Package input is the seam between the engine and the OS. It owns the "no
// motion" contract for cursor clicks and keeps every backend swappable.
package input

import (
	"fmt"
	"time"
)

// Button identifies a mouse button.
type Button int

const (
	Left Button = iota
	Right
	Middle
)

// String returns the profile vocabulary name.
func (b Button) String() string {
	switch b {
	case Right:
		return "right"
	case Middle:
		return "middle"
	default:
		return "left"
	}
}

// ParseButton converts a profile button name.
func ParseButton(s string) (Button, error) {
	switch s {
	case "left", "":
		return Left, nil
	case "right":
		return Right, nil
	case "middle", "center":
		return Middle, nil
	default:
		return Left, fmt.Errorf("unknown button %q", s)
	}
}

// Display is one monitor in physical pixels of the virtual desktop.
type Display struct {
	X, Y    int
	W, H    int
	Scale   float64
	Primary bool
}

// Capabilities reports what the active backend can do, for the UI.
type Capabilities struct {
	Backend              string
	FallbackReason       string
	CanReadCursor        bool
	CanPickPoint         bool
	CanEnumerateDisplays bool
	CanMoveAbsolute      bool
}

// Driver injects input and answers queries about the desktop.
type Driver interface {
	// ClickAt moves the pointer to (x,y) and clicks — target kind "absolute".
	ClickAt(x, y int, button Button, hold time.Duration) error

	// ClickHere presses and releases button at the pointer's CURRENT position
	// without issuing any motion event — target kind "cursor".
	ClickHere(button Button, hold time.Duration) error

	// Move moves the pointer without clicking.
	Move(x, y int) error

	ButtonDown(button Button) error
	ButtonUp(button Button) error

	// CursorPos reports the real pointer position. ok == false when the active
	// backend cannot read it. Callers must degrade, never guess.
	CursorPos() (x, y int, ok bool)

	// ScreenSize returns the virtual desktop size in physical pixels.
	ScreenSize() (w, h int, err error)

	// Displays enumerates monitors. A backend that cannot enumerate returns a
	// single synthetic display rather than an error.
	Displays() ([]Display, error)

	// ReleaseAll releases every button we may hold. Called on every exit path.
	ReleaseAll() error

	// Capabilities reports the active backend and its feature set.
	Capabilities() Capabilities

	Close() error
}
