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

	lastSuccessfulScale time.Time

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
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.desiredScale = s.applyMinMax(n)

	if s.desiredScale > s.currentScale {
		// Scale up immediately.
		log.Printf("Scaling up immediately")
		go func() {
			s.mtx.Lock()
			defer s.mtx.Unlock()
			if err := s.scaleFunc(s.desiredScale, false); err != nil {
				log.Printf("scale down to %v error: %v", s.desiredScale, err)
				return
			}
			s.lastSuccessfulScale = time.Now()
			log.Printf("Scaled up to %v successfully", s.desiredScale)
		}()
	} else if s.desiredScale == s.currentScale {
		log.Println("Desired scale equals current scale, doing nothing")
	} else {
		if s.lastSuccessfulScale.IsZero() || time.Since(s.lastSuccessfulScale) >= s.scaleDownDelay {
			go func() {
				s.mtx.Lock()
				defer s.mtx.Unlock()
				if err := s.scaleFunc(s.desiredScale, false); err != nil {
					log.Printf("scale down error: %v", err)
					return
				}
				log.Printf("Scaled down to %v successfully", s.desiredScale)
				s.lastSuccessfulScale = time.Now()
			}()
		} else {
			log.Printf("Waiting for scale down delay to pass, last scale down: %v. Waiting for at least another %v", s.lastSuccessfulScale, time.Since(s.lastSuccessfulScale))
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

func (s *scaler) getScale() scale {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return scale{Current: s.currentScale, Min: s.minScale, Max: s.maxScale}
}
