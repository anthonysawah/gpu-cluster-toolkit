// Package output renders scan summaries in different formats.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/anthonysawah/gpu-cluster-toolkit/cluster-validator/internal/probe"
)

// Text writes a human-readable summary to w.
func Text(w io.Writer, summary probe.ScanSummary) error {
	for _, v := range summary.Verdicts {
		marker := "ok"
		if v.IsStraggler {
			marker = fmt.Sprintf("STRAGGLER: %s (%.1fσ above mean)", v.Reason, v.SigmasAbove)
		}
		fmt.Fprintf(w, "  %-25s mean=%-12v errors=%-2d  %s\n",
			v.Name, v.MeanRTT.Truncate(time.Microsecond), v.Errors, marker)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Summary: %d nodes scanned, %d stragglers, healthy mean=%v\n",
		len(summary.Verdicts),
		summary.StragglerCount,
		summary.HealthyMean.Truncate(time.Microsecond))
	return nil
}

// JSON writes a machine-readable summary to w.
//
// Schema is intentionally flat and explicit so external tools can parse it
// without knowing about Go time.Duration internals: durations are emitted
// as milliseconds (float64) instead of nanoseconds (int64).
func JSON(w io.Writer, summary probe.ScanSummary) error {
	type nodeOut struct {
		Name        string  `json:"name"`
		Endpoint    string  `json:"endpoint"`
		MeanMS      float64 `json:"mean_ms"`
		Iterations  int     `json:"iterations"`
		Errors      int     `json:"errors"`
		IsStraggler bool    `json:"is_straggler"`
		SigmasAbove float64 `json:"sigmas_above"`
		Reason      string  `json:"reason,omitempty"`
	}

	type summaryOut struct {
		Nodes          []nodeOut `json:"nodes"`
		GlobalMeanMS   float64   `json:"global_mean_ms"`
		GlobalStdDevMS float64   `json:"global_stddev_ms"`
		HealthyMeanMS  float64   `json:"healthy_mean_ms"`
		FastestMeanMS  float64   `json:"fastest_mean_ms"`
		StragglerCount int       `json:"straggler_count"`
	}

	out := summaryOut{
		Nodes:          make([]nodeOut, 0, len(summary.Verdicts)),
		GlobalMeanMS:   toMS(summary.GlobalMean),
		GlobalStdDevMS: toMS(summary.GlobalStdDev),
		HealthyMeanMS:  toMS(summary.HealthyMean),
		FastestMeanMS:  toMS(summary.FastestMean),
		StragglerCount: summary.StragglerCount,
	}

	for _, v := range summary.Verdicts {
		out.Nodes = append(out.Nodes, nodeOut{
			Name:        v.Name,
			Endpoint:    v.Endpoint,
			MeanMS:      toMS(v.MeanRTT),
			Iterations:  len(v.Durations),
			Errors:      v.Errors,
			IsStraggler: v.IsStraggler,
			SigmasAbove: v.SigmasAbove,
			Reason:      v.Reason,
		})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

// toMS converts a duration to fractional milliseconds.
func toMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}