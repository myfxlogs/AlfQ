# 0006 - 采用 HashiCorp Vault 管理秘钥

| 字段 | 值 |
|---|---|
| 日期 | 2026-05-18 |
| 状态 | accepted |
| 决策者 | architecture, security |
| 影响范围 | 全局 |
| 关联 ADR | — |
| 关联 docs | docs/07 |

## 背景

秘钥（JWT 签名 key、TOTP secret、DB 凭据、第三方 API Key、内部 PKI）的存储与轮换需求。

> **注**：broker 账号密码因 MetaTrader 协议要求明文递交上游交易服务器，不纳入 Vault 管理，改为 PG 明文存储（依赖磁盘加密 + RLS + 审计补偿），见 doc 02 §1.3。

## 选项

### A. K8s Secrets（base64）
- 优点：零依赖
- 缺点：实际明文，etcd 加密需另配；无动态凭据；无审计

### B. 云 KMS + Secrets Manager（AWS/GCP/Azure）
- 优点：托管
- 缺点：云锁定；多云/自建场景不通用

### C. HashiCorp Vault
- 优点：动态凭据（DB/PKI）、Transit 加密、审计、KV v2、多后端、K8s 集成（Agent Injector）、跨云中立
- 缺点：自运维（HA + unseal）

### D. SOPS + Git
- 优点：版本化
- 缺点：无动态、无轮换、审计弱

## 决策

采用 **C. Vault**：
- KV v2：长生命周期秘钥（不含 broker 密码，见背景注）
- Transit：字段级加密（TOTP secret）
- Database：动态 PG/CH 凭据
- PKI：内部 mTLS 证书签发
- Agent Injector：sidecar 注入 Pod

## 后果

### 积极
- 凭据生命周期可控、可审计
- 内部服务 mTLS 自动化
- 不锁定云

### 消极
- 多一套服务运维（HA、unseal、备份）
- 学习曲线

### 跟进事项
- [ ] Vault HA 部署 Helm chart
- [ ] Unseal Runbook（docs/runbook/vault-unseal.md）
- [ ] 每服务 Vault Role + Policy 模板
