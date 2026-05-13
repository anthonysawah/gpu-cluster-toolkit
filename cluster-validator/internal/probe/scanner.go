// Scanner orchestrates parallel probing across a list of nodes.
// Each node is probed N times sequentially; nodes run concurrently.

package probe

import (
	"context"
	"sync"
	"time"
)

// NodeResult holds the per-node outcome of a scan.
type NodeResult struct {
	Name       string
	Endpoint   string
	Durations  []time.Duration  // per-probe timings
	MeanRTT    time.Duration    // average of Durations
	Errors     int              // count of failed probes
}

// ScanRequest is the input to Scanner.Scan.
type ScanRequest struct {
	Name         string
	Endpoint     string
	Iterations   int
	PayloadBytes int
	// SimulateFault, if true, adds an artificial delay to each probe.
	// Used by --simulate-fault to demo straggler detection.
	SimulateFault bool
}

// Scan probes every node in the request list in parallel and returns the
// per-node aggregated results.
func Scan(ctx context.Context, reqs []ScanRequest) []NodeResult {
	results := make(chan NodeResult, len(reqs))
	var wg sync.WaitGroup

	for _, req := range reqs {
		wg.Add(1)
		go func(r ScanRequest) {
			defer wg.Done()
			results <- scanOne(ctx, r)
		}(req)
	}

	// Close results once all workers are done.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect everything.
	collected := make([]NodeResult, 0, len(reqs))
	for r := range results {
		collected = append(collected, r)
	}
	return collected
}

// scanOne runs `iterations` probes against a single node sequentially,
// returning the per-node aggregate.
func scanOne(ctx context.Context, req ScanRequest) NodeResult {
	payload := MakePayload(req.PayloadBytes)
	result := NodeResult{
		Name:      req.Name,
		Endpoint:  req.Endpoint,
		Durations: make([]time.Duration, 0, req.Iterations),
	}

	for i := 0; i < req.Iterations; i++ {
		if ctx.Err() != nil {
			break // user cancelled
		}
		d, err := ProbeOnce(ctx, req.Endpoint, payload)
		if err != nil {
			result.Errors++
			continue
		}
		if req.SimulateFault {
			d += 50 * time.Millisecond
		}
		result.Durations = append(result.Durations, d)
	}

	result.MeanRTT = mean(result.Durations)
	return result
}

// mean computes the arithmetic mean of a slice of durations.
// Returns 0 for an empty slice.
func mean(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range ds {
		total += d
	}
	return total / time.Duration(len(ds))
}