package hotkey

import (
	"fmt"
	"strings"
)

// ModMask mirrors the X11 modifier mask bits we care about. It is defined here
// (rather than imported) so this mapping is testable on any platform.
type ModMask uint16

// X11 modifier masks.
const (
	ModShift   ModMask = 1 << 0
	ModLock    ModMask = 1 << 1
	ModControl ModMask = 1 << 2
	ModAlt     ModMask = 1 << 3 // Mod1
	ModNum     ModMask = 1 << 4 // Mod2
	ModSuper   ModMask = 1 << 6 // Mod4
)

// KeySpec is an X11 keycode lookup request.
type KeySpec struct {
	Keysym uint32
	Mods   ModMask
}

// keysymByName maps our supported named keys to X11 keysyms.
var keysymByName = map[string]uint32{
	"SPACE":     0x0020,
	"ESC":       0xFF1B,
	"ESCAPE":    0xFF1B,
	"TAB":       0xFF09,
	"ENTER":     0xFF0D,
	"RETURN":    0xFF0D,
	"BACKSPACE": 0xFF08,
	"DELETE":    0xFFFF,
	"INSERT":    0xFF63,
	"HOME":      0xFF50,
	"END":       0xFF57,
	"PAGEUP":    0xFF55,
	"PAGEDOWN":  0xFF56,
	"UP":        0xFF52,
	"DOWN":      0xFF54,
	"LEFT":      0xFF51,
	"RIGHT":     0xFF53,
	"PAUSE":     0xFF13,
	"PRINT":     0xFF61,
}

// TriggerToKey canonicalizes a trigger and converts it to an X11 KeySpec.
func TriggerToKey(trigger string) (KeySpec, error) {
	canon, err := CanonicalTrigger(trigger)
	if err != nil {
		return KeySpec{}, err
	}
	parts := strings.Split(canon, "+")
	key := parts[len(parts)-1]
	var spec KeySpec
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "SHIFT":
			spec.Mods |= ModShift
		case "CTRL":
			spec.Mods |= ModControl
		case "ALT":
			spec.Mods |= ModAlt
		case "LOGO":
			spec.Mods |= ModSuper
		case "CAPS":
			spec.Mods |= ModLock
		case "NUM":
			spec.Mods |= ModNum
		}
	}
	spec.Keysym, err = keyToKeysym(key)
	if err != nil {
		return KeySpec{}, err
	}
	return spec, nil
}

func keyToKeysym(key string) (uint32, error) {
	if sym, ok := keysymByName[key]; ok {
		return sym, nil
	}
	if len(key) == 1 {
		c := key[0]
		if c >= 'A' && c <= 'Z' {
			return uint32(c), nil // Latin-1: keysym == uppercase ASCII code
		}
		if c >= '0' && c <= '9' {
			return uint32(c), nil
		}
	}
	if strings.HasPrefix(key, "F") {
		var n int
		if _, err := fmt.Sscanf(key, "F%d", &n); err == nil && n >= 1 && n <= 24 {
			return uint32(0xFFBE + n - 1), nil // XK_F1 = 0xFFBE
		}
	}
	return 0, fmt.Errorf("hotkey: no X11 keysym for %q", key)
}
