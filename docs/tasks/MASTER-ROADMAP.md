# ALFQ 项目主路线图（任务清单）

> **版本**：v1.0 · 2026-05-20
> **作者**：Cascade
> **本文取代**：`docs/tasks/QUANT-EXECUTION-PLAN.md` + `docs/tasks/DATA-INFRA-AUDIT-2026-05-20.md`（前两份保留作为细节附录）
> **核心调整**：symbol 元数据改为**从 MT 账户实时拉取**（按 broker 独立），不再用静态共享表
>
> 🤖 **AI Agent 注意**：本文是**任务清单**，不是执行入口。
> 执行入口在 `docs/tasks/AGENT-RUNBOOK.md`——它会告诉你怎么用本文、怎么提交、卡住怎么办。
> 如果你还没读过 RUNBOOK，**立即去读，不要直接看本文做事**。

---

## 1. 全景视图

```
Phase A 数据底座             Phase B Symbol 元数据         Phase C 研究 SDK
  DP1~DP4: 写 CH/聚合 bar      SM1~SM3: MT 拉 / 入 PG /     RP1~RP3: DSL parity /
  DP5~DP7: 回填 / QC / 重放    canonical 映射 / 全链路用    向量化回测 / 事件驱动
                                                                  │
                                                                  ▼
                                                            Phase D 实盘引擎
                                                            EP1~EP3: ONNX / Spec
                                                            加载 / 信号到 OMS
                                                                  │
                                                                  ▼
                                                            Phase E 生命周期
                                                            LP1~LP2: BacktestSvc /
                                                            paper→live 双签
                                                                  │
                                                                  ▼
                                                            Phase F 生产化
                                                            OP1~OP3: 可观测 / SLO /
                                                            灾备 + Runbook
```

**关键路径耗时估算**（单 Agent，全职）：
- Phase A：5–7 工作日
- Phase B：2–3 工作日
- Phase C：5–7 工作日
- Phase D：4–6 工作日
- Phase E：3–4 工作日
- Phase F：3–5 工作日

**合计约 4–6 周**到首条策略实盘小资金运行。

---

## 2. 设计变更：Symbol 元数据按经纪商动态拉取

### 2.1 背景

- 不同经纪商 symbol 命名不同：`EURUSD` / `EURUSD.m` / `EURUSDm` / `EURUSD.ecn` / `EURUSDi`
- 同 symbol 在不同经纪商：digits、tick_size、contract_size、swap、margin 都不同
- 静态共享表不能表达多经纪商现实，且难以保持同步

### 2.2 新设计

#### 2.2.1 数据来源

| MT5 RPC | 用途 | 频率 |
|---|---|---|
| `SymbolParamsMany` | 批量拿当前账户全部 symbol 完整参数 | 账户连接成功时 + 每 6h |
| `SymbolParams` | 补单个 symbol 详情 | 按需 |
| `SymbolSessionsEx` | 交易时段（quotes/trades 两套） | 每 24h |
| `ServerTimezone` | broker 服务器时区 | 每 24h |

MT4 等价 RPC：`Symbols` / `SymbolParams`（参数较少但够用）。

#### 2.2.2 PostgreSQL Schema（新增）

```sql
-- 经纪商 symbol 元数据，per (broker, symbol_raw)
CREATE TABLE IF NOT EXISTS broker_symbols (
    broker_id           UUID NOT NULL,
    symbol_raw          TEXT NOT NULL,           -- 原始名 (EURUSD.m)
    canonical           TEXT NOT NULL,           -- 规范名  (EURUSD)
    digits              SMALLINT NOT NULL,
    point               DOUBLE PRECISION,        -- 最小价格变动
    tick_size           DOUBLE PRECISION,
    tick_value          DOUBLE PRECISION,
    contract_size       DOUBLE PRECISION,
    min_lot             DOUBLE PRECISION,
    max_lot             DOUBLE PRECISION,
    lot_step            DOUBLE PRECISION,
    margin_initial      DOUBLE PRECISION,
    margin_currency     TEXT,
    profit_currency     TEXT,
    swap_long           DOUBLE PRECISION,
    swap_short          DOUBLE PRECISION,
    swap_mode           SMALLINT,
    swap_rollover_day   SMALLINT,                -- 三倍仓息日 (1=Mon..5=Fri)
    trade_mode          SMALLINT,                -- 0 disabled / 1 long_only / 2 short_only / 3 full
    description         TEXT,
    sessions_quote      JSONB,                   -- 报价时段 7 天
    sessions_trade      JSONB,                   -- 交易时段 7 天
    server_timezone     TEXT,
    raw_payload         JSONB,                   -- 原始 MT 响应，做溯源
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (broker_id, symbol_raw)
);
CREATE INDEX idx_broker_symbols_canonical ON broker_symbols(canonical);
CREATE INDEX idx_broker_symbols_updated   ON broker_symbols(updated_at);

-- canonical 映射规则（用于补丁、特殊命名）
CREATE TABLE IF NOT EXISTS symbol_canonical_overrides (
    broker_id  UUID NOT NULL,
    symbol_raw TEXT NOT NULL,
    canonical  TEXT NOT NULL,
    note       TEXT,
    PRIMARY KEY (broker_id, symbol_raw)
);
```

#### 2.2.3 Canonical 规范化算法

```go
// canonicalize converts broker-specific symbol to canonical form.
// Priority:
//   1. Lookup symbol_canonical_overrides
//   2. Strip common suffixes: .m, m, .ecn, .raw, .pro, .i, i, .stp
//   3. Uppercase
func canonicalize(raw string) string {
    raw = strings.ToUpper(raw)
    suffixes := []string{".M", "M", ".ECN", ".RAW", ".PRO", ".I", "I", ".STP", ".C"}
    for _, s := range suffixes {
        if strings.HasSuffix(raw, s) && len(raw) > len(s)+5 {
            return strings.TrimSuffix(raw, s)
        }
    }
    return raw
}
```

#### 2.2.4 对所有下游模块的影响

| 模块 | 改动 |
|---|---|
| `accountconn` | 账户连接成功后触发 `SyncSymbols(accountID)` |
| `mdgateway` | 内存维护 broker→symbols 映射；tick 入库带 raw + canonical 两列 |
| `factorsvc` | 因子表达式按 canonical 引用；engine 按 (broker, canonical) 路由 |
| `quantengine` | Spec 只声明 canonical；下单时通过 (broker_id, canonical) → symbol_raw 翻译 |
| `oms` | `OrderSend` 前从 `broker_symbols` 取 symbol_raw、min_lot、lot_step 校验 |
| ClickHouse `md_ticks` / `md_bars` | 新增列 `symbol_raw` 与 `canonical` |
| 前端 | 展示 raw + canonical 两份；账户视图按 raw，策略视图按 canonical |

#### 2.2.5 错误处理

- 同一 canonical 在不同经纪商参数差异 > 阈值 → 仅记录告警，**不阻断**（这是预期事实）
- 新接入账户首次同步失败 → 账户状态置 `symbol_sync_failed`，拒绝下单
- canonical 命中规则歧义 → 写入 `symbol_canonical_overrides` 由人工锁定

---

## 3. Phase A · 数据底座（P0，必须首先完成）

> 完成本阶段才允许进 Phase B/C。所有改动遵守：单文件 ≤ 500 行、PR 范围最小化、单测覆盖关键路径。

### DP-1 · `feat(mdgateway): real ClickHouse writer`

**目标**：把 `clickhouse_writer.go` 的 JSONL stub 替换为真正的 CH 批量写入。

**改动范围**：
- 修改 `backend/go/internal/mdgateway/clickhouse_writer.go`
- 新增 `backend/go/internal/mdgateway/clickhouse_conn.go`（连接池）
- 新增 `backend/go/migrations/clickhouse/001_md_ticks.sql`
- 新增 `backend/go/migrations/clickhouse/migrate.go`（最小迁移器，启动时跑）
- 修改 `Dockerfile.builder` 添加 `gcc`（CH 驱动 cgo 依赖）

**Schema（CH）**：
```sql
CREATE TABLE IF NOT EXISTS alfq.md_ticks (
    tenant_id        LowCardinality(String),
    broker           LowCardinality(String),
    symbol_raw       LowCardinality(String),
    canonical        LowCardinality(String),
    ts_unix_ms       UInt64,
    arrived_unix_ms  UInt64,
    bid              Decimal(18, 6),
    ask              Decimal(18, 6),
    bid_volume       Float64,
    ask_volume       Float64
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(toDate(ts_unix_ms / 1000))
ORDER BY (broker, canonical, ts_unix_ms)
TTL toDate(ts_unix_ms / 1000) + INTERVAL 90 DAY;
```

**验收**：
```bash
# 1. 启动后 CH 表自动创建
docker exec deploy-clickhouse-1 clickhouse-client -d alfq -q 'SHOW TABLES'
# 应见 md_ticks

# 2. md-gateway 跑 5 min 后有数据
docker exec deploy-clickhouse-1 clickhouse-client -d alfq -q \
  'SELECT count() FROM md_ticks'
# > 0
```

---

### SM-1 · `feat(symbolsync): MT account → broker_symbols`

> 优先于 DP-2（bar 聚合需要知道 digits / tick_size）
>
> ⚠️ **强制阅读 `docs/29-MT4-MT5-差异参考.md` 后再动手**。MT4 与 MT5 的 symbol RPC 在字段命名、嵌套层级、单位惯例上**完全不同**，绝不能用同一函数抽象。

**目标**：账户连接成功后自动拉取 symbol 元数据并入 PG。

**实现规则（不可违反）**：

1. **双 fetcher 独立实现**：`mt5_fetcher.go` 与 `mt4_fetcher.go` 各自承担字段映射，不抽象出统一 client 接口
2. **共用**：只共用落库结构体 `BrokerSymbol`、canonical 算法、repo 层
3. **数据来源对应**：
   - MT5：`MT5Client.SymbolParamsMany(limit=10000)` + `SymbolSessionsEx(每个symbol)` + `ServerTimezone`
   - MT4：`MT4Client.SymbolParamsMany()` + `ServerTimezone`（**sessions 嵌入 GroupParams.Sessions，无独立 RPC**）
4. **字段映射文档化**：每个 fetcher 顶部用注释列出 "`broker_symbols.digits ← mt5.SymbolInfoEx.Digits`"，注明 proto path
5. **降级**：核心字段（digits / point / contract_size）任一为 0 → 写 `partial=true` 列，记录到指标 `symbol_partial_total{broker,platform}`，**禁止该 symbol 进入下单白名单**

**改动范围**：
- 新增包 `backend/go/internal/symbolsync/`
  - `types.go` — `BrokerSymbol` 统一落库结构
  - `service.go` — `Sync(ctx, accountID) error` 入口（按 platform 派发）
  - `mt5_fetcher.go` — MT5 路径（扁平 `SymbolInfoEx` 字段）
  - `mt4_fetcher.go` — MT4 路径（`SymbolParams.Symbol` + `SymbolParams.GroupParams` 双路径）
  - `canonical.go` — 规范化算法 + override 表合并
  - `sessions.go` — MT4 嵌入式 sessions 与 MT5 SessionsEx 各自展平到同一 JSONB 形态
  - `repo.go` — PG upsert
  - `mt5_fetcher_test.go` / `mt4_fetcher_test.go` — 用 `testdata/*.json` 真实 fixture
- 新增 `backend/go/migrations/008_broker_symbols.sql`（含主文 §2.2.2 两张表）
- 修改 `backend/go/internal/accountconn/connector.go`：streamLoop 成功建会话后异步触发 `symbolsync.Sync(...)`
- 新增 `cmd/symbol-sync/main.go` — CLI：`./symbol-sync --account <id> --force` 手动触发

**验收**：
```bash
# 1. MT5 账户连接 30s 后
docker exec deploy-postgres-1 psql -U alfq -d alfq -c \
  "SELECT broker_id, symbol_raw, canonical, digits, contract_size
   FROM broker_symbols WHERE digits > 0 LIMIT 10"
# 至少 50+ symbols，全部字段非零

# 2. MT4 账户同测
# 字段同样非零（注意 MT4 的 contract_size 来自 GroupParams，不是 Symbol）

# 3. partial 计数
curl http://localhost:9000/metrics | grep symbol_partial_total
# 应为 0（健康）或可解释（已知坑账户）

# 4. 双平台同 broker 同 canonical 比对
psql ... -c "SELECT canonical, COUNT(DISTINCT broker_id), array_agg(DISTINCT digits)
             FROM broker_symbols WHERE canonical='EURUSD' GROUP BY canonical"
# digits 一致或差 1 以内
```

---

### DP-2 · `feat(mdgateway): bar aggregator`

**目标**：tick → 1m/5m/15m/1h/4h/1d bar，发布 `md.bar.<broker>.<canonical>.<period>` + 写 CH `md_bars`。

**改动范围**：
- 新增 `backend/go/internal/mdgateway/bar_aggregator.go`
- 新增 `backend/go/migrations/clickhouse/002_md_bars.sql`
- 新增 `backend/go/internal/mdgateway/bar_publisher.go`

**Schema**：
```sql
CREATE TABLE IF NOT EXISTS alfq.md_bars (
    tenant_id        LowCardinality(String),
    broker           LowCardinality(String),
    symbol_raw       LowCardinality(String),
    canonical        LowCardinality(String),
    period           LowCardinality(String),
    open_ts_unix_ms  UInt64,
    close_ts_unix_ms UInt64,
    open  Decimal(18, 6),
    high  Decimal(18, 6),
    low   Decimal(18, 6),
    close Decimal(18, 6),
    volume   Float64,
    tick_count UInt32
) ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(toDate(close_ts_unix_ms / 1000))
ORDER BY (broker, canonical, period, close_ts_unix_ms);
```

**验收**：
```bash
docker exec deploy-clickhouse-1 clickhouse-client -d alfq -q \
  "SELECT period, count() FROM md_bars GROUP BY period"
# 至少 1m / 5m 出现
docker logs deploy-quant-engine-1 2>&1 | grep -c "bar received"
# > 0
```

---

### DP-3 · `feat(mdgateway): auto-load accounts + tenant propagation`

**目标**：md-gateway 启动 / 周期性从 PG `accounts` 表拉取应订阅账户；tick / bar 全链路携带 `tenant_id`。

**改动范围**：
- 修改 `cmd/md-gateway/main.go` — 启动后 select accounts WHERE enabled
- 修改 `backend/go/internal/mdgateway/manager.go` — 支持动态增删 connection
- 修改 `gateway_mt5.go` / `gateway_mt4.go` — 接收 tenant_id 并塞入 Tick
- 修复 MT5 `User: 0` 的 `strconv.ParseUint` 解析

**验收**：
```bash
# 1. PG 中添加一个账户后，md-gateway 自动建立连接（30s 内）
# 2. tick 入 CH 后 tenant_id 非空
docker exec deploy-clickhouse-1 clickhouse-client -d alfq -q \
  "SELECT tenant_id, count() FROM md_ticks GROUP BY tenant_id"
# 行数 > 0 且 tenant_id 非 ''
```

---

### DP-4 · `feat(mdgateway): historical backfill CLI`

**目标**：从 MT 拉历史 bar 灌入 CH。

**改动范围**：
- 新增 `cmd/md-backfill/main.go` — CLI 入口
- 新增 `backend/go/internal/mdgateway/backfill.go` — 调 MT5 `CopyRates` / `CopyTicks`

**CLI 签名**：
```bash
./md-backfill \
  --account <uuid> \
  --symbols "EURUSD,GBPUSD,XAUUSD" \
  --periods "1m,1h,1d" \
  --from 2023-01-01 \
  --to   2025-12-31 \
  [--mode skip-existing|overwrite]
```

**验收**：
```bash
# 拉 1 年 EURUSD 1h bar
./md-backfill --account ... --symbols EURUSD --periods 1h --from 2024-01-01 --to 2025-01-01
docker exec deploy-clickhouse-1 clickhouse-client -d alfq -q \
  "SELECT count(), min(close_ts_unix_ms), max(close_ts_unix_ms)
   FROM md_bars WHERE canonical='EURUSD' AND period='1h'"
# count ≈ 6240 (24h * 260 工作日)，时间范围覆盖
```

---

### DP-5 · `feat(mdgateway): data quality checks`

**目标**：在内存中校验 tick，记录 gap / outlier / 时钟漂移指标。

**改动范围**：
- 新增 `backend/go/internal/mdgateway/quality.go`
- 新增指标：`md_gap_count{symbol}` / `md_outlier_count{symbol}` / `md_clock_skew_seconds`
- 修改 `runner.go` 把每个 tick 过滤一遍

**规则**：
1. 同 (broker,symbol) 连续 tick 间隔 > 5s → gap+1（仅交易时段内）
2. `bid > ask` → outlier 直接丢弃
3. 价格相对前 100 tick 中位数偏移 > 5σ → outlier 标记并保留（不丢弃，写入 CH 列 `outlier=1`）
4. 本地 NTP vs broker_ts 差 > 30s → 告警

**验收**：
```bash
# Grafana 指标存在
curl http://localhost:9001/metrics | grep -E 'md_gap_count|md_outlier_count'
```

---

### DP-6 · `feat(mdgateway): spill replay job`

**目标**：md-gateway 启动时自动把 SpillDir 中的 jsonl 回放进 CH，归档到 `processed/`。

**改动范围**：
- 新增 `backend/go/internal/mdgateway/spill_replay.go`
- 修改 `runner.go` 启动调度

**验收**：
```bash
# 注入 100 条 jsonl 模拟历史失败
docker exec deploy-md-gateway-1 sh -c 'cat > /tmp/alfq-ch-spill/test.jsonl <<<"..."'
docker restart deploy-md-gateway-1
sleep 30
docker exec deploy-clickhouse-1 clickhouse-client -d alfq -q "SELECT count() FROM md_ticks"
# 增加 100
docker exec deploy-md-gateway-1 ls /tmp/alfq-ch-spill/processed/ | wc -l
# > 0
```

---

### DP-7 · `feat(observability): data infra metrics + Grafana panel`

**目标**：完整可观测：tick rate / bar lag / write success / CH disk usage / reconnect count。

**改动范围**：
- 修改 `runner.go` 注册新指标
- 新增 `deploy/grafana/dashboards/data-infra.json`

**验收**：登 Grafana 看见所有 panel 有数据；任一面板触发告警条件时 alertmanager 收到。

---

## 4. Phase A 验收门禁（gating）

进 Phase B 前必须全部 ☑️：

- [ ] DP-1 完成：`md_ticks` 表存在且持续写入
- [ ] SM-1 完成：`broker_symbols` 至少含 1 个账户的 50+ symbols
- [ ] DP-2 完成：`md_bars` 各周期均有数据
- [ ] DP-3 完成：tenant_id 全链路非空；至少 2 个账户被订阅
- [ ] DP-4 完成：至少回填 1 年 1h bar
- [ ] DP-5 完成：质量指标可查
- [ ] DP-6 完成：spill 自动回放可演示
- [ ] DP-7 完成：Grafana 可观测

---

## 5. Phase B · Symbol 元数据完整化

### SM-2 · `feat(symbolsync): periodic refresh + sessions/timezone`

- 6h 周期增量刷新
- 接入 `SymbolSessionsEx` + `ServerTimezone`
- 写入 `sessions_quote` / `sessions_trade` / `server_timezone` 字段

**验收**：PG 字段非 NULL；变动时 `updated_at` 跟随。

---

### SM-3 · `feat(adminapi): SymbolService Connect RPC`

新增 RPC：
- `ListBrokerSymbols(broker_id)` → 列出该经纪商全部 symbol
- `LookupSymbol(canonical, broker_id)` → 返回 raw + 完整元数据
- `ResolveSymbol(account_id, canonical)` → 返回（symbol_raw, digits, min_lot, lot_step, ...）

**验收**：buf curl 调通；前端 Symbols 页可加载。

---

## 6. Phase C · 研究 SDK（沿用旧 PR-1~3）

### RP-1 · `feat(research): DataClient + DSL parity`

> 等价旧 PR-1，唯一调整：`DataClient.bars()` 默认按 canonical 查询；可显式指定 `broker=...` 切换。

**改动**：`research/alfq_research/data/`、`factor/dsl/`、`tests/test_dsl_parity.py`

**验收**：`uv run pytest tests/test_dsl_parity.py -v` 全过；测试用例数 ≥ 5000。

---

### RP-2 · `feat(research): vectorized backtest`

> 等价旧 PR-2

**关键扩展**：BacktestConfig 加入 `broker_id`，引擎从 PG 拉对应 (broker, canonical) 的 contract_size / lot_step / swap 等参数计算成本。

---

### RP-3 · `feat(research): event-driven backtest + consistency`

> 等价旧 PR-3

**门禁**：vectorized vs event corr ≥ 0.95；日 PnL 偏差 < 1%

---

## 7. Phase D · 实盘引擎

### EP-1 · `feat(research): trainer + ONNX exporter + spec submitter`

> 等价旧 PR-4

**Spec 调整**：策略只声明 canonical symbols；trading-core 持久化时校验所有 canonical 在目标账户的 broker 下都有 mapping。

---

### EP-2 · `feat(quant-engine): strategy spec loader + ONNX runtime`

> 等价旧 PR-5

**新增**：信号产生后由 `oms` 在下单前 `ResolveSymbol(account_id, canonical)` 翻译为 broker symbol；`min_lot` / `lot_step` 校验；不通过则 reject 信号。

---

### EP-3 · `feat(strategysvc): signal → OMS wiring + risk gates`

把 strategy-svc Runner 真正接到 oms：
- Signal → OMS `OrderSend(account_id, symbol_raw, side, lots, sl, tp)`
- 经过 risksvc 10 条规则
- 写订单状态机
- 推 SSE 更新前端

---

## 8. Phase E · 生命周期与门禁

### LP-1 · `feat(api): BacktestService + auto consistency gate`

> 等价旧 PR-6

trading-core 暴露 BacktestService；提交 spec → 调研究端 CLI → 跑 vectorized + event → 一致性达标→ strategy.status: draft→ready；不达标停留 draft 并附 diff 报告。

---

### LP-2 · `feat(strategysvc): paper → live promotion with double sign-off`

> 等价旧 PR-7

状态机：draft → ready → paper → live。
- ready→paper：研究员自助
- paper→live：tenant_admin + risk_officer 双签 + Sharpe>1.0 + paper N 个交易日无 P0/P1 风控事件

---

## 9. Phase F · 生产化

### OP-1 · `feat(observability): full SLO dashboards`

按 `docs/15-可观测性详细规范.md`：
- 行情 SLO（tick latency p99 < 50ms / gap < 0.01%）
- 订单 SLO（submit p99 < 150ms / 成功率 > 99.9%）
- 风控 SLO（kill switch < 1s）
- 数据 SLO（CH 写入成功率 > 99.99%）

---

### OP-2 · `feat(runbook): incident playbooks`

`docs/runbook/` 增加：
- 行情中断
- CH 写入失败
- 经纪商连接被踢
- 单策略熔断
- Kill Switch 触发
- Spill 满

---

### OP-3 · `feat(deploy): backup + DR drill`

- PG 每日全量 + WAL 归档到 MinIO
- CH 关键表跨机房复制（生产）
- 每月 1 次 DR 演练，结果写 `docs/dr-runs/`

---

## 10. 跨阶段公共约定

### 10.1 单一事实源

| 内容 | 唯一来源 |
|---|---|
| Symbol 名称 / 参数 | `broker_symbols` 表（由 symbolsync 维护） |
| Canonical 映射 | `canonicalize()` 函数 + `symbol_canonical_overrides` 覆盖表 |
| 因子语义 | `backend/go/internal/factor/dsl/*.go` |
| Spec 结构 | `docs/06 §4` |
| MT 会计公式 | `docs/14` |
| 错误码 | `docs/20` |
| **MT4 / MT5 差异** | **`docs/29-MT4-MT5-差异参考.md`**（凡涉及 MT 平台差异的 PR 必读） |

### 10.2 PR 守则

- 单 PR 改动 ≤ 500 行
- 函数圈复杂度 ≤ 15
- 必须含单测（关键路径覆盖 ≥ 60%）
- proto 改动跑 `buf lint` + `buf breaking`
- 跨服务改动附 ADR

### 10.3 测试基线（CI 强制）

```bash
# Go
cd backend/go && GOTOOLCHAIN=local go test ./... -race -cover

# Python
cd research && uv run pytest -v --cov=alfq_research

# DSL Parity（最关键）
cd research && uv run pytest tests/test_dsl_parity.py -v

# 一致性 gate
cd research && uv run pytest tests/test_consistency.py -v

# E2E（每个 Phase 结束跑一次）
make e2e
```

### 10.4 安全网

- Phase A 完成前禁止上线任何实盘策略
- Phase D 完成前所有 spec 仅可挂 paper 账户
- Phase E LP-2 双签未通过的策略禁止 promote 到 live
- 任何 PR 触发 Kill Switch 测试失败 → 自动 revert

---

## 11. 进度跟踪表

> **AI Agent 每次开工前先在这里找下一个 `☐` 任务**。

### Phase A 数据底座

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| DP-1 | real CH writer | ☑ | — | — |
| SM-1 | MT symbol fetcher → PG | ☑ | — | — |
| DP-2 | bar aggregator | ☑ | — | — |
| DP-3 | auto-load accounts + tenant | ☑ | — | — |
| DP-4 | backfill CLI | ☑ | — | — |
| DP-5 | data quality checks | ☑ | — | — |
| DP-6 | spill replay | ☑ | — | — |
| DP-7 | data infra metrics + Grafana | ☑ | — | — |

### Phase B Symbol 元数据

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| SM-2 | periodic refresh + sessions | ☑ | — | — |
| SM-3 | SymbolService Connect RPC | ☑ | — | — |

### Phase C 研究 SDK

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| RP-1 | DataClient + DSL parity | ☑ | — | 2026-05-20 |
| RP-2 | vectorized backtest | ☑ | — | 2026-05-20 |
| RP-3 | event-driven backtest + consistency | ☑ | — | 2026-05-20 |

### Phase D 实盘引擎

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| EP-1 | trainer + ONNX exporter + spec submitter | ☐ | — | — |
| EP-2 | strategy spec loader + ONNX runtime | ☐ | — | — |
| EP-3 | signal → OMS wiring + risk gates | ☐ | — | — |

### Phase E 生命周期

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| LP-1 | BacktestService + auto consistency gate | ☐ | — | — |
| LP-2 | paper → live double sign-off | ☐ | — | — |

### Phase F 生产化

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| OP-1 | SLO dashboards | ☐ | — | — |
| OP-2 | incident runbooks | ☐ | — | — |
| OP-3 | backup + DR drill | ☐ | — | — |

---

## 12. AI Agent 工作日志（每次完工追加一行）

| 日期 | Agent | 任务 ID | 一句话简报 |
|---|---|---|---|
| 2026-05-20 | Cascade | (设计) | 主路线图建立；symbol 元数据改为按 broker 动态拉取 |
| 2026-05-20 | DeepSeek | RP-1 | DataClient 实现真实 CH/PG 连接；DSL 补齐 12 个算子；parity test 358 tests / ~6200 assertions |
| 2026-05-20 | DeepSeek | RP-2 | vectorized backtest 引擎 + broker_sim + metrics 扩展 + pg.py broker 元数据；backtest 集成测试 18 tests |
| 2026-05-20 | DeepSeek | RP-3 | event-driven backtest + consistency gate (corr≥0.95, 日PnL MAD<1%)；6 consistency tests |

---

## 13. 下一步立即行动

**给下一个接手的 Agent**：

1. 读 `AGENT.md` + 本文件 + `docs/12-AI-Agent实施指南.md`
2. 第一个任务：**DP-1**（real CH writer）
3. 完成 DP-1 后立即接 **SM-1**（symbolsync 是 DP-2 的依赖）
4. 之后按表 §11 顺序推进
5. 每完成一个任务勾选状态、填 commit hash、追加 §12 日志
6. Phase A 全完成后写 `docs/handover/M7.0-data-infra-handover.md`

**禁止跨阶段并行**，除非：
- Phase A 完成后，Phase B / Phase C 可并行（不同人 / 不同 Agent 实例）
- Phase D 必须等 Phase B/C 全完成
- Phase E 必须等 Phase D 全完成
- Phase F 与 Phase E 后期并行

**遇到阻塞**：
1. 不要绕过验收条件
2. 在 §12 日志记录阻塞原因
3. 必要时新增 ADR 说明决策变化

---

## 14. 附录：旧文档关系

- `docs/tasks/QUANT-EXECUTION-PLAN.md` — 仍然保留作为研究端细节参考（RP-* / EP-* / LP-*）
- `docs/tasks/DATA-INFRA-AUDIT-2026-05-20.md` — 仍然保留作为数据底座问题诊断详证
- 本文是**执行清单**，那两份是**问题分析**与**详细设计**
