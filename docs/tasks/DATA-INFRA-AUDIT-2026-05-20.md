# 数据基础设施审计报告（2026-05-20）

> 审计人：Cascade · 审计范围：行情接入 / 历史数据 / 外部数据 / 质量控制
> 结论：**数据底座未达企业级量化标准；不建议立即接入 AI 编写交易策略**。
> 需要先完成下方 P0 修复，才能进入 `docs/tasks/QUANT-EXECUTION-PLAN.md` 的 PR-1。

---

## 1. 关键发现摘要

| # | 发现 | 严重度 | 证据 |
|---|---|---|---|
| F1 | **ClickHouse 里没有任何 alfq 业务表** | 🔴 P0 | `SHOW TABLES FROM alfq` 空；无 `md_ticks` / `md_bars` / `factor_values` |
| F2 | **CHWriter 是占位实现**，tick 只写 `/tmp/*.jsonl` | 🔴 P0 | `clickhouse_writer.go:42` 注释明示 stub |
| F3 | **md-gateway accounts: [] 空配置**，未真正订阅任何账户 | 🔴 P0 | `configs/md-gateway.yaml:10` |
| F4 | **无 bar 聚合器**，factor-svc 订阅的 `md.bar.>` 无人发布 | 🔴 P0 | `grep "md.bar." backend/go/internal/...` 仅有 subscribe |
| F5 | **CH spill dir 几乎为空** (4K)，证明 md-gateway 没有 tick 入站 | 🟠 P1 | `du -sh /tmp/alfq-ch-spill` |
| F6 | MT5 `Login` 字段标注 `TODO: parse string→uint64`，**实盘订阅必失败** | 🟠 P1 | `gateway_mt5.go:37` |
| F7 | **无历史回填 / 重放机制**，新策略只能从当前 tick 起算 | 🟠 P1 | 仓库内无 backfill 实现 |
| F8 | **无外部数据源**（经济日历、新闻、利率、宏观）接入 | 🟡 P2 | 仓库无相关 adapter |
| F9 | **无数据质量校验**（gap / outlier / 时钟漂移） | 🟠 P1 | mdgateway 仅做格式归一化 |
| F10 | **无对账**：tick → bar、bar → factor 一致性无校验 | 🟠 P1 | 缺失工具 |
| F11 | trading-core 与 md-gateway 的 tenant_id 是空字符串 | 🟠 P1 | `gateway_mt5.go:72` 硬编码 `TenantId: ""` |
| F12 | OHLCV 输入只有 close，OHLC 与 volume 字段被 factor engine 忽略 | 🟡 P2 | `factorsvc/engine.go:75` 仅取 close |
| F13 | 监控指标极少（仅 `md_tick_total` counter） | 🟡 P2 | 缺 latency / gap / 写入失败率 |
| F14 | 无 symbol 元数据（tick_size / contract_size / digits / swap） | 🟠 P1 | docs/14 描述但未落地 |
| F15 | 行情时间戳无 monotonic / 时区基准对齐 | 🟡 P2 | 仅用 broker time + arrived_unix_ms |

---

## 2. 详细评估

### 2.1 数据接入机制（mdgateway）

**架构（设计 vs 实际）**

| 维度 | 设计文档 | 当前实现 |
|---|---|---|
| 入口 | MT4/MT5 gRPC OnQuote stream | ✅ `gateway_mt5.go` / `gateway_mt4.go` |
| 归一化 | pb.Tick（含 broker / symbol / bid / ask / ts） | ✅ `normalizer.go` |
| 总线 | NATS JetStream `md.tick.<broker>.<symbol>` | ✅ `publisher.go` |
| 持久化 | ClickHouse `md_ticks` 表 | ❌ **stub，写 JSONL** |
| 多账户分片 | 按 broker+login 分片 | ⚠️ 容器内 `accounts: []` |
| 心跳重连 | 15s tick + 指数退避 | ✅ `runner.go:83-112` |
| 健康端点 | `/readyz` | ✅ |

**稳定性问题**
- 连接断开期间 tick 完全丢失（无离线缓冲）
- 重连只重置 backoff 但不补齐缺失数据
- 没有断线告警（仅 `zap.Warn`）
- spill 文件不轮转回写 CH，磁盘满则静默丢数据

### 2.2 历史数据拉取

| 类别 | 状态 |
|---|---|
| MT4/MT5 `OrderHistory`（账户历史成交） | ✅ 本会话已修复，事件驱动可用 |
| 账户持仓 `OpenedOrders` | ✅ 本会话已实时缓存 |
| MT 网关 `QuoteHistory` / `BarHistory`（行情历史） | ❌ 未实现 |
| ClickHouse `md_ticks` / `md_bars` 表 | ❌ 不存在 |
| 历史回填 / 重放 | ❌ 无 |
| 时间范围查询 API | ❌ trading-core 无对应 RPC |
| 跨数据源对账（mtapi vs 本地） | ❌ 无 |

**结论**：**没有任何可被回测引擎使用的历史数据**。所有 doc 10 提到的 `dc.bars(symbols, period, start, end)` 当前会返回空集合。

### 2.3 外部数据源

| 类别 | 是否需要 | 状态 |
|---|---|---|
| 经纪商行情（MT4/MT5） | 必需 | ✅ 已对接（缺持久化） |
| 经济日历（NFP / CPI / 利率决议） | 高优 | ❌ 无 |
| 新闻 / 情绪 | 中优 | ❌ 无 |
| 利率 / 国债收益率（FRED） | 中优 | ❌ 无 |
| 宏观指标（PMI / GDP） | 低优 | ❌ 无 |
| 跨市场参考（CME EUR 期货） | 中优 | ❌ 无 |

**合规性**
- MT 行情数据使用受经纪商 ToS 约束，外发 / 转售 / 跨租户共用都需法务审核（docs/22 §3 已有矩阵但未实施）
- 现有代码无数据来源标识 → 无法做合规追溯

### 2.4 数据质量控制

完全缺失。**所有维度都是 0**：

- 时钟漂移检测（broker time vs server time vs NTP）
- 跳价 / 价差异常检测
- 缺口（gap）检测与告警
- 重复 tick 去重
- bar 对齐校验（close 是否等于下一根 open）
- 异常成交量 / 闪崩过滤

### 2.5 因子计算与下游

| 维度 | 评估 |
|---|---|
| DSL 解析 / 编译 | ✅ Go 端完整（15 文件） |
| Engine.Eval 输入 | ⚠️ 仅 close，OHLC + volume 未输入 |
| 因子值持久化 | ⚠️ `FactorCHWriter` 同样为 stub（推测） |
| 多 symbol / 多 period 并行 | ❌ 无路由层 |

---

## 3. 与"企业级"标准的差距

> 企业级量化系统需具备：**数据可用**、**数据可信**、**数据可追溯**、**数据可回放**。

| 维度 | 标准 | 当前 |
|---|---|---|
| 数据可用 | tick / bar 持久化 + 99.9% 写入成功率 | 0%（未写 CH） |
| 数据可信 | gap < 0.01%，outlier 标记，对账 0 差异 | 无校验 |
| 数据可追溯 | source / vendor / version 标注，PII 治理 | 无 |
| 数据可回放 | 任意时段重放、撮合一致 | 无 |
| 故障恢复 | spill → 自动恢复入 CH | 单向写出 |
| 多租户隔离 | row policy / RLS / 来源审计 | 部分（tenant_id 未传） |

**总体评分：35 / 100**（仅完成"实时 tick 接入框架"骨架）。

---

## 4. 是否具备加入量化策略 / AI Agent 写策略的条件？

### 4.1 决断：**目前不具备**

具体后果：
1. **AI Agent 写出的回测代码会跑在空 CH 上**，所有指标都是 NaN
2. **AI Agent 写出的实盘策略只能消费当前 tick**，没有任何历史上下文（移动均线类策略全部不可用）
3. **没有数据质量护栏** → AI 把异常跳价当真实信号下单，资金风险
4. **无对账与可追溯** → 出错后无法定位是数据问题还是策略问题
5. **多租户隔离漏洞** → AI 写的策略可能污染其它租户数据

### 4.2 最低必要条件（gating items）

在让 AI 写策略前，**必须**完成的 P0 项：

- [ ] **D1**：CHWriter 真正写入 ClickHouse（`clickhouse-go/v2`）
- [ ] **D2**：建立 CH schema：`md_ticks` / `md_bars` / `factor_values` / `symbol_meta`
- [ ] **D3**：bar 聚合器 — tick → 1m/5m/15m/1h/1d bar，发布 `md.bar.<broker>.<symbol>.<period>`
- [ ] **D4**：md-gateway 默认从 PG `accounts` 表自动加载账户（而非空配置）
- [ ] **D5**：MT5 login string→uint64 修复（F6）
- [ ] **D6**：历史回填工具（CLI / RPC）：从 MT 网关拉 N 天历史 bar 写入 CH
- [ ] **D7**：tenant_id 全链路传递（F11）
- [ ] **D8**：基础数据质量校验（gap / outlier / 重复 tick）
- [ ] **D9**：Spill → CH 重放工具（断电恢复）

完成后才可启动 `QUANT-EXECUTION-PLAN.md` PR-1。

### 4.3 强烈建议（高优非阻塞）

- [ ] **D10**：经济日历接入（Forex Factory / Investing 抓取）
- [ ] **D11**：symbol_meta 表 + MT 同步任务（contract size / digits / margin）
- [ ] **D12**：行情延迟 / gap / 写入失败率 Prometheus 指标
- [ ] **D13**：Grafana 数据基础设施看板
- [ ] **D14**：对账作业（每日 close 与 MT 报表比对）

---

## 5. 修复路线图（P0 阶段）

> 7 个 PR，先于 `QUANT-EXECUTION-PLAN.md` 的 PR-1 执行。每个 PR ≤ 500 行改动。

### DP-1 · `feat(mdgateway): real ClickHouse writer with batch insert`

**文件**：
- 修改 `backend/go/internal/mdgateway/clickhouse_writer.go` → 改用 `github.com/ClickHouse/clickhouse-go/v2`
- 新增 `backend/go/internal/mdgateway/clickhouse_conn.go` — 连接池 + 重试 + 失败 spill
- 修改 `Dockerfile.builder` 添加 `gcc`（CH 驱动需要）
- 新增 `backend/go/migrations/clickhouse/001_md_ticks.sql`

**Schema**（参考 docs/02 §6）：
```sql
CREATE TABLE IF NOT EXISTS alfq.md_ticks (
    tenant_id      LowCardinality(String),
    broker         LowCardinality(String),
    symbol         LowCardinality(String),
    ts_unix_ms     UInt64,
    arrived_unix_ms UInt64,
    bid            Decimal(18, 6),
    ask            Decimal(18, 6),
    bid_volume     Float64,
    ask_volume     Float64
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(toDate(ts_unix_ms / 1000))
ORDER BY (broker, symbol, ts_unix_ms)
TTL toDate(ts_unix_ms / 1000) + INTERVAL 90 DAY;
```

**验收**：
```bash
docker exec deploy-md-gateway-1 sh -c 'sleep 60'
docker exec deploy-clickhouse-1 clickhouse-client -d alfq -q 'SELECT count() FROM md_ticks'
# > 0
```

### DP-2 · `feat(mdgateway): bar aggregator`

**文件**：
- 新增 `backend/go/internal/mdgateway/bar_aggregator.go`
- 新增 `backend/go/migrations/clickhouse/002_md_bars.sql`

**逻辑**：每个 (broker,symbol) 维护多周期窗口（1m/5m/15m/1h/1d），tick 进入 → 更新当前窗口；窗口关闭 → 发 NATS `md.bar.<broker>.<symbol>.<period>` + 写 CH

**Schema**：
```sql
CREATE TABLE IF NOT EXISTS alfq.md_bars (
    tenant_id   LowCardinality(String),
    broker      LowCardinality(String),
    symbol      LowCardinality(String),
    period      LowCardinality(String),
    open_ts_unix_ms  UInt64,
    close_ts_unix_ms UInt64,
    open  Decimal(18, 6),
    high  Decimal(18, 6),
    low   Decimal(18, 6),
    close Decimal(18, 6),
    volume Float64,
    tick_count UInt32
) ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(toDate(close_ts_unix_ms / 1000))
ORDER BY (broker, symbol, period, close_ts_unix_ms);
```

**验收**：`SELECT count() FROM md_bars WHERE period='1m'` > 0；factor-svc 日志出现 `bar received`

### DP-3 · `feat(mdgateway): auto-load accounts from PG`

**文件**：
- 修改 `cmd/md-gateway/main.go` — 启动后查 PG `accounts` 表
- 修改 `mdgateway/manager.go` — 接受运行时新增 / 移除连接

**逻辑**：和 `accountconn.Manager` 类似的做法。

**验收**：PG 创建一个 demo 账户后，md-gateway 日志出现 `broker connected`

### DP-4 · `fix(mdgateway): MT5 login parse + tenant_id propagation`

**文件**：`gateway_mt5.go`、`gateway_mt4.go`、`AccountConfig`

**修复 F6 + F11**：login `strconv.ParseUint`；tenant_id 从 AccountConfig 传到 Tick。

**验收**：MT5 OnQuote 真实回流；CH 中 `tenant_id` 字段非空。

### DP-5 · `feat(mdgateway): backfill tool`

**文件**：
- 新增 `cmd/md-backfill/main.go` — CLI：`./md-backfill --account <id> --symbol EURUSD --period 1m --from 2024-01-01 --to 2025-01-01`
- 新增 `internal/mdgateway/backfill.go` — 调 MT5 `QuoteHistory` / `BarHistory` → 写 CH

**验收**：跑一次后 CH bar 数 > 0；`SELECT min(ts), max(ts) FROM md_bars` 覆盖请求范围。

### DP-6 · `feat(mdgateway): data quality checks`

**文件**：
- 新增 `internal/mdgateway/quality.go`
- 指标：`md_gap_count`、`md_outlier_count`、`md_clock_skew_seconds`

**检查项**：
1. 同 symbol 连续 tick 间隔 > 5s → gap+1
2. bid > ask 或价格变动 > 3σ → outlier+1
3. broker_ts 与本地 NTP 差异 → clock_skew

**验收**：Grafana 出现 `md_gap_count` 指标曲线；触发条件下日志告警

### DP-7 · `feat(mdgateway): spill replay job`

**文件**：
- 新增 `internal/mdgateway/spill_replay.go` — 启动时扫描 SpillDir，将 jsonl 回放进 CH
- 修改 `runner.go` — 启动调度

**验收**：注入 100 条 jsonl 模拟历史失败 → 重启 md-gateway → CH 行数增加 100；spill 文件归档到 `processed/` 子目录

---

## 6. 完成 P0 后的额外建议（P1 / P2）

| 优先 | 任务 | 目标 |
|---|---|---|
| P1 | 经济日历采集（Forex Factory） | 事件驱动策略可用 |
| P1 | symbol_meta 同步 | 滑点 / 手续费精确化 |
| P1 | 全链路监控（Latency / Gap / 写入失败率 / 重连次数） | SLO 可观测 |
| P1 | 每日对账作业（CH vs MT report） | 0 差异目标 |
| P2 | Polars / DuckDB 本地分析层 | 研究端加速 |
| P2 | 多 broker 行情交叉验证 | 数据可信度 |
| P2 | MinIO 历史归档 | TTL 后冷存 |

---

## 7. 决策建议

### 7.1 现在该做什么

1. **暂停** `QUANT-EXECUTION-PLAN.md` 的 PR-1 执行
2. 按本文 §5 顺序完成 DP-1 ~ DP-7（约 1-2 周）
3. 上线 DP-1/DP-2 后即可启动 PR-1（research 端 DSL + 数据客户端）并行推进
4. DP-5（回填）完成后才允许跑回测
5. **不要让 AI Agent 在数据底座不稳前写实盘策略**

### 7.2 风险提示

- 若强行接 AI 写策略：模型训练用零数据 → 输出无意义 spec → 实盘下单风险
- 若 P0 修复不到位上线实盘：行情 gap → 误信号 → 资金损失
- 若 tenant_id 未补：未来切多租户时需迁移历史数据 → 不可逆

### 7.3 验收门禁（gating）

进入 `QUANT-EXECUTION-PLAN.md` PR-1 前，本文 §4.2 的 D1–D9 必须全部 ☑️。
不满足任何一项时，AI Agent 应拒绝执行策略相关 PR，并引用本报告对应 F 编号说明原因。

---

## 8. 附：进度跟踪

| PR | 标题 | 状态 | 完成时间 |
|---|---|---|---|
| DP-1 | real CH writer | ☐ | — |
| DP-2 | bar aggregator | ☐ | — |
| DP-3 | auto-load accounts | ☐ | — |
| DP-4 | login parse + tenant | ☐ | — |
| DP-5 | backfill tool | ☐ | — |
| DP-6 | quality checks | ☐ | — |
| DP-7 | spill replay | ☐ | — |

完成后写 `docs/handover/M6.6-data-infra-handover.md` 并解锁 M7。
