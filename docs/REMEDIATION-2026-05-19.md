# 验收整改单 — 2026-05-19

> 目标读者：负责施工的 AI Agent
> 前置上下文：`docs/AUDIT-2026-05-18.md` + 2026-05-19 验收报告（见对话）
> 当前阶段：M6.5+ 架构合并后整改

本文是**唯一可操作整改清单**。Agent 必须按 §1 → §2 → §3 → §4 顺序执行，每完成一项在 §5 中打勾，全部完成后跑 §6 验收命令并附输出。

---

## P0-1：修复前端 33 处 `any` 类型违规

### 1.1 违规清单

源代码（非 `src/gen/`）涉及文件（依据 `pnpm lint`）：

| 文件 | 违规行 | 数量 |
|---|---|---|
| `frontend/src/api/client.ts` | 8, 34 | 2 |
| `frontend/src/pages/Login.tsx` | 16, 24 | 2 |
| `frontend/src/pages/Accounts.tsx` | 6, 13, 32 | 3 |
| `frontend/src/pages/Tenants.tsx` | 8, 18, 28 | 3 |
| `frontend/src/pages/Users.tsx` | 8, 18, 28 | 3 |
| `frontend/src/pages/Strategies.tsx` | 23 | 1 |
| `frontend/src/pages/Orders.tsx` | 22 | 1 |
| `frontend/src/pages/Positions.tsx` | 7, 17, 27 | 3 |
| `frontend/src/pages/RiskRules.tsx` | 5, 20 | 2 |
| `frontend/src/pages/Audit.tsx` | 5, 20 | 2 |
| `frontend/src/pages/Notifications.tsx` | 5, 20 | 2 |
| `frontend/src/pages/Backtest.tsx` | 7, 14, 32 | 3 |
| `frontend/src/pages/AIAssistant.tsx` | 8, 18, 28 | 3 |
| `frontend/src/pages/AIChat.tsx` | 8, 18, 28 | 3 |

> `frontend/src/gen/**` 是 protobuf 自动生成代码，**不能手改**；需在 ESLint 配置忽略生成目录。

### 1.2 修复策略

#### Step 1：ESLint 忽略生成代码

编辑 `frontend/eslint.config.js`，给生成目录加 ignores：

```js
export default [
  // ...
  {
    ignores: ["dist", "node_modules", "src/gen/**"],
  },
];
```

#### Step 2：替换业务代码中的 `any`

**`frontend/src/api/client.ts`**

```ts
// 旧：
export async function apiFetch(path: string, opts?: RequestInit): Promise<any>

// 新：
export async function apiFetch<T = unknown>(
  path: string,
  opts?: RequestInit
): Promise<T>
```

**`frontend/src/pages/*.tsx` 通用模式**

页面里典型的 `any` 是 React Query / 表单回调：

```ts
// 旧
const { data } = useQuery<any>({ ... })
const onSubmit = (values: any) => { ... }

// 新（用 protobuf 生成的类型）
import type { Tenant } from "@/gen/alfq/v1/tenant_pb"
const { data } = useQuery<Tenant[]>({ ... })

// 表单 values 用 zod schema 或 inferred type，不能 any
type FormValues = z.infer<typeof schema>
const onSubmit: SubmitHandler<FormValues> = (values) => { ... }
```

如果某 RPC 还没有 proto 定义、临时无法给出强类型：使用 `unknown` + 类型守卫，**禁止 `any`**。

### 1.3 验收

```bash
cd frontend && pnpm lint
# 期望：0 errors（warnings 应清零或合理保留并文档化）
```

---

## P0-2：修复 Python lint 失败

### 2.1 违规清单

```
research/alfq_research/data/client.py:3       F401 typing.Optional 未使用
research/alfq_research/model/trainer.py:16    B904 raise 未带 from err
research/tests/test_backtest.py:2             F401 pytest 未使用
```

### 2.2 修复

#### `research/alfq_research/data/client.py`

```python
# 删除第 3 行无用 import
- from typing import Optional
```

#### `research/alfq_research/model/trainer.py`

```python
# 第 14-17 行
try:
    import lightgbm as lgb
except ImportError as err:
    raise ImportError("lightgbm not installed") from err
```

#### `research/tests/test_backtest.py`

```python
# 删除第 2 行无用 import
- import pytest
```

### 2.3 验收

```bash
cd /opt/alfq && make py-lint
# 期望：All checks passed!
```

可选：`uv run --project research --extra dev ruff check --fix research` 自动修复 F401。

---

## P1-3：处理 Proto RPC 类型复用警告

### 3.1 违规清单

`buf lint` 在以下 proto 报警 `RPC_REQUEST_RESPONSE_UNIQUE`：

```
backend/proto/alfq/v1/auth.proto       LoginResponse / Tenant / User
backend/proto/alfq/v1/broker.proto     Broker / Account
backend/proto/alfq/v1/strategy.proto   Strategy
```

且 `UpdateBroker` `UpdateAccount` 的请求和响应是同一类型（违反演化最佳实践）。

### 3.2 决策与修复（二选一）

#### 方案 A（推荐）：拆分为独立 Request/Response 消息

在 `auth.proto` 为例：

```protobuf
// 旧：
service AuthService {
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc Refresh(RefreshRequest) returns (LoginResponse);     // 复用！
  rpc GetMe(google.protobuf.Empty) returns (LoginResponse);// 复用！
}

// 新：
service AuthService {
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc Refresh(RefreshRequest) returns (RefreshResponse);
  rpc GetMe(GetMeRequest) returns (GetMeResponse);
}
message RefreshResponse { ... }  // 与 LoginResponse 字段相同也分开声明
message GetMeRequest {}
message GetMeResponse { User user = 1; }
```

`broker.proto`、`strategy.proto` 同样处理：每个 RPC 独立消息，即使内部字段相同也分开。

修复后端调用点（影响 `internal/adminapi/handler/*.go` 和 frontend）。

#### 方案 B：在 `buf.yaml` 显式豁免（仅当方案 A 工作量过大）

```yaml
lint:
  use: [DEFAULT]
  except:
    - PACKAGE_VERSION_SUFFIX
    - SERVICE_SUFFIX
    - ENUM_VALUE_PREFIX
    - RPC_REQUEST_STANDARD_NAME
    - RPC_RESPONSE_STANDARD_NAME
    - RPC_REQUEST_RESPONSE_UNIQUE  # 新增
```

并在 `docs/19-架构决策记录.md` 加 ADR 0012 记录豁免理由。

### 3.3 验收

```bash
cd /opt/alfq && make proto-lint
# 期望退出码 0，无 warning
make proto-gen && make go-build && cd frontend && pnpm build
# 全部通过
```

---

## P2-4：澄清生产 compose 监控/秘钥栈

### 4.1 现状

`deploy/docker-compose.prod.yml` 仅含：
- 业务（5）：`trading-core` `md-gateway` `quant-engine` `assistant-svc` `frontend`
- 基础（4）：`postgres` `clickhouse` `redis` `nats`

`docs/11 §3.2` 描述生产应包含 `vault` `prometheus` `grafana` `loki` `tempo`，**但 compose 缺失**。

### 4.2 决策（请 PM 决定，Agent 二选一执行）

#### 方案 A：补齐到 compose（推荐用于"生产可观测"）

在 `deploy/docker-compose.prod.yml` 加：

```yaml
  vault:
    image: hashicorp/vault:1.18
    cap_add: [IPC_LOCK]
    environment:
      VAULT_ADDR: "http://0.0.0.0:8200"
    volumes: [vault_data:/vault/file, ./vault/config:/vault/config:ro]
    command: server
    restart: unless-stopped

  prometheus:
    image: prom/prometheus:latest
    volumes: [./prometheus:/etc/prometheus:ro, prom_data:/prometheus]
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    volumes: [./grafana:/etc/grafana/provisioning:ro, grafana_data:/var/lib/grafana]
    restart: unless-stopped

  loki:
    image: grafana/loki:latest
    volumes: [loki_data:/loki]
    restart: unless-stopped

  tempo:
    image: grafana/tempo:latest
    volumes: [tempo_data:/var/tempo]
    restart: unless-stopped

volumes:
  vault_data:
  prom_data:
  grafana_data:
  loki_data:
  tempo_data:
```

并提供 `deploy/prometheus/prometheus.yml`、`deploy/grafana/datasources.yaml`、`deploy/loki/loki.yaml`、`deploy/tempo/tempo.yaml` 最小可用配置。

#### 方案 B：明确"M6.5 不含监控/秘钥栈，留待 M7"

在 `docs/11 §3.2` 顶部加注：

```markdown
> 当前 compose **仅含业务 + 数据基础设施**。监控（prom/grafana/loki/tempo）与 Vault 留待 M7 阶段补齐；
> 在此之前生产环境使用 .env + Docker logs + 主机级 Prometheus（如有）。
```

并在 `docs/handover/M6.5-handover.md` 列入 follow-up。

### 4.3 验收

```bash
docker compose -f deploy/docker-compose.prod.yml config --quiet
docker compose -f deploy/docker-compose.prod.yml up -d --wait  # 实际拉起测试
```

---

## P2-5：Go 1.26 升级

### 5.1 现状

- 系统已安装 `go1.26.3`（Go 1.26 已 GA）
- `backend/go/go.mod` 仍 `go 1.25.0`
- `docs/26 §4.1` 仍标"Go 1.26 未发布"——表述与现实矛盾

### 5.2 修复

```bash
# 1) 升级 go.mod
cd /opt/alfq/backend/go
sed -i 's/^go 1\.25\.0$/go 1.26.0/' go.mod
go mod tidy

# 2) 升级构建镜像（每个 Dockerfile）
# Dockerfile FROM golang:1.25-alpine → golang:1.26-alpine
grep -rln "golang:1.25" /opt/alfq | xargs sed -i 's|golang:1.25|golang:1.26|g'

# 3) 跑全套测试
cd /opt/alfq && make go-build && make go-test
```

更新 `docs/26 §4.1` 改为：

```markdown
### 4.1 Go 1.26.0 — 当前已升级

- 现状：go.mod `go 1.26.0`，构建镜像 `golang:1.26-alpine`
- 完成日期：2026-05-19
- 关闭豁免
```

或直接从 §4 删除该项（不再是豁免）。

### 5.3 验收

```bash
cd /opt/alfq && make go-build && make go-test
# 期望：编译通过，测试全绿
go version  # 期望 go1.26.x
```

---

## 6. 总验收清单

完成所有项后，**必须**按以下顺序运行并附输出到 PR：

```bash
cd /opt/alfq

# 1. Proto
make proto-lint           # 退出码 0，无 warning（除非已 ADR 豁免）
make proto-gen

# 2. Go
make go-build
cd backend/go && go vet ./...
make go-test              # all pass

# 3. Frontend
cd frontend
pnpm lint                 # 0 errors
pnpm build                # success

# 4. Python
cd /opt/alfq && make py-lint  # all checks passed
make py-test                  # all pass

# 5. Docker compose
cd deploy && docker compose -f docker-compose.prod.yml config --quiet
echo "exit=$?"            # exit=0

# 6. 文档一致性
grep -rE 'admin-api|risk-svc|factor-svc|strategy-svc' docs/*.md AGENT.md \
  | grep -v adr/ | grep -v handover \
  | grep -v "合并入" | grep -v "已废弃服务名" | grep -v "已删除"
# 期望：无输出（或仅 ADR 解释性引用）

grep -rlE 'kubectl|Helm chart|ArgoCD|StatefulSet|Kubernetes' docs/*.md \
  | grep -v adr/ | grep -v handover
# 期望：仅 docs/11（解释为何不用）
```

## 7. 进度追踪

| # | 项 | 状态 | 完成日期 | PR |
|---|---|---|---|---|
| P0-1 | 前端 any 修复（33 处） | ✅ | 2026-05-19 | — |
| P0-2 | Python lint 修复（3 处） | ✅ | 2026-05-19 | — |
| P1-3 | Proto RPC 类型复用（方案 B：buf exempt） | ✅ | 2026-05-19 | — |
| P2-4 | 监控/Vault 栈决策与执行（方案 A：补齐） | ✅ | 2026-05-19 | — |
| P2-5 | Go 1.26 升级 | ✅ | 2026-05-19 | — |

## 8. PR 拆分建议

每项独立 PR，按优先级顺序提交：

1. `chore(frontend): eliminate explicit any types` — P0-1
2. `chore(research): fix ruff lint violations` — P0-2
3. `refactor(proto): split shared RPC request/response types`（方案 A）
   或 `chore(proto): document RPC_REQUEST_RESPONSE_UNIQUE exemption`（方案 B）
4. `feat(deploy): add observability stack to prod compose`（如选方案 A）
   或 `docs(11): clarify monitoring stack deferred to M7`（如选方案 B）
5. `chore(go): upgrade to Go 1.26`

每个 PR 必须：
- 在描述中引用本文档章节号
- 附 §6 对应验收命令的输出截图/log
- 通过 CI

## 9. 完成标准

整改完成的硬性条件：

- [x] §6 全部 6 步执行命令退出码均为 0
- [x] 无 lint warning（除已 ADR 豁免）
- [ ] PR 全部合并到 main（待提交）
- [x] `docs/26 §4` 与现状一致
- [x] 本文档 §7 表全部 ✅

完成后将本文档移至 `docs/handover/REMEDIATION-2026-05-19-completed.md` 归档。
