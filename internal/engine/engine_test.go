package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"auto-clicker/internal/input"
	"auto-clicker/internal/model"
)

// --- test clocks ----------------------------------------------------------

// autoClock advances instantly and records every requested sleep.
type autoClock struct {
	mu     sync.Mutex
	t      time.Time
	sleeps []time.Duration
}

func newAutoClock() *autoClock { return &autoClock{t: time.Unix(0, 0)} }

func (c *autoClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *autoClock) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.sleeps = append(c.sleeps, d)
	c.mu.Unlock()
	return nil
}

func (c *autoClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func (c *autoClock) Sleeps() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.sleeps...)
}

// gateClock blocks every sleep until the test releases it.
type gateClock struct {
	mu  sync.Mutex
	t   time.Time
	req chan time.Duration
	rel chan struct{}
}

func newGateClock() *gateClock {
	return &gateClock{t: time.Unix(0, 0), req: make(chan time.Duration, 32), rel: make(chan struct{}, 32)}
}

func (c *gateClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *gateClock) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	select {
	case c.req <- d:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-c.rel:
	case <-ctx.Done():
		return ctx.Err()
	}
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
	return nil
}

func (c *gateClock) release() { c.rel <- struct{}{} }

// fixedRng returns a constant, so loopDelay draws are predictable.
type fixedRng struct{ n int }

func (r fixedRng) IntN(n int) int { return r.n % n }

func waitIdle(t *testing.T, e *Engine) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if e.State() == StateIdle {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("engine did not return to idle; state=%s", e.State())
}

func onceProfile(mods ...model.Module) *model.Profile {
	p := model.NewProfile("test", model.Display{Width: 1920, Height: 1080}, 0)
	p.Loop = model.Loop{Mode: model.LoopOnce, Count: 1, LoopDelay: model.DelayRange{Min: 0, Max: 0}}
	p.Modules = mods
	return p
}

// --- tests ----------------------------------------------------------------

func TestFlattenOrderSkipsDisabled(t *testing.T) {
	m1 := model.NewModule()
	m1.Target = model.NewAbsoluteTarget(940, 520)
	m2 := model.NewModule()
	m2.Enabled = false
	m2.Target = model.NewAbsoluteTarget(1, 1)
	m3 := model.NewModule() // cursor

	drv := input.NewFake()
	e := New(drv, WithClock(newAutoClock()))
	if err := e.Run(onceProfile(m1, m2, m3)); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, e)

	clicks := drv.Clicks()
	if len(clicks) != 2 {
		t.Fatalf("want 2 clicks (disabled skipped), got %d: %+v", len(clicks), clicks)
	}
	if clicks[0].Op != "ClickAt" || clicks[0].X != 940 || clicks[0].Y != 520 {
		t.Errorf("first click wrong: %+v", clicks[0])
	}
	if clicks[1].Op != "ClickHere" {
		t.Errorf("second click should be ClickHere: %+v", clicks[1])
	}
	if drv.MoveBeforeClickHere {
		t.Error("cursor click must not be preceded by a Move")
	}
	if drv.ReleaseAllCount == 0 {
		t.Error("ReleaseAll must run on exit")
	}
	if got := e.Status().StopReason; got != ReasonDone {
		t.Errorf("stop reason = %q, want done", got)
	}
}

func TestSpacingMath(t *testing.T) {
	m := model.NewModule()
	m.Target = model.NewAbsoluteTarget(10, 10)
	m.Count = 3
	m.ClickInterval = 90
	m.HoldMs = 10
	m.DelayAfter = 250

	clk := newAutoClock()
	drv := input.NewFake()
	drv.OnHold = clk.Advance
	e := New(drv, WithClock(clk))
	if err := e.Run(onceProfile(m)); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, e)

	// clickInterval is additive: after each click (hold 10) we wait the full
	// 90 ms, then the 250 ms delayAfter.
	want := []time.Duration{90 * time.Millisecond, 90 * time.Millisecond, 250 * time.Millisecond}
	got := clk.Sleeps()
	if len(got) != len(want) {
		t.Fatalf("sleeps = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sleep[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestLoopDelayNotAfterLastPass(t *testing.T) {
	m := model.NewModule()
	m.HoldMs = 0
	m.DelayAfter = 0

	p := model.NewProfile("test", model.Display{}, 0)
	p.Loop = model.Loop{Mode: model.LoopCount, Count: 2, LoopDelay: model.DelayRange{Min: 50, Max: 50}}
	p.Modules = []model.Module{m}

	clk := newAutoClock()
	e := New(NewFakeDriver(clk), WithClock(clk))
	if err := e.Run(p); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, e)

	got := clk.Sleeps()
	if len(got) != 1 || got[0] != 50*time.Millisecond {
		t.Fatalf("want exactly one 50ms inter-pass delay, got %v", got)
	}
	if e.Status().LoopIteration != 2 {
		t.Errorf("iterations = %d, want 2", e.Status().LoopIteration)
	}
}

func NewFakeDriver(clk *autoClock) *input.FakeDriver {
	d := input.NewFake()
	d.OnHold = clk.Advance
	return d
}

func TestLoopDelayDrawUniformInclusive(t *testing.T) {
	e := New(input.NewFake())
	dr := model.DelayRange{Min: 500, Max: 1500}
	e.rng = fixedRng{0}
	if got := e.draw(dr); got != 500*time.Millisecond {
		t.Errorf("min draw = %v, want 500ms", got)
	}
	e.rng = fixedRng{1000}
	if got := e.draw(dr); got != 1500*time.Millisecond {
		t.Errorf("max draw = %v, want 1500ms", got)
	}
}

func TestStopBeforeNextClick(t *testing.T) {
	m := model.NewModule()
	m.Count = 5
	m.ClickInterval = 100
	m.HoldMs = 0
	m.DelayAfter = 0

	clk := newGateClock()
	drv := input.NewFake()
	e := New(drv, WithClock(clk))
	if err := e.Run(onceProfile(m)); err != nil {
		t.Fatal(err)
	}
	<-clk.req // engine is sleeping between clicks
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	clk.release()
	waitIdle(t, e)

	if got := len(drv.Clicks()); got != 1 {
		t.Errorf("clicks = %d, want 1 (stop before next click)", got)
	}
	if got := e.Status().StopReason; got != ReasonUser {
		t.Errorf("stop reason = %q, want user", got)
	}
	if drv.ReleaseAllCount == 0 {
		t.Error("ReleaseAll must run on stop")
	}
}

func TestPanicReleasesWithoutTrailingClick(t *testing.T) {
	m := model.NewModule()
	m.Count = 5
	m.ClickInterval = 100
	m.HoldMs = 0
	m.DelayAfter = 0

	clk := newGateClock()
	drv := input.NewFake()
	e := New(drv, WithClock(clk))
	if err := e.Run(onceProfile(m)); err != nil {
		t.Fatal(err)
	}
	<-clk.req
	if err := e.Panic(); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, e)

	if got := len(drv.Clicks()); got != 1 {
		t.Errorf("clicks = %d, want 1 (no trailing click)", got)
	}
	if got := e.Status().StopReason; got != ReasonPanic {
		t.Errorf("stop reason = %q, want panic", got)
	}
	if drv.ReleaseAllCount == 0 {
		t.Error("ReleaseAll must run on panic")
	}
}

func TestPauseGatesAndResumes(t *testing.T) {
	m := model.NewModule()
	m.Count = 2
	m.ClickInterval = 100
	m.HoldMs = 0
	m.DelayAfter = 0

	clk := newGateClock()
	drv := input.NewFake()
	e := New(drv, WithClock(clk))
	if err := e.Run(onceProfile(m)); err != nil {
		t.Fatal(err)
	}
	<-clk.req
	if err := e.Pause(); err != nil {
		t.Fatal(err)
	}
	clk.release() // finish the in-flight delay

	// The engine must now be gated at the next click boundary.
	time.Sleep(30 * time.Millisecond)
	if got := len(drv.Clicks()); got != 1 {
		t.Fatalf("clicks while paused = %d, want 1", got)
	}
	if e.State() != StatePaused {
		t.Fatalf("state = %s, want paused", e.State())
	}
	if err := e.Resume(); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, e)
	if got := len(drv.Clicks()); got != 2 {
		t.Errorf("clicks after resume = %d, want 2", got)
	}
}

func TestInjectionErrorAborts(t *testing.T) {
	m := model.NewModule()
	m.HoldMs = 0
	m.DelayAfter = 0
	drv := input.NewFake()
	drv.InjectErr = errors.New("backend boom")
	e := New(drv, WithClock(newAutoClock()))
	if err := e.Run(onceProfile(m)); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, e)
	st := e.Status()
	if st.StopReason != ReasonError {
		t.Errorf("stop reason = %q, want error", st.StopReason)
	}
	if st.Error == "" {
		t.Error("expected an error message")
	}
	if drv.ReleaseAllCount == 0 {
		t.Error("ReleaseAll must run after an error")
	}
}

func TestMaxRuntimeStops(t *testing.T) {
	m := model.NewModule()
	m.Count = 100
	m.ClickInterval = 1000
	m.HoldMs = 0
	m.DelayAfter = 0

	p := onceProfile(m)
	p.Failsafe.MaxRuntimeSec = 1

	clk := newAutoClock()
	e := New(NewFakeDriver(clk), WithClock(clk))
	if err := e.Run(p); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, e)
	if got := e.Status().StopReason; got != ReasonMaxRuntime {
		t.Errorf("stop reason = %q, want maxRuntime", got)
	}
}

func TestMoveOnlyModuleMovesWithoutClicking(t *testing.T) {
	m := model.NewModule()
	m.Button = model.ButtonNone
	m.Target = model.NewAbsoluteTarget(300, 400)
	m.Count = 2
	m.HoldMs = 0
	m.DelayAfter = 0

	drv := input.NewFake()
	e := New(drv, WithClock(newAutoClock()))
	if err := e.Run(onceProfile(m)); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, e)

	if got := len(drv.Clicks()); got != 0 {
		t.Errorf("move-only must not click, got %d clicks", got)
	}
	moves := 0
	for _, c := range drv.Calls {
		if c.Op == "Move" {
			moves++
			if c.X != 300 || c.Y != 400 {
				t.Errorf("Move to (%d,%d), want (300,400)", c.X, c.Y)
			}
		}
	}
	if moves != 2 {
		t.Errorf("moves = %d, want 2", moves)
	}
	if e.Status().Clicks != 0 {
		t.Errorf("progress clicks = %d, want 0", e.Status().Clicks)
	}
}

func TestGroupDisablesItsModules(t *testing.T) {
	off := model.NewGroup("City")
	off.Enabled = false
	m1 := model.NewModule()
	m1.Target = model.NewAbsoluteTarget(1, 1)
	off.Modules = []model.Module{m1}

	on := model.NewGroup("Adventure")
	m2 := model.NewModule()
	m2.Target = model.NewAbsoluteTarget(2, 2)
	on.Modules = []model.Module{m2}

	p := onceProfile()
	p.Modules = nil
	p.Groups = []model.Group{off, on}

	drv := input.NewFake()
	e := New(drv, WithClock(newAutoClock()))
	if err := e.Run(p); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, e)

	clicks := drv.Clicks()
	if len(clicks) != 1 || clicks[0].X != 2 || clicks[0].Y != 2 {
		t.Fatalf("want only the enabled group's click at (2,2), got %+v", clicks)
	}
}

func TestRunRefusesWithoutEnabledModules(t *testing.T) {
	m := model.NewModule()
	m.Enabled = false
	p := onceProfile(m)
	e := New(input.NewFake())
	if err := e.Run(p); !errors.Is(err, ErrNoModules) {
		t.Errorf("Run err = %v, want ErrNoModules", err)
	}
	if e.State() != StateIdle {
		t.Errorf("state after refusal = %s, want idle", e.State())
	}
}
