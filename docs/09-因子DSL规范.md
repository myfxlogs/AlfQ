# 09 - 因子 DSL 规范（Go / Python 双实现对齐）

> **单一事实源**：DSL 是字符串，Go 和 Python 各实现解析器/执行器，**对相同输入必须输出相同结果**（误差 < 1e-9）。

## 1. 语法

### 1.1 字面量

| 类型 | 例 |
|---|---|
| 数字 | `42`, `3.14`, `-1.5e-3` |
| 布尔 | `true`, `false` |
| 字符串 | `"EURUSD"` |
| 数据字段 | `$open`, `$high`, `$low`, `$close`, `$volume`, `$bid`, `$ask` |

### 1.2 运算符（优先级从低到高）

- 三元：`cond ? a : b`
- 逻辑：`||`、`&&`
- 比较：`==`、`!=`、`<`、`<=`、`>`、`>=`
- 加减：`+`、`-`
- 乘除模：`*`、`/`、`%`
- 一元：`-x`、`!x`
- 调用：`f(args...)`
- 分组：`(...)`

### 1.3 函数调用

`func_name(arg1, arg2, ...)`，参数可以是字面量、字段引用、嵌套函数。

### 1.4 语法定义（EBNF）

```
expr      = ternary ;
ternary   = logic_or [ "?" expr ":" expr ] ;
logic_or  = logic_and { "||" logic_and } ;
logic_and = equality  { "&&" equality } ;
equality  = compare   { ("==" | "!=") compare } ;
compare   = addsub    { ("<" | "<=" | ">" | ">=") addsub } ;
addsub    = muldiv    { ("+" | "-") muldiv } ;
muldiv    = unary     { ("*" | "/" | "%") unary } ;
unary     = [ "-" | "!" ] primary ;
primary   = number | string | bool | field | call | "(" expr ")" ;
field     = "$" ident ;
call      = ident "(" [ expr { "," expr } ] ")" ;
```

## 2. 内置算子（v1 必须支持）

| 函数 | 签名 | 增量 | 含义 |
|---|---|---|---|
| `ref(x, n)` | (series, int) → series | ✓ | n 期前的 x |
| `sma(x, n)` | (series, int) → series | ✓ | 简单移动均 |
| `ema(x, n)` | (series, int) → series | ✓ | 指数移动均（标准定义 α=2/(n+1)） |
| `wma(x, n)` | (series, int) → series | ✓ | 加权移动均 |
| `std(x, n)` | (series, int) → series | ✓ | 滚动标准差（样本） |
| `var(x, n)` | (series, int) → series | ✓ | 滚动方差 |
| `min(x, n)` | (series, int) → series | ✓ | 滚动最小 |
| `max(x, n)` | (series, int) → series | ✓ | 滚动最大 |
| `sum(x, n)` | (series, int) → series | ✓ | 滚动求和 |
| `delta(x, n)` | | ✓ | x - ref(x,n) |
| `pct_change(x, n)` | | ✓ | x/ref(x,n)-1 |
| `rank(x, n)` | | ✓ | 滚动百分位排名 |
| `corr(x, y, n)` | | ✓ | 滚动相关系数 |
| `cov(x, y, n)` | | ✓ | 滚动协方差 |
| `zscore(x, n)` | | ✓ | (x - sma)/std |
| `atr(n)` | (int) → series | ✓ | TR 平均 |
| `rsi(n)` | | ✓ | 相对强弱 |
| `macd(fast, slow, signal)` | | ✓ | 返回 macd 主线 |
| `bb_upper(x,n,k)` / `bb_lower` | | ✓ | 布林带 |
| `cross_up(x, y)` | | ✓ | x 上穿 y |
| `cross_down(x, y)` | | ✓ | x 下穿 y |
| `abs(x)` / `sign(x)` / `log(x)` / `exp(x)` / `sqrt(x)` / `pow(x,n)` | | ✓ | 标量 |
| `if_(cond, a, b)` | | ✓ | 等价 `cond ? a : b`，便于嵌套 |

### 2.1 算子语义说明

- 所有"窗口算子"在 warmup 期（不足 n 个值）输出 `NaN`
- 比较运算返回 `1.0` / `0.0`（避免布尔/浮点混淆）
- 除零返回 `NaN`，不抛错
- 日期/会话相关算子 v2 再加（`time_of_day()` 等）

## 3. 命名空间与变量

- `$<field>` 引用基础字段（来自 bar）
- 用户定义因子也是字段，但不带 `$`，可被其它因子引用（同 tenant）：
  - 例：`"mom_20_60 * atr(14)"`，其中 `mom_20_60` 是另一已定义因子
  - 引用图必须无环（提交校验时检测）

## 4. 编译/执行流程

```
expr 字符串
  → lex → tokens
  → parse → AST
  → validate（字段是否存在、引用因子是否存在、参数类型）
  → compile → 算子树（每个节点持有状态）
  → eval(bar)（流式）→ 输出 series 值
```

## 5. Go 实现

### 5.1 包结构

```
backend/go/internal/factor/dsl/
├── lex.go
├── parser.go
├── ast.go
├── validate.go
├── compile.go
└── ops/
    ├── ema.go
    ├── sma.go
    ├── std.go
    ├── ref.go
    ├── corr.go
    ├── atr.go
    └── ...
```

### 5.2 关键接口

```go
type Op interface {
    Eval(bar *Bar) float64    // 输入当前 bar，输出当前值
    Reset()
    Warmup() int              // 需要多少 bar 才能输出非 NaN
}

type Compiler interface {
    Compile(expr string, refs map[string]Op) (Op, error)
}
```

### 5.3 性能

- 算子内部状态用环形缓冲 / Welford 在线方差
- 避免 reflect，所有节点用 struct
- bench 目标：单 bar 平均算子树（10 个因子）< 10 µs

## 6. Python 实现

### 6.1 包结构

```
research/alfq_research/factor/dsl/
├── lexer.py
├── parser.py
├── ast_.py
├── validate.py
├── compile.py
└── ops/
    ├── ema.py
    ├── sma.py
    └── ...
```

### 6.2 两种执行模式

| 模式 | 用途 |
|---|---|
| **批量（Polars）** | 研究 / 回测：把 AST 翻译成 Polars 表达式，向量化执行 |
| **流式** | 事件驱动回测，与 Go 端逐 bar 一致 |

研究中默认用批量模式（快），上线前用流式模式对一致性。

### 6.3 一致性测试

`research/tests/test_dsl_parity.py`：

- 加载共享黄金数据集（CSV）
- 对 N 个表达式分别在 Python 流式 & Polars & Go（通过 gRPC test endpoint）跑
- 三方结果逐点比较，误差阈值 1e-9

CI 必须通过此测试。

## 7. 错误处理

| 错误 | 何时报 |
|---|---|
| `SyntaxError` | 解析失败 |
| `UnknownField` | `$xxx` 不存在 |
| `UnknownFunction` | 函数未注册 |
| `ArityMismatch` | 参数个数错 |
| `ArgumentType` | 参数类型错 |
| `CyclicReference` | 因子互相引用形成环 |
| `WindowInvalid` | n ≤ 0 |

## 8. 安全

- 表达式不可包含 shell/IO/网络调用（语法层就禁止）
- 表达式长度 ≤ 4 KB，AST 节点 ≤ 1000
- 解析有超时（100 ms）
- 服务端审批前必须 `ValidateExpression`

## 9. 示例

```
# 动量
"ema($close, 20) / ema($close, 60) - 1"

# 布林带突破
"$close > bb_upper($close, 20, 2.0) ? 1 : ($close < bb_lower($close, 20, 2.0) ? -1 : 0)"

# 引用其他因子
"zscore(mom_20_60, 100) * sign(macd(12,26,9))"

# RSI 反转
"rsi(14) < 30 ? 1 : (rsi(14) > 70 ? -1 : 0)"
```

## 10. 验收

- [ ] EBNF 语法完整实现
- [ ] 22 个算子全部实现且通过单测
- [ ] Go / Python 流式 / Polars 三方一致性测试通过
- [ ] 性能基准达标
- [ ] 安全限制生效（长度/超时/拒绝危险 token）
