// Orders page — lists historical orders for a selected account.
import { useEffect, useState } from "react";
import { accountClient } from "../api/client";
import type { Account } from "../gen/alfq/v1/broker_pb";
import type { HistoricalOrder } from "../gen/alfq/v1/broker_pb";

export default function Orders() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [orders, setOrders] = useState<HistoricalOrder[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  // Load account list for the dropdown
  useEffect(() => {
    accountClient.listAccounts({ tenantId: "" })
      .then(res => {
        const list = res.accounts ?? [];
        setAccounts(list);
        if (list.length > 0 && !selectedAccountId) {
          setSelectedAccountId(list[0].id);
        }
      })
      .catch(e => setError(e instanceof Error ? e.message : "加载账户失败"));
  }, []);

  // Load orders when account selection changes
  useEffect(() => {
    if (!selectedAccountId) return;
    setLoading(true);
    setError("");
    accountClient.listAccountOrders({ accountId: selectedAccountId })
      .then(res => { setOrders(res.orders ?? []); setLoading(false); })
      .catch(e => { setError(e instanceof Error ? e.message : "加载订单失败"); setLoading(false); });
  }, [selectedAccountId]);

  return (
    <div className="page">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1.5rem" }}>
        <div>
          <h1 className="page-title" style={{ marginBottom: "0.25rem" }}>订单管理</h1>
          <p style={{ color: "var(--color-text-muted)", margin: 0, fontSize: 14 }}>
            {orders.length} 笔历史订单
          </p>
        </div>
        <select
          className="input"
          value={selectedAccountId}
          onChange={e => setSelectedAccountId(e.target.value)}
          style={{ height: 40, fontSize: 14, minWidth: 200 }}
        >
          {accounts.map(a => (
            <option key={a.id} value={a.id}>
              {a.login} — {a.platform} {a.serverName || a.server}
            </option>
          ))}
        </select>
      </div>

      {error && <div style={{ color: "var(--color-danger)", marginBottom: 12 }}>{error}</div>}

      <div className="glass-card" style={{ overflow: "auto" }}>
        {loading ? (
          <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>加载中...</p>
        ) : orders.length === 0 ? (
          <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>
            {selectedAccountId ? "该账户暂无历史订单" : "请选择一个账户"}
          </p>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Ticket</th>
                <th>品种</th>
                <th>方向</th>
                <th>手数</th>
                <th>开仓价</th>
                <th>平仓价</th>
                <th>盈亏</th>
                <th>库存费</th>
                <th>开仓时间</th>
              </tr>
            </thead>
            <tbody>
              {orders.map(o => (
                <tr key={o.ticket}>
                  <td style={{ fontWeight: 600 }}>{String(o.ticket)}</td>
                  <td>{o.symbol}</td>
                  <td className={o.side === "buy" ? "price-up" : "price-down"}>
                    {o.side === "buy" ? "买入" : "卖出"}
                  </td>
                  <td>{o.lots?.toFixed(2)}</td>
                  <td>{o.openPrice?.toFixed(5)}</td>
                  <td>{o.closePrice?.toFixed(5) || "—"}</td>
                  <td className={(o.profit ?? 0) >= 0 ? "price-up" : "price-down"}>
                    {o.profit != null ? `$${o.profit.toFixed(2)}` : "—"}
                  </td>
                  <td>{o.swap != null ? `$${o.swap.toFixed(2)}` : "—"}</td>
                  <td style={{ fontSize: 12, color: "var(--color-text-muted)" }}>
                    {o.openTime ? new Date(o.openTime).toLocaleString() : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}