package timectrl

import (
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
	triggerEnd chan struct{}
}

func (tc *TimeController) RunTicker(wg sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-tc.ticker.C:
			tc.IntervalCh <- struct{}{}
		case <-tc.triggerEnd:
			tc.ticker.Stop()
			return
		}
	}
}

func (tc *TimeController) RunTimer(wg sync.WaitGroup) {
	select {
	case <-tc.timer.C:
		tc.DoneCh <- struct{}{}
	case <-tc.triggerEnd:
		tc.timer.Stop()
	}

	wg.Done()
}

func (tc *TimeController) Stop() {
	tc.triggerEnd <- struct{}{}
	tc.triggerEnd <- struct{}{}
}

func (tc *TimeController) TimePassed() time.Duration {
	return time.Since(tc.start)
}

type ControllerOption func(tc *TimeController)

func WithTicker(interval int) ControllerOption {
	return func(tc *TimeController) {
		tc.ticker = time.NewTicker(time.Duration(interval) * time.Second)
		fmt.Println("ticker started")
	}
}

func WithTimer(limit int) ControllerOption {
	return func(tc *TimeController) {
		tc.timer = time.NewTimer(time.Duration(limit) * time.Second)
		tc.TimedMode = true
		fmt.Println("timer started")
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
		triggerEnd: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(&tc)
	}

	fmt.Println("timeController created")
	return &tc
}
