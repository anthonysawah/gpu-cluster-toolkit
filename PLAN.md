# GPU Cluster Toolkit

A focused weekend project on GPU cluster reliability tooling. Two small Go services and a written brief, built to explore what production reliability looks like for multi-tenant GPU compute.

This is not a production system. It is a learning artifact, kept honest about what is real, what is mocked, and what would change in production. Every shortcut is documented.

---

## Why this project exists

Site reliability for GPU cloud is a different problem from web service SRE. The unit of work is a tightly-coupled training job spanning hundreds or thousands of accelerators, where one slow node stalls the entire run and a single ECC error can corrupt a checkpoint. The tooling and mental models are still maturing publicly. There is real value in working through them in code.

I built this to:

- Practice writing Go in the operator and CLI patterns that dominate cloud-native infrastructure
- Internalize the GPU cluster operations vocabulary: DCGM, NCCL, NVLink, XID errors, stragglers
- Map my existing SRE background (multi-cluster Kubernetes, Terraform, Ansible, on-call across regulated environments) onto a domain I have not yet operated in
- Have a concrete artifact to point at when discussing the work, instead of just describing it

---

## What is in here

### `gpu-node-guardian`

A Kubernetes controller written in Go using kubebuilder. It watches Node objects, scrapes the NVIDIA DCGM exporter for real GPU health metrics (XID errors, thermal threshold breaches, ECC counters), and cordons nodes when health degrades. Auto-uncordons on recovery. Emits Kubernetes Events so operators can audit decisions.

Real DCGM exporter integration, not a mock. The exporter runs in test mode without GPUs and emits synthetic metrics, which is enough to demonstrate the closed loop.

### `cluster-validator`

A Go CLI that connects to a list of nodes, runs a parallel timing test simulating the all-reduce communication pattern of distributed training, and identifies stragglers (nodes whose latency is more than two standard deviations above the mean). Outputs a structured JSON report. Can optionally annotate straggler nodes in Kubernetes so the guardian controller acts on the result.

Stragglers are the silent killer of large training jobs. One slow GPU stalls a thousand-GPU job. Detecting them automatically is core reliability work for any GPU cloud.

### `notes.md`

A short brief on GPU cluster reliability from an SRE's perspective. What I learned building this, what surprised me, and what I would build next. Honest about gaps, not selling anything.

---

## What I am exploring and learning

### Go (writing, not just reading)

- Module structure (`go.mod`, `go.sum`) and how Go projects are organized
- Standard library: `context`, `time`, `encoding/json`, `net/http`, `os/exec`, `sync`
- Goroutines and channels for parallel timing tests
- Error handling idioms (`if err != nil`, error wrapping with `fmt.Errorf("...%w", err)`)
- Interfaces and the difference between accepting interfaces and returning structs
- The `context.Context` pattern for cancellation and timeouts
- Working with `client-go` and `controller-runtime`

### Kubernetes operator pattern

- The reconcile loop as a state machine
- Idempotency: every Reconcile call must be safe to repeat
- Predicates for filtering which Node changes trigger reconcile
- Self-applied annotations to track controller-managed state
- RBAC manifests and the `+kubebuilder:rbac` markers
- Events vs logs: when to emit which
- How `manager.Manager` orchestrates controllers, caches, and webhooks

### NVIDIA GPU operations vocabulary

- DCGM (Data Center GPU Manager): what it monitors and how the exporter exposes Prometheus metrics
- DCGM exporter test mode: synthetic metrics without real GPUs
- Key metric names: `DCGM_FI_DEV_XID_ERRORS`, `DCGM_FI_DEV_GPU_TEMP`, `DCGM_FI_DEV_ECC_DBE_VOL_TOTAL`
- XID errors: which are recoverable, which require node drain
- NVLink vs InfiniBand vs RoCE: where each lives in the cluster topology
- NCCL collective communication: all-reduce, the pattern that dominates training comms
- Straggler theory: why one slow node stalls the whole job
- GPU Operator on Kubernetes: how NVIDIA packages drivers, runtime, and exporter as a deployment

### Cluster validation patterns

- What "validating a cluster" means in practice before customer handoff
- Parallel measurement patterns in Go (worker pools, fan-out/fan-in)
- Statistical detection: standard deviation as a straggler threshold, why p99 matters more than mean
- Closed-loop systems: detect, annotate, act, audit

### Things I deliberately did not learn

- Rust, eBPF, kernel internals, real bare-metal driver installs. Out of scope. Worth learning later, not in five days.

---

## The build plan

### Day 1: gpu-node-guardian with real DCGM

Working Go controller, real DCGM integration, on GitHub.

Morning
- Toolchain install (Go, kind, kubectl, kubebuilder, Docker Desktop)
- Local kind cluster up
- Scaffold project with kubebuilder
- Basic Reconcile loop using a fake annotation, working end to end

Afternoon
- Deploy NVIDIA DCGM exporter to the kind cluster in test mode
- Modify controller to scrape the exporter's Prometheus endpoint
- Decision logic: cordon if `DCGM_FI_DEV_XID_ERRORS > 0` or `DCGM_FI_DEV_GPU_TEMP > 90`
- Add self-applied annotation `gpu-cluster-toolkit/cordoned=true` for idempotency
- Test cordon and uncordon cycle

Evening
- README for `gpu-node-guardian/`
- Commit and push

### Day 2: cluster-validator basic version

Goal: Go CLI that detects synthetic stragglers across a list of nodes.

Morning
- New Go module: `cluster-validator`
- CLI scaffold using `cobra` or stdlib `flag`
- Input format: YAML file listing nodes (host, port)
- Parallel synthetic all-reduce timing test: each node sends a fixed-size message to a coordinator, gets a response, measures round-trip latency

Afternoon
- Aggregate timings: mean, p99, standard deviation
- Straggler detection: any node with round-trip more than 2 sigma above the mean
- Output: structured JSON report with per-node timings, straggler list, recommendation
- `--simulate-fault` flag that artificially slows one node so the demo works

Evening
- README for `cluster-validator/`
- Commit and push

### Day 3: integration and polish

Goal: the two artifacts work together. Closed-loop demo.

Morning
- Add `--annotate-stragglers` mode to validator: sets `gpu.cluster.io/straggler=true` on straggler nodes
- Modify guardian to also watch for this annotation and cordon on it
- Validator detects, guardian acts. Two components, one system.

Afternoon
- Top-level `Makefile` with `make demo` target
- Better logging and error handling in both projects
- Record a 60-second GIF: fault injected → validator detects → annotation applied → guardian cordons → events emitted

Evening
- Update both READMEs with the integration story
- Commit and push

### Day 4: notes.md

Goal: the brief. Two pages, dense, specific, no fluff.

Morning (3 hours of reading)
- SemiAnalysis ClusterMAX writeups on GPU cloud reliability
- NVIDIA GPU Operator overview docs
- DCGM exporter README
- NCCL introduction
- Slurm architecture overview (slurmctld, slurmd, slurmdbd, partitions, QoS)
- MaaS overview (Canonical's metal-as-a-service)
- Netbox overview

Afternoon (write the brief)
- Section 1: how I think about the GPU cloud stack, from the JD-and-blog-posts level down
- Section 2: what transfers from generalist SRE work, with concrete mappings
- Section 3: what is new and how an SRE would ramp on the GPU-specific layer
- Section 4: what I would build next, with the cluster-validator as a sketch of direction

Evening
- Tighten language, no em dashes, no AI-flavored phrasing
- Commit and push

### Day 5: top-level README and ship

Goal: tie everything together, polish, public.

Morning
- Top-level README: one-paragraph overview, three artifacts with one-sentence summaries each, embed the demo GIF, "what I learned" section
- Verify everything still works from a clean clone

Afternoon
- Polish all three READMEs for consistency and tone
- Re-record the demo GIF if it can be improved
- Final push to GitHub
- Make repo public

---

## What "good" looks like at the end

A public GitHub repo with this structure:

```
gpu-cluster-toolkit/
├── README.md
├── Makefile
├── notes.md
├── gpu-node-guardian/
│   ├── README.md
│   ├── go.mod
│   ├── cmd/
│   ├── internal/controller/
│   ├── config/
│   └── ...kubebuilder scaffolding
└── cluster-validator/
    ├── README.md
    ├── go.mod
    ├── main.go
    ├── internal/
    └── examples/
```

A 60-second GIF embedded in the top-level README showing the closed-loop demo.

A two-page brief that demonstrates I have actually thought about how GPU cluster reliability works.

Approximately 30 hours of work spread across five days.

---

## Daily discipline

- Start each day by reading this plan and the prior day's README
- End each day by committing and pushing, even if incomplete
- Do not over-engineer. Ship the small thing, not the perfect thing.
- Honesty in READMEs: mark what is mocked, what is real, what would change in production
- No em dashes anywhere. No AI-flavored phrasing. No overclaiming.

---

## What this is not

This is not a production system. It is not a hiring exercise. It is not a job application.

It is a focused exploration of a domain I find interesting and want to operate in next. The artifacts are real code, the brief is real thinking, and the gaps are honestly labeled. Anything beyond that scope is feature creep.

---

## Mental checklist before going public

- [ ] All three artifacts run from a fresh clone with documented commands
- [ ] Demo GIF works and embeds correctly in the top-level README
- [ ] No em dashes anywhere in any markdown file
- [ ] No technologies claimed that are not actually used in this repo
- [ ] Honest "what is mocked" sections in each README
- [ ] notes.md is two pages, not three, not one
- [ ] Top-level README is the framing document, not a wall of text
- [ ] Repo is public on GitHub
