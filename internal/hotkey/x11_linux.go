//go:build linux

package hotkey

import (
	"errors"
	"os"
	"sync"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// x11Source grabs keys on the root window with XGrabKey. It fires while any
// XWayland/X11 client has focus (which covers Steam/Proton games) but not while
// a native-Wayland window is focused (Appendix B.9).
type x11Source struct {
	conn    *xgb.Conn
	root    xproto.Window
	handler Handler

	mu    sync.Mutex
	grabs map[uint32]string // keycode|modmask -> binding id
	stop  chan struct{}
	once  sync.Once
}

// newX11 returns an X11 source, or an error if no X server is reachable.
func newX11(h Handler) (Source, error) {
	if !x11Reachable() {
		return nil, errors.New("hotkey: no X server (XWayland) available")
	}
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, err
	}
	setup := xproto.Setup(conn)
	root := setup.DefaultScreen(conn).Root
	return &x11Source{conn: conn, root: root, handler: h, grabs: map[uint32]string{}, stop: make(chan struct{})}, nil
}

func x11Reachable() bool {
	// xgb.NewConn does the real work; this avoids a second connection attempt
	// by checking DISPLAY first.
	return os.Getenv("DISPLAY") != ""
}

// Name implements Source.
func (s *x11Source) Name() string { return "x11" }

// Register implements Source.
func (s *x11Source) Register(bindings []Binding) (map[string]string, error) {
	mapping, err := s.keyboardMapping()
	if err != nil {
		return nil, err
	}
	granted := make(map[string]string, len(bindings))
	for _, b := range bindings {
		spec, err := TriggerToKey(b.Trigger)
		if err != nil {
			return nil, err
		}
		keycode, ok := mapping[spec.Keysym]
		if !ok {
			return nil, errors.New("hotkey: keysym not in keyboard mapping")
		}
		// Grab the trigger plus the NumLock/CapsLock combinations so the lock
		// modifiers do not defeat the grab.
		for _, extra := range []ModMask{0, ModNum, ModLock, ModNum | ModLock} {
			mask := uint16(spec.Mods | extra)
			xproto.GrabKey(s.conn, false, s.root, mask, keycode,
				xproto.GrabModeAsync, xproto.GrabModeAsync)
			s.mu.Lock()
			s.grabs[grabKey(keycode, mask)] = b.ID
			s.mu.Unlock()
		}
		granted[b.ID] = b.Trigger
	}
	s.conn.Sync()
	go s.loop()
	return granted, nil
}

func grabKey(keycode xproto.Keycode, mask uint16) uint32 {
	return uint32(keycode)<<16 | uint32(mask)
}

func (s *x11Source) keyboardMapping() (map[uint32]xproto.Keycode, error) {
	setup := xproto.Setup(s.conn)
	first := setup.MinKeycode
	count := byte(setup.MaxKeycode - setup.MinKeycode + 1)
	reply, err := xproto.GetKeyboardMapping(s.conn, first, count).Reply()
	if err != nil {
		return nil, err
	}
	per := int(reply.KeysymsPerKeycode)
	m := map[uint32]xproto.Keycode{}
	for i := 0; i < int(count); i++ {
		for j := 0; j < per; j++ {
			idx := i*per + j
			if idx >= len(reply.Keysyms) {
				break
			}
			sym := uint32(reply.Keysyms[idx])
			if sym == 0 {
				continue
			}
			if _, exists := m[sym]; !exists {
				m[sym] = xproto.Keycode(int(first) + i)
			}
		}
	}
	return m, nil
}

func (s *x11Source) loop() {
	for {
		ev, err := s.conn.WaitForEvent()
		if err != nil {
			return
		}
		select {
		case <-s.stop:
			return
		default:
		}
		kp, ok := ev.(xproto.KeyPressEvent)
		if !ok {
			continue
		}
		// Ignore the lock modifiers when matching.
		mask := kp.State &^ uint16(ModNum|ModLock)
		s.mu.Lock()
		id, found := s.grabs[grabKey(kp.Detail, mask)]
		s.mu.Unlock()
		if found && s.handler != nil {
			s.handler(id)
		}
	}
}

// Close implements Source.
func (s *x11Source) Close() error {
	s.once.Do(func() { close(s.stop) })
	s.mu.Lock()
	grabs := s.grabs
	s.grabs = map[uint32]string{}
	s.mu.Unlock()
	for g := range grabs {
		keycode := xproto.Keycode(g >> 16)
		mask := uint16(g)
		xproto.UngrabKey(s.conn, keycode, s.root, mask)
	}
	s.conn.Sync()
	s.conn.Close()
	return nil
}
