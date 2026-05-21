---
name: mt-connector
description: |
  MT4/MT5 mtapi gRPC connection and real-time quote retrieval patterns.
  Use when implementing or debugging MT gateway Connect / Subscribe / OnQuote /
  GetQuote calls, CH writer context issues, or metadata header requirements.
  Triggered by tasks involving gateway_mt4.go, gateway_mt5.go, runner.go,
  clickhouse_writer.go, or "no ticks in ClickHouse" / "OnQuote not receiving"
  problems.
---

# MT Connector — Correct Connection & Quote Patterns

> 验证环境：mtapi.io MT4/MT5 gateway（`mt4grpc3.mtapi.io:443` / `mt5grpc3.mtapi.io:443`）
> 参考实现：`/opt/alfq/references/anttrader/backend/internal/mt5client/`

## 1. 核心原则

**MT4 ≠ MT5**。两者的 gRPC 方法名相同、字段名相同（`Symbols` 等），但 proto 结构不同，必须各自独立实现，不可共用抽象层。

详见 `docs/29-MT4-MT5-差异参考.md`。

## 2. Connect 模式

### 必须做的事

```
1. 生成 temp UUID 作为初始 session id
2. metadata.AppendToOutgoingContext(ctx, "id", tempID)  ← 关键！
3. 解析 login string → uint64/int32
4. 调用 Connection.Connect(ctxWithID, req)
5. 检查 resp.GetError()（gRPC 成功不代表 MT 成功）
6. 验证 resp.GetResult() 非空
```

### 代码模板（MT5）

```go
func (g *mt5Gateway) Connect(ctx context.Context) error {
    cli, err := mt5.Dial(ctx, mt5.DefaultEndpoint)
    // ...
    tempID := "mdgw-" + g.cfg.Login
    ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", tempID)

    user, _ := parseUint64(g.cfg.Login)
    resp, err := cli.Connection.Connect(ctxWithID, &mt5pb.ConnectRequest{
        Host: g.cfg.Host, Port: 443, User: user, Password: g.cfg.Password,
    })
    if err != nil { return err }
    if resp.GetError() != nil && resp.GetError().GetMessage() != "" {
        return fmt.Errorf("mt5 connect error: %s", resp.GetError().GetMessage())
    }
    g.session = resp.GetResult()
    if g.session == "" {
        return fmt.Errorf("empty session id")
    }
    return nil
}
```

MT4 同理，字段差异：`User: int32(user)`, `Id: &tempID`（MT4 ConnectRequest 有 Id 字段）。

## 3. 实时报价获取

### 三步流程

```
1. SubscribeMany(symbols) → 加入 MarketWatch
2. Streams.OnQuote(session) → 开启推送流
3. goroutine 中 stream.Recv() → 持续接收
```

**每个 gRPC 调用都必须带 metadata header：**

```go
mdCtx := metadata.AppendToOutgoingContext(ctx, "id", g.session)
```

### 代码模板

```go
func (g *mt5Gateway) Subscribe(ctx context.Context, symbols []string, handler TickHandler) error {
    // 独立 context —— 调用者 ctx 可能短生命周期
    streamCtx, streamCancel := context.WithCancel(context.Background())
    g.streamCancel = streamCancel
    mdCtx := metadata.AppendToOutgoingContext(streamCtx, "id", g.session)

    // 1. SubscribeMany
    if len(symbols) > 0 {
        subReq := &mt5pb.SubscribeManyRequest{Id: g.session, Symbols: symbols}
        if _, err := g.client.Subscriptions.SubscribeMany(mdCtx, subReq); err != nil {
            return err
        }
    }

    // 2. OnQuote
    stream, err := g.client.Streams.OnQuote(mdCtx, &mt5pb.OnQuoteRequest{Id: g.session})
    if err != nil { return err }

    // 3. Recv loop
    go func() {
        defer func() { g.running = false }()
        for {
            reply, err := stream.Recv()
            if err != nil { return }
            if reply.GetError() != nil { continue }  // MT error in body
            q := reply.GetResult()
            // normalize → Tick → handler(q)
        }
    }()
    return nil
}
```

### Disconnect 清理

```go
func (g *mt5Gateway) Disconnect(_ context.Context) error {
    g.running = false
    if g.streamCancel != nil { g.streamCancel() }
    return nil
}
```

## 4. CH Writer 生命周期

**问题**：`RunGateway` 返回时 `defer cancel()` 杀死 ctx → CH writer loop 退出 → 所有 tick 丢失。

**修复**：CH writer 使用 `context.Background()`：

```go
chWriter := NewCHWriter(cfg, chConn, log)
chWriter.Start(context.Background())  // 不是 ctx 参数
// 不要 defer chWriter.Close() —— writer 需进程级生命周期
```

**注意**：writer 的 `Close()` 会触发最终 `flushBatch()`。最终 flush 也应使用 `context.Background()`（不是已 cancel 的 ctx）：

```go
case <-ctx.Done():
    w.flushBatch(context.Background(), batch)
    return
case <-w.done:
    w.flushBatch(context.Background(), batch)
    return
```

## 5. GetQuote 单向调用

```go
mdCtx := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
msNotOlder := int32(0)  // 非 nil！要求新鲜数据
resp, err := mt5Client.GetQuote(mdCtx, &pb.GetQuoteRequest{
    Id: sid, Symbol: "EURUSDm", MsNotOlder: &msNotOlder,
})
if err != nil { ... }
if resp.GetError() != nil { ... }  // 必须检查 body error
quote := resp.GetResult()
```

## 6. 验证清单

- [ ] Connect 使用 metadata `id` header
- [ ] Connect 检查 `resp.GetError()` + 验证 session 非空
- [ ] SubscribeMany + OnQuote 都使用 metadata context
- [ ] OnQuote stream 使用独立的 `context.Background()` context
- [ ] `stream.Recv()` 后检查 `reply.GetError()`
- [ ] Disconnect 调用 `streamCancel()`
- [ ] CH writer 使用 `context.Background()`
- [ ] CH writer 不 defer Close（进程生命周期）
- [ ] 最终 flush 使用 `context.Background()`（非已 cancel ctx）
- [ ] `docker exec clickhouse-client -q "SELECT count() FROM md_ticks"` 返回 >0

## 7. 常见陷阱

| 症状 | 根因 | 修复 |
|------|------|------|
| "Id header is required" | gRPC 调用未带 metadata | `metadata.AppendToOutgoingContext(ctx, "id", session)` |
| "Client with id = not found" | Connect 未带 metadata，session 为空 | Connect 加 metadata + 验证非空 |
| GetQuote 返回零值 | `MsNotOlder: nil`（接受过期数据） | `MsNotOlder: &int32(0)` |
| CH 无数据但日志有 tick | `defer cancel()` 杀 writer；或 `defer Close()` 过早 | `context.Background()` + 移除 Close defer |
| MT4 连接反复重连 | `User: 0`（login 未解析） | `parseUint64(login)` |
| OnQuote 流 2 ticks 后中断 | 使用了调用者短 ctx | 独立 `context.Background()` |
