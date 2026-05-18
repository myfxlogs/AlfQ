# 10 - Python 研究层实现规范

> 位置：`research/`（独立研究域）。用 **uv** 管理依赖与 venv。所有用户面 API 都通过 `alfq_research` 包暴露。

## 1. 项目结构

```
research/
├── pyproject.toml
├── uv.lock
├── README.md
├── alfq_research/                # SDK 主包
│   ├── __init__.py
│   ├── config.py                 # 读取环境变量、连接配置
│   ├── data/
│   │   ├── client.py             # DataClient（CH/PG/MinIO 统一入口）
│   │   ├── ch.py                 # ClickHouse 加载（Polars）
│   │   ├── pg.py                 # PG 元数据
│   │   └── minio.py
│   ├── factor/
│   │   ├── registry.py
│   │   ├── dsl/                  # 与 09 章 Go 端对齐
│   │   │   ├── parser.py
│   │   │   ├── compile.py
│   │   │   └── ops/
│   │   └── eval.py               # 流式 + 批量两种引擎
│   ├── backtest/
│   │   ├── vectorized.py         # Polars 向量化
│   │   ├── event.py              # 事件驱动（撮合）
│   │   ├── broker_sim.py         # 滑点/手续费/swap 模型
│   │   ├── metrics.py            # Sharpe / Sortino / MaxDD ...
│   │   └── runner.py
│   ├── optimize/
│   │   ├── grid.py
│   │   ├── optuna_runner.py
│   │   └── walk_forward.py
│   ├── model/
│   │   ├── trainer.py            # LightGBM / sklearn 包装
│   │   └── exporter.py           # to_onnx / upload
│   ├── report/
│   │   ├── html.py               # quantstats 风格
│   │   └── pdf.py                # 可选
│   ├── spec/
│   │   └── strategy_spec.py      # 提交 spec 到 admin-api
│   ├── client/
│   │   ├── connect_client.py     # 调 Connect RPC
│   │   └── auth.py
│   └── utils/
├── tests/
│   ├── test_dsl_parity.py        # 与 Go 一致性
│   ├── test_backtest.py
│   └── test_metrics.py
└── notebooks/
    └── examples/
```

## 2. pyproject.toml

```toml
[project]
name = "alfq_research"
version = "0.1.0"
requires-python = ">=3.12"
dependencies = [
  "polars>=1.0",
  "numpy>=1.26",
  "scipy>=1.13",
  "scikit-learn>=1.5",
  "lightgbm>=4.3",
  "onnx>=1.16",
  "skl2onnx>=1.16",
  "onnxruntime>=1.18",
  "clickhouse-connect>=0.7",
  "psycopg[binary]>=3.2",
  "minio>=7.2",
  "httpx>=0.27",
  "pydantic>=2.7",
  "pyyaml>=6.0",
  "jinja2>=3.1",
  "matplotlib>=3.9",
  "quantstats>=0.0.62",
  "optuna>=3.6",
  "rich>=13.7",
]

[tool.uv]
dev-dependencies = ["ruff", "pytest", "pytest-asyncio", "mypy"]
```

## 3. 配置

`alfq_research/config.py` 从环境变量读取：

```
ALFQ_CH_HOST, ALFQ_CH_USER, ALFQ_CH_PASSWORD, ALFQ_CH_DB
ALFQ_PG_DSN
ALFQ_MINIO_ENDPOINT, ALFQ_MINIO_AK, ALFQ_MINIO_SK, ALFQ_MINIO_BUCKET
ALFQ_API_BASE       # admin-api Connect 端点
ALFQ_API_TOKEN      # 用户 JWT
ALFQ_TENANT_ID
```

JupyterHub spawner 自动注入。

## 4. 关键 API

### 4.1 DataClient

```python
from alfq_research import DataClient

dc = DataClient()
bars = dc.bars(
    symbols=["EURUSD","GBPUSD"],
    period="1m",
    start="2024-01-01",
    end="2025-01-01",
)  # → polars.DataFrame (symbol, ts, open, high, low, close, volume)

ticks = dc.ticks("EURUSD", start, end)
fv    = dc.factor_values("mom_20_60", "EURUSD", start, end)
```

### 4.2 Factor

```python
from alfq_research.factor import compile_expr, eval_batch

ast = compile_expr("ema($close, 20) / ema($close, 60) - 1")
result = eval_batch(ast, bars)         # polars Series
```

### 4.3 Backtest

```python
from alfq_research.backtest import VectorizedBacktest, EventBacktest

bt = VectorizedBacktest(
    bars=bars,
    spec=spec_dict,
    init_cash=10000,
    commission_bps=2,
    slippage_bps=1,
)
result = bt.run()
print(result.metrics)
result.save_report("report.html")

# 事件驱动（更慢但更准）
eb = EventBacktest(...)
result2 = eb.run()
```

### 4.4 Optimize

```python
from alfq_research.optimize import OptunaRunner, WalkForward

wf = WalkForward(
    bars=bars,
    spec_template=spec,
    objective="sharpe",
    train_months=12,
    test_months=3,
    n_trials=200,
)
best = wf.run()
```

### 4.5 Model 导出

```python
from alfq_research.model import to_onnx, upload_model
model = lgb.train(...)
to_onnx(model, "momentum_v1.onnx", input_features=["mom_20_60","atr_14"])
uri = upload_model("strategies/momentum/v1.onnx")  # 返回 s3://...
```

### 4.6 StrategySpec

```python
from alfq_research import StrategySpec

spec = StrategySpec.from_yaml("specs/momentum.yaml")
spec.set_model_uri(uri)
spec.validate()                       # 本地语法/语义校验
spec.submit(name="momentum_v1")       # 调 Connect API 创建 strategies(draft)
```

## 5. 回测引擎细节

### 5.1 VectorizedBacktest

- 输入：bars + spec
- 步骤：
  1. 用 Polars 计算所有因子
  2. 计算 signal series（依据 spec.signal_rule）
  3. 计算 position series（差分得到入场/出场）
  4. 计算 PnL series（含滑点/手续费/swap）
  5. 生成指标
- 速度：百万根 bar / 秒级

### 5.2 EventBacktest

- 输入：tick 或 bar
- 复用 `factor.eval`（流式）—— **算子代码必须与 Go 端逻辑一致**
- 模拟撮合：
  - 市价：以 next bar open + 滑点；或当前 tick 对侧价
  - 限价：到价成交；考虑成交概率
  - 拒单概率（可配置）
- 复用 spec → signal 推理（用 onnxruntime 直接加载 ONNX）

### 5.3 metrics

```
return, cumret, daily_pnl, sharpe, sortino, calmar,
max_drawdown, max_dd_duration, win_rate, profit_factor,
trade_count, avg_trade, avg_holding, turnover
```

## 6. 报告（quantstats 风格）

`report/html.py` 用 Jinja2 渲染：

- 净值曲线 / 回撤曲线 / 月度热力图 / 收益分布 / 滚动 Sharpe
- 关键指标表
- 交易明细（可选下载 CSV）

输出 HTML 自包含（base64 图）或外链对象存储。

## 7. 与 admin-api 的交互

`client/connect_client.py`：

- 用 `httpx` 直调 Connect HTTP/JSON 端点（不需要 grpc，浏览器友好）
- 自动带 `Authorization: Bearer ALFQ_API_TOKEN`
- 失败重试 + 401 自动 refresh（如果有 refresh_token）

封装服务方法对齐 §03，如：

```python
client.factor.create(name=..., expression=...)
client.strategy.create(spec=...)
client.backtest.start(strategy_id, params)
```

## 8. 沙箱限制下的实现注意

- 不能 `subprocess`、不能写系统目录
- 大数据集走 ClickHouse 服务端聚合后再下载，避免内存爆
- 训练任务长时间运行：建议用 `optuna` + 检查点保存到 MinIO

## 9. 一致性测试

`tests/test_dsl_parity.py` 流程：

1. 加载 `tests/fixtures/golden_bars.parquet`
2. 对 30 个表达式分别用 Python 流式 + Python 批量 + Go（通过 admin-api `ValidateExpression` + `PreviewFactor` 的服务接口或单独的 test 端点）跑
3. 逐点对比，max abs diff < 1e-9
4. CI 步骤：`pytest tests/test_dsl_parity.py`

## 10. 验收

- [ ] `uv sync` 成功
- [ ] `pytest` 全部通过
- [ ] `ruff` + `mypy` 无错误
- [ ] DSL parity 测试通过
- [ ] 示例 notebook（`notebooks/examples/quickstart.ipynb`）能跑通完整流程：拉数据 → 建因子 → 回测 → 训练 → 导 ONNX → 提交 spec
