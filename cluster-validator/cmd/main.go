// Package main is the entry point for cluster-validator.
//
// cluster-validator probes a list of nodes with synthetic all-reduce-style
// traffic, times the round trips, and reports any nodes that are significantly
// slower than their peers (stragglers).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/anthonysawah/gpu-cluster-toolkit/cluster-validator/internal/config"
	"github.com/anthonysawah/gpu-cluster-toolkit/cluster-validator/internal/output"
	"github.com/anthonysawah/gpu-cluster-toolkit/cluster-validator/internal/probe"
)

// Options holds all CLI flags after parsing.
type Options struct {
	NodesFile          string
	Iterations         int
	PayloadBytes       int
	SimulateFault      string
	AnnotateStragglers bool
	OutputFormat       string
	ThresholdSigma     float64
}

func main() {
	opts := parseFlags()

	cfg, err := config.Load(opts.NodesFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	reqs := make([]probe.ScanRequest, 0, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		reqs = append(reqs, probe.ScanRequest{
			Name:          n.Name,
			Endpoint:      n.Endpoint,
			Iterations:    opts.Iterations,
			PayloadBytes:  opts.PayloadBytes,
			SimulateFault: n.Name == opts.SimulateFault,
		})
	}

	// Progress message goes to stderr so it doesn't corrupt structured output on stdout.
	fmt.Fprintf(os.Stderr, "Scanning %d nodes (%d iterations each, %d bytes payload)...\n",
		len(reqs), opts.Iterations, opts.PayloadBytes)

	results := probe.Scan(ctx, reqs)
	summary := probe.DetectStragglers(results, opts.ThresholdSigma)

	switch opts.OutputFormat {
	case "json":
		if err := output.JSON(os.Stdout, summary); err != nil {
			fmt.Fprintf(os.Stderr, "error rendering json: %v\n", err)
			os.Exit(1)
		}
	case "text", "":
		if err := output.Text(os.Stdout, summary); err != nil {
			fmt.Fprintf(os.Stderr, "error rendering text: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "error: unknown output format %q (valid: text, json)\n", opts.OutputFormat)
		os.Exit(2)
	}

	if opts.AnnotateStragglers {
		if err := annotateStragglers(ctx, summary); err != nil {
			fmt.Fprintf(os.Stderr, "annotate-stragglers failed: %v\n", err)
			os.Exit(1)
		}
	}

	if summary.StragglerCount > 0 {
		os.Exit(1)
	}
}

func parseFlags() Options {
	var opts Options

	flag.StringVar(&opts.NodesFile, "nodes", "", "path to YAML file listing nodes to probe (required)")
	flag.IntVar(&opts.Iterations, "iterations", 10, "number of probes per node")
	flag.IntVar(&opts.PayloadBytes, "payload-bytes", 1024*1024, "size of probe payload in bytes")
	flag.StringVar(&opts.SimulateFault, "simulate-fault", "", "node name to artificially slow (for demo)")
	flag.BoolVar(&opts.AnnotateStragglers, "annotate-stragglers", false, "set straggler annotation on detected nodes")
	flag.StringVar(&opts.OutputFormat, "output", "text", "output format: text or json")
	flag.Float64Var(&opts.ThresholdSigma, "threshold-sigma", 2.0, "stddev threshold for straggler detection")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "cluster-validator: detect straggler nodes in a GPU cluster\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s --nodes <path> [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if opts.NodesFile == "" {
		fmt.Fprintln(os.Stderr, "error: --nodes is required")
		flag.Usage()
		os.Exit(2)
	}

	return opts
}

// annotateStragglers is the placeholder for Day 3's Kubernetes integration.
// Currently logs what would be annotated; Day 3 replaces this with real patches
// using sigs.k8s.io/controller-runtime/pkg/client.
func annotateStragglers(ctx context.Context, summary probe.ScanSummary) error {
	count := 0
	for _, v := range summary.Verdicts {
		if !v.IsStraggler {
			continue
		}
		fmt.Fprintf(os.Stderr, "[annotate-stragglers] would set gpu.cluster.io/straggler=true on Node %s (reason: %s)\n",
			v.Name, v.Reason)
		count++
	}
	if count == 0 {
		fmt.Fprintln(os.Stderr, "[annotate-stragglers] no stragglers to annotate")
	}
	return nil
}