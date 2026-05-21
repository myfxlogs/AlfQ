// Dashboard page — account overview with stats + recent accounts
import { useEffect, useState } from "react";
import { brokerClient, accountClient, strategyClient } from "../api/client";
import type { Account } from "../gen/alfq/v1/broker_pb";

export default function Dashboard() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [, setBrokers] = useState(0);
  const [, setStrategies] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // 总览卡随 accounts 实时重算（避免 SSE 后总览与列表不同步）
  const connectedCount = accounts.filter((a) => a.status === "connected").length;
  const totalEquity = accounts.reduce((s, a) => s + (a.equity ?? 0), 0);
  const totalBalance = accounts.reduce((s, a) => s + (a.balance ?? 0), 0);
  const totalProfit = accounts.reduce((s, a) => s + (a.profit ?? 0), 0);

  useEffect(() => {
    async function load() {
      try {
        const [accRes, brkRes, stratRes] = await Promise.all([
          accountClient.listAccounts({ tenantId: "" }),
          brokerClient.listBrokers({ tenantId: "" }),
          strategyClient.listStrategies({ tenantId: "" }),
        ]);
        setAccounts(accRes.accounts ?? []);
        setBrokers(brkRes.brokers?.length ?? 0);
        setStrategies(stratRes.strategies?.length ?? 0);
      } catch (e: unknown) {
        setError(e instanceof Error ? e.message : "加载失败");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  // SSE — real-time account status updates (与 Accounts 页一致)
  useEffect(() => {
    const es = new EventSource("/sse/accounts");
    es.onmessage = (e) => {
      try {
        const u = JSON.parse(e.data) as {
          accountId: string; status?: string;
          balance: number; equity: number; margin: number;
          freeMargin: number; marginLevel: number; profit: number;
          currency: string; leverage: number;
        };
        setAccounts((prev) =>
          prev.map((a) =>
            a.id === u.accountId
              ? {
                  ...a,
                  status: u.status ?? a.status,
                  balance: u.balance, equity: u.equity, margin: u.margin,
                  freeMargin: u.freeMargin, marginLevel: u.marginLevel,
                  profit: u.profit,
                  currency: u.currency || a.currency,
                  leverage: u.leverage || a.leverage,
                }
              : a
          )
        );
      } catch { /* ignore malformed */ }
    };
    es.onerror = () => { /* EventSource auto-reconnects */ };
    return () => es.close();
  }, []);

  return (
    <div className="page">
      {/* Header */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "1.5rem" }}>
        <div>
          <h1 className="page-title" style={{ marginBottom: "0.25rem" }}>仪表盘</h1>
          <p style={{ color: "var(--color-text-muted)", margin: 0, fontSize: 14 }}>
            {loading ? "加载中..." : `${accounts.length} 个账户 · ${connectedCount} 在线`}
          </p>
        </div>
        <button className="btn-primary" onClick={() => window.location.hash = "#/bind"}>
          + 添加账号
        </button>
      </div>

      {error && <div style={{ color: "var(--color-danger)", marginBottom: 12 }}>{error}</div>}

      {/* Stats Grid */}
      <div className="stats-grid" style={{ marginBottom: "1.5rem" }}>
        <StatCard icon="💰" label="总净值" value={`$${totalEquity.toFixed(2)}`} color="var(--color-text)" />
        <StatCard icon="🏦" label="总余额" value={`$${totalBalance.toFixed(2)}`} color="var(--color-text)" />
        <StatCard icon="📈" label="浮动盈亏" value={`$${totalProfit.toFixed(2)}`}
          color={totalProfit >= 0 ? "var(--color-success)" : "var(--color-danger)"} />
        <StatCard icon="🔌" label="在线账户" value={loading ? "..." : String(connectedCount)} color="var(--color-success)" />
      </div>

      {/* Account list */}
      <div className="glass-card" style={{ overflow: "auto" }}>
        <div style={{ padding: "1rem 1.5rem", borderBottom: "1px solid var(--color-border)", fontWeight: 600 }}>
          交易账户
        </div>
        {loading ? (
          <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>加载中...</p>
        ) : accounts.length === 0 ? (
          <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>
            暂无账户，点击右上角「+ 绑定账户」开始
          </p>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>登录号</th>
                <th>平台</th>
                <th>服务器</th>
                <th>状态</th>
                <th>余额</th>
                <th>净值</th>
                <th>浮动盈亏</th>
                <th>杠杆</th>
              </tr>
            </thead>
            <tbody>
              {accounts.map((a) => (
                <tr
                  key={a.id}
                  onClick={() => window.location.hash = `#/account/${a.id}`}
                  style={{ cursor: "pointer", transition: "background 0.2s" }}
                  onMouseEnter={(e) => e.currentTarget.style.background = "rgba(0,0,0,0.02)"}
                  onMouseLeave={(e) => e.currentTarget.style.background = "transparent"}
                >
                  <td style={{ fontWeight: 600 }}>{a.login || "—"}</td>
                  <td>
                    <span style={{
                      display: "inline-block", padding: "2px 8px", borderRadius: 4, fontSize: 12, fontWeight: 600,
                      background: a.platform === "MT4" ? "rgba(33,150,243,0.1)" : "rgba(212,175,55,0.15)",
                      color: a.platform === "MT4" ? "#1976D2" : "#B8960B",
                    }}>
                      {a.platform || "—"}
                    </span>
                  </td>
                  <td style={{ fontSize: 12, color: "var(--color-text-secondary)", maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis" }}>
                    {a.serverName || a.server || "—"}
                  </td>
                  <td>
                    <span style={{
                      display: "inline-block", padding: "2px 8px", borderRadius: 12, fontSize: 12, fontWeight: 600,
                      background: a.status === "connected" ? "rgba(0,166,81,0.1)" :
                        a.status === "error" ? "rgba(229,57,53,0.1)" : "rgba(0,0,0,0.05)",
                      color: a.status === "connected" ? "var(--color-success)" :
                        a.status === "error" ? "var(--color-danger)" : "var(--color-text-muted)",
                    }}>
                      {a.status === "connected" ? "在线" : a.status === "error" ? "错误" : "离线"}
                    </span>
                  </td>
                  <td className={a.balance && a.balance > 0 ? "price-up" : ""}>
                    {a.balance != null ? `$${a.balance.toFixed(2)}` : "—"}
                  </td>
                  <td>{a.equity != null ? `$${a.equity.toFixed(2)}` : "—"}</td>
                  <td className={(a.profit ?? 0) >= 0 ? "price-up" : "price-down"}>
                    {a.profit != null ? `$${a.profit.toFixed(2)}` : "—"}
                  </td>
                  <td>{a.leverage ? `1:${a.leverage}` : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function StatCard({ icon, label, value, color }: { icon: string; label: string; value: string; color: string }) {
  return (
    <div className="stat-card">
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "0.5rem" }}>
        <div style={{
          width: 40, height: 40, borderRadius: 10, display: "flex", alignItems: "center", justifyContent: "center",
          background: "var(--color-bg-secondary)", fontSize: 18,
        }}>{icon}</div>
      </div>
      <div style={{ color: "var(--color-text-muted)", fontSize: 13, marginBottom: "0.25rem" }}>{label}</div>
      <div style={{ fontSize: 22, fontWeight: 700, color }}>{value}</div>
    </div>
  );
}
