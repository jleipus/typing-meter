package timectrl

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type TimeController struct {
	IntervalCh chan struct{}
	DoneCh     chan struct{}
	TimedMode  bool

	start      time.Time
	ticker     *time.Ticker
	timer      *time.Timer
	stopTicker chan struct{}
	stopTimer  chan struct{}
}

func (tc *TimeController) RunTicker(wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(tc.IntervalCh)
	defer close(tc.stopTicker)

	for {
		select {
		case <-tc.ticker.C:
			tc.IntervalCh <- struct{}{}
		case <-tc.stopTicker:
			tc.ticker.Stop()
			return
		}
	}
}

func (tc *TimeController) RunTimer(wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(tc.DoneCh)
	defer close(tc.stopTimer)

	select {
	case <-tc.timer.C:
		tc.DoneCh <- struct{}{}
	case <-tc.stopTimer:
		tc.timer.Stop()
		return
	}
}

func (tc *TimeController) StopTicker() error {
	if tc.ticker != nil {
		tc.stopTicker <- struct{}{}
		return nil
	}

	return errors.New("ticker is not running")
}

func (tc *TimeController) StopTimer() error {
	if tc.timer != nil {
		tc.stopTimer <- struct{}{}
		return nil
	}

	return errors.New("timer is not running")
}

func (tc *TimeController) StopAll() {
	tickerErr := tc.StopTicker()
	timerErr := tc.StopTimer()
	if tickerErr != nil || timerErr != nil {
		panic(fmt.Sprintf(tickerErr.Error(), timerErr.Error()))
	}
}

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

func NewTimeController(opts ...ControllerOption) *TimeController {
	tc := TimeController{
		IntervalCh: make(chan struct{}),
		DoneCh:     make(chan struct{}),
		TimedMode:  false,

		ticker:     nil,
		timer:      nil,
		start:      time.Now(),
		stopTicker: make(chan struct{}),
		stopTimer:  make(chan struct{}),
	}

	for _, opt := range opts {
		opt(&tc)
	}

	return &tc
}
