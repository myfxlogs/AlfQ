# ADR 0013 — ONNX Runtime Strategy

- **Status**: Proposed
- **Date**: 2026-05-20
- **Deciders**: (等待人类决策)

## Context

quant-engine 的 strategy inference 当前仅支持 DSL signal rule，`onnx_runtime.go` 是 placeholder。
项目需要决策是否及何时集成真正的 ONNX runtime。

## Options

### 1. `onnxruntime-go` (CGO binding to ONNX Runtime C API)
- 性能最好，可直接在 Go 进程内跑推理
- binary 显著增大 (~50MB)，构建复杂 (CGO + musl 冲突)
- 单机 docker-compose 部署可接受

### 2. assistant-svc 暴露 `EvaluateModel` Connect RPC（Python 加载 ORT）
- 与现有 ML 治理链路（docs/18）统一
- Assistant-svc 已有 Python 研究环境，可直接用 onnxruntime
- Quant-engine 不依赖 ORT binary，通过 RPC 调用

### 3. `gorgonia.org/onnx-go` (pure Go)
- 构建简单，无 CGO 依赖
- 算子覆盖有限，部分 ONNX ops 不支持

### 4. 不做（永远 DSL）
- 策略永远只跑 DSL 信号规则
- 放弃 ONNX 模型线上推理能力

## Decision

**暂保持 DSL fallback**。当以下触发条件全部满足时，选择 **选项 2** 升级到真集成：

1. EP-1 trainer 已能产出 .onnx 文件并写 `ai_artifacts` 表
2. 至少 1 个研究员愿意把 ONNX 上 paper 跑 ≥ 1 周
3. 模型治理（drift / shadow / lifecycle）已就绪

## Consequences

- 研究端 PyTorch/sklearn 训练全部 → ONNX → assistant-svc 加载 → 通过 RPC 调用
- quant-engine 不依赖 ORT binary，保持构建轻量
- 现阶段 DSL fallback 足够：没有已训练 ONNX 模型、策略 Spec 还在双签定型中
- 选项 2 需要在 assistant-svc 新增 `EvaluateModel` RPC（proto + handler），届时需 ADR 补充

## Gate

触发条件 1+2+3 全满足才推进 ADR 进 `Accepted`。
在此之前，DSL fallback 视为生产就绪。

## References

- `docs/18-AI-Agent工作流深化与策略助手.md`
- `docs/06-Python策略沙箱设计.md`
- OPEN-DECISIONS-2026-05-20.md §2
- `backend/go/internal/quantengine/onnx_runtime.go`
