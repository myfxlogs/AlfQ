# 0008 - AI 策略助手限定工具集，不直接执行交易

| 字段 | 值 |
|---|---|
| 日期 | 2026-05-18 |
| 状态 | accepted |
| 决策者 | architecture, security, risk, product |
| 影响范围 | assistant-svc 与所有 LLM 集成 |
| 关联 ADR | 0004 |
| 关联 docs | docs/18 §B |

## 背景

引入 AI 策略助手（Assistant）让用户用自然语言生成策略。但 LLM 不可信、可能被 Prompt Injection 攻击、可能产生幻觉。

如果 LLM 能直接下单/部署，风险无法承受。

## 选项

### A. LLM 可调用任何后端 RPC
- 优点：能力最强
- 缺点：风险不可控；越权/欺骗/幻觉直接造成资金损失

### B. LLM 只能"建议"，所有动作必须用户手动重复
- 优点：最安全
- 缺点：体验差

### C. LLM 调用受限白名单工具，敏感动作需用户在 UI 显式确认
- 优点：能力与安全平衡
- 缺点：工具集设计需谨慎

## 决策

采用 **C**：

**允许工具**（只读 / 草稿）：
- 检索文档/数据（search_docs, list_symbols, get_bars）
- 校验/预览（validate_dsl, preview_factor）
- 创建草稿（create_factor_draft, create_strategy_draft）
- 启动回测（占用配额，**需用户确认**）

**禁止工具**（永不注册给 LLM）：
- 下单 / 撤单 / 修改风控
- 启停部署
- 修改账户 / 凭据
- 触发 KillSwitch
- 跨租户访问

**所有敏感操作必须在前端 UI 由用户主动点击触发，并经 TOTP 等已有审批流程**。

## 后果

### 积极
- LLM 即使被 Injection / 幻觉，资金路径仍受 UI + 风控保护
- 工具集 schema 严格，输出可审计
- 与现有审批/风控流程兼容

### 消极
- "AI 自动交易"这种宣传卖点不存在（我们也不打算做）
- 用户多一步点击

### 跟进事项
- [ ] assistant-svc 工具注册表 + schema 校验
- [ ] Prompt Injection 测试集（≥ 50 样本）
- [ ] 100% 审计 LLM 调用与工具调用
- [ ] 配额与计费机制
