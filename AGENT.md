# AGENT.md — ALFQ

> 工作仓库 `/opt/alfq/` | M6.5+ 架构合并阶段 | 2026-05-19

## 项目身份

企业级量化交易平台（Go + Python），基于 MT4/5 mtapi gRPC 网关，多账户/多策略/多租户。外汇/贵金属/指数 CFD，分钟~小时 CTA/多因子。设计原则：**先抄后改**。

## 三域结构

`backend/`（Go 服务 + proto）| `research/`（Python 研究，uv）| `frontend/`（React SPA，pnpm）

- Go 1.25 / Python 3.12 / TS 6+ / Node 22 LTS（版本基线见 `docs/26 §2`）
- Proto 单一源 `backend/proto/alfq/v1/` → `buf generate` 出 Go/TS/Python stub
- 4 后端服务：trading-core:9000（admin-api+oms+risk）, md-gateway:9001, quant-engine:9002（factor+strategy）, assistant-svc:9003
- 1 前端服务：frontend:80（Nginx 静态托管 + `/api/` 反代到 trading-core）

## 硬性规则

**协议**：Connect RPC + SSE（Server Streaming）。禁止 REST 新接口、禁止 WebSocket。内部 gRPC，异步走 NATS JetStream。

**数据**：PG 17（主数据）+ ClickHouse 25（时序）+ Redis 8（缓存/锁）+ MinIO/S3（对象）+ Vault（秘钥）。版本以 `docs/26-依赖与版本管理规范.md` 为准。

**MT4 vs MT5**：两套完全独立的协议/平台，proto 定义、枚举语义、撮合模型均不可共用。`md-gateway`/`oms` 必须维护两套独立 mapper：`adapter/mt4/` 与 `adapter/mt5/`。详见 `docs/14-领域模型与交易规则.md` §3.4。

**安全红线**：用户 Python **不进生产**。生产仅 DSL + ONNX。sqlc 生成 SQL，不用 ORM。

**价格**：`NUMERIC(20,8)` / decimal，禁止 float64 直接比较。时间统一 UTC。

**日志**：结构化 JSON，必带 `trace_id` `tenant_id` `user_id` `request_id`。

**版本**：所有依赖、脚本、程序、语言版本号，**必须使用官网最新稳定版**。除非有明确的兼容性问题（API 不兼容 / 生态未跟进 / 数据格式不兼容 / 上游已知 regression），否则不得保留旧版本。每条豁免必须在 `docs/26-依赖与版本管理规范.md §4` 中列明原因和过期日期。详见该规范。

**部署形态**：**单机 docker-compose**。不引入 K8s/Helm/ArgoCD/Service Mesh/HPA/多副本。详见 ADR 0011（`docs/adr/0011-single-host-production.md`）+ `docs/11-部署与运维手册.md`。生产、staging 均为单机 compose；如需多机/K8s 必须先修订 ADR。

## 11 份 ADR（不可逆）

0001 Connect RPC+SSE · 0002 三域 monorepo · 0003 PG+CH+Redis · 0004 用户 Python 不进生产 · 0005 多租户 RLS + broker 物理隔离 · 0006 Vault 秘钥 · 0007 sqlc 不用 ORM · 0008 AI 助手 bounded tools · 0009 仅云端 LLM API（禁止本地大模型）· **0010 后端 7→4 服务合并（5 服务总架构）** · **0011 单机 docker-compose 生产部署（不用 K8s）**

新增决策 → `docs/adr/NNNN-<slug>.md`，编号单调递增。详见 `docs/19-架构决策记录.md`。

## 文档唯一源

不同文档冲突时，以下为权威：

| 主题 | 唯一源 |
|---|---|
| 订单状态机 | `docs/14-领域模型与交易规则.md` §3.1 |
| 全量错误码 | `docs/20-错误码与异常处理规范.md` §1.2 |
| 表索引（含后增 18 张） | `docs/02-数据库设计.md` §6.5 |
| 权限角色（5 业务 + 4 治理） | `docs/01-总体架构与技术决策.md` §2.6 |
| NFR（NFR ≥ SLO ≥ SLA） | `docs/01-总体架构与技术决策.md` §5 |
| 依赖白名单 | `docs/12-AI-Agent实施指南.md` §11 |
| 复杂度上限 | `docs/12-AI-Agent实施指南.md` §3.5 |
| Proto 包结构 | `docs/03-API与接口规范.md` §2 |
| Refresh Token 哈希 | sha256（`docs/05-多租户与权限设计.md`） |

冲突处理：选编号大的（更新的）+ PR 中指出。

## 复杂度硬上限（CI 强制）

| 维度 | Go | Python | TS |
|---|---|---|---|
| 单文件行数 | ≤300 | ≤400 | ≤250 |
| 单函数行数 | ≤50 | ≤50 | ≤50 |
| 圈复杂度 | ≤10 | ≤10 | ≤10 |
| 函数参数 | ≤5 | ≤5 | ≤5 |
| 嵌套深度 | ≤4 | ≤4 | ≤4 |

严禁 `// nolint`。PR ≤ 800 行业务代码（生成/YAML/CI/Dockerfile 不计入）。

## 工程纪律

1. 单一职责 — Handler 只编排，业务在 service
2. 接口驱动 — 跨边界先 interface
3. 代码生成优先 — RPC: buf / SQL: sqlc / 前端类型: buf
4. 三处下沉 — 重复 3 次 → `internal/common/`
5. 错误集中 — `errs` 包，禁裸字符串
6. 状态机外置 — 订单/连接等显式状态机
7. 零循环依赖 — CI 检测

## 编码要点

- **Go**：gofumpt+golangci-lint, zap 日志, `ctx` 首参, 禁 panic, `go test -race`
- **Python**：ruff+mypy strict, loguru, 类型注解强制
- **TS**：strict mode, 禁 any, TanStack Query + Zustand, Tailwind
- **通用**：Go snake_case / Py snake_case / TS kebab-case · 依赖白名单见 docs/12 §11 · 新增依赖需 ADR · 禁 AGPL 入仓

## 提交与 PR

Conventional Commits: `type(scope): subject`。分支: `feat|fix|chore|docs|refactor|test/<scope>`。main 保护，PR + 2 reviewer。PR 必带：关联文档、测试结果、风险评估。详见 `docs/12-AI-Agent实施指南.md` §3.2-§3.4。

## 当前阶段：M6.5+（架构合并后）

已完成 M0–M6.5：基建、proto、行情、因子+研究、策略+OMS、风控、AI 助手、灰度发布、架构合并。仓库现状：

- `backend/go/cmd/`：`trading-core` `md-gateway` `quant-engine` `assistant-svc` 四服务可编译运行
- `frontend/`：React SPA 已成型
- `research/`：Python SDK 已搭建
- `deploy/docker-compose.prod.yml`：单机生产编排已配置（PG17/CH25/Redis8）

历史里程碑详见 `docs/handover/M0-handover.md` … `M6.5-handover.md`。范围以最新 milestone 文档为准；M0-START.md 是历史指令（保留）。部署形态：单机 compose（详见 ADR 0011）。

## Makefile

```
make proto          # buf lint + breaking + generate
make build / test / lint
make go-lint / go-test / go-build
make py-lint / py-test
make web-lint / web-build
make dev-up / dev-down / dev-logs
make sec-scan       # govulncheck + trivy
```

## 禁止

- main 直接 push · force push 共享分支 · `--no-verify`
- REST 新接口（除 healthz/metrics）· WebSocket
- 用户 Python 进生产 · proto 不跑 buf breaking
- 硬编码秘钥 · Vault 路径入仓 · >100MB 入仓
- AGPL 代码复制 · 跨里程碑实施 · 凭常识决定安全/合规
