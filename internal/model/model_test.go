package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnknownFieldsPreserved(t *testing.T) {
	in := `{
	  "schemaVersion": 1,
	  "name": "Idle Tycoon",
	  "futureTop": {"a": 1},
	  "display": {"width": 5360, "height": 1440, "futureDisplay": true},
	  "loop": {"mode": "forever", "count": 10, "loopDelay": {"min": 500, "max": 1500}, "futureLoop": "x"},
	  "failsafe": {"maxRuntimeSec": 3600, "mouseToCornerStops": false, "futureFailsafe": 1},
	  "modules": [
	    {"kind": "click", "id": "m1", "enabled": true, "delayAfter": 250,
	     "target": {"kind": "absolute", "x": 940, "y": 520, "futureTarget": 1},
	     "button": "left", "count": 3, "clickInterval": 90, "holdMs": 10, "futureModule": [1,2]}
	  ]
	}`
	var p Profile
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(&p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	for _, key := range []string{"futureTop", "futureDisplay", "futureLoop", "futureFailsafe", "futureTarget", "futureModule"} {
		if !strings.Contains(s, key) {
			t.Errorf("unknown key %q lost on round-trip: %s", key, s)
		}
	}
	// encode -> decode -> encode must be stable.
	var p2 Profile
	if err := json.Unmarshal(out, &p2); err != nil {
		t.Fatalf("second unmarshal: %v", err)
	}
	out2, err := json.Marshal(&p2)
	if err != nil {
		t.Fatalf("second marshal: %v", err)
	}
	if string(out) != string(out2) {
		t.Errorf("round-trip not stable:\n%s\n%s", out, out2)
	}
}

func TestDefaultsForAbsentFields(t *testing.T) {
	// A minimal module: only an id. Everything else must take safe defaults.
	var m Module
	if err := json.Unmarshal([]byte(`{"id":"m1"}`), &m); err != nil {
		t.Fatal(err)
	}
	if m.Kind != KindClick || !m.Enabled || m.Button != ButtonLeft {
		t.Errorf("kind/enabled/button defaults wrong: %+v", m)
	}
	if m.Target.Kind != TargetCursor {
		t.Errorf("target default = %q, want cursor", m.Target.Kind)
	}
	if m.Count != 1 || m.HoldMs != DefaultHoldMs || m.DelayAfter != DefaultDelayAfter {
		t.Errorf("numeric defaults wrong: %+v", m)
	}
}

func TestAbsoluteZeroCoordsRoundTrip(t *testing.T) {
	// A legitimate (0,0) absolute target must survive a round-trip with its
	// coordinates present.
	m := NewModule()
	m.Target = NewAbsoluteTarget(0, 0)
	b, err := json.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}
	var back Module
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Target.HasCoords() {
		t.Fatalf("coords lost: %s", b)
	}
	if back.Target.Kind != TargetAbsolute || back.Target.X != 0 || back.Target.Y != 0 {
		t.Fatalf("target wrong: %+v", back.Target)
	}
}

func TestCursorTargetOmitsCoords(t *testing.T) {
	m := NewModule()
	b, err := json.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"x"`) || strings.Contains(string(b), `"y"`) {
		t.Errorf("cursor target should not carry coordinates: %s", b)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Idle Tycoon":    "idle-tycoon",
		"  spaced  ":     "spaced",
		"Café Com Leite": "cafe-com-leite",
		"日本語":            "profile",
		"":               "profile",
		"A/B:C":          "a-b-c",
		"--x--":          "x",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("a", 200)
	if got := Slugify(long); len(got) > 64 {
		t.Errorf("slug too long: %d", len(got))
	}
}

func TestUniqueSlug(t *testing.T) {
	taken := map[string]bool{"farm": true, "farm-2": true}
	got := UniqueSlug("farm", func(s string) bool { return taken[s] })
	if got != "farm-3" {
		t.Errorf("UniqueSlug = %q, want farm-3", got)
	}
}

func TestDuplicateName(t *testing.T) {
	taken := map[string]bool{"Farm copy": true}
	got := DuplicateName("Farm", func(s string) bool { return taken[s] })
	if got != "Farm copy 2" {
		t.Errorf("DuplicateName = %q, want Farm copy 2", got)
	}
}

func TestValidateBlockingAndWarnings(t *testing.T) {
	desk := Desktop{MinX: 0, MinY: 0, MaxX: 100, MaxY: 100, Width: 100, Height: 100}
	p := NewProfile("t", Display{Width: 100, Height: 100}, DefaultMaxRuntimeSec)
	if got := Validate(p, desk); !HasBlocking(got) {
		t.Errorf("empty profile should block (no enabled modules): %+v", got)
	}

	m := NewModule()
	p.Modules = []Module{m}
	p.Loop = Loop{Mode: LoopCount, Count: 0, LoopDelay: DefaultLoopDelay()}
	issues := Validate(p, desk)
	if !HasBlocking(issues) {
		t.Errorf("count loop with count 0 should block: %+v", issues)
	}

	p.Loop = DefaultLoop()
	m.Target = NewAbsoluteTarget(500, 500) // off-screen warning only
	p.Modules = []Module{m}
	issues = Validate(p, desk)
	if HasBlocking(issues) {
		t.Errorf("off-screen should warn, not block: %+v", issues)
	}
	found := false
	for _, i := range issues {
		if i.Level == Warning && i.ModuleID == m.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an off-screen warning: %+v", issues)
	}
}

func TestValidateAbsoluteMissingCoords(t *testing.T) {
	p := NewProfile("t", Display{}, DefaultMaxRuntimeSec)
	m := NewModule()
	m.Target = Target{Kind: TargetAbsolute} // no coordinates
	p.Modules = []Module{m}
	issues := Validate(p, Desktop{})
	if !HasBlocking(issues) {
		t.Errorf("absolute target without coords should block: %+v", issues)
	}
}

func TestGroupsGating(t *testing.T) {
	p := NewProfile("t", Display{}, DefaultMaxRuntimeSec)
	un := NewModule()
	disabled := NewGroup("City")
	disabled.Enabled = false
	gm := NewModule()
	disabled.Modules = []Module{gm}
	enabled := NewGroup("Adventure")
	em := NewModule()
	enabled.Modules = []Module{em}
	p.Modules = []Module{un}
	p.Groups = []Group{disabled, enabled}

	got := p.EnabledModules()
	if len(got) != 2 || got[0].ID != un.ID || got[1].ID != em.ID {
		t.Fatalf("EnabledModules = %d, want ungrouped + enabled group", len(got))
	}
	if p.EnabledCount() != 2 {
		t.Errorf("EnabledCount = %d, want 2", p.EnabledCount())
	}
	if p.TotalModules() != 3 {
		t.Errorf("TotalModules = %d, want 3", p.TotalModules())
	}

	// Only modules inside a disabled group -> run must be blocked.
	p.Modules = nil
	p.Groups = []Group{disabled}
	if !HasBlocking(Validate(p, Desktop{})) {
		t.Error("only-disabled-group profile should block")
	}
}

func TestGroupRoundTrip(t *testing.T) {
	in := `{"schemaVersion":1,"name":"t","modules":[],"groups":[{"id":"g1","name":"A","enabled":false,"modules":[{"id":"m1"}],"futureGroup":1}]}`
	var p Profile
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Groups) != 1 || p.Groups[0].Enabled || p.Groups[0].Name != "A" {
		t.Fatalf("group parse wrong: %+v", p.Groups)
	}
	out, _ := json.Marshal(&p)
	if !strings.Contains(string(out), "futureGroup") {
		t.Errorf("unknown group key lost: %s", out)
	}
	// Absent enabled defaults to true.
	var p2 Profile
	if err := json.Unmarshal([]byte(`{"name":"x","groups":[{"id":"g1","modules":[]}]}`), &p2); err != nil {
		t.Fatal(err)
	}
	if !p2.Groups[0].Enabled {
		t.Error("group enabled should default true")
	}
}

func TestSummaryAndLabel(t *testing.T) {
	m := NewModule()
	m.Target = NewAbsoluteTarget(940, 520)
	m.Count = 3
	m.ClickInterval = 90
	if got := m.Summary(); got != "(940, 520) left ×3 @90ms hold 10ms" {
		t.Errorf("Summary = %q", got)
	}
	m.Name = "Buy"
	if m.Label() != "Buy" {
		t.Errorf("Label should prefer name")
	}
}
