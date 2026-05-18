# 0007 - 数据访问采用 sqlc 生成代码，不引 ORM

| 字段 | 值 |
|---|---|
| 日期 | 2026-05-18 |
| 状态 | accepted |
| 决策者 | architecture, backend |
| 影响范围 | backend/go |
| 关联 ADR | 0003 |
| 关联 docs | docs/12 §3.6 |

## 背景

Go 服务访问 PostgreSQL，需在性能、可维护性、安全（SQL 注入）之间选择。

## 选项

### A. database/sql + 手写 SQL
- 优点：零依赖、可控
- 缺点：样板多、易错、类型不安全

### B. GORM（ORM）
- 优点：开发快
- 缺点：N+1 隐患、隐式行为、性能黑盒、复杂查询难表达

### C. sqlc（SQL-first 代码生成）
- 优点：写真 SQL，编译期类型检查，零运行时反射，性能等同手写
- 缺点：DDL/Query 修改需重生成

### D. ent / bun
- 优点：类型安全
- 缺点：仍有 ORM 思维 / 学习曲线

## 决策

采用 **C. sqlc**。

约束：
- DDL 由 goose 迁移管理
- Query 文件位于 `backend/go/internal/<svc>/repo/queries/*.sql`
- `sqlc generate` 出 Go 代码到 `backend/go/internal/<svc>/repo/gen/`
- 复杂分析查询仍可手写
- 多表 join 优先用 SQL 视图

## 后果

### 积极
- SQL 一目了然
- 类型安全
- 性能可预测
- 安全（参数化）

### 消极
- Query 变更需 generate
- 不同方言支持有限（PG 优先）

### 跟进事项
- [ ] sqlc.yaml 配置
- [ ] CI 检查 `sqlc diff` 无未生成变更
