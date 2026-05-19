// RiskRules page — ALFQ
import { useState } from "react";

interface RiskRule {
  id: string;
  name: string;
  type: string;
  threshold: string;
  active: boolean;
}

export default function RiskRules() {
  const [items] = useState<RiskRule[]>([]);
  return (
    <div style={{ padding: "2rem" }}>
      <h1>风控规则</h1>
      <table style={{ width: "100%", marginTop: 16, borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ textAlign: "left", borderBottom: "2px solid #ddd" }}>
            <th style={{ padding: 8 }}>规则ID</th>
            <th style={{ padding: 8 }}>名称</th>
            <th style={{ padding: 8 }}>类型</th>
            <th style={{ padding: 8 }}>阈值</th>
            <th style={{ padding: 8 }}>状态</th>
          </tr>
        </thead>
        <tbody>
          {items.map((r: RiskRule) => (
            <tr key={r.id} style={{ borderBottom: "1px solid #eee" }}>
              <td style={{ padding: 8 }}>{r.id}</td>
              <td style={{ padding: 8 }}>{r.name}</td>
              <td style={{ padding: 8 }}>{r.type}</td>
              <td style={{ padding: 8 }}>{r.threshold}</td>
              <td style={{ padding: 8 }}>{r.active ? "启用" : "禁用"}</td>
            </tr>
          ))}
          {items.length === 0 && <tr><td colSpan={5} style={{ padding: 8, color: "#888" }}>暂无规则（等待 trading-core 风控 API 接入）</td></tr>}
        </tbody>
      </table>
    </div>
  );
}
