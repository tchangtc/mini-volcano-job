# mini-volcano-job

> 從 [Volcano](https://github.com/volcano-sh/volcano)（約 25 萬行 Go 程式碼）中提取的最小批次排程引擎。
> 保留核心思想，移除 97% 的程式碼——專為學習與研究打造。

[![Go](https://img.shields.io/badge/Go-1.26.0-00ADD8?logo=go)](https://go.dev/)
[![k8s](https://img.shields.io/badge/k8s.io-v0.36.0-326CE5?logo=kubernetes)](https://kubernetes.io/)
[![Phase](https://img.shields.io/badge/phase-1%2F4-blue)](#藍圖)

---

## 目錄

- [專案簡介](#專案簡介)
- [藍圖](#藍圖)
- [架構](#架構)
- [目錄結構](#目錄結構)
- [快速開始](#快速開始)
- [與 Volcano 的差異](#與-volcano-的差異)
- [設計決策](#設計決策)

---

## 專案簡介

**mini-volcano-job** 是一個面向教學的 Kubernetes 批次排程引擎實作。

上游 Volcano 專案橫跨 **1,919 個檔案、252,195 行 Go 程式碼**，包含完整的排程器、24+ 個排程外掛、多個控制器、Webhook、Agent 等。mini-volcano-job 僅提取核心的作業管理與 Gang 排程邏輯，用 **約 3,000 行 Go 程式碼** 實作一個可運行的最小版本。

### 核心概念

```
Job ─── 擁有 ─── PodGroup ─── 原子排程 ─── Pods
 │                  │
 │  tasks[]         │  minMember
 │  minAvailable    │  queue
 │  queue           │
 └──────────────────┘
```

- **Job** — 由一個或多個 Task 組成的批次工作負載（每個 Task = Pod 範本 × 副本數）。
- **PodGroup** — Gang 排程原語：一組 Pod 要麼全部排程成功，要麼全部不排程。
- **Queue** — 資源分配的命名優先級桶（Phase 3 實作）。

---

## 藍圖

### Phase 1 — API 型別 + CRD ✅ 已完成

| 產物 | 描述 |
|------|------|
| `api/v1alpha1/types.go` | Job + PodGroup + 子型別，5 階段 Job 狀態機 |
| `api/v1alpha1/register.go` | Scheme 註冊 |
| `api/v1alpha1/zz_generated.deepcopy.go` | DeepCopy 方法（手寫；Phase 2 替換為程式碼生成） |
| `config/crd/jobs.mini-volcano.sh.yaml` | Job CRD |
| `config/crd/podgroups.mini-volcano.sh.yaml` | PodGroup CRD |
| `config/examples/` | 單任務 + 多任務 DAG 範例 |

### Phase 2 — Job 控制器 ⬜ 規劃中

| 模組 | 描述 | 預估程式碼量 |
|------|------|-----------|
| `pkg/controller/job_controller.go` | 主協調迴圈 | ~400 行 |
| `pkg/controller/job_state.go` | 5 狀態機（Pending→Running→Completed/Failed→Terminating） | ~200 行 |
| `pkg/controller/pod_control.go` | Pod 建立 / 刪除 / 追蹤 | ~150 行 |
| `pkg/controller/podgroup_control.go` | PodGroup 自動建立與狀態同步 | ~100 行 |
| `cmd/controller-manager/main.go` | 進入點 | ~80 行 |

**狀態機轉換圖：**

```
                    ┌──────────┐
                    │  Pending │ ←── 初始狀態
                    └────┬─────┘
               minAvailable 滿足
                         │
                    ┌────▼─────┐
            ┌───────│  Running │───────┐
            │       └────┬─────┘       │
      任務失敗（可重試）    │       全部成功
            │    minSuccess 滿足       │
       ┌────▼─────┐   / 全部完成  ┌────▼──────┐
       │  Failed  │              │ Completed │
       └──────────┘              └───────────┘
            │
   終止 / 逾時（任意階段）
            │
    ┌───────▼────────┐
    │  Terminating   │
    └───────┬────────┘
            │ 所有 Pod 清理完畢
    ┌───────▼────────┐
    │ （Pod 已刪除）  │
    └────────────────┘
```

### Phase 3 — 最小排程器 ⬜ 規劃中

| 模組 | 描述 | 預估程式碼量 |
|------|------|-----------|
| `pkg/scheduler/session.go` | 排程工作階段框架 | ~200 行 |
| `pkg/scheduler/framework.go` | Action / Plugin 介面 | ~100 行 |
| `pkg/scheduler/actions/enqueue.go` | Enqueue — 優先級排隊 | ~60 行 |
| `pkg/scheduler/actions/allocate.go` | Allocate — FIFO + binpack | ~150 行 |
| `pkg/scheduler/plugins/gang.go` | Gang — minMember 門控 | ~80 行 |
| `pkg/scheduler/plugins/predicates.go` | 節點過濾（資源 / 親和性） | ~100 行 |
| `pkg/scheduler/plugins/nodeorder.go` | 節點打分 | ~60 行 |
| `pkg/scheduler/cache.go` | 排程器快取（Informer 驅動） | ~300 行 |
| `cmd/scheduler/main.go` | 進入點 | ~80 行 |

**排程管線（單週期）：**

```
enqueue → allocate → backfill
  │          │          │
  │    (gang 檢查,  (碎片補充)
  │     binpack)
  │
  按優先級 + 建立時間排序
```

### Phase 4 — CLI + 範例 ⬜ 規劃中

| 元件 | 描述 | 預估程式碼量 |
|------|------|-----------|
| `cmd/cli/main.go` | CLI 進入點（cobra） | ~80 行 |
| `cmd/cli/submit.go` | `mvj submit -f job.yaml` | ~100 行 |
| `cmd/cli/list.go` | `mvj get jobs` | ~80 行 |
| `cmd/cli/delete.go` | `mvj delete job <name>` | ~60 行 |
| `cmd/cli/describe.go` | `mvj describe job <name>` | ~100 行 |
| `config/examples/` | 更多範例（MPI、TensorFlow 風格） | ~100 行 |

---

## 架構

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
│  │ (狀態機)    │  │ (狀態同步)   │  │ (Phase 3)    │ │
│  └──────┬──────┘  └──────┬───────┘  └──────────────┘ │
│         │                │                             │
│         └────────┬───────┘                             │
│                  │ 建立 / 更新 Pod                      │
└──────────────────┼─────────────────────────────────────┘
                   │
┌──────────────────▼─────────────────────────────────────┐
│  排程器 (Phase 3)                                       │
│  ┌─────────────────────────────────────────────────┐  │
│  │ Session                                         │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐        │  │
│  │  │ enqueue  │→│ allocate │→│ backfill │        │  │
│  │  └──────────┘ └──────────┘ └──────────┘        │  │
│  │  ┌──────────────────────────────────────────┐  │  │
│  │  │ 外掛: gang | predicates | nodeorder      │  │  │
│  │  └──────────────────────────────────────────┘  │  │
│  └─────────────────────────────────────────────────┘  │
│  快取（Informer 驅動快照）                               │
└────────────────────────────────────────────────────────┘
```

---

## 目錄結構

```
mini-volcano-job/
├── README.zh-CN.md                      # 簡體中文版
├── README.zh-TW.md                      # ← 本檔案
├── README.md                            # English version
├── go.mod                               # Go 1.26 + k8s v0.36
├── api/
│   └── v1alpha1/
│       ├── doc.go                       # 套件文件 + 程式碼生成標記
│       ├── types.go                     # Job / PodGroup 型別定義
│       ├── register.go                  # Scheme 註冊
│       └── zz_generated.deepcopy.go     # DeepCopy 方法
├── config/
│   ├── crd/
│   │   ├── jobs.mini-volcano.sh.yaml
│   │   └── podgroups.mini-volcano.sh.yaml
│   └── examples/
│       ├── job-nginx.yaml               # 單任務範例
│       └── job-multi-task.yaml          # 多任務 + DAG 範例
├── cmd/                                 # Phase 2+（進入點）
├── pkg/                                 # Phase 2+（核心邏輯）
└── hack/                                # Phase 2+（程式碼生成）
```

---

## 快速開始

### 安裝 CRD

```bash
kubectl apply -f config/crd/jobs.mini-volcano.sh.yaml
kubectl apply -f config/crd/podgroups.mini-volcano.sh.yaml
```

### 建立 Job

```bash
kubectl apply -f config/examples/job-nginx.yaml
kubectl get vj          # 列出 mini-volcano Job
kubectl get pg          # 列出 PodGroup
```

### 檢視 Job 詳情

```bash
kubectl get vj nginx-example -o yaml
kubectl describe vj nginx-example  # Phase 4
```

---

## 與 Volcano 的差異

| 維度 | Volcano | mini-volcano-job |
|------|---------|------------------|
| Go 程式碼量 | 252,195 行 | ~3,000 行（目標） |
| 檔案數 | 1,135 | ~25 |
| Job 狀態 | 10 種（含 Aborting、Aborted、Restarting） | **5 種**（Pending、Running、Completed、Failed、Terminating） |
| 排程 action | 8 個 | **3 個**（enqueue、allocate、backfill） |
| 排程外掛 | 24+ | **3 個**（gang、predicates、nodeorder） |
| 排程策略 | gang、binpack、DRF、proportion、priority、NUMA、deviceshare、SLA、TDM… | **gang + binpack** |
| CRD 數量 | 11 | **2**（Job + PodGroup） |
| API 群組 | batch / scheduling / bus | **1**（`mini-volcano.sh`） |
| JobFlow | ✅ DAG 編排 | ❌ |
| CronJob | ✅ | ❌ |
| Queue 控制器 | ✅ | Phase 3 |
| Webhook | ✅ 准入校驗 | ❌ |
| Agent | ✅ 節點守護 | ❌ |
| CLI 指令 | 7 個 | **4 個**（Phase 4） |
| Go 依賴 | ~100 個模組 | **2 個**（k8s.io/api + apimachinery） |

### 保留的特性

- **Job → PodGroup → Pods** Gang 排程核心模型
- **狀態機驅動**的 Job 生命週期管理
- **Session-Plugin** 可擴充排程框架
- **FIFO + binpack** 基礎排程策略

### 捨棄的特性

- 搶佔、回收、gang 搶佔/回收——生產環境必備，學習場景可選
- 多佇列公平共享（proportion、DRF）
- NUMA、裝置共享、網路拓撲感知
- JobFlow DAG 編排
- 准入 Webhook
- 節點 Agent

---

## 設計決策

| 決策 | 理由 |
|------|------|
| 單一 API 群組（`mini-volcano.sh`） | 規模小——三個群組屬於過度設計 |
| 手寫 DeepCopy | Phase 1 無程式碼生成工具鏈；Phase 2 替換為 `controller-gen` |
| Pod 範本中使用 `x-kubernetes-preserve-unknown-fields` | 嵌入完整 `PodTemplateSpec`，無需逐欄位 CRD schema |
| k8s.io v0.36.0 + Go 1.26 | 目前最新穩定組合（2026 年中） |
| 零 `controller-runtime` 依賴 | Phase 1–2 僅使用 apimachinery 原生 API；Phase 3 可選引入 |
| `localSchemeBuilder` 模式 | 符合標準 K8s 註冊約定；支援未來 `init()` 型別註冊 |

---

## 授權條款

Apache 2.0 — 與上游 Volcano 保持一致。
