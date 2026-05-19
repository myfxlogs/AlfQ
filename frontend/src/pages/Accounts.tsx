// Accounts page — 交易账户管理
import { useEffect, useState } from "react";
import { accountClient } from "../api/client";
import type { Account } from "../gen/alfq/v1/broker_pb";
import BindAccount from "./BindAccount";

export default function Accounts() {
  const [items, setItems] = useState<Account[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showBind, setShowBind] = useState(false);

  const load = () => {
    setLoading(true);
    accountClient.listAccounts({ tenantId: "" })
      .then((res) => { setItems(res.accounts ?? []); setLoading(false); })
      .catch((e: unknown) => { setError(e instanceof Error ? e.message : "加载失败"); setLoading(false); });
  };

  useEffect(load, []);

  if (showBind) {
    return <BindAccount onDone={() => { setShowBind(false); load(); }} />;
  }

  const statusLabel = (s: string | undefined) => {
    switch (s) {
      case "connected": return "已连接";
      case "connecting": return "连接中";
      case "disconnected": return "未连接";
      case "error": return "错误";
      default: return s || "—";
    }
  };
  const statusColor = (s: string | undefined) => {
    switch (s) {
      case "connected": return "var(--color-success)";
      case "error": return "var(--color-danger)";
      case "connecting": return "var(--color-primary)";
      default: return "var(--color-text-muted)";
    }
  };

  return (
    <div className="page">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1.5rem" }}>
        <h1 className="page-title" style={{ margin: 0 }}>交易账户</h1>
        <button className="btn-primary" onClick={() => setShowBind(true)}>
          + 绑定账户
        </button>
      </div>
      {error && <div style={{ color: "var(--color-danger)", marginBottom: 12 }}>{error}</div>}
      {loading ? (
        <p style={{ color: "var(--color-text-muted)" }}>加载中...</p>
      ) : (
        <div className="glass-card" style={{ overflow: "auto" }}>
          <table className="table">
            <thead>
              <tr>
                <th>登录号</th>
                <th>服务器</th>
                <th>类型</th>
                <th>状态</th>
                <th>余额</th>
                <th>净值</th>
                <th>杠杆</th>
                <th>货币</th>
              </tr>
            </thead>
            <tbody>
              {items.map((a: Account) => (
                <tr key={a.id}>
                  <td style={{ fontWeight: 600 }}>{a.login}</td>
                  <td style={{ fontSize: 12, color: "var(--color-text-secondary)" }}>{a.server}</td>
                  <td>{a.accountType}</td>
                  <td>
                    <span style={{ color: statusColor(a.status), fontWeight: 600, fontSize: 13 }}>
                      {statusLabel(a.status)}
                    </span>
                    {a.lastError && (
                      <span style={{ fontSize: 11, color: "var(--color-text-muted)", marginLeft: 4 }} title={a.lastError}>
                        ⚠
                      </span>
                    )}
                  </td>
                  <td className={a.balance && a.balance > 0 ? "price-up" : ""}>
                    {a.balance !== undefined ? `$${a.balance.toFixed(2)}` : "—"}
                  </td>
                  <td>{a.equity !== undefined ? `$${a.equity.toFixed(2)}` : "—"}</td>
                  <td>{a.leverage ? `1:${a.leverage}` : "—"}</td>
                  <td>{a.currency || "—"}</td>
                </tr>
              ))}
              {items.length === 0 && (
                <tr>
                  <td colSpan={8} style={{ textAlign: "center", color: "var(--color-text-muted)", padding: "2rem" }}>
                    暂无交易账户，点击「+ 绑定账户」添加
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
