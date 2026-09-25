package input

import "testing"

func TestFakeDriverFlagsMoveBeforeClickHere(t *testing.T) {
	d := NewFake()
	// A well-behaved cursor click: no preceding motion.
	if err := d.ClickHere(Left, 0); err != nil {
		t.Fatal(err)
	}
	if d.MoveBeforeClickHere {
		t.Error("ClickHere without a Move must not be flagged")
	}

	// A misbehaving sequence is detected.
	d2 := NewFake()
	_ = d2.Move(10, 10)
	_ = d2.ClickHere(Left, 0)
	if !d2.MoveBeforeClickHere {
		t.Error("Move immediately before ClickHere must be flagged")
	}
}

func TestParseButton(t *testing.T) {
	for in, want := range map[string]Button{"left": Left, "": Left, "right": Right, "middle": Middle, "center": Middle} {
		got, err := ParseButton(in)
		if err != nil || got != want {
			t.Errorf("ParseButton(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := ParseButton("nope"); err == nil {
		t.Error("unknown button should error")
	}
}
