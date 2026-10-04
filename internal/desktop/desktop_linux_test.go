//go:build linux && !android && !server

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureWritesEntryAndIcon(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	if err := Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	desktopPath := filepath.Join(dir, "applications", "auto-clicker.desktop")
	got, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("read .desktop: %v", err)
	}
	for _, want := range []string{"Name=Auto Clicker", "Icon=auto-clicker", "StartupWMClass=auto-clicker"} {
		if !strings.Contains(string(got), want) {
			t.Errorf(".desktop missing %q:\n%s", want, got)
		}
	}

	iconPath := filepath.Join(dir, "icons", "hicolor", "256x256", "apps", "auto-clicker.png")
	if _, err := os.Stat(iconPath); err != nil {
		t.Fatalf("icon not written: %v", err)
	}

	// A second run must be idempotent and must not error.
	if err := Ensure(); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
}
