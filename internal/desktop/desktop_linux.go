//go:build linux && !android && !server

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Ensure writes the .desktop entry and icon under the user's XDG data
// directory. It is idempotent and best-effort: any error is returned for the
// caller to log, never fatal.
func Ensure() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dataDir = filepath.Join(home, ".local", "share")
	}

	appsDir := filepath.Join(dataDir, "applications")
	iconPath := filepath.Join(dataDir, "icons", "hicolor", "256x256", "apps", appID+".png")
	desktopPath := filepath.Join(appsDir, appID+".desktop")

	iconChanged, err := writeIfChanged(iconPath, iconPNG)
	if err != nil {
		return err
	}
	entryChanged, err := writeIfChanged(desktopPath, []byte(entry(exe)))
	if err != nil {
		return err
	}
	if iconChanged || entryChanged {
		refreshCaches(appsDir)
	}
	return nil
}

// refreshCaches rebuilds the desktop database so the new entry is picked up
// immediately. Best-effort: the tool may be absent.
func refreshCaches(appsDir string) {
	_ = exec.Command("update-desktop-database", appsDir).Run()
}
