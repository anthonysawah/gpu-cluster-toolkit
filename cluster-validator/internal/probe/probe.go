// Package probe sends a synthetic payload to a single endpoint and measures
// the round-trip time. It is the building block used by parallel cluster scans.
package probe

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultClient is reused across probes so we benefit from connection pooling.
// Without this every probe would do a fresh TCP handshake, which would skew
// timings, especially over TLS.
var defaultClient = &http.Client{
	// Per-request timeout. If a node is so dead it can't respond in 10s,
	// we want the probe to fail rather than hang the whole scan.
	Timeout: 10 * time.Second,
}

// ProbeOnce sends payload to endpoint via HTTP POST and returns the round-trip
// time. The context can be used to cancel the probe early.
//
// "Round trip" here means: time from sending the first byte of the request to
// finishing reading the last byte of the response body. This is what matters
// for all-reduce-style workloads where one side can't proceed until the data
// fully crosses the wire in both directions.
func ProbeOnce(ctx context.Context, endpoint string, payload []byte) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(len(payload))

	start := time.Now()

	resp, err := defaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Drain the response body. The timing is only correct if we actually
	// receive all the bytes, not just the headers. io.Copy reads to EOF.
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return 0, fmt.Errorf("reading response: %w", err)
	}

	elapsed := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		return elapsed, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return elapsed, nil
}

// MakePayload returns a byte slice of size n, used as the probe body.
// Content doesn't matter for our purposes (an echo server reflects whatever
// you send), so we use a fixed pattern that's cheap to allocate.
func MakePayload(n int) []byte {
	return bytes.Repeat([]byte{0xAB}, n)
}