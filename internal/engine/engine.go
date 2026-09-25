// Package engine runs a profile: snapshot → flatten → schedule → execute.
// All timing goes through an injectable Clock and all randomness through an
// injectable Rng, so runs are deterministic under test.
package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	"auto-clicker/internal/input"
	"auto-clicker/internal/model"
)

// State is the runner state.
type State string

const (
	StateIdle     State = "idle"
	StateRunning  State = "running"
	StatePaused   State = "paused"
	StateStopping State = "stopping"
)

// Stop reasons (Progress.StopReason).
const (
	ReasonDone       = "done"
	ReasonUser       = "user"
	ReasonPanic      = "panic"
	ReasonMaxRuntime = "maxRuntime"
	ReasonError      = "error"
)

// Errors returned by Run.
var (
	ErrBusy      = errors.New("engine: already running")
	ErrNoModules = errors.New("engine: no enabled modules")
)

// Progress is the event payload pushed to the UI (DESIGN §4.2).
type Progress struct {
	State         State  `json:"state"`
	ProfileName   string `json:"profileName"`
	ModuleID      string `json:"moduleId"`
	ModuleLabel   string `json:"moduleLabel"`
	ModuleIndex   int    `json:"moduleIndex"`
	ModuleTotal   int    `json:"moduleTotal"`
	ClickIndex    int    `json:"clickIndex"`
	ClickTotal    int    `json:"clickTotal"`
	LoopIteration int    `json:"loopIteration"`
	Clicks        int64  `json:"clicks"`
	ElapsedMs     int64  `json:"elapsedMs"`
	StopReason    string `json:"stopReason,omitempty"`
	Error         string `json:"error,omitempty"`
}

// Rng is the randomness source for loopDelay windows.
type Rng interface{ IntN(n int) int }

// Clock abstracts time so tests can run instantly and deterministically.
type Clock interface {
	Now() time.Time
	// Sleep blocks for d, returning ctx.Err() if cancelled first. d <= 0
	// returns immediately.
	Sleep(ctx context.Context, d time.Duration) error
}

// RealClock is the production Clock.
type RealClock struct{}

// Now implements Clock.
func (RealClock) Now() time.Time { return time.Now() }

// Sleep implements Clock with an interruptible timer.
func (RealClock) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Option configures an Engine.
type Option func(*Engine)

// WithClock sets the clock.
func WithClock(c Clock) Option { return func(e *Engine) { e.clock = c } }

// WithRng sets the randomness source.
func WithRng(r Rng) Option { return func(e *Engine) { e.rng = r } }

// WithEmitter sets the progress callback.
func WithEmitter(fn func(Progress)) Option { return func(e *Engine) { e.emitFn = fn } }

// Engine owns one run at a time.
type Engine struct {
	drv     input.Driver
	clock   Clock
	rng     Rng
	emitFn  func(Progress)
	emitMin time.Duration

	mu            sync.Mutex
	cond          *sync.Cond
	state         State
	paused        bool
	stopRequested bool
	cancel        context.CancelFunc

	profileName string
	moduleID    string
	moduleLabel string
	moduleIndex int
	moduleTotal int
	clickIndex  int
	clickTotal  int
	iteration   int
	clicks      int64
	active      time.Duration
	maxRuntime  time.Duration
	stopReason  string
	errText     string
	lastEmit    time.Time
}

// New builds an Engine.
func New(drv input.Driver, opts ...Option) *Engine {
	e := &Engine{
		drv:     drv,
		clock:   RealClock{},
		rng:     defaultRng{},
		emitMin: 100 * time.Millisecond,
		state:   StateIdle,
	}
	e.cond = sync.NewCond(&e.mu)
	for _, o := range opts {
		o(e)
	}
	return e
}

type defaultRng struct{}

func (defaultRng) IntN(n int) int { return int(time.Now().UnixNano() % int64(n)) }

// Run starts a run with a deep-enough snapshot of p. It refuses when a run is
// already active or when no module is enabled.
func (e *Engine) Run(p *model.Profile) error {
	if p == nil {
		return errors.New("engine: nil profile")
	}
	enabled := p.EnabledModules()
	if len(enabled) == 0 {
		return ErrNoModules
	}
	snap := *p
	snap.Modules = append([]model.Module(nil), p.Modules...)

	e.mu.Lock()
	if e.state != StateIdle {
		e.mu.Unlock()
		return ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.state = StateRunning
	e.paused = false
	e.stopRequested = false
	e.profileName = snap.Name
	e.moduleTotal = len(enabled)
	e.iteration = 0
	e.clicks = 0
	e.active = 0
	e.maxRuntime = time.Duration(snap.Failsafe.MaxRuntimeSec) * time.Second
	e.stopReason = ""
	e.errText = ""
	e.moduleID, e.moduleLabel = "", ""
	e.moduleIndex, e.clickIndex, e.clickTotal = 0, 0, 0
	e.mu.Unlock()

	e.emit(true)
	go e.loop(ctx, &snap, enabled)
	return nil
}

func (e *Engine) loop(ctx context.Context, p *model.Profile, enabled []model.Module) {
	defer func() {
		_ = e.drv.ReleaseAll()
		e.finish(ctx)
	}()

	for {
		if e.shouldStop(ctx) {
			return
		}
		e.mu.Lock()
		e.iteration++
		iter := e.iteration
		e.mu.Unlock()
		e.emit(true)

		for i, m := range enabled {
			if e.shouldStop(ctx) {
				return
			}
			e.mu.Lock()
			e.moduleIndex = i + 1
			e.moduleID = m.ID
			e.moduleLabel = m.Label()
			e.clickIndex = 0
			e.clickTotal = m.Count
			e.mu.Unlock()
			e.emit(true)

			if err := e.runModule(ctx, m); err != nil {
				switch {
				case errors.Is(err, errMaxRuntime):
					e.setStop(ReasonMaxRuntime, "")
				case errors.Is(err, errInjection):
					e.setStop(ReasonError, err.Error())
				case errors.Is(err, errStopped):
					// shouldStop already recorded the reason.
				}
				return
			}
		}

		if p.Loop.Mode == model.LoopOnce {
			e.setStop(ReasonDone, "")
			return
		}
		if p.Loop.Mode == model.LoopCount && iter >= p.Loop.Count {
			e.setStop(ReasonDone, "")
			return
		}
		if err := e.delay(ctx, e.draw(p.Loop.LoopDelay)); err != nil {
			return
		}
	}
}

var (
	errMaxRuntime = errors.New("max runtime reached")
	errInjection  = errors.New("injection failed")
	errStopped    = errors.New("run stopped")
)

// runModule executes one module's clicks. clickInterval is an additional pause
// AFTER the click completes: the button is released, then we wait clickInterval
// before the next click. So the period is holdMs + clickInterval.
func (e *Engine) runModule(ctx context.Context, m model.Module) error {
	interval := time.Duration(m.ClickInterval) * time.Millisecond
	moveOnly := m.Button == model.ButtonNone
	var btn input.Button
	if !moveOnly {
		var err error
		btn, err = input.ParseButton(m.Button)
		if err != nil {
			return err
		}
	}

	for k := 0; k < m.Count; k++ {
		if e.maxRuntimeHit() {
			return errMaxRuntime
		}
		if e.shouldStop(ctx) {
			return errStopped
		}
		if err := e.waitWhilePaused(ctx); err != nil {
			return err
		}
		e.mu.Lock()
		e.clickIndex = k + 1
		e.mu.Unlock()
		e.emit(false)

		hold := time.Duration(m.HoldMs) * time.Millisecond
		switch {
		case moveOnly:
			// "Move only": no button event. Meaningless for a cursor target
			// (validation warns), so only absolute targets do anything.
			if m.Target.Kind == model.TargetAbsolute {
				if err := e.drv.Move(m.Target.X, m.Target.Y); err != nil {
					return wrapInjection(err)
				}
			}
		case m.Target.Kind == model.TargetAbsolute:
			if err := e.drv.ClickAt(m.Target.X, m.Target.Y, btn, hold); err != nil {
				return wrapInjection(err)
			}
		default:
			if err := e.drv.ClickHere(btn, hold); err != nil {
				return wrapInjection(err)
			}
		}
		e.mu.Lock()
		if !moveOnly {
			e.clicks++
		}
		e.active += hold
		e.mu.Unlock()
		e.emit(false)

		if k < m.Count-1 && interval > 0 {
			if err := e.delay(ctx, interval); err != nil {
				return err
			}
		}
	}

	if m.DelayAfter > 0 {
		if err := e.delay(ctx, time.Duration(m.DelayAfter)*time.Millisecond); err != nil {
			return err
		}
	}
	return nil
}

func wrapInjection(err error) error {
	return errors.Join(errInjection, err)
}

// delay gates on pause, then sleeps the full duration. Pausing before a delay
// makes it start from zero on resume.
func (e *Engine) delay(ctx context.Context, d time.Duration) error {
	if err := e.waitWhilePaused(ctx); err != nil {
		return err
	}
	return e.sleep(ctx, d)
}

func (e *Engine) sleep(ctx context.Context, d time.Duration) error {
	if err := e.clock.Sleep(ctx, d); err != nil {
		return err
	}
	e.mu.Lock()
	e.active += d
	e.mu.Unlock()
	return nil
}

func (e *Engine) draw(dr model.DelayRange) time.Duration {
	if dr.Max <= dr.Min {
		return time.Duration(dr.Min) * time.Millisecond
	}
	n := e.rng.IntN(dr.Max-dr.Min+1) + dr.Min
	return time.Duration(n) * time.Millisecond
}

func (e *Engine) maxRuntimeHit() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.maxRuntime <= 0 {
		return false
	}
	return e.active >= e.maxRuntime
}

func (e *Engine) shouldStop(ctx context.Context) bool {
	e.mu.Lock()
	stopped := e.stopRequested
	e.mu.Unlock()
	if stopped {
		e.setStop(ReasonUser, "")
		return true
	}
	if ctx.Err() != nil {
		e.setStop(ReasonPanic, "")
		return true
	}
	return false
}

func (e *Engine) waitWhilePaused(ctx context.Context) error {
	e.mu.Lock()
	for e.paused && e.state != StateIdle {
		e.cond.Wait()
	}
	e.mu.Unlock()
	return ctx.Err()
}

// --- control --------------------------------------------------------------

// Pause gates the run at the next click boundary.
func (e *Engine) Pause() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.state != StateRunning {
		return nil
	}
	e.paused = true
	e.state = StatePaused
	e.emitLocked(true)
	return nil
}

// Resume continues a paused run; the pending delay restarts from zero.
func (e *Engine) Resume() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.state != StatePaused {
		return nil
	}
	e.paused = false
	e.state = StateRunning
	e.cond.Broadcast()
	e.emitLocked(true)
	return nil
}

// Stop is graceful: finish the current click, stop before the next.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.state == StateIdle {
		return nil
	}
	e.stopRequested = true
	e.paused = false
	e.state = StateStopping
	e.cond.Broadcast()
	return nil
}

// Panic cancels immediately, interrupting sleeps and releasing all inputs.
func (e *Engine) Panic() error {
	e.mu.Lock()
	if e.state == StateIdle {
		e.mu.Unlock()
		return nil
	}
	e.stopRequested = true
	e.paused = false
	e.state = StateStopping
	cancel := e.cancel
	e.cond.Broadcast()
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (e *Engine) finish(ctx context.Context) {
	e.mu.Lock()
	if e.stopReason == "" {
		if ctx.Err() != nil {
			e.stopReason = ReasonPanic
		} else {
			e.stopReason = ReasonDone
		}
	}
	e.state = StateIdle
	e.paused = false
	e.emitLocked(true)
	e.mu.Unlock()
}

func (e *Engine) setStop(reason, errText string) {
	e.mu.Lock()
	if e.stopReason == "" {
		e.stopReason = reason
		e.errText = errText
	}
	e.mu.Unlock()
}

// Status returns the latest Progress.
func (e *Engine) Status() Progress {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.progressLocked()
}

// State returns the current state.
func (e *Engine) State() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

func (e *Engine) progressLocked() Progress {
	return Progress{
		State:         e.state,
		ProfileName:   e.profileName,
		ModuleID:      e.moduleID,
		ModuleLabel:   e.moduleLabel,
		ModuleIndex:   e.moduleIndex,
		ModuleTotal:   e.moduleTotal,
		ClickIndex:    e.clickIndex,
		ClickTotal:    e.clickTotal,
		LoopIteration: e.iteration,
		Clicks:        e.clicks,
		ElapsedMs:     e.active.Milliseconds(),
		StopReason:    e.stopReason,
		Error:         e.errText,
	}
}

// emit throttles to ~10 Hz unless force is set (state/module changes and run
// end always emit immediately).
func (e *Engine) emit(force bool) {
	e.mu.Lock()
	e.emitLocked(force)
	e.mu.Unlock()
}

func (e *Engine) emitLocked(force bool) {
	if e.emitFn == nil {
		return
	}
	now := e.clock.Now()
	if !force && now.Sub(e.lastEmit) < e.emitMin {
		return
	}
	e.lastEmit = now
	e.emitFn(e.progressLocked())
}
