# AGENT.md — ALFQ

> 工作仓库 `/opt/alfq/` | M6.5+ 架构合并阶段 | 2026-05-19
>
> 🤖 **AI Agent 第一次进仓库**：读完本文后**立即跳到** `docs/tasks/AGENT-RUNBOOK.md`。
> 那份文件告诉你阅读顺序、执行循环、提交规范、卡住怎么办——是你的唯一执行入口。

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

**前后端职责**：所有业务计算在后端完成，前端仅负责展示和渲染。后端对前端零信任——所有输入必须独立验证，不可依赖前端校验。数字格式化、货币计算、状态推断、数据转换等逻辑一律在后端执行后返回最终展示值。

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
- 已知违规：assistant-svc `/chat` `/tools` REST 端点需迁移到 Connect RPC（见 `AIChat.tsx` TODO）
- 用户 Python 进生产 · proto 不跑 buf breaking
- 硬编码秘钥 · Vault 路径入仓 · >100MB 入仓
- AGPL 代码复制 · 跨里程碑实施 · 凭常识决定安全/合规

---

## 防偷懒约束（强制）

落地 `docs/tasks/*.md` / `docs/audit/*.md` 中任何卡片必须遵守以下 7 条；违反任一条即视为"假完成"，相关卡片自动降回 🅒，禁止再宣称完成。

### 1. 物证强制留痕

每条卡片必须在 `docs/handover/RS-final-verify.log` 留下 ≥20 行连续真实 stdout，含可复现的 UUID / 时间戳 / ticket 号 / 行数计数。文档仅改字、不带验收日志 = 失败。

所有验收命令统一以下列形式留痕：
```bash
<verify_cmd> 2>&1 | tee -a docs/handover/RS-final-verify.log
```

### 2. 验收命令禁止改动

plan / audit 里写的 `psql -c "..."` / `grpcurl -d ...` / `docker exec ...` 等验收命令是契约。Agent 若改命令使其更"好通过"（放宽 WHERE 条件、降低 COUNT 阈值、换更宽容的 SQL）= 整轮失败。

### 3. 重跑可复现（24h 窗口）

卡片完成提交后 24h 内，人类审查者随时挑任意 3 条已标 ☑ 的卡片，原样重跑其验收命令。要求：

- 涉及非时间窗 SQL（`COUNT(*) FROM risk_events` / `broker_ticket IS NOT NULL` 等）：结果必须仍非 0、且与 Agent 贴的数量级相符（±10%）；
- 涉及时间窗 SQL（`WHERE created_at > now()-interval N`）：必须仍为非 0（系统在线持续产出），数量不要求精确一致；
- 涉及 UUID/ticket 等具体值：原样 SELECT 必须仍能查到对应行。

任一条不满足 → 整轮作废、所有 ☑ 卡片全部降回 🅒。

### 4. 禁止 mock / stub 顶替真实依赖

凡涉及"broker 真实下单 / risk_events 真写表 / factor_values 真入 CH / vault 真读 secret / MT 网关真返回 ticket"，必须用真实容器、真实账户、真实 NATS/PG/CH。

```bash
# 仅检查生产路径，排除测试 / fixtures / broker_sim 等合法 stub
git diff --name-only HEAD~1 \
  | grep -E '^(backend/go/(cmd|internal)|research/src/alfq_research|frontend/src)/' \
  | grep -vE '_test\.go$|test_.*\.py$|/testutil/|fixtures/|broker_sim\.py' \
  | xargs -r grep -lE '\b(mock|stub|[Ff]ake[A-Z])' 2>/dev/null \
  && echo "FAIL: production code references mock/stub/Fake" && exit 1 || true
```

例外白名单（合法 stub，不计入违规）：`research/src/alfq_research/broker_sim.py`（RS04 设计就是模拟撮合）、`internal/testutil/`、proto 生成代码、Connect RPC interceptor 链中的 noop。

### 5. 禁止删 / 弱化测试

```bash
# 任意 _test.go / test_*.py 净行数为负 → 失败
git diff --stat HEAD~1 -- '*_test.go' 'test_*.py' | awk '/[0-9]+ -/{exit 1}'
```

新增功能必须配回归测试，否则该卡片不许标 ☑。`t.Skip()` / `pytest.skip` / `xfail` 数量不得净增。

### 6. 卡片状态机闭环检查

每个 `☑` 卡片必须满足：

- 有对应 commit hash（在卡片表格"备注"列写出短 sha）；
- commit 实际改动行数 ≥ 该卡片预估工作量的 30%（防"改一行注释就标完成"，行数按 `git show --stat` 统计，proto 生成代码不计入）；
- commit message 遵守现有 Conventional Commits 规则（line 96），并在 **commit body**（非 subject）中追加一行：
  ```
  Verify: docs/handover/RS-final-verify.log:<起始行>-<结束行>
  ```

对 audit 文档（`docs/audit/DESIGN-REVIEW-*.md`）的卡片，落地完成时把 heading 后追加 `[☑]` 标记，自检脚本据此统计完成度（见下条）。

### 7. 自检脚本卡口（落地前必跑全过）

以 `当前活跃的 plan/audit 文件` 为输入，下列每条非 0 退出即失败：

```bash
set -euo pipefail
PLAN=$(ls -t docs/tasks/REMEDIATION-PLAN-*.md docs/tasks/ROADMAP-*.md 2>/dev/null | head -1)
AUDIT=$(ls -t docs/audit/DESIGN-REVIEW-*.md 2>/dev/null | head -1)
test -n "$PLAN" && test -n "$AUDIT"

# (a) plan 中无未完成标记 / 待办占位
grep -cE '^🅒|TODO|FIXME|XXX-hack' "$PLAN" | awk '$1>0{exit 1}'

# (b) audit 文档卡片必须 100% 标完（heading 后追加 [☑]）
total=$(grep -cE '^### (CR|MR|MN)-[0-9]+' "$AUDIT")
done_n=$(grep -cE '^### (CR|MR|MN)-[0-9]+.*\[☑\]' "$AUDIT")
test "$total" = "$done_n" || { echo "AUDIT incomplete $done_n/$total"; exit 1; }

# (c) 验收日志行数下限（每卡片 20 行 × 卡片总数）
wc -l docs/handover/RS-final-verify.log | awk -v t="$total" '$1 < t*20 {exit 1}'

# (d) 关键运行时断言（非 0 行）— 时间窗 SQL 使用真实 paper-trade 数据
docker exec deploy-postgres-1 psql -U alfq -d alfq -tAc \
  "SELECT COUNT(*) FROM risk_events" | awk '$1==0{exit 1}'
docker exec deploy-postgres-1 psql -U alfq -d alfq -tAc \
  "SELECT COUNT(*) FROM orders WHERE state=7" | awk '$1==0{exit 1}'   # FILLED
docker exec deploy-postgres-1 psql -U alfq -d alfq -tAc \
  "SELECT COUNT(*) FROM orders WHERE broker_ticket IS NOT NULL" | awk '$1==0{exit 1}'
docker exec deploy-clickhouse-1 clickhouse-client -q \
  "SELECT COUNT(*) FROM factor_values WHERE created_at>now()-interval 1 hour" | awk '$1==0{exit 1}'

# (e) Vault 持久化（重启后 secret 仍可读 = 真 file backend）
docker exec deploy-vault-1 vault status | grep -q 'Storage Type *file'
```

任一行非 0 退出 → 自动撤回所有 ☑、降回 🅒，禁止 git tag、禁止提交 handover。

### 兜底原则

> **"代码已写"≠"卡片完成"。卡片完成 = 代码 + 测试 + 真实运行时 stdout 三者齐全且可复现。**

若卡片确实做不通（依赖缺失、broker 限制、设计错误等），坦诚降级写明"🅒 + 阻塞原因 + 已尝试方案"并停下汇报；**禁止用文档改字、mock 替换、放宽验收命令绕过**。
