# 多 Broker Symbol 统一寻址设计 (Canonical-Centric)

**Status**: Draft
**Author**: AlfQ Architecture
**Date**: 2026-05-22
**Supersedes**: ad-hoc `symbol_raw` handling in `risksvc.Whitelist`, quant-engine `main.go`

---

## 1. 问题陈述

不同 MT4/MT5 broker 对同一标的使用不同 symbol 名称（如 BTC/USD: `BTCUSD`, `BTCUSDm`, `BTCUSD.x`, `BTCUSDpro`, `Bitcoin`）。导致：

- 策略 spec 写 `BTCUSD`，部署到 `BTCUSDm` 的 broker 时下单失败
- 风控白名单跨 broker 取并集（`risksvc/engine.go` `loadWhitelistFromDB` SELECT DISTINCT），错放/错拦
- 用户切换 broker 必须重写策略
- 前端无法用统一商品名展示

## 2. 设计原则

1. **策略以 canonical（逻辑名）表达，绝不持有 broker 特定的 `symbol_raw`。**
2. **`symbol_raw` 只在订单送往 broker 的最后一公里解析。**
3. **授权 (Whitelist / 风控) 以 canonical 维度执行。**
4. **解析失败立即拒单并产生审计事件，绝不静默兜底。**
5. **Canonical 字典由平台治理 (类似交易所代码)，用户不可自由创建。**

## 3. 数据模型

### 3.1 新增表

```sql
-- 平台级 canonical 字典 (治理表)
CREATE TABLE canonical_symbols (
    canonical       text PRIMARY KEY,                 -- e.g. 'BTCUSD'
    asset_class     text NOT NULL,                    -- 'forex'|'crypto'|'metal'|'index'|'stock'|'commodity'
    base_ccy        text NOT NULL,
    quote_ccy       text NOT NULL,
    description     text NOT NULL,
    enabled         boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- 租户级 canonical 白名单 (合规边界)
CREATE TABLE tenant_canonical_whitelist (
    tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    canonical   text NOT NULL REFERENCES canonical_symbols(canonical),
    enabled     boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, canonical)
);
ALTER TABLE tenant_canonical_whitelist ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant_canonical_whitelist
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- 策略级 canonical 白名单 (用户编辑)
CREATE TABLE strategy_symbols (
    strategy_id uuid NOT NULL REFERENCES strategies(id) ON DELETE CASCADE,
    canonical   text NOT NULL REFERENCES canonical_symbols(canonical),
    enabled     boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (strategy_id, canonical)
);
ALTER TABLE strategy_symbols ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON strategy_symbols
    USING (EXISTS (
        SELECT 1 FROM strategies s
        WHERE s.id = strategy_symbols.strategy_id
        AND s.tenant_id = current_setting('app.tenant_id', true)::uuid
    ));

-- NOTIFY trigger for hot reload
CREATE OR REPLACE FUNCTION notify_strategy_symbols() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('strategy_symbols', NEW.strategy_id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_strategy_symbols_notify
    AFTER INSERT OR UPDATE OR DELETE ON strategy_symbols
    FOR EACH ROW EXECUTE FUNCTION notify_strategy_symbols();
```

### 3.2 复用既有表

| 表 | 用途 | 改动 |
|---|---|---|
| `broker_symbols` | `(broker_id, symbol_raw) → canonical` 映射 | 加外键 `canonical REFERENCES canonical_symbols`；symbol-sync 必须填 canonical |
| `accounts` | `account_id → broker_id` | 无改动 |
| `strategies` | 策略主表 | 移除 `spec.canonical_symbols` (改由 `strategy_symbols` 表存) |

### 3.3 关系图

```
canonical_symbols (字典)
        ▲
        │ FK
        ├─── tenant_canonical_whitelist (admin 配置)
        ├─── strategy_symbols           (用户配置)
        └─── broker_symbols.canonical   (symbol-sync 归一化)

strategies ──── strategy_symbols
    │
    └──(策略运行时绑定的 account)──→ accounts ──→ brokers
                                          │
                                          ▼
                                    broker_symbols
```

## 4. 三层授权链 (订单提交前)

```
1. 策略产生信号 canonical = "BTCUSD"
2. [Gate-1] strategy_symbols 是否包含该 canonical？           否 → REJECTED reason=not_in_strategy_whitelist
3. [Gate-2] tenant_canonical_whitelist 是否包含？              否 → REJECTED reason=tenant_not_authorized
4. [Gate-3] broker_symbols(account.broker_id, canonical) 存在 否 → REJECTED reason=symbol_not_on_broker
            且 trade_mode > 0?                                 否 → REJECTED reason=symbol_disabled_on_broker
5. 解析得 symbol_raw → mtapi.OrderSend(symbol=symbol_raw)
```

每次拒绝写一条 `risk_events` 审计行（已有表）。

## 5. 后端实现拆解

### 5.1 风控规则替换
- **删除**：`risksvc.Whitelist`（全局 map 数据结构）和 `tradingcore.runner.go` `loadWhitelistFromDB`
- **新增**：`risksvc.CanonicalAuth` rule
  ```go
  type CanonicalAuth struct {
      pool *pg.Pool
      cache *sync.Map // (strategyID, canonical) → bool, refreshed by NOTIFY
  }
  func (r *CanonicalAuth) Check(ctx, req, state) *RiskCheckResult
  ```
- **数据流**：监听 `strategy_symbols` 和 `tenant_canonical_whitelist` 的 NOTIFY，按需失效缓存；命中走缓存

### 5.2 OrderRequest 协议
`pb.OrderRequest.Symbol` 字段语义改为 **canonical**。新增 `BrokerSymbolRaw string` 字段（OMS 内部填写、broker adapter 使用）。废弃 quant-engine `main.go` 内的解析逻辑。

### 5.3 OMS executor 解析下沉
```go
func (e *OrderExecutor) Submit(ctx, req) {
    // ... Gate 1/2/3 通过后
    info, ok, err := e.symbolResolver.ResolveCanonical(ctx, req.AccountId, req.Symbol)
    if !ok { return REJECTED reason=err }
    req.BrokerSymbolRaw = info.SymbolRaw
    return e.adapter.Submit(ctx, req)
}
```

### 5.4 symbol-sync 归一化
- 拉到 `symbol_raw` 后，按规则去后缀映射 canonical：
  - `BTCUSDm` → `BTCUSD`
  - `BTCUSD.x`, `BTCUSDpro`, `BTCUSD#`, `BTCUSD!` → `BTCUSD`
  - 无法匹配字典 → `partial=true`, `canonical=NULL`，留管理后台人工映射
- 字典查表 + 规则引擎，新规则在 `internal/symbolsync/canonical_mapper.go`

## 6. ConnectRPC API

### 6.1 Admin
```proto
service AdminCanonicalService {
    rpc ListCanonicalSymbols(Empty) returns (ListResp);
    rpc CreateCanonicalSymbol(CanonicalSymbol) returns (Empty);
    rpc EnableCanonical(CanonicalKey) returns (Empty);
    rpc UpdateTenantWhitelist(TenantWhitelistReq) returns (Empty);
}
```

### 6.2 用户 / 策略
```proto
service StrategyService {
    // 列出当前租户白名单内的 canonical (供策略编辑器多选)
    rpc ListAvailableCanonicals(Empty) returns (ListResp);
    // 列出某账户对一组 canonical 的可交易映射 (供策略详情展示)
    rpc ResolveCanonicalsForAccount(ResolveReq) returns (ResolveResp);
    // 设置策略的 canonical 白名单
    rpc UpdateStrategySymbols(UpdateSymbolsReq) returns (Empty);
}
```

## 7. 前端 UX

### 策略编辑页
1. **Symbol 多选器**：调 `ListAvailableCanonicals`，显示 `BTCUSD — Bitcoin / USD (crypto)` 等
2. 保存 → `UpdateStrategySymbols(strategy_id, [canonicals])`
3. **不需要先选账号**

### 策略详情页
- 上方显示 canonical 列表
- 旁边展开 "在当前绑定账户上的实际名称":
  ```
  BTCUSD  →  account A (MT5-XYZ): BTCUSDm  ✔ tradable
             account B (MT5-ABC): BTCUSD.x ✔ tradable
             account C (MT5-DEF): —        ✖ not available
  ```
- 数据来源 `ResolveCanonicalsForAccount`

### 管理后台 (Admin)
- canonical 字典 CRUD
- 租户白名单管理
- broker_symbols 中 `partial=true` 的待映射列表，人工指定 canonical

## 8. 迁移路径

| 阶段 | 内容 | 验收 |
|---|---|---|
| M1 | migration: 建 3 张表 + seed canonical 字典 (~50 个主流商品) | psql 建表成功 |
| M2 | symbol-sync 归一化规则 + 回填 `broker_symbols.canonical` | 90%+ symbol_raw 自动归一 |
| M3 | OrderRequest proto 加 `BrokerSymbolRaw`；OMS Resolve 下沉；quant-engine main.go 解析逻辑删除 | 端到端跑通 |
| M4 | `risksvc.CanonicalAuth` rule 上线，下线 `Whitelist` + `loadWhitelistFromDB` | 拒单原因清晰 |
| M5 | Admin & Strategy ConnectRPC + 前端编辑器 | 用户可编辑 |
| M6 | NOTIFY hot-reload；老的 `spec.canonical_symbols` 字段迁移到 `strategy_symbols` 表 | 修改即时生效 |

## 9. 回滚

每阶段独立可回滚：
- M1-2 纯加表加列，回滚 drop
- M3 protobuf 字段保留向后兼容
- M4 保留旧 `Whitelist` 一个版本，feature flag `ALFQ_USE_CANONICAL_AUTH=true` 切换
- M5-6 前端独立部署

## 10. 与现有代码的对照

| 现状 | 目标 | 改动文件 |
|---|---|---|
| `risksvc.Whitelist` 全局 map | `CanonicalAuth` 按 (strategy, tenant) | `internal/risksvc/canonical_auth.go` (new) |
| `tradingcore/runner.go:loadWhitelistFromDB` 跨 broker 并集 | 删除 | `internal/tradingcore/runner.go` |
| `quant-engine/main.go` 解析 symbol_raw | 解析下沉到 OMS | `cmd/quant-engine/main.go`, `internal/oms/executor.go` |
| `spec.CanonicalSymbols` (yaml) | `strategy_symbols` 表 | `internal/strategysvc/spec/spec.go`, migrations |
| 前端无 symbol 编辑器 | canonical 多选 + 映射查看 | `frontend/src/pages/strategy/edit.tsx` (new) |

## 11. 风险与开放问题

| 风险 | 缓解 |
|---|---|
| 字典治理工作量 (谁维护 canonical 列表？) | 平台运维角色 + Admin UI；首期手工 seed 50-100 个 |
| 归一化规则误判 (如 `EURUSDc` 是不同合约不是仓位别名) | `partial=true` 兜底 + 人工 review |
| 老策略 spec 持有 broker-specific 名 | M3 上线时做数据迁移脚本，反查 `broker_symbols` 回溯 canonical |
| broker 改名/下架 | symbol-sync 周期任务发现差异 → 写 alert，订单 Gate-3 兜底 |

## 12. 接受标准 (Definition of Done)

- [ ] 同一策略绑定不同 broker 账号无需修改即可正常下单
- [ ] 用户能在前端选择 canonical（不接触 broker 后缀）
- [ ] 不在策略白名单的 canonical 信号被拒单，审计事件包含 reason
- [ ] broker 不支持的 canonical 信号被拒单，审计事件包含 `symbol_not_on_broker`
- [ ] `risksvc.Whitelist` 代码路径删除（grep 全工程为零）
- [ ] 单元测试覆盖三道 Gate + 解析失败分支
- [ ] 端到端：BTCUSD 信号 → broker MT5 实际下单 ticket 落 `orders.broker_ticket`
