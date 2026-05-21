# M7 量化策略落地执行计划（AI Agent Runbook）

> **日期**：2026-05-20
> **作者**：Cascade（基于代码审计 + 文档对账）
> **目标读者**：后续负责实现量化策略闭环的 AI Agent
> **本文形态**：可分步执行的 PR 任务清单，每个 PR 自包含验收命令
>
> 阅读前提：已读完 `AGENT.md`、`docs/01-总体架构与技术决策.md`、`docs/06-Python策略沙箱设计.md`、`docs/09-因子DSL规范.md`、`docs/10-Python研究层实现规范.md`、`docs/14-领域模型与交易规则.md`。

---

## A. 现有文档评估

### A.1 与量化直接相关的文档

| 路径 | 字节 | 核心内容 | 完整度 | 当前是否被代码满足 |
|---|---|---|---|---|
| `docs/企业级量化交易系统落地方案.md` | 26489 | 总览：架构 / 里程碑 / 风控 / SLO / 排期 | ★★★★★ | 文档层 100%，代码层 ~40% |
| `docs/06-Python策略沙箱设计.md` | 7991 | DSL+ONNX 策略模型、沙箱形态、上线流程 | ★★★★★ | ~10%（沙箱未起，ONNX 未集成） |
| `docs/09-因子DSL规范.md` | 6426 | DSL EBNF、算子表、Go/Py 对齐 | ★★★★★ | Go 端 ~90%，Py 端 ~5% |
| `docs/10-Python研究层实现规范.md` | 7447 | `alfq_research` 包结构、回测引擎、CLI | ★★★★★ | ~10%（仅骨架 + metrics.py） |
| `docs/14-领域模型与交易规则.md` | 16860 | MT 会计口径、滑点 / 手续费 / swap 公式 | ★★★★★ | 公式层未编码 |
| `docs/18-AI-Agent工作流深化与策略助手.md` | 18307 | 研发 Agent 防漂移、策略助手产品、ML 治理 | ★★★★★ | 未编码 |
| `docs/16-测试与质量保证.md` | 10495 | 测试金字塔、Parity、回测基线 | ★★★★ | 部分（DSL 单测） |

### A.2 已交付的里程碑（来自 `docs/handover/`）

```
M0  Monorepo + proto + CI + Docker         ✅
M1  md-gateway + ClickHouse + NATS         ✅
M2  DSL (Go+Py) + factor-svc               ✅（Go 完整、Py 仅骨架）
M3  state machine + risk + strategy        ✅（最小可用）
M4  10 rules + Kill Switch                 ✅
M5  Capital alloc + hot reload             ✅
M6  Feature flags + Runbook                ✅
M6.5 Cloud provider abstraction            ✅
此外（2026-05-20，本会话）账户实时数据流：持仓秒开、历史订单事件驱动、JWT 持久化 ✅
```

### A.3 结论

**文档侧无需新增设计**。所有问题域（DSL、回测、风控、ONNX、部署、SLA、合规）已有详细规范。
**代码侧存在显著缺口**：量化业务的"研究→回测→上线→实盘"主流程没有真正贯通。本计划聚焦弥合该缺口。

---

## B. 实现差距矩阵

| 模块 | 当前文件 | 期望（按 docs） | 缺口 |
|---|---|---|---|
| Go DSL 解释器 | `backend/go/internal/factor/dsl/`（15 文件，齐全） | 09 章全部算子 | 算子已覆盖 v1，**OK** |
| Go factor-svc | `internal/factorsvc/` 4 文件 | 订阅 bar → 写 CH | 已可用 |
| Go strategy-svc | `runner.go` 57 行 + `loader.go` 94 行 | Spec 解析 / 信号生成 / 下单 | **Spec 解析缺失、信号未接入 OMS** |
| Go quant-engine | `runner.go` 114 行 demo | 加载策略 Spec、ONNX 推理、订阅因子 | **绝大部分缺失** |
| Go ONNX 推理 | 无 | `onnxruntime-go` 集成 | **完全缺失** |
| Go risk-svc | `internal/risksvc/` 7 文件 | 10 条规则 + Kill Switch | 已可用 |
| Go OMS | `internal/oms/` 7 文件 | 下单 / 撤单 / 状态机 | 基础可用 |
| Python `alfq_research` | data/factor/backtest/model 仅骨架 | 完整 SDK（10 章） | **大面积缺失** |
| Python DSL parser | 缺失 | 与 Go 对齐 | **完全缺失** |
| Python 向量化回测 | 缺失 | Polars 实现 | **完全缺失** |
| Python 事件驱动回测 | 缺失 | broker_sim + 撮合 | **完全缺失** |
| Python ONNX 导出 | 缺失 | LightGBM → ONNX → MinIO | **完全缺失** |
| BacktestService Connect API | 缺失 | proto + handler | **完全缺失** |
| 一致性校验 CI gate | 缺失 | vectorized vs event ≥ 0.95 corr | **完全缺失** |

---

## C. 执行路线图（M7 → 首条实盘策略）

> 路线图分 7 个 PR，每个 PR 独立可合并、可回滚。Agent 一次只做一个 PR。
> 完成顺序严格按下方编号；如某 PR 阻塞，更新本文档说明并停车。

### PR-1 · `feat(research): data client + DSL parser parity with Go`

**目标**：让 Python 端可以读 ClickHouse bar 数据并解析 DSL；与 Go 端结果对齐（误差 < 1e-9）。

**新增 / 修改文件**：

```
research/alfq_research/data/
    ch.py              # ClickHouseClient (clickhouse-driver / polars)
    pg.py              # 元数据查询
    client.py          # DataClient 统一入口（已有，扩展）
research/alfq_research/factor/dsl/
    __init__.py
    lexer.py           # 与 backend/go/internal/factor/dsl/lex.go 对齐
    parser.py          # 与 parser.go 对齐
    ops/
        __init__.py
        moving_average.py
        statistics.py
        oscillators.py
        ref_delta.py
        scalar.py
        corr_cov.py
        bb_cross.py
    eval.py            # 批量执行
research/tests/
    test_dsl_parity.py # 与 Go 端逐 op 对比（5000+ 用例）
```

**验收**：
```bash
# 在 research/ 下
uv run pytest tests/test_dsl_parity.py -v
# 全部用例通过，误差打印 < 1e-9
```

**完成标志**：在 `docs/handover/M7-handover.md` 增加 "PR-1 完成" 段落。

### PR-2 · `feat(research): vectorized backtest engine`

**目标**：实现向量化回测，秒级出 Sharpe / MaxDD / 年化等指标。

**新增**：

```
research/alfq_research/backtest/
    vectorized.py      # 基于 Polars 的向量化引擎
    broker_sim.py      # 滑点 / 手续费 / swap（参照 docs/14 §3）
    metrics.py         # 扩展现有 metrics.py：Sortino/Calmar/胜率/盈亏比
    runner.py          # BacktestRunner 高层 API
    report/
        html.py        # quantstats 风格 HTML
research/tests/
    test_backtest_vectorized.py
    test_metrics.py
```

**输入合约**：
```python
@dataclass
class BacktestConfig:
    symbols: list[str]
    period: str          # "1m" | "5m" | "1h" | "1d"
    start: str           # ISO date
    end: str
    initial_capital: float
    factors: dict[str, str]    # name -> DSL expression
    signal_rule: dict[str, Any]   # {"type": "dsl_rule", "expr": "..."}
    sizing: dict[str, Any]
    fees: dict[str, float]
```

**输出合约**：
```python
@dataclass
class BacktestResult:
    pnl_series: pl.DataFrame  # cols: ts, pnl, equity
    trades: pl.DataFrame      # cols: open_ts, close_ts, symbol, side, lots, profit
    metrics: dict[str, float] # sharpe, sortino, max_dd, calmar, ...
    html_report_path: str
```

**验收**：
```bash
uv run pytest tests/test_backtest_vectorized.py -v
# 跑一个 SMA 交叉策略，1 年 EURUSD 1h 数据：Sharpe / MaxDD 在合理区间
```

### PR-3 · `feat(research): event-driven backtest + consistency gate`

**目标**：事件驱动回测，与 PR-2 结果 corr > 0.95。

**新增**：

```
research/alfq_research/backtest/
    event.py           # 事件驱动撮合（参考 nkaz001/hftbacktest 简化版）
    consistency.py     # vectorized vs event 一致性校验
research/tests/
    test_backtest_event.py
    test_consistency.py
```

**关键算法**：bar-close 撮合，开盘价滑点 ±0.5 × spread，限价单到达模型用 `min(low, price) <= price <= max(high, price)`。

**验收**：
```bash
uv run pytest tests/test_consistency.py -v
# 同一 spec：vectorized.daily_pnl 与 event.daily_pnl 相关系数 ≥ 0.95，日 PnL 偏差 < 1%
```

### PR-4 · `feat(research): model trainer + ONNX exporter + spec submitter`

**目标**：研究员在 Notebook 一行代码训练 LightGBM、导 ONNX、传 MinIO、提交策略 Spec。

**新增**：

```
research/alfq_research/model/
    trainer.py         # LightGBM/sklearn 封装
    exporter.py        # to_onnx + upload to MinIO
research/alfq_research/spec/
    strategy_spec.py   # StrategySpec，调 Connect RPC 提交
research/alfq_research/client/
    connect_client.py  # ConnectRPC client（用 connectrpc/python）
    auth.py            # JWT 注入
research/tests/
    test_onnx_roundtrip.py    # train → export → re-load → predict 一致
```

**Spec 结构遵循 docs/06 §4**。提交后 trading-core 持久化到 `strategies` 表（status=draft）。

**验收**：
```bash
# Notebook 内
from alfq_research import ModelExporter, StrategySpec
ModelExporter.to_onnx(lgbm_model, "/tmp/m.onnx")
ModelExporter.upload("/tmp/m.onnx", "s3://alfq/.../m.onnx")
spec = StrategySpec(name="momentum_v1", ...)
spec.submit()    # 返回 strategy_id
# 在 PG 中能查到 strategy_id，status='draft'
```

### PR-5 · `feat(quant-engine): strategy spec loader + ONNX runtime`

**目标**：Go 端 quant-engine 从 PG 加载 Spec、下载 ONNX、按因子推理出信号。

**新增 / 修改**：

```
backend/go/internal/quantengine/
    spec_loader.go     # 从 PG strategies 表读 spec(JSON) -> 结构体
    onnx_runner.go     # 用 github.com/yalue/onnxruntime_go 加载推理
    signal_emitter.go  # 推理结果 -> Signal -> 投递到 strategy-svc Runner
    sizing.go          # docs/14 资金分配公式
backend/go/internal/quantengine/runner.go  # 替换 demo 硬编码，改为按 spec 启动
backend/go/internal/strategysvc/runner.go  # 信号 → OMS 下单（已有占位，需接入）
```

**依赖新增**：`github.com/yalue/onnxruntime_go`（Apache-2.0）。Dockerfile.builder 增加 `onnxruntime` so 库。

**SQL 新增**：`migrations/00X_strategies_spec.up.sql`（若 `strategies.spec` 列已存在则跳过）。

**验收**：
```bash
# 1. submit 一个 Spec 后
go test ./internal/quantengine/... -run TestSpecLoader

# 2. 集成：
# - submit DSL-only spec → 推理后产生信号 → OMS 下 paper 单 → strategy-svc Runner 维护持仓
# - submit ONNX spec     → 同上
```

### PR-6 · `feat(api): BacktestService + ConsistencyGate Connect RPC`

**目标**：trading-core 暴露 BacktestService，研究员从前端 / Notebook 触发回测；策略上线前自动跑一致性校验。

**新增**：

```
backend/proto/alfq/v1/backtest.proto    # SubmitBacktest / GetResult / ValidateConsistency
backend/go/internal/adminapi/backtest_handler.go
backend/go/internal/adminapi/backtest_worker.go  # 内部触发 research/ python CLI
backend/go/internal/adminapi/consistency_gate.go  # 控制 strategy.status 转换：draft→ready 需通过
research/alfq_research/cli/
    backtest.py        # python -m alfq_research.cli.backtest --spec ... --out ...
```

trading-core 启动 backtest worker：从 NATS 队列拉任务、子进程跑 `uv run python -m alfq_research.cli.backtest`、产物写 MinIO、状态写 PG。

**验收**：
```bash
# 提交 spec → 自动跑 backtest → 自动跑一致性 → 通过则状态变 ready，未通过停留 draft
buf curl --schema backend/proto -d '{"strategy_id":"..."}' \
  http://localhost:9000/alfq.v1.BacktestService/SubmitBacktest
# 轮询 GetResult 拿 metrics
```

### PR-7 · `feat(deploy): paper-trading gate + audit trail`

**目标**：通过一致性校验的策略可一键部署到 paper 账户；运行 N 个交易日达标后由人工双签批准实盘。

**新增**：

```
backend/go/internal/strategysvc/lifecycle.go  # draft→ready→paper→live 状态机
backend/go/internal/strategysvc/promotion.go  # 升级规则（Sharpe>1.0、无 risk 事件、双签）
backend/proto/alfq/v1/strategy.proto         # PromoteStrategy RPC 增强
frontend/src/pages/Strategies.tsx            # 策略详情 + 升级按钮 + 审计日志
backend/go/internal/common/audit/             # 已有审计模块扩展
```

**验收清单**（docs/06 §6 + docs/11 §11.3）：
1. 回测 Sharpe > 1.0、MaxDD < 1.5× 目标
2. 向量化 vs 事件驱动 corr > 0.95
3. paper 5 交易日无 P0/P1 风控事件
4. 双签：`tenant_admin` + `risk_officer` 都签字
5. 升级动作写入审计表

---

## D. 跨 PR 公共约束

### D.1 数据流形态

```
                    [ClickHouse: bar/tick]
                              │
              ┌───────────────┼───────────────┐
              ▼               │               ▼
      [research vectorized]   │       [Go factor-svc 订阅 md.bar.>]
              │               │               │
              ▼               │               ▼
      [research event-driven] │       [Go quant-engine: ONNX 推理]
              │               │               │
              └────一致性 corr > 0.95─────────┘
                              │
                  [strategy.status = ready]
                              │
                              ▼
                       [paper 账户运行]
                              │
                       [risk-svc 监控]
                              │
                  人工双签 → [live 部署]
```

### D.2 单一事实源

| 内容 | 唯一来源 |
|---|---|
| 因子表达式语法 | `docs/09-因子DSL规范.md` |
| 算子语义 | `backend/go/internal/factor/dsl/*.go`（Go 实现，Py 必须对齐） |
| Spec 结构 | `docs/06-Python策略沙箱设计.md` §4 |
| MT 会计公式 | `docs/14-领域模型与交易规则.md` |
| Connect RPC 错误码 | `docs/20-错误码与异常处理规范.md` |
| 状态机定义 | `docs/14` + `backend/go/internal/strategysvc/lifecycle.go`（PR-7 新增） |

任何二次实现必须引用上述源；冲突时以源为准。

### D.3 防漂移 checklist（每个 PR 合并前）

- [ ] 文件单文件 ≤ 500 行（docs/12 §3.5）
- [ ] 函数圈复杂度 ≤ 15
- [ ] 单元测试覆盖率 ≥ 60%（覆盖关键路径）
- [ ] 新增 proto 经过 `buf lint` + `buf breaking`
- [ ] 涉及架构变更附 ADR（`docs/adr/`）
- [ ] PR description 用 `docs/17 §6.2` 模板
- [ ] 一次只动一个领域目录（避免跨服务大改）

### D.4 测试基线（CI 强制）

```bash
# Go 端
cd backend/go && GOTOOLCHAIN=local go test ./... -race -cover

# Python 端
cd research && uv run pytest -v --cov=alfq_research

# DSL Parity（最关键）
cd research && uv run pytest tests/test_dsl_parity.py -v -k "parity"

# 一致性 gate
cd research && uv run pytest tests/test_consistency.py -v
```

### D.5 回滚预案

每个 PR 都遵循 `docs/17-发布与变更管理.md` 的回滚 playbook。
若 PR-5（ONNX 集成）出现段错误，立刻通过 Feature Flag `quant_engine.onnx_enabled=false` 降级到 DSL-only 路径。

---

## E. 不在本计划范围（M8+）

- 多账户资金动态再平衡（M5 已有静态分配，动态留 M8）
- 策略组合优化（多策略相关性、风险平价）
- 强化学习 / Transformer 模型
- HFT / 做市
- 跨经纪商套利

---

## F. 进度跟踪

每个 PR 完成后在下表打勾，并将 commit hash 填入：

| PR | 标题 | 状态 | Commit | Handover |
|---|---|---|---|---|
| PR-1 | research: data + DSL parity | ☐ | — | — |
| PR-2 | research: vectorized backtest | ☐ | — | — |
| PR-3 | research: event-driven + consistency | ☐ | — | — |
| PR-4 | research: trainer + ONNX + spec submitter | ☐ | — | — |
| PR-5 | quant-engine: spec loader + ONNX runtime | ☐ | — | — |
| PR-6 | api: BacktestService + gate | ☐ | — | — |
| PR-7 | deploy: paper → live promotion | ☐ | — | — |

完成全部 PR 后写 `docs/handover/M7-handover.md`，结束 M7。

---

## G. AI Agent 启动指令

下一个接手的 Agent 直接执行：

1. 读完本文件 + `docs/12-AI-Agent实施指南.md` + `docs/18-AI-Agent工作流深化与策略助手.md`
2. 在 `docs/tasks/QUANT-EXECUTION-PLAN.md` 表 F 中勾选下一个 `☐` 的 PR
3. 严格按该 PR 的 "新增/修改文件" 范围编码
4. 跑完 "验收" 命令并截屏 / 保存日志
5. 提交 PR，在 description 中引用本文件章节号
6. 合并后更新表 F 与 `M7-handover.md`
