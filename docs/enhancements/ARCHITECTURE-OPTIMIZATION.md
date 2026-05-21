# ALFQ 架构优化建议

> **日期**：2026-05-21  
> **背景**：Phase A–E 全部通过验收，功能已跑通，但当前实现存在多处可优化空间。  
> **目的**：提出优化方案，供人工审核决策。

---

## 1. 当前架构概览

```
┌──────────────────────────────────────────────────────────────┐
│ 14 容器 Docker Compose                                       │
│                                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐ │
│  │ md-      │  │ trading- │  │ quant-   │  │ assistant-  │ │
│  │ gateway  │  │ core     │  │ engine   │  │ svc         │ │
│  │ (Go)     │  │ (Go)     │  │ (Go)     │  │ (Go)        │ │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────┬──────┘ │
│       │              │             │                │        │
│  ─────┼──────────────┼─────────────┼────────────────┼──────  │
│       │         ┌────┴─────┐       │                │        │
│       │         │ NATS     │◄──────┘                │        │
│       │         └──────────┘                        │        │
│  ┌────┴─────┐  ┌──────────┐  ┌──────────┐          │        │
│  │ Postgres │  │ ClickH.  │  │ Redis    │          │        │
│  └──────────┘  └──────────┘  └──────────┘          │        │
│                                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐ │
│  │ research │  │ frontend │  │ Grafana  │  │ Prometheus  │ │
│  │ (Python) │  │ (Nginx)  │  │ +Loki    │  │ +Tempo      │ │
│  │ 宿主机    │  └──────────┘  │ +Vault   │  └─────────────┘ │
│  └──────────┘                └──────────┘                   │
└──────────────────────────────────────────────────────────────┘
```

---

## 2. 已识别问题

### 2.1 构建效率

| 问题 | 当前 | 建议 |
|------|------|------|
| Dockerfile.builder 每次从零构建 | proto生成 → go mod download → 编译，平均 120s | 分层缓存 + 预编译 base image |
| 4 个 Go 服务共享 Dockerfile 但独立构建 | 改一行代码 → 重编 4 次 | 合并为单体或使用多阶段缓存 |
| Build cache 膨胀 | 80 GB | 定期 `docker builder prune` |

### 2.2 Go→Python 桥接脆弱

`trading-core` 通过 `exec.Command("uv", "run", "python", ...)` 调 CLI：

- 容器内依赖 Python + uv + research 所有 pip 包
- alpine(musl) 下 polars/lz4 等原生扩展不兼容
- 进程超时/崩溃无重试
- 输出通过 stdout 解析 JSON，无结构化错误

### 2.3 MT 连接管理

| 组件 | 连接方式 | 问题 |
|------|---------|------|
| md-gateway | 长连接 (persistent) | ✅ 最优 |
| OMS adapter | 每次下单拨号→认证→下单→断开 | 每次新建 gRPC 连接 + TLS 握手 + MT 认证，浪费 ~3s |
| backfill CLI | 每次拨号→认证→拉数据→断开 | 同 OMS，且无法复用 md-gateway 已有连接 |
| symbol-sync CLI | 同 backfill | 独立建连 |

### 2.4 服务拆分过细

| 服务 | 职责 | 独立必要性 |
|------|------|-----------|
| md-gateway | tick 采集 + bar 聚合 + CH 写入 | ✅ 核心 |
| trading-core | API + OMS + auth | ✅ 面向用户 |
| quant-engine | 策略推理 | ❌ 功能极其简单（DSL 编译器 + runner），独立部署增加 NATS 延迟 |
| assistant-svc | AI 助手（空壳） | ❌ 无实际功能，可 merge 到 trading-core |

### 2.5 ClickHouse 类型转换

`Decimal(18,6)` 列（bid/ask/open/high/low/close）需 `float64 → string` 手动转换。ClickHouse Go 驱动已支持 Decimal，但当前 spill_replay 和 bar aggregator 仍用 workaround。

### 2.6 无服务健康检查 + 重启策略

| 服务 | depends_on | 健康检查 | restart |
|------|-----------|---------|---------|
| md-gateway | nats, redis, CH | ❌ | ❌ |
| trading-core | PG, redis, nats | ❌ | ❌ |
| quant-engine | nats | ❌ | ❌ |

服务崩溃不自动重启（今天已遇到 md-gateway panic 直接退出）。

---

## 3. 优化方案

### 方案 A：最小改动（1–2 天）

改动最少，解决最痛的问题：

1. **OMS 连接池化**：adapter 维护长连接，首次拨号后缓存 session，后续复用
2. **Dockerfile 优化**：预编译 Go 依赖层，增量构建从 120s → 15s
3. **重启策略**：所有 app 容器加 `restart: unless-stopped`
4. **健康检查**：trading-core/md-gateway 加 `healthcheck`

```
改动量：~200 行 Go + 20 行 compose
风险：低
```

### 方案 B：服务合并（3–5 天）

1. **合并 quant-engine → trading-core**：策略推理是 API 的附属功能，独立部署增加 NATS 跳转延迟
2. **合并 assistant-svc → trading-core**（或删除）
3. **mtapi 连接池独立为 sidecar**：md-gateway 暴露 gRPC 给 OMS/backfill 复用连接
4. **Python 独立为 research-svc 容器**（glibc based）

```
新架构：
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│ md-gateway   │   │ trading-core │   │ research-svc │
│ + mtapi pool │   │ + quant      │   │ (Python)     │
│ + symbol sync│   │ + assistant  │   │ glibc image  │
│ + OMS bridge │   │ + admin API  │   │              │
└──────────────┘   └──────────────┘   └──────────────┘
      │                    │                   │
      └────────────────────┼───────────────────┘
                    NATS / gRPC
```

容器数从 14 → **10**（减少 4 个）。

### 方案 C：三平面架构（1–2 周）

按照数据流自然边界重组为三个平面：

```
┌─ data-plane ─────────────────────────────┐
│ md-gateway + OMS + bar aggregator        │
│ + symbol sync + CH writer + spill replay │
│ 所有直接接触 MT 连接的逻辑                    │
│ 共享一个 mtapi connection pool             │
└──────────────────────────────────────────┘
         │ gRPC / NATS
┌─ control-plane ──────────────────────────┐
│ trading-core + admin API + auth           │
│ + strategy lifecycle + backtest svc       │
│ + audit logging                           │
└──────────────────────────────────────────┘
         │ gRPC
┌─ research-plane ─────────────────────────┐
│ research-svc (Python, glibc)              │
│ + backtest CLI + ONNX inference           │
│ + trainer API + DataClient                │
└──────────────────────────────────────────┘
```

容器数从 14 → **6 + infra**（PG/CH/Redis/NATS/Grafana/Prometheus）。

### 方案对比

| 维度 | A（最小改动） | B（合并） | C（三平面） |
|------|-------------|----------|------------|
| 改动量 | ~200 行 | ~500 行 + 重构 | ~1000 行 + 新 Dockerfile |
| 容器数 | 14（不变） | 10 | 6 + infra |
| 构建时间 | 15s/服务 | 10s（更少目标） | 8s |
| MT 连接复用 | OMS 池化 | sidecar 池 | 平面内共享 |
| Python 环境 | 无变化 | glibc 容器 | 独立平面 |
| 回滚风险 | 极低 | 低 | 中 |
| 可独立扩展 | ✗ | ✗ | ✓ |

---

## 4. 独立于方案选择的关键改进

以下改进与方案选择无关，应该立即做：

| # | 改进 | 影响 |
|---|------|------|
| 1 | 所有 app 容器加 `restart: unless-stopped` | 崩溃自动恢复 |
| 2 | md-gateway/trading-core 加 healthcheck | 编排可靠 |
| 3 | `docker builder prune --keep-storage 20GB` | 释放 60GB 磁盘 |
| 4 | OMS adapter 复用长连接 | 下单延迟从 3s → 100ms |
| 5 | Go→Python 桥接改为 RPC（非 exec） | 跨平台兼容 |
| 6 | ClickHouse Decimal 直接使用驱动原生支持 | 消除 float64↔string 转换 |

---

## 5. 建议

> **短期（本周）**：采用方案 A + §4 全部 6 项改进。改动小、风险低，解决当前最痛的问题（崩溃恢复、下单延迟、构建速度）。  
> **中期（下轮迭代）**：评估方案 B 或 C。根据业务增长速度决定是否需要平面拆分。  
> **长期**：如果策略数量 > 100 或日 tick > 1 亿，方案 C 的三平面架构可以独立扩缩。

---

*由 DeepSeek 分析生成，待人工审核*

---

## 6. Cascade 评审（2026-05-21）

> 评审范围：本文 §2 / §3 三套方案，对照 ADR-0010、ADR-0011、`docs/01-总体架构与技术决策.md`，
> 以及 `docs/enhancements/2026-05-20-order-sync-incremental-design.md` 的实施现状。
> 结论：**§4 全部 6 项立即做；§3 方案 A 大部分采纳；方案 B/C 拒绝**；另引入下文方案 D。

### 6.1 与既有 ADR 的冲突

| 本文建议 | 冲突点 | 依据 |
|---|---|---|
| §3 方案 B：合并 `assistant-svc → trading-core` | LLM 调用的延迟/失败必须独立熔断，污染交易主链路是 ADR-0010 否决方案 C 的核心理由 | `docs/adr/0010-consolidate-services.md` §备选方案 C |
| §3 方案 B：合并 `quant-engine → trading-core` | ADR-0010 已明确"因子→策略数据流是单向纯计算管道，进程内通信比 NATS 快两个数量级"——**它本身就是合并产物**，再合并到 trading-core 会让 GC/CPU 抖动影响 OrderSend p99 | ADR-0010 §合并保留独立的理由 |
| §3 方案 C：拆分 `trading-core` 为 data-plane（含 OMS）+ control-plane（API+风控） | ADR-0010 明确"API 层、订单管理、风控引擎在下单路径上**强同步耦合**，必须一个进程内调用"。下单路径再加跨进程 RTT (~1.2ms) 等于回退到 M0–M2 的 7 服务架构 | ADR-0010 §合并保留独立的理由 |
| §3 方案 C：把 `md-gateway` 改名"data-plane"塞进 OMS/symbol-sync/backfill | 这正是 ADR-0010 已经规划的方向，**只是没执行完**——见 §6.2 真正的问题 | — |

> **结论**：§3 方案 B 与 C **不予采纳**。方案 A 的 4 条改进采纳。

### 6.2 本文未识别的核心问题

ADR-0010 写明："`md-gateway` 是**唯一与外部 broker 建立长连接的服务**"。
实际代码却违反了这条 ADR：

| 服务 | MT 长连接位置 | 用途 |
|---|---|---|
| `md-gateway` | `internal/mdgateway/gateway_mt[45].go` 的 streamLoop | Tick 订阅 + Bar 聚合 |
| **`trading-core`** | `internal/accountconn/connector.go` 的 streamLoop | 账户绑定 + OnOrderUpdate + 持仓刷新 + OMS 下单 + sync_worker.FullSync 新建临时连接 |

也就是说，**同一个 MT 账号在同一台机器上被 trading-core 与 md-gateway 各登录了一次**，
导致：

1. MT 服务器侧 session 计数浪费，部分经纪商有 "同账号并发会话数 ≤ N" 限制
2. backfill / symbol-sync / OMS / sync_worker.FullSync 每次都要重新拨号+认证（~3s/次），与本文 §2.3 描述的痛点 100% 吻合
3. 行情断流后 trading-core 端的 OnOrderUpdate 也跟着断，两边重连逻辑各跑各的
4. 与 `docs/enhancements/2026-05-20-order-sync-incremental-design.md` §4.2.1 "三种触发同步"中"重连成功对账"无法可靠实现——重连事件目前只在 trading-core 内感知

这才是本文 §2.3 "MT 连接管理" 真正应当解决的根因，而不是去拆分 trading-core。

### 6.3 修订方案 D：MT Session Hub（推荐）

> 改动量在 A 与 B 之间；**完全遵守 ADR-0010**；同时一次性解决 §6.2 与
> `2026-05-20-order-sync-incremental-design.md` §4.2.3 的连接复用需求。

#### 6.3.1 思路

把目前散落在 `trading-core/accountconn` 与 `md-gateway/runner` 里的两套 MT 会话管理，
统一收敛到 **md-gateway 的 SessionHub**，对内通过 gRPC 暴露：

```
                ┌─────────────────────────────────────┐
                │  md-gateway (唯一 MT 长连接持有者)   │
                │  ┌─────────────────────────────┐    │
                │  │     SessionHub (新增)        │    │
                │  │  - per-account 长连接 + sid │    │
                │  │  - 自动重连 + 健康检查       │    │
                │  │  - OnQuote / OnOrderUpdate  │    │
                │  │  - WithSession(fn) 接口     │    │
                │  └────────┬────────────────────┘    │
                │           │ 内部 gRPC                │
                │  ┌────────┴────────────────────┐    │
                │  │  internal/mthub.Service      │    │
                │  │    OrderSend / OrderHistory │    │
                │  │    SymbolParams / PriceHist │    │
                │  │    SubscribeOrderEvents     │    │
                │  └─────────────────────────────┘    │
                └──────┬──────────────────────────────┘
                       │ gRPC: alfq.mthub.v1.MtHubService
              ┌────────┴────────┐
              ▼                 ▼
    ┌──────────────┐   ┌──────────────┐
    │ trading-core │   │ research-svc │
    │  OMS / sync  │   │  backfill    │
    │  worker      │   │  CLI         │
    └──────────────┘   └──────────────┘
```

`trading-core/accountconn` **不再持有 MT gRPC 连接**，改为通过 mthub RPC 借用 md-gateway 现有 session。

#### 6.3.2 关键变更

| 模块 | 变更 |
|---|---|
| `md-gateway` | 新增 `internal/mthub/` 包：SessionHub + Service + per-platform fetcher（复用现有 `adapter/mtapi`） |
| proto | 新增 `alfq.mthub.v1` 内部 RPC（不暴露到前端 Nginx）：`OrderSend` / `OrderHistory` / `OrderHistoryStream`（增量推送）/ `SymbolParamsMany` / `PriceHistory` |
| `trading-core/accountconn` | 删除直接 `mtapi.Dial`；改为持有 `mthub.Client`；OnOrderUpdate 改为消费 mthub 的 `OrderHistoryStream` |
| `trading-core/accountconn/sync_worker.go` | `FullSync` 的 `DialAndFetchOrderHistory` 整段删除，全部改走 `mthub.Client.OrderHistory(account_id, from, to)` |
| `mtapi/DialAndFetchOrderHistory` | 标记 deprecated，仅保留给 CLI 应急使用 |
| `symbol-sync` / `md-backfill` CLI | 改为通过 mthub gRPC（保留 `--direct` 兜底模式） |

#### 6.3.3 与 ADR-0010 的一致性

- `md-gateway` 仍然是**唯一持有 MT 长连接**的服务 ✓（ADR-0010 §合并保留独立的理由）
- `trading-core` 内进程内 API→OMS→风控 强同步耦合**不变** ✓
- 服务数 4 + 1 前端 **不变** ✓

#### 6.3.4 改动估算

| 项 | 行数 |
|---|---|
| proto + 生成代码 | ~150 行 proto，自动生成 |
| `internal/mthub/` 新包 | ~600 行 |
| `trading-core/accountconn` 改造 | -400 / +200 行（净 -200，复杂度大幅下降） |
| migration 脚本 | 0 |
| **合计** | ~800 行（含删除），中等改动 |

风险：中（涉及 OMS 热路径）；可分阶段灰度：先 sync_worker.FullSync → 再 symbol-sync → 再 OMS。

### 6.4 方案对比（修订版）

| 维度 | A 最小改动 | ~~B 合并~~ | ~~C 三平面~~ | **D MT Hub** |
|------|-----------|-----------|--------------|--------------|
| 改动量 | ~200 行 | ~500 + 重构 | ~1000 行 | ~800 行 |
| 容器数 | 14 | 10 | 6+infra | **14** |
| ADR-0010 合规 | ✓ | ✗ | ✗ | ✓ |
| 解决 §2.3 连接复用 | 部分（仅 OMS） | 部分 | ✓ | ✓ |
| 解决 §6.2 双 MT 连接根因 | ✗ | ✗ | ✓ | ✓ |
| 与 order-sync 设计对齐 | 部分 | 部分 | 重写 | ✓ |
| 回滚风险 | 极低 | 低 | 中 | 低（可灰度） |

### 6.5 修订建议

- **本周做**：§4 全部 6 项 + §3 方案 A 中第 1 / 3 / 4 条（OMS 池化、restart、healthcheck）
- **下一轮（与 order-sync Phase 5 同步）**：方案 D MT Session Hub
- **拒绝**：方案 B、方案 C（违反 ADR-0010）

详细的 AI Agent 可执行任务清单见 `docs/enhancements/2026-05-21-final-optimization-plan.md`。

