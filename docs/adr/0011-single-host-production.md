# ADR 0011 — 单机生产部署（docker-compose）

> 状态：**已生效** | 日期：2026-05-19 | 编号：0011

## 上下文

早期文档（`docs/11-部署与运维手册.md`）规划生产环境采用 **K8s + Helm + ArgoCD** 多机集群。

实际业务规模评估后：

- **租户规模**：当前 < 100 个租户，单机性能完全够用
- **服务数**：合并后仅 4 后端 + 1 前端 = 5 服务（详见 ADR 0010）
- **行情/订单 QPS**：峰值 < 1k QPS，单机 16C/32G 余量极大
- **运维人力**：≤ 2 名 DevOps，K8s 学习/维护成本远大于收益
- **可用性目标**：99.5%（不追求 99.99%），单机 + 本地热备 + 异地冷备足够

## 决策

**生产环境采用单机 docker-compose 部署**，不使用 K8s/Helm/ArgoCD。

### 部署形态

| 环境 | 形态 | 说明 |
|---|---|---|
| **dev** | docker-compose | 开发者本机 |
| **staging** | docker-compose | 单机，与 prod 同构，配置弱化 |
| **prod** | docker-compose | **单机**，16C/32G/SSD ≥ 500GB，固定 IP |

### 单机部署架构

```
┌─────────────────────────────────────────────────────────┐
│  Production Host (Linux, Docker Engine + compose v2)   │
│                                                          │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │  frontend   │  │ trading-core │  │ md-gateway   │  │
│  │  (nginx:80) │  │  (9000)      │  │  (9001)      │  │
│  └─────────────┘  └──────────────┘  └──────────────┘  │
│                                                          │
│  ┌──────────────┐  ┌──────────────┐                    │
│  │ quant-engine │  │ assistant-svc│                    │
│  │  (9002)      │  │  (9003)      │                    │
│  └──────────────┘  └──────────────┘                    │
│                                                          │
│  Infra: postgres(17) clickhouse(25) redis(8)            │
│         nats(2.10) prometheus grafana loki vault        │
└─────────────────────────────────────────────────────────┘
```

### 高可用降级方案

- **服务进程级**：`restart: unless-stopped` + `healthcheck` 自动拉起
- **数据持久化**：Docker named volume，主机磁盘 RAID 1（或云盘）
- **备份**：
  - PG：每日 `pg_dump` → 异地对象存储
  - CH：每日 `BACKUP TABLE` → 异地
  - Redis：AOF + RDB 双开
  - 整机：每周快照
- **故障切换**：人工 RTO < 15 分钟（从异地备份恢复到备用机）

### 不做的事

- ❌ K8s / K3s
- ❌ Helm Chart
- ❌ ArgoCD
- ❌ HPA / 自动水平扩展
- ❌ 多副本服务
- ❌ Service Mesh（Istio/Linkerd）
- ❌ etcd / Consul（除已用的 NATS、Redis、Vault）

## 影响

### docs/11 重写

`docs/11-部署与运维手册.md` 生产章节全部按单机 compose 重写。

### deploy/ 目录精简

- `deploy/docker-compose.prod.yml`：唯一生产编排文件
- `deploy/helm/`：**不创建**
- `deploy/k8s/`：**不创建**

### CI/CD 简化

- 不需要 ArgoCD GitOps 流程
- Release：构建镜像 → 推到生产机 → `docker compose pull && docker compose up -d`
- 通过 SSH + 脚本部署，或用 GitHub Actions `appleboy/ssh-action`

### 监控降级

- 单机 Prometheus + Grafana + Loki + Tempo（all-in-one 模式）
- 不需要 Thanos / Cortex / Mimir 多副本聚合

## 备选方案（已否决）

### 方案 A：K8s 单节点（K3s）

否决：K3s 仍然引入 etcd/kubelet/容器运行时复杂性，对运维人员仍是负担，而单机 compose 是 Linux + Docker 的常识。

### 方案 B：多机 docker-compose + Swarm

否决：业务规模未达到需要多机；Swarm 生态弱，未来想迁 K8s 不如直接迁。

### 方案 C：Cloud Run / ECS

否决：项目对 broker 长连接、低延迟、固定 IP 有要求，托管 serverless 不合适。

## 未来扩展触发点

当出现以下情况时，重新评估升级到 K8s：

1. 租户数 > 1000
2. 峰值 QPS > 5000
3. 团队 DevOps 人数 ≥ 4
4. 单机 CPU/内存持续 > 70% 一周
5. 跨地域多 region 需求

## 关联

- ADR 0010：4+1 服务架构（为单机部署提供前提）
- `docs/11-部署与运维手册.md`：必须按本 ADR 重写
- `deploy/docker-compose.prod.yml`：唯一生产配置
