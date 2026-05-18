# 0003 - 数据存储三件套：PostgreSQL + ClickHouse + Redis

| 字段 | 值 |
|---|---|
| 日期 | 2026-05-18 |
| 状态 | accepted |
| 决策者 | architecture, data |
| 影响范围 | 全局 |
| 关联 ADR | — |
| 关联 docs | docs/02 |

## 背景

需要存储：
- 交易主数据（订单 / 账户 / 用户 / 审计）：强一致、ACID
- 时序数据（tick / bar / 因子 / 信号 / 成交）：高写入、列存
- 缓存 / 会话 / 锁 / 限流：低延迟 KV

## 选项

### A. 单一 PostgreSQL（含 TimescaleDB）
- 优点：单库简单
- 缺点：tick 量级（TB/年）下 OLAP 性能不足，列压缩与并行查询弱于 CH

### B. PG + InfluxDB
- 优点：InfluxDB 专注时序
- 缺点：InfluxQL/Flux 学习成本，社区不如 CH 活跃

### C. PG + ClickHouse + Redis
- 优点：各司其职，CH 在金融时序量级最优；Redis 解决高频 KV
- 缺点：运维三套

### D. PG + TimescaleDB + Redis
- 优点：保留 SQL 一致性
- 缺点：在数 TB tick 量下压缩比与查询并发不如 CH

## 决策

采用 **C. PG + ClickHouse + Redis**。

辅以：
- MinIO/S3 用于对象（模型、报告、归档）
- NATS JetStream 用于消息总线

## 后果

### 积极
- 各 DB 按场景最优
- CH 列压缩 + 物化视图，K 线/因子查询毫秒级
- Redis 提供分布式锁、限流、会话

### 消极
- 三套备份/恢复/监控
- 跨库事务不存在（需 Outbox/Saga，详见文档 21）

### 跟进事项
- [ ] 备份策略文档（11 章已有）
- [ ] 跨库一致性方案（21 章）
- [ ] DR 演练手册（11 章追加）
