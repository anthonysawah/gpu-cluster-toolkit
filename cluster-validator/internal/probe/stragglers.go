// Straggler detection. Given per-node results, identify nodes whose mean RTT
// is significantly higher than the population, suggesting a performance issue.

package probe

import (
	"time"

	"github.com/anthonysawah/gpu-cluster-toolkit/cluster-validator/internal/stats"
)

// AbsoluteSlowdownFactor is the secondary detection rule. A node is flagged
// as a straggler if its mean RTT is more than this multiple of the fastest
// node's mean RTT, regardless of statistical distribution.
//
// This guards against the small-N problem where one extreme outlier inflates
// the standard deviation enough to hide itself from sigma-based detection.
const AbsoluteSlowdownFactor = 2.0

// Verdict augments a NodeResult with straggler detection.
type Verdict struct {
	NodeResult
	IsStraggler bool
	SigmasAbove float64 // how many standard deviations above the global mean
	Reason      string  // why this node was flagged (or empty if healthy)
}

// ScanSummary is the full report after straggler detection.
type ScanSummary struct {
	Verdicts       []Verdict
	GlobalMean     time.Duration
	GlobalStdDev   time.Duration
	HealthyMean    time.Duration // mean excluding stragglers
	FastestMean    time.Duration
	StragglerCount int
}

// DetectStragglers computes straggler verdicts for the given results.
// A node is a straggler if either:
//   1. Its mean RTT is more than thresholdSigma standard deviations above
//      the global mean (statistical rule).
//   2. Its mean RTT is more than AbsoluteSlowdownFactor times the fastest
//      node's mean RTT (absolute rule, guards against small-N edge cases).
func DetectStragglers(results []NodeResult, thresholdSigma float64) ScanSummary {
	summary := ScanSummary{
		Verdicts: make([]Verdict, 0, len(results)),
	}

	if len(results) == 0 {
		return summary
	}

	// Gather per-node means.
	means := make([]time.Duration, 0, len(results))
	for _, r := range results {
		means = append(means, r.MeanRTT)
	}

	summary.GlobalMean = stats.Mean(means)
	summary.GlobalStdDev = stats.StdDev(means)
	summary.FastestMean = minDuration(means)

	// Thresholds.
	sigmaThreshold := summary.GlobalMean + time.Duration(thresholdSigma*float64(summary.GlobalStdDev))
	absoluteThreshold := time.Duration(float64(summary.FastestMean) * AbsoluteSlowdownFactor)

	healthyMeans := make([]time.Duration, 0, len(results))

	for _, r := range results {
		v := Verdict{NodeResult: r}

		if summary.GlobalStdDev > 0 {
			v.SigmasAbove = float64(r.MeanRTT-summary.GlobalMean) / float64(summary.GlobalStdDev)
		}

		switch {
		case r.MeanRTT > sigmaThreshold:
			v.IsStraggler = true
			v.Reason = "statistical outlier"
		case r.MeanRTT > absoluteThreshold && len(results) > 1:
			v.IsStraggler = true
			v.Reason = "more than 2x fastest node"
		}

		if v.IsStraggler {
			summary.StragglerCount++
		} else {
			healthyMeans = append(healthyMeans, r.MeanRTT)
		}

		summary.Verdicts = append(summary.Verdicts, v)
	}

	summary.HealthyMean = stats.Mean(healthyMeans)
	return summary
}

// minDuration returns the smallest duration in the slice. Returns 0 for empty.
func minDuration(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	min := ds[0]
	for _, d := range ds[1:] {
		if d < min {
			min = d
		}
	}
	return min
}