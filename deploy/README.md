# Deploy — 部署配置

## 目录

```
deploy/
├── docker-compose.yml      # 开发环境
├── prometheus/             # Prometheus 配置与告警规则
├── grafana/                # Grafana 仪表板
└── README.md
```

## 环境

- dev: docker-compose 单机
- prod: Kubernetes + Helm + ArgoCD
