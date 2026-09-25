package hotkey

import (
	"fmt"
	"strings"
)

// Windows virtual-key codes and RegisterHotKey modifier flags. Defined here so
// the mapping is testable on any platform.
const (
	ModAltWin     uint32 = 0x0001
	ModControlWin uint32 = 0x0002
	ModShiftWin   uint32 = 0x0004
	ModWinWin     uint32 = 0x0008
)

var vkByName = map[string]uint32{
	"SPACE": 0x20, "ESC": 0x1B, "ESCAPE": 0x1B, "TAB": 0x09,
	"ENTER": 0x0D, "RETURN": 0x0D, "BACKSPACE": 0x08,
	"DELETE": 0x2E, "INSERT": 0x2D, "HOME": 0x24, "END": 0x23,
	"PAGEUP": 0x21, "PAGEDOWN": 0x22, "UP": 0x26, "DOWN": 0x28,
	"LEFT": 0x25, "RIGHT": 0x27, "PAUSE": 0x13, "PRINT": 0x2C,
}

// TriggerToVK converts a trigger to a Windows virtual-key code and modifier
// flags for RegisterHotKey.
func TriggerToVK(trigger string) (vk uint32, mods uint32, err error) {
	canon, err := CanonicalTrigger(trigger)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Split(canon, "+")
	key := parts[len(parts)-1]
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "SHIFT":
			mods |= ModShiftWin
		case "CTRL":
			mods |= ModControlWin
		case "ALT":
			mods |= ModAltWin
		case "LOGO":
			mods |= ModWinWin
		}
	}
	vk, err = keyToVK(key)
	return vk, mods, err
}

func keyToVK(key string) (uint32, error) {
	if v, ok := vkByName[key]; ok {
		return v, nil
	}
	if len(key) == 1 {
		c := key[0]
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			return uint32(c), nil
		}
	}
	if strings.HasPrefix(key, "F") {
		var n int
		if _, err := fmt.Sscanf(key, "F%d", &n); err == nil && n >= 1 && n <= 24 {
			return uint32(0x70 + n - 1), nil // VK_F1 = 0x70
		}
	}
	return 0, fmt.Errorf("hotkey: no virtual-key code for %q", key)
}
