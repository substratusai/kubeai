package main

import (
	"log"
	"sync"
	"time"
)

type scaler struct {
	// mtx protects scaler fields from concurrent access.
	mtx          sync.Mutex
	currentScale int32
	desiredScale int32

	// scaleFuncMtx ensures the scale function is not run concurrently.
	scaleFuncMtx sync.Mutex
	scaleFunc    func(n int32) error

	scaleDownDelay   time.Duration
	scaleDownStarted bool
	scaleDownTimer   *time.Timer
}

func (s *scaler) AtLeastOne() {
	log.Printf("AtLeastOne()")
	s.compareScales(-1, -1, true)
}

func (s *scaler) SetCurrentScale(n int32) {
	log.Printf("SetCurrentScale(%v)", n)
	s.compareScales(n, -1, false)
}

func (s *scaler) SetDesiredScale(n int32) {
	log.Printf("SetDesiredScale(%v)", n)
	s.compareScales(-1, n, false)
}

func (s *scaler) compareScales(current, desired int32, zeroToOne bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if zeroToOne {
		// Could be 0 or -1
		if s.desiredScale < 1 {
			s.desiredScale = 1
		} else {
			return
		}
	} else {
		if current != -1 {
			s.currentScale = current
		}
		if desired != -1 {
			s.desiredScale = desired
		}
	}

	if s.currentScale == -1 || s.desiredScale == -1 {
		// Nothing to compare if we only have partial information
		return
	}

	if s.desiredScale > s.currentScale {
		// Scale up immediately.
		go s.scaleFunc(s.desiredScale)
		s.scaleDownStarted = false
	} else if s.desiredScale == s.currentScale {
		// Do nothing, schedule nothing.
		if s.scaleDownTimer != nil {
			s.scaleDownTimer.Stop()
		}
		s.scaleDownStarted = false
	} else {
		// Schedule a scale down.

		if s.scaleDownTimer == nil {
			s.scaleDownTimer = time.AfterFunc(s.scaleDownDelay, func() {
				if err := s.scaleFunc(s.desiredScale); err != nil {
					log.Printf("task: run error: %v", err)
				} else {
					s.scaleDownStarted = false
					s.compareScales(s.desiredScale, -1, false)
				}
			})
		} else if !s.scaleDownStarted {
			s.scaleDownTimer.Reset(s.scaleDownDelay)
		}

		s.scaleDownStarted = true
	}
}

func newScaler(scaleDownDelay time.Duration, scaleFunc func(int32) error) *scaler {
	s := &scaler{
		// -1 represents unknown
		currentScale:   -1,
		desiredScale:   -1,
		scaleDownDelay: scaleDownDelay,
	}

	s.scaleFunc = func(n int32) error {
		s.scaleFuncMtx.Lock()
		err := scaleFunc(n)
		s.scaleFuncMtx.Unlock()

		if err != nil {
			return err
		}

		return nil
	}

	return s
}
