# ALFQ Runbook

> M6 灰度上线运维手册

## 部署概览

| 服务 | 端口 | 部署方式 |
|---|---|---|
| admin-api | 8080 | Docker / K8s |
| md-gateway | 9001 | Docker / K8s |
| factor-svc | 9002 | Docker / K8s |
| strategy-svc | 9003 | Docker / K8s |
| risk-svc | 9004 | Docker / K8s |
| oms | 9005 | Docker / K8s |

## Paper → Live 灰度流程

1. Paper 环境跑通完整回测 + 模拟交易
2. 策略部署到 paper 账号验证 24h 稳定
3. 配置 Feature Flag（先在 paper 全量）
4. 生产环境：1% → 10% → 50% → 100% 灰度
5. 每个阶段观察 1h，无异常继续
6. 出现异常立即回滚

## 紧急操作

### Kill Switch
```bash
# 全局止损
curl -X POST http://admin-api:8080/killswitch -d '{"scope":"global","reason":"manual"}'
```

### 回滚服务
```bash
docker compose -f deploy/docker-compose.yml up -d --force-recreate <service>
```

### 数据回滚
```bash
# PG 回滚到最近快照
pg_restore -d alfq latest_backup.dump
```

## 监控告警

- P1（立即响应）：Kill Switch 激活、broker 断连 >5min、margin level <100%
- P2（30min内）：策略 RPS 异常、CH 写入延迟 >30s、心跳超时
- P3（工作日）：Flag 过期未清理、磁盘 >80%

## 日志查询
```bash
# 按 trace_id 查全链路
make dev-logs | grep "trace_id=<id>"
```
