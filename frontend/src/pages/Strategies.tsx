// Strategies page — ALFQ
import { useEffect, useState } from "react";
import { strategyClient } from "../api/client";
import type { Strategy, ListStrategiesResponse } from "../gen/alfq/v1/strategy_pb";

export default function Strategies() {
  const [items, setItems] = useState<Strategy[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    strategyClient.listStrategies({ tenantId: "" })
      .then((res: ListStrategiesResponse) => { setItems(res.strategies ?? []); setLoading(false); })
      .catch((e: unknown) => { setError(e instanceof Error ? e.message : "加载失败"); setLoading(false); });
  }, []);

  return (
    <div style={{ padding: "2rem" }}>
      <h1>策略管理</h1>
      {error && <div style={{ color: "red", marginTop: 12 }}>{error}</div>}
      {loading ? <p>加载中...</p> : (
        <table style={{ width: "100%", marginTop: 16, borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ textAlign: "left", borderBottom: "2px solid #ddd" }}>
              <th style={{ padding: 8 }}>ID</th>
              <th style={{ padding: 8 }}>名称</th>
              <th style={{ padding: 8 }}>类型</th>
              <th style={{ padding: 8 }}>状态</th>
            </tr>
          </thead>
          <tbody>
            {items.map((s: Strategy) => (
              <tr key={s.id} style={{ borderBottom: "1px solid #eee" }}>
                <td style={{ padding: 8 }}>{s.id}</td>
                <td style={{ padding: 8 }}>{s.name}</td>
                <td style={{ padding: 8 }}>{s.description || "—"}</td>
                <td style={{ padding: 8 }}>{s.status || "—"}</td>
              </tr>
            ))}
            {items.length === 0 && <tr><td colSpan={4} style={{ padding: 8, color: "#888" }}>暂无策略</td></tr>}
          </tbody>
        </table>
      )}
    </div>
  );
}
