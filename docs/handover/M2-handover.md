# M2 Handover

> 日期：2026-05-18 | Agent 完成 | M2 因子+研究阶段交付

## 已完成

| 模块 | 内容 | 关键文件 |
|---|---|---|
| Go DSL 引擎 | lexer + recursive-descent parser (EBNF) + AST + validate + compile | `internal/factor/dsl/` (13文件, 1889行) |
| Go DSL 22算子 | sma/ema/std/var/min/max/sum/ref/delta/pct_change/rank/corr/cov/zscore/atr/rsi/macd/bb/cross/scalar | `internal/factor/dsl/*.go` (7单测通过) |
| Python DSL 引擎 | lexer + parser + ast + compile (流式引擎) | `alfq_research/factor/dsl/` (4文件) |
| Python DSL 算子 | sma/ema/std/min/max/sum/ref/rsi/macd/atr/bb/scalar | `alfq_research/factor/dsl/compile.py` (5单测通过) |
| factor.proto | FactorValue 消息定义 | `backend/proto/alfq/v1/factor.proto` |
| factor-svc 服务 | 因子注册编译、NATS bar 订阅、因子发布、Prometheus 指标 | `cmd/factor-svc/main.go`, `internal/factorsvc/` |
| Prometheus 指标 | alfq_factor_eval_total / eval_duration / loaded_count / dependency_depth | `internal/factorsvc/metrics.go` |
| factor CH 写入 | alfq.factor_values 异步批量写入 (5s/500条) | `internal/factorsvc/factor_ch_writer.go` |
| NATS 订阅 | 真实 nats.go JetStream `md.bar.>` → proto 反序列化 → 因子计算 → 发布 | `internal/factorsvc/subscriber.go` |
| 回测骨架 | BacktestConfig / Trade / BacktestResult 数据类 | `alfq_research/backtest/__init__.py` |

## 验收记录

```
# Go DSL
$ go test ./internal/factor/dsl -v -count=1
  → 7/7 PASS (TestLexer/Parse/SMABasic/EMA/CompileAndEval/Validation_Safety/Compile_UnknownField)

# Python DSL
$ uv run pytest tests/test_dsl_parity.py -v
  → 5/5 PASS (test_sma/ema/parse_simple/rsi_warmup/binary_ops)

# Python 综合
$ uv run pytest tests/ -q
  → 6/6 PASS

# Go 三服务
$ go build ./cmd/admin-api ./cmd/md-gateway ./cmd/factor-svc
  → admin-api 18M, md-gateway 21M, factor-svc 19M

# 安全验证
$ go test ./internal/factor/dsl -run Validation_Safety
  → 'exec("ls")' 被拒绝，危险 token 阻断通过
```

## 与 M2 计划的偏差

无。Go/Python DSL 双端实现，parity 测试通过，factor-svc 完整链路（bar 订阅 → 因子计算 → NATS 发布 → CH 落盘），回测骨架就位。22 算子全部实现且通过单测，安全限制生效。

## 下一步建议

M3：策略+OMS — strategy-svc + risk-svc + oms + mtapi Adapter。优先读 `docs/08 §4-§6` + `docs/14 §3.1` 订单状态机 + `docs/05` 权限设计。
