//go:build !linux || android || server

package desktop

// Ensure is a no-op outside Linux. The other platforms integrate through the
// packaged binary rather than a user-level launcher entry.
func Ensure() error { return nil }
