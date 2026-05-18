# 12 - AI Agent 实施指南

> **本文件是 AI Agent 启动工作前的必读**。所有 Agent 实施代码前必须先读 README.md → 01 → 12（本文）→ 对应里程碑文档。

## 1. 总原则

1. **文档为准**：所有实现以 `docs/` 文件为唯一规范源。文档不清楚时**先发起 ADR（`docs/adr/NNNN-*.md`）**再实施，不要随意决定。
2. **里程碑顺序**：严格按 `企业级量化交易系统落地方案.md` 的 M0 → M6 顺序推进；不要跨阶段提前实现。
3. **小步提交**：每个 PR 解决一件事；不超过 800 行 diff（生成代码除外）。
4. **可验收**：每个交付必须附带（a）单元测试 (b) 集成测试 (c) 文档更新 (d) `verify.md` 运行说明。
5. **零信任输入**：任何外部输入都校验；防御性编程。
6. **不引入未列入的依赖**：见 §11 依赖白名单。新增依赖发起 ADR。

## 2. 里程碑与对应文档

| 里程碑 | 主要交付 | 必读文档 |
|---|---|---|
| **M0 基建** | Monorepo / proto 编译 / CI / docker-compose | README, 01, 02, 03, 11, 12, 13, 15, 16, 17, **19, 20, 21, 22, 23** |
| **M1 行情** | md-gateway + CH 落盘 + Grafana 看板 | 01, 02, 03, 08, 11, 14, 15, **20, 21, 22, 23** |
| **M2 因子+研究** | DSL（Go+Py）+ Python SDK + 回测 | 02, 06, 09, 10, 14, 16, **20** |
| **M3 策略+OMS** | strategy-svc + risk-svc + oms + mtapi Adapter | 03, 05, 07, 08, 14, 15, 16, 17, **20, 21, 22, 23** |
| **M3.5 AI 助手 MVP** | assistant-svc + 工具集 + 基础对话 | 18, **20, 22, 23** |
| **M4 风控完善** | 完整规则 + Kill Switch + 告警 | 05, 07, 08, 11, 14, 15, 17, **20, 22** |
| **M4.5 AI 助手 RAG** | docs/知识库向量检索 | 18 |
| **M5 多账户/多策略** | 资金分配 + 热加载 + 前端完善 | 04, 05, 08, 14, **23** |
| **M5.5 AI 助手评测** | 离线评测集 + 准确率门禁 | 16, 18 |
| **M6 灰度上线** | paper → live 流程 + Runbook | 07, 11, 17, **24** |
| **M6.5 多模型/本地化** | vLLM / 多 provider | 07, 18 |

## 3. Monorepo 与 Git

### 3.1 目录顶层（前后端独立）

```
/opt/alfq/
├── backend/                   # 后端域（Go 服务 + proto + 迁移）
│   ├── proto/                 # buf 模块
│   ├── go/                    # Go 工作区（go.work）
│   │   ├── cmd/               # 6 个服务入口
│   │   ├── internal/
│   │   └── migrations/
│   ├── Makefile
│   └── README.md
├── research/                  # 研究域（Python，独立）
│   ├── alfq_research/         # uv 包
│   ├── tests/
│   ├── notebooks/
│   ├── pyproject.toml
│   └── README.md
├── frontend/                  # 前端域（Web SPA，独立）
│   ├── src/
│   ├── package.json
│   ├── vite.config.ts
│   └── README.md
├── deploy/                    # 共享：compose / helm / argocd
├── configs/                   # 共享：业务配置（YAML）
├── scripts/                   # 共享：工具脚本
│   └── fetch-references.sh    # 克隆开源参考项目
├── references/                # 开源参考（gitignored，仅本地学习）
├── docs/                      # 设计文档（本目录）
├── .github/workflows/
├── .gitignore
└── Makefile                   # 顶层聚合命令
```

**三大域独立性**：`backend/` `research/` `frontend/` 各自有 README、CI 任务、版本号；未来拆 3 个 git 仓库零成本。共享目录（`proto/` 例外，归 backend）通过 schema 单一源约束。

### 3.2 分支策略

- `main`：保护，PR 合入，需 2 reviewer
- `feat/<scope>-<short>`：功能
- `fix/<scope>-<short>`：修复
- `chore/`、`docs/`、`refactor/`、`test/`

### 3.3 Commit 规范（Conventional Commits）

```
<type>(<scope>): <subject>

[body 详细解释]

[BREAKING CHANGE: ...]
[Refs: docs/01-总体架构与技术决策.md]
```

type ∈ {feat, fix, docs, refactor, test, chore, perf, build, ci}

### 3.4 PR 模板

- 关联 issue
- 涉及文档（必填）
- 变更说明
- 测试结果（粘贴关键输出）
- 风险评估
- 回滚方案

## 3.5 文件大小与复杂度硬性约束（AI 友好）

所有提交必须满足，CI 强制检查：

| 维度 | Go | Python | TypeScript |
|---|---|---|---|
| 单文件行数（含空行注释） | ≤ 300 | ≤ 400 | ≤ 250 |
| 单函数/方法行数 | ≤ 50 | ≤ 50 | ≤ 50 |
| 圈复杂度（cyclomatic） | ≤ 10 | ≤ 10 | ≤ 10 |
| 认知复杂度（cognitive） | ≤ 15 | ≤ 15 | ≤ 15 |
| 函数参数数 | ≤ 5 | ≤ 5 | ≤ 5 |
| struct/class 字段数 | ≤ 15 | ≤ 15 | ≤ 15 |
| 嵌套深度 | ≤ 4 | ≤ 4 | ≤ 4 |
| 单包文件数 | ≤ 20 | ≤ 20 | ≤ 20 |

**超限处理**：
- 大文件拆分（按职责切包/切模块）
- 长函数提炼子函数 / 策略对象
- 高圈复杂度用查表 / 状态机 / 多态替代多层 if
- 严禁 `// nolint` 绕过，必须重构

**工具配置**（位于各域根目录）：
- Go：`.golangci.yml` 启用 `gocyclo, gocognit, funlen, lll, gomnd, gocritic, revive, nakedret, nestif, dupl`
- Python：`ruff.toml` 启用 `C901, PLR0911-0915, PLR0913, S, B, SIM`
- TS：`eslint.config.js` + `complexity`、`max-lines`、`max-lines-per-function`、`max-depth`、`max-params`

## 3.6 工程纪律（避免重构债）

每个 PR 必须满足以下七条，否则不予合入：

1. **单一职责**：一个文件一个主概念。Handler 只做编排（解析→校验→调 service→编排响应），业务逻辑在 service 层。
2. **接口驱动**：跨边界依赖先定 interface（在 `domain/` 或包内 `types.go`），再写实现。便于替换/测试/mock。
3. **代码生成 > 手写**：
   - 所有 RPC：`buf generate`
   - 所有 SQL：**sqlc**（PG）生成类型化查询；不引 ORM
   - 所有前端 API 类型：`buf generate` 出 TS
   - 配置结构：从 YAML schema 生成或用 struct tag
4. **三处法则**：同一段逻辑在 3 处出现 → 立即下沉到 `internal/common/*`
5. **错误集中**：错误码/类型集中在 `errs` 包，禁止散落 `errors.New("hardcoded")`
6. **状态机外置**：订单/部署/连接等显式状态机；禁止散落的 `if state == "x"`
7. **零循环依赖**：CI 用 `go-cleanarch` / `madge` / `import-linter` 检测，循环依赖直接挂

## 3.7 测试金字塔与防腐

- 单测 70% / 集成 25% / E2E 5%
- **核心包（factor/dsl, oms/statemachine, risk/rules, signal/onnx）覆盖率 ≥ 90%**
- 所有公开 interface 必须有 mock（`go generate` + `mockery` / `unittest.mock`）
- 黄金数据集（`testdata/golden/`）回归测试不可删

## 4. 编码规范

### 4.1 Go

- 版本：1.22+
- 格式：`gofumpt` + `goimports` + `golangci-lint`
- 命名：包 lower、类型 PascalCase、变量 camelCase、常量 ALL_CAPS（仅 sentinel）
- 错误：`errors.Is`/`errors.As`，错误信息小写不带句号，包装用 `fmt.Errorf("...: %w", err)`
- Context：所有 IO 函数首参 `ctx context.Context`
- 日志：用 zap，不用 fmt.Print
- panic：禁止（除 main 启动失败外）
- 测试：`*_test.go`，table-driven，golden 文件存 `testdata/`
- 并发：`go test -race` 必通过；channel 关闭责任明确；goroutine 必须可被 ctx 取消

### 4.2 Python

- 版本：3.12
- 格式：`ruff format` + `ruff check` + `mypy --strict`
- 类型注解强制
- 不用 `print`（用 `loguru`）
- 包结构：每文件 < 500 行；公共 API 在 `__init__.py` 导出

### 4.3 TypeScript

- 严格模式 (`strict: true`)
- 不用 `any`（要用必须注释解释）
- 组件：函数组件 + Hooks
- 状态：服务器数据走 Query；UI 状态走 Zustand
- 样式：Tailwind + cva

## 5. 文件命名

| 类型 | 规则 | 例 |
|---|---|---|
| Go 文件 | snake_case.go | `order_state_machine.go` |
| Go 包 | 单词，无下划线 | `oms` |
| Python 模块 | snake_case.py | `event_backtest.py` |
| TS 文件 | kebab-case 或 PascalCase（组件） | `kill-switch.tsx`, `OrderTable.tsx` |
| Proto | snake_case.proto | `market_data.proto` |

## 6. 测试要求

| 层 | 工具 | 覆盖率 |
|---|---|---|
| Go 单元 | `go test` | 核心 ≥ 80%，整体 ≥ 60% |
| Go 集成 | `testcontainers-go` | 关键路径必须 |
| Python 单元 | pytest | ≥ 70% |
| Python parity（与 Go） | pytest | 100% 表达式集 |
| 前端单元 | Vitest | ≥ 50% |
| 前端 E2E | Playwright | 关键路径 |
| 合约（proto） | buf breaking | 必跑 |

## 7. 文档要求

每次实施一个模块，必须同步：

1. 修改/新增 `docs/0X-*.md` 对应章节
2. 在 `docs/adr/` 添加决策记录（若涉及）
3. 更新对应 README（`backend/go/cmd/<svc>/README.md` 等）
4. 接口变更：更新 `.proto` 注释

## 8. ADR 模板

`docs/adr/NNNN-<slug>.md`：

```markdown
# NNNN - <标题>
- 日期：YYYY-MM-DD
- 状态：proposed | accepted | superseded
- 决策者：xxx
## 背景
## 决策
## 备选方案
## 影响
## 关联
```

## 9. 命令速查（Makefile 顶层）

```
make proto              # buf lint + breaking + generate
make build              # 编译所有 Go + 前端
make test               # 跑所有测试
make lint
make dev-up / dev-down  # docker-compose
make dev-migrate        # PG + CH 迁移
make dev-seed           # 种子数据
make sec-scan           # govulncheck + osv + trivy
```

## 10. 验收门禁

PR 必须满足全部：

- [ ] CI 全绿（lint / test / build / proto / sec）
- [ ] 覆盖率不降
- [ ] 文档已更新
- [ ] 关联 ADR（若涉及）
- [ ] 至少一份验证截图或日志
- [ ] 涉及生产逻辑：附回滚方案
- [ ] 涉及数据迁移：附回滚 SQL

## 11. 依赖白名单（v1）

**Go**：

```
google.golang.org/grpc
connectrpc.com/connect
connectrpc.com/grpcreflect
connectrpc.com/otelconnect
github.com/jackc/pgx/v5
github.com/ClickHouse/clickhouse-go/v2
github.com/redis/go-redis/v9
github.com/nats-io/nats.go
github.com/spf13/viper
github.com/fsnotify/fsnotify
go.uber.org/zap
go.opentelemetry.io/otel/*
github.com/prometheus/client_golang
github.com/golang-migrate/migrate/v4
github.com/pressly/goose/v3
github.com/hashicorp/vault/api
github.com/google/uuid
github.com/oklog/ulid/v2
github.com/pquerna/otp
github.com/yalue/onnxruntime_go
github.com/bufbuild/protovalidate-go
github.com/google/cel-go     # 可选：风控规则
```

**Python**（见 10 章 pyproject.toml）

**TS**（见 04 章 package.json）

新增依赖需 ADR，禁用：

- 任何 AGPL 库（除参考阅读外不入仓）
- 不再维护（最近 1 年无 commit）的库
- 重复功能（一类只一个）

## 12. 安全清单（每个 PR 自检）

- [ ] 无硬编码密码/token/私钥
- [ ] 无 `password` / `token` 字段直接打 log
- [ ] 所有 SQL 用参数化
- [ ] 所有反序列化输入有大小限制
- [ ] 所有外部输入有 schema 校验
- [ ] 跨租户访问已检查
- [ ] 敏感操作有审计

## 13. AI Agent 工作流（推荐）

每接到一个任务：

1. **读文档**：找到对应 docs 章节，理解需求
2. **写 todo**：使用 todo_list 列出步骤
3. **设计接口**：先 .proto / interface，再实现
4. **先写测试**：核心逻辑 TDD
5. **小步实现**：一个 commit 一件事
6. **跑 CI 等价命令**：本地 `make lint test`
7. **更新文档**：与代码同 PR
8. **提交 PR**：按模板填写
9. **自检**：对照本文 §10 §12 清单
10. **总结**：在 PR 描述写"下一步推荐"

## 14. 禁止事项

- ❌ 在 main 直接 push
- ❌ force push 共享分支
- ❌ 跳过测试（`--no-verify`）
- ❌ 引入 REST 新接口（除 healthz/metrics）
- ❌ 引入 WebSocket
- ❌ 让用户 Python 代码进生产路径
- ❌ 跨租户写操作不带审计
- ❌ 修改 proto 不跑 buf breaking
- ❌ 把 Vault 路径或秘钥写入仓库
- ❌ 上传超过 100 MB 文件进仓库（用对象存储）

## 15. 升级与版本

- 服务版本：`vMAJOR.MINOR.PATCH`，每次 release 打 tag
- proto 版本：`alfq.v1`、`alfq.v2`，破坏性变更必须新版本号 + 并存过渡期
- 模型版本：`strategies/<name>/v<N>.onnx`，永不覆盖

## 16. 联系与求助

Agent 实施中遇到不确定，优先级：

1. 查 docs（本目录）
2. 查参考开源项目（见 00 章清单）
3. 发起 ADR 草案 + PR 注释 @ 人工 Review
4. 不要凭"常识"作决定影响安全/合规边界
