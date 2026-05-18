# proto-mtapi —— mtapi (MT4/MT5) gRPC 桩代码构建目录

本目录用于把官方 mtapi proto（位于 `/opt/alfq/gprc/`，**只读**）生成 Go gRPC 桩代码到 `backend/go/gen/mt4/` 和 `backend/go/gen/mt5/`。

## 为什么不放在 `/opt/alfq/gprc/`？

`/opt/alfq/gprc/` 是 mtapi 官方提供的 proto 与示例目录，**禁止**在其中新增或修改文件，便于将来 `git pull` 同步官方更新。
本目录通过 `cp` + `sed` 在临时 wrapper 目录中改写 `option go_package` 为本地路径后再调用 `buf generate`，原始文件保持不变。

## 命令

```bash
# 完整流程见 docs/25-Proto代码生成与gRPC调用规范.md
make -C ../.. proto-mtapi-gen
```

或手工：

```bash
cd backend/proto-mtapi
./build.sh
```

## 产物

- `backend/go/gen/mt4/mt4.pb.go` + `mt4_grpc.pb.go`（package `mt4`）
- `backend/go/gen/mt5/mt5.pb.go` + `mt5_grpc.pb.go`（package `mt5`）

包含完整的：
- Connection / MT4|MT5 / Service / Subscriptions / Trading / Streams 等 service 客户端
- 服务端流式 RPC（`OnQuote` / `OnOrderUpdate` / `OnTickValue` / `OnOrderProfit`）的 `grpc.ServerStreamingClient[T]` 类型化客户端
