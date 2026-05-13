# gpu-cluster-toolkit

Reliability tooling for GPU clusters, built as a focused portfolio project on AI infrastructure SRE. Two small Go services and a written brief, kept honest about what is real and what is mocked.

This is not a production system. It is a learning artifact. The patterns in here are the same patterns NVIDIA, GPU clouds, and AI labs use, applied to a problem space (GPU cluster operations) that the public open-source ecosystem is still catching up to.

## Why this project exists

SRE for GPU cloud is a different problem from SRE for web services. The unit of work is a tightly-coupled training job spanning hundreds or thousands of accelerators. One slow GPU stalls the entire run. One ECC error corrupts a checkpoint. The mental models and tooling are still maturing publicly.

I built this to:

- Practice writing Go in the operator and CLI patterns that dominate cloud-native infrastructure
- Internalize the GPU cluster operations vocabulary: DCGM, NCCL, NVLink, XID errors, stragglers
- Map my existing SRE background (multi-cluster Kubernetes, Terraform, Ansible, on-call in regulated environments) onto a domain I have not yet operated in
- Have a concrete artifact to point at when discussing the work, instead of describing it

## What is in here

### [`gpu-node-guardian/`](./gpu-node-guardian/) — the reactive half

A Kubernetes controller written in Go using kubebuilder. Watches Node objects, scrapes the NVIDIA DCGM exporter for per-node GPU health, and cordons nodes whose GPUs are throwing errors or running too hot. Auto-uncordons on recovery. Emits Kubernetes Events so operators can audit decisions.

Real DCGM exporter integration. The exporter runs in test mode without GPUs, emits synthetic metrics that follow the real DCGM format. The Go code parses it as if it were real.

### [`cluster-validator/`](./cluster-validator/) — the proactive half

A Go CLI that connects to a list of nodes, sends synthetic all-reduce-style traffic in parallel, times the round trips, and flags statistical outliers as stragglers. Outputs text or JSON. Optionally annotates Kubernetes Node objects so the controller can act on its findings.

Stragglers are the silent killer of large training jobs. One slow GPU stalls the entire run. DCGM metrics catch hardware failures, but a node that is just a little bit slow is invisible to error counters. The validator catches that.

### Coming soon

- **Day 3**: integrate the two artifacts. `cluster-validator --annotate-stragglers` writes a Kubernetes annotation; `gpu-node-guardian` watches the annotation and cordons. Closed-loop demo.
- **Day 4**: a written brief on GPU cluster reliability from an SRE's perspective. What I learned building this, what surprised me, what I would build next.
- **Day 5**: top-level `make demo`, polish, a 60-second GIF of the closed loop.

## Quick demo

```bash
# Build a kind cluster with NodePort port mappings.
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

# Run the validator against the mock echo servers.
cd cluster-validator
kubectl apply -f config/echo/echo-server.yaml
go build -o bin/cluster-validator ./cmd
./bin/cluster-validator --nodes examples/nodes.yaml --iterations 10 --simulate-fault gpu-toolkit-worker
```

Output:
Scanning 2 nodes (10 iterations each, 1048576 bytes payload)...
gpu-toolkit-worker        mean=58.3ms       errors=0   STRAGGLER: more than 2x fastest node (1.0σ above mean)
gpu-toolkit-worker2       mean=7.9ms        errors=0   ok
Summary: 2 nodes scanned, 1 stragglers, healthy mean=7.9ms

See each subdirectory's README for the full demo of that component.

## What is mocked, what is real

The honest table:

| Concern | Mocked | Real |
|---|---|---|
| GPU metrics | Static text served by Nginx | DCGM exporter format, parsed by Prometheus library |
| GPU hardware | None present | Code structure assumes none anyway, by design |
| All-reduce traffic | HTTP echo round trips | Synchronous fan-out, parallel timing, statistical detection |
| Failure injection | `--simulate-fault` adds 50ms | Outlier math, threshold logic, exit codes |
| Kubernetes API | Local kind cluster | Real K8s API: Watch, Patch, Events, RBAC, Runnable interface |
| Cordon mechanics | Identical to `kubectl cordon` | Same code path the real tool uses |

To make this production-ready against real GPUs, the DCGM mock is replaced with NVIDIA's GPU Operator, and the HTTP echo target is replaced with `nccl-tests`. Every other layer stays.

## Project structure
gpu-cluster-toolkit/
├── README.md                            (this file)
├── PLAN.md                              the working plan I built to
├── gpu-node-guardian/                   Kubernetes controller
│   ├── README.md                        full design and demo
│   ├── cmd/                             entry point
│   ├── internal/controller/             reconcile loop + DCGM scraper
│   ├── internal/dcgm/                   Prometheus text parser
│   └── config/dcgm-mock/                mock exporter manifest
└── cluster-validator/                   parallel probing CLI
├── README.md                        full design and demo
├── cmd/                             entry point
├── internal/probe/                  HTTP timing + parallel fan-out + outlier detection
├── internal/stats/                  mean and stddev
├── internal/output/                 text and JSON renderers
└── config/echo/                     mock echo server manifest

## Engineering principles I applied

A few that show up across both artifacts and are worth calling out:

**Separation of data source from decision logic.** Both artifacts decouple "where does the data come from" from "what do we do about it." `gpu-node-guardian` reads annotations, regardless of who set them. `cluster-validator` consumes a YAML file, not a hardcoded list. Substituting one data source for another is a config change, not a code change.

**Honest mocking.** Where real hardware or external systems are not available, mock them. Label every mock in the README. Make the substitution path obvious: "swap this manifest, swap this URL, code unchanged."

**Tests where they matter.** Pure functions get unit tests. HTTP code uses `httptest` for realistic in-process testing. No mocks for things that have real test infrastructure available.

**Layered detection rules.** The straggler detector uses both a statistical rule (sigma above mean) and an absolute rule (multiple of baseline). Each catches what the other misses. Production reliability tools work this way; toy demos often pick one and call it done.

**Senior IC framing.** READMEs explain the why, not just the how. Design decisions are surfaced with reasoning. Future work sections describe known-but-unimplemented improvements. This is the kind of project structure that should make an interviewer's job easy.

## About this project

I am a senior infrastructure engineer with deep Kubernetes operations and database experience but limited Go writing experience and no GPU-specific exposure. This project is me closing both gaps in public.

If you found this useful, broke something, or want to discuss the design, open an issue or reach out: [anthonysawah@live.com](mailto:anthonysawah@live.com).