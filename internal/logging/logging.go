// Package logging sends the app's log to a file in the app folder. This is
// essential on Windows, where the GUI binary (-H windowsgui) has no console.
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

const (
	maxBytes = 1 << 20 // 1 MiB
	keep     = 3
)

// Path returns the log file path inside appDir (whether or not it exists).
func Path(appDir string) string {
	return filepath.Join(appDir, "app.log")
}

// Setup redirects the standard logger to <appDir>/app.log (and to stderr on
// non-Windows), rotating at 1 MiB with `keep` files. It returns the log path.
func Setup(appDir string) (string, error) {
	path := Path(appDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	rotate(path)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	var w io.Writer = f
	if runtime.GOOS != "windows" {
		w = io.MultiWriter(os.Stderr, f)
	}
	log.SetOutput(w)
	log.SetFlags(log.LstdFlags)
	return path, nil
}

func rotate(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxBytes {
		return
	}
	for i := keep - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", path, i), fmt.Sprintf("%s.%d", path, i+1))
	}
	_ = os.Rename(path, path+".1")
}
