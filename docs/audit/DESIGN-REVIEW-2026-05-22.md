# ALFQ 全项目设计审查报告 — 设计缺陷与改进建议

> **审查日期**：2026-05-22（2026-05-22 二次核查修正）  
> **审查范围**：全栈（Go 5 服务 + React 前端 + Python 研究层 + 基础设施 + 工程化）  
> **发现总数**：Critical 12 / Major 14 / Minor 16（共计 42 条）  
> **审查方法**：全量代码扫描 + 项目结构分析 + 运行时健康检查 + 配置审计  
> **核查状态**：本文档已按真实仓库代码逐项核对；存在事实/根因偏差的条目已就地修正，以此版为 AI 修复落地的权威依据。  

---

## 目录

1. [P0 · Critical — 上线/安全阻塞（12 项）](#p0--critical--上线安全阻塞12-项)
2. [P1 · Major — 用户体验/工程化严重不足（14 项）](#p1--major--用户体验工程化严重不足14-项)
3. [P2 · Minor — 打磨不足（16 项）](#p2--minor--打磨不足16-项)
4. [优先级排序修复路线](#优先级排序修复路线)
5. [附录：模块审查详情](#附录模块审查详情)

---

## P0 · Critical — 上线/安全阻塞（12 项）

### CR-01 · CI lint 失败不阻塞、双 workflow 并存

**文件**：`.github/workflows/ci.yml`、`.github/workflows/test.yml`

**核查修正**：报告原稿称"整个 CI 没有任何 `go test`"不实——`test.yml` 已包含 `go-test` job（带 `-race -coverprofile`，覆盖率门禁 ≥40%）和 `python-test` job。真实缺陷有两条：

1. `ci.yml:29` 的 `go-lint` 设了 `continue-on-error: true`；`ci.yml:50` 的 `py-lint`、`test.yml:44` 的 `golangci`、`test.yml:65` 的 `frontend lint` 同样不阻塞。
2. `ci.yml` 与 `test.yml` 职责重叠（前者跑 build/lint，后者跑 test/lint/build），易漂移。

```yaml
# ci.yml go-lint job
- run: make go-lint
  continue-on-error: true  # ← P0：lint 失败不阻塞
```

**修复**：
- 删除上述 4 处 `continue-on-error: true`；
- 合并 `ci.yml` 与 `test.yml` 为单一 workflow（保留 test.yml 的 coverage gate）。

---

### CR-02 · 数据库密码硬编码

**文件**：`backend/go/internal/common/bootstrap/bootstrap.go:37`

```go
dsn = "postgres://alfq:alfq_dev@localhost:5432/alfq?sslmode=disable"
```

**影响**：任何能读到源码的人（包括 GitHub Actions 日志、外部贡献者、供应链节点）都知道生产数据库凭据。`Makefile` 的 `db-migrate` 同样裸写 `PGPASSWORD=alfq_dev`。

**修复**：默认值只能使用占位符（如 `"postgres://localhost:5432/alfq?sslmode=disable"`），实际凭据必须来自环境变量或 Vault。

---

### CR-03 · 前端 Token 存 localStorage — XSS 风险

**文件**：`frontend/src/api/client.ts:20-30`

```typescript
export function getToken(): string | null {
  return localStorage.getItem("alfq_token");
}
```

**影响**：JWT access token、refresh token、过期时间全存 `localStorage`。任何 XSS 注入（npm 供应链攻击、第三方 CDN 脚本）都能直接读取 token，完全绕过 CSP nonce 保护。

**修复**：迁移到 httpOnly secure SameSite cookie。需要后端配合：login 返回 `Set-Cookie` header，前端不再手动管理 token。

---

### CR-04 · Vault 密钥未迁移——业务代码不读 Vault

**文件**：`deploy/docker-compose.prod.yml` assistant-svc 环境变量

```yaml
environment:
  OPENAI_API_KEY: ${OPENAI_API_KEY:-}
  ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
```

ADR-0006 规定所有 secret 走 Vault。RC06 修复了 Vault 自身的持久化，但 `assistant-svc` 仍从 `.env` 读取 LLM API key。`internal/common/vault/` 包存在但业务代码 **0 处真读 Vault**。

**修复**：`assistant-svc` 启动时从 Vault 拉取 API key（`vaultClient.LoadSecrets(ctx)`），不再读 `.env` 业务 secret。

---

### CR-05 · 容器无资源限制

**文件**：`deploy/docker-compose.prod.yml`（全部 14 个服务）

所有服务均无 `deploy.resources.limits` / `mem_limit` / `cpus` 定义。

**影响**：ClickHouse（时序数据）或 NATS（JetStream 持久化）内存泄漏或 CPU 飙升可直接拖死整机，无 Docker OOM killer 控制粒度。

**修复**：每个服务至少加 `deploy.resources.limits.memory` 和 `cpus`，先根据当前实测值设定。

---

### CR-06 · 观测栈缺 healthcheck

**文件**：`deploy/docker-compose.prod.yml`

**核查修正**：报告原稿暗示"监控面板零健康检查"过于宽泛——经核对，`postgres / clickhouse / redis / nats / trading-core / md-gateway / quant-engine / assistant-svc / backtest-runner / frontend` 均已配置 `healthcheck`。**真正缺失** healthcheck 的仅为：`prometheus / grafana / loki / tempo / vault`。

**影响**：观测/密钥栈挂掉时 Docker 不会重启，`depends_on: condition: service_healthy` 也无法依赖。

**修复**：为上述 5 个服务添加 `healthcheck`：
- Prometheus: `wget -qO- http://localhost:9090/-/healthy`
- Grafana: `wget -qO- http://localhost:3000/api/health`
- Loki: `wget -qO- http://localhost:3100/ready`
- Tempo: `wget -qO- http://localhost:3200/ready`
- Vault: `wget -qO- http://localhost:8200/v1/sys/health` 或 `vault status`

---

### CR-07 · pyproject.toml 第二段 `dependencies` 位置错乱

**文件**：`research/pyproject.toml:36-50`

**核查修正**：报告原稿声称"PEP 621 后者覆盖前者，core 依赖被静默丢弃"是**根因误判**。实测：line 33 的 `[tool.hatch.build.targets.wheel]` 之后，line 36 的 `dependencies = [...]` 在 TOML 解析上归属于该 hatch 表（而非 `[project]`），并不会覆盖 line 12 的 `[project].dependencies`。`uv sync` 实际安装的就是 line 12 的列表（含 numpy/polars/pandas），所以构建并非"巧合"成功。

但这段 `dependencies` 对 hatch 而言是未识别字段，会被忽略——属于死配置 + 维护陷阱（未来若有人误以为它生效会踩坑）。

**修复**：直接删除 `research/pyproject.toml:36-50` 整段，无需合并。

---

### CR-08 · AdminSettings 绕过 Auth Interceptor

**文件**：`frontend/src/pages/AdminSettings.tsx:3-8`

```typescript
const transport = createConnectTransport({ baseUrl: "/api" });
const settingsClient = createClient(SystemSettingsService, transport);
// ↑ 创建独立 transport，未挂 authInterceptor / sessionExpiryInterceptor
```

**核查修正**：报告原稿同时点名 `AdminBrokers.tsx`，但该文件**不存在于仓库**——`frontend/src/pages/` 下仅有 17 个文件，无 `AdminBrokers.tsx`。本条仅适用于 `AdminSettings.tsx`。

**影响**：访问 `#/admin/settings` 时调用 `GetSystemSettings` 不带 `Authorization` 头；后端返回 401 时前端也不会触发统一登出跳转。

**修复**：删除 `AdminSettings.tsx:3-8` 的自建 transport，改为 `import { settingsClient } from "../api/client"`。

---

### CR-09 · Go 健康检查是空壳——不检查后端依赖

**文件**：
- `trading-core/runner.go` — `w.Write([]byte("{\"status\":\"ok\"}"))`
- `quant-engine/runner.go` — `w.Write([]byte("ready"))`
- `md-gateway` / `assistant-svc` — 同样只返回 200

**影响**：服务启动后即使 PG/Redis/NATS/CH 全部断开，健康检查仍然返回 200。负载均衡器和 Docker healthcheck 无法检测实际故障。

**修复**：`/health` 应 ping 核心依赖（PG、Redis、NATS），返回各依赖状态。

---

### CR-10 · Go 测试覆盖大面积缺失

**文件**：`backend/go/internal/` 下多个包无 `_test.go`

以下是零测试覆盖的关键包：

| 包 | 源文件 | 影响 |
|---|---|---|
| `accountconn` | connector.go, sync_worker.go, types.go, mtapi_bridge.go | 账户连接/同步无测试 |
| `netimport` | 全部 | 网络层无测试 |
| `vault` | client.go | Vault 集成无测试 |
| `marketdataview` | 全部 | 统一数据接口无测试 |
| `risksvc` | breaker.go, kill.go, event.go | 风控关键路径无测试 |
| `oms` | reconciler.go | 订单状态机缺少关键路径 |

**修复**：至少为 `accountconn`、`oms`、`risksvc` 补充核心路径单元测试。

---

### CR-11 · 无 404 / 无"未授权"页面

**文件**：`frontend/src/App.tsx:166`

```tsx
const Page = isAdmin ? adminRoutes[hash] || Users : userRoutes[hash] || Dashboard;
```

未知用户路由 fallthrough 到 `Dashboard`，未知 admin 路由 fallthrough 到 `Users`；session 过期被 `sessionExpiryInterceptor` 拦截后直接 `location.hash = "#/login"`，用户不知道被踢的原因。

**影响**：用户体验差——打错 URL 没有任何反馈。安全事件（token 过期、权限不足）对用户不可见。

**修复**：新增 `<NotFound>` 路由（兜底）和 `<Unauthorized>` 页面；Login 跳转时携带 `?reason=expired|forbidden` 参数。

---

### CR-12 · `.env` 生产密钥暴露

**文件**：`deploy/.env`（虽然已在 `.gitignore`，但文件存在于本地磁盘）

包含 OPENAI_API_KEY、ANTHROPIC_API_KEY、POSTGRES_PASSWORD、CLICKHOUSE_PASSWORD 等至少 10 个生产密钥。

**影响**：`git add -f` 误操作即可提交。磁盘被读取即可泄露。

**修复**：移除 `.env` 中所有生产值，改为占位符 `change-me`。生产部署通过 `docker-compose` 的 `env_file` 指向外部安全路径（如 `/etc/alfq/secrets.env`）。

---

## P1 · Major — 用户体验/工程化严重不足（14 项）

### MR-01 · 前端无重试机制

**文件**：`frontend/src/api/client.ts`

ConnectRPC transport **无 retry interceptor**。网络抖动直接抛错误，用户看到白屏或一闪过。

**修复**：新增 retry interceptor，对 `Unavailable`/`DeadlineExceeded` 做指数退避重试（最多 3 次）。

---

### MR-02 · 前端无请求日志/遥测

所有 API 调用不记录耗时、状态码、trace ID。排查"首页加载慢"只能靠浏览器 DevTools。

**修复**：新增 logging interceptor，记录 `method + duration + statusCode`。生产环境发送到 Loki。

---

### MR-03 · Accessibility 近乎为零

17 个页面，无 `aria-label`、无键盘导航、无 focus trap。`<a>` 标签无 `href` 用 `onClick` 替代——屏幕阅读器完全不可用。

**修复**：至少为核心页面（Dashboard、Orders、Login）添加 `aria-label` 和 `role` 属性。所有交互元素确保键盘可达。

---

### MR-04 · RiskRules / Settings 硬编码假数据

`RiskRules.tsx` 和 `AdminSettings.tsx` 使用硬编码 JSON 数组，不读 API。

**影响**：前端展示的风控规则与后端 `risksvc` 的 10 条规则完全脱节。用户修改了设置看不到变化。

**修复**：从 API 获取实际数据（`brokerClient` / `settingsClient`）。

---

### MR-05 · 无全局状态管理

无 React Context / Zustand / Redux。token 直接从 `localStorage` 读。多页面间不共享用户信息、租户信息、账户列表——每页独立 fetch。

**修复**：引入轻量状态管理（React Context + useReducer 或 Zustand），管理 `auth` / `tenant` / `accounts` 状态。

---

### MR-06 · 研究-生产 DSL 算子覆盖不足

Python 有 19 个因子算子（SMA/EMA/WMA/STD/VAR/Min/Max/Sum/Ref/Delta/PctChange/ZScore/Rank/ATR/MACD/RSI...）。Go 侧只验证了 SMA/EMA/RSI 3 个的 bit-identical parity（`test_rs01_parity.py`）。

**影响**：16 个算子**未验证 Go-Py 一致性**。研究员用 Python 算出的 WMA 值，实盘 Go 侧可能不同——"回测漂亮实盘亏钱"的经典坑。

**修复**：扩展 `factor-golden` 生成全部 19 个算子的 golden JSON，接入 CI 的 parity gate。

---

### MR-07 · 无结构化日志 / OpenTelemetry Tracing

Go 服务用 `zap.NewDevelopment()`（console 彩色格式），无 JSON 结构化输出。Loki 很难按字段搜索。无 OpenTelemetry trace propagation——跨服务调用（backend → md-gateway → MT API）无法关联。

**修复**：生产模式切换到 `zap.NewProduction()`（JSON 格式）。引入 OpenTelemetry SDK 做 HTTP/gRPC trace propagation。

---

### MR-08 · Python 无 CLI 入口点

`pyproject.toml` 无 `[project.scripts]`。用户无法通过 `pip install -e .` 或 `uv run alfq` 使用 CLI。

**修复**：
```toml
[project.scripts]
alfq = "alfq_research.cli:main"
```

---

### MR-09 · 策略 Spec 无表达式验证

`StrategySpec.validate()` 只检查 `name`/`period`/`canonical_symbols` 是否存在，**不验证 factor 表达式是否合法 DSL**。

**影响**：`sma($closse, 20)`（拼写错误）静默通过验证，运行时 factor engine 才报错。

**修复**：在 `validate()` 中调用 DSL compiler 预编译所有 factor 表达式。

---

### MR-10 · NATS factor stream 不存在——因子数据不持久化

`subscriber.go:81` 向 `factor.sma20.EURUSDm` 等 subject 发布因子值，但 NATS 没有对应的 JetStream stream。`Publish` 失败只打 warn 日志（静默丢弃）。

**影响**：重启后历史因子值全部丢失。回测因子分析无法利用实时计算数据。

**修复**：创建 `FACTOR_VALUES` JetStream stream（Subjects=`factor.>`，Storage=File，MaxAge=7d）。

---

### MR-11 · 无 DB 迁移自动化工具

`Makefile` 的 `db-migrate` 硬编码两条 SQL 文件路径，无 goose/flyway/atlas 等版本迁移工具。

**影响**：开发环境 schema 可能与生产漂移。迁移历史不可追溯。

**修复**：引入 goose 迁移工具，迁移文件按序号管理。

---

### MR-12 · dev docker-compose 与 prod 严重不一致

`deploy/docker-compose.yml`（dev）和 `deploy/docker-compose.prod.yml` 差异巨大：
- dev 缺少 `restart` 策略
- dev 端口绑 `0.0.0.0`（prod 是 `127.0.0.1`）
- dev 缺少 minio、trading-core、quant-engine、assistant-svc 等服务

**修复**：将共享配置抽取为 `docker-compose.base.yml`，dev/prod 通过 override 合并。

---

### MR-13 · 无 API 文档

API 定义全在 `.proto` 文件中，没有自动生成的 OpenAPI/Swagger 文档页面。

**影响**：研究员/前端开发者需要读 proto 源码才能理解接口。onboarding 成本高。

**修复**：`buf generate` 增加 OpenAPIv3 插件 + Swagger UI 容器。

---

### MR-14 · 前端 ConnectRPC stub 生成链断裂

`frontend/src/gen/` 下的文件是手动维护的（git 中有修改记录），没有 `buf generate` 集成到前端构建流程。

**影响**：proto 更新后前端 gen 文件可能不同步，导致运行时类型错误。

**修复**：在 `package.json` 中添加 `buf generate` 脚本，`pnpm build` 前自动执行。

---

## P2 · Minor — 打磨不足（16 项）

| ID | 项 | 文件 | 修复 |
|---|---|---|---|
| MN-01 | ESLint `numFmt` 禁用——ESLint 不检查未使用变量 | `eslint.config.js` | 移除或启用 |
| MN-02 | `errs` package 定义了错误类型但无调用方——`fmt.Errorf` 遍地 | `internal/common/errs/` | 全局替换或删除 |
| MN-03 | Makefile 无 `prod-up`/`prod-down` 目标——生产部署靠手动 | `Makefile` | 新增 target |
| MN-04 | Grafana dashboard 目录为空 | `deploy/grafana/dashboards/` | 预置基础面板 |
| MN-05 | Prometheus rules 目录为空 | `deploy/prometheus/rules/` | 预置告警规则 |
| MN-06 | Vault `init.sh` 未被 compose 自动调用 | `deploy/vault/init.sh` | 集成到启动流程 |
| MN-07 | assistant-svc 跳过 Redis/NATS 但保留相关 stub | `assistant-svc/main.go` | 清理死代码 |
| MN-08 | backtest-runner Dockerfile 与 Go 服务 Dockerfile 完全不同 | `deploy/Dockerfile.backtest-runner` vs `Dockerfile.builder` | 统一构建 |
| MN-09 | quant-engine 硬编码 account UUID / strategy UUID——不支持多租户 | `cmd/quant-engine/main.go` | 改为从 spec 或配置读取 |
| MN-10 | dev compose 的 NATS 无 `-m 8222`——dev 无监控端口 | `deploy/docker-compose.yml` | 添加 monitoring 端口 |
| MN-11 | `tsconfig.node.json` 未被 `tsconfig.json` 引用 | `frontend/tsconfig.json` | 清理或引用 |
| MN-12 | `CODEOWNERS` 为空 | 根目录 | 填写责任分配 |
| MN-13 | `scripts/migrate.sh` 未被 Makefile 调用 | `scripts/migrate.sh` | 集成到 `db-migrate` |
| MN-14 | 29 个 docs + 14 个 ADR——无 index | `docs/` | 自动生成目录 |
| MN-15 | backtest-runner healthcheck 用 Python 实现——异于 Go 服务的 `wget` | `deploy/docker-compose.prod.yml` | 统一为 `wget` |
| MN-16 | `pnpm-lock.yaml` 和 `package-lock.json` 并存 | `frontend/` | 删除 `package-lock.json` |

---

## 优先级排序修复路线

### Phase 1 — 立即（~2h，阻塞上线）

| # | ID | 项 | 工作量 |
|---|---|---|---|
| 1 | CR-01 | CI 加 `go test` / 移除 `continue-on-error` | 10m |
| 2 | CR-02 | DB 密码从硬编码迁移到环境变量 | 30m |
| 3 | CR-03 | Token 从 localStorage 迁移到 httpOnly cookie | 4h（含后端改造） |
| 4 | CR-05 | 容器加 `resources.limits` | 30m |
| 5 | CR-08 | AdminSettings/AdminBrokers 使用共享 transport | 1h |
| 6 | CR-07 | pyproject.toml 合并 dependencies | 5m |

### Phase 2 — 本周（~6h）

| # | ID | 项 | 工作量 |
|---|---|---|---|
| 7 | CR-09 | 健康检查加入真实依赖 ping | 1h |
| 8 | CR-04 | assistant-svc 从 Vault 读 API key | 2h |
| 9 | CR-12 | `.env` 生产密钥清理 | 10m |
| 10 | MR-01 | 前端 retry interceptor | 30m |
| 11 | MR-07 | 结构化日志 + OTel trace | 2h |
| 12 | MR-04 | RiskRules/Settings 接入真实 API | 1h |

### Phase 3 — 本月

| # | ID | 项 | 工作量 |
|---|---|---|---|
| 13 | CR-10 | 补充核心包测试覆盖 | 3d |
| 14 | MR-06 | 扩展 DSL parity 到全部 19 算子 | 3d |
| 15 | MR-10 | NATS factor stream 创建 | 1h |
| 16 | MR-11 | goose 迁移工具集成 | 2h |

---

## 附录：模块审查详情

### A. 后端 Go 服务

| 服务 | 端口 | 依赖 | 测试覆盖 | 健康检查 |
|---|---|---|---|---|
| trading-core | 9000 | PG/Redis/NATS | 部分（adminapi 有集成测试） | 空壳 |
| md-gateway | 9001 | PG/Redis/NATS/CH | 部分（chmigrate 有测试） | 空壳 |
| quant-engine | 9002 | NATS/CH/PG | 无 | 空壳 |
| assistant-svc | 9003 | 外部 LLM API | 无 | 空壳 |
| backtest-runner | 9009 | CH | Python 侧有测试 | Python 实现 |

### B. 前端 React

> **核查修正**：原稿列出 17 条路由，含 `AdminBrokers / AdminTenants / AdminAPILogs / AdminAuditLogs / AdminRoles / Accounts` 等并不存在于代码的页面，已按 `App.tsx` 的实际路由表与 `frontend/src/pages/` 实际文件清单重写。

| 页面 | 路径 | Loading | Error | Empty |
|---|---|---|---|---|
| Dashboard | `#/` | ❌ | ❌ | ❌ |
| Login | `#/login` | ❌ | ❌ | N/A |
| BindAccount | `#/bind` | ❌ | ❌ | N/A |
| Orders | `#/orders` | ❌ | ❌ | ❌ |
| Positions | `#/positions` | ❌ | ❌ | ❌ |
| RiskRules | `#/risk` | ❌ | ❌ | N/A（硬编码数据） |
| Strategies | `#/strategies` | ❌ | ❌ | ❌ |
| Backtest | `#/backtest` | ❌ | ❌ | ❌ |
| AIChat | `#/assistant` | ❌ | ❌ | ❌ |
| Audit | `#/audit` | ❌ | ❌ | ❌ |
| Notifications | `#/notifications` | ❌ | ❌ | ❌ |
| Settings | `#/settings` | ❌ | ❌ | N/A |
| AccountDetails | `#/account/:id` | ❌ | ❌ | ❌ |
| Users（admin） | `#/admin/users` | ❌ | ❌ | ❌ |
| AdminSettings（admin） | `#/admin/settings` | ❌ | ❌ | ❌（绕过 auth） |
| ServiceManagement（admin） | `#/admin/services` | ❌ | ❌ | ❌ |

> 报告原稿提及的 Tenants.tsx 文件存在但**未注册路由**（dead code），需要纳入清理。

### C. Python 研究层

| 模块 | 源文件 | 测试覆盖 | DSL parity |
|---|---|---|---|
| `backtest/` | 6 | ✅ test_backtest.py, test_backtest_event.py | N/A |
| `cli/` | 2 | ❌ | N/A |
| `client/` | 3 | ❌ | N/A |
| `data/` | 2 | ❌（pg.py 无测试） | N/A |
| `factor/dsl/` | 3 | ✅ test_dsl_parity.py（3/19 算子） | 3/19 |
| `model/` | 2 | ✅ test_model_trainer.py, test_onnx_roundtrip.py | N/A |
| `spec/` | 1 | ❌（validate 无测试） | N/A |

### D. 基础设施

| 组件 | 健康检查 | 资源限制 | 数据持久化 |
|---|---|---|---|
| postgres | ✅ | ❌ | ✅ |
| clickhouse | ✅ | ❌ | ✅ |
| redis | ✅ | ❌ | ✅ |
| nats | ✅ | ❌ | ✅ |
| vault | ❌ | ❌ | ✅（file storage） |
| prometheus | ❌ | ❌ | ✅ |
| grafana | ❌ | ❌ | ✅ |
| loki | ❌ | ❌ | ✅ |
| tempo | ❌ | ❌ | ✅ |
| trading-core | ✅（空壳） | ❌ | N/A |
| quant-engine | ✅（空壳） | ❌ | N/A |
| md-gateway | ✅（空壳） | ❌ | N/A |
| assistant-svc | ✅（空壳） | ❌ | N/A |
| backtest-runner | ✅（Python） | ❌ | N/A |
| frontend | ✅ | ❌ | N/A |
