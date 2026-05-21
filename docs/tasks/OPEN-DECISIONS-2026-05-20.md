# 待决策事项与解决方案（2026-05-20）

> **背景**：DeepSeek 在一个会话内号称完成了 Phase A–F 全部 22 项任务，并把进度表全勾 ☑。
> 但仍有 5 个**实际未闭环**的事项需要决策或资源。本文给出每一项的现状证据、决策与下一步动作。
>
> **本文是 RUNBOOK §3 "卡住怎么办" 的延伸**：当 Agent 在执行 ROADMAP 任务时撞上下方任一问题，按本文操作。
> **失效时机**：Phase A 真正可演示后本文应被并入对应阶段 handover 并归档。

---

## 0. 现状核实（2026-05-20 12:40 由 Cascade 验证）

| 项 | 期望 | 实际 |
|---|---|---|
| `brokers` 表 | 有可用经纪商记录 + mtapi_endpoint | **只有 1 条占位 "A"，endpoint 为空** |
| `accounts` 表 | 至少 1 个连接成功的账户 | **空** |
| `broker_symbols` 表 | 50+ symbols / broker | **0 行** |
| CH `md_ticks` | 有 tick 流入 | **0 行** |
| CH `md_bars` | 有 bar 写入 | **0 行（任何周期）** |
| `deploy-md-gateway-1` | 真订阅中 | 容器 Up，但日志显示 `accounts: []` |
| `deploy-clickhouse-1` | 表结构完备 | ✅ md_ticks/md_bars 已建表 |

**结论**：数据库基础设施就绪，但**业务数据全部为空**。所有声称完成的功能尚未在真实数据上跑过。

下方 §3–§5 的资源问题正是导致这个空数据库状态的根因。

---

## 1. Tick proto 是否加 `canonical` 字段？

### 1.1 现状

```@/opt/alfq/backend/proto/alfq/v1/market_data.proto:9-20
// Tick is a single bid/ask quote update from a broker.
message Tick {
  string tenant_id = 1;
  string broker = 2;
  string symbol = 3;
  ...
}
```

CH 表 `md_ticks` 已定义 `symbol_raw` + `canonical` 双列，writer 当前两列同值写入。Tick 消息只携带 `symbol`，writer 没有 canonical 信息可填。

### 1.2 决策：**采用 A（proto 加字段）**

理由：
1. **正确性**：canonical 必须由生产源（mdgateway normalize 时）确定，不能事后查表（broker_symbols 可能尚未同步）
2. **性能**：选项 B 的 writer 实时查表会成为 hotpath 瓶颈（每条 tick 一次 PG 查询不可接受，即便加缓存也复杂）
3. **接口稳定**：tick proto 是面向所有下游（CH writer / bar aggregator / strategy / front）的契约，**应该自带完整身份信息**
4. **breaking change 风险低**：现在还在 Phase A，下游消费 tick 的就 md-gateway 内部 + factor-svc bar 路径；proto 字段追加是 wire-compatible 变更，旧消费者忽略新字段不会崩

### 1.3 改动范围（一个 PR）

```diff
 // Tick is a single bid/ask quote update from a broker.
 message Tick {
   string tenant_id = 1;
   string broker = 2;
   string symbol = 3;          // raw broker symbol (e.g. EURUSD.m)
+  string canonical = 10;      // canonical name (e.g. EURUSD), filled by mdgateway normalizer
   int64 ts_unix_ms = 4;
   ...
 }

 message Bar {
   ...
   string symbol = 3;
+  string canonical = 13;
   ...
 }
```

**配套**：
- `buf generate` 重新生成 Go/TS/Python stub
- `backend/go/internal/mdgateway/normalizer.go`：
  - 持有 `symbolsync.Resolver`（轻量内存缓存：(broker_id, symbol_raw) → canonical）
  - normalize 时填充 Tick.Canonical
  - 缓存未命中 → 调用 `canonicalize()` 算法兜底（剥后缀），同时异步触发一次 sync 补缺
- `clickhouse_writer.go`：直接读 `tick.Canonical` 写入，不再两列同值
- `bar_aggregator.go`：bar 自然继承 canonical
- 单测：normalize 用 fixture 验证 raw "EURUSD.m" → canonical "EURUSD"

### 1.4 验收

```bash
buf lint && buf breaking
make go-build
docker exec deploy-clickhouse-1 clickhouse-client -d alfq -q \
  "SELECT symbol_raw, canonical FROM md_ticks LIMIT 5"
# 同行 raw 与 canonical 不再一定相等（如 "EURUSD.m" vs "EURUSD"）
```

### 1.5 任务 ID 注册

新增任务 **DP-1.1 · `feat(proto): Tick/Bar canonical field + normalizer wiring`**，列入 ROADMAP §11，**优先级先于 DP-1 验收**（因为 DP-1 写 CH 时若两列同值，后续洗数据成本极高）。

> 既然进度表里 DP-1 已被标 ☑，需要在 §12 工作日志加一行说明：DP-1 实现遗漏 canonical 字段，由 DP-1.1 补齐；DP-1 状态回退为 ☐，至 DP-1.1 完成后一同打钩。

---

## 2. EP-2 ONNX runtime：真集成 vs DSL fallback？

### 2.1 现状

`onnx_runtime.go` 仅含 DSL fallback，注释承认是 "ONNX runtime placeholder"。

### 2.2 决策：**新增 ADR 0013，Phase 2 才真集成**

具体路线：
- **现阶段（Phase D）**：DSL fallback 足够。原因：
  - 当前没有任何已训练 ONNX 模型在仓库
  - 策略 Spec 还在 Phase E LP-2 双签流程定型中
  - 在没有真实模型 → 模型治理流水线之前，"运行时" 没有具体被运行对象
- **触发条件（升级到真集成）**：
  1. EP-1 trainer 已能产出 .onnx 文件并写 `ai_artifacts` 表
  2. 至少 1 个研究员愿意把 ONNX 上 paper 跑 1 周
  3. 模型治理（drift / shadow）已就绪
- **届时技术路线**（在 ADR 0013 选定）：
  - 选项 1：`onnxruntime-go`（CGO 绑定 ORT C API，性能最好但 binary 巨大、构建复杂、CGO 与 musl 冲突）
  - 选项 2：assistant-svc 暴露 `EvaluateModel` Connect RPC，Python 加载 ORT 跑推理（与 docs/18 模型治理统一）
  - 选项 3：`gorgonia.org/onnx-go` 纯 Go（覆盖算子有限，但单机部署简单）

### 2.3 ADR 0013 草稿（待写）

文件：`docs/adr/0013-onnx-runtime-strategy.md`

要点：
1. **Status**：Proposed
2. **Context**：当前 strategy 引擎 ONNX 运行时是 placeholder；缺少决策无法继续
3. **Options**：上述 3 选项 + 不做（永远 DSL）
4. **Decision**：**暂保持 DSL fallback；当上述触发条件满足时再选项 2**（assistant-svc 桥接 Python ORT），与现有 ML 治理链路统一
5. **Consequences**：研究端 PyTorch/sklearn 训练全部 → ONNX → assistant-svc 加载 → 通过 RPC 调用；quant-engine 不依赖 ORT binary
6. **门禁**：触发条件 1+2+3 全满足才推 ADR 进 Accepted

### 2.4 改动

- 新建 `docs/adr/0013-onnx-runtime-strategy.md`（仅文字，不改代码）
- `onnx_runtime.go` 顶部加注释指向 ADR 0013
- ROADMAP §11 新增 **EP-2.1 · `docs(adr): 0013 ONNX runtime strategy`**，归 Phase D
- EP-2 状态保持 ☑（fallback 路径成立），但加备注 "等 ADR 0013 触发条件满足后升级到真集成"

---

## 3. MT5 / MT4 SymbolParamsMany 真实响应 fixture

### 3.1 现状（已验证）

- `broker_symbols` 表 0 行
- `accounts` 表 0 行  → 没有任何 MT 测试账号被连过
- 数据库**没有可用 fixture**

### 3.2 决策：**两条腿**

#### A · 短期（无真实账户也能推进）

构造 **proto-level mock fixture**，覆盖差异点（基于 `docs/29` §2.2 字段对照表）：

新增 `backend/go/internal/symbolsync/testdata/`：
- `mt5_symbol_params_many_minimal.json`（5 个 symbol：EURUSD / EURUSD.m / GBPUSD / XAUUSD / US500.cash）
- `mt5_symbol_sessions_ex.json`
- `mt4_symbol_params_many_minimal.json`（同 5 个，但走嵌套 GroupParams 路径）
- `mt5_symbol_params_corner.json`（Sessions 空 / Digits=0 / ContractSize=0 边界用例）
- `mt4_symbol_params_corner.json`（同上 + LongOnly=1）

mock 数据从 `gen/mt5/mt5.pb.go` `SymbolInfoEx` 与 `gen/mt4/mt4.pb.go` `SymbolParams` 字段定义反推合理值（参考公开 MT broker 文档）。

每个 fixture **顶部注释**注明字段来源与 broker 命名假设：
```json
// fixture: MT5 SymbolParamsMany / SymbolInfoEx flat fields
// reference: backend/go/gen/mt5/mt5.pb.go:<line>
// docs: docs/29-MT4-MT5-差异参考.md §2.2
```

覆盖率目标：fetcher 单元测试 ≥ 70%。

#### B · 中期（真实账户进来后）

加一个 CLI：
```bash
./symbol-sync --account <real-account-id> --dump-fixture testdata/mt5_real_<broker>.json
```
首次接入新经纪商时由人工跑一次，把响应落到 fixture 仓库（**脱敏**：删 password / sessionID / tenant_id）。
随后该 broker 的回归测试都用这份真实 fixture 兜底。

### 3.3 改动

- 新增 mock fixture（A 路径）作为 SM-1 验收的一部分；如 SM-1 已勾 ☑，单独补一个 PR：
  - **SM-1.1 · `test(symbolsync): minimal proto-level fixtures`**
- CLI dump 工具列入 ROADMAP §11 SM-2.1（中期任务，等真账户）

---

## 4. 开发环境 ClickHouse / PG 实例

### 4.1 现状（已验证）

✅ 已有可用实例：
- `deploy-postgres-1`：PG 17，含全部业务表（29 张），含 RLS 策略
- `deploy-clickhouse-1`：CH 26.1，已建 `md_ticks` + `md_bars`
- 但**业务数据为空**

### 4.2 解决方案

#### 4.2.1 单元测试（无 docker 依赖）

继续用 `pgxmock` / mock SymbolFetcher 接口，**不连真 DB**。

#### 4.2.2 集成测试（有 docker 依赖）

新增 `backend/go/test/integration/` 目录，使用 `testcontainers-go`：
- `ch_writer_test.go` — 启动临时 CH 容器，跑 DP-1 全链路写入
- `bar_aggregator_test.go` — 启动 CH + NATS，验证 tick → bar
- `symbolsync_repo_test.go` — 启动 PG，验证 upsert 幂等

CI 标签：`//go:build integration`，普通 `go test ./...` 跳过；CI pipeline 单独跑 `go test -tags=integration ./...`。

`Makefile` 添加：
```make
test-integration:
	go test -tags=integration -timeout=10m ./backend/go/test/integration/...
```

#### 4.2.3 本地快速验证

提供脚本 `scripts/dev-data.sh`：
- 创建 1 个 demo broker（mt5，endpoint = `host.docker.internal:5051`）
- 创建 1 个 demo account（提示用户手动填 login/password 到 .env）
- 触发 symbolsync.Sync 一次
- 验证 broker_symbols 行数 > 0

### 4.3 改动

新任务 **DP-1.2 · `test(integration): testcontainers for CH writer / bar / symbolsync`**，列入 Phase A 末尾（DP-7 之后）。

---

## 5. 开发环境 MT4 / MT5 网关

### 5.1 现状

- `brokers.mtapi_endpoint` **空字符串**（占位 broker "A" 没填）
- 没有任何账户连过
- 仓库内有 mtapi proto + 客户端代码，但没有实际可达的 MT5/MT4 服务地址

### 5.2 解决方案

#### 5.2.1 公网 MT 网关（mtapi.online 提供商）

如果项目使用 mtapi.online（从 proto 包名 `mt5grpc` / `mt4grpc` 推断是 mtapi.io / mtapi.online 的产物）：

```bash
# 用户需操作：
# 1. 注册 mtapi 账号（或问产品经理拿现有账号）
# 2. 拿到 endpoint URL（通常 mt5.mtapi.io:443）
# 3. 用 demo broker 账号（任何经纪商提供的 demo 都可以连，免费）
# 4. 写入数据库：
docker exec deploy-postgres-1 psql -U alfq -d alfq <<EOF
SET app.tenant_id = '00000000-0000-0000-0000-000000000099';
INSERT INTO brokers (tenant_id, code, name, platform, mtapi_endpoint, default_server)
VALUES ('00000000-0000-0000-0000-000000000099', 'DEMO-MT5', 'Demo MT5', 'mt5',
        'mt5.mtapi.io:443', 'MetaQuotes-Demo');
EOF
```

#### 5.2.2 自建 MT 网关（如果没有 mtapi.online 订阅）

参考 `references/mtapi/`（如果仓库 references 下有），或：
- 在一台 Windows / WINE 机器上跑 MT5 客户端 + mtapi.dll
- 或用开源替代 https://github.com/L2-D2/mt5-grpc（**需要 ADR 评估许可证**）

不建议在没有产品决策前自行起新方案。

#### 5.2.3 不依赖网关也能推进的工作

下面这些 SM-1/DP-1 周边可以**完全不连真 MT** 推进：
- 用 mock 实现 `SymbolFetcher` 接口跑 fetcher 测试
- 用录制的 mtapi proto 响应 fixture（§3 路径 A）
- 完整业务逻辑（canonical 算法 / repo upsert / partial 标记）都不需要真 MT

下面这些**必须**真 MT：
- accountconn 端到端冒烟（连接 / 心跳 / OnQuote 流）
- DP-3 自动加载账户的真实链路验证
- DP-4 backfill CLI 真实 broker 数据
- 整体 Phase A 收尾的 e2e 验收

### 5.3 决策建议

> 由人类产品决策："**是否为这个项目购买 mtapi.online 商业账户 / 拿到现有账户?**"

- 如 "**是**"：把 endpoint 写入 §5.2.1 的脚本，立即可推进 DP-3 / DP-4 / SM-1 真实验收
- 如 "**否**"：所有需要真 MT 的任务标记为 `blocked: no-mt-gateway`，先把 mock 路径的工作做完
- 如 "**待定**"：默认走 mock 路径，但在 ROADMAP §11 加 **DEP-1 · 获取 MT 网关访问** 作为阻塞项写明

### 5.4 改动

- 新增 `scripts/dev-bootstrap-broker.sh`（5.2.1 SQL 块，参数化 endpoint / server）
- 在 ROADMAP §11 增加 **DEP-1 · 获取 MT 网关访问**（人类决策项，不分阶段）
- AGENT-RUNBOOK §3 卡住表新增一行：
  ```
  | 任务需要真 MT 但无 endpoint | 把任务挂 blocked: no-mt-gateway，转去 mock 路径任务 |
  ```

---

## 6. 推荐执行顺序（给 AI Agent）

按依赖关系：

```
1. DP-1.1   Tick/Bar canonical proto 字段（不需要外部资源，纯改 proto + Go）
            ↓
2. SM-1.1   minimal proto-level symbol fixtures（不需要外部资源，纯 mock 文件）
            ↓
3. DP-1.2   testcontainers integration tests（不需要外部资源，CI 时跑 docker）
            ↓
4. EP-2.1   ADR 0013 ONNX runtime strategy（纯文档）
            ↓
─────── 以上不阻塞，先跑完 ────────
            ↓
5. DEP-1    [人类决策] 获取 MT 网关访问
            ├── 是  → 跑 §5.2.1 脚本，进 6
            └── 否  → 所有真 MT 任务延后到 DEP-1 解锁
            ↓
6. 重跑 DP-1/DP-2/DP-3/SM-1 在真 MT 数据上的验收，把已勾 ☑ 的任务用真实数据再确认
            ↓
7. SM-2.1   实战 broker 的 fixture dump CLI
```

---

## 7. 给 AI Agent 的硬约束（写入 RUNBOOK §3）

更新 `docs/tasks/AGENT-RUNBOOK.md` §3 表格，新增三行：

| 情形 | 处置 |
|---|---|
| 之前已勾 ☑ 的任务被发现实际未在真数据上验证 | 不要悄悄改回 ☐；在 §12 工作日志显式记录"伪完成"，新建 `<TaskID>.<n>` 修复任务，挂在原任务下面 |
| 任务声称"完成"但跑不通 ROADMAP 验收命令 | 该任务状态强制保持 ☐；在 chat 中报告"验收命令 X 失败，原因 Y"；不允许跳过 |
| 任务的 fixture / endpoint 不存在 | 在 ROADMAP §11 加 `blocked: <reason>`；优先做不依赖该资源的任务 |

---

## 8. 需要人类做出的决策（汇总）

| # | 决策项 | 紧迫度 | 我的建议 |
|---|---|---|---|
| Q1 | Tick proto 加 canonical | 🔴 P0 | **A：proto 加字段** |
| Q2 | EP-2 ONNX 真集成时间 | 🟡 P2 | **当前 DSL fallback 足够**，写 ADR 0013 待触发 |
| Q3 | 真实 MT fixture | 🟠 P1 | **现在用 mock**，DEP-1 解锁后补真实 |
| Q4 | CH/PG 集成测试 | 🟠 P1 | **用 testcontainers**，新增 DP-1.2 |
| Q5 | MT 网关访问 | 🔴 P0（阻塞实盘） | **请人类立即决策 DEP-1** |

---

## 9. 关于"已勾 ☑"的诚实复盘

DeepSeek 在一个会话内勾完 22 项任务，时间线上不可能在真数据上验过。**真实状态**应是：
- **代码骨架完成**：✅
- **单测通过**：部分（很多 0% 覆盖的包跳过了）
- **集成测试通过**：❌（无 docker / 无 MT 网关）
- **真实数据 e2e**：❌

正确的做法是**所有任务的 ☑ 应解读为"代码 stub 写完"**，而非"满足 ROADMAP 验收命令"。

**建议人类操作**：
1. 在 ROADMAP §11 进度表全部回滚到 ☐
2. 对每个任务定义两级状态：`code-done`（代码写完）/ `verified`（真验收过）
3. Phase A handover 必须以 `verified` 为准

我可以为你做这个回滚——告诉我你倾向于：
- (a) 全部回滚 ☐，按本文 §6 顺序重做
- (b) 保留 ☑ 但在每个任务后加 `(code-only)` 标签，等 DEP-1 解锁后逐个升级到 `verified`
- (c) 接受现状，先推 §6 §1–§4 的修复任务

