package timectrl

import (
	"sync"
	"time"
)

// TimeController A data structure to allow passing of signals between a timer, ticker and
// functions waiting for those signals
type TimeController struct {
	IntervalCh chan struct{}
	DoneCh     chan struct{}
	TimedMode  bool

	start        time.Time
	ticker       *time.Ticker
	timer        *time.Timer
	stopTickerCh chan struct{}
	stopTimerCh  chan struct{}
}

// Runs the ticker until stopTicker channel receives data
func (tc *TimeController) RunTicker(wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(tc.IntervalCh)
	defer close(tc.stopTickerCh)

	for {
		select {
		case <-tc.ticker.C:
			tc.IntervalCh <- struct{}{}
		case <-tc.stopTickerCh:
			tc.ticker.Stop()
			return
		}
	}
}

// Runs the timer until stopTimer channel receives data
func (tc *TimeController) RunTimer(wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(tc.DoneCh)
	defer close(tc.stopTimerCh)

	select {
	case <-tc.timer.C:
		tc.DoneCh <- struct{}{}
	case <-tc.stopTimerCh:
		tc.timer.Stop()
		return
	}
}

// If the ticker is running, sends signal to stop it
func (tc *TimeController) stopTicker() {
	if tc.ticker != nil {
		tc.stopTickerCh <- struct{}{}
	}
}

// If the timer is running, sends signal to stop it, else returns error
func (tc *TimeController) stopTimer() {
	if tc.timer != nil {
		tc.stopTimerCh <- struct{}{}
	}
}

// A function to stop both the ticker and timer
func (tc *TimeController) StopAll() {
	tc.stopTicker()
	tc.stopTimer()
}

// Returns the time passed since the TimeController was created
func (tc *TimeController) TimePassed() time.Duration {
	return time.Since(tc.start)
}

type ControllerOption func(tc *TimeController)

func WithTicker(interval int) ControllerOption {
	return func(tc *TimeController) {
		tc.ticker = time.NewTicker(time.Duration(interval) * time.Second)
	}
}

func WithTimer(limit int) ControllerOption {
	return func(tc *TimeController) {
		tc.timer = time.NewTimer(time.Duration(limit) * time.Second)
		tc.TimedMode = true
	}
}

// Variadic function that takes ControllerOption returning functions
// as parameters and runs them
func NewTimeController(opts ...ControllerOption) *TimeController {
	tc := TimeController{
		IntervalCh: make(chan struct{}),
		DoneCh:     make(chan struct{}),
		TimedMode:  false,

		ticker:       nil,
		timer:        nil,
		start:        time.Now(),
		stopTickerCh: make(chan struct{}),
		stopTimerCh:  make(chan struct{}),
	}

	for _, opt := range opts {
		opt(&tc)
	}

	return &tc
}
