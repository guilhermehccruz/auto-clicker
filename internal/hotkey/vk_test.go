package hotkey

import "testing"

func TestTriggerToVK(t *testing.T) {
	cases := []struct {
		in   string
		vk   uint32
		mods uint32
	}{
		{"F8", 0x70 + 7, 0},
		{"Ctrl+Shift+F12", 0x70 + 11, ModControlWin | ModShiftWin},
		{"Super+F9", 0x70 + 8, ModWinWin},
		{"A", 0x41, 0},
		{"7", 0x37, 0},
		{"Alt+Delete", 0x2E, ModAltWin},
	}
	for _, c := range cases {
		vk, mods, err := TriggerToVK(c.in)
		if err != nil {
			t.Errorf("TriggerToVK(%q): %v", c.in, err)
			continue
		}
		if vk != c.vk || mods != c.mods {
			t.Errorf("TriggerToVK(%q) = {0x%X, %d}, want {0x%X, %d}", c.in, vk, mods, c.vk, c.mods)
		}
	}
}
