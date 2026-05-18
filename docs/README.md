# ALFQ 文档索引

> 本目录是**给 AI Agent 实施的指令集合**。所有文档遵循"可落地、可验收、可测试"原则，包含明确的目录结构、接口签名、字段定义和验收标准。
>
> **实施顺序**：先读 `00` 总览，再读 `12` 实施指南（约束/规范/验收），然后按 M0→M6 里程碑顺序消费 01~11。

## 文档清单

| 序号 | 文档 | 描述 |
|---|---|---|
| 00 | [企业级量化交易系统落地方案.md](./企业级量化交易系统落地方案.md) | 总览、目标、里程碑（已存在） |
| 01 | [01-总体架构与技术决策.md](./01-总体架构与技术决策.md) | 架构图、技术栈、协议（Connect+SSE）、分层职责 |
| 02 | [02-数据库设计.md](./02-数据库设计.md) | PG/CH/Redis 完整 DDL、索引、分区、迁移 |
| 03 | [03-API与接口规范.md](./03-API与接口规范.md) | Connect RPC 服务定义、SSE 流、错误码 |
| 04 | [04-前端设计.md](./04-前端设计.md) | React + TS + shadcn/ui，页面/路由/状态/组件 |
| 05 | [05-多租户与权限设计.md](./05-多租户与权限设计.md) | Tenant 隔离、RBAC、JWT、ACL、RLS |
| 06 | [06-Python策略沙箱设计.md](./06-Python策略沙箱设计.md) | 研究 Notebook 沙箱、策略部署模型（DSL+ONNX） |
| 07 | [07-安全设计.md](./07-安全设计.md) | 秘钥、mTLS、审计、Kill Switch、合规 |
| 08 | [08-Go服务实现规范.md](./08-Go服务实现规范.md) | 6 个 Go 服务的实现细节与目录结构 |
| 09 | [09-因子DSL规范.md](./09-因子DSL规范.md) | DSL 语法、算子表、Go/Py 解释器对齐 |
| 10 | [10-Python研究层实现规范.md](./10-Python研究层实现规范.md) | 数据/因子/回测/优化/报告 |
| 11 | [11-部署与运维手册.md](./11-部署与运维手册.md) | Docker、K8s Helm、CI/CD、Runbook |
| 12 | [12-AI-Agent实施指南.md](./12-AI-Agent实施指南.md) | **AI Agent 必读**：编码规范、提交规范、验收标准、文件大小与复杂度上限 |
| 13 | [13-参考项目研习指南.md](./13-参考项目研习指南.md) | **先抄后改**：开源项目克隆、模块映射、阅读路线 |
| 14 | [14-领域模型与交易规则.md](./14-领域模型与交易规则.md) | **金融事实层**：MT 会计口径、滑点/手续费/swap 公式、交易日历、资金分配 |
| 15 | [15-可观测性详细规范.md](./15-可观测性详细规范.md) | 指标命名、日志字段、trace 属性、SLO、告警规则表 |
| 16 | [16-测试与质量保证.md](./16-测试与质量保证.md) | 测试金字塔、契约测试、Parity、性能基准、混沌 |
| 17 | [17-发布与变更管理.md](./17-发布与变更管理.md) | Feature Flag、灰度、变更评审、回滚 playbook、值班 |
| 18 | [18-AI-Agent工作流深化与策略助手.md](./18-AI-Agent工作流深化与策略助手.md) | **三部分**：研发 Agent 防漂移 · AI 策略助手产品设计 · ML 模型治理 |
| 19 | [19-架构决策记录.md](./19-架构决策记录.md) | ADR 模板 + 首批 8 份 ADR（存 `docs/adr/`） |
| 20 | [20-错误码与异常处理规范.md](./20-错误码与异常处理规范.md) | 全量错误码、Connect Code 映射、重试矩阵、跨语言实现 |
| 21 | [21-跨服务一致性与幂等.md](./21-跨服务一致性与幂等.md) | Saga / Outbox / 幂等键 / 对账 / 死信队列 |
| 22 | [22-数据治理与合规矩阵.md](./22-数据治理与合规矩阵.md) | 数据分级 / PII 矩阵 / 保留期 / GDPR / 模型治理 |
| 23 | [23-限流熔断与配额.md](./23-限流熔断与配额.md) | 多层限流、熔断、舱壁、超时、租户配额、计费预留 |
| 24 | [24-客户成功与SLA.md](./24-客户成功与SLA.md) | SLA 等级、工单、Status Page、Postmortem、Onboarding |
| 25 | [25-Proto代码生成与gRPC调用规范.md](./25-Proto代码生成与gRPC调用规范.md) | buf generate 全流程、业务 proto vs mtapi 官方 proto、`google.golang.org/grpc` 调用规范、MT4/MT5 远端入口 |
| ⚡ | [M0-START.md](./M0-START.md) | **AI Agent 启动指令**：M0 范围、必读顺序、PR 拆分、验收 |
| 📋 | [AUDIT-2026-05-18.md](./AUDIT-2026-05-18.md) | **文档审核报告**：9 类一致性问题与修复记录、唯一源指引、CI 检查建议 |

## 全局约定（所有 Agent 必须遵守）

1. **Monorepo 根目录**：`/opt/alfq/`，前后端三域独立：`backend/` `research/` `frontend/`
2. **语言版本**：Go 1.22+；Python 3.12（uv 管理）；Node 20+；TypeScript 5.4+
3. **所有跨服务通信**：Connect RPC（对外）/ gRPC（内部），不用 REST
4. **服务器推送**：Server Streaming RPC（SSE 传输），不用 WebSocket
5. **协议单一源**：所有消息定义在 `backend/proto/alfq/v1/`，`buf generate` 出 Go/TS/Python stub
6. **配置**：YAML + Viper + fsnotify 热加载，秘钥走 Vault
7. **日志**：结构化 JSON，必带 `trace_id` / `tenant_id` / `user_id` / `request_id`
8. **每个 PR**：必须附带 (a) 单元测试 (b) 文档更新 (c) ADR（若涉及架构）
9. **文件大小与复杂度有硬性上限**（详见 12 章 §3.5），CI 强制
10. **每个模块/服务的 README 顶部必须标注参考项目来源**（详见 13 章）

## 决策摘要（来自讨论）

| 议题 | 决策 |
|---|---|
| 实盘语言 | **Go 1.22+** |
| 研究语言 | **Python 3.12** |
| 前后端 | **分离**，前端独立 SPA |
| 协议 | **Connect RPC + SSE（Server Streaming）** |
| 关系库 | **PostgreSQL 16** |
| 时序库 | **ClickHouse 24+** |
| 缓存/会话 | **Redis 7** |
| 对象存储 | **MinIO（dev）/ S3（prod）** |
| 消息总线 | **NATS JetStream** |
| 用户策略执行 | **Sandbox（DSL + ONNX，禁止用户 Python 入生产）** |
| 多租户 | **逻辑隔离 + PG RLS**，broker 账号物理隔离 |
| 权限 | **RBAC + 资源 ACL**，JWT 短期 + Refresh，敏感操作 TOTP |
| 秘钥管理 | **HashiCorp Vault** |
| 部署 | **Docker Compose（dev）/ Kubernetes + Helm（prod）** |
