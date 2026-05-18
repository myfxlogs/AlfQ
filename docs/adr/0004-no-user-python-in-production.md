# 0004 - 用户 Python 代码不进生产，仅 DSL + ONNX

| 字段 | 值 |
|---|---|
| 日期 | 2026-05-18 |
| 状态 | accepted |
| 决策者 | architecture, security, risk |
| 影响范围 | 策略执行路径 |
| 关联 ADR | — |
| 关联 docs | docs/06, docs/09 |

## 背景

用户研究阶段用 Python 写策略 / 训练模型。但生产实盘进程不能信任任意用户代码：
- 安全：可能调用网络/IO/破坏 host
- 稳定：Python GIL、内存泄漏、阻塞 event loop
- 一致：研究 vs 实盘行为偏差难追

## 选项

### A. 用户 Python 直接进 strategy-svc（嵌入 CPython）
- 优点：无缝
- 缺点：安全/稳定/一致性都差

### B. 用户 Python 跑在受限沙箱进程，IPC 给 Go
- 优点：一定隔离
- 缺点：仍有 GIL/资源风险；难审计；性能差

### C. 生产仅运行 DSL + ONNX；Python 仅在研究沙箱
- 优点：严格隔离；DSL 可形式化验证；ONNX 可签名/校验；研究→实盘转换走审批
- 缺点：算子受限（必须由平台扩展）；用户不能用任意 sklearn 模型（必须可导出 ONNX）

## 决策

采用 **C**：
- 生产 `strategy-svc` 只执行：**因子 DSL** + **ONNX 模型推理** + **平台审核的 Spec**
- 研究层 Python 跑在 JupyterHub 沙箱容器
- 转换路径：Python 研究 → 导出 ONNX + DSL → 提交 spec → 审批 → 部署

## 后果

### 积极
- 实盘进程零未知代码
- 模型可追溯/签名
- DSL/ONNX 在 Go 与 Py 双端可对账

### 消极
- 用户初期可能不习惯 ONNX 流程
- 不支持 ONNX 之外的模型（如 prophet）→ 需 case-by-case 扩展

### 跟进事项
- [ ] alfq_research SDK 提供 LightGBM/sklearn 一键 ONNX 导出
- [ ] DSL 与 Python 端的 parity 测试（文档 16）
- [ ] 模型签名 + Vault Transit（文档 22）
