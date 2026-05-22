# MD-Gateway 行情流阻塞修复执行文档

> **日期**：2026-05-21  
> **优先级**：P0（阻塞量化交易闭环）  
> **前置阻塞**：当前 CH `md_ticks=0 / md_bars=0`，mtapi OnQuote 流未产生任何 tick  
> **预计耗时**：1–2 小时（单 PR）  
> **执行者**：AI Agent（DeepSeek / Claude / 其他）  
> **本文形态**：可分步执行的 PR 任务清单，每个 PR 自包含验收命令

---

## 1. 问题根因

### 1.1 现象

- **容器状态**：14/14 容器 healthy，md-gateway 已连接 2 个 MT4 broker（d6ad41cd / abd5a77d）
- **MT 连接正常**：`mthub OpenedOrders` 每 ~30s 正常返回持仓（证明 MT gRPC 长连接可用）
- **行情流断**：CH `alfq.md_ticks=0` / `alfq.md_bars=0`，md-gateway 日志无 `OnQuote` 相关输出
- **订阅异常**：md-gateway 启动日志显示 `subscribing to symbols count=3 sample=["EURUSD","EURUSDm","EURUSD."]`（3 个不同经纪商命名硬编码混在一起）

### 1.2 根因（双 Bug 叠加）

#### Bug B1：MT4 fetcher 把所有 `trade_mode` 写成 0

**位置**：`@/opt/alfq/backend/go/internal/symbolsync/convert.go:61-95`（MT4 路径）

MT4 的 `SymbolParams.GroupParams` 不像 MT5 有可靠的 `TradeMode` 字段（MT5 有 `SYMBOL_TRADE_MODE_FULL=4` 等枚举），MT4 mtapi 返回的 `TradeMode` 通常为 0 或不可信。当前 `ConvertMT4Symbol` **完全不赋值 `TradeMode`**，导致入库时该字段默认为 0。

**PG 实测**：
```sql
SELECT trade_mode, partial, count(*) FROM broker_symbols GROUP BY trade_mode, partial;
-- trade_mode | partial | count
-- -----------+---------+------
--          0 | f       |  661
--          0 | t       |   14
```

#### Bug B2：`loadBrokerSymbols` 过滤 `trade_mode=3` 命中 0 条 + 硬编码 fallback 把多 broker 别名混发

**位置**：`@/opt/alfq/backend/go/internal/mdgateway/runner.go:298-325`

```go
func loadBrokerSymbols(pool *pgxpool.Pool, isMT4 bool) []string {
    // ...
    rows, err := pool.Query(ctx,
        `SELECT symbol_raw FROM broker_symbols
         WHERE trade_mode = 3 AND partial = false
         ORDER BY symbol_raw LIMIT 100`)
    // ...
    if len(symbols) == 0 {
        return nil  // 实际走到下面 fallback
    }
}
```

但下游 `runner.go:213-225`：
```go
symbols := loadBrokerSymbols(d.PG.Pool, gw.Platform() == "mt4")
if len(symbols) == 0 {
    symbols = []string{"EURUSD", "EURUSDm", "EURUSD."} // fallback
}
```

**问题**：
1. 过滤 `trade_mode=3` 在 MT4 环境命中 0 条 → 走 fallback
2. fallback 硬编码 3 个不同经纪商别名（EURUSD / EURUSDm / EURUSD.），但当前 broker 可能只持有其中 1 个
3. `SubscribeMany` 把这 3 个别名发给 broker，broker 对不存在的 symbol 静默丢弃 → OnQuote 无下游 → tick=0

### 1.3 为什么历史 ACCEPTANCE-REPORT 显示 66,154 ticks

该报告（2026-05-21）基于另一份环境快照，当时 `broker_symbols` 可能有部分 `trade_mode=3`，或测试环境手动改过。**当前容器是冷启动**，CH 库 `alfq.md_ticks=0` 且 md-gateway 启动时刚跑过 `chmigrate: applied 001_md_ticks.sql / 002_md_bars.sql`，证明这是新建/清空过的库。

---

## 2. 最小修复方案（单 PR）

### 2.1 修复 B1：MT4 fetcher 强制 `TradeMode=4`（视为 full）

**改动**：`@/opt/alfq/backend/go/internal/symbolsync/convert.go`

```go
// ConvertMT4Symbol converts a single MT4 SymbolParams to BrokerSymbol.
// sessions come from SymbolInfoEx.Sessions (may be nil).
func ConvertMT4Symbol(sp *mt4pb.SymbolParams, brokerID string, sessions []*mt4pb.ConSessions, tz *mt4pb.ServerTimezoneReply) BrokerSymbol {
    raw := sp.GetSymbolName()
    canon := Canonicalize(raw)
    sym := BrokerSymbol{BrokerID: brokerID, SymbolRaw: raw, Canonical: canon}

    if info := sp.GetSymbol(); info != nil {
        sym.Digits = int16(info.GetDigits())
        sym.Point = info.GetPoint()
        sym.ContractSize = info.GetContractSize()
        sym.SwapLong = info.GetSwapLong()
        sym.SwapShort = info.GetSwapShort()
    }
    if gp := sp.GetGroupParams(); gp != nil {
        sym.MinLot = gp.GetMinLot()
        sym.MaxLot = gp.GetMaxLot()
        sym.LotStep = gp.GetLotStep()
    }

    // MT4 GroupParams.TradeMode is unreliable; assume full trading for valid symbols.
    // MT5 uses proper enum (SYMBOL_TRADE_MODE_FULL=4), but MT4 mtapi returns 0.
    if sym.Digits > 0 && sym.Point > 0 && sym.ContractSize > 0 {
        sym.TradeMode = 4  // SYMBOL_TRADE_MODE_FULL (per MT5 enum)
    }

    if sym.Digits == 0 || sym.Point == 0 || sym.ContractSize == 0 {
        sym.Partial = true
    }

    // Sessions from SymbolInfoEx (one ConSessions per day, each with Quote/Trade sessions)
    sym.SessionsQuote = sessionsMT4ToJSON(sessions, true)
    sym.SessionsTrade = sessionsMT4ToJSON(sessions, false)

    // Timezone
    if tz != nil {
        sym.ServerTimezone = fmt.Sprintf("%+d", tz.GetResult()/3600)
    }

    return sym
}
```

**说明**：
- MT4 fetcher 不再依赖 `GroupParams.TradeMode`（不可信）
- 对 `digits>0 && point>0 && contract_size>0` 的 symbol 强制设为 `TradeMode=4`（full）
- `partial` 判定逻辑保留（不完整数据仍标记为 partial）

### 2.2 修复 B2：`loadBrokerSymbols` 按 broker 分离加载 + 删除硬编码 fallback

**改动**：`@/opt/alfq/backend/go/internal/mdgateway/runner.go`

```go
// loadBrokerSymbols loads symbol_raw names from broker_symbols per broker.
// For MT4 the mt4Gateway ignores the symbol list anyway (OnQuote is global),
// but we return the correct names for logging and future MT5 use.
func loadBrokerSymbols(pool *pgxpool.Pool, brokerID string) []string {
    if pool == nil {
        return nil
    }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    rows, err := pool.Query(ctx,
        `SELECT symbol_raw FROM broker_symbols
         WHERE broker_id = $1 AND partial = false AND digits > 0
         ORDER BY symbol_raw LIMIT 200`,
        brokerID,
    )
    if err != nil {
        return nil
    }
    defer rows.Close()

    var symbols []string
    for rows.Next() {
        var s string
        if err := rows.Scan(&s); err == nil {
            symbols = append(symbols, s)
        }
    }
    return symbols
}
```

**调用点修改**（`runner.go:211-225`）：
```go
// Load broker-specific symbol names from broker_symbols;
// each broker uses its own naming (e.g. EURUSDm vs EURUSD).
brokerID, _ := extractBrokerID(key)  // key format: "brokerID-login"
symbols := loadBrokerSymbols(d.PG.Pool, brokerID)
if len(symbols) == 0 {
    d.Log.Error("no tradable symbols found for broker, skipping subscription",
        zap.String("broker_id", brokerID),
        zap.String("key", key),
    )
    continue  // skip this broker instead of fallback
}
d.Log.Info("subscribing to symbols",
    zap.String("key", key),
    zap.Int("count", len(symbols)),
    zap.Strings("sample", firstN(symbols, 5)),
)
go func(key string, gw Gateway) {
    _ = gw.Subscribe(ctx, symbols, func(tick *pb.Tick) { handler(key, gw, tick) })
}(key, gw)
```

**新增辅助函数**（`runner.go` 末尾）：
```go
// extractBrokerID parses "brokerID-login" or returns the full key.
func extractBrokerID(key string) (string, string) {
    for i := len(key) - 1; i >= 0; i-- {
        if key[i] == '-' {
            return key[:i], key[i+1:]
        }
    }
    return key, key
}
```

### 2.3 数据修复：重新同步 broker_symbols

修复代码后，需要手动触发 symbol sync 刷新现有数据（否则 `trade_mode` 仍为 0）。

**命令**：
```bash
# 方式 1：重启 md-gateway（触发 periodic refresh，6h 周期）
docker compose restart md-gateway

# 方式 2：手动触发 symbol sync（推荐，立即生效）
docker exec deploy-md-gateway-1 /app/md-gateway --sync-symbols-once
# 或通过 RPC 调用（需实现 TriggerSymbolSync，若无则用方式 1）
```

---

## 3. 验收命令

### 3.1 单元测试

```bash
cd backend/go
go test ./internal/symbolsync/... -v -run TestConvertMT4Symbol
# 验证 TradeMode=4 且 partial 判定正确

go test ./internal/mdgateway/... -v -run TestLoadBrokerSymbols
# 验证按 broker 分离加载，空集返回 nil（不 fallback）
```

### 3.2 编译验证

```bash
make go-build SVC=md-gateway
make go-build SVC=trading-core
# 全 PASS
```

### 3.3 数据修复验证

```bash
# 重启 md-gateway
docker compose restart md-gateway
sleep 30

# 验证 broker_symbols trade_mode 更新
docker exec deploy-postgres-1 psql -U alfq -d alfq -c \
  "SELECT trade_mode, partial, count(*) FROM broker_symbols GROUP BY trade_mode, partial;"
# 期望：trade_mode=4 且 partial=f 的行数 > 0
```

### 3.4 行情流验证（核心验收）

```bash
# 等待 5 分钟
sleep 300

# 查询 CH tick/bar 数据
docker exec deploy-clickhouse-1 clickhouse-client -q \
  "SELECT count() FROM alfq.md_ticks"
# 期望：> 0（正常市场时段应 > 1000）

docker exec deploy-clickhouse-1 clickhouse-client -q \
  "SELECT count() FROM alfq.md_bars"
# 期望：> 0（1m/5m/15m/30m/1h 任一周期有数据）
```

### 3.5 日志验证

```bash
docker logs deploy-md-gateway-1 2>&1 | grep -iE 'subscribing to symbols|no tradable symbols'
# 期望：每个 broker 显示 "subscribing to symbols count=N" 且 N > 0
# 不应出现 "no tradable symbols found"
```

---

## 4. 回滚预案

### 4.1 Git 回滚

修复前打 tag：
```bash
git tag -a pre-md-quote-fix -m "Before md-gateway quote stream fix"
```

若修复后出现严重问题（如 md-gateway 无法启动、CH 写入失败）：
```bash
git revert <commit-hash>
docker compose restart md-gateway
```

### 4.2 数据回滚

若 `broker_symbols` 被错误更新（如 trade_mode 全部变成 4 但实际不可交易），可手动回退：
```sql
-- 备份当前状态
CREATE TABLE broker_symbols_backup_20260521 AS SELECT * FROM broker_symbols;

-- 若需回退，删除错误数据，等待下次 periodic refresh
DELETE FROM broker_symbols WHERE broker_id = '<affected-broker-id>';
```

---

## 5. 依赖与约束

### 5.1 遵守 ADR-0010

- md-gateway 是唯一持有 MT 长连接的服务（本次修复不改变）
- trading-core 通过 mthub 借用 session（本次修复不涉及 mthub）

### 5.2 遵守 MASTER-ROADMAP §10.2

- 单 PR ≤ 500 行（本次约 50 行）
- 圈复杂度 ≤ 15（本次逻辑简单）
- 关键路径单测覆盖 ≥ 60%（需补充 `TestConvertMT4Symbol` / `TestLoadBrokerSymbols`）

### 5.3 遵守 docs/12 §3.5 复杂度上限

- 单文件行数 ≤ 300（`convert.go` 当前 128 行，`runner.go` 当前 342 行，需拆分或保持改动最小）

---

## 6. 执行顺序

1. **修改代码**（B1 + B2）
2. **补充单测**（`convert_test.go` + `runner_test.go`）
3. **编译验证**（`make go-build SVC=md-gateway`）
4. **提交 PR**（description 引用本文档）
5. **合并后重启 md-gateway**（触发数据修复）
6. **跑验收命令**（§3 全部 PASS）
7. **更新 MASTER-ROADMAP §11**（新增阻塞项 `MD-1: md-gateway quote stream fix`，状态改为 ☑）

---

## 7. 关联文档

- `docs/tasks/ACCEPTANCE-REPORT-2026-05-21.md`（历史 66,154 ticks 快照）
- `docs/tasks/MASTER-ROADMAP.md`（主路线图，本文作为阻塞项前置）
- `docs/29-MT4-MT5-差异参考.md`（MT4/MT5 字段差异参考）
- `docs/enhancements/2026-05-21-final-optimization-plan.md`（Stage 1/2/3 已完成，本文不依赖）

---

*本文档由 Cascade 创建（2026-05-21），供后续 AI Agent 落地执行。*
