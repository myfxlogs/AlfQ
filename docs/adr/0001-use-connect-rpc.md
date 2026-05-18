# 0001 - 采用 Connect RPC + SSE 替代 REST + WebSocket

| 字段 | 值 |
|---|---|
| 日期 | 2026-05-18 |
| 状态 | accepted |
| 决策者 | architecture |
| 影响范围 | 全局（所有对外/对内 API） |
| 关联 ADR | — |
| 关联 docs | docs/01, docs/03, docs/04 |

## 背景

需要选择一套覆盖以下场景的通信协议：
- 浏览器 ↔ Admin API
- Go 服务之间
- 服务器推送（行情、订单事件、风控告警）
- Python 研究层调 API

候选传统方案：REST + WebSocket。问题：
- Schema 多源（OpenAPI + WS schema）
- 前端类型需手写或额外 codegen
- WS 经常被代理/防火墙影响
- 服务端要同时维护两套 handler

## 选项

### A. REST + WebSocket
- 优点：业界默认，工具齐
- 缺点：Schema 多源、前端类型脆弱、WS 维护成本、过代理不稳

### B. gRPC + grpc-gateway
- 优点：内部一致，gateway 出 REST
- 缺点：gateway 增加一层，TS 客户端弱，浏览器走 grpc-web 体验差

### C. Connect RPC + Server Streaming（SSE）
- 优点：一份 .proto 同时出 gRPC / gRPC-Web / Connect 三协议；浏览器走 HTTP/JSON 即可；Server Streaming 自动以 SSE 传输；TS 客户端强类型；与 grpc 服务端同源
- 缺点：生态较新（2022+），社区比 grpc 小

### D. GraphQL + Subscriptions
- 优点：前端灵活
- 缺点：服务端复杂度高，限流/缓存难，业务场景非通用查询

## 决策

我们选择 **C. Connect RPC + Server Streaming（SSE）**。

理由：
- 单一 schema 源（`.proto`），TS/Go/Python 强类型自动生成
- 浏览器零额外抽象（HTTP/JSON 也能调）
- 流式天然走 SSE，过代理穿透好
- 服务端一套 handler 同时服务 gRPC / Connect / gRPC-Web
- 内外协议统一，降低团队心智成本

## 后果

### 积极
- 减少协议层重复工作
- 前后端类型契约严格
- 服务器推送场景无需独立 WS 实现

### 消极 / 成本
- 团队需熟悉 buf / connect-go / connect-web
- 部分 IDE/工具对 .proto 支持需配置
- Connect 协议社区比 REST 小，问题需自查

### 跟进事项
- [x] 文档 03 章规定 proto 包结构与代码生成流程
- [x] M0 落地 buf.yaml / buf.gen.yaml（含前端 TS V2 生成：`buf.build/bufbuild/es` + `buf.build/connectrpc/es`）
- [ ] M0 落地 admin-api 装载 Connect handler 骨架
- [ ] CI 跑 `buf lint` + `buf breaking`
- [ ] M1 起逐步生成前端 `src/gen/alfq/v1/*.pb.ts` / `*_connect.ts`

## 验收

- [x] 评审通过
- [ ] M0 代码引用本 ADR
- [x] docs/03-API与接口规范.md 已对齐
