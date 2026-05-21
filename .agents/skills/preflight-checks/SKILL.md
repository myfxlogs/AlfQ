---
name: preflight-checks
description: |
  Code-before-you-code: scan for duplication signals, extract shared infrastructure,
  and split by responsibility before writing implementations. Use when about to
  write multiple files that share patterns (push handlers, collectors, RPC handlers,
  batch scanners). Triggers on phrases like "落地实现", "新建多个 handler", "批量推送",
  "分页扫描用户", or after reading an implementation plan with parallel file structure.
---

# Preflight Checks

写多文件代码前的三步检查，避免写完再重构。

## Step 1: Skeleton First

同时写出所有同类文件的函数签名（只签名，不实现）。这一步强制你看到全景：

```
push_calendar.go   → func pushCalendar(env)  { /* TODO */ }
push_digest.go     → func pushDailyDigest(env) { /* TODO */ }
push_macro.go      → func pushMacro(env)     { /* TODO */ }
push_cot.go        → func pushCOTRelease(env) { /* TODO */ }
```

## Step 2: Scan for Duplication Signals

扫描骨架，找出所有重复模式。判断标准：

| 信号 | 阈值 |
|---|---|
| 同一结构体/类型在 ≥ 2 个文件中定义 | 立即提取到共享文件 |
| 同一 for/if 模式出现 ≥ 2 次且 > 3 行 | 立即提取为函数 |
| 同一 SQL 查询/API 调用在 ≥ 2 处 | 立即提取 |

**常见重复信号清单：**

- 分页扫描循环（`for { query(offset); if len < N break }`）
- 去重 → 发送 → 记录的三步模板
- 时间范围计算（"今天 00:00 到明天 00:00 UTC"）
- 货币/影响级别/品种过滤（`if !contains(currencies, c) return`）
- `userID → query prefs → check enabled → call fn` 的用户遍历模式

## Step 3: Split by Responsibility

对每个识别出的重复信号，判断它"属于哪个文件的职责"：

| 逻辑 | 属于 | 不属于 |
|---|---|---|
| "如何分页扫描用户" | 基础设施层 | `push_calendar.go`（它的职责是日历事件检测） |
| "如何判断经济日历事件窗口" | 业务层 | 基础设施层 |
| "如何调用 notify.Service.Send 并落库" | 通知中枢 | 每个检测器各自封装 |

**命名约定：** 基础设施文件命名 `*_util.go` 或 `_shared.go`（同包）或独立包（跨包复用）。

## Step 4: Write Implementations

前三步完后再写实现。每个业务文件只调用已提取的公共层，不包含基础设施逻辑。

## Example: Before vs After

See `references/example-go-push.md` for a real before/after comparison from the AntClaw push notification module.

## Integration with AGENTS.md

This skill operationalizes the "写代码前，先扫重复模式" rule in AGENTS.md §11. The AGENTS.md rule is the policy; this skill is the procedure.
