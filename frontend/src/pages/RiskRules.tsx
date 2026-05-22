// RiskRules page — displays the 10 active risk rules and their status.
import { useState } from "react";

const RULES: { id: string; name: string; type: string; threshold: string; description: string }[] = [
  { id: "max_lot",      name: "最大手数",    type: "仓位限制", threshold: "100 lots",    description: "单笔订单超过最大手数限制" },
  { id: "max_position", name: "最大持仓",    type: "仓位限制", threshold: "10 lots/品种", description: "单一品种持仓超过上限" },
  { id: "daily_loss",   name: "日内亏损",    type: "亏损限制", threshold: "$5,000",      description: "当日浮动亏损超限" },
  { id: "drawdown",     name: "最大回撤",    type: "亏损限制", threshold: "15%",         description: "从峰值回撤超过阈值" },
  { id: "whitelist",    name: "品种白名单",  type: "品种限制", threshold: "EURUSD, GBPUSD, USDJPY", description: "不在白名单的品种不允许交易" },
  { id: "session",      name: "交易时段",    type: "时间限制", threshold: "UTC 周一-周五", description: "非交易时段禁止下单" },
  { id: "margin",       name: "保证金水平",  type: "资金限制", threshold: ">150%",       description: "保证金水平低于阈值" },
  { id: "slippage",     name: "滑点保护",    type: "执行限制", threshold: "5 pips",      description: "滑点超过限制" },
  { id: "heartbeat",    name: "心跳检测",    type: "连接检测", threshold: "5 min",       description: "连接超时未收到心跳" },
  { id: "reject_rate",  name: "拒绝率",      type: "质量限制", threshold: "30%",         description: "订单被拒率过高" },
];

export default function RiskRules() {
  const [filter, setFilter] = useState("");

  const filtered = RULES.filter(r =>
    !filter || r.name.includes(filter) || r.id.includes(filter) || r.type.includes(filter)
  );

  return (
    <div className="page">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1.5rem" }}>
        <div>
          <h1 className="page-title" style={{ marginBottom: "0.25rem" }}>风控规则</h1>
          <p style={{ color: "var(--color-text-muted)", margin: 0, fontSize: 14 }}>
            共 {RULES.length} 条规则，下单前逐条强制执行
          </p>
        </div>
        <input className="input" placeholder="搜索规则..." value={filter}
          onChange={e => setFilter(e.target.value)}
          style={{ height: 40, fontSize: 14, width: 200 }} />
      </div>

      <div className="glass-card" style={{ overflow: "auto" }}>
        <table className="table">
          <thead>
            <tr>
              <th>规则 ID</th>
              <th>名称</th>
              <th>类型</th>
              <th>阈值</th>
              <th>说明</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map(r => (
              <tr key={r.id}>
                <td style={{ fontFamily: "monospace", fontSize: 12 }}>{r.id}</td>
                <td style={{ fontWeight: 600 }}>{r.name}</td>
                <td><span style={{ padding: "2px 8px", borderRadius: 4, fontSize: 12, background: "var(--color-bg-tertiary)" }}>{r.type}</span></td>
                <td style={{ fontFamily: "monospace", fontSize: 13 }}>{r.threshold}</td>
                <td style={{ fontSize: 13, color: "var(--color-text-secondary)" }}>{r.description}</td>
                <td><span style={{ color: "var(--color-success)", fontWeight: 600 }}>● 启用</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
