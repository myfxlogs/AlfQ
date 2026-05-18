# 0005 - 多租户：逻辑隔离 + PG Row-Level Security

| 字段 | 值 |
|---|---|
| 日期 | 2026-05-18 |
| 状态 | accepted |
| 决策者 | architecture, security |
| 影响范围 | 全局 |
| 关联 ADR | 0003 |
| 关联 docs | docs/05 |

## 背景

多租户隔离选型，平衡：成本、安全、运维复杂度。

## 选项

### A. 单租户独立部署（每客户一套环境）
- 优点：物理隔离最强
- 缺点：成本极高，运维不可扩展

### B. 共享应用 + 独立数据库 schema per tenant
- 优点：数据库层强隔离
- 缺点：迁移繁琐，连接数倍增

### C. 共享应用 + 共享 schema + 行级隔离（RLS）
- 优点：资源利用率最高；运维简单
- 缺点：每张表必须有 tenant_id；RLS 不开启即漏；CH 无原生 RLS

### D. C + 部分关键资源物理隔离（broker 账号、对象存储桶）
- 优点：风险与成本平衡
- 缺点：实现略复杂

## 决策

采用 **D**：
- PG/CH/Redis 共享，业务表带 `tenant_id` + PG RLS（应用层每 conn `SET app.tenant_id`）
- CH 配置 Row Policy + 应用强制 WHERE
- **Broker 账号物理隔离**（每租户独立 mtapi 连接 + 凭据）
- **对象存储桶前缀隔离 + IAM**
- **Notebook 沙箱物理隔离**（每用户独立容器）

## 后果

### 积极
- 成本可控
- 资金路径（broker 账号）物理隔离，最大风险点不共享
- 大部分代码自动多租户（interceptor 注入）

### 消极
- 所有 SQL 必须 tenant 化（CI 检测）
- super_admin 越权风险（审计 + 双人）

### 跟进事项
- [ ] interceptor + RLS 自动化测试
- [ ] CH row policy 在租户创建时自动配置
- [ ] 跨租户访问审计告警
