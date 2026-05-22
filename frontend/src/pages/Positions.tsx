// Positions page — live positions for a selected account.
import { useEffect, useState } from "react";
import { accountClient } from "../api/client";
import type { Account, AccountPosition } from "../gen/alfq/v1/broker_pb";

const fmtPrice = (v: number | undefined | null, d = 5) => {
  if (v == null || v === 0) return "—";
  return v.toFixed(d).replace(/0+$/, "").replace(/\.$/, "");
};

export default function Positions() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [positions, setPositions] = useState<AccountPosition[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    accountClient.listAccounts({ tenantId: "" })
      .then(res => {
        const list = res.accounts ?? [];
        setAccounts(list);
        if (list.length > 0 && !selectedAccountId) setSelectedAccountId(list[0].id);
      })
      .catch(e => setError(e instanceof Error ? e.message : "加载账户失败"));
  }, []);

  useEffect(() => {
    if (!selectedAccountId) return;
    setLoading(true);
    setError("");
    accountClient.listAccountPositions({ accountId: selectedAccountId })
      .then(res => { setPositions(res.positions ?? []); setLoading(false); })
      .catch(e => { setError(e instanceof Error ? e.message : "加载持仓失败"); setLoading(false); });
  }, [selectedAccountId]);

  const totalProfit = positions.reduce((s, p) => s + (p.profit ?? 0), 0);

  return (
    <div className="page">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1.5rem" }}>
        <div>
          <h1 className="page-title" style={{ marginBottom: "0.25rem" }}>持仓管理</h1>
          <p style={{ color: "var(--color-text-muted)", margin: 0, fontSize: 14 }}>
            {positions.length} 个持仓 · 浮动盈亏 <span className={totalProfit >= 0 ? "price-up" : "price-down"}>
              ${totalProfit.toFixed(2)}
            </span>
          </p>
        </div>
        <select className="input" value={selectedAccountId} onChange={e => setSelectedAccountId(e.target.value)}
          style={{ height: 40, fontSize: 14, minWidth: 200 }}>
          {accounts.map(a => (
            <option key={a.id} value={a.id}>{a.login} — {a.platform} {a.serverName || a.server}</option>
          ))}
        </select>
      </div>
      {error && <div style={{ color: "var(--color-danger)", marginBottom: 12 }}>{error}</div>}
      <div className="glass-card" style={{ overflow: "auto" }}>
        {loading ? <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>加载中...</p>
        : positions.length === 0 ? <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>暂无持仓</p>
        : (
          <table className="table">
            <thead><tr><th>Ticket</th><th>品种</th><th>方向</th><th>手数</th><th>开仓价</th><th>当前价</th><th>开仓时间</th><th>盈亏</th><th>库存费</th><th>手续费</th></tr></thead>
            <tbody>
              {positions.map(p => (
                <tr key={p.ticket}>
                  <td style={{ fontWeight: 600 }}>{String(p.ticket)}</td>
                  <td>{p.symbol}</td>
                  <td className={p.side === "buy" ? "price-up" : "price-down"}>{p.side === "buy" ? "买入" : "卖出"}</td>
                  <td>{p.lots?.toFixed(2)}</td>
                  <td>{fmtPrice(p.openPrice)}</td>
                  <td>{fmtPrice(p.currentPrice)}</td>
                  <td>{p.openTimeMs > 0 ? new Date(Number(p.openTimeMs)).toLocaleString("zh-CN") : "—"}</td>
                  <td className={(p.profit ?? 0) >= 0 ? "price-up" : "price-down"}>
                    {p.profit != null ? `$${p.profit.toFixed(2)}` : "—"}
                  </td>
                  <td>{p.swap != null ? `$${p.swap.toFixed(2)}` : "—"}</td>
                  <td>{p.commission != null ? `$${p.commission.toFixed(2)}` : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}