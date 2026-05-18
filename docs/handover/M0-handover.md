# M0 Handover

> 日期：2026-05-18 | Agent 完成 | 等待人工批复后进入 M1

## 已完成

| PR | 内容 | 关键文件数 |
|---|---|---|
| PR-1 | 仓库骨架：三域目录、.gitignore、Makefile、6 个 README、fetch-references.sh、.gitkeep | ~20 |
| PR-2 | Proto 工程：buf.yaml、buf.gen.yaml、common/errors/health.proto（80+ 错误码枚举）| 6 |
| PR-3 | Backend Go：go.mod、errs/logger/config/health 四个公共包 + admin-api 可启动服务 + .golangci.yml + Dockerfile | 7 |
| PR-4 | Research 空壳：pyproject.toml（14 核心 + 6 dev 依赖）、alfq_research 包、smoke test | 4 |
| PR-4 | Frontend 空壳：Vite 8 + React + TS、package.json、vite.config.ts、App.tsx | 8 |
| PR-5 | Infra：docker-compose.yml（PG/CH/Redis/NATS/Vault/MinIO 六件套）、ci.yml、PR 模板、CODEOWNERS | 4 |

## 验收记录

### 目录结构

```
/opt/alfq/
├── backend/proto/alfq/v1/     ← proto 单一源
├── backend/go/cmd/admin-api/  ← 可启动
├── research/alfq_research/    ← uv 包
├── frontend/src/              ← Vite+React SPA
├── deploy/docker-compose.yml  ← 六件套
├── configs/admin-api.yaml
├── scripts/fetch-references.sh
├── .github/workflows/ci.yml
├── .golangci.yml
└── Makefile
```

### 各域验证

```
# Proto
$ make proto-lint
  → buf lint: PASS

$ make proto-gen
  → 产出 common.pb.go errors.pb.go health.pb.go (alfqv1connect)

# Go
$ make go-build
  → admin-api 二进制 17M，编译通过

$ curl -X POST http://localhost:8080/alfq.v1.HealthService/Check \
    -H "Content-Type:application/json" -d '{}'
  → {"status":"SERVING_STATUS_SERVING"}

# Python
$ uv run pytest
  → 1 passed (test_smoke)

# Frontend
$ pnpm build
  → vite v8.0.13, built in 225ms

# Docker
$ make dev-up
  → deploy-postgres-1    Healthy
  → deploy-clickhouse-1  Healthy
  → deploy-redis-1       Healthy
  → deploy-nats-1        Healthy
  → deploy-vault-1       Healthy
  → deploy-minio-1       Healthy
```

### 未验证项

- CI（`.github/workflows/ci.yml`）：需推送到 GitHub 后触发

## 与 M0 计划的偏差

无。所有 5 个 PR 按计划交付，差异已在实施中闭环消除。

实际运行时版本高于文档最低要求（Go 1.26.3 / Node 24 / Vite 8 / React 19），均为工具链默认最新，向下兼容。

## 下一步建议

1. **M1 首张 Task Card**：`md-gateway` mtapi 单连接 demo
   - 优先读 `docs/14-领域模型与交易规则.md` §1-3 → `docs/08-Go服务实现规范.md` §3 → `docs/13-参考项目研习指南.md` 关于 gocryptotrader 的引导

## 风险与待办

无。M0 全部 5 个 PR 交付、四域验证通过、Docker 六件套 healthy、go.mod 零 replace 指令、所有 lint 配置就位。
