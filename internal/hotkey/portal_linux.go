//go:build linux

package hotkey

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// PortalAppID is the application id registered with the portal. It must be
// stable: KDE keys shortcuts per component id (DESIGN §2.4).
const PortalAppID = "auto-clicker"

const (
	portalDest           = "org.freedesktop.portal.Desktop"
	portalPath           = "/org/freedesktop/portal/desktop"
	ifaceGlobalShortcuts = "org.freedesktop.portal.GlobalShortcuts"
	ifaceRequest         = "org.freedesktop.portal.Request"
	ifaceSession         = "org.freedesktop.portal.Session"
	ifaceRegistry        = "org.freedesktop.host.portal.Registry"
)

// portalSource implements Source through org.freedesktop.portal.GlobalShortcuts.
type portalSource struct {
	conn       *dbus.Conn
	obj        dbus.BusObject
	uniqueName string
	session    dbus.ObjectPath
	handler    Handler

	mu    sync.Mutex
	ids   map[string]bool // shortcut ids we registered
	token uint64
}

// newPortal establishes a GlobalShortcuts session. It may block on the user
// consent dialog when shortcuts are bound, so callers should not run it on the
// UI thread.
func newPortal(appID string, h Handler) (Source, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("hotkey/portal: session bus: %w", err)
	}
	p := &portalSource{
		conn:       conn,
		obj:        conn.Object(portalDest, dbus.ObjectPath(portalPath)),
		uniqueName: conn.Names()[0],
		handler:    h,
		ids:        map[string]bool{},
	}
	// Unsandboxed callers must register an app id before CreateSession
	// (Appendix B.4). Not fatal if the portal does not expose the registry.
	if call := p.obj.Call(ifaceRegistry+".Register", 0, appID, map[string]dbus.Variant{}); call.Err != nil {
		log.Printf("hotkey/portal: app id registration failed (component may be wrong): %v", call.Err)
	}
	if err := p.createSession(); err != nil {
		conn.Close()
		return nil, err
	}
	p.watchActivated()
	return p, nil
}

// Name implements Source.
func (p *portalSource) Name() string { return "portal" }

func (p *portalSource) nextToken(prefix string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.token++
	return fmt.Sprintf("autoclicker_%s_%d", prefix, p.token)
}

func (p *portalSource) escapedSender() string {
	s := p.uniqueName
	if len(s) > 0 && s[0] == ':' {
		s = s[1:]
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, '_')
		} else {
			out = append(out, s[i])
		}
	}
	return string(out)
}

// callRequest invokes a portal method that returns a Request handle and waits
// for its Response signal (subscribed before the call to avoid a race).
func (p *portalSource) callRequest(iface, method string, args ...any) (map[string]dbus.Variant, error) {
	token := p.nextToken("req")
	reqPath := dbus.ObjectPath(fmt.Sprintf(
		"/org/freedesktop/portal/desktop/request/%s/%s", p.escapedSender(), token))

	if err := p.conn.AddMatchSignal(
		dbus.WithMatchObjectPath(reqPath),
		dbus.WithMatchInterface(ifaceRequest),
		dbus.WithMatchMember("Response"),
	); err != nil {
		return nil, err
	}
	defer p.conn.RemoveMatchSignal(
		dbus.WithMatchObjectPath(reqPath),
		dbus.WithMatchInterface(ifaceRequest),
		dbus.WithMatchMember("Response"),
	)

	sigCh := make(chan *dbus.Signal, 4)
	p.conn.Signal(sigCh)
	defer p.conn.RemoveSignal(sigCh)

	if len(args) == 0 {
		return nil, fmt.Errorf("hotkey/portal: %s needs options", method)
	}
	if opts, ok := args[len(args)-1].(map[string]dbus.Variant); ok {
		opts["handle_token"] = dbus.MakeVariant(token)
	}

	var returned dbus.ObjectPath
	if call := p.obj.Call(iface+"."+method, 0, args...); call.Err != nil {
		return nil, call.Err
	} else if err := call.Store(&returned); err != nil {
		return nil, err
	}

	timeout := time.NewTimer(120 * time.Second) // consent dialog can be slow
	defer timeout.Stop()
	for {
		select {
		case sig := <-sigCh:
			if sig == nil || sig.Path != reqPath || sig.Name != ifaceRequest+".Response" {
				continue
			}
			if len(sig.Body) < 2 {
				return nil, fmt.Errorf("hotkey/portal: malformed response")
			}
			code, _ := sig.Body[0].(uint32)
			results, _ := sig.Body[1].(map[string]dbus.Variant)
			if code == 0 {
				return results, nil
			}
			return nil, fmt.Errorf("hotkey/portal: %s response code %d", method, code)
		case <-timeout.C:
			return nil, fmt.Errorf("hotkey/portal: %s timed out", method)
		}
	}
}

func (p *portalSource) createSession() error {
	sessionToken := p.nextToken("session")
	opts := map[string]dbus.Variant{
		"session_handle_token": dbus.MakeVariant(sessionToken),
	}
	results, err := p.callRequest(ifaceGlobalShortcuts, "CreateSession", opts)
	if err != nil {
		return err
	}
	// The handle may arrive as a string or an object path (Appendix B.2).
	if v, ok := results["session_handle"]; ok {
		switch s := v.Value().(type) {
		case dbus.ObjectPath:
			p.session = s
		case string:
			p.session = dbus.ObjectPath(s)
		}
	}
	if p.session == "" {
		p.session = dbus.ObjectPath(fmt.Sprintf(
			"/org/freedesktop/portal/desktop/session/%s/%s", p.escapedSender(), sessionToken))
	}
	return nil
}

// portalShortcut maps to the D-Bus struct (sa{sv}) used by BindShortcuts.
// A concrete struct is required: []interface{} would be marshalled as "av",
// which the portal rejects with a type mismatch (as seen in the wild).
type portalShortcut struct {
	ID    string
	Props map[string]dbus.Variant
}

// Register implements Source. All shortcuts go in one BindShortcuts call
// (Appendix B.5); the reply is deferred until the user answers the consent
// dialog (Appendix B.6).
func (p *portalSource) Register(bindings []Binding) (map[string]string, error) {
	shortcuts := make([]portalShortcut, 0, len(bindings))
	for _, b := range bindings {
		trigger, err := CanonicalTrigger(b.Trigger)
		if err != nil {
			return nil, err
		}
		desc := b.Label
		if desc == "" {
			desc = b.ID
		}
		shortcuts = append(shortcuts, portalShortcut{
			ID: b.ID,
			Props: map[string]dbus.Variant{
				"description":       dbus.MakeVariant(desc),
				"preferred_trigger": dbus.MakeVariant(trigger),
			},
		})
	}

	// BindShortcuts is asynchronous: it returns a Request handle and the
	// granted shortcuts arrive on org.freedesktop.portal.Request.Response
	// (the reply is deferred until the consent dialog is answered). So it goes
	// through callRequest like CreateSession, not a direct Store of the return.
	results, err := p.callRequest(ifaceGlobalShortcuts, "BindShortcuts",
		p.session, shortcuts, "", map[string]dbus.Variant{})
	if err != nil {
		return nil, err
	}

	granted := map[string]string{}
	var list []portalShortcut
	if v, ok := results["shortcuts"]; ok {
		if err := v.Store(&list); err != nil {
			return nil, err
		}
	}
	p.mu.Lock()
	for _, s := range list {
		trigger := ""
		if tv, ok := s.Props["trigger_description"]; ok {
			trigger, _ = tv.Value().(string)
		}
		granted[s.ID] = trigger
		p.ids[s.ID] = true
	}
	p.mu.Unlock()
	log.Printf("hotkey/portal: bound %d shortcuts: %v", len(granted), granted)
	return granted, nil
}

func (p *portalSource) watchActivated() {
	// Match by interface + member only, NOT by object path: the compositor may
	// emit Activated from a path other than the session handle, and filtering by
	// path then silently drops every activation.
	if err := p.conn.AddMatchSignal(
		dbus.WithMatchInterface(ifaceGlobalShortcuts),
		dbus.WithMatchMember("Activated"),
	); err != nil {
		log.Printf("hotkey/portal: watch Activated: %v", err)
		return
	}
	ch := make(chan *dbus.Signal, 8)
	p.conn.Signal(ch)
	go func() {
		defer p.conn.RemoveSignal(ch)
		for sig := range ch {
			if sig == nil || !strings.HasSuffix(sig.Name, ".Activated") {
				continue
			}
			if len(sig.Body) < 2 {
				continue
			}
			// Body: (o session_handle, s shortcut_id, t timestamp, a{sv} options)
			if s, ok := sig.Body[0].(dbus.ObjectPath); ok && p.session != "" && s != p.session {
				continue
			}
			id, _ := sig.Body[1].(string)
			p.mu.Lock()
			known := p.ids[id]
			p.mu.Unlock()
			log.Printf("hotkey/portal: Activated %q (known=%v)", id, known)
			if known && p.handler != nil {
				p.handler(id)
			}
		}
	}()
}

// Close implements Source. Shortcuts are session-scoped, so closing the session
// removes them from kglobalshortcutsrc (Appendix B.7).
func (p *portalSource) Close() error {
	if p.session != "" {
		_ = p.obj.Call(ifaceSession+".Close", 0).Err
	}
	return p.conn.Close()
}
