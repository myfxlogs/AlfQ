---
name: mt-account-binding
description: |
  MT4/MT5 交易账户绑定的全栈实现知识。涵盖前后端完整流程、Proto 契约、MT 网关连接、
  密码处理策略和安全考量。当需要实现、修改、审查 MT 账户绑定功能时使用，包括：
  (1) 新增 MT 账户绑定向导，(2) 经纪商搜索与服务器选择，(3) MT 网关连接校验，
  (4) 账户 CRUD 与状态管理，(5) 交易密码的安全存储策略
---

# MT 交易账户绑定

参考实现：`/opt/antclaw/emulator/anttrader`

## 核心架构

```
前端 (React/TypeScript → 或 Android/Kotlin)
  │  三步绑定向导：选择经纪商 → 输入凭据 → 确认绑定
  │  → Connect-RPC: /antrader.AccountService/CreateAccount
  ▼
后端 API Handler (connect-rpc)
  │  鉴权 → 校验 → 入库 → MT 连接测试 → 回滚或返回
  │
后端 Service 层
  │  连接 MT4/MT5 gRPC 网关 → AccountSummary → 填充字段
  │
数据层
  │  PostgreSQL: mt_accounts 表
  │  密码以明文存储（原因见下文）
  ▼
MT 网关 (mt4grpc / mt5grpc)
     gRPC: Connect(login, password, host, port)
     → AccountSummary() → balance/equity/margin/leverage/...
```

## Proto 契约

```protobuf
// 请求
message CreateAccountRequest {
  string login           = 1;  // MT 交易账号
  string password        = 2;  // MT 交易密码
  string mt_type         = 3;  // "MT4" | "MT5"
  string broker_company  = 4;  // 经纪商名称
  string broker_server   = 5;  // 服务器名称
  string broker_host     = 6;  // 服务器 host:port
}

// 响应
message Account {
  string id, user_id, login, mt_type, broker_company, broker_server, broker_host;
  string status, token, currency, account_type, alias, last_error;
  bool is_disabled, is_investor;
  double balance, credit, equity, margin, free_margin, margin_level, profit, profit_percent;
  int32 leverage;
  Timestamp connected_at, created_at, updated_at;
}

// 完整 RPC
service AccountService {
  rpc CreateAccount(CreateAccountRequest) returns (Account);
  rpc ConnectAccount(ConnectAccountRequest) returns (ConnectAccountResponse);
  rpc SearchBroker(SearchBrokerRequest) returns (SearchBrokerResponse);
  rpc ListAccounts/GetAccount/UpdateAccount/DeleteAccount/DisconnectAccount/ReconnectAccount;
  rpc VerifyTradePermission/UpdateTradingPassword;
}
```

## 前端流程

### 三步向导（3-step wizard）

**Step 1 — 选择经纪商**：
1. 选择 MT4 或 MT5 平台
2. 输入经纪商名称关键词 → 调用 `SearchBroker` RPC
3. 下拉选择公司 → 下拉选择服务器
4. 得到 `brokerHost`（服务器地址，如 `mt4-demo.roboforex.com:443`）

**Step 2 — 输入凭据**：
- 输入交易账号（login）和交易密码（password）
- 密码框使用 `type="text"`（非 password 类型，用户需确认）

**Step 3 — 确认提交**：
- 预览全部信息：经纪商、服务器、平台、账号、密码
- 点击「确认绑定」→ `createAccount()` → 成功跳转首页

### 核心前端代码路径

| 文件 | 职责 |
|---|---|
| `src/types/account.ts` | `Account`、`BindAccountRequest` 类型 |
| `src/client/account.ts` | `accountApi.create()` / `searchBroker()` |
| `src/hooks/useAccount.ts` | `createAccount()` 编排：调 API → 写入 store → toast |
| `src/stores/accountStore.ts` | Zustand: `addAccount` / `updateAccount` / `removeAccount` |
| `src/pages/accounts/BindAccount.tsx` | 三步向导 UI 组件 |
| `src/pages/accounts/components/AddAccountCard.tsx` | "+" 入口按钮 |

## 后端流程

### CreateAccount Handler（推荐路径）

```go
func (s *AccountService) CreateAccount(ctx, req) (*Account, error) {
    userID := interceptor.GetUserID(ctx)     // 1. JWT 鉴权

    account := &model.MTAccount{             // 2. 构造模型
        ID: uuid.New(), UserID: uid,
        Login: req.Login, Password: req.Password,
        MTType: req.MtType, BrokerCompany: req.BrokerCompany,
        BrokerServer: req.BrokerServer, BrokerHost: req.BrokerHost,
        AccountStatus: "connecting",
    }

    s.accountRepo.Create(ctx, account)       // 3. 先入库

    err := s.connManager.Connect(ctx, account)  // 4. MT 连接测试
    if err != nil {
        s.accountRepo.Delete(ctx, account.ID)   //    失败 → 回滚删除
        return nil, connect.NewError(...)
    }

    s.connManager.Disconnect(ctx, account.ID)   // 5. 断开测试连接
    return convertMTAccount(account), nil
}
```

### 关键设计：密码明文存储

MT 交易密码 **以明文存储** 在 `mt_accounts` 表，**不做任何加密/哈希**。

**原因**：
1. 连接 MT 服务器时，必须将原始密码以明文形式提交给 MT gRPC 网关
2. 后端无法用哈希值连接 MT 服务器
3. 加密存储（如 AES）只是把明文换成"密文 + 密钥"，密钥同样在服务器上，等于没加密
4. 增加一次加解密操作，徒增 CPU 负担，无安全增益

**补偿措施**：
- 传输层：所有 API 走 HTTPS/TLS，密码不会在网络中明文暴露
- 访问控制：`GetAccount` RPC 校验 `userID`，用户只能查看自己的账户
- 日志脱敏：密码字段不打入日志（`zap` 日志中不记录 password 字段）
- 数据库层面：`mt_accounts` 表的 SELECT 权限受限于应用账户，外部不可直接访问

**与用户登录密码的区别**：

| | 用户登录密码 (`users.password_hash`) | MT 交易密码 (`mt_accounts.password`) |
|---|---|---|
| 存储方式 | bcrypt 哈希 | 明文 |
| 用途 | 验证用户身份（比较哈希） | 转发给 MT 服务器 |
| 是否需要原文 | 不需要 | 必须 |

### BrokerService 搜索逻辑

- 经纪商/服务器列表通过 `SearchBroker` RPC 返回
- 数据来源：MT4/MT5 官方 `brokers.dat` 或内置配置
- 前端用 `companyName` + `access[]` 结构渲染选择器

### 账户状态机

```
connecting  →  connected  →  disconnected
     │              │              │
     └── error ←────┘              │
                                   │
                        disabled (is_disabled=true)
```

- `connecting`：创建中 / 连接测试中
- `connected`：MT 连接正常，实时数据流活跃
- `disconnected`：用户主动断开或会话过期
- `error`：连接失败
- `disabled`：用户禁用账户（不接收数据流）

## 与 AntClaw 项目的集成要点

1. **Proto 复用**：AntClaw 已有 `mt5_handler.go`，可直接复用 `AccountService` 定义
2. **Android 端**：AntClaw 用 Kotlin/Jetpack Compose，参考 `BindAccount.tsx` 的三步流程实现
3. **MT 网关**：anttrader 的 `mt4/mt5` 目录包含完整 gRPC 客户端，可直接拷贝
4. **ConnectionManager**：管理多账户连接池、自动重连、健康检查
5. **StreamService**：实时推送 AccountSummary 变更到前端（SSE/WebSocket）
