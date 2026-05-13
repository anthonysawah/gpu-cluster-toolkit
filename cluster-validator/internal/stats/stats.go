// Package stats provides simple statistics primitives over slices of durations,
// used by the straggler detection logic.
package stats

import (
	"math"
	"time"
)

// Mean returns the arithmetic mean of a slice of durations.
// Returns 0 for an empty slice.
func Mean(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range ds {
		total += d
	}
	return total / time.Duration(len(ds))
}

// StdDev returns the population standard deviation of a slice of durations.
// "Population" (divide by N) not "sample" (N-1) because we treat the observed
// nodes as the entire population we care about, not a sample from a larger one.
// Returns 0 for slices of length 0 or 1 (no variance possible).
func StdDev(ds []time.Duration) time.Duration {
	if len(ds) < 2 {
		return 0
	}
	mean := Mean(ds)
	var sumSquares float64
	for _, d := range ds {
		diff := float64(d - mean)
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(ds))
	return time.Duration(math.Sqrt(variance))
}