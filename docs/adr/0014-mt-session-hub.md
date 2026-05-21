# ADR-0014: MT Session Hub

**状态**: Accepted  
**日期**: 2026-05-21  
**决策者**: AI Agent (DeepSeek TUI)

## 上下文

`docs/adr/0010-consolidate-services.md` 确立了 4+1 服务划分，其中 `md-gateway` 是唯一持有 MT4/MT5 长连接的服务。`docs/enhancements/ARCHITECTURE-OPTIMIZATION.md` §6.2 识别出当前架构的根因：`trading-core` 和 `md-gateway` 各自独立 Dial MT，导致双倍连接数 + 同步竞态。

## 决策

**所有 MT 长连接收敛到 `md-gateway`，通过内部 gRPC 服务 `alfq.mthub.v1.MtHubService` 暴露。**

- `md-gateway` 新增 `internal/mthub/` 包，实现 SessionHub + MtHubService（ConnectRPC）。
- `trading-core` 删除 MT gRPC 直连，改用 `mthub.Client` 借用 session。
- CLI 工具（`symbol-sync`、`md-backfill`）默认走 mthub，保留 `--direct` 兜底。

## 后果

| 方面 | 影响 |
|---|---|
| **正面** | trading-core 不再直接接触 MT，可独立水平扩展；MT session 数从 2 → 1 每账户；CLI 启动时间 ~3s → ~100ms |
| **负面** | md-gateway 成为 MT 连接的单点；需新增内部 RPC 层 |
| **缓解** | md-gateway 已有 `restart: unless-stopped` + healthcheck；mthub 内部 RPC 不暴露到 Nginx |

## 关联

- `docs/adr/0010-consolidate-services.md` — 服务划分约束（本文遵守）
- `docs/enhancements/2026-05-21-final-optimization-plan.md` — 实施计划（MH-1~5）
- `proto/alfq/mthub/v1/mthub.proto` — RPC 定义
- `deploy/grafana/dashboards/mt-hub.json` — 可观测面板
