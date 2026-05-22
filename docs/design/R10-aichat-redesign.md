# R10 · AIChat 页面重构设计（完整版）

> 日期：2026-05-22 | 关联：R10 + R22 | 版本：v2

## 设计原则

- **单页一体**：配置 + 对话 + 用量统计同页，不跳转
- **租户隔离**：每个用户使用自己设置的 API Key，不共享
- **简洁优先**：默认显示对话区，配置区通过齿轮展开
- **不留负债**：多租户隔离、用量统计、RAG 知识库一次做完

---

## 页面布局

```
┌──────────────────────────────────────────────────┐
│ AI 助手                           [⚙️] [📊]     │  ← 齿轮=配置, 📊=用量
├──────────────────────────────────────────────────┤
│ ┌─ 配置面板（齿轮展开）─────────────────────┐    │
│ │ 服务商:  [OpenAI ▼]                        │    │
│ │ API Key: [sk-...                    ] [测试]│    │
│ │ 模型:    gpt-4o                             │    │
│ │ 状态:    ● 已连接   延迟 180ms              │    │
│ │ ☑ RAG 知识库检索（使用文档库增强回复）       │    │
│ └──────────────────────────────────────────────┘    │
│                                                    │
│ ┌─ 用量面板（📊展开）───────────────────────┐     │
│ │ 今日:  12,450 tokens  ·  本月: 89,200      │     │
│ │ 预算:  $5.00 / 月    ·  已用: $1.24 (24%)  │     │
│ │ ████░░░░░░░░░░░░░░░░░░░░░░░                 │     │
│ └──────────────────────────────────────────────┘     │
│                                                    │
│ ┌──────────────────────────────────────────────┐   │
│ │ [A] 你好，我是 ALFQ 策略助手。我可以：       │   │
│ │     · 帮您编写策略因子 DSL                   │   │
│ │     · 查询账户状态和持仓                     │   │
│ │     · 解释技术指标和回测结果                 │   │
│ │                                              │   │
│ │                   [U] 帮我写一个 EURUSD 的   │   │
│ │                       SMA20/60 交叉策略      │   │
│ │                                              │   │
│ │ [A] 好的，以下是策略 Spec：                  │   │
│ │     ```json                                  │   │
│ │     {"factors": {"sma20": "sma($close,20)"}} │   │
│ │     ```                                      │   │
│ │     [复制] [保存为草稿] [运行回测]           │   │
│ └──────────────────────────────────────────────┘   │
│                                                    │
│ [输入消息…                               ] [发送] │
└──────────────────────────────────────────────────┘
```

---

## 多租户 API Key 隔离

### 存储

```sql
CREATE TABLE user_api_keys (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id),
    provider   text NOT NULL,          -- "openai" | "anthropic"
    model      text NOT NULL DEFAULT '',
    key_cipher text NOT NULL,          -- AES-256-GCM encrypted
    key_prefix text NOT NULL,          -- "sk-...****a1b2"
    quota_limit_cents int NOT NULL DEFAULT 500,  -- 月度预算（美分）
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id, provider)
);
```

### 读写流程

```
写入：用户输入 key → 前端调 UpdateSystemSetting
  → 后端 AES 加密 → INSERT INTO user_api_keys

读取：assistant-svc 收到 /chat 请求
  → 从 JWT 提取 user_id
  → SELECT key_cipher FROM user_api_keys WHERE user_id=$1 AND provider='openai'
  → AES 解密 → 注入 Router
```

### 隔离保证

- 每个 provider 每个 user 一条记录
- JWT 中的 `sub`（user_id）作为查询条件
- 不同用户之间 key 物理隔离（不同行）
- 管理员也无法查看其他用户的明文 key（AES 加密）

---

## 用量统计

### 存储

```sql
CREATE TABLE ai_usage_logs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL,
    provider    text NOT NULL,
    model       text NOT NULL,
    tokens_in   int NOT NULL DEFAULT 0,
    tokens_out  int NOT NULL DEFAULT 0,
    cost_cents  int NOT NULL DEFAULT 0,   -- 美分
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_usage_user_date ON ai_usage_logs(user_id, created_at);
```

### 统计维度

| 维度 | 来源 |
|---|---|
| 今日 tokens | `SUM(tokens_in + tokens_out) WHERE created_at::date = today` |
| 本月 tokens | `SUM(...) WHERE date_trunc('month', created_at) = this_month` |
| 预算 | `user_api_keys.quota_limit_cents` |
| 已用 | `SUM(cost_cents) WHERE date_trunc('month', created_at) = this_month` |

### 扣费时机

```
assistant-svc Chat() 返回 HTTP response
  → 解析 response.usage.total_tokens / prompt_tokens / completion_tokens
  → 计算 cost（按模型定价）
  → INSERT INTO ai_usage_logs
  → 如果超出 quota_limit_cents → 返回 429 Too Many Requests
```

---

## RAG 知识库检索

### 架构

```
用户消息 → assistant-svc
  ├─ 1. Embed(user_message) → query vector
  ├─ 2. pgvector cosine_search(docs_embeddings) → top-3 chunks
  ├─ 3. 拼接 system prompt:
  │      "你是 ALFQ 策略助手。参考以下文档回答问题：
  │       [chunk1] [chunk2] [chunk3]
  │       如果不确定，直接说不知道。"
  └─ 4. Chat(system_prompt, user_message) → 回复
```

### 索引流程（首次运行或文档更新时）

```
docs/ 目录扫描 → 分块（每 500 字，重叠 50 字）
  → OpenAI Embed(text) → vector
  → INSERT INTO docs_embeddings (chunk, embedding, source_file)
```

### 存储

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE docs_embeddings (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    chunk       text NOT NULL,
    embedding   vector(1536) NOT NULL,
    source_file text NOT NULL,
    chunk_idx   int NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_docs_embedding ON docs_embeddings USING ivfflat (embedding vector_cosine_ops);
```

---

## 完整文件改动清单

| 文件 | 改动 |
|---|---|
| `frontend/src/pages/AIChat.tsx` | 重写：配置面板 + 对话 + 用量条 |
| `backend/go/internal/adminapi/system_settings_handler.go` | Key 存取 + AES 加密 |
| `backend/go/internal/assistantsvc/runner.go` | 启动从 DB 读 key + RAG 初始化 |
| `backend/go/internal/assistantsvc/provider.go` | Chat 返回 usage + 用量写入 |
| `backend/go/internal/assistantsvc/knowledge.go` | RAG 检索实现 |
| `backend/go/migrations/012_user_api_keys.sql` | 新表 |
| `backend/go/migrations/013_ai_usage_logs.sql` | 新表 |
| `backend/go/migrations/014_docs_embeddings.sql` | 新表 + pgvector |

---

## 验收

```bash
# 1. 多租户隔离
用户A 设置 key → 对话成功 → 用户B 无 key → 对话拒绝
# 2. 用量统计
对话 3 轮后 → 刷新页面 → 今日用量 > 0
# 3. 超预算拦截
设置 quota_limit=1 美分 → 对话 → 返回 "月度预算已用尽"
# 4. RAG 检索
输入 "如何计算 Sharpe 比率" → 回复引用 docs/ 内容
```

---

## 预估

| 步骤 | 时间 |
|---|---|
| 数据库迁移（3 表） | 30 min |
| 后端 handler（加密 + 用量 + RAG） | 2 h |
| assistant-svc 集成 | 1 h |
| 前端 AIChat 重写 | 2 h |
| 联调验证 | 1 h |
| **合计** | **~6.5 h** |
