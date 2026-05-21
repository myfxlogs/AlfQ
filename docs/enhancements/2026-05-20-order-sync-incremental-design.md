# 历史订单本地化同步与增量更新机制 — 实施设计

> **状态**: Approved (2026-05-20)
> **类型**: Enhancement (Incremental Optimization)
> **作者**: ALFQ Tech Team
> **适用范围**: AI Agent 实施账号订单同步功能的指导与依据
> **关联代码**:
> - `backend/go/internal/accountconn/connector.go`
> - `backend/go/internal/adminapi/account_handler.go`
> - `backend/go/internal/mdgateway/adapter/mtapi/client.go`
> - `frontend/src/pages/AccountDetails.tsx`

---

## 1. 背景与问题陈述

### 1.1 现状

历史订单数据获取走"实时穿透拉取"路线：

- 前端每次切换 tab / 时间范围 / 收到 `orderEvent` SSE 信号，都会调用 `ListAccountOrders`。
- 后端 handler 通过 `mtapi.FetchOrderHistory` 直接打到 `mt4grpc3.mtapi.io` / `mt5grpc3.mtapi.io` 网关，单次超时 30s。
- 没有本地持久化，每次都是**全量重拉**。

### 1.2 痛点

| 维度 | 当前 | 目标 |
|---|---|---|
| 历史订单首屏延迟 | 200ms ~ 30s | < 100ms |
| 增量更新延迟 | 网关 RTT × 2（拉取+解析） | < 50ms（DB 直读 + SSE 增量） |
| MT 网关调用频次 | 每次订单事件触发全量重拉 | 仅断流对账 + 兜底校验 |
| 数据可靠性 | 网关单点，离线即不可用 | 本地 DB 优先，网关掉线不影响读 |

### 1.3 关键约束

- MT 账号可**多端登录**，平仓操作不一定来自 ALFQ 客户端 → `OnOrderUpdate` 必须能捕获服务器侧所有交易活动。
- mtapi.io 桥接服务的 gRPC stream **没有持久化重放能力**，断流期间事件会永久丢失 → 必须有对账兜底。

---

## 2. 目标与非目标

### 2.1 目标

1. 历史订单数据**本地持久化**，前端读操作完全走本地 DB。
2. **三层同步机制**：账号绑定首次全量 → `OnOrderUpdate` 实时增量 → 断流/兜底对账。
3. SSE 推送从"事件信号"升级为"**增量数据**"，前端 merge 即可，不再触发全量重拉。
4. 数据**幂等写入**，支持多 session 并发、重复事件去重。

### 2.2 非目标

- 不引入 CQRS / 事件溯源架构（过度设计）。
- 不替换现有 mtapi.io 网关（合约稳定）。
- 不实现实时持仓的 DB 持久化（持仓数据高频跳动，仍走内存 + SSE）。

---

## 3. 架构总览

```
┌──────────────────────────────────────────────────────────┐
│                       MT Gateway                          │
│  (mt4grpc3.mtapi.io / mt5grpc3.mtapi.io)                  │
└────────┬──────────────────────────────┬──────────────────┘
         │ Unary RPC                    │ Stream
         │  OrderHistory                │  OnOrderUpdate
         │  OpenedOrders                │  OnOrderProfit
         ▼                              ▼
┌──────────────────────────────────────────────────────────┐
│                  accountconn.Manager                      │
│                                                           │
│  ┌────────────────┐  ┌─────────────────┐                 │
│  │ Sync Worker    │  │  Stream Loop    │                 │
│  │  - 绑定全量    │  │  - 实时增量    │                 │
│  │  - 重连对账    │  │  - 服务端事件  │                 │
│  │  - 手动同步    │  │                 │                 │
│  │  - 定时兜底    │  │                 │                 │
│  └───────┬────────┘  └────────┬────────┘                 │
│          │                     │                          │
│          └──────────┬──────────┘                          │
│                     │ UPSERT (account_id, ticket)         │
│                     ▼                                     │
│             ┌──────────────────┐                          │
│             │ orders_history   │ Postgres                 │
│             │ (tenant-scoped)  │                          │
│             └──────────────────┘                          │
│                     │                                     │
│                     │ 写入差分 → 仅推送真实变更           │
│                     ▼                                     │
│              ┌─────────────┐                              │
│              │  NATS       │ account.orders.{accountId}   │
│              └─────┬───────┘                              │
└────────────────────┼─────────────────────────────────────┘
                     ▼
              ┌─────────────┐
              │  SSE Hub    │  /sse/accounts
              └─────┬───────┘
                    ▼
        ┌──────────────────────┐
        │  Frontend            │
        │   - 首屏: REST 读 DB │
        │   - 增量: SSE merge  │
        │   - 同步按钮: 触发拉取│
        └──────────────────────┘
```

---

## 4. 详细设计

### 4.1 数据库设计

#### 4.1.1 新建表 `orders_history`

```sql
CREATE TABLE orders_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    ticket          BIGINT NOT NULL,
    symbol          TEXT NOT NULL,
    side            TEXT NOT NULL,                     -- 'buy' | 'sell'
    lots            NUMERIC(20, 4) NOT NULL,
    open_price      NUMERIC(20, 5) NOT NULL,
    close_price     NUMERIC(20, 5),
    profit          NUMERIC(20, 4) NOT NULL DEFAULT 0,
    swap            NUMERIC(20, 4) NOT NULL DEFAULT 0,
    commission      NUMERIC(20, 4) NOT NULL DEFAULT 0,
    open_time       TIMESTAMPTZ NOT NULL,
    close_time      TIMESTAMPTZ,                       -- NULL 表示尚未平仓（也可单独表，见 4.1.2）
    state           TEXT NOT NULL DEFAULT 'closed',    -- 'pending' | 'open' | 'closed' | 'cancelled'
    raw_payload     JSONB,                             -- 原始 MT 字段，便于调试与扩展
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, ticket)
);

CREATE INDEX idx_orders_history_account_close_time
    ON orders_history (account_id, close_time DESC NULLS LAST);
CREATE INDEX idx_orders_history_tenant
    ON orders_history (tenant_id);

-- RLS（与现有 multi-tenant 一致）
ALTER TABLE orders_history ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON orders_history
    USING (tenant_id = current_setting('app.tenant_id')::uuid);
```

#### 4.1.2 持仓与历史分离

- `positions`：内存维护（已有），实时跳动的浮动盈亏不入库。
- `orders_history`：仅存"已平仓"或"已取消"的订单（终态）。
- 状态机：`pending → open → closed | cancelled`，只允许向前推进。

#### 4.1.3 同步状态表 `account_sync_state`

```sql
CREATE TABLE account_sync_state (
    account_id        UUID PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    last_full_sync_at TIMESTAMPTZ,                     -- 最后一次成功全量同步时间
    last_incr_sync_at TIMESTAMPTZ,                     -- 最后一次成功增量/对账时间
    sync_status       TEXT NOT NULL DEFAULT 'idle',    -- 'idle' | 'syncing' | 'error'
    last_error        TEXT,
    total_synced      INTEGER NOT NULL DEFAULT 0,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.2 后端实现

#### 4.2.1 同步入口（三种触发）

| 触发场景 | 调用方 | 行为 |
|---|---|---|
| **账号绑定成功** | `CreateAccount` handler | 异步启动全量同步 worker（分批，按月窗口） |
| **重连成功** | `streamLoop` 进入 `eventLoop` 之前 | 拉取 `(last_incr_sync_at - 5min, now)` 做对账 |
| **手动同步** | 前端"同步历史"按钮 → `SyncAccountHistory` RPC | 与绑定时同步逻辑相同，可指定时间范围 |
| **定时兜底** | 后台 ticker（默认 10min） | 拉取 `(last_incr_sync_at - 5min, now)` 校验差分 |

#### 4.2.2 UPSERT 语句（核心幂等）

```sql
INSERT INTO orders_history
    (tenant_id, account_id, ticket, symbol, side, lots,
     open_price, close_price, profit, swap, commission,
     open_time, close_time, state, raw_payload, updated_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, now())
ON CONFLICT (account_id, ticket) DO UPDATE
SET symbol      = EXCLUDED.symbol,
    side        = EXCLUDED.side,
    lots        = EXCLUDED.lots,
    open_price  = EXCLUDED.open_price,
    close_price = EXCLUDED.close_price,
    profit      = EXCLUDED.profit,
    swap        = EXCLUDED.swap,
    commission  = EXCLUDED.commission,
    open_time   = EXCLUDED.open_time,
    close_time  = EXCLUDED.close_time,
    state       = EXCLUDED.state,
    raw_payload = EXCLUDED.raw_payload,
    updated_at  = now()
WHERE
    -- 防止旧事件覆盖新状态：以 close_time 为版本字段
    orders_history.close_time IS NULL
    OR (EXCLUDED.close_time IS NOT NULL AND EXCLUDED.close_time >= orders_history.close_time)
RETURNING (xmax = 0) AS inserted, id;
```

- `xmax = 0` 表示真插入（非 update），用于区分"新订单"和"更新"。
- `RETURNING` 用于判断是否需要广播 SSE（无变更不推送）。

#### 4.2.3 OnOrderUpdate 处理流程

```go
// 伪代码：accountconn/order_sync.go
func (m *Manager) onOrderUpdate(ctx context.Context, info AccountInfo, sessionID string, conn *grpc.ClientConn) error {
    stream, err := subscribeOrderUpdateStream(ctx, conn, info.Platform, sessionID)
    if err != nil { return err }

    for {
        evt, err := stream.Recv()
        if err != nil { return err }

        // 1. 收到事件 → 拉取最近 5min 窗口的订单（防止 stream payload 不全）
        orders, err := mtapi.FetchOrderHistory(ctx, conn, info.Platform, sessionID,
            timeRangeRecent(5*time.Minute))
        if err != nil { m.log.Warn(...); continue }

        // 2. UPSERT 到 DB，返回真正变更的订单
        changed := m.upsertOrders(ctx, info, orders)

        // 3. 只对真正变更的订单广播 SSE
        if len(changed) > 0 {
            m.publishOrderDelta(info.ID, changed)
            m.updateSyncState(ctx, info.ID, "incr")
        }
    }
}
```

#### 4.2.4 SSE 消息格式升级

**旧格式**（仅事件信号）：
```json
{ "accountId": "...", "orderEvent": true }
```

**新格式**（增量数据）：
```json
{
  "accountId": "uuid",
  "type": "order_delta",
  "changes": [
    {
      "op": "upsert",
      "order": {
        "ticket": 123456,
        "symbol": "EURUSD",
        "side": "buy",
        "lots": 0.1,
        "openPrice": 1.0850,
        "closePrice": 1.0870,
        "profit": 20.0,
        "openTime": "2026-05-20T10:00:00Z",
        "closeTime": "2026-05-20T10:30:00Z",
        "state": "closed"
      }
    }
  ]
}
```

> 保留 `account.status.{id}`（balance/equity/positions）与 `account.orders.{id}`（订单增量）两个独立 subject，避免互相干扰。

#### 4.2.5 新增 RPC

在 `account.proto` 添加：

```proto
service AccountService {
    // ... existing ...
    rpc SyncAccountHistory(SyncAccountHistoryRequest) returns (SyncAccountHistoryResponse);
    rpc GetSyncStatus(GetSyncStatusRequest) returns (GetSyncStatusResponse);
}

message SyncAccountHistoryRequest {
    string account_id = 1;
    string from = 2;             // 可选，默认账号开户以来
    string to = 3;               // 可选，默认 now
    bool full_resync = 4;        // 强制全量重同步
}

message SyncAccountHistoryResponse {
    string sync_id = 1;
    string status = 2;           // 'started' | 'in_progress' | 'completed'
}
```

`ListAccountOrders` handler 改为**读本地 DB**：

```go
func (s *Service) ListAccountOrders(ctx context.Context, req *pb.ListAccountOrdersRequest) (*pb.ListAccountOrdersResponse, error) {
    if err := s.setRLS(ctx); err != nil { return nil, err }
    rows, err := s.pool.Query(ctx, `
        SELECT ticket, symbol, side, lots, open_price, close_price,
               profit, swap, commission, open_time, close_time
        FROM orders_history
        WHERE account_id = $1
          AND close_time BETWEEN $2 AND $3
        ORDER BY close_time DESC
        LIMIT 1000
    `, req.AccountId, req.From, req.To)
    // ... scan + return
}
```

### 4.3 前端实现

#### 4.3.1 数据流改造

| 旧 | 新 |
|---|---|
| `useEffect` 监听 `orderEventTick` → 全量重拉 | 初始化时拉一次 → SSE 增量 merge |
| 时间范围切换 → 重拉 | 时间范围切换 → 本地过滤（或带参数重拉 DB） |
| SSE 仅传 `orderEvent: true` | SSE 传 `changes[]` 增量 |

#### 4.3.2 HistoryTab 改造（伪代码）

```tsx
function HistoryTab({ accountId, orderDeltas }: Props) {
    const [orders, setOrders] = useState<Order[]>([]);

    // 初始拉取：仅 1 次
    useEffect(() => {
        accountClient.listAccountOrders({ accountId, from, to })
            .then(res => setOrders(res.orders));
    }, [accountId, timeRange]);

    // SSE 增量 merge
    useEffect(() => {
        if (!orderDeltas) return;
        setOrders(prev => mergeDeltas(prev, orderDeltas));
    }, [orderDeltas]);

    // ...
}

function mergeDeltas(orders: Order[], deltas: OrderDelta[]): Order[] {
    const map = new Map(orders.map(o => [o.ticket, o]));
    for (const d of deltas) {
        if (d.op === 'upsert') map.set(d.order.ticket, d.order);
        else if (d.op === 'delete') map.delete(d.order.ticket);
    }
    return Array.from(map.values()).sort((a, b) => b.closeTime - a.closeTime);
}
```

#### 4.3.3 同步按钮

`AccountDetails` 头部"同步历史"按钮调用 `SyncAccountHistory`，并通过 `GetSyncStatus` 轮询展示进度（如"同步中 320/1500"）。

---

## 5. 替代方案对比

| 方案 | 优点 | 缺点 | 决策 |
|---|---|---|---|
| **A. 本设计 (DB + SSE 增量)** | 实时性好，前端轻量，复用现有 NATS+SSE | 需保证 DB 是事件流唯一出口 | ✅ 采用 |
| B. CQRS + 事件溯源 (JetStream 持久化) | 可重放、跨服务消费 | 复杂度高，对单账号场景过度设计 | ❌ |
| C. 纯轮询 + Redis 缓存 | 实现最简单 | 实时性差，无法支撑策略联动 | ❌ |

> **可选增强**：将 `OnOrderUpdate` 派生事件先发到 NATS JetStream（持久化 24h），再由 consumer 写 DB。这样订单事件还可供未来的策略 / 风控 / 审计模块复用。当前阶段**不强制**，作为 P2 演进项。

---

## 6. 实施步骤（按优先级）

### Phase 1 — 基础设施（P0）

1. **数据库迁移**
   - 新增 `orders_history` 表 + RLS + 索引
   - 新增 `account_sync_state` 表
   - 文件：`backend/go/migrations/NNN_orders_history.sql`

2. **新增 OrderRepo**
   - 路径：`backend/go/internal/oms/repo/order_history_repo.go`
   - 暴露 `Upsert(ctx, order) (changed bool, err error)` 和 `List(ctx, accountID, from, to)`

### Phase 2 — 同步逻辑（P0）

3. **全量同步 Worker**
   - 路径：`backend/go/internal/accountconn/sync_worker.go`
   - 分批按月窗口拉取，写入 `orders_history`
   - 更新 `account_sync_state`

4. **绑定后触发全量同步**
   - 修改 `CreateAccount` handler，账号绑定成功后异步调用 `SyncWorker.FullSync(accountID)`

5. **重连后对账拉取**
   - 修改 `streamLoop`，在 `eventLoop` 之前调用 `SyncWorker.IncrSync(accountID, since)`

6. **OnOrderUpdate 处理升级**
   - 现有 `subscribeOrderUpdateStream` 收到事件后，调用 UPSERT 而非仅广播 flag

### Phase 3 — API 与 SSE 改造（P1）

7. **新增 RPC**
   - `SyncAccountHistory` / `GetSyncStatus`
   - 修改 `ListAccountOrders` → 读本地 DB

8. **SSE 消息格式升级**
   - 新增 subject `account.orders.{id}`
   - payload 改为增量数据

### Phase 4 — 前端改造（P1）

9. **AccountDetails / HistoryTab**
   - 改为 SSE 增量 merge 模式
   - 接入 `SyncAccountHistory` 按钮 + 进度展示

### Phase 5 — 兜底与可观测（P2）

10. **定时对账 Ticker**
    - 默认 10min / 账号 / 拉取最近 5min 窗口做差分校验

11. **监控指标**
    - `order_sync_full_total`, `order_sync_incr_total`, `order_sync_delta_count`
    - `order_sync_lag_seconds`（事件触达 → DB 写入完成）

12. **可选**: NATS JetStream 持久化订单事件

---

## 7. 风险与回滚

### 7.1 风险

| 风险 | 缓解措施 |
|---|---|
| 首次全量同步耗时长 | 分批 + 异步 + 前端进度展示 |
| `OnOrderUpdate` 丢消息 | 5min 重叠窗口的对账拉取 + 定时兜底 |
| 多端登录并发写入 | UPSERT + close_time 版本守卫 |
| mtapi.io 返回数据格式变化 | 保留 `raw_payload` JSONB，便于回溯 |
| 历史数据回填影响 DB 性能 | 按月分批，批间 sleep；大账号可考虑后台低优先级队列 |

### 7.2 回滚方案

- 数据库迁移可回滚（保留旧 `ListAccountOrders` 走网关的代码路径，通过 feature flag 切换）。
- 前端通过环境变量 `VITE_ORDER_LOCAL_DB=true|false` 控制读取来源。

---

## 8. 验收标准

| 指标 | 当前 | 目标 |
|---|---|---|
| 历史订单首屏延迟 (P95) | 3000ms | < 200ms |
| 平仓事件到 UI 可见延迟 (P95) | 800ms | < 100ms |
| MT 网关 `OrderHistory` 调用次数 / 账号 / 小时 | ~120 | < 10（仅对账） |
| 数据丢失率（与定时对账差分对比） | 不可测 | < 0.01% |
| 多端登录场景的订单数据一致性 | 不保证 | 100% （以服务端为准） |

---

## 9. 关联文档

- [`docs/02-数据库设计.md`](../02-数据库设计.md) — `accounts` 表设计
- [`docs/03-API与接口规范.md`](../03-API与接口规范.md) — RPC 规范
- [`docs/21-跨服务一致性与幂等.md`](../21-跨服务一致性与幂等.md) — 幂等性设计原则
- [`docs/29-MT4-MT5-差异参考.md`](../29-MT4-MT5-差异参考.md) — MT4/5 字段差异

---

## 10. 变更历史

| 日期 | 版本 | 变更 |
|---|---|---|
| 2026-05-20 | v1.0 | 初稿，Approved |
