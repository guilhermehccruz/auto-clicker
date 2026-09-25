//go:build linux

package hotkey

import (
	"log"
	"os"
	"strings"
)

// New returns the best available global-hotkey source for this session:
// the GlobalShortcuts portal on Wayland, XGrabKey on X11, and XGrabKey as the
// Wayland fallback (it fires while an XWayland client has focus — Appendix B.9).
func New(h Handler) Source {
	var reasons []string
	if os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		if s, err := newPortal(PortalAppID, h); err == nil {
			log.Printf("hotkey: using GlobalShortcuts portal")
			FallbackReason = ""
			return s
		} else {
			reasons = append(reasons, "portal: "+err.Error())
			log.Printf("hotkey: portal unavailable: %v", err)
		}
	}
	if s, err := newX11(h); err == nil {
		log.Printf("hotkey: using XGrabKey (fires only while an XWayland client is focused)")
		FallbackReason = strings.Join(reasons, "; ")
		return s
	} else {
		reasons = append(reasons, "x11: "+err.Error())
	}
	FallbackReason = strings.Join(reasons, "; ")
	log.Printf("hotkey: no global source: %s", FallbackReason)
	return Unavailable{Reason: FallbackReason}
}
