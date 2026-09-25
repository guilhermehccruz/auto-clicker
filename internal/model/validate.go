package model

import (
	"fmt"
	"strings"
)

// IssueLevel distinguishes a run blocker from an advisory warning.
type IssueLevel string

const (
	Blocking IssueLevel = "blocking"
	Warning  IssueLevel = "warning"
)

// Issue is one validation finding, addressable to a module/field so the UI can
// render it next to the offending control.
type Issue struct {
	Level    IssueLevel `json:"level"`
	ModuleID string     `json:"moduleId,omitempty"`
	Field    string     `json:"field,omitempty"`
	Message  string     `json:"message"`
}

// Desktop is the canonical coordinate space: bounds relative to the primary
// monitor's top-left (so a monitor left/above gives negative coordinates), plus
// the virtual desktop size for the display snapshot.
type Desktop struct {
	MinX, MinY, MaxX, MaxY int
	Width, Height          int
}

// Validate checks p against the rules in DESIGN §5.6. current is the present
// desktop; pass the zero Desktop to skip the geometry warnings.
func Validate(p *Profile, current Desktop) []Issue {
	var issues []Issue
	add := func(l IssueLevel, moduleID, field, format string, a ...any) {
		issues = append(issues, Issue{Level: l, ModuleID: moduleID, Field: field, Message: fmt.Sprintf(format, a...)})
	}

	if p == nil {
		return []Issue{{Level: Blocking, Message: "no profile loaded"}}
	}
	if p.EnabledCount() == 0 {
		add(Blocking, "", "", "No modules enabled — enable at least one to run")
	}
	if p.SchemaVersion > SchemaVersion {
		add(Blocking, "", "schemaVersion", "Profile schema version %d is newer than this app supports (%d)", p.SchemaVersion, SchemaVersion)
	}

	// Loop.
	switch p.Loop.Mode {
	case LoopOnce, LoopCount, LoopForever:
	default:
		add(Blocking, "", "loop.mode", "Unknown loop mode %q", p.Loop.Mode)
	}
	if p.Loop.Mode == LoopCount && p.Loop.Count < 1 {
		add(Blocking, "", "loop.count", "Loop count must be at least 1")
	}
	if p.Loop.LoopDelay.Min < 0 || p.Loop.LoopDelay.Max < 0 {
		add(Blocking, "", "loop.loopDelay", "Loop delay cannot be negative")
	}
	if p.Loop.LoopDelay.Min > p.Loop.LoopDelay.Max {
		add(Blocking, "", "loop.loopDelay", "Loop delay min (%d) is greater than max (%d)", p.Loop.LoopDelay.Min, p.Loop.LoopDelay.Max)
	}

	// Failsafe.
	if p.Failsafe.MaxRuntimeSec < 0 {
		add(Blocking, "", "failsafe.maxRuntimeSec", "Max runtime cannot be negative")
	}

	// Modules. A grouped module is "enabled" only when its group is too.
	checkModule := func(m Module, effectiveEnabled bool) {
		label := m.Label()
		if m.Count < MinCount || m.Count > MaxCount {
			add(Blocking, m.ID, "count", "%s: count must be %d–%d", label, MinCount, MaxCount)
		}
		if m.ClickInterval < MinClickInterval || m.ClickInterval > MaxClickInterval {
			add(Blocking, m.ID, "clickInterval", "%s: interval must be %d–%d ms", label, MinClickInterval, MaxClickInterval)
		}
		if m.HoldMs < MinHoldMs || m.HoldMs > MaxHoldMs {
			add(Blocking, m.ID, "holdMs", "%s: hold must be %d–%d ms", label, MinHoldMs, MaxHoldMs)
		}
		if m.DelayAfter < MinDelayAfter || m.DelayAfter > MaxDelayAfter {
			add(Blocking, m.ID, "delayAfter", "%s: delay after must be %d–%d ms", label, MinDelayAfter, MaxDelayAfter)
		}
		switch m.Button {
		case ButtonLeft, ButtonRight, ButtonMiddle:
		case ButtonNone:
			if m.Target.Kind == TargetCursor {
				add(Warning, m.ID, "button", "%s: move-only does nothing with a cursor target", label)
			}
		default:
			add(Blocking, m.ID, "button", "%s: unknown button %q", label, m.Button)
		}
		switch m.Target.Kind {
		case TargetAbsolute:
			if effectiveEnabled && !m.Target.HasCoords() {
				add(Blocking, m.ID, "target", "%s: absolute target has no coordinates", label)
			}
		case TargetCursor:
		default:
			add(Blocking, m.ID, "target.kind", "%s: unknown target kind %q", label, m.Target.Kind)
		}
		if len(m.Name) > MaxNameLen {
			add(Warning, m.ID, "name", "%s: label is longer than %d characters", label, MaxNameLen)
		}

		// Warnings: off-screen coordinates.
		if effectiveEnabled && m.Target.Kind == TargetAbsolute && m.Target.HasCoords() && current.MaxX > current.MinX && current.MaxY > current.MinY {
			if m.Target.X < current.MinX || m.Target.Y < current.MinY || m.Target.X >= current.MaxX || m.Target.Y >= current.MaxY {
				add(Warning, m.ID, "target", "%s: (%d, %d) is outside the current desktop", label, m.Target.X, m.Target.Y)
			}
		}
	}

	for _, m := range p.Modules {
		checkModule(m, m.Enabled)
	}
	seenGroups := map[string]bool{}
	for _, g := range p.Groups {
		if g.ID == "" {
			add(Blocking, "", "groups", "A group has no id")
		} else if seenGroups[g.ID] {
			add(Blocking, "", "groups", "Duplicate group id %q", g.ID)
		}
		seenGroups[g.ID] = true
		if strings.TrimSpace(g.Name) == "" {
			add(Warning, "", "groups", "A group has no name")
		}
		for _, m := range g.Modules {
			checkModule(m, g.Enabled && m.Enabled)
		}
	}

	// Warning: display snapshot mismatch.
	if current.Width > 0 && current.Height > 0 && p.Display.Width > 0 && p.Display.Height > 0 {
		if p.Display.Width != current.Width || p.Display.Height != current.Height {
			add(Warning, "", "display", "This profile was recorded for %d×%d; the current desktop is %d×%d — verify your coordinates",
				p.Display.Width, p.Display.Height, current.Width, current.Height)
		}
	}
	return issues
}

// HasBlocking reports whether any issue blocks a run.
func HasBlocking(issues []Issue) bool {
	for _, i := range issues {
		if i.Level == Blocking {
			return true
		}
	}
	return false
}

// Label returns the module's user label or a generated summary (DESIGN §5.3).
func (m Module) Label() string {
	if strings.TrimSpace(m.Name) != "" {
		return m.Name
	}
	return m.Summary()
}

// Summary is the generated, always-current one-line description of a module.
func (m Module) Summary() string {
	var b strings.Builder
	if m.Target.Kind == TargetAbsolute {
		fmt.Fprintf(&b, "(%d, %d)", m.Target.X, m.Target.Y)
	} else {
		b.WriteString("at cursor")
	}
	if m.Button == ButtonNone {
		if m.Count > 1 {
			fmt.Fprintf(&b, " move ×%d", m.Count)
		} else {
			b.WriteString(" move only")
		}
		return b.String()
	}
	fmt.Fprintf(&b, " %s ×%d", m.Button, m.Count)
	if m.ClickInterval > 0 {
		fmt.Fprintf(&b, " @%dms", m.ClickInterval)
	}
	fmt.Fprintf(&b, " hold %dms", m.HoldMs)
	return b.String()
}
