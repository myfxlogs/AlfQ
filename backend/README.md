# Backend — ALFQ 实盘核心

Go 微服务集群。所有实盘逻辑（行情、因子、策略、风控、OMS）在此域。

## 目录

```
backend/
├── proto/          # buf 工程，协议单一源
│   └── alfq/v1/    # 内部 proto 定义
├── go/             # Go 工作区
│   ├── cmd/        # 7 个服务入口
│   ├── internal/   # 内部包
│   └── migrations/ # 数据库迁移
└── README.md
```

## 服务

| 服务 | 端口 | 职责 |
|---|---|---|
| trading-core | 8080 | 对前端唯一入口（Connect+SSE） |
| md-gateway | 9001 | mtapi 行情接入 |
| quant-engine | 9002 | 增量因子计算 |
| quant-engine | 9003 | 策略执行（DSL+ONNX） |
| trading-core | 9004 | 风控网关 |
| oms | 9005 | 订单管理 |
| assistant-svc | 9006 | AI 策略助手（M3.5+） |

## 技术栈

- Go 1.22+, gofumpt, golangci-lint, sqlc
- Connect RPC + gRPC + NATS JetStream

## 参考项目

参考：bbgo (策略接口设计), nautilus_trader (分层架构), gocryptotrader (Adapter 模式)
