# mini-volcano-job

> 从 [Volcano](https://github.com/volcano-sh/volcano)（约 25 万行 Go 代码）中提取的最小批量调度引擎。
> 保留核心思想，去除 97% 的代码——专为学习和研究打造。

[![Go](https://img.shields.io/badge/Go-1.26.0-00ADD8?logo=go)](https://go.dev/)
[![k8s](https://img.shields.io/badge/k8s.io-v0.36.0-326CE5?logo=kubernetes)](https://kubernetes.io/)
[![Phase](https://img.shields.io/badge/phase-1%2F4-blue)](#蓝图)

---

## 目录

- [项目简介](#项目简介)
- [蓝图](#蓝图)
- [架构](#架构)
- [目录结构](#目录结构)
- [快速开始](#快速开始)
- [与 Volcano 的差异](#与-volcano-的差异)
- [设计决策](#设计决策)

---

## 项目简介

**mini-volcano-job** 是一个面向教学的 Kubernetes 批量调度引擎实现。

上游 Volcano 项目横跨 **1,919 个文件、252,195 行 Go 代码**，包含完整的调度器、24+ 个调度插件、多个控制器、Webhook、Agent 等。mini-volcano-job 仅提取核心的作业管理和 Gang 调度逻辑，用 **约 3,000 行 Go 代码** 实现一个可运行的最小版本。

### 核心概念

```
Job ─── 拥有 ─── PodGroup ─── 原子调度 ─── Pods
 │                  │
 │  tasks[]         │  minMember
 │  minAvailable    │  queue
 │  queue           │
 └──────────────────┘
```

- **Job** — 由一个或多个 Task 组成的批量工作负载（每个 Task = Pod 模板 × 副本数）。
- **PodGroup** — Gang 调度原语：一组 Pod 要么全部调度成功，要么全部不调度。
- **Queue** — 资源分配的命名优先级桶（Phase 3 实现）。

---

## 蓝图

### Phase 1 — API 类型 + CRD ✅ 已完成

| 产物 | 描述 |
|------|------|
| `api/v1alpha1/types.go` | Job + PodGroup + 子类型，5 阶段 Job 状态机 |
| `api/v1alpha1/register.go` | Scheme 注册 |
| `api/v1alpha1/zz_generated.deepcopy.go` | DeepCopy 方法（手写；Phase 2 替换为代码生成） |
| `config/crd/jobs.mini-volcano.sh.yaml` | Job CRD |
| `config/crd/podgroups.mini-volcano.sh.yaml` | PodGroup CRD |
| `config/examples/` | 单任务 + 多任务 DAG 示例 |

### Phase 2 — Job 控制器 ⬜ 计划中

| 模块 | 描述 | 预估代码量 |
|------|------|-----------|
| `pkg/controller/job_controller.go` | 主协调循环 | ~400 行 |
| `pkg/controller/job_state.go` | 5 状态机（Pending→Running→Completed/Failed→Terminating） | ~200 行 |
| `pkg/controller/pod_control.go` | Pod 创建 / 删除 / 追踪 | ~150 行 |
| `pkg/controller/podgroup_control.go` | PodGroup 自动创建与状态同步 | ~100 行 |
| `cmd/controller-manager/main.go` | 入口 | ~80 行 |

**状态机转换图：**

```
                    ┌──────────┐
                    │  Pending │ ←── 初始状态
                    └────┬─────┘
               minAvailable 满足
                         │
                    ┌────▼─────┐
            ┌───────│  Running │───────┐
            │       └────┬─────┘       │
      任务失败（可重试）    │       全部成功
            │    minSuccess 满足       │
       ┌────▼─────┐   / 全部完成  ┌────▼──────┐
       │  Failed  │              │ Completed │
       └──────────┘              └───────────┘
            │
   终止 / 超时（任意阶段）
            │
    ┌───────▼────────┐
    │  Terminating   │
    └───────┬────────┘
            │ 所有 Pod 清理完毕
    ┌───────▼────────┐
    │ （Pod 已删除）  │
    └────────────────┘
```

### Phase 3 — 最小调度器 ⬜ 计划中

| 模块 | 描述 | 预估代码量 |
|------|------|-----------|
| `pkg/scheduler/session.go` | 调度会话框架 | ~200 行 |
| `pkg/scheduler/framework.go` | Action / Plugin 接口 | ~100 行 |
| `pkg/scheduler/actions/enqueue.go` | Enqueue — 优先级排队 | ~60 行 |
| `pkg/scheduler/actions/allocate.go` | Allocate — FIFO + binpack | ~150 行 |
| `pkg/scheduler/plugins/gang.go` | Gang — minMember 门控 | ~80 行 |
| `pkg/scheduler/plugins/predicates.go` | 节点过滤（资源 / 亲和性） | ~100 行 |
| `pkg/scheduler/plugins/nodeorder.go` | 节点打分 | ~60 行 |
| `pkg/scheduler/cache.go` | 调度器缓存（Informer 驱动） | ~300 行 |
| `cmd/scheduler/main.go` | 入口 | ~80 行 |

**调度流水线（单周期）：**

```
enqueue → allocate → backfill
  │          │          │
  │    (gang 检查,  (碎片补充)
  │     binpack)
  │
  按优先级 + 创建时间排序
```

### Phase 4 — CLI + 示例 ⬜ 计划中

| 组件 | 描述 | 预估代码量 |
|------|------|-----------|
| `cmd/cli/main.go` | CLI 入口（cobra） | ~80 行 |
| `cmd/cli/submit.go` | `mvj submit -f job.yaml` | ~100 行 |
| `cmd/cli/list.go` | `mvj get jobs` | ~80 行 |
| `cmd/cli/delete.go` | `mvj delete job <name>` | ~60 行 |
| `cmd/cli/describe.go` | `mvj describe job <name>` | ~100 行 |
| `config/examples/` | 更多示例（MPI、TensorFlow 风格） | ~100 行 |

---

## 架构

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
│  控制器管理器 (Phase 2)                                 │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │ Job 控制器  │  │ PodGroup 控制│  │ Queue 控制   │ │
│  │ (状态机)    │  │ (状态同步)   │  │ (Phase 3)    │ │
│  └──────┬──────┘  └──────┬───────┘  └──────────────┘ │
│         │                │                             │
│         └────────┬───────┘                             │
│                  │ 创建 / 更新 Pod                      │
└──────────────────┼─────────────────────────────────────┘
                   │
┌──────────────────▼─────────────────────────────────────┐
│  调度器 (Phase 3)                                       │
│  ┌─────────────────────────────────────────────────┐  │
│  │ Session                                         │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐        │  │
│  │  │ enqueue  │→│ allocate │→│ backfill │        │  │
│  │  └──────────┘ └──────────┘ └──────────┘        │  │
│  │  ┌──────────────────────────────────────────┐  │  │
│  │  │ 插件: gang | predicates | nodeorder      │  │  │
│  │  └──────────────────────────────────────────┘  │  │
│  └─────────────────────────────────────────────────┘  │
│  缓存（Informer 驱动快照）                               │
└────────────────────────────────────────────────────────┘
```

---

## 目录结构

```
mini-volcano-job/
├── README.zh-CN.md                      # ← 本文件
├── README.zh-TW.md                      # 繁體中文版
├── README.md                            # English version
├── go.mod                               # Go 1.26 + k8s v0.36
├── api/
│   └── v1alpha1/
│       ├── doc.go                       # 包文档 + 代码生成标记
│       ├── types.go                     # Job / PodGroup 类型定义
│       ├── register.go                  # Scheme 注册
│       └── zz_generated.deepcopy.go     # DeepCopy 方法
├── config/
│   ├── crd/
│   │   ├── jobs.mini-volcano.sh.yaml
│   │   └── podgroups.mini-volcano.sh.yaml
│   └── examples/
│       ├── job-nginx.yaml               # 单任务示例
│       └── job-multi-task.yaml          # 多任务 + DAG 示例
├── cmd/                                 # Phase 2+（入口）
├── pkg/                                 # Phase 2+（核心逻辑）
└── hack/                                # Phase 2+（代码生成）
```

---

## 快速开始

### 安装 CRD

```bash
kubectl apply -f config/crd/jobs.mini-volcano.sh.yaml
kubectl apply -f config/crd/podgroups.mini-volcano.sh.yaml
```

### 创建 Job

```bash
kubectl apply -f config/examples/job-nginx.yaml
kubectl get vj          # 列出 mini-volcano Job
kubectl get pg          # 列出 PodGroup
```

### 查看 Job 详情

```bash
kubectl get vj nginx-example -o yaml
kubectl describe vj nginx-example  # Phase 4
```

---

## 与 Volcano 的差异

| 维度 | Volcano | mini-volcano-job |
|------|---------|------------------|
| Go 代码量 | 252,195 行 | ~3,000 行（目标） |
| 文件数 | 1,135 | ~25 |
| Job 状态 | 10 种（含 Aborting、Aborted、Restarting） | **5 种**（Pending、Running、Completed、Failed、Terminating） |
| 调度 action | 8 个 | **3 个**（enqueue、allocate、backfill） |
| 调度插件 | 24+ | **3 个**（gang、predicates、nodeorder） |
| 调度策略 | gang、binpack、DRF、proportion、priority、NUMA、deviceshare、SLA、TDM… | **gang + binpack** |
| CRD 数量 | 11 | **2**（Job + PodGroup） |
| API 组 | batch / scheduling / bus | **1**（`mini-volcano.sh`） |
| JobFlow | ✅ DAG 编排 | ❌ |
| CronJob | ✅ | ❌ |
| Queue 控制器 | ✅ | Phase 3 |
| Webhook | ✅ 准入校验 | ❌ |
| Agent | ✅ 节点守护 | ❌ |
| CLI 命令 | 7 个 | **4 个**（Phase 4） |
| Go 依赖 | ~100 个模块 | **2 个**（k8s.io/api + apimachinery） |

### 保留的特性

- **Job → PodGroup → Pods** Gang 调度核心模型
- **状态机驱动**的 Job 生命周期管理
- **Session-Plugin** 可扩展调度框架
- **FIFO + binpack** 基础调度策略

### 舍弃的特性

- 抢占、回收、gang 抢占/回收——生产环境必备，学习场景可选
- 多队列公平共享（proportion、DRF）
- NUMA、设备共享、网络拓扑感知
- JobFlow DAG 编排
- 准入 Webhook
- 节点 Agent

---

## 设计决策

| 决策 | 理由 |
|------|------|
| 单一 API 组（`mini-volcano.sh`） | 规模小——三个组属于过度设计 |
| 手写 DeepCopy | Phase 1 无代码生成工具链；Phase 2 替换为 `controller-gen` |
| Pod 模板中使用 `x-kubernetes-preserve-unknown-fields` | 嵌入完整 `PodTemplateSpec`，无需逐字段 CRD schema |
| k8s.io v0.36.0 + Go 1.26 | 当前最新稳定组合（2026 年中） |
| 零 `controller-runtime` 依赖 | Phase 1–2 仅使用 apimachinery 原生 API；Phase 3 可选引入 |
| `localSchemeBuilder` 模式 | 匹配标准 K8s 注册约定；支持未来 `init()` 类型注册 |

---

## 许可证

Apache 2.0 — 与上游 Volcano 保持一致。
