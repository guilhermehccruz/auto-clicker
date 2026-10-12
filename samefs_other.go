//go:build !linux

package main

// sameFSTempDir is a no-op outside Linux: the Windows swap helper already
// falls back to a copy when the staging file is on another drive.
func sameFSTempDir() func() { return func() {} }
