# 0002 - 采用三域 monorepo（backend / research / frontend）

| 字段 | 值 |
|---|---|
| 日期 | 2026-05-18 |
| 状态 | accepted |
| 决策者 | architecture |
| 影响范围 | 全局 |
| 关联 ADR | — |
| 关联 docs | docs/12, docs/README.md |

## 背景

需要决定代码仓库结构：单体 vs 多仓 vs Monorepo 分域。

约束：
- AI Agent 易索引、易导航
- 三种语言（Go / Python / TS）
- 前后端独立交付，但部分协议（proto）共享
- 未来可能拆为独立仓库

## 选项

### A. 三个独立仓库（multi-repo）
- 优点：边界清晰，权限独立
- 缺点：proto 共享需 submodule 或包发布；跨域 PR 难追溯

### B. 单仓 + 平级 `go/` `py/` `web/`
- 优点：简单
- 缺点：语义不清，AI Agent 不易理解每个目录的"业务身份"

### C. 三域 monorepo：`backend/` `research/` `frontend/`
- 优点：语义清晰，proto 归 backend，每域独立 README/CI/版本号；未来拆仓零成本
- 缺点：仓库稍大；CI 需路径过滤

## 决策

采用 **C. 三域 monorepo**：

```
/opt/alfq/
├── backend/    # Go 服务 + proto + 迁移
├── research/   # Python 研究层
├── frontend/   # Web SPA
├── deploy/ configs/ scripts/ references/ docs/
```

## 后果

### 积极
- AI Agent 易识别三大业务域
- 跨域改动一次 PR 完成
- 共享设施（deploy/configs/docs）零冲突
- 拆仓只需 `git filter-repo`

### 消极
- CI workflow 需用 `paths:` 过滤避免重复触发
- 仓库整体克隆稍大

### 跟进事项
- [ ] 三域 README.md 模板
- [ ] CI workflow 分别为三域定义
- [ ] CODEOWNERS 按域配置
