# mini-volcano-job

> A minimal batch-scheduling engine extracted from [Volcano](https://github.com/volcano-sh/volcano) (~250k lines of Go).
> Keeps the core ideas, drops 97% of the code — purpose-built for learning and research.

[![Go](https://img.shields.io/badge/Go-1.26.0-00ADD8?logo=go)](https://go.dev/)
[![k8s](https://img.shields.io/badge/k8s.io-v0.36.0-326CE5?logo=kubernetes)](https://kubernetes.io/)
[![Phase](https://img.shields.io/badge/phase-1%2F4-blue)](#blueprint)

---

## Table of Contents

- [What Is This](#what-is-this)
- [Blueprint](#blueprint)
- [Architecture](#architecture)
- [Directory Structure](#directory-structure)
- [Quick Start](#quick-start)
- [Differences from Volcano](#differences-from-volcano)
- [Design Decisions](#design-decisions)

---

## What Is This

**mini-volcano-job** is a pedagogical implementation of a Kubernetes batch-scheduling engine.

The upstream Volcano project spans **1,919 files and 252,195 lines of Go**, including a full scheduler, 24+ scheduling plugins, multiple controllers, webhooks, agents, and more. mini-volcano-job extracts only the core Job-management and Gang Scheduling logic, reimplementing a runnable minimum in **~3,000 lines of Go**.

### Core Concepts

```
Job ─── owns ─── PodGroup ─── schedules ─── Pods (atomically)
 │                  │
 │  tasks[]         │  minMember
 │  minAvailable    │  queue
 │  queue           │
 └──────────────────┘
```

- **Job** — a batch workload composed of one or more Tasks (each Task = pod template × replicas).
- **PodGroup** — the Gang Scheduling primitive: a set of pods that are scheduled all-or-nothing.
- **Queue** — a named priority bucket for resource allocation (implemented in Phase 3).

---

## Blueprint

### Phase 1 — API Types + CRDs ✅ Done

| Artifact | Description |
|----------|-------------|
| `api/v1alpha1/types.go` | Job + PodGroup + sub-types, 5-phase Job state machine |
| `api/v1alpha1/register.go` | Scheme registration |
| `api/v1alpha1/zz_generated.deepcopy.go` | DeepCopy methods (hand-written; replaced by codegen in Phase 2) |
| `config/crd/jobs.mini-volcano.sh.yaml` | Job CRD |
| `config/crd/podgroups.mini-volcano.sh.yaml` | PodGroup CRD |
| `config/examples/` | Single-task + multi-task DAG examples |

### Phase 2 — Job Controller ⬜ Planned

| Module | Description | Est. LOC |
|--------|-------------|----------|
| `pkg/controller/job_controller.go` | Main reconcile loop | ~400 |
| `pkg/controller/job_state.go` | 5-state state machine (Pending→Running→Completed/Failed→Terminating) | ~200 |
| `pkg/controller/pod_control.go` | Pod create / delete / track | ~150 |
| `pkg/controller/podgroup_control.go` | PodGroup auto-creation & status sync | ~100 |
| `cmd/controller-manager/main.go` | Entry point | ~80 |

**State machine transitions:**

```
                    ┌──────────┐
                    │  Pending │ ←── initial
                    └────┬─────┘
               minAvailable met
                         │
                    ┌────▼─────┐
            ┌───────│  Running │───────┐
            │       └────┬─────┘       │
      task failures      │       all Succeeded
     within retry   minSuccess         │
            │        met / all done    │
       ┌────▼─────┐              ┌────▼──────┐
       │  Failed  │              │ Completed │
       └──────────┘              └───────────┘
            │
   terminate / timeout (any phase)
            │
    ┌───────▼────────┐
    │  Terminating   │
    └───────┬────────┘
            │ all pods cleaned up
    ┌───────▼────────┐
    │ (pods deleted) │
    └────────────────┘
```

### Phase 3 — Minimal Scheduler ⬜ Planned

| Module | Description | Est. LOC |
|--------|-------------|----------|
| `pkg/scheduler/session.go` | Scheduling session framework | ~200 |
| `pkg/scheduler/framework.go` | Action / Plugin interfaces | ~100 |
| `pkg/scheduler/actions/enqueue.go` | Enqueue — priority-ordered queueing | ~60 |
| `pkg/scheduler/actions/allocate.go` | Allocate — FIFO + binpack | ~150 |
| `pkg/scheduler/plugins/gang.go` | Gang — minMember gate | ~80 |
| `pkg/scheduler/plugins/predicates.go` | Node filtering (resources / affinity) | ~100 |
| `pkg/scheduler/plugins/nodeorder.go` | Node scoring | ~60 |
| `pkg/scheduler/cache.go` | Scheduler Cache (Informer-driven) | ~300 |
| `cmd/scheduler/main.go` | Entry point | ~80 |

**Scheduling pipeline (one cycle):**

```
enqueue → allocate → backfill
  │          │          │
  │    (gang check,  (fragment
  │     binpack)      backfill)
  │
  ordered by priority + creationTime
```

### Phase 4 — CLI + Examples ⬜ Planned

| Component | Description | Est. LOC |
|-----------|-------------|----------|
| `cmd/cli/main.go` | CLI entry point (cobra) | ~80 |
| `cmd/cli/submit.go` | `mvj submit -f job.yaml` | ~100 |
| `cmd/cli/list.go` | `mvj get jobs` | ~80 |
| `cmd/cli/delete.go` | `mvj delete job <name>` | ~60 |
| `cmd/cli/describe.go` | `mvj describe job <name>` | ~100 |
| `config/examples/` | More examples (MPI, TensorFlow-style) | ~100 |

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│  kubectl apply -f job.yaml                           │
│  kubectl get vj                                       │
└───────────────────────┬──────────────────────────────┘
                        │
┌───────────────────────▼──────────────────────────────┐
│  Kubernetes API Server                               │
│  ┌─────────────────┐  ┌─────────────────────────────┐│
│  │ Job CRD          │  │ PodGroup CRD                ││
│  │ (mini-volcano    │  │ (mini-volcano               ││
│  │  .sh/v1alpha1)   │  │  .sh/v1alpha1)              ││
│  └────────┬────────┘  └──────────────┬──────────────┘│
└───────────┼──────────────────────────┼────────────────┘
            │                          │
┌───────────▼──────────────────────────▼────────────────┐
│  Controller-Manager (Phase 2)                         │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │ Job Ctrl    │  │ PodGroup Ctrl│  │ Queue Ctrl   │ │
│  │ (state      │  │ (status      │  │ (Phase 3)    │ │
│  │  machine)   │  │  sync)       │  │              │ │
│  └──────┬──────┘  └──────┬───────┘  └──────────────┘ │
│         │                │                             │
│         └────────┬───────┘                             │
│                  │ create / update Pods                │
└──────────────────┼─────────────────────────────────────┘
                   │
┌──────────────────▼─────────────────────────────────────┐
│  Scheduler (Phase 3)                                   │
│  ┌─────────────────────────────────────────────────┐  │
│  │ Session                                         │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐        │  │
│  │  │ enqueue  │→│ allocate │→│ backfill │        │  │
│  │  └──────────┘ └──────────┘ └──────────┘        │  │
│  │  ┌──────────────────────────────────────────┐  │  │
│  │  │ Plugins: gang | predicates | nodeorder   │  │  │
│  │  └──────────────────────────────────────────┘  │  │
│  └─────────────────────────────────────────────────┘  │
│  Cache (Informer-driven snapshot)                      │
└────────────────────────────────────────────────────────┘
```

---

## Directory Structure

```
mini-volcano-job/
├── README.md                            # ← this file (English)
├── README.zh-CN.md                      # 简体中文版 (Simplified Chinese)
├── README.zh-TW.md                      # 繁體中文版 (Traditional Chinese)
├── go.mod                              # Go 1.26 + k8s v0.36
├── api/
│   └── v1alpha1/
│       ├── doc.go                      # package doc + codegen markers
│       ├── types.go                    # Job / PodGroup type definitions
│       ├── register.go                 # Scheme registration
│       └── zz_generated.deepcopy.go    # DeepCopy methods
├── config/
│   ├── crd/
│   │   ├── jobs.mini-volcano.sh.yaml
│   │   └── podgroups.mini-volcano.sh.yaml
│   └── examples/
│       ├── job-nginx.yaml              # single-task example
│       └── job-multi-task.yaml         # multi-task + DAG example
├── cmd/                                # Phase 2+ (entry points)
├── pkg/                                # Phase 2+ (core logic)
└── hack/                               # Phase 2+ (code generation)
```

---

## Quick Start

### Install the CRDs

```bash
kubectl apply -f config/crd/jobs.mini-volcano.sh.yaml
kubectl apply -f config/crd/podgroups.mini-volcano.sh.yaml
```

### Create a Job

```bash
kubectl apply -f config/examples/job-nginx.yaml
kubectl get vj          # list mini-volcano Jobs
kubectl get pg          # list PodGroups
```

### Inspect a Job

```bash
kubectl get vj nginx-example -o yaml
kubectl describe vj nginx-example  # Phase 4
```

---

## Differences from Volcano

| Dimension | Volcano | mini-volcano-job |
|-----------|---------|------------------|
| Go LOC | 252,195 | ~3,000 (target) |
| Files | 1,135 | ~25 |
| Job phases | 10 (incl. Aborting, Aborted, Restarting) | **5** (Pending, Running, Completed, Failed, Terminating) |
| Scheduler actions | 8 | **3** (enqueue, allocate, backfill) |
| Scheduler plugins | 24+ | **3** (gang, predicates, nodeorder) |
| Scheduling policies | gang, binpack, DRF, proportion, priority, NUMA, deviceshare, SLA, TDM, ... | **gang + binpack** |
| CRDs | 11 | **2** (Job + PodGroup) |
| API groups | batch / scheduling / bus | **1** (`mini-volcano.sh`) |
| JobFlow | ✅ DAG orchestration | ❌ |
| CronJob | ✅ | ❌ |
| Queue Controller | ✅ | Phase 3 |
| Webhook | ✅ admission validation | ❌ |
| Agent | ✅ node daemon | ❌ |
| CLI commands | 7 | **4** (Phase 4) |
| Go dependencies | ~100 modules | **2** (k8s.io/api + apimachinery) |

### What We Kept

- **Job → PodGroup → Pods** gang-scheduling core model
- **State-machine-driven** Job lifecycle management
- **Session-Plugin** extensible scheduling framework
- **FIFO + binpack** fundamental scheduling policy

### What We Dropped

- Preemption, reclaim, gang-preempt / gang-reclaim — essential in production, optional for learning
- Multi-queue fair-share (proportion, DRF)
- NUMA, device-sharing, network-topology awareness
- JobFlow DAG orchestration
- Admission webhooks
- Node agent

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Single API group (`mini-volcano.sh`) | Low scale — three groups would be over-engineering |
| Hand-written deepcopy | No codegen toolchain in Phase 1; replaced by `controller-gen` in Phase 2 |
| `x-kubernetes-preserve-unknown-fields` on pod template | Embeds full `PodTemplateSpec` without field-by-field CRD schema |
| k8s.io v0.36.0 + Go 1.26 | Latest stable pair (mid-2026) |
| Zero `controller-runtime` dependency | Phases 1–2 use apimachinery native APIs only; optional adoption in Phase 3 |
| `localSchemeBuilder` pattern | Matches standard K8s registration convention; supports future `init()` type registration |

---

## License

Apache 2.0 — consistent with upstream Volcano.
