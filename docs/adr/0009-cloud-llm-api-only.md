# 0009 - 仅使用云端 LLM API，不自建/不部署本地大模型

| 项 | 值 |
|---|---|
| 日期 | 2026-05-18 |
| 状态 | accepted |
| 决策者 | architecture, product, security, ops |
| 影响范围 | `assistant-svc`、所有 LLM 集成、M6.5 里程碑、运维栈 |
| 关联 ADR | 0004（禁止用户 Python 进生产）、0008（LLM 受限白名单工具） |
| 关联 docs | docs/01 §2.5、docs/12 §2、docs/18 §B、docs/23 §5/§6 |

## 背景

AI 策略助手（doc 18 §B）需要大语言模型能力。可能路径有三：

1. 仅调用云端商用 LLM API（OpenAI / Anthropic / Google / 国内厂商等）
2. 自建本地 LLM（vLLM / Ollama / TGI 部署开源模型）
3. 云 + 本地混合

原初稿 doc 18 §B10 写了"M6.5 多模型 / 本地化 = vLLM 自建 / 多 provider 切换"，§B5.5 还提出过"本地 LLM only 开关"。**该方向已废弃**，本 ADR 给出正式决策。

## 决策

**ALFQ 平台不自建、不部署、不维护本地大模型。所有 LLM 能力一律通过云端商用 API 接入。**

具体约束：

1. **禁止**在 `deploy/` 中出现 GPU 节点、vLLM / Ollama / TGI / LocalAI / llama.cpp / SGLang 等本地推理服务的镜像、Helm、compose 片段或 K8s manifest。
2. **禁止**新增模型权重文件（`.safetensors` / `.gguf` / `.bin` / `.pt`，ONNX 交易模型除外，见 ADR 0004）进入仓库或 S3 的"模型推理"路径。
3. **禁止**在 `assistant-svc` 中实现「内置推理引擎 / 进程内模型加载 / GPU 调度」等代码路径；只保留 HTTP/Connect 出站调用云 LLM 的 client。
4. **允许并要求**：在 `assistant-svc` 实现 **provider 抽象层**，至少支持两家云供应商（一主一备），用于跨厂商故障切换与成本路由。Provider 列表通过 YAML + Vault 凭据注入，**不写死**。
5. **允许**：Embedding 调用云端 API（如 OpenAI embedding、Voyage、智源在线），向量入 pgvector；**禁止**部署本地 embedding 模型服务。

## 备选方案

### 方案 A：本地自建大模型（vLLM / Ollama）—— **拒绝**
- 优点：数据不出域、长期 token 成本可控、可定制
- 缺点：
  - GPU 采购 / 托管 / 运维 / 监控成本 → 偏离量化平台核心
  - 模型迭代速度慢于云厂商，能力始终落后 1-2 代
  - 安全更新、CVE、推理加速框架升级负担转嫁给本团队
  - 团队无 ML Ops / 推理优化人力配置

### 方案 B：云 + 本地混合 —— **拒绝**
- 优点：隐私场景可走本地
- 缺点：双栈维护，所有限流/熔断/审计/计费/Prompt 测试都要做两遍；隐私场景应通过云厂商企业级合规（如 OpenAI Enterprise / Azure OpenAI / 国内政企版）解决，而非自建

### 方案 C：仅云端商用 API —— **采纳**
- 优点：能力最强、零运维、随业务量按需付费、聚焦核心业务
- 缺点：依赖外部、需做配额控制与隐私脱敏

## 后果

### 积极
- M6.5 范围**显著缩窄**：仅做 provider 抽象 + 多家切换 + 成本路由，无 GPU 运维负担
- 部署架构简化：仍是 docker-compose（dev）/ K8s（prod），**无 GPU 节点要求**
- 安全/合规由云厂商企业合同分担
- 模型升级零迁移成本（切版本号）

### 消极 / 风险
- 完全依赖外部 API，必须做：熔断（doc 23 §5）、配额（doc 23 §6）、Prompt 数据脱敏（doc 18 §B5.5）
- token 成本随用量线性增长 → 通过 doc 23 §6 配额上限保护

### 跟进事项

- [ ] doc 12 §2 里程碑表：M6.5 改为"多 provider 切换 / 成本路由 / 容灾"
- [ ] doc 18 §B2 架构图：删除"本地 vLLM"
- [ ] doc 18 §B5.5：删除"本地 LLM only 开关"，改为"企业 plan 走云厂商合规端点 + 字段脱敏"
- [ ] doc 18 §B10：M6.5 重写
- [ ] doc 19 §8 backlog ADR 0015 LLM Provider 抽象层 → 明确为「云端 provider 抽象」
- [ ] `deploy/` Helm / compose 不得引入 GPU runtimeClass / NVIDIA device plugin / vLLM 镜像
- [ ] `assistant-svc` 代码评审 checklist：禁止引入 `vllm` / `ollama` / `transformers` / `torch` / `accelerate` 等本地推理依赖

## 不在本 ADR 范围

- **ONNX 交易模型**（strategy-svc 内嵌的 Go ONNX Runtime）不受影响 — 那是受控的、确定性的、低维输入输出的判别式模型，不是 LLM，见 ADR 0004。
- **pgvector 向量检索**不受影响 — 数据库侧能力，向量由云端 embedding API 生成。
