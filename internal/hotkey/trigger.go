// Package hotkey registers global shortcuts. This file owns trigger
// normalization: the portal requires UPPERCASE modifier names, and a
// wrong-case modifier silently produces an empty binding (Appendix B.3).
package hotkey

import (
	"fmt"
	"strings"
)

// modifierOrder is the canonical order modifiers are emitted in.
var modifierOrder = []string{"CTRL", "SHIFT", "ALT", "LOGO", "CAPS", "NUM"}

// modifierAliases maps accepted spellings to the canonical portal name.
var modifierAliases = map[string]string{
	"CTRL": "CTRL", "CONTROL": "CTRL",
	"SHIFT": "SHIFT",
	"ALT":   "ALT", "META": "ALT",
	"LOGO": "LOGO", "SUPER": "LOGO", "WIN": "LOGO",
	"CAPS": "CAPS", "CAPSLOCK": "CAPS",
	"NUM": "NUM", "NUMLOCK": "NUM",
}

// CanonicalTrigger normalizes a human trigger such as "Ctrl+Shift+F12" into the
// portal form "CTRL+SHIFT+F12": modifiers uppercased, deduplicated and put in a
// fixed order, key preserved.
func CanonicalTrigger(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("hotkey: empty trigger")
	}
	parts := strings.Split(s, "+")
	key := ""
	mods := map[string]bool{}
	for i, raw := range parts {
		p := strings.TrimSpace(raw)
		if p == "" {
			return "", fmt.Errorf("hotkey: malformed trigger %q", s)
		}
		if i == len(parts)-1 {
			key = p
			continue
		}
		m, ok := modifierAliases[strings.ToUpper(p)]
		if !ok {
			return "", fmt.Errorf("hotkey: unknown modifier %q in %q", p, s)
		}
		mods[m] = true
	}
	if key == "" {
		return "", fmt.Errorf("hotkey: %q has no key", s)
	}
	if err := validateKey(key); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, m := range modifierOrder {
		if mods[m] {
			if b.Len() > 0 {
				b.WriteByte('+')
			}
			b.WriteString(m)
		}
	}
	if b.Len() > 0 {
		b.WriteByte('+')
	}
	b.WriteString(normalizeKey(key))
	return b.String(), nil
}

// namedKeys are keys whose spelling is passed through to xkb_keysym.
var namedKeys = map[string]bool{
	"SPACE": true, "ESC": true, "ESCAPE": true, "TAB": true, "ENTER": true,
	"RETURN": true, "BACKSPACE": true, "DELETE": true, "INSERT": true,
	"HOME": true, "END": true, "PAGEUP": true, "PAGEDOWN": true,
	"UP": true, "DOWN": true, "LEFT": true, "RIGHT": true,
	"PAUSE": true, "PRINT": true,
}

func normalizeKey(key string) string {
	u := strings.ToUpper(key)
	if len(u) == 1 {
		return u
	}
	if strings.HasPrefix(u, "F") && len(u) <= 3 {
		return u
	}
	return u
}

func validateKey(key string) error {
	u := strings.ToUpper(key)
	if namedKeys[u] {
		return nil
	}
	if len(u) == 1 {
		if (u[0] >= 'A' && u[0] <= 'Z') || (u[0] >= '0' && u[0] <= '9') {
			return nil
		}
		return fmt.Errorf("hotkey: unsupported key %q", key)
	}
	if u[0] == 'F' && len(u) <= 3 {
		for _, c := range u[1:] {
			if c < '0' || c > '9' {
				return fmt.Errorf("hotkey: unsupported key %q", key)
			}
		}
		return nil
	}
	return fmt.Errorf("hotkey: unsupported key %q", key)
}
