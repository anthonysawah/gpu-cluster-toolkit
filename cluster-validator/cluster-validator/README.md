# cluster-validator

A Go CLI that detects straggler nodes in a GPU cluster by sending synthetic all-reduce-style traffic to each node, timing the round trips, and flagging statistical outliers.

This is the proactive half of [gpu-cluster-toolkit](../README.md). The reactive half, [gpu-node-guardian](../gpu-node-guardian/), cordons nodes when DCGM reports hardware errors. cluster-validator catches the failure mode DCGM doesn't see: nodes that are just slow, the silent killer in distributed training.

## What it does
[validator] ──parallel HTTP POST──┬──> [echo pod on node-01]
├──> [echo pod on node-02]
└──> [echo pod on node-N]
│
▼
time each round trip
│
▼
compute mean, stddev, baseline
│
▼
flag outliers (statistical + absolute rules)
│
▼
text / json / annotate node

One probe is: build a 1MB payload, POST it to the node, read the full response, record the round trip. Each node gets N probes (default 10) sequentially. All nodes run in parallel. After the scan, the tool aggregates per-node means and flags any node that meets either:

1. **Statistical rule**: mean RTT is more than `--threshold-sigma` standard deviations above the global mean.
2. **Absolute rule**: mean RTT is more than 2x the fastest node's mean.

A node is a straggler if either rule triggers. The output reports which rule caught it.

## Why two rules

The statistical rule alone has a known weakness: a single extreme outlier inflates its own standard deviation enough to hide itself. With 4 nodes at 10ms and 1 at 100ms, the global stddev is ~39ms, the 2σ threshold sits at ~108ms, and the 100ms straggler slips through.

The absolute rule is the safety net. A node that is more than 2x the baseline is suspicious regardless of how the rest of the cluster is distributed. Layered rules like this are standard in production cluster validation.

A future version should use median absolute deviation (MAD) instead of mean and stddev for the statistical rule. MAD is robust against outliers by design. See Future Work.

## Why this design

**File-based config over Kubernetes auto-discovery.** The tool takes a YAML list of nodes rather than reading them from the K8s API. This keeps it decoupled from kubeconfig, RBAC, and cluster credentials. A version that adds `--auto-discover` mode would be cheap to extend; defaulting to file-based config keeps the tool portable.

**Single CLI, no subcommands.** The tool does one thing: probe a list of nodes, report stragglers. Standard library `flag` package is sufficient. No cobra. Less code, fewer dependencies.

**Probes share a single HTTP client.** A package-level `http.Client` is reused across all probes. Without this, every probe pays a TCP handshake cost that would dominate timings and make results meaningless. Connection pooling matters when you're measuring milliseconds.

**Drain the response body before stopping the timer.** A common bug in network timing tools is timing only to the headers, not the full body transfer. `io.Copy(io.Discard, resp.Body)` forces a read to EOF before `time.Since(start)` is called.

**Parallel fan-out, sequential per-node.** The N nodes are probed concurrently (one goroutine each), but within a node the M iterations run sequentially. This isolates per-node noise from cross-node measurement skew.

## Architecture
cluster-validator/
├── cmd/main.go                              # CLI entry point, flag parsing, dispatch
├── internal/config/                         # YAML parser for the node list
│   ├── config.go
│   └── config_test.go
├── internal/probe/                          # the core logic
│   ├── probe.go                             # single-probe HTTP timing
│   ├── scanner.go                           # parallel fan-out across nodes
│   ├── stragglers.go                        # outlier detection
│   └── *_test.go
├── internal/stats/                          # mean and stddev primitives
│   ├── stats.go
│   └── stats_test.go
├── internal/output/                         # text and JSON renderers
│   └── output.go
├── examples/nodes.yaml                      # example config file
└── config/echo/echo-server.yaml             # mock workload for local testing

Five packages, single responsibilities:

- **config**: load and validate the YAML node list. No network, no Kubernetes.
- **probe**: time a single HTTP round trip. No state, no concurrency.
- **probe (scanner.go)**: orchestrate parallel probes using goroutines and channels.
- **probe (stragglers.go)**: pure math on a slice of NodeResults.
- **stats**: arithmetic mean and population standard deviation.
- **output**: render a ScanSummary as text or JSON.

## What's mocked vs real

**Mocked:**
- The probe target is a generic HTTP echo server, not a real NCCL all-reduce. The shape of the traffic (synchronous fan-out, blocking round trip, fixed payload) is structurally similar but the semantics are very different. Real NCCL has collective operations, bandwidth-vs-latency tradeoffs, ring vs tree topologies. We measure none of that.
- The `--simulate-fault` flag adds a fixed 50ms to recorded timings for the named node. This guarantees a reproducible demo without requiring real failure injection (like Linux `tc` traffic control, packet drops, or thermal throttling).
- The `--annotate-stragglers` flag currently only logs what it would annotate. The Day 3 integration with gpu-node-guardian replaces this with a real Kubernetes Node patch.

**Real:**
- The Go code is real. Standard library HTTP, goroutines with WaitGroup-coordinated channels, context-based cancellation.
- The statistics are real. Population standard deviation, sigma-above-mean diagnostics, outlier thresholds.
- The two-rule detection logic mirrors what production cluster validation tools actually do.
- Exit codes are real. Stragglers found returns exit code 1, useful in CI pipelines that should fail on bad cluster health.
- The JSON output schema is suitable for piping into alerting, dashboards, or downstream automation.

To make this production-ready against real GPUs, the probe target would be replaced with NVIDIA's `nccl-tests` binary running as a DaemonSet on each node, and the per-probe HTTP measurement would be replaced with the actual all-reduce bandwidth and latency numbers from those tests. The straggler detection layer above stays the same.

## Running it locally

Prerequisites: Docker Desktop, [kind](https://kind.sigs.k8s.io/), kubectl, Go 1.22+, optionally `jq`.

```bash
# 1. Create a kind cluster with NodePort port mappings for the echo servers.
kind create cluster --name gpu-toolkit --config=- <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
  extraPortMappings:
  - containerPort: 30081
    hostPort: 30081
- role: worker
  extraPortMappings:
  - containerPort: 30082
    hostPort: 30082
EOF

# 2. Deploy the mock echo servers (one per worker node).
kubectl apply -f config/echo/echo-server.yaml

# 3. Wait for both echo pods to be Running.
kubectl get pods -n validator-echo -w

# 4. Build the tool.
go build -o bin/cluster-validator ./cmd

# 5. Run a clean scan: both nodes should be reported as ok.
./bin/cluster-validator --nodes examples/nodes.yaml --iterations 10

# 6. Run a scan with a simulated fault on one node.
./bin/cluster-validator --nodes examples/nodes.yaml \
    --iterations 10 \
    --simulate-fault gpu-toolkit-worker

# 7. Same scan, JSON output, piped through jq.
./bin/cluster-validator --nodes examples/nodes.yaml \
    --iterations 10 \
    --simulate-fault gpu-toolkit-worker \
    --output json | jq .

# 8. Same scan, with annotation dry-run (real K8s patches arrive in Day 3).
./bin/cluster-validator --nodes examples/nodes.yaml \
    --iterations 10 \
    --simulate-fault gpu-toolkit-worker \
    --annotate-stragglers
```

## Sample output

Healthy cluster:
Scanning 2 nodes (10 iterations each, 1048576 bytes payload)...
gpu-toolkit-worker        mean=8.4ms        errors=0   ok
gpu-toolkit-worker2       mean=8.7ms        errors=0   ok
Summary: 2 nodes scanned, 0 stragglers, healthy mean=8.5ms

With a simulated fault:
Scanning 2 nodes (10 iterations each, 1048576 bytes payload)...
gpu-toolkit-worker        mean=58.3ms       errors=0   STRAGGLER: more than 2x fastest node (1.0σ above mean)
gpu-toolkit-worker2       mean=7.9ms        errors=0   ok
Summary: 2 nodes scanned, 1 stragglers, healthy mean=7.9ms

JSON output (excerpt):

```json
{
  "nodes": [
    {
      "name": "gpu-toolkit-worker",
      "mean_ms": 58.3,
      "is_straggler": true,
      "reason": "more than 2x fastest node"
    }
  ],
  "global_mean_ms": 33.1,
  "fastest_mean_ms": 7.9,
  "straggler_count": 1
}
```

## Flags

| Flag | Default | Purpose |
| --- | --- | --- |
| `--nodes` | (required) | path to YAML file listing nodes to probe |
| `--iterations` | 10 | probes per node |
| `--payload-bytes` | 1048576 | bytes per probe |
| `--threshold-sigma` | 2.0 | stddev threshold for the statistical rule |
| `--simulate-fault NAME` | (none) | add 50ms to recorded times for the named node |
| `--annotate-stragglers` | false | dry-run today, real K8s patches on Day 3 |
| `--output` | text | output format: `text` or `json` |

Exit code is 1 when stragglers are detected, 0 otherwise. Useful in CI scripts.

## Future work

In rough order of value:

1. **Wire `--annotate-stragglers` to a real Kubernetes client.** Day 3 work. Patches Node objects directly, integrating with gpu-node-guardian so the controller cordons stragglers automatically.
2. **Replace mean and stddev with median absolute deviation (MAD).** Robust to outliers by construction, fixes the small-N edge case that the absolute-slowdown rule currently patches over.
3. **Real NCCL probes.** Run `nccl-tests` as a DaemonSet on each node and consume its results instead of synthetic HTTP. Same straggler detection layer applies.
4. **Continuous mode.** Today the tool is one-shot. A daemon mode that probes every 60s and exports Prometheus metrics would let operators graph straggler frequency over time.
5. **Per-pair probing.** Instead of all-to-coordinator, probe N(N-1)/2 pairs to find topology problems where any two specific nodes are slow with each other.
6. **Configurable per-probe timeout, jitter, and warmup.** Today the per-probe timeout is hardcoded at 10s. Production tools tune these per-workload.

## What I learned building this

I came to this with deep Kubernetes operations experience but no Go writing background. Two days of building gpu-cluster-toolkit (this plus the controller) taught me three things that compound on each other.

Channels are simpler than the tutorials make them seem. Once you internalize "a channel is a typed pipe between goroutines, sends block until someone receives, range over it until close," the parallel fan-out pattern in `scanner.go` writes itself.

Network timing tools have a measurement-vs-method problem: how you time something changes what you measure. The "drain the response body before stopping the clock" detail and the "share a single HTTP client" detail aren't optimizations, they're correctness fixes. Without them, you're not measuring latency, you're measuring artifacts of your own code.

Cluster validation lives in the gap between "is this hardware working" (DCGM territory) and "is this workload happy" (application metrics). The interesting failures are subtle: a node that's 30% slow on small transfers but fine on big ones, a node that's fast in isolation but slow when paired with one specific peer. Catching these requires active probing, not passive monitoring. Building this gave me a concrete sense of why GPU clouds invest so heavily in pre-handoff validation suites.