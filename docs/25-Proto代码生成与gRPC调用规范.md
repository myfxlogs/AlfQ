# 25 - Proto 代码生成与 gRPC 调用规范

> 本文统一规范两类 proto 与 gRPC 桩代码的产出与调用：
> 1. **业务自有 proto**（`backend/proto/alfq/v1/*.proto`）：用 `buf generate` + `protocolbuffers/go` + `connectrpc/go` + `bufbuild/es` + `connectrpc/es` 同时输出 Go / TS。
> 2. **mtapi 官方 proto**（`/opt/alfq/gprc/{mt4,mt5}.proto`）：**只读**，通过 `backend/proto-mtapi/build.sh` 构建出标准 Go gRPC 桩代码（`google.golang.org/grpc`）。
>
> 参考：doc 03（API 与接口规范）、doc 04（前端设计）、doc 08（Go 服务实现规范）、doc 14 §3.4（MT4 vs MT5 协议差异）。

---

## 1. 工具链版本

| 工具 | 版本 | 安装 |
|---|---|---|
| `buf` | ≥ 1.29 | `go install github.com/bufbuild/buf/cmd/buf@latest` 或下载 binary |
| `protoc-gen-go` | ≥ v1.36 | `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` |
| `protoc-gen-go-grpc` | ≥ v1.6 | `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest` |
| `protoc-gen-connect-go` | 由 buf 自动从 buf.build 拉取 | — |
| `protoc-gen-es` / `protoc-gen-connect-es` | 由 buf 自动从 buf.build 拉取 | — |

**`PATH` 必须包含 `$HOME/go/bin`**，否则 `buf generate` 找不到本地 `protoc-gen-go` / `protoc-gen-go-grpc`。

国内代理推荐：
```bash
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=off
```

---

## 2. 业务自有 proto（`backend/proto/`）

### 2.1 目录结构

```
backend/proto/
├── buf.yaml              # name + deps + lint/breaking 规则
├── buf.gen.yaml          # 4 个生成插件（Go × 2 + TS × 2）
├── buf.lock              # protovalidate 等依赖锁
└── alfq/v1/
    ├── common.proto
    ├── errors.proto
    └── health.proto
```

### 2.2 `buf.gen.yaml`

```yaml
version: v1
managed:
  enabled: true
  go_package_prefix:
    default: github.com/alfq/backend/go/gen
    except:
      - buf.build/bufbuild/protovalidate
plugins:
  # Go 后端
  - plugin: buf.build/protocolbuffers/go
    out: ../go/gen
    opt: [paths=source_relative]
  - plugin: buf.build/connectrpc/go
    out: ../go/gen
    opt: [paths=source_relative]
  # 前端 TypeScript（V2：@bufbuild/protobuf + @connectrpc/connect-web）
  - plugin: buf.build/bufbuild/es
    out: ../../frontend/src/gen
    opt: [target=ts]
  - plugin: buf.build/connectrpc/es
    out: ../../frontend/src/gen
    opt: [target=ts]
```

### 2.3 命令

```bash
make proto         # = proto-lint + proto-gen
make proto-lint    # buf lint
make proto-gen     # buf generate
```

产物：
- `backend/go/gen/alfq/v1/*.pb.go`（message）
- `backend/go/gen/alfq/v1/alfqv1connect/*.connect.go`（Connect service）
- `frontend/src/gen/alfq/v1/*_pb.ts`（TS message）
- `frontend/src/gen/alfq/v1/*_connect.ts`（TS 客户端 stub）

### 2.4 调用样例（Go 服务端）

```go
import (
    healthv1 "github.com/alfq/backend/go/gen/alfq/v1"
    "github.com/alfq/backend/go/gen/alfq/v1/alfqv1connect"
    "connectrpc.com/connect"
)

type healthSvc struct{}
func (h *healthSvc) Check(ctx context.Context, req *connect.Request[healthv1.HealthRequest]) (*connect.Response[healthv1.HealthReply], error) {
    return connect.NewResponse(&healthv1.HealthReply{Status: "ok"}), nil
}

mux := http.NewServeMux()
path, handler := alfqv1connect.NewHealthServiceHandler(&healthSvc{})
mux.Handle(path, handler)
```

### 2.5 调用样例（前端 TS）

```ts
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { HealthService } from "@/gen/alfq/v1/health_connect";

const transport = createConnectTransport({ baseUrl: "/api" });
const client = createClient(HealthService, transport);
const reply = await client.check({});
```

---

## 3. mtapi 官方 proto（`/opt/alfq/gprc/`）

### 3.1 重要约束 ⚠️

> **`/opt/alfq/gprc/` 是 mtapi 官方目录，禁止在其中新增或修改任何文件。**
> - 包含官方 proto：`mt4.proto`、`mt5.proto`
> - 包含官方 Go 客户端示例：`mt4goExample/`、`mt5goExample/`
> - 升级方式：直接 `git pull` 同步上游（或重新解压官方包）
>
> 我们通过 `backend/proto-mtapi/build.sh` 在临时目录中复制并改写 `option go_package` 后再生成，原始文件保持只读。

### 3.2 远端 gRPC 服务器

| 平台 | 远端入口（gRPC over TLS） |
|---|---|
| **MT4** | `mt4grpc3.mtapi.io:443` |
| **MT5** | `mt5grpc3.mtapi.io:443` |

> 注：官方 example 使用历史地址 `mt4grpc.mtapi.io` / `mt5grpc.mtapi.io`，本项目统一使用 `mt4grpc3` / `mt5grpc3`（v3 入口）。

### 3.3 生成命令

```bash
make proto-mtapi-gen
# 等价于：bash backend/proto-mtapi/build.sh
```

`build.sh` 工作流：

```
┌─────────────────────────┐
│ /opt/alfq/gprc/         │  只读
│   mt4.proto             │
│   mt5.proto             │
└────────────┬────────────┘
             │ cp + sed (仅在 tmp 改写 go_package)
             ▼
┌─────────────────────────┐
│ /tmp/xxx/               │  临时
│   mt4.proto (改写后)    │
│   mt5.proto (改写后)    │
│   buf.yaml              │
│   buf.gen.yaml          │
└────────────┬────────────┘
             │ buf generate
             ▼
┌─────────────────────────┐
│ backend/go/gen/         │
│   mt4/mt4.pb.go         │  package mt4
│   mt4/mt4_grpc.pb.go    │
│   mt5/mt5.pb.go         │  package mt5
│   mt5/mt5_grpc.pb.go    │
└─────────────────────────┘
```

### 3.4 产物结构

每份 `*_grpc.pb.go` 包含完整的 service 客户端：

| Service | MT4 | MT5 |
|---|---|---|
| `Connection` | ✓ | ✓ |
| `MT4` / `MT5` | ✓ | ✓ |
| `Trading` | ✓（5 RPC） | ✓（3 RPC） |
| `Service` | ✓ | ✓ |
| `Subscriptions` | ✓ | ✓ |
| `Streams` | ✓（4 路） | ✓（9 路） |
| `QuoteHistory` | — | ✓ |
| `TickHistory` | — | ✓ |

服务端流式 RPC（如 `OnQuote`、`OnOrderUpdate`）使用 `grpc.ServerStreamingClient[T]` 类型化客户端。

### 3.5 适配器实现（已落地骨架）

```
backend/go/internal/mdgateway/adapter/
├── mt4/client.go      # *mt4.Client 含 6 个 service handle
└── mt5/client.go      # *mt5.Client 含 8 个 service handle
```

最小调用样例（MT4，参考 `gprc/mt4goExample/main.go`）：

```go
import (
    "context"
    "fmt"
    "log"

    "github.com/alfq/backend/go/internal/mdgateway/adapter/mt4"
    mt4pb "github.com/alfq/backend/go/gen/mt4"
)

func main() {
    ctx := context.Background()
    cli, err := mt4.Dial(ctx, mt4.DefaultEndpoint) // mt4grpc3.mtapi.io:443
    if err != nil { log.Fatal(err) }
    defer cli.Close()

    // 1. 登录获得 session id
    resp, err := cli.Connection.Connect(ctx, &mt4pb.ConnectRequest{
        Host: "mt4-demo.roboforex.com", Port: 443,
        User: 500476959, Password: "ehj4bod",
    })
    if err != nil || resp.Error != nil { log.Fatal(err, resp.GetError()) }
    id := resp.Result

    // 2. 订阅 + 流式消费
    if _, err := cli.Subscriptions.Subscribe(ctx, &mt4pb.SubscribeRequest{
        Id: id, Symbol: "EURUSD", Interval: 0,
    }); err != nil { log.Fatal(err) }

    stream, err := cli.Streams.OnQuote(ctx, &mt4pb.OnQuoteRequest{Id: id})
    if err != nil { log.Fatal(err) }
    for {
        quote, err := stream.Recv()
        if err != nil { log.Fatal(err) }
        fmt.Println(quote)
    }
}
```

MT5 调用方式相同，但 `import` 与 message/enum 名**不可与 MT4 共用**（详见 doc 14 §3.4）。

### 3.6 升级官方 proto 流程

```bash
# 1. 同步 gprc/ 到上游最新（git pull / 解压）
# 2. 重新生成桩代码
make proto-mtapi-gen
# 3. 修复任何 API 变化导致的编译错误
cd backend/go && go build ./...
# 4. 跑回归测试
make go-test
# 5. 提 PR：commit message 包含 mtapi 上游 commit hash
```

---

## 4. 常见问题

### 4.1 `buf generate` 报错：`module identity ... is invalid`

`go_package_prefix.override` 的 key 必须是 buf module 标识（`remote/owner/repo`），**不接受裸 proto package 名**。改用 `Mfile.proto=path` 写在每个 plugin 的 `opt` 中。

### 4.2 生成的代码 `package _go`

protoc-gen-go 没拿到正确的 `go_package`。检查：
1. proto 文件 `option go_package = "..."` 是否存在
2. 若用 `M` 映射，`-Mxxx.proto=...` 写法和 import path 是否完全匹配

### 4.3 `protoc-gen-go: program not found`

`buf generate` 调用本地 plugin 时找不到。解决：
```bash
export PATH="$PATH:$HOME/go/bin"
```
或在 `buf.gen.yaml` 用远程 plugin（`buf.build/protocolbuffers/go`），buf 会自动从 buf.build 容器化运行。

### 4.4 国内访问 `buf.build` 慢/失败

- `buf.build/protocolbuffers/go` 等远程 plugin 走 `https://buf.build`，国内可能慢但通常可达。
- 慢时改用本地 binary plugin（`plugin: go` 而非 `plugin: buf.build/protocolbuffers/go`），见 `backend/proto-mtapi/build.sh` 写法。

---

## 5. 验收清单

- [x] `make proto-gen` 输出 Go + TS（业务 proto）
- [x] `make proto-mtapi-gen` 输出 `backend/go/gen/{mt4,mt5}/`（mtapi proto）
- [x] `cd backend/go && go build ./...` 双绿
- [x] `gprc/` 目录 `git status` 干净（不含我方任何新文件/修改）
- [x] MT4 与 MT5 各自独立 package，不交叉导入
