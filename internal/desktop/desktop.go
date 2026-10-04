// Package desktop installs a Linux launcher entry (a .desktop file and icon)
// so the app shows up in the desktop's application search without an installer.
//
// It also satisfies the GlobalShortcuts portal requirement from DESIGN
// Appendix B.4: for an unsandboxed app, `org.freedesktop.host.portal.Registry.
// Register` fails with "App info not found for '<id>'" unless
// `~/.local/share/applications/<id>.desktop` exists.
//
// On non-Linux platforms Ensure is a no-op.
package desktop

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// appID is the portal application id (DESIGN §2.4). It is also the .desktop
// basename and the value GTK reports as the program name, so the running window
// groups with the launcher entry.
const appID = "auto-clicker"

//go:embed icon.png
var iconPNG []byte

// entry renders the .desktop file for a binary at execPath.
func entry(execPath string) string {
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Version=1.0\n")
	b.WriteString("Name=Auto Clicker\n")
	b.WriteString("GenericName=Autoclicker\n")
	b.WriteString("Comment=Single-purpose autoclicker for repetitive clicking\n")
	fmt.Fprintf(&b, "Exec=%s\n", quoteExec(execPath))
	b.WriteString("Icon=" + appID + "\n")
	b.WriteString("Terminal=false\n")
	b.WriteString("Categories=Utility;\n")
	b.WriteString("Keywords=autoclicker;clicker;idle;automation;\n")
	b.WriteString("StartupNotify=false\n")
	b.WriteString("StartupWMClass=" + appID + "\n")
	return b.String()
}

// quoteExec quotes a path for the Exec key. The field is split on spaces, so an
// absolute path with spaces must be quoted; embedded quotes and backslashes are
// escaped as the spec requires.
func quoteExec(path string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`)
	return `"` + replacer.Replace(path) + `"`
}

// writeIfChanged writes data to path, creating parent directories, and reports
// whether the file changed. Leaving an identical file untouched avoids churning
// the mtime on every launch (which some launchers watch).
func writeIfChanged(path string, data []byte) (bool, error) {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(data) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}
