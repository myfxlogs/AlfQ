# Research — ALFQ 研究层

Python 研究环境。数据探索、因子开发、回测、参数优化、模型训练。

## 目录

```
research/
├── alfq_research/  # 研究主包 (uv)
│   ├── data/       # 数据加载
│   ├── factor/     # 因子库 + DSL 解析器
│   ├── backtest/   # 回测引擎
│   ├── optimize/   # 参数搜索
│   └── report/     # 绩效报告
├── notebooks/      # 研究 notebook
├── tests/          # 测试
├── pyproject.toml
└── README.md
```

## 技术栈

- Python 3.12, uv 管理, ruff + mypy strict
- 日志: loguru

## 参考项目

参考：microsoft/qlib (因子 DSL), polakowo/vectorbt (向量化回测), ranaroussi/quantstats (绩效报告)
