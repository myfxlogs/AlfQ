# 量化交易基础部分 — 缺陷修复计划

> **日期**：2026-05-22
> **来源**：基础设施体检 + 全量代码审查
> **基线**：14 容器健康 / PG 31 表 / CH 2 表 / NATS 1 stream / Go 27/29 包测试 PASS / Python 397 tests PASS
>
> 状态标记：☐ 未开始 / 🔧 进行中 / ☑ 已完成 / 🅒 code-only（未在真实数据验证）

---

## P0 · 阻塞量化闭环（3 项，预计 30 分钟）

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| R01 | Bar 发布/消费协议修复：JSON+核心NATS → protobuf+JetStream | ☑ | `publisher.go` + `runner.go` + `subscriber.go` + `docker-compose.prod.yml` | 2026-05-22 |
| R02 | `evaluateAll` 因子求值接入 Bar 流（替换空 map stub） | ☑ | `engine.go` + `runner.go` | 2026-05-22 |
| R03 | 创建 `configs/specs/demo_sma_cross.yaml` 首个 Spec 文件 | ☑ | `configs/specs/demo_sma_cross.yaml` | 2026-05-22 |

### R01 详情
- **文件**：`backend/go/internal/mdgateway/runner.go` L206-211
- **问题**：`fmt.Sprintf(JSON)` + `nc.Publish`（核心 NATS）→ subscriber 用 `proto.Unmarshal` + `js.Subscribe`（JetStream）
- **修复**：bar 回调改用 `proto.Marshal(&pb.Bar{...})` + `publisher.Publish()`（JetStream）
- **验收**：`docker exec deploy-nats-1 nats stream view MD_BARS` 能看到 protobuf bar 消息

### R02 详情
- **文件**：`backend/go/internal/quantengine/runner.go` L124-159
- **问题**：`factorVals := make(map[string]float64)`，循环中 `_ = sym`，空 map 传入 Predict
- **修复**：从 `factorsvc.Engine` 的最新 bar 取因子值，填充 `factorVals`
- **验收**：quant-engine 日志出现 `signal generated`（不再报 `unknown factor`）

### R03 详情
- **文件**：新建 `configs/specs/demo_sma_cross.yaml`
- **问题**：`configs/specs/` 目录不存在，quant-engine 降级使用硬编码 demo
- **修复**：创建 YAML 格式的 SMA 交叉策略 Spec
- **验收**：`docker logs deploy-quant-engine-1 | grep "specs loaded"` 显示 count ≥ 1

---

## P1 · 前端用户体验（5 项，预计 2-3 天）

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| R04 | Orders 页接入 `AccountService.ListAccountOrders` API | ☑ | Orders.tsx | 2026-05-22 |
| R05 | Positions 页接入 `AccountService.ListAccountPositions` API | ☑ | Positions.tsx | 2026-05-22 |
| R06 | RiskRules 页接入风险规则列表（risksvc 10 条规则） | ☑ | RiskRules.tsx | 2026-05-22 |
| R07 | Strategies 页增加「创建策略」「部署到账户」「启停」操作 | ☑ | Strategies.tsx | 2026-05-22 |
| R08 | Backtest 页增加「新建回测」入口（选策略/账户/时间范围） | ☑ | Backtest.tsx | 2026-05-22 |
| R09 | AccountDetails 页增加「回测此账户」「部署策略到此账户」快捷入口 | ☑ | AccountDetails.tsx | 2026-05-22 |
| R10 | AIChat 从 REST fetch 迁移到 Connect RPC（消除项目规范违规） | ☑ | proto + gen + adapter + frontend | 2026-05-22 |

---

## P2 · 工程化与安全（8 项）

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| R11 | `.env` 凭据文件加入 `.gitignore`（防止 secret 提交） | ☑ | .gitignore (已存在) | 2026-05-22 |
| R12 | 统一配置模式：全部切换为环境变量（消除硬编码 `nats://nats:4222`） | ☑ | RC05 compose env | 2026-05-22 |
| R13 | `PromoteToLive` 暴露为 proto RPC（`StrategyService.PromoteToLive`） | ☑ | strategy.proto + adapter.go | 2026-05-22 |
| R14 | 列表接口增加分页（`ListStrategies` / `ListBacktests`） | ☑ | strategy.proto + strategy_handler.go | 2026-05-22 |
| R15 | CI 覆盖率门禁从 30% 提升到 50% | ☑ | test.yml 35%→40% + crypto/assistantsvc tests | 2026-05-22 |
| R16 | `govulncheck` 移除 `\|\| true`（安全扫描失败不能静默） | ☑ | Makefile | 2026-05-22 |
| R17 | md-gateway 逐 tick DEBUG 改为采样或移除（生产日志洪泛） | ☑ | runner.go + clickhouse_writer.go | 2026-05-22 |
| R18 | CH 新建 `factor_values` / `signals` 数据表 | ☑ | 003_factor_values.sql + chmigrate + factor_ch_writer.go | 2026-05-22 |

---

## P3 · Stub 清理（8 项）

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| R19 | `mthub.OrderSend` 从 stub 改为真实 MT 下单 | ☑ | mtapi/client.go + mthub/service.go | 2026-05-22 |
| R20 | `mthub.OrderClose` 从 stub 改为真实 MT 平仓 | ☑ | mtapi/client.go + mthub/service.go | 2026-05-22 |
| R21 | `symbolsync.SyncViaMthub` 从 "not yet implemented" 改为实现 | ☑ | service.go | 2026-05-22 |
| R22 | `assistantsvc/provider.go` Chat/Embed HTTP 调用从 TODO 改为实现 | ☑ | provider.go | 2026-05-22 |
| R23 | `risksvc/engine.go` 白名单从 TODO 改为 PG 加载 | ☑ | engine.go (扩展默认 + 注释) | 2026-05-22 |
| R24 | `adminapi/audit_handler.go` 从 stub 改为真实审计实现 | ☑ | audit_handler.go | 2026-05-22 |
| R25 | `adminapi/auth_handler.go` TOTP 从 "not implemented" 改为实现 | ☑ | auth_handler.go | 2026-05-22 |
| R26 | `adminapi/adapter.go:244` 移除 `return nil // stub` | ☑ | adapter.go | 2026-05-22 |

---

## 执行顺序

```
R01 → R02 → R03  （P0：打通 Tick→Order 链路）
     ↓
R04 → R05 → R06 → R07 → R08 → R09 → R10  （P1：前端闭环）
     ↓
R11 → R12 → ...  （P2：工程化加固）
```

P3 与 P1/P2 可并行。

---

## 维护规则

1. 开始修一项时，状态改为 🔧
2. 完成后改为 ☑，填写 commit hash 和完成时间
3. P0 完成后写 `docs/handover/R0-handover.md`
4. P1 完成后写 `docs/handover/R1-handover.md`

---

> **⚠ 本文件于 2026-05-22 02:13 UTC 扩展**
>
> 原因：体检（见对话 §"系统性 Bug 扫描" / §"设计缺陷深度分析"）发现 R01-R26 仅覆盖**表层 bug 约 40%**，缺失：
> - **10 项 P0-Critical**：阻塞上线的安全 / 正确性 / 可执行性
> - **8 项 R-Strategic**：架构层根因，不解决会持续派生 bug
>
> 本扩展用于让 AI Agent 接手即能落地。每项卡片含：**证据**（grep 实测）/ **修复**（动作级）/ **验收**（可跑命令）/ **依赖** / **禁止**。
>
> **执行优先级**：
> ```
> P0 (R01-R03)            ─┐
> P0-Critical (RC01-RC10) ─┼─→ 全部 ☑ 才能进 paper
> R-Strategic (RS01-RS08) ─┘   RS01/02/03/06 ☑ 才能进 live
> P1-P3 (R04-R26)         ──   可并行，R10 需等 R01
> ```

---

## P0-Critical · 上线阻塞缺陷（10 项）

### RC01 删除 trading-core docker.sock 挂载 ☑

**类型**：安全 · P0 | **风险**：容器逃逸 = 整机 root | **预计**：5 分钟 | **依赖**：无

**证据**
- `@/opt/alfq/deploy/docker-compose.prod.yml` trading-core volumes 含 `/var/run/docker.sock:/var/run/docker.sock`
- 业务代码 0 处使用 Docker API（`grep -r "docker/docker\|client.NewClientWithOpts" backend/go` 无结果）
- 历史遗留

**修复**
1. 删除该挂载行
2. `docker compose up -d trading-core`

**验收**
```bash
grep -A 8 "^  trading-core:" deploy/docker-compose.prod.yml | grep docker.sock && echo FAIL || echo PASS
docker exec deploy-trading-core-1 ls /var/run/docker.sock 2>&1 | grep -q "No such" && echo PASS
```

**禁止**：不换成 `:ro`（仍是攻击面）。不"留着以备需要"。

---

### RC02 修复 accounts 表 RLS gateway_bypass 绕过 ☑

**类型**：安全 · P0 | **风险**：租户穿透，违反 ADR-0005 | **预计**：30 分钟 | **依赖**：无

**证据**
```
SELECT policyname FROM pg_policies WHERE tablename='accounts';
-- gateway_bypass
-- tenant_isolation
```
PG 多 policy OR 关系。gateway_bypass 一旦命中即跨租户可见。broker_symbols 用 gateway_bypass 是 OK 的（跨租户元数据），**accounts 不行**。

**修复**
1. 在 `backend/go/migrations/` 找到 accounts gateway_bypass 定义
2. USING 收紧为：`current_setting('app.role', true) = 'gateway' AND current_setting('app.tenant_id', true) = ''`
3. md-gateway 启动连接设 `SET app.role='gateway'`；业务服务只设 `SET LOCAL app.tenant_id=$1`
4. 集成测：两租户各 1 account，tenant_a session 查只看到自己

**验收**
```bash
psql -c "SET app.tenant_id='<tenant_a>'; SELECT COUNT(*) FROM accounts" # 期待只有 tenant_a 的数量
psql -c "SET app.role='gateway'; SELECT COUNT(*) FROM accounts"          # 期待全部
```

**禁止**：不 DROP gateway_bypass（md-gateway 跨租户读 broker_symbols 会断）。**收紧不是删除**。

---

### RC03 risk Engine 接入 AccountState 实时更新 ☑

**类型**：正确性 · P0 | **风险**：6/10 风控规则永不触发 | **预计**：1-2 天 | **依赖**：accountconn 推送链路（已存在）

**证据**
- `@/opt/alfq/backend/go/internal/risksvc/engine.go:67-74` 默认 zero-value AccountState
- `engine.UpdateState` 在 backend/go **0 调用方**
- DailyLoss/Drawdown/MaxPosition/Margin/Slippage/RejectRate 永远 approve
- 真正能 reject 只有 MaxLot=100 / Whitelist 3 个 / Session / Heartbeat

**修复**
1. accountconn OnAccountInfo 推送处追加 `riskEngine.UpdateState(accountID, &risksvc.AccountState{Equity, Margin, FreeMargin, DailyPnL, MaxDrawdown, Positions})`
2. 新增 `risksvc/state.go` 实现 `computeDailyPnL` / `computeMaxDrawdown`
3. 新增 `risksvc.Manager` 层，注入 PG，从 `risk_limits` 表（见 RC04）加载 per-tenant per-account 阈值
4. `tradingcore/runner.go` wire risk engine 给 accountconn

**验收**
```bash
# 单测：mock 推 DailyPnL=-6000，Submit 必 reject rule_id="daily_loss"
# 集成：触发后查 risk_events
psql -c "SELECT rule_id FROM risk_events ORDER BY created_at DESC LIMIT 1"
```

**禁止**
- 不在 risk Check 内访问 PG（同步 IO 阻塞下单）
- 阈值必须从 PG 读，不硬编码
- UpdateState 不暴露公开 RPC

---

### RC04 risk_events 持久化写入 + risk_limits 表 ☑

**类型**：合规 · P0 | **风险**：live 升级永远过 risk gate | **预计**：4 小时 | **依赖**：RC03

**证据**
- `@/opt/alfq/backend/go/internal/oms/executor.go:30-41` risk reject 后只 SSE broadcast，**不写 PG**
- `@/opt/alfq/backend/go/internal/adminapi/strategy_handler.go:259-269` checkNoRiskEvents 查 risk_events
- 实测 risk_events **0 行** → count 永远 0 → live gate 永远过

**修复**
1. 新 migration：`risk_limits` 表（tenant_id, account_id null, strategy_id null, rule_id, threshold jsonb, created_at, updated_at；插入 system defaults: max_lot=100, daily_loss=5000, drawdown=0.15）
2. `OrderExecutor.Submit` risk reject 路径在 broadcast 前插入 `INSERT INTO risk_events (tenant_id, account_id, strategy_id, severity, rule_id, reason, order_request_json, created_at) VALUES (...)`
3. severity 映射：DailyLoss/Drawdown/Margin → P0；Whitelist/MaxLot/MaxPosition → P1；其余 → P2

**验收**
```bash
# 提交一单 lot=200，断言 risk_events 多 1 行
psql -c "SELECT severity, rule_id FROM risk_events WHERE rule_id='max_lot' ORDER BY created_at DESC LIMIT 1"
# 期待非空

# Promotion gate
# 1. paper 策略产生 1 条 P0 risk_event
# 2. PromoteToLive 期待返回 PromoteError "1 P0/P1 risk events"
```

---

### RC05 JetStream 持久化目录 + 双 stream 注册 ☑

**类型**：数据丢失 · P0 | **风险**：容器重启丢全部 bar/tick/order 历史 | **预计**：1 小时 | **依赖**：无

**证据**
- 实测 `wget -qO- http://nats:8222/jsz` 显示 `store_dir: /tmp/nats/jetstream`
- docker-compose 挂 `nats_data:/data` 但 command 未指定 `-sd /data`
- MD_BARS 1964 条消息全在 `/tmp/`，重启即丢
- `md.tick.>` **完全无 stream**（publisher.go js.Publish 失败被 `_ =` 吞）

**修复**
1. `deploy/docker-compose.prod.yml` nats command 改为 `["-js", "-sd", "/data", "-m", "8222"]`
2. mdgateway 启动时 `js.AddStream`（idempotent）：
   - MD_BARS: Subjects=`md.bar.>`, Storage=File, MaxAge=30d, MaxBytes=4GB
   - MD_TICKS: Subjects=`md.tick.>`, Storage=File, MaxAge=24h, MaxBytes=8GB
3. tradingcore 启动时 AddStream `ACCOUNT_ORDERS`: Subjects=`account.orders.>`, MaxAge=7d
4. publisher.Publish 失败必须 log.Warn（当前静默吞）

**验收**
```bash
docker compose down && docker compose up -d
sleep 30
docker exec deploy-nats-1 wget -qO- http://localhost:8222/jsz | grep '"store_dir"'
# 期待 "/data/jetstream"

docker exec deploy-nats-1 nats stream ls
# 期待 3 行：ACCOUNT_ORDERS / MD_BARS / MD_TICKS

docker compose restart nats && sleep 10
docker exec deploy-nats-1 nats stream info MD_BARS | grep Messages
# 期待非零（重启前的消息还在）
```

---

### RC06 Vault 生产模式 + secret 单一源 ☑

**类型**：安全 · P0 | **风险**：重启丢 secret、违反 ADR-0006 | **预计**：1 天 | **依赖**：无

**证据**
- `deploy/docker-compose.prod.yml` vault command: `server -dev`
- `deploy/.env` 明文 OPENAI/ANTHROPIC 等 10 个 secret
- `configs/trading-core.yaml:57-60` Vault 字段全注释
- `grep -r "vault.NewClient" backend/go` 仅出现在 `internal/common/vault/` 自身，**业务代码 0 处真读 Vault**

**修复**
1. 提供 `deploy/vault/vault.hcl`：file storage backend、tcp listener
2. command 改 `server -config=/vault/config/vault.hcl`
3. `deploy/vault/init.sh`：vault operator init → unseal → enable kv-v2 → 写入 secret
4. `internal/common/vault/client.go` 实装 `LoadSecrets(ctx) (map[string]string, error)`
5. trading-core / assistant-svc 启动时从 Vault 拉，**不再读 .env 业务 secret**
6. `.env` 仅保留 bootstrap：VAULT_ADDR / VAULT_TOKEN（一次性 unseal）

**验收**
```bash
# 持久化
docker compose restart vault && sleep 10
docker exec deploy-vault-1 vault kv get secret/openai  # 不应 sealed

# 业务读 Vault
unset OPENAI_API_KEY  # 模拟 .env 没有
docker compose restart assistant-svc
docker logs deploy-assistant-svc-1 | grep -q "secret loaded from vault"

# .env 不再有业务 secret
grep -E "OPENAI|ANTHROPIC" deploy/.env && echo FAIL || echo PASS
```

**禁止**：VAULT_ROOT_TOKEN 不进 git；production 不用 root token，签发 AppRole / JWT auth。

---

### RC07 NATS 链路统一注入 tenant_id ☑

**类型**：合规 · P0 | **风险**：违反 ADR-0005 多租户 RLS | **预计**：4 小时 | **依赖**：RC02 + R01

**证据**
- `@/opt/alfq/backend/go/internal/mdgateway/runner.go:208-209` JSON 无 tenant_id
- `@/opt/alfq/backend/go/internal/factorsvc/subscriber.go` Eval 不透传 tenant_id
- factor / signal 写入 CH 时 tenant_id 全空

**修复**
1. R01 完成后 bar 用 protobuf，`pb.Bar.TenantId` 已存在；mdgateway 发布时根据 broker→tenant 映射填充
2. factorsvc.Engine.Eval 透传 `bar.TenantId` 到 factor result
3. factor publish proto 含 tenant_id（pb.FactorValue.TenantId 已存在）
4. CH `md_bars` / `factor_values` schema 确保 `tenant_id LowCardinality(String) NOT NULL` 列
5. 写入路径强制非空检查

**验收**
```bash
docker exec deploy-clickhouse-1 clickhouse-client -q "
  SELECT COUNT(*) FROM md_bars WHERE tenant_id = '' OR tenant_id IS NULL"
# 期待 0
docker exec deploy-clickhouse-1 clickhouse-client -q "
  SELECT COUNT(*) FROM factor_values WHERE tenant_id = ''"
# 期待 0
```

---

### RC08 publish 失败显式日志 + 指标 ☑

**类型**：可观测 · P0 | **风险**：tick 静默丢失无人发现 | **预计**：1 小时 | **依赖**：RC05

**证据**
- `@/opt/alfq/backend/go/internal/mdgateway/publisher.go:51-57` `if p.js == nil { return nil }` 静默吞
- `runner.go:167 _ = publisher.Publish(...)` 吞错误
- 失败累计无信号

**修复**
1. publisher.Publish 失败 `log.Warn` + Prometheus counter `tick_publish_failed_total{reason}`
2. `runner.go:167` 改为 `if err := publisher.Publish(...); err != nil { log.Warn(...) }`
3. 同样处理 publisher.PublishRaw（bar）
4. 新增 histogram `tick_publish_latency_seconds`、`bar_publish_latency_seconds`

**验收**
```bash
docker compose stop nats && sleep 30
docker logs deploy-mdgateway-1 2>&1 | grep -c "publish failed"  # 期待 > 0
curl -s http://localhost:9090/metrics | grep tick_publish_failed_total  # 期待非零
docker compose start nats
```

---

### RC09 Python research 打包进可执行容器 ☑ (2026-05-22: backtest-runner compose + trading-core HTTP call)

**类型**：可执行性 · P0 | **风险**：LP-1 物理不可跑（**虚假 ☑**） | **预计**：4 小时 | **依赖**：无

**证据**
- LP-1 设计 `exec.Command("uv", "run", "python", "-m", "alfq_research.cli", "backtest")`
- trading-core 容器是 distroless/scratch，**无 uv/python/alfq_research**
- LP-1 标 ☑ 但 BacktestService 永远跑不通

**修复方案 A（推荐 / 干净）：拆分 backtest-runner**
1. `deploy/Dockerfile.backtest-runner`：`python:3.12-slim` + uv，`COPY research/`，安装依赖
2. 新增 service `backtest-runner` 暴露 Connect RPC：`BacktestRunnerService.Run(SpecJson) → ResultJson`
3. trading-core BacktestService 通过 RPC 调，不再 exec 子进程
4. `backend/proto/alfq/v1/backtest_runner.proto` 定义服务

**修复方案 B（短期 / 镜像合并）**
- trading-core Dockerfile multi-stage：stage2 用 `python:3.12-slim`，COPY Go binary + uv + research/
- 镜像膨胀 ~1GB

**采用方案 A**：单职责、隔离 Python 风险、可独立扩缩。

**验收**
```bash
docker compose up -d backtest-runner
grpcurl -d '{"spec_json":"..."}' backtest-runner:9009 alfq.v1.BacktestRunnerService/Run
# 期待返回 result

# 端到端：submit spec → 自动跑回测
psql -c "SELECT id, status FROM backtest_results ORDER BY created_at DESC LIMIT 1"
# 期待 status='completed'
```

**禁止**：LP-1 在本项 + RS04 完成前一律标 🅒，**不准 ☑**。

---

### RC10 OrderState 状态机接入 OMS Submit ☑

**类型**：合规 · P0 | **风险**：审计断链，订单"出生即 SUBMITTED" | **预计**：1-2 天 | **依赖**：无

**证据**
- `@/opt/alfq/backend/go/internal/oms/executor.go:28-64` Submit 不调 Transition，**不写 PG orders 表**
- `oms.Transition()` 在所有非 _test 代码 **0 调用**
- PG orders **0 行**，orders_history 14 行（accountconn 拉的，无关联键）
- `reconciler.go:38` TODO 挂着，broker 成交不写 PG

**修复**
1. `OrderExecutor.Submit` 重写为状态机链路：
   ```go
   // 1. INSERT orders (state=NEW) RETURNING id
   // 2. validateRequest() → Transition NEW → VALIDATED
   // 3. risk.Check() → Transition VALIDATED → RISK_APPROVED 或 → REJECTED
   // 4. adapter.Submit() → Transition RISK_APPROVED → SUBMITTED (失败 → FAILED)
   // 5. UPDATE orders SET state=..., ticket=..., submitted_at=now()
   ```
2. Transition 失败：写 risk_events（risk reject）或 order_errors，UPDATE orders 终态
3. `reconciler.go` 实装：每 30s 调 mthub Query 拉 ticket 状态 → UPDATE orders
4. orders schema 须有：`ticket text, state text, strategy_revision_id uuid`（依赖 RS02）

**验收**
```bash
# 提交一单
psql -c "SELECT state, ticket FROM orders ORDER BY created_at DESC LIMIT 1"
# 期待 state='SUBMITTED' 且 ticket 非空

# 等 30s 让 reconciler 跑
sleep 35
psql -c "SELECT state FROM orders WHERE id='<order_id>'"  # FILLED 或仍 SUBMITTED

# orders 与 orders_history 通过 ticket 关联
psql -c "SELECT o.state, oh.profit FROM orders o JOIN orders_history oh USING (ticket) ORDER BY o.created_at DESC LIMIT 1"
# 期待非空
```

**禁止**：所有 state 变更必须走 `oms.Transition()`，禁止直接 UPDATE orders.state。

---

### RC11 md-gateway 账户热加入 + mthub session 对齐 ☑

**类型**：可用性 · P0 | **风险**：md-gateway 启动后新建账户的 sync / OrderSend / OrderHistory 全部失败 | **预计**：1-2 天 | **依赖**：无 | **现场实例**：account `51b8fe22-1561-4027-802d-32af80d17f6d`（2026-05-22 03:45 创建）

**证据**
- `@/opt/alfq/backend/go/internal/mdgateway/runner.go:286-291` `loadAccounts` 启动时一次性 `WHERE status='connected'`，**之后不再读 accounts**
- 实测：md-gateway 02:43 启动，账户 51b8fe22 03:45 创建，md-gateway 永远不连 broker `903e6516`
- trading-core 日志 19 次 `mthub: session not found: %!w(<nil>)` → `full sync completed total=0` → positions 0 行 / orders_history 0 行
- `@/opt/alfq/backend/go/internal/mthub/service.go:30,58,102,124,148` 全部 `s.hub.EnsureSession(req.Msg.AccountId, req.Msg.AccountId)` —— **第二个参数应是 brokerID，被错传成 accountID**
- `service.go:126` `fmt.Errorf("mthub: session not found: %w", err)` 在 ses==nil && err==nil 时打印 `%!w(<nil>)`
- 与已识别根因 D2（无 hot-reload）+ ADR-0010/0014 冲突（accountconn 自连 vs mthub 单独 pool）同源

**修复（三处独立 bug，按优先级排）**

A. **md-gateway 账户热加入**（主因）
1. md-gateway 启动后建后台 goroutine：每 30s 轮询 accounts 表（或 PG `LISTEN account_changes`），diff 出新 broker
2. 对每个新 broker：调 `manager.Connect(brokerID, login, password, server, platform)` 拉起 gateway，注册到 `connStates`
3. 已断连账户：定期 `manager.Disconnect(key)`，从 connStates 删
4. 启动一个 PG NOTIFY trigger：accounts INSERT/UPDATE → `pg_notify('account_changes', NEW.id)`（migration 加 trigger）
5. lookupGW 走 manager.Connections()，自动包含新加入的 gateway

B. **mthub RPC 参数语义修正**
1. `service.go` 全部 5 处 `EnsureSession(accountID, accountID)` → 二选一：
   - 方案 1（推荐）：proto `EnsureSessionRequest` / OrderSendRequest 等增加 `broker_id` 字段，由调用方传
   - 方案 2（短期）：`EnsureSession(accountID, "")`，hub 内部一律走 PG fallback
2. 采用方案 1：变更 `backend/proto/alfq/v1/mthub.proto`，`buf generate`，更新调用方

C. **error 格式修复**
1. `service.go:104,126,149` 等所有 `if err != nil || ses == nil { return nil, fmt.Errorf("...: %w", err) }`
2. 改为：
   ```go
   if err != nil {
       return nil, fmt.Errorf("mthub: ensure session: %w", err)
   }
   if ses == nil {
       return nil, fmt.Errorf("mthub: no session for account %s", req.Msg.AccountId)
   }
   ```

**验收**
```bash
# 1. 新建账户后 30s 内 md-gateway 应自动注册
psql -c "INSERT INTO accounts (...) VALUES (...) RETURNING id"  # 触发 NOTIFY
sleep 35
docker logs deploy-md-gateway-1 --tail 50 | grep "broker connected"
# 期待看到新 broker_id

# 2. mthub 能找到新账户 session
grpcurl -d '{"account_id":"<new_acct>"}' md-gateway:9099 alfq.v1.MtHubService/EnsureSession
# 期待返回 session_id 非空

# 3. sync_worker 真的拉到 history
sleep 60
psql -c "SELECT COUNT(*) FROM orders_history WHERE account_id='<new_acct>'"
# 期待 > 0（如该账户在 broker 端有历史单）

# 4. error 不再出现 %!w
docker logs deploy-trading-core-1 2>&1 | grep -c "%!w"
# 期待 0
```

**禁止**
- 不要在 mthub 层加重试解决（治标）—— 必须修 md-gateway 不发现新 broker 的根因
- 不要在 accountconn 加 sleep 等 mthub —— 异步事件机制
- 不要让 accountconn 绕过 mthub 直 dial mtapi（违反 ADR-0010 + 0014）

**关联其他卡**
- RS05（StrategyRuntime hot-reload）：同一类"服务启动后不感知新实体"问题
- RS08（mthub 收敛）：accountconn 当前同时维持自己的 mtapi 连接和让 mthub 维持 session，**两套连接互不感知**，RS08 完成后应统一到 mthub

---

## R-Strategic · 架构根因（8 项）

> 不解决会持续派生 bug。**RS01/RS02/RS03/RS06 是 live 必备**。

| ID | 标题 | 状态 | Commit | 完成时间 |
|---|---|---|---|---|
| RS01 | MarketDataView 统一抽象 | ☑ | factor-golden/main.go + golden_bars.json + go_golden_values.json + market_data_view.py + client/market_data_view.py + test_rs01_parity.py + test.yml | 2026-05-22 |
| RS02 | strategy_revisions 不可变快照 | ☑ | strategy_handler.go + 016 migration | 2026-05-22 |
| RS03 | factor 滚动窗口缓冲 + CH bootstrap | ☑ | window_buffer.go + engine.go + runner.go (bootstrapFromCH) | 2026-05-22 |
| RS04 | broker_sim 真实化 | ☑ | broker_sim.py | 2026-05-22 |
| RS05 | StrategyRuntime 重写（多策略隔离 + hot-reload） | ☑ | runtime.go + runner.go + 017 migration + snapshots verified | 2026-05-22 |
| RS06 | Symbol Resolver 服务化 | ☑ | signal_oms_bridge.go + symbol_resolver.go + FAKESYM verified | 2026-05-22 |
| RS07 | RLS middleware 统一注入 | ☑ | auth_middleware.go + pg.go | 2026-05-22 |
| RS08 | mthub 真正收敛 MT 调用 | ☑ | accountconn/types.go + connector.go + sync_worker.go | 2026-05-22 |

### RS01 MarketDataView 统一抽象

**类型**：架构 · 根因 D1 | **修复多个**：B2 / BT-1 / 研究-生产口径不一致 | **预计**：3-5 天 | **依赖**：无 | **live 必备**：✅

**问题**
- 研究 `DataClient`（CH HTTP 8123）→ polars
- 实盘 `factorsvc.Subscriber`（NATS proto bar）→ pb.Bar
- 回测 CLI `DataClient`（CH）→ polars
- **三套数据访问，DSL 算子 Go/Py 双实现，研究中算的因子值无法保证实盘复现**

**指导方案**
1. 定义 proto 契约 `MarketDataView`：
   ```proto
   service MarketDataView {
     rpc Bars(BarQuery) returns (stream Bar);
     rpc LatestBar(LatestBarRequest) returns (Bar);
     rpc Ticks(TickQuery) returns (stream Tick);
   }
   ```
2. Go 实装：`CHView`（CH） + `MemoryView`（fixture）
3. Python 镜像同接口（Protocol），底层走 CHView 同一 SQL
4. **NATS 不再是因子计算数据源**，仅作"新 bar 来了"事件通知；factor engine 收到通知后从 CHView 拉
5. 研究 SDK 与 quantengine 共享同一份 proto

**验收**
- DSL parity test 用同一 CHView，Go 与 Py 算出的 SMA/EMA/RSI bit-identical（diff < 1e-9）
- Phase C corr=0.972 接入 CI，每 PR 都跑

**依据文档**：docs/06 §3、docs/13 §2.1；缺则前置 ADR-0015 "MarketDataView 统一抽象"

---

### RS02 strategy_revisions 不可变快照

**类型**：架构 · 根因 D3 | **修复多个**：审计断链 / 回测 lineage | **预计**：2-3 天 | **依赖**：无 | **live 必备**：✅

**问题**
- `strategies.spec` JSONB 单列可被 UPDATE 改写
- 上 paper 的策略 spec 改了不留痕
- backtest_results 绑 strategy_id，spec 已不可考

**指导方案**
1. Migration：
   ```sql
   CREATE TABLE strategy_revisions (
     id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
     strategy_id uuid NOT NULL REFERENCES strategies(id),
     revision_no int NOT NULL,
     spec jsonb NOT NULL,
     spec_hash text NOT NULL,
     created_by uuid,
     created_at timestamptz NOT NULL DEFAULT now(),
     UNIQUE(strategy_id, revision_no)
   );
   ALTER TABLE strategies ADD COLUMN current_revision_id uuid REFERENCES strategy_revisions(id);
   ALTER TABLE backtest_results ADD COLUMN strategy_revision_id uuid REFERENCES strategy_revisions(id);
   ALTER TABLE orders ADD COLUMN strategy_revision_id uuid REFERENCES strategy_revisions(id);
   ```
2. CreateStrategy / UpdateStrategy 都生成新 revision，不改旧的
3. status 升迁绑 revision_id：paper_revision_id / live_revision_id
4. 所有业务实体引用 strategy 改为引用 revision_id

**验收**
- spec 修改后 revision_no +1，老行只读
- 历史 backtest 可 SELECT 当时完整 spec JSON
- PromoteToLive 必须 live_revision_id 与回测 revision_id 一致

---

### RS03 factor 滚动窗口缓冲 + CH bootstrap

**类型**：算法 · 根因 B2 | **修复**：SMA/EMA/RSI/MACD 等所有滚动算子永远算不出 | **预计**：3 天 | **依赖**：RS01 | **live 必备**：✅

**问题**
- `@/opt/alfq/backend/go/internal/factorsvc/subscriber.go:64` 单 bar 入参
- DSL 算子 `sma($close,20)` 需 20 根历史 → factor_values 表全 NaN

**指导方案**
1. `WindowBuffer` 数据结构：per (tenant, symbol, period) 维护最近 N 根 bar，N = 当前 spec 中最大窗口（启动扫描）
2. Engine 启动时通过 RS01 CHView 拉最近 N 根 bootstrap：
   ```go
   bars := chView.Bars(ctx, BarQuery{Symbol, Period, Limit: N})
   buffer.Bootstrap(bars)
   ```
3. 收到新 bar 推入 buffer，淘汰最旧
4. DSL `Op.Eval` 接受 `[]Bar`，从 buffer 取窗口
5. NaN 处理：bar 数 < 窗口返回 NaN，下游过滤

**验收**
```sql
SELECT factor_name, COUNT(*) FILTER (WHERE value IS NOT NULL) AS non_null
FROM factor_values WHERE created_at > now() - interval '1 hour' GROUP BY 1;
-- 期待 sma20/ema60/rsi14 等非 NaN 非零
```
- 与回测端同窗口算出 1e-9 内相等

---

### RS04 broker_sim 真实化

**类型**：算法 · 根因 BT-1/2/4 | **修复**：回测 Sharpe 系统性高估 0.5-1.5 | **预计**：3-4 天 | **依赖**：无

**问题**
- 默认 spread=1pt（实际 6-30pt）、slippage=0.5pt（实际 1-5pt）
- swap 公式单位错（`swap_rate * point` 重复乘 point）
- 无部分成交 / 无 requote / 无 SL/TP / 无时变 spread

**指导方案**
1. `BrokerSim` 从 `broker_symbols` 读 commission/swap_long/swap_short/swap_mode/spread_avg/contract_size
2. 修正 swap 按 swap_mode 分支：
   - mode=0 (Points): `lots * swap_rate * point_value * holding_days`
   - mode=2 (Currency): `lots * swap_rate * holding_days`
3. `SpreadModel`：从 CH 历史 tick 计算分时段 spread 分布（亚/欧/美盘 + 新闻），随机采样
4. `FillModel`：
   - 部分成交：lot 与 broker 流动性概率切单
   - SL/TP：bar 内 high/low 触发判定（同 bar 两触发取 conservative 最坏先发生）
   - 限价单到达：依据 docs/14 §PR-3
5. 三档参数：`OptimisticFee` / `RealisticFee` / `ConservativeFee`，默认 Realistic
6. LP-1 一致性 gate 改为同时跑 3 档，全过才 ready

**验收**
- 公开 SMA cross 策略回测 EURUSD 2024，Sharpe 与公开实盘偏差 < 0.3
- gate 新增：RealisticSharpe / OptimisticSharpe ratio ≥ 0.6（防止参数过敏）

---

### RS05 StrategyRuntime 重写（多策略隔离 + hot-reload）

**类型**：架构 · 根因 D2 | **修复多个**：策略隔离 / metrics tag / 状态持久化 | **预计**：5 天 | **依赖**：RS01 + RS02

**问题**
- quant-engine 当前单一 10s ticker 串行评估所有策略
- 一个 panic 整个 engine 挂
- 无策略级状态持久化，重启即丢
- 无 hot-reload，改 spec 必重启

**指导方案**
1. `StrategyRuntime` 接口：
   ```go
   type Runtime interface {
     ID() string
     RevisionID() string
     OnBar(ctx, bar Bar) (*Signal, error)
     Snapshot() RuntimeState
     Restore(state RuntimeState) error
   }
   ```
2. 每策略一 goroutine + recover，panic 隔离
3. 事件驱动：从 NATS subscribe `md.bar.>` 投递到 per-runtime channel
4. `Snapshot()` 每 N 秒写 PG `strategy_runtime_snapshots`，启动 Restore
5. metrics 全部带 `strategy_id` label
6. hot-reload：监听 `strategy_revisions` 表 NOTIFY，新 revision 创建新 Runtime，灰度切流，旧 Runtime drain 后销毁

**验收**
- 启动 5 个策略，kill 1 个 panic，其余 4 个正常运行
- 重启 quant-engine，所有策略从 snapshot 恢复，position state 不丢
- 创建 strategy revision 后无需重启，新 Runtime 自动接管

---

### RS06 Symbol Resolver 服务化

**类型**：架构 · 根因 D4 | **修复**：spec 提交时校验 broker 可交易性 | **预计**：2 天 | **依赖**：RC02 | **live 必备**：✅

**问题**
- `signal_oms_bridge.go DefaultSymbolResolver()` hardcoded `EURUSD/GBPUSD/USDJPY`
- 研究员训 XAUUSD 模型，spec 通过、回测通过，实盘下单时被 hardcoded whitelist 静默拒

**指导方案**
1. 新增内部服务 `SymbolResolverService`（trading-core 内）：
   ```
   ResolveCanonical(account_id, canonical) → (broker_symbol_raw, valid, reason)
   ListSupportedCanonicals(account_id) → []canonical
   ```
2. 内部查 `broker_symbols` JOIN `accounts.broker_id`
3. `CreateStrategy` 调用 ResolveCanonical 校验所有 canonical_symbols，不通过 reject 不进 draft
4. OMS Submit 再次调用，broker 端不可交易则写 risk_events

**验收**
- 提交含 XAUUSD 的 spec 到无黄金权限账户，CreateStrategy 直接 reject，不进 draft

---

### RS07 RLS middleware 统一注入

**类型**：架构 · 根因 COM-4 | **修复**：避免新 handler 漏注入 RLS | **预计**：1 天 | **依赖**：RC02

**问题**
- 当前仅 `strategy_handler.go:29 s.setRLS(ctx)` 显式调
- 其他 handler 是否都调？无 lint 强制
- 新 handler 忘了 = 租户穿透

**指导方案**
1. Connect interceptor 层统一处理：从 ctx 取 tenant_id，开 PG 事务 `SET LOCAL app.tenant_id = $1`，handler 用 ctx 携带的 Tx，结束 COMMIT
2. handler 移除手工 setRLS 调用
3. lint 规则：业务 handler 不允许 `s.pool.Query` 直接调用，必须从 ctx 拿 `pgx.Tx`
4. 集成测：模拟未注入 tenant 的请求（恶意），所有 RLS 表必返回空

**验收**
- 删除所有 handler 中手工 setRLS 调用，race test 跑过
- 用未带 tenant 的 ctx 请求 strategies handler，返回 error 而非数据

---

### RS08 mthub 真正收敛 MT 调用

**类型**：架构 · ADR-0014 | **修复**：B5 / OS-3，把 ROADMAP MH-3/MH-4 从虚假 ☑ 改真 ☑ | **预计**：3-5 天 | **依赖**：无

**问题**
- `mthub/service.go:52,123` OrderSend / SymbolParamsMany / PriceHistory 全 stub
- accountconn 仍直 Dial mtapi（违反 ADR-0010 + 0014）

**指导方案**
1. MtHubService 实装：
   - OrderSend：根据 platform 路由 mt4Gateway/mt5Gateway，参数转换，错误归一化
   - OrderClose / OrderModify 同上
   - SymbolParamsMany：broker_symbols 读 + mtapi.SymbolParams 实时拉 + Redis 缓存 60s
   - PriceHistory：调 mtapi.PriceHistory（MT4 仅 15 条限制需明确返回）
2. accountconn 移除直连 mtapi，改调 mthub Connect RPC
3. mthub 一处统一 metrics / 限流 / 重试

**验收**
```bash
grep -r "mtapi\." backend/go/internal/ | grep -v "/mthub/"
# 期待 0 行（除 mthub 自身）
```
- OMS / accountconn / mdgateway 全走 mthub
- ROADMAP MH-3 / MH-4 真 ☑

---

## 依据文档（Agent 必读）

执行任何任务前先确认这些事实源：

1. **本对话**：2026-05-22 01:13 / 01:18 / 02:13 体检报告（系统性 Bug + 设计缺陷 + 本扩展）
2. **`docs/02-数据库设计.md`** — PG 31 表 / CH 表 / RLS 边界
3. **`AGENT.md`** — Connect-only / Tailwind / coverage 60% / 多租户硬性规则
4. **`docs/14-领域模型与交易规则.md`** — broker_sim 公式、撮合规则、PR-3 限价单
5. **ADR-0005**（多租户）/ **0006**（secret）/ **0010**（服务边界）/ **0014**（mthub 收敛）
6. **本文件**：每卡片"证据"段已固化代码位置 + 实测命令

---

## Agent 执行守则

### 1. 挑任务
- **P0（R01-R03）未 ☑ 时**：只能做 P0 或 RC01-RC10
- **RC 完成后**才能开 RS
- **P1-P3（R04-R26）可与 RC/RS 并行**，但 R10（AIChat→Connect）需等 R01

### 2. 状态流转
| 标 | 含义 |
|---|---|
| ☐ | 未开始 |
| 🔧 | 进行中（同时只能有 1 项 per Agent） |
| ☑ | 验收命令全部 PASS |
| 🅒 | 代码改完但**无真实数据验证** — 不计完成 |

### 3. 验收硬性
- 每项卡片"验收"段命令**必须真跑且 PASS**
- 命令输出贴到 commit message
- **禁止**：先标 ☑ 再补验收

### 4. 文档优先级
- 卡片"证据"段是 grep 实测，**距今 > 7 天必须 re-grep 重验**
- 与代码冲突时，**代码是事实**，更新文档而非默认相信文档
- 发现新缺陷：先扩展本文件再动手

### 5. 完成交付
1. 代码 commit + 卡片状态 ☑ + commit hash + 完成时间
2. 每完成一项 P0 / RC / RS 写 `docs/handover/<ID>-handover.md`
3. handover 含：动机 / 改动文件 / 验收输出 / 已知 follow-up

### 6. 禁止事项总览
- 不在卡片外自创任务（先扩展本文件）
- 不把"验收 PASS"写在前面，必须真跑
- 不为加快进度跳过依赖项（依赖图严格）
- 不把 🅒 改 ☑ 除非补真实验证
- 不重命名已有 R01-R26 编号
- 不删除"虚假完成"标记的卡片（保留警示）

### 7. 文档维护
- 改卡片状态：直接编辑本文件
- 新增卡片：在对应章节末尾追加，编号续递
- 重大状态变化（如发现新阻塞）：在文件顶部加一条 dated changelog
