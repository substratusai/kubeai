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
		{
			name:              "Scale to min only when below min scale",
			current:           5,
			minScale:          1,
			maxScale:          10,
			desiredScale:      0,
			expectedScaleFunc: true,
			expectedLastScale: 1,
		},
		{
			name:              "Scale down within bounds",
			current:           5,
			minScale:          1,
			maxScale:          10,
			desiredScale:      3,
			expectedScaleFunc: true,
			lastScaleDown:     time.Now().Add(-2 * time.Second),
			expectedLastScale: 3,
		},
		{
			name:              "Scale down needs to wait",
			current:           5,
			minScale:          1,
			maxScale:          10,
			desiredScale:      3,
			expectedScaleFunc: false,
			lastScaleDown:     time.Now().Add(5 * time.Second),
			expectedLastScale: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new scaler for each test case
			var lastScale int32
			var scaleFuncCalled bool

			var mockScaleMtx sync.Mutex // Mutex to protect shared state in mockScaleFunc
			var mockScaleWG sync.WaitGroup
			mockScaleFunc := func(n int32, atLeastOne bool) error {
				defer mockScaleWG.Done()

				mockScaleMtx.Lock()
				defer mockScaleMtx.Unlock()
				lastScale = n
				scaleFuncCalled = true
				return nil
			}
			s := newScaler(1*time.Second, mockScaleFunc)
			s.lastScaleDown = tc.lastScaleDown

			// Setup
			s.UpdateState(tc.current, tc.minScale, tc.maxScale)
			scaleFuncCalled = false

			if tc.expectedScaleFunc {
				mockScaleWG.Add(1)
			}
			// Action
			s.SetDesiredScale(tc.desiredScale)

			// Wait for the scale function to be called
			mockScaleWG.Wait()

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
