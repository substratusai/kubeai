package movingaverage_test

import (
	"testing"

	"github.com/substratusai/lingo/pkg/movingaverage"
)

func TestSimple(t *testing.T) {
	cases := []struct {
		name   string
		seed   []float64
		values []float64
		want   float64
	}{
		{
			name:   "1-2-3",
			seed:   []float64{0, 0, 0},
			values: []float64{1, 2, 3},
			want:   2,
		},
		{
			name:   "3-2-1",
			seed:   make([]float64, 3),
			values: []float64{3, 2, 1},
			want:   2,
		},
		{
			name:   "3-2-1-1-1-1",
			seed:   make([]float64, 3),
			values: []float64{3, 2, 1, 1, 1, 1},
			want:   1,
		},
		{
			name:   "2-3",
			seed:   make([]float64, 2),
			values: []float64{2, 3},
			want:   2.5,
		},
		{
			name:   "2-2-2",
			seed:   []float64{0, 0, 0},
			values: []float64{2, 2, 2},
			want:   2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := movingaverage.NewSimple(tc.seed)
			for _, v := range tc.values {
				a.Next(v)
			}
			got := a.Calculate()
			if got != tc.want {
				t.Errorf("got %v; want %v", got, tc.want)
			}
		})
	}
}
