package cursor

import (
	"testing"

	"auto-clicker/internal/input"
)

func TestAvailableAndPos(t *testing.T) {
	drv := input.NewFake()
	drv.CursorX, drv.CursorY = 42, 99
	p := New(drv)
	if !p.Available() {
		t.Fatal("fake driver should report a readable cursor")
	}
	x, y, ok := p.Pos()
	if !ok || x != 42 || y != 99 {
		t.Fatalf("Pos = (%d,%d,%v)", x, y, ok)
	}
}

func TestUnavailableDegrades(t *testing.T) {
	drv := input.NewFake()
	drv.CursorOK = false
	p := New(drv)
	if p.Available() {
		t.Fatal("should be unavailable")
	}
	if _, _, ok := p.Pos(); ok {
		t.Fatal("Pos must report ok=false when unavailable")
	}
}
