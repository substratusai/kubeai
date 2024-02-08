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

	lastScaleDown time.Time

	// scaleFuncMtx ensures the scale function is not run concurrently.
	scaleFuncMtx sync.Mutex
	scaleFunc    func(n int32, atLeastOne bool) error

	scaleDownDelay time.Duration
}

func newScaler(scaleDownDelay time.Duration, scaleFunc func(int32, bool) error) *scaler {
	s := &scaler{
		// -1 represents unknown
		currentScale:   -1,
		desiredScale:   -1,
		scaleDownDelay: scaleDownDelay,
		lastScaleDown:  time.Now(),
	}

	s.scaleFunc = func(n int32, atLeastOne bool) error {
		s.scaleFuncMtx.Lock()
		err := scaleFunc(n, atLeastOne)
		s.scaleFuncMtx.Unlock()

		if err != nil {
			return err
		}

		return nil
	}

	return s
}

// AtLeastOne schedules a scale up if the current scale is zero.
func (s *scaler) AtLeastOne() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	log.Printf("AtLeastOne()")
	if err := s.scaleFunc(-1, true); err != nil {
		log.Printf("scale error: %v", err)
	}
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
	nMinMax := s.applyMinMax(n)
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.desiredScale = nMinMax

	if s.desiredScale > s.currentScale {
		// Scale up immediately.
		log.Printf("Scaling up immediately")
		go func() {
			if err := s.scaleFunc(s.desiredScale, false); err != nil {
				log.Printf("scale down to %v error: %v", s.desiredScale, err)
			} else {
				log.Printf("Scaled up to %v successfully", s.desiredScale)
			}
		}()
	} else if s.desiredScale == s.currentScale {
		log.Println("Desired scale equals current scale, doing nothing")
	} else {
		if time.Since(s.lastScaleDown) >= s.scaleDownDelay {
			go func() {
				if err := s.scaleFunc(s.desiredScale, false); err != nil {
					log.Printf("scale down error: %v", err)
				} else {
					log.Printf("Scaled down to %v successfully", s.desiredScale)
					s.mtx.Lock()
					s.lastScaleDown = time.Now()
					s.mtx.Unlock()
				}
			}()
		} else {
			log.Printf("Waiting for scale down delay to pass, last scale down: %v. Waiting for at least another %v", s.lastScaleDown, time.Since(s.lastScaleDown))
		}
	}
}

func (s *scaler) applyMinMax(n int32) int32 {
	s.mtx.Lock()
	min := s.minScale
	max := s.maxScale
	s.mtx.Unlock()
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

func (s *scaler) getScale() scale {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return scale{Current: s.currentScale, Min: s.minScale, Max: s.maxScale}
}
