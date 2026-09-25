package hotkey

import "testing"

func TestTriggerToKey(t *testing.T) {
	cases := []struct {
		in     string
		keysym uint32
		mods   ModMask
	}{
		{"F8", 0xFFBE + 7, 0},
		{"Ctrl+Shift+F12", 0xFFBE + 11, ModControl | ModShift},
		{"Super+F9", 0xFFBE + 8, ModSuper},
		{"A", 0x41, 0},
		{"7", 0x37, 0},
		{"Right", 0xFF53, 0},
		{"Alt+PageDown", 0xFF56, ModAlt},
	}
	for _, c := range cases {
		spec, err := TriggerToKey(c.in)
		if err != nil {
			t.Errorf("TriggerToKey(%q): %v", c.in, err)
			continue
		}
		if spec.Keysym != c.keysym || spec.Mods != c.mods {
			t.Errorf("TriggerToKey(%q) = {0x%X, %d}, want {0x%X, %d}",
				c.in, spec.Keysym, spec.Mods, c.keysym, c.mods)
		}
	}
	if _, err := TriggerToKey("Ctrl+F25"); err == nil {
		t.Error("F25 should be unsupported")
	}
}
