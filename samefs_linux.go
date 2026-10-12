//go:build linux

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

// sameFSTempDir points TMPDIR at the executable's directory when the current
// temp dir is on a different filesystem, so the updater's staging file and the
// binary share a device. The Unix swap helper replaces the binary with
// os.Rename, which fails with EXDEV across mounts (e.g. tmpfs /tmp and ext4
// $HOME). It returns a function that restores TMPDIR.
func sameFSTempDir() func() {
	noop := func() {}
	exe, err := os.Executable()
	if err != nil {
		return noop
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)

	device := func(p string) (uint64, bool) {
		var st syscall.Stat_t
		if err := syscall.Stat(p, &st); err != nil {
			return 0, false
		}
		return uint64(st.Dev), true
	}
	tmpDev, okTmp := device(os.TempDir())
	dirDev, okDir := device(dir)
	if !okTmp || !okDir || tmpDev == dirDev {
		return noop
	}

	old, had := os.LookupEnv("TMPDIR")
	if err := os.Setenv("TMPDIR", dir); err != nil {
		return noop
	}
	return func() {
		if had {
			_ = os.Setenv("TMPDIR", old)
		} else {
			_ = os.Unsetenv("TMPDIR")
		}
	}
}
