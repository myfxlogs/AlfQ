# 03 - API 与接口规范（Connect RPC + SSE）

> 所有对外/对内 API 用 **Connect RPC**。一份 `.proto` 同时提供 gRPC / gRPC-Web / Connect 三协议。服务器推送一律走 **Server Streaming RPC**（Connect 上自动为 SSE 传输）。

## 1. 工具链

- **buf CLI**：lint、breaking 检查、代码生成
- **Go**：`connectrpc.com/connect`、`connectrpc.com/grpcreflect`、`connectrpc.com/otelconnect`
- **TS**：`@connectrpc/connect-web`、`@connectrpc/connect-query`、`@bufbuild/protobuf`
- **Python（仅研究层）**：`betterproto` 或 `grpc` 客户端

### 1.1 `buf.yaml`（根目录）

```yaml
version: v2
modules:
  - path: proto
breaking:
  use: [FILE]
lint:
  use: [STANDARD]
```

### 1.2 `buf.gen.yaml`

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: backend/go/gen
    opt: paths=source_relative
  - remote: buf.build/connectrpc/go
    out: backend/go/gen
    opt: paths=source_relative
  - remote: buf.build/bufbuild/es
    out: frontend/src/gen
    opt: target=ts
  - remote: buf.build/connectrpc/es
    out: frontend/src/gen
    opt: target=ts
```

## 2. Proto 包结构

```
backend/proto/
├── buf.yaml
├── mtapi/                       # 外部依赖（已存在，从 /opt/alfq/gprc 复制）
│   ├── mt4.proto
│   └── mt5.proto
└── alfq/v1/
    ├── common.proto             # 通用消息（错误、分页、时间范围）
    ├── auth.proto               # 登录/刷新/2FA
    ├── tenant.proto             # 租户管理
    ├── user.proto               # 用户/角色/权限
    ├── broker.proto             # 经纪商配置
    ├── account.proto            # 交易账户
    ├── symbol.proto             # 品种
    ├── marketdata.proto         # 行情消息 + Stream
    ├── factor.proto             # 因子管理
    ├── strategy.proto           # 策略 CRUD/部署
    ├── order.proto              # 订单 + 内部 OMS
    ├── risk.proto               # 风控规则/事件
    ├── backtest.proto           # 回测任务
    ├── notify.proto             # 站内消息/告警
    └── audit.proto              # 审计
```

## 3. 通用规范

### 3.1 命名

- Service：`PascalCaseService`（如 `StrategyService`）
- RPC：动词在前（`CreateStrategy`、`ListOrders`、`StreamTicks`）
- 流式：`Stream<Resource>` 前缀
- 消息：`<RPC>Request` / `<RPC>Response`

### 3.2 通用消息（`common.proto`）

```proto
syntax = "proto3";
package alfq.v1;

import "google/protobuf/timestamp.proto";

message PageRequest {
  int32 page_size = 1;     // 默认 50，最大 500
  string page_token = 2;   // 不透明游标
}
message PageResponse {
  string next_page_token = 1;
  int32 total = 2;
}

message TimeRange {
  google.protobuf.Timestamp start = 1;
  google.protobuf.Timestamp end = 2;
}

enum Side {
  SIDE_UNSPECIFIED = 0;
  SIDE_FLAT = 1;
  SIDE_LONG = 2;
  SIDE_SHORT = 3;
}

enum OrderType {
  ORDER_TYPE_UNSPECIFIED = 0;
  ORDER_TYPE_MARKET = 1;
  ORDER_TYPE_LIMIT = 2;
  ORDER_TYPE_STOP = 3;
}
```

### 3.3 错误码

> **唯一源**：完整错误码枚举与 Connect Code 映射、重试矩阵、跨语言实现规则定义在 **doc 20**。本节仅说明 wire 格式与对接方式，**不再重复枚举值**。

Connect 标准 `connect.Code` + 业务错误详情（`ErrorDetail` 定义于 `backend/proto/alfq/v1/errors.proto`，详见 doc 20 §1.2）：

```proto
import "alfq/v1/errors.proto";

message ErrorDetail {           // 详见 doc 20 §1.2
  ErrCode code = 1;
  string  message = 2;
  map<string, string> meta = 3;
  string  trace_id = 4;
  google.protobuf.Timestamp at = 5;
  repeated string remediation = 6;
}
```

- **客户端**：解码 `connect.Error.details[]` 拿到 `ErrorDetail`，按 `code` 走 i18n（doc 20 §4）与重试逻辑（doc 20 §2）。
- **服务端**：业务层返回 `*errs.Error`（doc 20 §3.1），handler 出口统一 `errs.ToConnect(err)`，Connect Code 自动按 doc 20 §1.3 映射。
- **常见错误码示例**（完整清单见 doc 20 §1.2）：`ERR_UNAUTHENTICATED` / `ERR_PERMISSION_DENIED` / `ERR_TENANT_MISMATCH` / `ERR_VALIDATION_FAILED` / `ERR_RISK_REJECTED` / `ERR_BROKER_DISCONNECTED` / `ERR_BROKER_TIMEOUT` / `ERR_IDEMPOTENCY_CONFLICT` / `ERR_RATE_LIMITED` / `ERR_QUOTA_EXCEEDED`。

### 3.4 标准头

| Header | 必填 | 说明 |
|---|---|---|
| `Authorization: Bearer <jwt>` | 是（已登录接口） | 短期 JWT (15 min) |
| `Idempotency-Key` | 写操作必填 | UUIDv7，5 分钟内幂等 |
| `X-Trace-Id` | 可选 | 客户端传入或网关注入 |
| `X-Tenant-Id` | 可选 | 仅 super_admin 跨租户操作 |

## 4. 服务定义（对前端）

> 完整 .proto 由 AI Agent 实施时按本节签名生成；下面给关键 RPC 签名样例，Agent 必须覆盖所有 RPC。

### 4.1 AuthService（`auth.proto`）

```proto
service AuthService {
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc Refresh(RefreshRequest) returns (RefreshResponse);
  rpc Logout(LogoutRequest) returns (LogoutResponse);
  rpc VerifyTOTP(VerifyTOTPRequest) returns (VerifyTOTPResponse);
  rpc EnrollTOTP(EnrollTOTPRequest) returns (EnrollTOTPResponse);
  rpc Me(MeRequest) returns (MeResponse);
}

message LoginRequest {
  string email = 1;
  string password = 2;
  string tenant = 3;            // 租户 slug，可选（多租户登录页）
}
message LoginResponse {
  string access_token = 1;      // JWT 15min
  string refresh_token = 2;     // 7d
  bool   require_totp = 3;
  string totp_challenge = 4;
}
```

### 4.2 AccountService

```proto
service AccountService {
  rpc CreateAccount(CreateAccountRequest) returns (Account);
  rpc UpdateAccount(UpdateAccountRequest) returns (Account);
  rpc DeleteAccount(DeleteAccountRequest) returns (Empty);
  rpc GetAccount(GetAccountRequest) returns (Account);
  rpc ListAccounts(ListAccountsRequest) returns (ListAccountsResponse);
  rpc ConnectAccount(ConnectAccountRequest) returns (ConnectAccountResponse);  // 触发 mtapi Connect
  rpc DisconnectAccount(DisconnectAccountRequest) returns (Empty);
  rpc StreamAccountStatus(StreamAccountStatusRequest) returns (stream AccountStatusEvent);
}
```

### 4.3 StrategyService

```proto
service StrategyService {
  rpc CreateStrategy(CreateStrategyRequest) returns (Strategy);
  rpc UpdateStrategy(UpdateStrategyRequest) returns (Strategy);
  rpc GetStrategy(GetStrategyRequest) returns (Strategy);
  rpc ListStrategies(ListStrategiesRequest) returns (ListStrategiesResponse);
  rpc DeployStrategy(DeployStrategyRequest) returns (Deployment);    // 部署到账户
  rpc StopDeployment(StopDeploymentRequest) returns (Deployment);
  rpc StreamDeploymentEvents(StreamDeploymentEventsRequest) returns (stream DeploymentEvent);
}
```

### 4.4 OrderService

```proto
service OrderService {
  rpc PlaceOrder(PlaceOrderRequest) returns (Order);    // 手工下单（仅 trader 角色）
  rpc CancelOrder(CancelOrderRequest) returns (Order);
  rpc GetOrder(GetOrderRequest) returns (Order);
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse);
  rpc StreamOrders(StreamOrdersRequest) returns (stream OrderEvent);
}
```

### 4.5 MarketDataService

```proto
service MarketDataService {
  rpc GetQuote(GetQuoteRequest) returns (Quote);
  rpc GetBars(GetBarsRequest) returns (GetBarsResponse);     // 历史 K 线
  rpc StreamQuotes(StreamQuotesRequest) returns (stream QuoteEvent);   // SSE
  rpc StreamBars(StreamBarsRequest) returns (stream BarEvent);
}
```

### 4.6 RiskService

```proto
service RiskService {
  rpc CreateRule(CreateRuleRequest) returns (RiskRule);
  rpc UpdateRule(UpdateRuleRequest) returns (RiskRule);
  rpc ListRules(ListRulesRequest) returns (ListRulesResponse);
  rpc KillSwitch(KillSwitchRequest) returns (KillSwitchResponse);  // 二次验证
  rpc StreamRiskEvents(StreamRiskEventsRequest) returns (stream RiskEvent);
}
```

### 4.7 BacktestService

```proto
service BacktestService {
  rpc StartBacktest(StartBacktestRequest) returns (Backtest);
  rpc GetBacktest(GetBacktestRequest) returns (Backtest);
  rpc ListBacktests(ListBacktestsRequest) returns (ListBacktestsResponse);
  rpc StreamBacktestLogs(StreamBacktestLogsRequest) returns (stream LogLine);
}
```

### 4.8 FactorService

```proto
service FactorService {
  rpc CreateFactor(CreateFactorRequest) returns (Factor);
  rpc UpdateFactor(UpdateFactorRequest) returns (Factor);
  rpc ValidateExpression(ValidateExpressionRequest) returns (ValidateExpressionResponse);
  rpc ApproveFactor(ApproveFactorRequest) returns (Factor);    // 需 factor.approve 权限
  rpc PreviewFactor(PreviewFactorRequest) returns (PreviewFactorResponse);
  rpc ListFactors(ListFactorsRequest) returns (ListFactorsResponse);
}
```

### 4.9 TenantService / UserService / AuditService / NotifyService

参考相同模式，由 AI Agent 按 02 数据库设计的表结构补全 CRUD + 必要的列表/流式 RPC。

## 5. 内部服务间 RPC（gRPC）

### 5.1 RiskCheck（OMS → risk-svc）

```proto
service RiskCheckService {
  rpc Check(CheckRequest) returns (CheckResponse);
  rpc OnFill(FillEvent) returns (Empty);
}
message CheckRequest {
  string tenant_id = 1;
  string account_id = 2;
  string strategy_id = 3;
  OrderRequest order = 4;
}
message CheckResponse {
  bool   approved = 1;
  string reject_code = 2;
  string reject_msg = 3;
}
```

### 5.2 BrokerAdapter（OMS → mtapi 封装层）

OMS 不直接调 mtapi proto，而是调内部 `BrokerAdapterService`，由 Adapter 层封装：

```proto
service BrokerAdapterService {
  rpc Submit(SubmitRequest) returns (SubmitResponse);
  rpc Cancel(CancelRequest) returns (CancelResponse);
  rpc Modify(ModifyRequest) returns (ModifyResponse);
  rpc QueryOrder(QueryOrderRequest) returns (Order);
  rpc StreamOrderUpdates(StreamOrderUpdatesRequest) returns (stream OrderUpdate);
}
```

好处：将来接 FIX / CTP / Binance 不动 OMS 代码。

## 6. SSE 客户端约定

前端：

```typescript
const transport = createConnectTransport({ baseUrl: "/api" });
const client = createClient(OrderService, transport);

for await (const evt of client.streamOrders({ accountId })) {
  // evt 是强类型 OrderEvent
}
```

服务器：用 `connect-go` 的 `ServerStream`：

```go
func (s *Server) StreamOrders(ctx context.Context, req *connect.Request[...], stream *connect.ServerStream[...]) error {
    // 订阅 NATS subject，发到 stream.Send()
}
```

**重连策略**：客户端 SSE 自动重连；服务端用 `last_event_id`（请求字段）做断点续传。

## 7. 鉴权 / 租户 Interceptor

`admin-api` 必须装载以下 Connect Interceptor（顺序）：

1. **OTel**：注入 trace
2. **Logger**：访问日志
3. **Recover**：panic 转 Internal
4. **Auth**：解析 JWT，注入 `user_id` / `tenant_id` / `roles` 到 context
5. **Tenant Guard**：拒绝跨租户资源访问
6. **Permission**：基于注解或显式检查
7. **Audit**：写敏感操作审计
8. **RateLimit**：Redis token bucket
9. **Idempotency**：写操作幂等校验

## 8. OpenAPI / 文档

`buf` 可生成 OpenAPI（`buf.build/community/google-gnostic-openapi`），但前端用 Connect 直接强类型不需要。仅给第三方集成时生成 OpenAPI。

## 9. 验收检查项

- [ ] `buf lint` 通过、`buf breaking` 与 main 分支对比无破坏
- [ ] `buf generate` Go + TS 代码可编译
- [ ] 所有写 RPC 支持 `Idempotency-Key`
- [ ] 所有流式 RPC 支持客户端断线重连
- [ ] 错误返回包含 `ErrorDetail`
- [ ] curl 用 Connect HTTP/JSON 协议可调通（无浏览器也能测）

---

## 附录 A. API 版本演进策略

### A.1 版本号体现于 proto package

- 包名 `alfq.v1`、`alfq.v2`，路径 `backend/proto/alfq/v1/`
- 一个服务可同时注册多个版本 handler
- 旧版本至少**保留 12 个月**后 sunset

### A.2 兼容性规则（同 v1 内）

**允许**（向后兼容）：
- 新增 message / 新增 enum value（放最后）
- 新增 service method
- 新增 optional field
- field 改名（保持 number）

**禁止**（破坏性 → 必须升 v2）：
- 删字段 / 改 field number / 改 field 类型
- 改 method 签名
- 删 enum value
- required → optional 之外的语义变更

CI 跑 `buf breaking --against '.git#branch=main'`。

### A.3 v1 → v2 共存策略

```
1. proto 新建 alfq/v2/，先全量复制 v1
2. 在 v2 中破坏性修改
3. 服务端同时实现 v1 + v2（v1 内部转发到 v2 业务逻辑或独立保留）
4. 新客户端默认 v2；老客户端继续 v1
5. v1 标 deprecated，文档与告警通知
6. 12 个月后 sunset，移除 v1
```

### A.4 弃用流程

- 在 proto 字段 / method 上加注释 `[deprecated]`
- 服务端响应 header：`Deprecation: true`、`Sunset: <RFC 8594 日期>`
- 用户面通知 + Status Page 公告
- 监控弃用 API 调用量，确保下降

### A.5 版本号 vs 服务版本

- API 版本（v1/v2）独立于服务镜像版本（1.2.3）
- 服务镜像可同时支持 v1 + v2

