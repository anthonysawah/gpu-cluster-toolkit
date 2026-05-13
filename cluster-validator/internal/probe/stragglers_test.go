package probe

import (
	"testing"
	"time"
)

func TestDetectStragglers_NoStraggler(t *testing.T) {
	results := []NodeResult{
		{Name: "a", MeanRTT: 10 * time.Millisecond},
		{Name: "b", MeanRTT: 11 * time.Millisecond},
		{Name: "c", MeanRTT: 9 * time.Millisecond},
		{Name: "d", MeanRTT: 10 * time.Millisecond},
	}
	summary := DetectStragglers(results, 2.0)
	if summary.StragglerCount != 0 {
		t.Errorf("expected 0 stragglers, got %d", summary.StragglerCount)
	}
}

func TestDetectStragglers_StatisticalOutlier(t *testing.T) {
	// Many healthy nodes plus one straggler. Statistical detection triggers.
	results := []NodeResult{
		{Name: "a", MeanRTT: 10 * time.Millisecond},
		{Name: "b", MeanRTT: 11 * time.Millisecond},
		{Name: "c", MeanRTT: 9 * time.Millisecond},
		{Name: "d", MeanRTT: 10 * time.Millisecond},
		{Name: "e", MeanRTT: 11 * time.Millisecond},
		{Name: "f", MeanRTT: 10 * time.Millisecond},
		{Name: "g", MeanRTT: 9 * time.Millisecond},
		{Name: "h", MeanRTT: 10 * time.Millisecond},
		{Name: "straggler", MeanRTT: 100 * time.Millisecond},
	}
	summary := DetectStragglers(results, 2.0)
	if summary.StragglerCount != 1 {
		t.Fatalf("expected 1 straggler, got %d", summary.StragglerCount)
	}
	for _, v := range summary.Verdicts {
		if v.Name == "straggler" {
			if !v.IsStraggler {
				t.Errorf("expected 'straggler' to be flagged")
			}
			if v.Reason != "statistical outlier" {
				t.Errorf("expected reason 'statistical outlier', got %q", v.Reason)
			}
		}
	}
}

func TestDetectStragglers_AbsoluteSlowdown_TwoNodes(t *testing.T) {
	// Two nodes, one much slower. Statistical detection fails on small N
	// but absolute slowdown rule catches it.
	results := []NodeResult{
		{Name: "fast", MeanRTT: 10 * time.Millisecond},
		{Name: "slow", MeanRTT: 60 * time.Millisecond},
	}
	summary := DetectStragglers(results, 2.0)
	if summary.StragglerCount != 1 {
		t.Fatalf("expected 1 straggler, got %d", summary.StragglerCount)
	}
	for _, v := range summary.Verdicts {
		if v.Name == "slow" {
			if !v.IsStraggler {
				t.Errorf("expected 'slow' to be flagged")
			}
			if v.Reason != "more than 2x fastest node" {
				t.Errorf("expected reason 'more than 2x fastest node', got %q", v.Reason)
			}
		}
		if v.Name == "fast" && v.IsStraggler {
			t.Errorf("did not expect 'fast' to be flagged")
		}
	}
}

func TestDetectStragglers_TwoNodes_CloseEnough(t *testing.T) {
	// Two nodes within 2x of each other. Neither should flag.
	results := []NodeResult{
		{Name: "a", MeanRTT: 10 * time.Millisecond},
		{Name: "b", MeanRTT: 15 * time.Millisecond},
	}
	summary := DetectStragglers(results, 2.0)
	if summary.StragglerCount != 0 {
		t.Errorf("expected 0 stragglers, got %d", summary.StragglerCount)
	}
}

func TestDetectStragglers_Empty(t *testing.T) {
	summary := DetectStragglers(nil, 2.0)
	if summary.StragglerCount != 0 {
		t.Errorf("expected 0 stragglers for empty input, got %d", summary.StragglerCount)
	}
}