//go:build windows

package hotkey

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

var errClosed = errors.New("hotkey: source closed")

const (
	wmHotkey    = 0x0312
	wmQuit      = 0x0012
	modNoRepeat = 0x4000
)

var (
	user32             = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey = user32.NewProc("RegisterHotKey")
	procUnregHotKey    = user32.NewProc("UnregisterHotKey")
	procGetMessage     = user32.NewProc("GetMessageW")
	procPeekMessage    = user32.NewProc("PeekMessageW")
	procPostThreadMsg  = user32.NewProc("PostThreadMessageW")
)

// msgT mirrors the Windows MSG structure (amd64).
type msgT struct {
	hwnd    uintptr
	message uint32
	_       uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

// winSource registers global hotkeys with RegisterHotKey(NULL, …) on a dedicated
// thread that owns its message queue. With a NULL hwnd the WM_HOTKEY messages
// are posted to that thread's queue, so no window is needed. RegisterHotKey must
// run on the same thread that pumps messages, so registration is marshalled onto
// the worker.
type winSource struct {
	handler Handler

	regCh chan []Binding
	resCh chan map[string]string
	errCh chan error

	threadID uint32
	mu       sync.Mutex
	ids      map[int32]string
	once     sync.Once
	closed   atomic.Bool
}

// New returns a Windows RegisterHotKey source.
func New(h Handler) Source {
	s := &winSource{
		handler: h,
		ids:     map[int32]string{},
		regCh:   make(chan []Binding, 1),
		resCh:   make(chan map[string]string, 1),
		errCh:   make(chan error, 1),
	}
	ready := make(chan uint32, 1)
	go s.worker(ready)
	s.threadID = <-ready
	FallbackReason = ""
	return s
}

// Name implements Source.
func (s *winSource) Name() string { return "win" }

func (s *winSource) worker(ready chan<- uint32) {
	runtime.LockOSThread()
	ready <- windows.GetCurrentThreadId()

	// Wait for the (single) registration request before pumping messages.
	bindings, ok := <-s.regCh
	if !ok {
		return
	}

	// Force the thread's message queue into existence; RegisterHotKey with a
	// NULL hwnd associates with this thread's queue and needs it to exist.
	var peek msgT
	procPeekMessage.Call(uintptr(unsafe.Pointer(&peek)), 0, 0, 0, 0) // PM_NOREMOVE
	granted := make(map[string]string, len(bindings))
	id := int32(1)
	for _, b := range bindings {
		vk, mods, err := TriggerToVK(b.Trigger)
		if err != nil {
			s.errCh <- err
			return
		}
		ret, _, callErr := procRegisterHotKey.Call(0, uintptr(id), uintptr(mods|modNoRepeat), uintptr(vk))
		if ret == 0 {
			s.errCh <- callErr
			return
		}
		s.mu.Lock()
		s.ids[id] = b.ID
		s.mu.Unlock()
		granted[b.ID] = b.Trigger
		id++
	}
	s.resCh <- granted

	var msg msgT
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 { // 0 = WM_QUIT, -1 = error
			return
		}
		if msg.message == wmHotkey {
			s.mu.Lock()
			bindingID := s.ids[int32(msg.wParam)]
			s.mu.Unlock()
			if bindingID != "" && s.handler != nil {
				s.handler(bindingID)
			}
		}
	}
}

// Register implements Source. It hands the bindings to the worker thread, which
// must be the one calling RegisterHotKey for a NULL-hwnd hotkey.
func (s *winSource) Register(bindings []Binding) (map[string]string, error) {
	if s.closed.Load() {
		return nil, errClosed
	}
	s.regCh <- bindings
	select {
	case g := <-s.resCh:
		return g, nil
	case e := <-s.errCh:
		return nil, e
	}
}

// Close implements Source.
func (s *winSource) Close() error {
	s.once.Do(func() {
		s.closed.Store(true)
		close(s.regCh) // unblock the worker if it never registered
		procPostThreadMsg.Call(uintptr(s.threadID), wmQuit, 0, 0)
	})
	return nil
}
