# ADR 0010 — 后端服务从 7 合并为 4（5 服务总架构）

> 状态：**已生效**（不可逆） | 日期：2026-05-19 | 编号：0010

## 上下文

M0–M2 阶段，后端按职责分为 **7 个 Go 微服务**：

```
admin-api : 8080
md-gateway: 9001
factor-svc: 9002
strategy-svc:9003
risk-svc  : 9004
oms       : 9005
assistant-svc:9006
```

实际运行后发现：

1. **粒度过细**：单团队（≤ 10 人）维护 7 个服务，运维成本远大于收益
2. **下单热路径跨进程**：`admin-api → oms → risk-svc` 三跳网络 RTT，量化场景延迟敏感
3. **故障域虚假分离**：实际部署在同一台机器/同一 K8s namespace，进程隔离的 fault tolerance 收益微弱
4. **量化数据流耦合**：`factor-svc` 输出几乎只被 `strategy-svc` 消费，中间走 NATS 序列化纯粹浪费

## 决策

合并为 **4 个后端服务 + 1 个前端服务 = 5 个服务**。

### 合并方案

| 新服务 | 端口 | 由原服务合并 | 职责 |
|---|---|---|---|
| **`trading-core`** | 9000 | `admin-api` + `oms` + `risk-svc` | 交易主链路：API、认证、订单状态机、风控 |
| **`quant-engine`** | 9002 | `factor-svc` + `strategy-svc` | 量化引擎：因子计算 + 策略评估 |
| **`md-gateway`** | 9001 | （保留） | 行情接入（MT4/MT5 异构协议） |
| **`assistant-svc`** | 9003 | （保留） | AI 助手（独立故障域，不影响交易） |
| **`frontend`** | 80 | （保留） | React SPA + Nginx，`/api/` 反代到 `trading-core` |

### 端口废弃

`8080`（admin-api 独立端口）、`9004`（risk-svc）、`9005`（oms）**不再使用**；`9003`（strategy-svc）已合并，`9006`（assistant-svc 旧端口）已调整，当前 assistant-svc 使用 `9003`。

### 合并保留独立的理由

- **`md-gateway` 保留**：唯一与外部 broker（MT4/MT5）建立长连接的服务，故障隔离重要；且需要独立水平扩展（每 broker 一个实例）
- **`assistant-svc` 保留**：调用外部 LLM API（OpenAI/Anthropic），延迟和稳定性独立于交易主链路，必须能单独熔断
- **`trading-core` 合并的逻辑**：API 层、订单管理、风控引擎在下单路径上**强同步耦合**，必须一个进程内调用，否则每次下单都要 3 次网络 RTT
- **`quant-engine` 合并的逻辑**：因子→策略数据流是单向纯计算管道，进程内通信比 NATS 序列化快两个数量级

## 影响

### 代码组织

- **`backend/go/internal/`**：保持原样（`adminapi/`、`oms/`、`risksvc/`、`factorsvc/`、`strategysvc/` 子包不动）
- **`backend/go/cmd/`**：从 7 个目录变成 4 个：`trading-core/`、`quant-engine/`、`md-gateway/`、`assistant-svc/`
- 业务逻辑代码 **零修改**，只是 `main.go` 重新组合

### 部署

- `deploy/docker-compose.prod.yml` 5 个 service：`trading-core`、`quant-engine`、`md-gateway`、`assistant-svc`、`frontend`
- 前端通过 Nginx 反代访问 `/api/`，避免 CORS

### Makefile

```
go-build: cmd/trading-core cmd/quant-engine cmd/md-gateway cmd/assistant-svc
```

### 性能预期

- 下单链路：`admin-api → oms → risk-svc` 3 次 RTT（~1.2ms）→ 进程内函数调用（< 0.05ms）
- 因子→策略：NATS publish/subscribe → Go channel（同等量级提速）

## 备选方案（已否决）

### 方案 A：保留 7 服务

否决：单团队 ≤ 10 人，运维 7 个服务的成本远大于"伪故障隔离"的收益。

### 方案 B：合并为 1 个 monolith

否决：`md-gateway` 必须独立扩展（每 broker 一实例）；`assistant-svc` 必须独立熔断（外部 LLM 不可控）。

### 方案 C：合并为 3 服务（trading-core + md-gateway + ai+quant）

否决：`assistant-svc` 与 `quant-engine` 合并会让 LLM 故障污染量化引擎的稳定性。

## 后续

- 旧 ADR 中提及 7 服务的，仍有效（这些 ADR 的核心决策与服务数无关）
- `docs/01-总体架构与技术决策.md` §1 系统上下文图已更新
- `docs/11-部署与运维手册.md` 已更新部署图
- `AGENT.md` "三域结构"段已更新
- 任何新文档**禁止**再使用 `admin-api`、`factor-svc`、`strategy-svc`、`risk-svc`、`oms` 这些已合并服务名，改用合并后的服务名

## 关联

- 取代了 `docs/01 §1` 旧的 7 服务图
- 部分撤销 `docs/19-架构决策记录.md` 中的服务划分（架构演化是正常事件，非违反 ADR）
