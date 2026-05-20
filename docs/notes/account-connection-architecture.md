# ALFQ 账户长连接与实时推送功能 — 技术总结报告

> 扫描时间：2026-05-20  
> 范围：`accountconn` / `ssehub` / `tradingcore` / `adminapi` / `frontend` / `migrations`

---

## 一、架构全景图

```
┌─────────────────────────────────────────────────────────────┐
│                        MT4 / MT5 Server                     │
│   gRPC API: Connect, AccountSummary, OnOrderProfit (stream) │
└──────────────┬──────────────────────────────────────────────┘
               │ TLS gRPC (mt5grpc3.mtapi.io:443)
               ▼
┌─────────────────────────────────────────────────────────────┐
│                    trading-core (Go)                        │
│                                                             │
│  ┌───────────────────┐    ┌──────────────────────────────┐  │
│  │ accountconn       │    │ ssehub                       │  │
│  │  Manager          │    │  Hub                         │  │
│  │  ├ run()          │    │  ├ ServeHTTP (SSE endpoint)  │  │
│  │  ├ streamLoop()   │    │  └ Broadcast(data []byte)    │  │
│  │  ├ eventLoop()    │    └──────────┬───────────────────┘  │
│  │  └ pollLoop()     │               │                      │
│  └───┬───────────────┘               │                      │
│      │ NATS Publish                   │ NATS Subscribe       │
│      ▼                               │                      │
│  ┌─────────┐     ┌──────────┐       │                      │
│  │  NATS   │────▶│ trading- │───────┘                      │
│  │  Pub/Sub│     │ core     │                               │
│  └─────────┘     └──────────┘                               │
│      │                                                      │
│      └──▶ PostgreSQL (accounts 表)                          │
│      └──▶ Redis (alfq:account:<id>, TTL 120s)               │
└─────────────────────────────────────────────────────────────┘
               │ /sse/accounts (text/event-stream)
               ▼
┌─────────────────────────────────────────────────────────────┐
│                     nginx (frontend)                        │
│  proxy_buffering off, proxy_read_timeout 86400s             │
└──────────────┬──────────────────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────────────────────┐
│                    Browser (React)                          │
│  new EventSource("/sse/accounts")                           │
│  └ onmessage → JSON.parse → setItems() → 表格实时刷新       │
└─────────────────────────────────────────────────────────────┘
```

---

## 二、数据流路径（三条并行通道）

### 通道 1：`OnOrderProfit` gRPC 流（主通道，~2-10次/秒）
```
MT Server → OnOrderProfit stream
  → eventLoop().Recv()
  → 提取 Balance, Equity
  → Profit = Equity - Balance（MT 公式）
  → DB QueryRow 读取 margin/freeMargin/marginLevel/currency/leverage
  → 合并成 AccountInfo
  → publishAndUpdate()
      ├→ NATS Publish("account.status.<id>", JSON)
      └→ DB UPDATE accounts SET balance=, equity=, profit=, ...
```

### 通道 2：`FetchAccountSummary` 周期轮询（辅助通道，~30s）
```
eventLoop() 子 goroutine → ticker 30s
  → mtapi.FetchAccountSummary(conn, platform, sessionID)
  → 获取全量字段（Balance/Equity/Margin/FreeMargin/MarginLevel/Profit/Currency/Leverage）
  → publishAndUpdate()
```

### 通道 3：NATS → SSE → 前端（广播通道，实时）
```
accountconn.publish()
  → nc.Publish("account.status.*", JSON)
  → trading-core nc.Subscribe("account.status.*")
  → sse.Broadcast(data)
  → 遍历所有 SSE 客户端 chan → fmt.Fprintf(w, "data: %s\n\n")
  → nginx proxy_buffering off
  → 前端 EventSource.onmessage → setItems()
```

---

## 三、核心模块详解

### 3.1 accountconn.Manager — 连接管理器

**文件**: `backend/go/internal/accountconn/connector.go`

| 方法 | 用途 |
|------|------|
| `NewManager(log, pool, rdb, nc, js, mt4gw, mt5gw)` | 创建管理器，注入 PG/Redis/NATS/Gateway 依赖 |
| `Connect(ctx, info)` | 启动一个 per-account 持久连接 goroutine |
| `Disconnect(accountID)` | 取消 context，停止 goroutine |
| `Shutdown()` | 批量停止全部连接（优雅关闭） |
| `ActiveCount()` | 活跃会话数 |
| `run(ctx, info)` | **核心循环**：dial → streamLoop → 指数退避重连 |
| `streamLoop(ctx, conn, info)` | 连接 → 首轮 AccountSummary → eventLoop（fallback pollLoop） |
| `eventLoop(ctx, conn, info, sessionID)` | 订阅 OnOrderProfit 流 + 后台 30s 全量轮询 |
| `pollLoop(ctx, conn, info, sessionID)` | 降级方案：每 10s 轮询 AccountSummary |
| `mtConnect(ctx, conn, info)` | MT4/MT5 gRPC Connect 认证，返回 sessionID |
| `publish(ctx, accountID, info)` | NATS `nc.Publish` 发送 JSON |
| `updateDB(ctx, accountID, info)` | DB UPDATE + Redis `SET`（TTL 120s） |
| `updateStatus(ctx, accountID, status, err)` | 写入错误状态到 DB |

**重连策略**（`run` 函数）:
```
backoff = 1s
loop:
  dial → streamLoop
  on error → updateStatus("error")
  sleep(backoff), backoff = min(backoff*2, 60s)
  if ctx.Done() → return
```

**流字段合并策略**（`eventLoop` 函数）:
- `OnOrderProfit` 流仅提供 Balance + Equity
- `Profit = Equity - Balance`（MT 公式，浮动盈亏实时计算）
- margin/freeMargin/marginLevel/currency/leverage 从 DB 读取合并
- 策略保证 浮动盈亏与净值同步实时更新，不被后续流事件清零

### 3.2 ssehub.Hub — SSE 广播器

**文件**: `backend/go/internal/ssehub/hub.go`

| 方法 | 用途 |
|------|------|
| `New()` | 创建 Hub |
| `ServeHTTP(w, r)` | SSE 端点处理：注册 chan → 发送 `{"type":"connected"}` → 阻塞等待 Broadcast |
| `Broadcast(data)` | 向所有客户端 chan 推送，满 chan 跳过（慢客户端保护） |
| `ClientCount()` | 当前连接数 |

关键参数：
- 客户端 chan buffer: 64
- SSE Content-Type: `text/event-stream`
- 跨域: `Access-Control-Allow-Origin: *`

### 3.3 runner.go — 集成入口

**文件**: `backend/go/internal/tradingcore/runner.go`

关键集成：
1. `accountconn.NewManager(d.Log, d.PG, d.RDB, nc, js, ...)` — 注入 Redis
2. 启动重连：查询 `status IN ('connected','error') AND is_disabled=false`
3. `svc.WithAccountConnector(&acctAdapter{acctMgr})` — 类型适配
4. `mux.HandleFunc("/sse/accounts", sse.ServeHTTP)` — SSE 端点
5. `nc.Subscribe("account.status.*", sse.Broadcast)` — NATS→SSE 桥接
6. `shutdown` 闭包：`acctMgr.Shutdown()` → `nc.Close()` → `busClient.Close()`

### 3.4 adminapi — 绑定触发点

**文件**: `backend/go/internal/adminapi/account_handler.go`

`CreateAccount` 成功后触发长连接：
```go
if s.acctConn != nil {
    s.acctConn.Connect(context.Background(), AccountInfo{
        ID: a.Id, Login: req.Login, Password: req.Password,
        Server: brokerHost, Platform: platform,
    })
}
```

**文件**: `backend/go/internal/adminapi/service.go`

`AccountConnector` 接口解耦 adminapi 和 accountconn，通过 `acctAdapter` 适配类型。

### 3.5 bootstrap — 优雅关闭

**文件**: `backend/go/internal/common/bootstrap/server.go`
- `ServeMuxAdapter.OnShutdown []func()` — 注册关闭钩子

**文件**: `backend/go/internal/common/bootstrap/bootstrap.go`
- 收到 SIGTERM 后逆序执行 `OnShutdown` 钩子

---

## 四、数据库 Schema 变更

### accounts 表最终结构（关键列）

| 列 | 类型 | 说明 | 迁移 |
|----|------|------|------|
| `user_id` | UUID NOT NULL | 归属用户 | `005_` |
| `platform` | TEXT NOT NULL | `mt4`/`mt5` | `006_` |
| `server_name` | TEXT | 显示名（Exness-Trial） | proto+代码 |
| `server` | TEXT | 连接地址（IP:Port） | 原有 |

### 迁移文件

| 文件 | 内容 |
|------|------|
| `005_add_account_user_id.sql` | user_id 列、FK 约束、UNIQUE(user_id, broker_id, login) |
| `006_add_account_platform.sql` | platform 列，解决重连时 MT4/MT5 协议混淆 |

---

## 五、Proto 扩展

**文件**: `backend/proto/alfq/v1/broker.proto`

```protobuf
message Account {
  string server_name = 22;  // 显示名
  string platform = 23;     // "MT4" 或 "MT5"
}
message CreateAccountRequest {
  string server_name = 8;   // 绑定传入
}
```

---

## 六、前端关键代码

### 6.1 SSE 实时更新
**文件**: `frontend/src/pages/Accounts.tsx`

```typescript
useEffect(() => {
  const es = new EventSource("/sse/accounts");
  es.onmessage = (e) => {
    const update = JSON.parse(e.data);
    setItems(prev => prev.map(a =>
      a.id === update.accountId
        ? { ...a, balance: update.balance, equity: update.equity,
            profit: update.profit, ... }
        : a
    ));
  };
  return () => es.close();
}, []);
```

### 6.2 Auth Interceptor
**文件**: `frontend/src/api/client.ts`

```typescript
const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getToken();
  if (token) req.header.set("Authorization", `Bearer ${token}`);
  return next(req);
};
```

### 6.3 Nginx SSE 代理
**文件**: `frontend/nginx.conf`

```nginx
location /sse/ {
    proxy_pass http://trading-core:9000/sse/;
    proxy_buffering off;         # 关键：SSE 不能缓冲
    proxy_read_timeout 86400s;   # 24h 长连接
    chunked_transfer_encoding off;
}
```

---

## 七、关键技术决策

| 决策 | 理由 |
|------|------|
| 事件驱动而非轮询 | gRPC `OnOrderProfit` 流原生推送，延迟最低 |
| 每账户一 goroutine | 故障隔离，无锁竞争 |
| 流字段从 DB 合并 | 流数据不完整（仅 Balance/Equity），其他字段不能置零 |
| `Profit = Equity - Balance` | MT 标准公式，实时推导，与净值同步更新 |
| 指数退避重连 (1s→60s) | 标准重连策略，避免打爆服务器 |
| NATS pub/sub 中转 | 解耦推送源和消费端，SSE hub 无状态 |
| Redis 可选（2min TTL） | 热缓存加速，不可用时自动降级 |
| `--no-cache` Docker 构建 | 解决构建层缓存导致前端代码不生效 |
| `accounts.platform` 独立存储 | 避免重连时依赖 brokers 表导致 MT4/MT5 协议混淆 |
