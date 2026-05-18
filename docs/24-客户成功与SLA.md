# 24 - 客户成功与 SLA

> 多租户必备：SLA 承诺、工单系统、Status Page、客户成功流程。

## 1. SLA 等级

### 1.1 服务等级表（30 天滚动）

| 项目 | Basic | Pro | Enterprise |
|---|---|---|---|
| API 可用性 | 99.5% | 99.9% | 99.95% |
| 行情可用性 | 99.5% | 99.9% | 99.95% |
| 下单成功率 | 99.5% | 99.9% | 99.95% |
| 计划维护提前通知 | 24h | 48h | 72h |
| 工单 P1 响应 | 4h | 1h | 15min |
| 工单 P2 响应 | 8h | 4h | 1h |
| 工单 P3 响应 | 24h | 8h | 4h |
| 工单 P4 响应 | 72h | 24h | 8h |
| 客户成功经理 | — | 共享 | 专属 |
| 数据保留 | 90 天 | 365 天 | 730 天 |

### 1.2 SLA 违约赔偿（toB 模式预留）

| 月可用性 | 月费抵扣 |
|---|---|
| ≥ SLA | 0 |
| SLA-0.5% ~ SLA | 10% |
| SLA-2% ~ SLA-0.5% | 25% |
| < SLA-2% | 50% |

### 1.3 SLA 排除项

- 计划维护窗口
- 用户违规导致的封禁
- 第三方依赖（broker / LLM provider）故障
- 用户网络问题
- 外部 DDoS

## 2. 工单优先级

| 级别 | 定义 | 例 |
|---|---|---|
| **P1 严重** | 服务不可用 / 资金风险 | 无法登录 / 大面积拒单 / 资金对账异常 |
| **P2 高** | 主功能不可用，有 workaround | 单策略部署失败 |
| **P3 中** | 非核心功能问题 | UI bug / 报告生成慢 |
| **P4 低** | 咨询、建议 | 文档问题 / feature request |

## 3. 工单系统

### 3.1 流程

```
用户提交 → 自动分级 → 客服分类 → 工程跟进 → 升级 on-call lead（如需）
       → 解决/告知 → 用户确认/关闭 → 满意度评分（CSAT）
```

### 3.2 数据表

```sql
CREATE TABLE support_tickets (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    UUID NOT NULL,
  user_id      UUID NOT NULL,
  number       SERIAL UNIQUE,
  title        TEXT NOT NULL,
  category     TEXT NOT NULL,            -- bug/question/billing/security/feature
  priority     TEXT NOT NULL,
  status       TEXT NOT NULL,            -- open/pending/solved/closed
  assignee_id  UUID,
  trace_id     TEXT,
  artifacts    JSONB,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  first_response_at TIMESTAMPTZ,
  resolved_at  TIMESTAMPTZ,
  csat_score   SMALLINT
);

CREATE TABLE support_messages (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id   UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
  author_id   UUID NOT NULL,
  body        TEXT NOT NULL,
  attachments JSONB,
  internal    BOOLEAN NOT NULL DEFAULT FALSE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.3 RPC

`SupportTicketService`：CreateTicket / AddMessage / ListTickets / CloseTicket / RateCSAT。客服面有跨租户 List（仅平台运营角色）。

## 4. 支持渠道

| 渠道 | Basic | Pro | Enterprise |
|---|---|---|---|
| 平台内工单 | ✓ | ✓ | ✓ |
| 邮箱 support@alfq.io | ✓ | ✓ | ✓ |
| 文档/知识库 | ✓ | ✓ | ✓ |
| Slack/Telegram | — | 共享 | 专属 |
| 视频会议 | — | 月 1 次 | 周 1 次 + 按需 |
| 专属 CSM | — | — | ✓ |

## 5. Status Page

### 5.1 内容

`status.alfq.io`：
- 各服务实时状态
- 进行中事件
- 历史事件
- 计划维护
- 月度 SLA 报告

### 5.2 自动化

- Prometheus 告警 + on-call 决策更新
- 客户可订阅邮件 / Webhook / RSS

### 5.3 事件沟通模板

```
[Investigating] 2026-05-18 10:23 UTC
  正在调查 EURUSD 行情延迟。下次更新：30 分钟后。
[Identified] 10:45 UTC
  定位为 md-gateway OOM，正在重启扩容。
[Monitoring] 11:00 UTC
  恢复中，观察。
[Resolved] 11:30 UTC
  已解决。Postmortem 将在 48h 内发布。
```

## 6. 客户成功流程

### 6.1 Onboarding（7 天内）

1. 欢迎邮件 + 入门文档
2. 引导：第一次创建策略 → 回测 → paper 部署
3. 30 天内主动调研

### 6.2 健康度评分

每周计算 `customer_health_score`：

```
score = w1*活跃天数 + w2*策略数 + w3*CSAT - w4*P1工单 - w5*配额触顶
```

低分租户主动触达。

### 6.3 续约 / 升级

- 续约前 30 天 CSM 触达
- 配额逼近 → 推荐升级
- 数据导出 / 流失挽回流程

## 7. 内部 SLO vs 外部 SLA

| 类型 | 含义 |
|---|---|
| **SLI** | 内部测量值 |
| **SLO** | 内部目标（比 SLA 严格 0.05~0.1%） |
| **SLA** | 对外承诺 |

错误预算 = 1 - SLO，决定发布门禁（见 17 章）。

## 8. Postmortem

P1 / P2 事件 48h 内发布：

```markdown
# Postmortem: <事件标题>
## 摘要 / 影响范围 / 时间线
## 根因
## 应急动作
## 后续改进（含 owner + due date）
## 经验教训
```

模板存 `docs/postmortems/`。

## 9. 客服与隐私

- 客服查看用户数据走 **impersonation**：
  - 用户授权一次性 token，或
  - super_admin 紧急介入（必须审计 + 双人）
- 工单中附带的截图、日志必须脱敏（自动 redact PII）

## 10. AI Agent 实施要求

- [ ] `support_tickets` / `support_messages` 表 + RPC
- [ ] 工单前端页（用户 + 客服两套视图）
- [ ] 健康度评分定时 job
- [ ] Status Page 接入（Atlassian / 自建）
- [ ] 与 Prometheus 告警的双向同步
- [ ] CSAT 邮件模板
- [ ] Postmortem 模板与归档

## 11. 验收

- [ ] 工单完整流程跑通（创建 → 回复 → 关闭 → CSAT）
- [ ] SLA 履约月报可自动生成
- [ ] Status Page 与告警联动
- [ ] 健康度评分按周产出
- [ ] Impersonation 全程审计
