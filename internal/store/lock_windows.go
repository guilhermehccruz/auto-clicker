//go:build windows

package store

import (
	"errors"

	"golang.org/x/sys/windows"
)

// ErrLocked is returned when another instance already holds the lock.
var ErrLocked = errors.New("store: another instance is already running")

// Lock is a single-instance named mutex. The OS releases it if the process
// dies, so a stale lock cannot wedge the app.
type Lock struct{ h windows.Handle }

// AcquireLock creates the named mutex, reporting ErrLocked if it already
// exists.
func AcquireLock(_ string) (*Lock, error) {
	name, err := windows.UTF16PtrFromString(`Global\auto-clicker-single-instance`)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return nil, ErrLocked
		}
		return nil, err
	}
	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(h)
		return nil, ErrLocked
	}
	return &Lock{h: h}, nil
}

// Release drops the mutex.
func (l *Lock) Release() error {
	if l == nil || l.h == 0 {
		return nil
	}
	err := windows.CloseHandle(l.h)
	l.h = 0
	return err
}
