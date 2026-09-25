package hotkey

import "testing"

func TestCanonicalTrigger(t *testing.T) {
	cases := map[string]string{
		"F8":             "F8",
		"f8":             "F8",
		"Ctrl+Shift+F12": "CTRL+SHIFT+F12",
		"shift+ctrl+f12": "CTRL+SHIFT+F12",
		"Ctrl+F12":       "CTRL+F12",
		"Control+Alt+F1": "CTRL+ALT+F1",
		"Super+F9":       "LOGO+F9",
		"Ctrl+Ctrl+F9":   "CTRL+F9", // deduplicated
		"Alt+Shift+P":    "SHIFT+ALT+P",
	}
	for in, want := range cases {
		got, err := CanonicalTrigger(in)
		if err != nil {
			t.Errorf("CanonicalTrigger(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("CanonicalTrigger(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCanonicalTriggerErrors(t *testing.T) {
	for _, in := range []string{"", "Ctrl+", "+F8", "Ctrl+Bogus+F8", "Ctrl+€"} {
		if _, err := CanonicalTrigger(in); err == nil {
			t.Errorf("CanonicalTrigger(%q) should fail", in)
		}
	}
}

func TestDefaultBindingsCanonicalize(t *testing.T) {
	// The default panic trigger must survive canonicalization, since a
	// wrong-case modifier silently produces an empty binding.
	for _, tr := range []string{"F8", "F9", "Ctrl+Shift+F12"} {
		if _, err := CanonicalTrigger(tr); err != nil {
			t.Errorf("default trigger %q invalid: %v", tr, err)
		}
	}
}
