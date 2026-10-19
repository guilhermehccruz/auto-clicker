package model

import (
	"fmt"
	"strconv"
	"strings"
)

// keyModifiers is the canonical modifier order for key combos.
var keyModifiers = []string{"ctrl", "shift", "alt", "cmd"}

var namedKeys = map[string]bool{
	"space": true, "enter": true, "return": true, "tab": true,
	"esc": true, "escape": true, "backspace": true, "delete": true,
	"insert": true, "home": true, "end": true, "pageup": true,
	"pagedown": true, "up": true, "down": true, "left": true, "right": true,
	"capslock": true, "print": true, "printscreen": true, "menu": true,
}

// ParseKeyCombo parses a key combo such as "ctrl+shift+c" or "f5" into its
// canonical modifiers (ctrl/shift/alt/cmd) and key name (lowercase). It is the
// single validator used by the UI and the engine.
func ParseKeyCombo(combo string) (mods []string, key string, err error) {
	c := strings.ToLower(strings.TrimSpace(combo))
	if c == "" {
		return nil, "", fmt.Errorf("empty key combo")
	}
	parts := strings.Split(c, "+")
	key = parts[len(parts)-1]
	if key == "" {
		return nil, "", fmt.Errorf("malformed key combo %q", combo)
	}
	set := map[string]bool{}
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "ctrl", "control":
			set["ctrl"] = true
		case "shift":
			set["shift"] = true
		case "alt":
			set["alt"] = true
		case "cmd", "super", "win", "logo", "meta":
			set["cmd"] = true
		default:
			return nil, "", fmt.Errorf("unknown modifier %q", m)
		}
	}
	if !validKeyName(key) {
		return nil, "", fmt.Errorf("unsupported key %q", key)
	}
	for _, m := range keyModifiers {
		if set[m] {
			mods = append(mods, m)
		}
	}
	return mods, key, nil
}

func validKeyName(k string) bool {
	if namedKeys[k] {
		return true
	}
	if len(k) == 1 {
		c := k[0]
		return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			strings.ContainsRune("`-=[]\\;',./", rune(c))
	}
	if strings.HasPrefix(k, "f") && len(k) <= 3 {
		if n, err := strconv.Atoi(k[1:]); err == nil && n >= 1 && n <= 24 {
			return true
		}
	}
	return false
}
