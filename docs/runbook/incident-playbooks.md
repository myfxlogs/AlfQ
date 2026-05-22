# ALFQ Incident Runbooks — 6 类故障处置流程

> 版本：v1.0 · 2026-05-20
> 来源：MASTER-ROADMAP OP-2
> 原则：先止损、后定位、再复盘。每次故障必须写 postmortem (`docs/postmortems/`)。

---

## 1. 行情中断 (Market Data Interruption)

**症状**：
- `alfq_md_tick_total` 计数器 5 分钟无增长
- Grafana SLO 面板 "Tick Gap Rate" 告警触发
- 前端价格停滞

**严重级别**：P0（无法交易）

**处置步骤**：
1. **确认范围**：`curl http://md-gateway:9001/readyz` → 检查返回
2. **检查连接**：查看 md-gateway 日志 `grep "disconnect\|reconnect"` 确认与经纪商连接状态
3. **切换备用连接**（如果有备用 broker）：`curl -X POST http://trading-core:9000/api/accounts/{id}/reconnect`
4. **启用 spill 重放**（如果连接恢复）：启动 `md-backfill` 回补缺口时段
5. **15 分钟内无法恢复**：触发 Kill Switch，停止所有策略
6. **事后**：写 postmortem，记录中断时长、缺口 bar 数、影响策略

**SLO**：gap < 0.01%，p99 latency < 50ms

---

## 2. ClickHouse 写入失败 (CH Write Failure)

**症状**：
- `alfq_ch_insert_rows_total{status="error"}` 增长
- Grafana "CH Write Success Rate" 低于 99.99%
- md-gateway 日志大量 `ch write error`

**严重级别**：P1（数据丢失风险，但不影响实时交易）

**处置步骤**：
1. **检查 CH 状态**：`docker exec deploy-clickhouse-1 clickhouse-client -q "SELECT 1"`
2. **检查磁盘空间**：`df -h` / `docker exec deploy-clickhouse-1 df -h`
3. **清理旧数据**（如果是磁盘满）：CH 有 90 天 TTL，检查是否 TTL 未生效
4. **重启 CH**：`docker restart deploy-clickhouse-1`
5. **检查 spill 目录**：md-gateway 的 JSONL spill 是否在增长（正常情况 CH 故障时 spill 增长）
6. **CH 恢复后**：spill 自动重放（md-gateway 的 spill_replay.go）
7. **验证**：Grafana 面板确认写入成功率恢复到 > 99.99%
8. **事后**：检查是否丢失任何 minute bar；如丢失，运行 `md-backfill`

**SLO**：写入成功率 > 99.99%

---

## 3. 经纪商连接被踢 (Broker Disconnect)

**症状**：
- `alfq_md_connection_state == 0`（gauge 归零）
- `alfq_md_reconnect_total` 计数器增长
- 前端显示 "账户已断开"

**严重级别**：P1（单账户无法交易，其他账户正常）

**处置步骤**：
1. **确认账户**：检查 Prometheus 标签 `broker` 确定哪个经纪商断开
2. **检查原因**：查看 `accountconn` 日志，常见原因：
   - 经纪商服务器维护（非我方问题）
   - 网络超时（检查 VPS 网络）
   - 账户密码变更
   - 经纪商 API 限流
3. **手动重连**：`curl -X POST http://trading-core:9000/api/accounts/{id}/connect`
4. **自动重连**：md-gateway 自带指数退避重连（1s→2s→4s→...→max 60s）
5. **重连超过 10 次失败**：标记账户状态 `error`，通知人工介入
6. **替换 broker**（如果可用）：更新账户配置指向备用经纪商
7. **事后**：记录断开时长、重试次数、根本原因

---

## 4. 单策略熔断 (Single Strategy Breaker)

**症状**：
- `alfq_risk_event_total{severity="P1"}` 增长
- 特定 `strategy_id` 的 `alfq_risk_breaker_state != 0`
- 该策略不再产生新订单

**严重级别**：P2（单策略停止，不影响全局）

**处置步骤**：
1. **确认熔断范围**：查看 Prometheus `alfq_risk_breaker_state{strategy_id="xxx"}`
2. **查看违规原因**：检查 risk-svc 日志，获取触发熔断的具体规则
3. **评估是否误判**：
   - 检查最近 1h 行情（是否异常波动）
   - 检查该策略的 Paper 回测指标（Sharpe/MaxDD 是否正常）
4. **手动解除熔断**（如确认是误判）：
   ```bash
   curl -X POST http://trading-core:9000/api/strategies/{id}/breaker-reset
   ```
5. **如果非误判**：禁止手动解除，需修改策略参数后重新走 paper→live 流程
6. **调整熔断阈值**（如果需要）：更新 risk-svc 配置
7. **事后**：记录熔断触发条件、影响时间段、是否误判

---

## 5. Kill Switch 触发 (Kill Switch Active)

**症状**：
- `alfq_risk_killswitch_active == 1`
- 所有 `alfq_risk_check_total` 返回 `approved=false`
- Grafana "Risk Kill Switch Active" 显示 ACTIVE（红色）

**严重级别**：P0（全局交易停止）

**处置步骤**：
1. **立即通知**：通知交易团队 + 风控团队 + 技术负责人
2. **确认原因**：查看触发 Kill Switch 的具体事件：
   - 日亏损超限（DailyLoss > $5000）
   - 总回撤超限（MaxDrawdown > 15%）
   - 手动触发（`/killswitch` API）
3. **如果是自动触发**：
   - **不要立即解除**！先分析风控事件
   - 检查所有活跃策略的仓位和盈亏
   - 确认是否为真实亏损 or 数据错误
4. **手动平仓**（如果需要）：通过 MT 客户端平掉所有未平仓仓位
5. **解除 Kill Switch**（仅在确认安全后）：
   ```bash
   curl -X POST http://trading-core:9000/killswitch/reset
   ```
6. **恢复交易**：按策略逐个重新启用（paper→live 流程）
7. **事后**：必须写 postmortem（`docs/postmortems/`），记录触发原因、损失、预防措施

**重要**：Kill Switch 解除需要双签（tenant_admin + risk_officer）

---

## 6. Spill 写满 (Spill Buffer Full)

**症状**：
- `alfq_md_buffer_used_bytes` 接近或达到上限
- md-gateway 日志 `"spill buffer full, dropping ticks"`
- Grafana 数据面板出现数据空洞

**严重级别**：P1（数据丢失，但不影响实时交易）

**处置步骤**：
1. **确认 CH 状态**：Spill 满通常是因为 CH 不可用 → 先按 §2 处置 CH 写入失败
2. **检查 spill 目录大小**：`du -sh /data/alfq/spill/`
3. **手动清理 spill**（如果 CH 已恢复且 spill 已重放）：
   ```bash
   rm /data/alfq/spill/*.jsonl
   ```
4. **扩容 spill 磁盘**：修改 docker-compose 卷大小
5. **增加 spill 缓冲区**（代码配置）：调整 `clickhouse_writer.go` 中的 `maxSpillBytes`
6. **验证**：确认 spill 目录不再增长，CH 写入成功
7. **事后**：回填缺失时段数据（`md-backfill`）

---
---

## 7. 因子计算异常 (Factor Anomaly)

**症状**：
- `factor_values_null_ratio` 超过 50%
- `alfq_factor_eval_error_total` 计数器增长
- 策略信号全部为 flat（无新订单产生）

**严重级别**：P1（策略静默停止）

**处置步骤**：
1. **确认范围**：`clickhouse-client -q "SELECT factor_name, count() FILTER (WHERE value IS NULL) FROM factor_values WHERE ts > now() - INTERVAL 10 MINUTE GROUP BY 1"`
2. **检查数据源**：CH 中 md_bars 是否正常写入（因子计算依赖 bar 数据）
3. **检查窗口缓冲**：WindowBuffer 是否初始化完成（启动后 bootstrap 需 N 根 bar）
4. **重启因子引擎**（如果是启动时序问题）：`docker restart deploy-quant-engine-1`
5. **检查 DSL 表达式**：最近是否有策略 spec 变更导致因子表达式不兼容
6. **验证恢复**：`clickhouse-client -q "SELECT factor_name, count() FROM factor_values WHERE ts > now() - INTERVAL 5 MINUTE GROUP BY 1"`
7. **事后**：记录异常因子名称、持续时长、根本原因

---

## 8. 回测失败 (Backtest Failure)

**症状**：
- `POST /run` 返回 500 或 504
- `alfq_backtest_total{status="failed"}` 增长
- 前端回测页面显示 "backtest failed"

**严重级别**：P2（研究受阻，不影响实盘）

**处置步骤**：
1. **检查 backtest-runner 状态**：`docker logs deploy-backtest-runner-1 --tail 50`
2. **常见原因**：
   - 数据不足（所选时间段内 CH 无 bar 数据）
   - 策略 spec 中的因子表达式语法错误
   - Python 依赖缺失（`ModuleNotFoundError`）
   - 超时（回测时间超过 5 分钟）
3. **验证数据**：`clickhouse-client -q "SELECT min(ts), max(ts), count() FROM md_bars WHERE symbol='EURUSD' AND period='M5'"`
4. **本地调试**：`cd research && uv run python -m alfq_research.cli backtest --spec '<json>'`
5. **清理重试**：如果文件系统残留，`rm -rf /tmp/alfq-backtest-spec-*.json`
6. **事后**：如果是代码 bug，修 → 加测试 → 重新部署

---

## 通用工具命令

```bash
# 查看所有服务状态
docker compose ps

# 查看实时日志
docker compose logs -f --tail=100 md-gateway

# 检查 Prometheus 告警
curl http://prometheus:9090/api/v1/alerts | jq

# 检查 Grafana 面板
open http://grafana:3000/d/slo-overview
```
