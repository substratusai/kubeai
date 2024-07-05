package deployments

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
	minScale     int32
	maxScale     int32

	// The time that scaler decided to scale down which gets delayed by scaleDownDelay
	desiredScaleDownTime time.Time

	// scaleFuncMtx ensures the scale function is not run concurrently.
	scaleFuncMtx sync.Mutex
	scaleFunc    func(n int32, atLeastOne bool)

	scaleDownDelay time.Duration
}

func newScaler(scaleDownDelay time.Duration, scaleFunc func(int32, bool) error) *scaler {
	s := &scaler{
		// -1 represents unknown
		currentScale:   -1,
		desiredScale:   -1,
		scaleDownDelay: scaleDownDelay,
	}

	// do error handling by logging here and do not return error
	s.scaleFunc = func(n int32, atLeastOne bool) {
		s.scaleFuncMtx.Lock()
		err := scaleFunc(n, atLeastOne)
		s.scaleFuncMtx.Unlock()

		if err != nil {
			log.Printf("error scaling: %+v", err)
		}
	}

	return s
}

// AtLeastOne schedules a scale up if the current scale is zero.
func (s *scaler) AtLeastOne() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	log.Printf("AtLeastOne()")
	s.scaleFunc(-1, true)
}

// UpdateState updates the current state of the scaler
func (s *scaler) UpdateState(replicas, min, max int32) {
	log.Printf("UpdateState(%v, %v, %v)", replicas, min, max)
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.minScale = min
	s.maxScale = max
	s.currentScale = replicas
}

// SetDesiredScale sets the desired scale of the scaler and scales
// if needed.
func (s *scaler) SetDesiredScale(n int32) {
	log.Printf("SetDesiredScale(%v), current: %v, min: %v, max: %v", n, s.currentScale, s.minScale, s.maxScale)
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.desiredScale = s.applyMinMax(n)

	if s.desiredScale > s.currentScale {
		// Scale up immediately.
		log.Printf("Scaling up to %v immediately", s.desiredScale)
		s.scaleFunc(s.desiredScale, false)
		s.desiredScaleDownTime = time.Time{}
	} else if s.desiredScale == s.currentScale {
		log.Printf("Desired scale %v equals current scale %v, doing nothing", s.desiredScale, s.currentScale)
		s.desiredScaleDownTime = time.Time{}
	} else {
		if s.desiredScaleDownTime.IsZero() {
			s.desiredScaleDownTime = time.Now()
			expectedNextScaleDown := s.desiredScaleDownTime.Add(s.scaleDownDelay)
			log.Printf("Delaying scale down to happen on or after %v", expectedNextScaleDown)
		} else if time.Since(s.desiredScaleDownTime) >= s.scaleDownDelay {
			log.Printf("Scaling down to %v immediately", s.desiredScale)
			s.scaleFunc(s.desiredScale, false)
		}
	}
}

// applyMinMax applies the min and max scale to the given number
// function needs to be called within the locked scaler.mtx
func (s *scaler) applyMinMax(n int32) int32 {
	min := s.minScale
	max := s.maxScale
	if n < min {
		n = min
	} else if n > max {
		n = max
	}
	return n
}

type scale struct {
	Current, Min, Max int32
}

func (s *scaler) GetScale() scale {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return scale{Current: s.currentScale, Min: s.minScale, Max: s.maxScale}
}
