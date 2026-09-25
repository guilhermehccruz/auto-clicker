// Command phase05 is the Phase 0.5 blocking spike for the Auto Clicker design
// (see ../../DESIGN.md, "Phase 0.5"). It probes the robotgo pure-Go sub-packages
// that the app will actually ship: robotgo/x11 and robotgo/libei.
//
// Safety: every subcommand that injects input or opens a portal session requires
// the explicit -yes flag. Read-only geometry/cursor queries do not. Default (no
// subcommand) prints usage and exits without touching the portal.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-vgo/robotgo/libei"
	"github.com/go-vgo/robotgo/x11"
)

func main() {
	yes := flag.Bool("yes", false, "acknowledge that this may move the real pointer, inject clicks, or open a portal consent dialog")
	linkSC := flag.Bool("link-screencast", false, "libei: link a ScreenCast monitor source (extra 'share screen' consent; required for absolute motion and geometry)")
	count := flag.Int("count", 1, "number of clicks for *-clickhere")
	holdMs := flag.Int("hold", 10, "button-down duration in ms for *-clickhere")
	button := flag.String("button", "left", "left|right|middle")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
		os.Exit(2)
	}

	switch flag.Arg(0) {
	case "read":
		readOnly()
	case "x11-clickhere":
		guard(yes)
		x11ClickHere(*count, *holdMs, *button)
	case "x11-click":
		guard(yes)
		if flag.NArg() < 3 {
			fatalf("x11-click needs X Y")
		}
		x11Click(atoi(flag.Arg(1)), atoi(flag.Arg(2)), *holdMs, *button)
	case "libei-info":
		guard(yes)
		libei.LinkScreenCast = *linkSC
		libeiInfo()
	case "libei-clickhere":
		guard(yes)
		libei.LinkScreenCast = *linkSC
		libeiClickHere(*count, *holdMs, *button)
	case "libei-move":
		guard(yes)
		libei.LinkScreenCast = *linkSC
		if flag.NArg() < 3 {
			fatalf("libei-move needs X Y")
		}
		libeiMove(atoi(flag.Arg(1)), atoi(flag.Arg(2)))
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `phase05 - Phase 0.5 spike for Auto Clicker

  read                       x11 read-only: screen size, displays, cursor, scale
  x11-clickhere              XTEST button down/up at the CURRENT pointer (no motion)
  x11-click X Y              XTEST move to X,Y then click
  libei-info                 open the RemoteDesktop portal and report geometry/caps
  libei-clickhere            libei button down/up at the CURRENT pointer (no motion)
  libei-move X Y             libei move to X,Y (absolute with -link-screencast, else relative)

Flags:
  -yes                       required for any injecting/portal subcommand
  -link-screencast           libei: also request a ScreenCast monitor source
  -count N -hold MS -button left|right|middle

Examples:
  go run . read
  go run . -yes -link-screencast libei-info
  go run . -yes x11-clickhere -count 3 -hold 20
`)
}

func guard(yes *bool) {
	if !*yes {
		fatalf("refusing to inject input or open a portal session without -yes")
	}
}

func fatalf(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "phase05: "+f+"\n", a...)
	os.Exit(1)
}

func atoi(s string) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		fatalf("bad integer %q", s)
	}
	return n
}

func readOnly() {
	fmt.Println("== x11 (read-only) ==")
	w, h := x11.GetScreenSize()
	fmt.Printf("GetScreenSize       = %d x %d\n", w, h)
	fmt.Printf("DisplaysNum         = %d\n", x11.DisplaysNum())
	fmt.Printf("MainDisplayID       = %d\n", x11.MainDisplayID())
	fmt.Printf("ScaleX (dpi)        = %d\n", x11.ScaleX())
	for i := 0; i < x11.DisplaysNum(); i++ {
		r := x11.GetScreenRect(i)
		sw, sh := x11.GetScaleSize(i)
		fmt.Printf("  display[%d] rect   = x=%d y=%d w=%d h=%d (scaleSize %dx%d)\n",
			i, r.X, r.Y, r.W, r.H, sw, sh)
	}
	x, y := x11.Location()
	fmt.Printf("Location (real cur) = %d, %d\n", x, y)
}

func x11ClickHere(count, holdMs int, btn string) {
	fmt.Printf("x11: ClickHere x%d hold=%dms button=%s at the current pointer\n", count, holdMs, btn)
	time.Sleep(2 * time.Second)
	for i := 0; i < count; i++ {
		if err := x11.MouseDown(btn); err != nil {
			fatalf("MouseDown: %v", err)
		}
		time.Sleep(time.Duration(holdMs) * time.Millisecond)
		if err := x11.MouseUp(btn); err != nil {
			fatalf("MouseUp: %v", err)
		}
	}
	x, y := x11.Location()
	fmt.Printf("done. pointer still at %d, %d (should be unchanged)\n", x, y)
}

func x11Click(x, y, holdMs int, btn string) {
	fmt.Printf("x11: move to %d,%d then click hold=%dms button=%s\n", x, y, holdMs, btn)
	time.Sleep(2 * time.Second)
	x11.Move(x, y)
	if err := x11.MouseDown(btn); err != nil {
		fatalf("MouseDown: %v", err)
	}
	time.Sleep(time.Duration(holdMs) * time.Millisecond)
	if err := x11.MouseUp(btn); err != nil {
		fatalf("MouseUp: %v", err)
	}
	gx, gy := x11.Location()
	fmt.Printf("done. pointer at %d, %d (expected %d, %d)\n", gx, gy, x, y)
}

func libeiInfo() {
	fmt.Printf("== libei (RemoteDesktop portal), LinkScreenCast=%v ==\n", libei.LinkScreenCast)
	start := time.Now()
	w, h := libei.GetScreenSize()
	fmt.Printf("GetScreenSize       = %d x %d   (0x0 without a linked ScreenCast stream)\n", w, h)
	fmt.Printf("DisplaysNum         = %d\n", libei.DisplaysNum())
	for i := 0; i < libei.DisplaysNum(); i++ {
		r := libei.GetScreenRect(i)
		sw, sh := libei.GetScaleSize(i)
		fmt.Printf("  display[%d] rect   = x=%d y=%d w=%d h=%d (scaleSize %dx%d)\n",
			i, r.X, r.Y, r.W, r.H, sw, sh)
	}
	lx, ly := libei.Location()
	fmt.Printf("Location (injected) = %d, %d   (NOT the real cursor)\n", lx, ly)
	fmt.Printf("init+queries took   = %s\n", time.Since(start))
}

func libeiClickHere(count, holdMs int, btn string) {
	fmt.Printf("libei: ClickHere x%d hold=%dms button=%s at the current pointer\n", count, holdMs, btn)
	time.Sleep(3 * time.Second)
	for i := 0; i < count; i++ {
		if err := libei.MouseDown(btn); err != nil {
			fatalf("MouseDown: %v", err)
		}
		time.Sleep(time.Duration(holdMs) * time.Millisecond)
		if err := libei.MouseUp(btn); err != nil {
			fatalf("MouseUp: %v", err)
		}
	}
	fmt.Println("done. VERIFY the click landed at the real pointer, not at 0,0.")
}

func libeiMove(x, y int) {
	fmt.Printf("libei: move to %d,%d (LinkScreenCast=%v)\n", x, y, libei.LinkScreenCast)
	time.Sleep(2 * time.Second)
	libei.Move(x, y)
	lx, ly := libei.Location()
	fmt.Printf("done. libei tracks position as %d, %d\n", lx, ly)
}
