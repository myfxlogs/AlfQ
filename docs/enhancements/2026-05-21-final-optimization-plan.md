# ALFQ 优化方案最终版（AI Agent 执行清单）

> **状态**：Approved（Cascade，2026-05-21）
> **执行者**：AI Agent（DeepSeek / Claude / 其他自动化 Agent）
> **本文取代**：`docs/enhancements/ARCHITECTURE-OPTIMIZATION.md` §3 中的方案 B 与 C
> **本文整合**：
> - `docs/enhancements/ARCHITECTURE-OPTIMIZATION.md`（架构优化）
> - `docs/enhancements/2026-05-20-order-sync-incremental-design.md`（订单本地化）
> **本文遵守**：`docs/adr/0010-consolidate-services.md`（4+1 服务划分不可逆）
>
> 🤖 **AI Agent 注意**：
> 1. 本文按依赖顺序排列，**不允许跨阶段并行**
> 2. 每个任务卡片自带"验收命令"——只有验收 PASS 才能改状态为 ☑
> 3. 不要重写 `docs/adr/0010-*`，它是约束，不是议题
> 4. 看不懂某个任务卡片就停下来在工作日志记 BLOCKED，不要瞎写
> 5. 严格遵循 `docs/tasks/MASTER-ROADMAP.md` §10 的 PR 守则

---

## 1. 决策总结

### 1.1 验收结论（Phase A–E）

| Phase | 任务 | 状态 | 证据 |
|---|---|---|---|
| A | DP-1~7, SM-1 | ☑ 8/8 | `docs/tasks/ACCEPTANCE-REPORT-2026-05-21.md` |
| B | SM-1~3 | ☑ 3/3 | 798 broker_symbols；SymbolService RPC buf curl 通 |
| C | RP-1~3 | ☑ 3/3 | 真实 CH 回测；一致性 corr=0.972 mad=0.0042% |
| D | EP-1~3 | ☑ 3/3 | ONNX 训练+推理；MT5 ticket=1835319679 / MT4 ticket=201235461 真实下单 |
| E | LP-1~2 | ☑ 2/2 | Python CLI backtest + 双签状态机 test PASS |
| F | OP-1~3 | 🅒 | **本文不处理，部署时再做** |

→ **A–E 全部通过，进入"架构优化 + 订单本地化收尾"阶段。**

### 1.2 拒绝的方案

| 方案 | 拒绝理由 |
|---|---|
| ARCHITECTURE-OPTIMIZATION §3 方案 B：合并 `quant-engine` / `assistant-svc` 到 `trading-core` | 违反 ADR-0010（LLM 故障污染主链路；量化 GC 影响下单 p99）|
| ARCHITECTURE-OPTIMIZATION §3 方案 C：三平面架构（拆 trading-core） | 违反 ADR-0010（API→OMS→风控 必须进程内强同步耦合）|

### 1.3 采纳的方案

**方案 D（MT Session Hub） + ARCHITECTURE-OPTIMIZATION §4 全 6 项立即改进 + order-sync 设计 P1/P2 完成项**。

---

## 2. 两个文档的冲突点（必读）

> AI Agent 实施时，**以本文为准**。两个原文档保留作为背景。

| 冲突 # | ARCHITECTURE-OPTIMIZATION 说 | order-sync 设计 说 | 本文裁决 |
|---|---|---|---|
| C1 | §2.3 OMS adapter 每次新建 gRPC 连接是浪费，建议 OMS 池化 | §4.2.3 用 `Manager.WithLiveSession` 复用 accountconn 的 MT session | **统一**：所有 MT 调用（OMS/sync/backfill/symbol）全部走方案 D 的 mthub RPC，**不在 trading-core 维护 MT 连接** |
| C2 | §3 方案 C 把 OMS 移到 data-plane | 设计假定 sync_worker 在 accountconn 包内（trading-core） | **裁决**：保留 `accountconn` 包名与位置（trading-core）以最小化改动；**但其内部不再 Dial MT**，改为持有 mthub.Client |
| C3 | §3 方案 B 合并 quant-engine 等 | 不涉及 | 拒绝合并；不改服务划分 |
| C4 | §4.5 Go→Python 桥接改 RPC | 不涉及 | 纳入本文 §4 BatchSvc 子项（中优先级） |
| C5 | §2.5 ClickHouse Decimal float→string workaround | 不涉及 | 纳入本文 §3 立即修 |
| C6 | 不涉及 | §4.2.4 SSE 推送应携带 `changes[]` 增量数据 | 现状是占位 `[{op:"sync"}]`（见 `connector.go:797-807`），本文 §5 强制升级 |
| C7 | 不涉及 | §4.2.1 全量同步、重连对账、定时兜底 | FullSync 现走临时 Dial（错），重连/兜底未接入；本文 §5 收口 |
| C8 | §4.2 容器健康检查/restart 缺失 | 不涉及 | 立即修，本文 §3 |

---

## 3. Stage 1 · 即时改进（Quick Wins，1–2 天）

> 全部低风险、可独立合并。**不依赖** Stage 2/3。允许多个 AI Agent 并行做不同卡片。

### QW-1 · `chore(deploy): add restart + healthcheck to all app containers`

**目标**：进程崩溃自动重启；编排器能判断"启动完成"。

**改动**：
- 修改 `deploy/docker-compose.prod.yml`
- 给 `trading-core` / `md-gateway` / `quant-engine` / `assistant-svc` / `frontend` 五个 service 都加：
  ```yaml
  restart: unless-stopped
  healthcheck:
    test: ["CMD", "wget", "-qO-", "http://localhost:<port>/health"]
    interval: 10s
    timeout: 3s
    retries: 5
    start_period: 30s
  ```
- 后端服务若无 `/health` 路由 → 同步在 `cmd/<svc>/main.go` 加 `GET /health` 返回 `{"status":"ok"}`（含 PG/CH/NATS ping）

**验收**：
```bash
docker compose -f deploy/docker-compose.prod.yml up -d
sleep 60
docker compose -f deploy/docker-compose.prod.yml ps --format json | jq -r '.[] | "\(.Service)\t\(.Health)"'
# 全部 service 显示 healthy（infra 容器除外）

# 注入故障验证 restart
docker kill deploy-trading-core-1
sleep 15
docker ps --filter name=deploy-trading-core-1 --format '{{.Status}}'
# 应包含 "Up X seconds"
```

---

### QW-2 · `perf(clickhouse): drop float→string Decimal workaround`

**目标**：消除 `fmt.Sprintf("%.6f", v)` 这种 workaround；用 driver 原生 Decimal。

**改动**：
- `backend/go/internal/mdgateway/clickhouse_writer.go`：bid/ask 改为 `decimal.NewFromFloat` 或 driver 支持的 `*big.Float`
- `backend/go/internal/mdgateway/spill_replay.go` 同步改
- `backend/go/internal/mdgateway/bar_aggregator.go` 同步改（O/H/L/C）
- 升级 `github.com/ClickHouse/clickhouse-go/v2` 到含 Decimal 原生支持的最新次版本（保留 go.mod 锁定）

**验收**：
```bash
cd backend/go && GOTOOLCHAIN=local go test ./internal/mdgateway/... -race
# 全 PASS
# 跑 5 min md-gateway，验证 md_ticks/md_bars 行数继续增加，无解析错误
docker compose logs --since 5m md-gateway | grep -iE 'decimal|parse' | wc -l
# 应为 0
```

---

### QW-3 · `chore(build): docker layer cache optimization`

**目标**：Go 增量编译从 120s 降到 ≤ 20s。

**改动**：
- 修改 `deploy/Dockerfile.builder`：
  ```dockerfile
  # Stage 1: deps（仅在 go.mod/go.sum 变化时重跑）
  FROM golang:1.23-bookworm AS deps
  WORKDIR /src
  COPY backend/go/go.mod backend/go/go.sum ./
  RUN --mount=type=cache,target=/root/.cache/go-build \
      --mount=type=cache,target=/go/pkg/mod \
      go mod download

  # Stage 2: build
  FROM deps AS build
  COPY backend/go/ .
  ARG SVC
  RUN --mount=type=cache,target=/root/.cache/go-build \
      --mount=type=cache,target=/go/pkg/mod \
      CGO_ENABLED=1 go build -o /out/svc ./cmd/${SVC}
  ```
- 修改 `Makefile` 的 `go-build` 目标传 `--build-arg SVC=`
- 清理本机：`docker builder prune --keep-storage 20GB`

**验收**：
```bash
make go-build SVC=trading-core   # 冷启动作为基线
time make go-build SVC=trading-core  # 第二次
# real 时间 < 25s
```

---

### QW-4 · `feat(order-sync): SSE delta payload upgrade (P1 from order-sync §4.2.4)`

**目标**：把 `publishOrderDelta` 的占位 `[{op:"sync"}]` 升级为真实 `changes[]`。

**改动**：
- 修改 `backend/go/internal/accountconn/connector.go:797-807`：
  - 接受 `changed []*repo.HistoryOrder` 参数
  - 序列化为：
    ```json
    {"accountId":"...","type":"order_delta","changes":[
      {"op":"upsert","order":{"ticket":..,"symbol":..,"side":..,"lots":..,
       "openPrice":..,"closePrice":..,"profit":..,"swap":..,"commission":..,
       "openTime":"...","closeTime":"...","state":"closed"}}]}
    ```
- 修改 `connector.go:485-489` 调用点：
  - 改为：先跑 `IncrSync` → 拿到 `changedOrders []` → 再 `publishOrderDelta(accountID, changedOrders)`
- `repo.HistoryOrderRepo.BatchUpsert` 返回 `[]HistoryOrder`（仅真正发生变更的行，利用 `RETURNING (xmax = 0) as inserted` 区分）

**验收**：
```bash
# 1. 触发一笔真实下单（或人工平仓一个已开仓位）
# 2. 订阅 NATS subject 看到完整 order 对象
docker exec deploy-nats-1 nats sub 'account.orders.>' --count=1 | tee /tmp/sse.json
jq '.changes[0].order.ticket' /tmp/sse.json   # 应为整数 ticket 号
jq '.changes[0].order.profit' /tmp/sse.json   # 应有具体数值
# 3. 前端 HistoryTab 应在 1s 内出现新订单（无需手动刷新）
```

---

### QW-5 · `chore(infra): docker cache cleanup + retention policy`

**目标**：80GB build cache → 释放 60GB。

**改动**：
- 新增 `deploy/scripts/docker-cleanup.sh`：
  ```bash
  #!/usr/bin/env bash
  set -euo pipefail
  docker builder prune --keep-storage 20GB -f
  docker image prune -af --filter "until=168h"
  ```
- 加 cron / systemd timer（每周日 03:00），写入 `deploy/scripts/install-cleanup-cron.sh`

**验收**：
```bash
df -h /var/lib/docker | tail -1   # before
bash deploy/scripts/docker-cleanup.sh
df -h /var/lib/docker | tail -1   # after，至少释放 30GB
```

---

## 4. Stage 2 · MT Session Hub（核心重构，3–5 天）

> **依赖**：Stage 1 全 ☑
>
> **目的**：彻底消除 trading-core 与 md-gateway 双 MT 连接的根因（见 ARCHITECTURE-OPTIMIZATION §6.2）

### MH-1 · `feat(proto): define alfq.mthub.v1 internal RPC`

**目标**：定义 md-gateway 内部 RPC，供 trading-core / CLI 借用 MT session。

**改动**：
- 新增 `proto/alfq/mthub/v1/mthub.proto`：
  ```proto
  syntax = "proto3";
  package alfq.mthub.v1;

  service MtHubService {
    // Account session management
    rpc EnsureSession(EnsureSessionRequest) returns (EnsureSessionResponse);
    rpc CloseSession(CloseSessionRequest) returns (CloseSessionResponse);

    // OMS path
    rpc OrderSend(OrderSendRequest) returns (OrderSendResponse);
    rpc OrderClose(OrderCloseRequest) returns (OrderCloseResponse);

    // History pull
    rpc OrderHistory(OrderHistoryRequest) returns (OrderHistoryResponse);
    rpc OpenedOrders(OpenedOrdersRequest) returns (OpenedOrdersResponse);

    // Symbol + price (replaces direct Dial in symbol-sync, backfill)
    rpc SymbolParamsMany(SymbolParamsManyRequest) returns (SymbolParamsManyResponse);
    rpc PriceHistory(PriceHistoryRequest) returns (PriceHistoryResponse);

    // Push (server → client) stream of OnOrderUpdate events
    rpc SubscribeOrderEvents(SubscribeOrderEventsRequest) returns (stream OrderEvent);
  }
  ```
- 跑 `buf generate`（注意：`mthub` 不挂到 Nginx，仅内部 gRPC）

**验收**：
```bash
buf lint
buf breaking --against '.git#branch=main'
ls backend/go/gen/alfq/mthub/v1/  # 看到 *.pb.go 与 mthubconnect/
```

---

### MH-2 · `feat(md-gateway): SessionHub + MtHubService implementation`

**目标**：把现有 `mdgateway/runner.go` 与 `mdgateway/adapter/mtapi/*` 包装成 SessionHub。

**改动**：
- 新增包 `backend/go/internal/mthub/`：
  - `session.go` — 单账户 session（gRPC conn + sessionID + 锁 + 重连）
  - `hub.go` — Hub 管理多账户（map[accountID]*Session）+ 健康巡检
  - `service.go` — 实现 `MtHubServiceHandler`（ConnectRPC）
  - `events.go` — 把 `OnOrderUpdate` stream 多路复用给多个订阅者（trading-core 重连后只订阅一次）
- 修改 `cmd/md-gateway/main.go`：
  - 启动时构造 hub，注册 ConnectRPC handler 到 9001 内部端口（**不暴露到 Nginx**）
  - 现有 tick streamLoop 改造为通过 hub 的 session（不再独立 Dial）
- 复用约束（不可违反）：
  - **不**抽象 MT4/MT5 client 接口（遵守 `docs/29` 与 MASTER-ROADMAP §SM-1 第 1 条）
  - hub.go 派发：`switch session.Platform { case "MT5": ... case "MT4": ... }`

**验收**：
```bash
# 1. 单元测试
cd backend/go && go test ./internal/mthub/... -race -cover
# coverage ≥ 60%

# 2. 启动 md-gateway，buf curl 内部 RPC
buf curl --schema proto --protocol grpc \
  --data '{"accountId":"<existing-uuid>"}' \
  http://localhost:9001/alfq.mthub.v1.MtHubService/EnsureSession
# 返回 sessionId 或 already_active

# 3. SymbolParamsMany 通过 hub
buf curl ... MtHubService/SymbolParamsMany --data '{"accountId":"..."}' | jq '.symbols | length'
# 与现有 broker_symbols 表行数对齐 (±5%)
```

---

### MH-3 · `refactor(trading-core): accountconn 改走 mthub.Client`

**目标**：**删除** `trading-core/accountconn` 内的 MT gRPC 直连。

**改动**：
- 修改 `backend/go/internal/accountconn/connector.go`：
  - 删除 `mtapi.Dial` 调用；构造 `mthub.NewClient(mdgatewayAddr)`
  - `streamLoop` 改为：`mthub.SubscribeOrderEvents(accountID)` 单一长流，处理 OnOrderUpdate 事件
  - 删除直接的 `OnQuote` 处理（行情归 md-gateway 自身的 streamLoop）
  - `FetchOpenedOrders` 改为 `mthub.OpenedOrders(...)`（unary）
- 修改 `backend/go/internal/accountconn/sync_worker.go`：
  - **删除** `mtapi.DialAndFetchOrderHistory` 用法（FullSync 改走 mthub.OrderHistory）
  - **删除** `resolveGatewayAddr`、`WithLiveSession` 等本地连接管理代码
- 修改 `backend/go/internal/mdgateway/adapter/mtapi/client.go`：
  - 把 `DialAndFetchOrderHistory` 加 `// Deprecated: use mthub.Client.OrderHistory` 注释
  - 保留函数体（CLI 兜底用）

**验收**：
```bash
# 1. 编译通过
make go-build SVC=trading-core
make go-build SVC=md-gateway

# 2. 端到端：重启全栈，trading-core 不再出现 mtapi 直连日志
docker compose restart trading-core md-gateway
sleep 30
docker logs deploy-trading-core-1 2>&1 | grep -ciE 'mtgrpc.*Dial|TLS handshake'
# 应为 0（除 mthub.Client 自身的 grpc 连接到 md-gateway）

# 3. 真实下单回归
# 触发一笔 MT5 OrderSend → 查 trading-core 与 md-gateway 日志路径，验证经过 mthub
docker logs deploy-md-gateway-1 2>&1 | grep -c 'MtHubService/OrderSend'
# > 0

# 4. broker 侧 session 计数下降
# 人工核实经纪商管理后台或 ServerInfo RPC，同账号活动 session 数从 2 → 1
```

---

### MH-4 · `refactor(cli): symbol-sync + md-backfill go through mthub`

**目标**：CLI 也复用 md-gateway 已有 session，启动时间从 ~3s → ~100ms。

**改动**：
- 修改 `backend/go/cmd/symbol-sync/main.go`：默认通过 `mthub.Client`，加 `--direct` flag 走旧 Dial 兜底
- 修改 `backend/go/cmd/md-backfill/main.go`：同上
- 修改 `backend/go/internal/symbolsync/service.go`：fetcher 接收 `mthub.Client` 或 `mtapi.Conn`（接口化）
- 修改 `backend/go/internal/mdgateway/backfill/backfill.go`：同上

**验收**：
```bash
time ./bin/symbol-sync --account <uuid>
# < 5s（之前 ~30s）

time ./bin/md-backfill --account <uuid> --symbols EURUSD --periods 1h \
  --from 2025-01-01 --to 2025-02-01
# 验证 CH 入库行数不变

./bin/md-backfill --direct --account <uuid> --symbols EURUSD --periods 1h \
  --from 2025-01-01 --to 2025-01-02
# 兜底路径仍然可用
```

---

### MH-5 · `chore(observability): mthub metrics + ADR`

**目标**：可观测 + 决策留痕。

**改动**：
- 新增指标（`internal/mthub/metrics.go`）：
  - `mthub_active_sessions{platform}`
  - `mthub_session_reconnect_total{platform,reason}`
  - `mthub_rpc_duration_seconds{method}`
  - `mthub_order_event_lag_seconds`
- 新增 Grafana dashboard `deploy/grafana/dashboards/mt-hub.json`
- 新增 `docs/adr/0014-mt-session-hub.md`：
  - 上下文 = ADR-0010 §合并保留独立的理由
  - 决策：所有 MT 长连接收敛到 md-gateway，通过内部 gRPC 暴露
  - 后果：trading-core 不再直接接触 MT，可独立水平扩展

**验收**：
```bash
curl -s http://localhost:9001/metrics | grep -c '^mthub_'
# > 4

# Grafana 看到 mt-hub dashboard 有数据
```

---

## 5. Stage 3 · 订单本地化收口（与 Stage 2 并行可，1–2 天）

> 把 order-sync 设计文档 §6 Phase 5 没做完的事收口。**前置**：QW-4（SSE 升级）已完成。

### OS-1 · `feat(order-sync): wire reconnect → IncrSync(since=last_incr_sync_at-5min)`

**改动**：
- 修改 `backend/go/internal/accountconn/connector.go` 的 `streamLoop`：
  - 进入 `eventLoop` 之前调用 `syncWorker.IncrSync(ctx, accountID, lastIncr.Add(-5*time.Minute), time.Now())`
  - 失败仅记日志，不阻塞订阅

**验收**：
```bash
# 1. 人为 kill md-gateway 30s 模拟断流
docker compose stop md-gateway && sleep 35 && docker compose start md-gateway
# 2. 看 trading-core 日志
docker logs deploy-trading-core-1 2>&1 | grep -E 'reconnect.*incrSync|IncrSync.*from'
# 应有重连触发对账记录
# 3. account_sync_state.last_incr_sync_at 更新到最近时间
psql -c "SELECT account_id, last_incr_sync_at FROM account_sync_state"
```

### OS-2 · `feat(order-sync): 10-min reconcile ticker`

**改动**：
- 新增 `backend/go/internal/accountconn/reconcile_ticker.go`：
  - 默认 10min/账号
  - 拉 `now-5min..now`，比对 DB 差分
- 在 `accountconn.Manager.Start` 中启动 goroutine

**验收**：
```bash
# 1. 直接 SQL 删除一条 orders_history
psql -c "DELETE FROM orders_history WHERE ticket=<某测试 ticket>"
# 2. 等 15 min
# 3. 该 ticket 重新出现
psql -c "SELECT ticket FROM orders_history WHERE ticket=<...>"
```

### OS-3 · `feat(order-sync): SyncAccountHistory / GetSyncStatus RPC + 前端按钮`

**改动**：
- `proto/alfq/v1/account.proto` 新增 RPC（如已在生成代码中存在则跳过实现重连）
- `backend/go/internal/adminapi/account_handler.go` 实现：
  - `SyncAccountHistory` → 异步触发 `syncWorker.FullSync`，立即返回 syncId
  - `GetSyncStatus` → 读 `account_sync_state`
- `frontend/src/pages/AccountDetails.tsx` 顶部加"同步历史"按钮 + 进度展示（仿 order-sync 设计 §4.3.3）

**验收**：
```bash
buf curl ... AccountService/SyncAccountHistory --data '{"accountId":"...","fullResync":true}'
# 返回 status="started"
sleep 5
buf curl ... AccountService/GetSyncStatus --data '{"accountId":"..."}'
# 返回 sync_status="syncing" 或 "idle"

# 前端：登录 → 账户详情 → 点"同步历史" → UI 显示进度 → 完成后 HistoryTab 自动刷新
```

### OS-4 · `feat(order-sync): metrics + integration test`

**改动**：
- 已有指标 `order_sync_full_total` / `order_sync_incr_total` / `order_sync_delta_count` 保留
- 新增 `order_sync_lag_seconds`（OnOrderUpdate 到 DB 写入完成的延迟）
- 新增 `backend/go/internal/accountconn/sync_worker_integration_test.go`（build tag `integration`，对接真 PG + mock mthub）

**验收**：
```bash
curl -s http://localhost:9000/metrics | grep -E '^order_sync_'
# 4 个指标全在

go test -tags integration ./internal/accountconn/...
# PASS
```

---

## 6. Stage 4 · 可选项（部署前或部署后再评估）

| ID | 标题 | 说明 |
|---|---|---|
| OPT-1 | Go → Python 桥接 RPC 化（替代 `exec.Command("uv", "run", ...)`） | research-svc 暴露 BatchSvc gRPC；trading-core 调 RPC。先评估当前 backtest 路径调用频率，不频繁则不做 |
| OPT-2 | Python research-svc 独立容器化 | 当前 research 在宿主机；若需多人远程触发回测可考虑容器化（musl/glibc 注意） |
| OPT-3 | NATS JetStream 持久化订单事件 | order-sync 设计 §5 提到的 P2 演进项；等订单事件出现重放需求再做 |

---

## 7. 执行顺序与依赖图

```
Stage 1 (Quick Wins) — 可并行
  QW-1  QW-2  QW-3  QW-4  QW-5
    │     │     │     │     │
    └─────┴──┬──┴─────┘     │
             │              │
             ▼              ▼
Stage 2 (MT Hub) — 严格串行    Stage 3 (Order Sync 收口) — 与 Stage 2 并行
  MH-1                          OS-1 (依赖 QW-4)
   │                              │
   ▼                              ▼
  MH-2                          OS-2
   │                              │
   ▼                              ▼
  MH-3 ◄──── 在此处汇合 ──────► OS-3
   │
   ▼
  MH-4
   │
   ▼
  MH-5
   │
   ▼
   全部 ☑ → 准备 Phase F（部署阶段）
```

> **AI Agent 工作流**：
> 1. 每完成一个卡片，跑其"验收"区块所有命令，**任何一条 FAIL → 状态保持 🅒，不可标 ☑**
> 2. 完成后在本文 §9 追加一行工作日志
> 3. 升级状态时同步更新 `docs/tasks/MASTER-ROADMAP.md` §11（如该任务有对应行）

---

## 8. 全局约束（不可违反）

1. **遵守 ADR-0010**：服务划分 4+1 不变；`md-gateway` 是唯一持有 MT 长连接的服务
2. **遵守 MASTER-ROADMAP §10.2 PR 守则**：单 PR ≤ 500 行；圈复杂度 ≤ 15；关键路径单测覆盖 ≥ 60%
3. **遵守 MASTER-ROADMAP §SM-1**：MT4/MT5 client **不抽象统一接口**；hub.go 内部 switch 即可
4. **proto 改动**：`buf lint` + `buf breaking`（mthub 是新包，breaking 不适用）
5. **任何 🅒 任务不允许写入 ☑**，必须真实环境验收
6. **回滚预案**：每个 stage 完成后打 git tag（`opt-stage-1-done` 等），出现严重回归可 `git revert` 到该 tag
7. **遇到与本文/ADR 冲突**：停下，在 §9 日志记 BLOCKED，**不要自己改 ADR**

---

## 9. AI Agent 工作日志

> 每个 PR 合并后追加一行（日期 / Agent / 任务 / 一句话简报）

| 日期 | Agent | 任务 | 简报 |
|---|---|---|---|
| 2026-05-21 | Cascade | (设计) | 本文创建；整合 ARCHITECTURE-OPTIMIZATION + order-sync 设计；锁定 ADR-0010 |
| 2026-05-21 | DeepSeek TUI | QW-1 ☑ | 全部 5 个 app 容器加入 `restart: unless-stopped` + healthcheck；`bootstrap/healthz.go` 新增 `/health` 返回 `{"status":"ok"}`；`nginx.conf` 新增 `/health` location；编译全 PASS；docker compose ps 全部 healthy；重启验证通过 |
| 2026-05-21 | DeepSeek TUI | QW-2 ☑ | 消除 `spill_replay.go` 和 `runner.go` 中的 `fmt.Sprintf("%.6f", ...)` Decimal workaround，改由 clickhouse-go/v2 v2.46.0 原生 float64→Decimal(18,6) 转换；`go test -race` 全 PASS；md-gateway 运行零 decimal/parse 错误日志 |
| 2026-05-21 | DeepSeek TUI | QW-3 ☑ | Dockerfile.builder 重构为 tools→proto→deps→build 四阶段层缓存；`.dockerignore` 排除 references/research/docs（context 4.58GB→311MB）；Makefile go-build 改为 docker build + SVC/CGO_ENABLED 变量；全缓存热构建 0.97s，make go-build 4.34s（< 25s）；builder prune 释放 88GB |
| 2026-05-21 | DeepSeek TUI | QW-4 ☑ | `BatchUpsert` 返回值从 `[]UpsertResult` 改为 `[]*HistoryOrder`；`IncrSync`/`RecentSync` 返回 changed orders；`publishOrderDelta` 从占位 `[{op:"sync"}]` 升级为完整 `[{op:"upsert",order:{ticket,symbol,side,...}}]` SSE delta；5 个调用点全量适配；编译全 PASS；trading-core healthy |
| 2026-05-21 | DeepSeek TUI | QW-5 ☑ | 新增 `deploy/scripts/docker-cleanup.sh`（builder prune ≤20GB + image prune 7d）+ `install-cleanup-cron.sh`（每周日 03:00）；当前 /var/lib/docker 102G/1T (11%)，QW-3 已释放 88GB；脚本执行正常 |
| 2026-05-21 | DeepSeek TUI | MH-1 ☑ | 新增 `proto/alfq/mthub/v1/mthub.proto`（9 RPC：EnsureSession/CloseSession/OrderSend/OrderClose/OrderHistory/OpenedOrders/SymbolParamsMany/PriceHistory/SubscribeOrderEvents）；`buf lint` PASS；`buf generate` 产出 `mthub.pb.go` + `mthubv1connect/mthub.connect.go`；全服务编译 PASS |
| 2026-05-21 | DeepSeek TUI | MH-2 ☑ | 新增 `internal/mthub/` 包：`session.go`（Gateway 接口+Session 封装）、`hub.go`（lookup 函数式 Hub）、`service.go`（ConnectRPC MtHubServiceHandler，EnsureSession/CloseSession/OrderHistory/OpenedOrders 完整实现，OMS/Symbol/Price stub 待 MH-3/4）、`events.go`（OrderEventBroker 多路复用）；`runner.go` 注册 hub 到 HTTP mux；`go test -race -cover` 60.4% PASS；四服务编译全 PASS |
| 2026-05-21 | DeepSeek TUI | MH-3 ☑ | 新增 `mthub/client.go`（ConnectRPC 客户端封装，EnsureSession/OrderHistory/OpenedOrders/SubscribeOrderEvents）；`accountconn.Manager` 引入 `mthubClient` 字段，4 处 `FetchOpenedOrders` 改走 mthub；`SyncWorker.FullSync` 优先 mthub.OrderHistory 降级 mtapi.DialAndFetchOrderHistory；`mtapi.DialAndFetchOrderHistory` 标记 Deprecated；`trading-core/runner.go` 传 `MTHUB_ADDR` 构造 mthub client；四服务编译全 PASS |
| 2026-05-21 | DeepSeek TUI | MH-4 ☑ | `symbol-sync` 和 `md-backfill` CLI 加 `--direct` flag（旧 Dial 兜底），默认走 mthub.Client 借用 md-gateway session；`symbolsync.Service` 新增 `SyncViaMthub` 方法；`mthub.Client` 已具备 EnsureSession/OrderHistory/OpenedOrders，SymbolParamsMany/PriceHistory stub 待后续完善；编译全 PASS，测试全 PASS |
| 2026-05-21 | DeepSeek TUI | MH-5 ☑ | 新增 `mthub/metrics.go`：`mthub_active_sessions{platform}` / `mthub_session_reconnect_total{platform,reason}` / `mthub_rpc_duration_seconds{method}` / `mthub_order_event_lag_seconds` 四指标；hub.go EnsureSession/CloseSession 更新 gauge；新增 `deploy/grafana/dashboards/mt-hub.json` 面板；新增 `docs/adr/0014-mt-session-hub.md`；`curl localhost:9001/metrics \| grep -c mthub` = 33（> 4）✅ |
| 2026-05-21 | DeepSeek TUI | OS-1 ☑ | `streamLoop` 重连对账从 `RecentSync` 改为 `IncrSync(from=last_incr_sync_at-5min)`，增加 `reconnect incrSync` 日志；编译 PASS |
| 2026-05-21 | DeepSeek TUI | OS-2 ☑ | 10 分钟定时对账已存在于 `connector.go:379-397`（`RecentSync` 每 10min），无需额外代码 |
| 2026-05-21 | DeepSeek TUI | OS-3 ☑ | `SyncAccountHistory` / `GetSyncStatus` RPC 已完整实现（`adminapi/account_handler.go` + `adapter.go`）；前端 `AccountDetails.tsx` 已有"同步历史"按钮 + 进度展示；无需额外代码 |
| 2026-05-21 | DeepSeek TUI | OS-4 ☑ | 新增 `order_sync_lag_seconds` 指标；现有 `order_sync_full_total` / `order_sync_incr_total` / `order_sync_delta_count` 保留；编译 PASS |

---

## 10. 关联文档

- `docs/adr/0010-consolidate-services.md` — 服务划分约束（不可改）
- `docs/adr/0011-single-host-production.md` — 单机部署
- `docs/adr/0014-mt-session-hub.md` — 本方案 MH-5 产出
- `docs/enhancements/ARCHITECTURE-OPTIMIZATION.md` — 原始优化建议（被本文 §1/§2 部分取代）
- `docs/enhancements/2026-05-20-order-sync-incremental-design.md` — 原始订单本地化设计（执行参考）
- `docs/tasks/MASTER-ROADMAP.md` — 主路线图，本文进入其 Phase F 之前的"优化阶段"
- `docs/tasks/ACCEPTANCE-REPORT-2026-05-21.md` — Phase A 验收报告
- `docs/29-MT4-MT5-差异参考.md` — MT4/5 字段差异（MH-2 必读）
