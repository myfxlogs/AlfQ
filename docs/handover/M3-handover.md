# M3 Handover

> 日期：2026-05-18 | Agent 完成 | M3 策略+OMS 阶段交付

## 已完成

| 模块 | 内容 | 关键文件 |
|---|---|---|
| order.proto | OrderState(12状态)/Order/Position/OrderRequest/RiskCheckResult 消息定义 | `backend/proto/alfq/v1/order.proto` |
| OMS 状态机 | 12 状态完整转换表 + IsTerminal 终态判断 + Transition 校验 | `internal/oms/statemachine.go` |
| OMS BrokerAdapter | 4 方法接口 + MT4Adapter/MT5Adapter 独立实现（零交叉） | `internal/oms/adapter.go` |
| risk-svc 规则引擎 | 5 规则 (max_lot/max_position/daily_loss/drawdown/whitelist) + Check 顺序执行 | `internal/risksvc/engine.go` |
| risk-svc Kill Switch | Activate/Deactivate/IsActive/Status + 熔断器 | `internal/risksvc/killswitch.go` |
| strategy-svc | Strategy 接口 (OnFactor/OnBar) + Runner + Signal 信号模型 | `internal/strategysvc/runner.go` |
| 三服务入口 | oms:9005, risk-svc:9004, strategy-svc:9003 + healthz 端点 | `cmd/{oms,risk-svc,strategy-svc}/main.go` |

## 验收记录

```
# Proto 生成
$ make proto-gen
  → gen/alfq/v1/order.pb.go (OrderState 12值 / Order / Position / OrderRequest / RiskCheckResult)

# 六服务编译
$ go build ./cmd/admin-api ./cmd/md-gateway ./cmd/factor-svc \
    ./cmd/oms ./cmd/risk-svc ./cmd/strategy-svc
  → 六服务全绿

# 状态机终态覆盖
$ grep IsTerminal internal/oms/statemachine.go
  → CANCELLED / REJECTED / FAILED / FILLED / EXPIRED 全覆盖

# MT4/MT5 分离
$ grep -c "MT4Adapter\|MT5Adapter" internal/oms/adapter.go
  → 18 处引用，各自独立类型，零交叉导入

# go.mod 零 replace
$ grep replace go.mod
  → 0 条
```

## 服务全景

| 服务 | 端口 | 状态 |
|---|---|---|
| admin-api | 8080 | ✅ |
| md-gateway | 9001 | ✅ |
| factor-svc | 9002 | ✅ |
| strategy-svc | 9003 | ✅ |
| risk-svc | 9004 | ✅ |
| oms | 9005 | ✅ |

## 与 M3 计划的偏差

无。proto 定义、状态机转换表、BrokerAdapter 接口、MT4/MT5 独立 adapter、规则引擎、Kill Switch、熔断器、Strategy Runner 全部按 `docs/08 §4-§6` 和 `docs/14 §3.1` 实现。

## 下一步建议

M4：风控完善 — 完整规则集 + Kill Switch 实际执行（撤单/平仓）+ 告警。优先读 `docs/08 §5` + `docs/15 §5` 风险指标 + `docs/17` 发布管理。
