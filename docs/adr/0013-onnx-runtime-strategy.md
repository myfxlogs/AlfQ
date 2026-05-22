# ADR 0013 · ONNX Runtime Strategy

> 日期：2026-05-22 | 状态：提议（Proposed）

## 背景

量化策略的信号生成有两种路径：
1. **DSL 因子表达式**：手写因子 + 信号规则（如 `ema20/ema60 > 1`），可解释性强但表达能力有限
2. **ML 模型推理**：LightGBM/XGBoost 训练 → ONNX 导出 → 推理，适合复杂非线性模式

ADR 0013 定义 ONNX 作为 ALFQ 唯一的 ML 模型交换格式和推理运行时。

## 决策

**ALFQ 使用 ONNX Runtime 进行模型推理，不直接调用 sklearn/lightgbm 原生 API。**

理由：
- ONNX 是跨框架标准格式，LightGBM、XGBoost、PyTorch 均可导出
- `onnxruntime-go` 提供 Go 原生绑定，与 quant-engine 语言一致
- 模型与策略 spec 解耦：spec 引用 `model_uri`，模型文件存储在 MinIO/S3
- 推理延迟 < 1ms（单样本），适合实时 bar 级信号生成

## 架构

```
Python 研究层                        Go 生产层
┌──────────────────┐              ┌──────────────────────┐
│ ModelTrainer      │              │ ONNXModelRunner      │
│  train_lightgbm() │── ONNX ──→  │  LoadModel(uri)      │
│  export_onnx()    │   .onnx     │  Predict(features)    │
│  upload_model()   │   MinIO     │  → float64 signal     │
└──────────────────┘              └──────────────────────┘
```

## 模型生命周期

1. **训练**（研究层）
   - `ModelTrainer` 从 ClickHouse 拉取历史因子值
   - 使用 LightGBM 训练二分类模型（long/short/flat）
   - 输出：`strategy_{id}_v{revision}.onnx`

2. **导出**
   - `export_onnx()` 调用 `hummingbird-ml` 或 `onnxmltools` 转换
   - 验证：roundtrip 测试（Python 原生 vs ONNX 推理，diff < 1e-6）

3. **存储**
   - `upload_model()` 上传到 MinIO `ai-artifacts` bucket
   - 路径：`models/{strategy_id}/{revision}.onnx`

4. **部署**
   - quant-engine 的 `ONNXModelRunner` 从 MinIO 下载模型
   - 每个 bar 调用 `Predict(features)`，特征向量从 `factorsvc.Engine` 获取

5. **回滚**
   - 保留最近 3 个 revision 的 .onnx 文件
   - Paper→Live 升级时锁死 revision

## 因子→特征映射

`ONNXModelRunner` 的特征向量由策略 spec 的 `input_features` 字段定义：

```json
{
  "input_features": ["ema20", "ema60", "rsi14", "bb_upper", "bb_lower"],
  "model_uri": "s3://ai-artifacts/models/strat_xxx/v3.onnx"
}
```

特征值从 `factorsvc.Engine.LatestFactors()` 按名称提取，缺失值填 0。

## 模型版本管理

- 模型文件绑 `strategy_revisions.revision_no`（RS02）
- 每次 retrain 生成新 revision，旧模型保留不删
- Paper 阶段可热替换模型（灰度切流），Live 阶段锁定

## 回退策略

如果 ONNX 模型不可用（MinIO 挂掉、模型损坏、首次部署），自动回退到 DSL 信号规则。`ONNXModelRunner.useDSL` 标记控制。

## 验收标准

- `research/tests/test_onnx_roundtrip.py` 通过：LightGBM → ONNX → Go 推理，diff < 1e-6
- quant-engine 加载真实 .onnx 文件，单次推理 < 1ms
- ONNX 不可用时自动回退 DSL，不阻塞信号链

## 参考资料

- `backend/go/internal/quantengine/onnx_runtime.go`（Go 端实现）
- `research/alfq_research/model/trainer.py`（训练）
- `research/alfq_research/model/exporter.py`（导出）
- `research/tests/test_onnx_roundtrip.py`（一致性测试）
- `docs/06-Python策略沙箱设计.md` §4（训练流程）
