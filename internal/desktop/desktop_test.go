package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEntry(t *testing.T) {
	got := entry("/opt/Auto Clicker/auto-clicker")

	for _, want := range []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Auto Clicker",
		"Icon=auto-clicker",
		"Terminal=false",
		"Categories=Utility;",
		`Exec="/opt/Auto Clicker/auto-clicker"`,
		"StartupWMClass=auto-clicker",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("entry() missing %q:\n%s", want, got)
		}
	}
}

func TestQuoteExec(t *testing.T) {
	cases := map[string]string{
		"/usr/bin/auto-clicker":        `"/usr/bin/auto-clicker"`,
		"/opt/my app/auto-clicker":     `"/opt/my app/auto-clicker"`,
		`/opt/a"b/auto-clicker`:        `"/opt/a\"b/auto-clicker"`,
		`/opt/back\slash/auto-clicker`: `"/opt/back\\slash/auto-clicker"`,
	}
	for in, want := range cases {
		if got := quoteExec(in); got != want {
			t.Errorf("quoteExec(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWriteIfChanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "auto-clicker.desktop")

	changed, err := writeIfChanged(path, []byte("first"))
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if !changed {
		t.Fatal("first write reported no change")
	}

	changed, err = writeIfChanged(path, []byte("first"))
	if err != nil {
		t.Fatalf("idempotent write: %v", err)
	}
	if changed {
		t.Fatal("identical write reported a change")
	}

	changed, err = writeIfChanged(path, []byte("second"))
	if err != nil {
		t.Fatalf("update write: %v", err)
	}
	if !changed {
		t.Fatal("changed content reported no change")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("content = %q, want %q", got, "second")
	}
}
