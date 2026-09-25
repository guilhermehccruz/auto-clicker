package hotkey

// Binding is one desired global shortcut.
type Binding struct {
	ID    string // "run" | "pause" | "panic"
	Label string // human description shown in diagnostics
	// Trigger is a human form ("Ctrl+Shift+F12"); Register canonicalizes it.
	Trigger string
}

// Handler receives the id of an activated binding ("run", "pause", "panic").
type Handler func(id string)

// FallbackReason records why the preferred global-shortcut source was not used
// (empty when it was). Platform implementations set it in New; diagnostics
// reads it.
var FallbackReason string

// Source registers global shortcuts and reports what was granted.
type Source interface {
	// Name identifies the mechanism: "portal", "x11", "win", "unavailable".
	Name() string
	// Register binds every shortcut in a single call (the portal wipes
	// previously-registered shortcuts on each call — Appendix B.5). It returns
	// the granted trigger description per binding id.
	Register(bindings []Binding) (map[string]string, error)
	Close() error
}

// Unavailable is the fallback when no global source could be established. The
// app then relies on in-window shortcuts, the max-runtime cap and the corner
// failsafe, and shows a startup warning (DESIGN §4.1).
type Unavailable struct{ Reason string }

// Name implements Source.
func (Unavailable) Name() string { return "unavailable" }

// Register implements Source.
func (u Unavailable) Register([]Binding) (map[string]string, error) { return nil, nil }

// Close implements Source.
func (Unavailable) Close() error { return nil }
