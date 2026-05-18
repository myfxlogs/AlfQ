---
name: proto-grpc
description: >
  ALFQ 项目 proto 代码生成与 gRPC 调用规范。两类 proto：业务自有（buf generate → Connect Go/TS）和 mtapi 官方（build.sh → 标准 gRPC）。
  覆盖 buf.gen.yaml 四插件、mt4/mt5 各自独立 gen、adapter 布局、常见问题，以及 MT4 vs MT5 协议级差异（禁止共用 proto 类型）。
---

# Proto 代码生成与 gRPC 调用规范

## 两类 proto

| 类型 | 源 | 生成方式 | 产物 |
|---|---|---|---|
| 业务自有 | `backend/proto/alfq/v1/*.proto` | `make proto-gen` (buf) | `gen/alfq/v1/*.pb.go` + `gen/alfq/v1/alfqv1connect/*.connect.go` + `frontend/src/gen/alfq/v1/*_pb.ts` + `*_connect.ts` |
| mtapi 官方 | `/opt/alfq/gprc/{mt4,mt5}.proto` | `make proto-mtapi-gen` (build.sh) | `gen/mt4/{mt4.pb.go,mt4_grpc.pb.go}` + `gen/mt5/{mt5.pb.go,mt5_grpc.pb.go}` |

## 硬性约束

1. **`/opt/alfq/gprc/` 只读** —— 禁止新增或修改任何文件。升级走 `git pull` 同步上游。
2. **MT4 与 MT5 独立** —— gen/mt4/ 和 gen/mt5/ 分离 package，不可交叉导入；adapter 各一套 client.go（见 `internal/mdgateway/adapter/mt4/` 和 `mt5/`）。
3. **buf.gen.yaml 四插件** —— `protocolbuffers/go` + `connectrpc/go` + `bufbuild/es` + `connectrpc/es`。

## mtapi 远端入口

| 平台 | 地址 |
|---|---|
| MT4 | `mt4grpc3.mtapi.io:443` |
| MT5 | `mt5grpc3.mtapi.io:443` |

## 命令

```bash
make proto              # buf lint + buf generate（业务 proto → Go + TS）
make proto-mtapi-gen    # bash backend/proto-mtapi/build.sh（mtapi proto → Go gRPC）
```

## adapter 布局

```
internal/mdgateway/adapter/
├── mt4/client.go      # *mt4.Client：Dial(DefaultEndpoint) → Connection/MT4/Streams 等 service handles
└── mt5/client.go      # *mt5.Client：Dial(DefaultEndpoint) → Connection/MT5/Streams 等 8 个 service handles
```

## 调用样例（MT4）

```go
import (
    "github.com/alfq/backend/go/internal/mdgateway/adapter/mt4"
    mt4pb "github.com/alfq/backend/go/gen/mt4"
)

cli, _ := mt4.Dial(ctx, mt4.DefaultEndpoint)
resp, _ := cli.Connection.Connect(ctx, &mt4pb.ConnectRequest{...})
id := resp.Result

stream, _ := cli.Streams.OnQuote(ctx, &mt4pb.OnQuoteRequest{Id: id})
for {
    quote, _ := stream.Recv()
    // normalize → alfq.v1.Tick
}
```

MT5 调用方式相同，但 import `gen/mt5` 与 `adapter/mt5`，类型名不可与 MT4 混用。

## MT4 vs MT5 协议差异（关键）

- MT4 持仓 Hedging，MT5 默认 Netting（账号可选 Hedging）
- MT4 `enum Op` (8值) / MT5 `enum OrderType` (11值) —— 不可枚举映射
- MT4 无 Deal 概念，MT5 `DealType` 含 Balance/Credit/Charge 等账户调整
- MT4 Streams 4 路，MT5 Streams 9 路
- MT4 有 High/Low，MT5 有 Last/Volume
- 详见 `docs/14-领域模型与交易规则.md` §3.4

## 常见问题

- `buf generate` 报 module identity invalid → `go_package_prefix.override` key 必须是 buf module 标识
- `protoc-gen-go: program not found` → `export PATH="$PATH:$HOME/go/bin"`
- 国内慢 → 本地 binary plugin（`plugin: go` 而非 `plugin: buf.build/protocolbuffers/go`）
