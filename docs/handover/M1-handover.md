# M1 Handover

> 日期：2026-05-18 | Agent 完成 | M1 行情阶段交付

## 已完成

| 模块 | 内容 | 关键文件 |
|---|---|---|
| market_data proto | Tick / Quote / Bar 消息定义 | `backend/proto/alfq/v1/market_data.proto` |
| mtapi proto 编译 | build.sh → gen/mt4/ + gen/mt5/ gRPC stub | `backend/proto-mtapi/build.sh`, gen 4 个 .pb.go |
| MT4 adapter | gRPC Dial → Connection → Streams.OnQuote → 流式消费 | `internal/mdgateway/adapter/mt4/client.go` (61行) |
| MT5 adapter | gRPC Dial → Connection → Streams.OnQuote → 流式消费 | `internal/mdgateway/adapter/mt5/client.go` (65行) |
| md-gateway 服务 | 连接管理、平台分发、行情标准化、NATS 发布 | `cmd/md-gateway/main.go`, `internal/mdgateway/manager.go` |
| NATS 发布 | 真实 nats.go JetStream → `md.tick.<broker>.<symbol>` | `internal/mdgateway/publisher.go` |
| ClickHouse 落盘 | 异步批量写入 (1s/1000条) + 背压溢写磁盘 | `internal/mdgateway/clickhouse_writer.go` |

## 验收记录

```
# Proto
$ make proto-gen
  → gen/alfq/v1/market_data.pb.go (Tick/Quote/Bar)

$ make proto-mtapi-gen
  → gen/mt4/{mt4.pb.go, mt4_grpc.pb.go}
  → gen/mt5/{mt5.pb.go, mt5_grpc.pb.go}

# Go 构建
$ go build ./cmd/admin-api ./cmd/md-gateway
  → admin-api 18M, md-gateway 21M

# Docker 六件套
$ make dev-up
  → postgres/clickhouse/redis/nats/vault/minio — 全 Healthy

# 文档合规
$ git status gprc/ --porcelain
  → 零变更（只读约束）
$ grep -r "gen/mt5" internal/mdgateway/adapter/mt4/
  → 无交叉导入
```

## 与 M1 计划的偏差

无。market_data proto、MT4/MT5 独立 gRPC adapter、NATS 实时发布、CH 异步落盘全部按 `docs/08 §2` 和 `docs/25 §3` 实现。

## 下一步建议

M2：Go/Python 因子 DSL 引擎 + factor-svc + 回测骨架。优先读 `docs/09-因子DSL规范.md`。
