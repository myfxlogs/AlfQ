# M0 启动指令（AI Agent 入口）—— **历史文档**

> 🗄️ **本文已归档**（2026-05-19）。M0 基建阶段于 2026-04 完成，项目当前阶段为 **M6.5+（架构合并后）**。
>
> **新加入的 Agent 请勿按本文顺序操作**：仓库已有完整代码（4 后端 + 1 前端服务）。直接看：
> 1. `AGENT.md` —— 项目身份与硬性规则
> 2. `docs/01-总体架构与技术决策.md` —— 当前架构
> 3. `docs/adr/0010-consolidate-services.md` —— 5 服务合并方案
> 4. `docs/adr/0011-single-host-production.md` —— 单机部署
> 5. `docs/handover/M6.5-handover.md` —— 最新交接状态
>
> 本文以下内容仅用于**理解项目演进史**，不再作为操作指令。

## 0. 本文你必须做的事

1. 读完本文（不要跳读）
2. 按 §3 顺序读完所有 M0 必读文档
3. 按 §5 拆分提交 5 个 PR，每个 PR 做一件事
4. 每个 PR 完成后运行 §6 验收命令，截图/日志附 PR
5. M0 全部完成后写 Handover 报告（按 docs/18 §A3.4），结束 M0

## 1. 你是谁

你是负责 **ALFQ 量化平台 M0 基建阶段**的 AI Agent。
- 角色：Architect + Backend Builder + DevOps（一人多角，单 Agent 顺序推进）
- 工作仓库：`/opt/alfq/`
- 当前状态：仅有 24 份主文档 + 8 份 ADR，**无任何代码**
- 你的成果：让仓库进入"可 `make dev-up` 跑通基础设施 + `buf generate` 出 stub + CI 全绿"的状态

## 2. M0 范围（不要超出）

✅ **要做**：
- Monorepo 三域目录骨架（`backend/` `research/` `frontend/` `deploy/` `configs/` `scripts/` `docs/`）
- `backend/proto/` buf 工程、`alfq/v1/common.proto` `errors.proto` 两份基础 proto
- `backend/go/` go.work、`internal/common/` 几个最基础包（errs、logger、config 占位）
- 一个**最小可启动**的 `trading-core` 服务（仅 health 端点 + Connect handler 装载）
- `research/` pyproject.toml + uv.lock + 空 `alfq_research` 包
- `frontend/` Vite + React + TS 空壳
- `deploy/docker-compose.yml`：PG / CH / Redis / NATS / Vault 五件套
- `Makefile` 顶层命令
- `.golangci.yml` / `ruff.toml` / `eslint.config.js` / `.gitignore` / `.editorconfig`
- `scripts/fetch-references.sh`（按 docs/13 §2 实现）
- `.github/workflows/ci.yml` 跑 lint + buf + 占位 test
- `.github/PULL_REQUEST_TEMPLATE.md`（按 docs/17 §6.2 + docs/18 §A3.2）
- `CODEOWNERS`（按 docs/19 §6）
- 三域 README.md + 顶层 README.md

❌ **不要做**（超出 M0）：
- 任何业务逻辑（订单、风控、因子、策略、行情）
- 数据库 schema 落地（仅占位迁移目录，DDL 留 M1）
- mtapi 集成
- 前端业务页面（仅 hello world）
- AI 助手
- 部署到生产单机（详见 ADR 0011）
- 性能/混沌测试

## 3. 必读文档（严格按顺序）

读完每篇做笔记到 `docs/notes/M0-reading-notes.md`，AI Agent 自用。

1. `docs/README.md` — 全局约定
2. `docs/企业级量化交易系统落地方案.md` — 总览
3. `docs/01-总体架构与技术决策.md` — 架构
4. `docs/12-AI-Agent实施指南.md` — **必须吃透 §3-§11，尤其 §3.5 复杂度上限、§3.6 七条纪律**
5. `docs/19-架构决策记录.md` + `docs/adr/0001-0008` — 已决策项，**不要改变**
6. `docs/03-API与接口规范.md` — proto 与 Connect
7. `docs/02-数据库设计.md` — 知道未来 schema 形状（M0 只建空目录）
8. `docs/13-参考项目研习指南.md` — 实施 fetch-references.sh
9. `docs/15-可观测性详细规范.md` § 1-3 命名（M0 占位）
10. `docs/16-测试与质量保证.md` § 11 CI 流水线
11. `docs/17-发布与变更管理.md` § 6 PR Checklist
12. `docs/18-AI-Agent工作流深化与策略助手.md` § A1-A6 — Task Card / 防漂移 / Handover
13. `docs/20-错误码与异常处理规范.md` — 实现 `errs` 包雏形

其他文档（11 / 14 / 21-24）M1+ 时再深读。

## 4. 关键全局约定（再强调一遍）

来自 `docs/README.md`：

- 三域：`backend/` `research/` `frontend/`
- Go 1.26+ / Python 3.12（uv）/ Node 20+ / TS 5.4+
- Connect RPC + SSE，**不**用 REST/WS
- proto 单一源 `backend/proto/alfq/v1/`
- 配置 YAML + Viper + fsnotify
- 日志结构化 JSON，必带 `trace_id`/`tenant_id`/`user_id`/`request_id`
- 时间统一 UTC
- 价格/金额用 `decimal`/`NUMERIC(20,8)`，**禁止**直接 float64 比较

来自 `docs/12 §3.5`：

- 单文件 ≤ 300 行（Go）/ 400（Py）/ 250（TS）
- 单函数 ≤ 50 行
- 圈复杂度 ≤ 10
- 严禁 `// nolint` 绕过

## 5. M0 PR 拆分（按顺序提交，每个 PR 独立可合）

> **关于 800 行口径**（docs/12 §1.3）：M0 基建期 PR 含较多装配/样板代码（buf 工程、Makefile、docker-compose、CI workflow、空壳目录等）。统计时**生成代码、配置 YAML、CI YAML、Dockerfile、空壳/占位文件不计入**；仅业务/工具 Go/TS/Py 源代码计入。任一 PR 若超 800 行业务代码请进一步拆分；M0 五个 PR 设计上均控制在 500 行业务代码以内。

### PR-1：仓库骨架与全局配置

**新增**：
```
.gitignore
.editorconfig
.gitattributes
README.md                      (顶层，简短指向 docs/)
LICENSE                        (Apache 2.0 占位，由用户最终确定)
Makefile                       (顶层，按 docs/12 §9)
backend/README.md
research/README.md
frontend/README.md
deploy/README.md
configs/README.md
scripts/README.md
scripts/fetch-references.sh    (按 docs/13 §2 完整实现，chmod +x)
docs/notes/.gitkeep
docs/tasks/.gitkeep
docs/adr/.gitkeep              (已有内容)
docs/runbook/.gitkeep
docs/postmortems/.gitkeep
docs/incidents/.gitkeep
docs/dr-runs/.gitkeep
references/.gitkeep            (gitignored 内容)
```

**.gitignore 必须包含**：
```
references/
*/node_modules/
*/dist/
*/.venv/
*.pyc
__pycache__/
.env
.env.local
*.log
backend/go/gen/
frontend/src/gen/
research/alfq_research/_gen/
```

**验收**：
- `tree -L 2` 输出符合三域结构
- `bash scripts/fetch-references.sh QuantDinger` 能克隆到 `references/`（一个示例验证）

### PR-2：Proto 工程与生成

**新增**：
```
backend/proto/buf.yaml
backend/proto/buf.gen.yaml
backend/proto/buf.lock
backend/proto/alfq/v1/common.proto      (Pagination/TimeRange/Money 等基础)
backend/proto/alfq/v1/errors.proto      (按 docs/20 §1.2 完整 enum)
backend/proto/alfq/v1/health.proto      (HealthService.Check)
```

**Makefile 增**：
- `make proto-lint` → `buf lint && buf breaking --against ...`
- `make proto-gen` → `buf generate`

**验收**：
- `make proto-lint` 通过
- `make proto-gen` 在 `backend/go/gen/` 与 `frontend/src/gen/` 产出文件
- 生成代码已 gitignore，不入仓库

### PR-3：Backend Go 工作区与最小 trading-core

**新增**：
```
backend/go/go.work
backend/go/go.work.sum
backend/go/internal/common/errs/         (按 docs/20 §3.1 雏形)
backend/go/internal/common/logger/       (zap + 结构化字段)
backend/go/internal/common/config/       (Viper + fsnotify)
backend/go/internal/common/health/       (HealthService impl)
backend/go/cmd/trading-core/main.go         (启动 Connect handler，仅注册 HealthService)
backend/go/cmd/trading-core/Dockerfile
.golangci.yml                            (按 docs/12 §3.5 配置)
```

**Makefile 增**：
- `make go-lint` / `make go-test` / `make go-build`

**验收**：
- `make go-lint` 通过
- `make go-build` 产物可执行
- 启动 trading-core，`curl localhost:9000/alfq.v1.HealthService/Check -H "Content-Type:application/json" -d '{}'` 返回 OK
- 单测覆盖率配置就位（即使无业务测试）

### PR-4：Research 与 Frontend 空壳

**research/**：
```
research/pyproject.toml          (uv 管理，依赖按 docs/12 §11 Python 白名单初始集)
research/alfq_research/__init__.py
research/alfq_research/_version.py
research/tests/test_smoke.py     (assert True)
ruff.toml                        (按 docs/12 §3.5)
```

**frontend/**：
```
frontend/package.json            (Vite + React + TS + Tailwind + shadcn 基础)
frontend/vite.config.ts
frontend/tsconfig.json
frontend/tailwind.config.ts
frontend/postcss.config.cjs
frontend/index.html
frontend/src/main.tsx
frontend/src/App.tsx             (Hello ALFQ)
frontend/eslint.config.js
```

**Makefile 增**：
- `make py-lint` / `make py-test`
- `make web-lint` / `make web-build`

**验收**：
- `uv sync && uv run pytest` 通过
- `pnpm install && pnpm build` 通过
- `pnpm dev` 起开发服务器看到 Hello

### PR-5：基础设施 docker-compose + CI

**deploy/docker-compose.yml**：
- postgres:16 + 健康检查
- clickhouse:24
- redis:7
- nats:2 + jetstream
- vault:1.15 (dev mode)
- minio (准备未来用)
- 网络 `alfq-net`，端口仅暴露 localhost

**Makefile 增**：
- `make dev-up` / `make dev-down` / `make dev-logs`

**新增**：
```
.github/workflows/ci.yml         (lint + proto + test，按 docs/16 §11 缩水版)
.github/PULL_REQUEST_TEMPLATE.md (按 docs/17 §6.2 + docs/18 §A3.2)
CODEOWNERS                        (docs/adr/ 双人 review；其他 owner 占位)
deploy/prometheus/rules/.gitkeep
deploy/grafana/dashboards/.gitkeep
```

**验收**：
- `make dev-up` 五件套全 healthy
- `make dev-down` 干净
- CI 在 PR 上跑通（lint + proto + test 三项绿）

## 6. 全局验收（M0 完成判据）

按顺序执行：

```bash
# 1. 仓库结构
tree -L 2 -I 'node_modules|.venv|gen|references' /opt/alfq

# 2. 参考项目克隆（取一个示例）
bash /opt/alfq/scripts/fetch-references.sh QuantDinger
test -d /opt/alfq/references/QuantDinger

# 3. proto 全套
cd /opt/alfq && make proto-lint && make proto-gen

# 4. Go 全套
cd /opt/alfq && make go-lint && make go-test && make go-build

# 5. Python 全套
cd /opt/alfq/research && uv sync && uv run pytest && uv run ruff check .

# 6. Frontend 全套
cd /opt/alfq/frontend && pnpm install && pnpm lint && pnpm build

# 7. 基础设施
cd /opt/alfq && make dev-up
docker ps --format '{{.Names}}\t{{.Status}}' | grep alfq
make dev-down

# 8. CI（在 PR 上跑通）
# (查看 GitHub Actions 全绿)
```

## 7. 防漂移自检清单（PR 描述必填）

每个 PR 描述顶部必须贴：

```markdown
## 约束摘要自检（M0）
- [ ] 文件大小 ≤ docs/12 §3.5 上限（Go 300 / Py 400 / TS 250）
- [ ] 单函数 ≤ 50 行
- [ ] 圈复杂度 ≤ 10
- [ ] 未引入 docs/12 §11 白名单外的依赖
- [ ] 未实现 M0 范围之外的功能（见 docs/M0-START.md §2）
- [ ] 三域目录结构未偏离 docs/12 §3.1
- [ ] 配置/秘钥处理符合 docs/07
- [ ] 错误码使用 errs 包，不裸字符串
- [ ] 无 // nolint 绕过
- [ ] 关联 ADR（如新增决策）
```

## 8. 遇到困难时

按以下顺序处理：

1. **决策不在文档**：先开 ADR PR（按 docs/19 §3-§4），评审通过后再实施
2. **文档之间冲突**：选**编号大**的（更新的）+ 在 PR 里明确指出冲突，建议后续修订
3. **不可解决的技术问题**：在 PR 里**清楚标注问题**与已尝试方案，用 Handover 报告交给人

**禁止**：
- 自行约定业务规则（必须问人或开 ADR）
- 跳过 docs/12 的复杂度/规模上限
- 复制 AGPL 项目（bbgo）代码

## 9. 完成 M0 后

写 `docs/handover/M0-handover.md`：

```markdown
# M0 Handover

## 已完成
- PR-1 ... PR-5（链接）

## 验收记录
（贴 §6 各步骤截图/输出）

## 已知差异
（与本文计划的偏离及原因）

## 下一步建议
- M1 第一张 Task Card（建议）：md-gateway mtapi 单连接 demo
- 优先读 docs/14 §1-3 + docs/08 §3 + docs/13 关于 gocryptotrader 的引导

## 风险与待办
（M0 留尾）
```

**M0 验收通过 = 仓库可在新机器上 5 分钟内启动 `make dev-up` + CI 全绿**。

完成 M0 后**等待人类批复**才进入 M1，不要自动越级。
