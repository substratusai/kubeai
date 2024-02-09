package deployments

import (
	"sync"
	"testing"
	"time"
)

func TestSetDesiredScale(t *testing.T) {
	// Test case setup
	testCases := []struct {
		name              string
		current           int32
		minScale          int32
		maxScale          int32
		desiredScale      int32
		lastScaleDown     time.Time
		expectedScaleFunc bool
		expectedLastScale int32
	}{
		{
			name:              "Scale up within bounds",
			current:           5,
			minScale:          1,
			maxScale:          10,
			desiredScale:      7,
			expectedScaleFunc: true,
			expectedLastScale: 7,
		},
		{
			name:              "Scale to max only when exceeding max scale",
			current:           5,
			minScale:          1,
			maxScale:          10,
			desiredScale:      11,
			expectedScaleFunc: true,
			expectedLastScale: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new scaler for each test case
			var lastScale int32
			var scaleFuncCalled bool

			var mockScaleMtx sync.Mutex // Mutex to protect shared state in mockScaleFunc
			mockScaleFunc := func(n int32, atLeastOne bool) error {
				mockScaleMtx.Lock()
				defer mockScaleMtx.Unlock()
				lastScale = n
				scaleFuncCalled = true
				return nil
			}
			s := newScaler(1*time.Second, mockScaleFunc)

			// Setup
			s.UpdateState(tc.current, tc.minScale, tc.maxScale)
			scaleFuncCalled = false

			if tc.expectedScaleFunc {
				time.Sleep(100 * time.Millisecond)
			}

			// Action
			s.SetDesiredScale(tc.desiredScale)

			// Assertions
			mockScaleMtx.Lock() // Ensure consistency of the checked state
			if scaleFuncCalled != tc.expectedScaleFunc {
				t.Errorf("expected scaleFuncCalled to be %v, got %v", tc.expectedScaleFunc, scaleFuncCalled)
			}
			if scaleFuncCalled && lastScale != tc.expectedLastScale {
				t.Errorf("expected lastScale to be %v, got %v", tc.expectedLastScale, lastScale)
			}
			mockScaleMtx.Unlock()
		})
	}
}

// Tests that right afer a scale up, a scale down is not triggered,
// but once scaleDownDelay passes the scale down will be triggered.
func TestNoScaleDownAfterScaleUp(t *testing.T) {
	var lastScale int32
	var scaleFuncCalled bool

	var mockScaleMtx sync.Mutex // Mutex to protect shared state in mockScaleFunc
	mockScaleFunc := func(n int32, atLeastOne bool) error {
		mockScaleMtx.Lock()
		defer mockScaleMtx.Unlock()
		lastScale = n
		scaleFuncCalled = true
		return nil
	}

	// Create a new scaler
	s := newScaler(2*time.Second, mockScaleFunc)

	// Trigger a scale up
	s.UpdateState(5, 1, 10)
	s.SetDesiredScale(7)
	time.Sleep(10 * time.Millisecond)

	// Verify scale up worked as expected
	mockScaleMtx.Lock() // Ensure consistency of the checked state
	if scaleFuncCalled != true {
		t.Errorf("expected scaleFuncCalled to be true, got %v", scaleFuncCalled)
	}
	if scaleFuncCalled && lastScale != 7 {
		t.Errorf("expected lastScale to be 7, got %v", lastScale)
	}
	scaleFuncCalled = false
	mockScaleMtx.Unlock()

	// Trigger a scale down right after scale up and verify it won't happen
	s.SetDesiredScale(3)
	time.Sleep(1 * time.Second)
	mockScaleMtx.Lock() // Ensure consistency of the checked state
	if scaleFuncCalled != false {
		t.Errorf("expected scaleFuncCalled to be false, got %v", scaleFuncCalled)
	}
	mockScaleMtx.Unlock()

	// Trigger a scale down after 2 more seconds and verify scale down succeeded
	time.Sleep(2 * time.Second)
	s.SetDesiredScale(3)
	mockScaleMtx.Lock() // Ensure consistency of the checked state
	if scaleFuncCalled != true {
		t.Errorf("expected scaleFuncCalled to be true, got %v", scaleFuncCalled)
	}
	mockScaleMtx.Unlock()
}

// Tests that scale down would still be delayed even if
// current replicas has been stable for a duration beyond the scaleDownDelay
func TestDelayedScaleDownAfterSustainedUsage(t *testing.T) {
	var lastScale int32
	var scaleFuncCalled bool

	var mockScaleMtx sync.Mutex // Mutex to protect shared state in mockScaleFunc
	mockScaleFunc := func(n int32, atLeastOne bool) error {
		mockScaleMtx.Lock()
		defer mockScaleMtx.Unlock()
		lastScale = n
		scaleFuncCalled = true
		return nil
	}

	// Create a new scaler
	s := newScaler(2*time.Second, mockScaleFunc)

	// Trigger a scale up
	s.UpdateState(5, 1, 10)
	s.SetDesiredScale(7)

	// Verify scale up worked as expected
	mockScaleMtx.Lock() // Ensure consistency of the checked state
	if scaleFuncCalled != true {
		t.Errorf("expected scaleFuncCalled to be true, got %v", scaleFuncCalled)
	}
	if scaleFuncCalled && lastScale != 7 {
		t.Errorf("expected lastScale to be 7, got %v", lastScale)
	}
	scaleFuncCalled = false
	mockScaleMtx.Unlock()
	s.UpdateState(7, 1, 10)
	// Wait for scale down delay to pass
	time.Sleep(2*time.Second + 10*time.Millisecond)

	// Trigger a scale down
	s.SetDesiredScale(3)
	time.Sleep(1 * time.Second)
	mockScaleMtx.Lock() // Ensure consistency of the checked state
	if scaleFuncCalled != false {
		t.Errorf("expected scaleFuncCalled to be false, got %v", scaleFuncCalled)
	}
	mockScaleMtx.Unlock()

	// Trigger a scale down after 2 more seconds and verify scale down succeeded
	time.Sleep(2*time.Second + 10*time.Millisecond)
	s.SetDesiredScale(3)
	mockScaleMtx.Lock() // Ensure consistency of the checked state
	if scaleFuncCalled != true {
		t.Errorf("expected scaleFuncCalled to be true, got %v", scaleFuncCalled)
	}
	mockScaleMtx.Unlock()
}
