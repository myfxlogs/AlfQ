// Accounts page — ALFQ
import { useEffect, useState } from "react";
import { accountClient } from "../api/client";
import type { Account } from "../gen/alfq/v1/broker_pb";

export default function Accounts() {
  const [items, setItems] = useState<Account[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    accountClient.listAccounts({ tenantId: "" })
      .then(res => { setItems(res.accounts ?? []); setLoading(false); })
      .catch((e: unknown) => { setError(e instanceof Error ? e.message : "加载失败"); setLoading(false); });
  }, []);

  return (
    <div style={{ padding: "2rem" }}>
      <h1>账户管理</h1>
      {error && <div style={{ color: "red", marginTop: 12 }}>{error}</div>}
      {loading ? <p>加载中...</p> : (
        <table style={{ width: "100%", marginTop: 16, borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ textAlign: "left", borderBottom: "2px solid #ddd" }}>
              <th style={{ padding: 8 }}>ID</th>
              <th style={{ padding: 8 }}>名称</th>
              <th style={{ padding: 8 }}>经纪商</th>
              <th style={{ padding: 8 }}>登录</th>
              <th style={{ padding: 8 }}>状态</th>
            </tr>
          </thead>
          <tbody>
            {items.map((a: Account) => (
              <tr key={a.id} style={{ borderBottom: "1px solid #eee" }}>
                <td style={{ padding: 8 }}>{a.id}</td>
                <td style={{ padding: 8 }}>{a.accountType}</td>
                <td style={{ padding: 8 }}>{a.brokerId}</td>
                <td style={{ padding: 8 }}>{a.login}</td>
                <td style={{ padding: 8 }}>{a.server || "—"}</td>
              </tr>
            ))}
            {items.length === 0 && <tr><td colSpan={5} style={{ padding: 8, color: "#888" }}>暂无账户</td></tr>}
          </tbody>
        </table>
      )}
    </div>
  );
}
