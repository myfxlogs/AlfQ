// Positions page — ALFQ
import { useState } from "react";
import type { Position } from "../gen/alfq/v1/order_pb";

export default function Positions() {
  const [items] = useState<Position[]>([]);
  return (
    <div style={{ padding: "2rem" }}>
      <h1>持仓管理</h1>
      <table style={{ width: "100%", marginTop: 16, borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ textAlign: "left", borderBottom: "2px solid #ddd" }}>
            <th style={{ padding: 8 }}>持仓ID</th>
            <th style={{ padding: 8 }}>品种</th>
            <th style={{ padding: 8 }}>数量</th>
            <th style={{ padding: 8 }}>均价</th>
            <th style={{ padding: 8 }}>浮盈</th>
          </tr>
        </thead>
        <tbody>
          {items.map((p: Position) => (
            <tr key={p.positionId} style={{ borderBottom: "1px solid #eee" }}>
              <td style={{ padding: 8 }}>{p.positionId}</td>
              <td style={{ padding: 8 }}>{p.symbol}</td>
              <td style={{ padding: 8 }}>{p.qty}</td>
              <td style={{ padding: 8 }}>{p.avgPrice?.value || "—"}</td>
              <td style={{ padding: 8 }}>{p.unrealizedPnl?.value || "—"}</td>
            </tr>
          ))}
          {items.length === 0 && <tr><td colSpan={5} style={{ padding: 8, color: "#888" }}>暂无持仓（等待 trading-core OMS API 接入）</td></tr>}
        </tbody>
      </table>
    </div>
  );
}
