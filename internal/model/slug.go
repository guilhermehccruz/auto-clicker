package model

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	slugFallback = "profile"
	slugMaxLen   = 64
)

// Slugify converts a profile name into a filename stem (see DESIGN §3.8):
// lowercase, diacritics folded away, every run of non-[a-z0-9] replaced by "-",
// edges trimmed, truncated to 64 chars. An empty result falls back to "profile".
func Slugify(name string) string {
	folded := fold(name)
	var b strings.Builder
	lastDash := false
	for _, r := range folded {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > slugMaxLen {
		s = strings.Trim(s[:slugMaxLen], "-")
	}
	if s == "" {
		return slugFallback
	}
	return s
}

func fold(s string) string {
	s = strings.ToLower(s)
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return out
}

// UniqueSlug returns base if free, otherwise base-2, base-3, … where free is
// supplied by the caller (it owns the filesystem view).
func UniqueSlug(base string, exists func(string) bool) string {
	if base == "" {
		base = slugFallback
	}
	if !exists(base) {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !exists(candidate) {
			return candidate
		}
	}
}

// DuplicateName returns the name for a duplicated profile: "X copy", then
// "X copy 2", … where taken reports whether a name is already in use.
func DuplicateName(name string, taken func(string) bool) string {
	base := name + " copy"
	if !taken(base) {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s copy %d", name, i)
		if !taken(candidate) {
			return candidate
		}
	}
}
