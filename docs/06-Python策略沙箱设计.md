# 06 - Python 策略与研究沙箱设计

> 核心原则：**用户编写的 Python 代码绝不进入生产实盘进程**。Python 仅在隔离沙箱中用于研究、训练、回测。生产策略由 DSL + ONNX 组成，由 Go 服务执行。

## 1. 总体模型

```
┌─────────────────────────────────────────────────────────────┐
│  研究层（隔离沙箱）                                            │
│   Python: 数据探索 / 因子草稿 / 模型训练 / 回测                 │
│      ↓ 产出                                                   │
│   - 因子表达式 DSL（字符串）                                   │
│   - ONNX 模型文件                                              │
│   - 策略参数 YAML                                              │
│      ↓ 走审批/CI                                              │
├─────────────────────────────────────────────────────────────┤
│  生产层（Go 服务，无 Python）                                  │
│   quant-engine：                                              │
│   - 解析 DSL → 增量算子树                                      │
│   - 加载 ONNX → onnxruntime-go 推理                            │
│   - 调用风控/OMS                                              │
└─────────────────────────────────────────────────────────────┘
```

## 2. 研究沙箱

### 2.1 形态

- **JupyterHub** + Docker/K8s spawner（推荐 `kubespawner`）
- 每用户一个沙箱容器：`alfq/notebook:<version>`
- 镜像内含：Python 3.12 + Polars + LightGBM + scikit-learn + ONNX + 内置 `alfq_research` SDK

### 2.2 容器约束

```yaml
resources:
  requests: { cpu: "1", memory: "2Gi" }
  limits:   { cpu: "4", memory: "8Gi" }     # 按租户 plan 调整
network:
  egress_whitelist:                          # 仅允许出向以下目标
    - clickhouse:9000
    - postgres:5432
    - minio:9000
    - trading-core:9000
  ingress: []                                # 仅 JupyterHub 可访问
filesystem:
  read_only_root: true
  writable_volumes:
    - /home/jovyan/work  (per-user PVC, 限额 10G)
    - /tmp              (emptyDir, 限额 1G)
security_context:
  run_as_non_root: true
  allow_privilege_escalation: false
  seccomp_profile: RuntimeDefault
  drop_capabilities: [ALL]
timeouts:
  idle_kill_seconds: 1800
  max_session_hours: 8
```

### 2.3 数据访问

容器启动时由 JupyterHub spawner 注入：

| 凭据 | 范围 | 注入方式 |
|---|---|---|
| PG 只读账号 | RLS = 当前 tenant，且仅可读用户被授权的表 | 环境变量 |
| CH 只读账号 | row policy = 当前 tenant | 环境变量 |
| MinIO STS Token | 前缀 `tenants/{tid}/notebooks/{uid}/` | 环境变量，1h 过期，自动续 |
| trading-core JWT | 用户自身 | 环境变量 |

### 2.4 SDK：`alfq_research`

```python
from alfq_research import (
    DataClient,        # 加载历史 bar/tick/因子值
    FactorRegistry,    # 注册因子 DSL（与 Go 端共用语义）
    Backtest,          # 回测引擎（向量化 / 事件驱动）
    ModelExporter,     # 训练完成后导出 ONNX 并上传
    StrategySpec,      # 构建策略 spec 提交审批
)

# 数据
dc = DataClient()      # 自动用环境变量凭据
df = dc.bars(symbols=["EURUSD"], period="1m",
             start="2024-01-01", end="2025-01-01")

# 因子（DSL 字符串，Python 端可执行验证）
expr = "ema($close, 20) / ema($close, 60) - 1"
fr = FactorRegistry()
preview = fr.preview(expr, df)

# 训练
import lightgbm as lgb
model = lgb.train(...)
ModelExporter.to_onnx(model, "momentum_v1.onnx")
ModelExporter.upload("strategies/momentum/v1.onnx")

# 提交策略
spec = StrategySpec(
    name="momentum_v1",
    factors=[{"name":"mom_20_60","expr":expr}],
    signal_rule={"type":"onnx_threshold",
                 "model":"strategies/momentum/v1.onnx",
                 "long_th":0.6, "short_th":-0.6},
    bar_period="1m",
)
spec.submit()  # 调 trading-core 创建 strategy（草稿态）
```

### 2.5 Notebook 安全限制

- 禁止 `pip install` 安装到系统目录（只能 `--user`）
- 禁止 `subprocess` 外部命令（用 seccomp profile 限制 syscall）
- 禁止访问宿主网络（egress 白名单）
- 禁用 GPU 或限定 GPU（按 plan）
- 自动审计每个 cell 执行（JupyterHub `pre_spawn_hook` + `nbgitpuller` 可选）

## 3. 平台开发者扩展（不是用户代码）

如果需要新增 DSL 算子或新 ML 推理类型：

- 平台开发者在主仓库 `backend/go/internal/factor/op_*.go` 添加算子
- Python 端 `research/alfq_research/factor/op_*.py` 同步实现
- 经 PR / CI / Code Review 合入
- **用户无法注入此类代码**

## 4. 策略 Spec 规约

策略由 JSON Spec 描述，存 `strategies.spec`：

```json
{
  "version": 1,
  "factors": [
    {"name": "mom_20_60", "expr": "ema($close,20)/ema($close,60)-1"},
    {"name": "atr_14",    "expr": "atr(14)"}
  ],
  "bar_period": "1m",
  "symbols": ["EURUSD","GBPUSD"],
  "signal": {
    "type": "onnx_threshold",
    "model_uri": "s3://alfq/tenants/{tid}/models/momentum_v1.onnx",
    "input_features": ["mom_20_60","atr_14"],
    "output": "score",
    "long_threshold": 0.6,
    "short_threshold": -0.6
  },
  "position_sizing": {
    "type": "fixed_lot",
    "lot": 0.1
  },
  "exit": {
    "type": "atr_stop",
    "sl_atr": 2.0,
    "tp_atr": 3.0
  }
}
```

`quant-engine` 启动时按 Spec：

1. 注册需要的因子到 `quant-engine`
2. 从对象存储下载 ONNX 模型，加载到 onnxruntime-go
3. 订阅 NATS 上对应 factor 主题
4. 每根 bar 关闭时拉取因子 → 推理 → 信号

### 4.1 支持的 signal 类型

| type | 含义 |
|---|---|
| `dsl_rule` | 纯表达式：`mom_20_60 > 0` 触发 long |
| `onnx_threshold` | 神经网络输出阈值 |
| `onnx_classifier` | 多分类（long/flat/short） |
| `linear_combo` | 因子线性组合 + 阈值 |

新增类型由平台开发者扩展。

## 5. 回测引擎

### 5.1 双引擎

| 引擎 | 实现 | 用途 |
|---|---|---|
| 向量化 | Python（Polars + numpy） | 参数搜索，秒级出结果，不精确撮合 |
| 事件驱动 | Python（**与生产 quant-engine 共用 DSL 解释器**） | 接近实盘的撮合，验证 spec |

事件驱动撮合细节：滑点 / 点差 / 手续费 / swap / 拒单概率，参考 `nkaz001/hftbacktest` 简化版。

### 5.2 一致性校验

策略上线前自动跑：

```
向量化结果（每日 PnL 序列）
事件驱动结果（每日 PnL 序列）
两者 corr > 0.95 且 dailyPnL 偏差 < 1% 才允许 Deploy
```

由 `BacktestService` 提供 `ValidateConsistency` RPC，CI 强制门禁。

## 6. 流程：从研究到实盘

```
[1] 研究员在 Notebook 写 DSL + 训练 ONNX
[2] StrategySpec.submit() → trading-core → strategies(draft)
[3] 后台运行 Backtest，向量化 + 事件驱动 + 一致性校验
[4] 提交审批 → Quant Lead 在前端审批
[5] approved → 可 Deploy 到 paper 账户
[6] paper N 天达标 → 申请实盘 → tenant_admin + risk_officer 双签
[7] live 部署，初始资金小额，逐步放量
```

## 7. 验收检查项

- [ ] JupyterHub 启动后用户独立沙箱、网络白名单生效
- [ ] 容器 egress 测试：访问外网 IP 失败、访问 CH/PG 成功
- [ ] PG 只读账号无法跨租户查询（RLS 命中）
- [ ] `alfq_research` SDK 安装在镜像内，import 可用
- [ ] ONNX 导出 → 上传 MinIO → Go 加载推理一致性测试通过
- [ ] 同 DSL 表达式在 Python 与 Go 端结果对齐（误差 < 1e-9）
- [ ] 一致性校验失败的策略不能进入 live 状态
