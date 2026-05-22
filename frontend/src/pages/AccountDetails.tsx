// Account Details page — reference anttrader layout
import { useEffect, useState, useRef } from "react";
import { accountClient } from "../api/client";
import type { Account } from "../gen/alfq/v1/broker_pb";

const fmtPrice = (v: number | undefined | null, d = 5) => {
  if (v == null || v === 0) return "—";
  return v.toFixed(d).replace(/0+$/, "").replace(/\.$/, "");
};

export default function AccountDetails() {
  const [accountId] = useState(() => {
    const match = window.location.hash.match(/#\/account\/([^/]+)/);
    return match?.[1] || "";
  });
  const [account, setAccount] = useState<Account | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeTab, setActiveTab] = useState("positions"); // positions | history | settings
  const [showDropdown, setShowDropdown] = useState(false);
  const [showDisableConfirm, setShowDisableConfirm] = useState(false);
  // null = 未加载完成（显示"加载中"）; [] = 已加载且无持仓; [...] = 真实持仓数据
  // REST 初始拉取的可能是陈旧缓存（openTimeMs=0/currentPrice=0），需等 SSE 流帧推真值
  const [positions, setPositions] = useState<any[] | null>(null);
  // Incremented every time the server pushes an `orderEvent` via SSE so child
  // tabs (e.g. HistoryTab) can re-fetch on demand.
  const [orderEventTick, setOrderEventTick] = useState(0);
  const [orderDeltas, setOrderDeltas] = useState<any[] | null>(null);
  const [syncStatus, setSyncStatus] = useState("");
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!accountId) {
      setError("缺少账号 ID");
      setLoading(false);
      return;
    }
    async function load() {
      try {
        const res = await accountClient.getAccount({ id: accountId });
        setAccount(res);
      } catch (e: unknown) {
        const err = e as { message?: string };
        setError(err.message || "加载失败");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [accountId]);

  // Fetch latest positions immediately on page open. The server-side cache may
  // be stale (openTimeMs=0 / currentPrice=0) right after process restart — in
  // that case we keep positions=null ("加载中") and wait for the SSE stream
  // to push the authoritative snapshot, avoiding flashing wrong values.
  useEffect(() => {
    if (!accountId) return;
    (async () => {
      try {
        const res = await accountClient.listAccountPositions({ accountId });
        const rows = res.positions || [];
        const complete = rows.length === 0 || rows.every(
          (p: any) => (p.openTimeMs ?? 0) > 0 && (p.currentPrice ?? 0) > 0,
        );
        if (complete) {
          setPositions(rows);
        }
        // else: leave positions=null; SSE will deliver real data shortly.
      } catch (e) {
        console.error("加载持仓失败", e);
      }
    })();
  }, [accountId]);

  // SSE — real-time account updates + positions
  useEffect(() => {
    const es = new EventSource("/sse/accounts");
    es.onmessage = (e) => {
      try {
        const u = JSON.parse(e.data) as {
          accountId: string; status?: string;
          balance: number; equity: number; margin: number;
          freeMargin: number; marginLevel: number; profit: number;
          currency: string; leverage: number;
          positions?: Array<{
            ticket: number; symbol: string; type: string;
            lots: number; openPrice: number; profit: number;
            swap: number; commission: number;
            openTimeMs: number; currentPrice: number;
          }>;
          orderEvent?: boolean;
        };
        if (u.accountId === accountId) {
          setAccount((prev) =>
            prev
              ? {
                  ...prev,
                  status: u.status ?? prev.status,
                  balance: u.balance,
                  equity: u.equity,
                  margin: u.margin,
                  freeMargin: u.freeMargin,
                  marginLevel: u.marginLevel,
                  profit: u.profit,
                  currency: u.currency || prev.currency,
                  leverage: u.leverage || prev.leverage,
                }
              : prev
          );
          if (u.positions !== undefined) {
            // SSE is the authoritative source — unconditionally replace.
            setPositions(u.positions);
          }
          if (u.orderEvent) {
            setOrderEventTick((t) => t + 1);
          }
          if ((u as any).type === "order_delta") {
            setOrderDeltas((u as any).changes || []);
          }
          if ((u as any).type === "order_sync_done") {
            setSyncStatus("同步完成");
            setOrderEventTick((t) => t + 1); // trigger HistoryTab refresh
            setTimeout(() => setSyncStatus(""), 3000);
          }
        }
      } catch { /* ignore malformed */ }
    };
    es.onerror = () => { /* EventSource auto-reconnects */ };
    return () => es.close();
  }, [accountId]);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setShowDropdown(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  if (loading) return <p style={{ padding: "2rem", textAlign: "center" }}>加载中...</p>;
  if (error) return <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-danger)" }}>{error}</p>;
  if (!account) return <p style={{ padding: "2rem", textAlign: "center" }}>账号不存在</p>;

  const tabs = [
    { id: "positions", label: "持仓" },
    { id: "history", label: "历史订单" },
  ];

  const handleRefresh = async () => {
    try {
      const res = await accountClient.getAccount({ id: accountId });
      setAccount(res);
    } catch (e: unknown) {
      console.error("刷新失败", e);
    }
  };

  const formatCurrency = (value: number | undefined) => {
    if (value === undefined || value === null) return "$0.00";
    return `$${value.toFixed(2)}`;
  };

  const handleToggleStatus = () => {
    if (!account) return;
    if (account.isDisabled) {
      alert("启用账号（待实现）");
    } else {
      setShowDisableConfirm(true);
    }
  };

  const statusConfig = (() => {
    if (account.isDisabled) return { color: "#8A9AA5", bg: "rgba(138, 154, 165, 0.1)", text: "已停用" };
    switch (account.status) {
      case "connected": return { color: "#00A651", bg: "rgba(0, 166, 81, 0.1)", text: "在线" };
      case "connecting": return { color: "#FF9800", bg: "rgba(255, 152, 0, 0.1)", text: "连接中" };
      case "disconnected": return { color: "#E53935", bg: "rgba(229, 57, 53, 0.1)", text: "离线" };
      case "error": return { color: "#E53935", bg: "rgba(229, 57, 53, 0.1)", text: "错误" };
      default: return { color: "#8A9AA5", bg: "rgba(138, 154, 165, 0.1)", text: "未知" };
    }
  })();

  return (
    <div className="page" style={{ background: "#F5F7F9", minHeight: "100vh", padding: "1rem" }}>
      <div style={{ maxWidth: "1280px", margin: "0 auto" }}>
        {/* Header Section */}
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "1.5rem" }}>
          <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
            <button
              onClick={() => window.location.hash = "#/"}
              style={{
                display: "flex", alignItems: "center", justifyContent: "center",
                width: 48, height: 48, borderRadius: 10, border: "1px solid #E5E6EB", background: "#FFF",
                color: "#1D2129", cursor: "pointer", transition: "all 0.2s",
                fontSize: 20, fontWeight: 600,
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = "#F2F3F5"; e.currentTarget.style.borderColor = "#165DFF"; }}
              onMouseLeave={(e) => { e.currentTarget.style.background = "#FFF"; e.currentTarget.style.borderColor = "#E5E6EB"; }}
            >
              ←
            </button>
            <div>
              <div style={{ display: "flex", alignItems: "center", gap: "0.5rem", marginBottom: "0.25rem" }}>
                <h1 style={{ fontSize: 24, fontWeight: 700, color: "#141D22", margin: 0 }}>{account.login || "—"}</h1>
                <Tag label={account.platform || "—"} color={account.platform === "MT4" ? "blue" : "purple"} />
                <Tag label={account.accountType === "real" ? "实盘" : "模拟"} color={account.accountType === "real" ? "red" : "blue"} />
                <Tag label={statusConfig.text} bg={statusConfig.bg} color={statusConfig.color} />
              </div>
              <div style={{ display: "flex", alignItems: "center", gap: "1rem", color: "#8A9AA5", fontSize: 14, flexWrap: "wrap" }}>
                <span>{account.serverName || account.server || "—"}</span>
                <span>•</span>
                <span>货币 {account.currency || "—"}</span>
                <span>•</span>
                <span>杠杆 1:{account.leverage}</span>
                <span>•</span>
                <span>创建于 {account.createdAt ? new Date(Number(account.createdAt.seconds) * 1000).toLocaleDateString("zh-CN") : "—"}</span>
              </div>
            </div>
          </div>
          <div style={{ display: "flex", gap: "0.5rem" }}>
            <button className="btn-secondary" onClick={handleRefresh} style={{ borderRadius: 8 }}>刷新</button>
            <button
              className="btn-secondary"
              disabled={account.status !== "connected" || syncStatus === "同步中..."}
              style={{ borderRadius: 8 }}
              onClick={async () => {
                try {
                  setSyncStatus("同步中...");
                  await accountClient.syncAccountHistory({ accountId });
                } catch (e) {
                  console.error("同步历史失败", e);
                  setSyncStatus("同步失败");
                }
              }}
            >
              {syncStatus || "同步历史"}
            </button>
            <div style={{ position: "relative" }} ref={dropdownRef}>
              <button
                className="btn-secondary"
                onClick={() => setShowDropdown(!showDropdown)}
                style={{ borderRadius: 8, padding: "0 12px" }}
              >
                ⋮
              </button>
              {showDropdown && (
                <div style={{
                  position: "absolute", right: 0, top: "calc(100% + 8px)",
                  background: "#FFF", borderRadius: 8, boxShadow: "0 4px 12px rgba(0,0,0,0.15)",
                  minWidth: 160, zIndex: 100,
                }}>
                  <button
                    onClick={() => { setShowDropdown(false); handleToggleStatus(); }}
                    style={{
                      width: "100%", textAlign: "left", padding: "10px 16px",
                      border: "none", background: "transparent", fontSize: 14,
                      cursor: "pointer", borderRadius: 0,
                    }}
                    onMouseEnter={(e) => e.currentTarget.style.background = "rgba(0,0,0,0.05)"}
                    onMouseLeave={(e) => e.currentTarget.style.background = "transparent"}
                  >
                    {account.isDisabled ? "启用账号" : "停用账号"}
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* First Row Cards (3 columns) */}
        <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: "1rem", marginBottom: "1.5rem" }}>
          <InfoCard label="余额" value={formatCurrency(account.balance)} />
          <InfoCard label="净值" value={formatCurrency(account.equity)} />
          <ProfitCard label="浮动盈亏" value={account.profit} percent={account.profitPercent} />
        </div>

        {/* Second Row Cards (4 columns) */}
        <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: "1rem", marginBottom: "1.5rem" }}>
          <SmallInfoCard label="保证金占用" value={formatCurrency(account.margin)} />
          <SmallInfoCard label="可用保证金" value={formatCurrency(account.freeMargin)} />
          <SmallInfoCard label="保证金水平" value={account.margin > 0 ? `${account.marginLevel?.toFixed(2)}%` : "--"} />
          <SmallInfoCard label="信用额" value={formatCurrency(0)} />
        </div>

        {/* Trade Tabs Section */}
        <div className="glass-card" style={{ padding: 24, marginBottom: "1.5rem" }}>
          <div style={{
            display: "flex", gap: 0, borderBottom: "1px solid var(--color-border)",
            marginBottom: "1.5rem",
          }}>
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                style={{
                  padding: "12px 20px", borderRadius: "8px 8px 0 0", border: "none", background: "transparent",
                  color: activeTab === tab.id ? "#165DFF" : "#4E5969",
                  fontSize: 14, fontWeight: 500, cursor: "pointer",
                  borderBottom: activeTab === tab.id ? "2px solid #165DFF" : "2px solid transparent",
                  transition: "all 0.3s",
                }}
              >
                {tab.label}
              </button>
            ))}
          </div>
          {activeTab === "positions" && <PositionsTab accountId={accountId} positions={positions} />}

          {activeTab === "history" && <HistoryTab accountId={accountId} orderDeltas={orderDeltas} setOrderDeltas={setOrderDeltas} />}
        </div>

        {/* Disable Confirmation Modal */}
        {showDisableConfirm && (
          <div style={{
            position: "fixed", top: 0, left: 0, right: 0, bottom: 0,
            background: "rgba(0,0,0,0.5)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 1000,
          }}>
            <div style={{ background: "#FFF", padding: 24, borderRadius: 8, minWidth: 400 }}>
              <h4 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, color: "#F53F3F" }}>确认停用账号</h4>
              <p style={{ fontSize: 14, color: "#4E5969", marginBottom: 16 }}>
                停用后，该账号将停止接收实时数据推送，且无法进行交易操作。此操作可随时撤销。
              </p>
              <div style={{ display: "flex", gap: 12, justifyContent: "flex-end" }}>
                <button
                  className="btn-secondary"
                  onClick={() => setShowDisableConfirm(false)}
                >
                  取消
                </button>
                <button
                  className="btn-primary"
                  style={{ background: "#F53F3F", borderColor: "#F53F3F" }}
                  onClick={() => { alert("停用账号（待实现）"); setShowDisableConfirm(false); }}
                >
                  确认停用
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// Tag Component
function Tag({ label, color, bg }: { label: string; color?: string; bg?: string }) {
  const colorMap: Record<string, string> = {
    blue: "#2196F3",
    purple: "#9C27B0",
    red: "#E53935",
    green: "#00A651",
  };
  const bgMap: Record<string, string> = {
    blue: "rgba(33, 150, 243, 0.1)",
    purple: "rgba(156, 39, 176, 0.1)",
    red: "rgba(229, 57, 53, 0.1)",
    green: "rgba(0, 166, 81, 0.1)",
  };
  return (
    <span style={{
      padding: "4px 12px", borderRadius: 6, fontSize: 12, fontWeight: 500,
      background: bg || bgMap[color || "blue"] || "rgba(33, 150, 243, 0.1)",
      color: color || colorMap[color || "blue"] || "#2196F3",
    }}>
      {label}
    </span>
  );
}

// Info Card Component (large)
function InfoCard({ label, value, loading }: { label: string; value: string; loading?: boolean }) {
  return (
    <div className="glass-card" style={{ padding: 20, borderRadius: 16 }}>
      <div style={{ fontSize: 14, color: "#8A9AA5", marginBottom: 8 }}>{label}</div>
      <div style={{ fontSize: 24, fontWeight: 700, color: "#141D22" }}>{loading ? "..." : value}</div>
    </div>
  );
}

// Profit Card (with percentage)
function ProfitCard({ label, value, percent }: { label: string; value: number | undefined; percent: number | undefined }) {
  const isPositive = (value || 0) >= 0;
  const color = isPositive ? "#00A651" : "#E53935";
  return (
    <div className="glass-card" style={{ padding: 20, borderRadius: 16 }}>
      <div style={{ fontSize: 14, color: "#8A9AA5", marginBottom: 8 }}>{label}</div>
      <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
        <span style={{ fontSize: 24, fontWeight: 700, color }}>{isPositive ? "+" : ""}{value?.toFixed(2) || "0.00"}</span>
        <span style={{ fontSize: 14, color }}>({(percent ?? 0) >= 0 ? "+" : ""}{(percent ?? 0).toFixed(2)}%)</span>
      </div>
    </div>
  );
}

// Small Info Card Component
function SmallInfoCard({ label, value, loading, valueColor }: { label: string; value: string; loading?: boolean; valueColor?: string }) {
  return (
    <div className="glass-card" style={{ padding: 16, borderRadius: 12 }}>
      <div style={{ fontSize: 13, color: "#8A9AA5", marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 18, fontWeight: 600, color: valueColor || "#141D22" }}>{loading ? "..." : value}</div>
    </div>
  );
}

// Positions Tab — table with fixed header
function PositionsTab({ accountId, positions }: { accountId: string; positions: any[] | null }) {
  if (positions === null) {
    return (
      <div className="glass-card" style={{ padding: 24 }}>
        <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 16 }}>持仓订单</h3>
        <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>持仓加载中…</p>
      </div>
    );
  }
  if (positions.length === 0) {
    return (
      <div className="glass-card" style={{ padding: 24 }}>
        <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 16 }}>持仓订单</h3>
        <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>暂无持仓</p>
      </div>
    );
  }

  return (
    <div className="glass-card" style={{ padding: 24 }}>
      <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 16 }}>持仓订单</h3>
      <table className="table">
        <thead>
          <tr>
            <th>订单号</th>
            <th>产品</th>
            <th>方向</th>
            <th>手数</th>
            <th>开仓价</th>
            <th>当前价</th>
            <th>开仓时间</th>
            <th>利息</th>
            <th>手续费</th>
            <th>浮动盈亏</th>
          </tr>
        </thead>
        <tbody>
          {positions.map((p) => (
            <tr key={p.ticket}>
              <td>{p.ticket}</td>
              <td style={{ fontWeight: 600 }}>{p.symbol}</td>
              <td>
                <span style={{
                  display: "inline-block", padding: "2px 8px", borderRadius: 4, fontSize: 12, fontWeight: 600,
                  background: p.type === "buy" ? "rgba(0,166,81,0.1)" : "rgba(229,57,53,0.1)",
                  color: p.type === "buy" ? "#00A651" : "#E53935",
                }}>
                  {p.type === "buy" ? "买入" : "卖出"}
                </span>
              </td>
              <td>{p.lots?.toFixed(2) ?? "—"}</td>
              <td>{fmtPrice(p.openPrice)}</td>
              <td>{fmtPrice(p.currentPrice)}</td>
              <td>{p.openTimeMs > 0 ? new Date(p.openTimeMs).toLocaleString("zh-CN") : "—"}</td>
              <td>{p.swap?.toFixed(2) ?? "0.00"}</td>
              <td>{p.commission?.toFixed(2) ?? "0.00"}</td>
              <td style={{ color: (p.profit ?? 0) >= 0 ? "#00A651" : "#E53935", fontWeight: 600 }}>
                {(p.profit ?? 0) >= 0 ? "+" : ""}{p.profit?.toFixed(2) ?? "0.00"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// History Tab — paginated table with filters
function HistoryTab({ accountId, orderDeltas, setOrderDeltas }: { accountId: string; orderDeltas: any[] | null; setOrderDeltas: (v: any[] | null) => void }) {
  const [orders, setOrders] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [timeRange, setTimeRange] = useState("today");

  // Initial load + timeRange change → full fetch from local DB
  useEffect(() => {
    async function load() {
      setLoading(true);
      try {
        const fmt = (d: Date) => d.toISOString();
        const now = new Date();
        let fromDate: Date;
        if (timeRange === "today") {
          fromDate = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0);
        } else if (timeRange === "week") {
          fromDate = new Date(now);
          fromDate.setDate(now.getDate() - now.getDay());
          fromDate.setHours(0, 0, 0, 0);
        } else if (timeRange === "month") {
          fromDate = new Date(now.getFullYear(), now.getMonth(), 1, 0, 0, 0);
        } else {
          fromDate = new Date(2000, 0, 1, 0, 0, 0);
        }
        const from = fmt(fromDate);
        const to = fmt(now);
        const res = await accountClient.listAccountOrders({ accountId, from, to });
        setOrders(res.orders);
      } catch (e: unknown) {
        console.error("加载历史订单失败", e);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [accountId, timeRange]);

  // SSE incremental merge — when orderDeltas arrive, merge into local state
  useEffect(() => {
    if (!orderDeltas || orderDeltas.length === 0) return;
    setOrders((prev) => {
      const map = new Map<number, any>(prev.map((o) => [o.ticket, o]));
      for (const d of orderDeltas) {
        if (d.op === "upsert" && d.order) {
          map.set(d.order.ticket, d.order);
        } else if (d.op === "delete" && d.order) {
          map.delete(d.order.ticket);
        } else if (d.op === "sync") {
          // Trigger a full reload on next tick
          setTimeout(() => {
            accountClient.listAccountOrders({ accountId }).then((res) => setOrders(res.orders));
          }, 0);
          return prev;
        }
      }
      const merged = Array.from(map.values());
      merged.sort((a: any, b: any) => {
        const at = a.closeTime ? new Date(a.closeTime).getTime() : 0;
        const bt = b.closeTime ? new Date(b.closeTime).getTime() : 0;
        return bt - at;
      });
      return merged;
    });
    setOrderDeltas(null);
  }, [orderDeltas, accountId, setOrderDeltas]);

  // Client-side pagination
  const total = orders.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const pagedOrders = orders.slice((page - 1) * pageSize, page * pageSize);

  if (loading) return <p style={{ padding: "2rem", textAlign: "center" }}>加载中...</p>;

  return (
    <div className="glass-card" style={{ padding: 24 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <h3 style={{ fontSize: 16, fontWeight: 600 }}>历史订单</h3>
        <div style={{ display: "flex", gap: 8 }}>
          {["today", "week", "month", "all"].map((r) => (
            <button
              key={r}
              onClick={() => { setTimeRange(r); setPage(1); }}
              style={{
                padding: "6px 16px", borderRadius: 4, border: "1px solid var(--color-border)",
                background: timeRange === r ? "#165DFF" : "transparent",
                color: timeRange === r ? "#FFF" : "#4E5969", fontSize: 13,
              }}
            >
              {r === "today" ? "今日" : r === "week" ? "本周" : r === "month" ? "本月" : "全部"}
            </button>
          ))}
        </div>
      </div>
      {orders.length === 0 ? (
        <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>暂无订单</p>
      ) : (
        <>
          <table className="table">
            <thead>
              <tr>
                <th>订单号</th>
                <th>开仓时间</th>
                <th>平仓时间</th>
                <th>方向</th>
                <th>产品</th>
                <th>手数</th>
                <th>开仓价</th>
                <th>平仓价</th>
                <th>盈亏</th>
              </tr>
            </thead>
            <tbody>
              {pagedOrders.map((o) => (
                <tr key={o.ticket}>
                  <td>{o.ticket}</td>
                  <td>{o.openTime ? new Date(o.openTime).toLocaleString("zh-CN") : "—"}</td>
                  <td>{o.closeTime ? new Date(o.closeTime).toLocaleString("zh-CN") : "—"}</td>
                  <td>
                    <span style={{
                      display: "inline-block", padding: "2px 8px", borderRadius: 4, fontSize: 12, fontWeight: 600,
                      background: o.side === "buy" ? "rgba(0,166,81,0.1)" : "rgba(229,57,53,0.1)",
                      color: o.side === "buy" ? "#00A651" : "#E53935",
                    }}>
                      {o.side === "buy" ? "买入" : "卖出"}
                    </span>
                  </td>
                  <td style={{ fontWeight: 600 }}>{o.symbol}</td>
                  <td>{o.lots?.toFixed(2) ?? "—"}</td>
                  <td>{fmtPrice(o.openPrice)}</td>
                  <td>{fmtPrice(o.closePrice)}</td>
                  <td style={{ color: (o.profit ?? 0) >= 0 ? "#00A651" : "#E53935", fontWeight: 600 }}>
                    {(o.profit ?? 0) >= 0 ? "+" : ""}{o.profit?.toFixed(2) ?? "0.00"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {/* Pagination */}
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 16 }}>
            <span style={{ fontSize: 13, color: "#8A9AA5" }}>共 {total} 条</span>
            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
              <select
                value={pageSize}
                onChange={(e) => { setPageSize(Number(e.target.value)); setPage(1); }}
                style={{ padding: "6px 12px", borderRadius: 4, border: "1px solid var(--color-border)" }}
              >
                <option value={10}>10 条/页</option>
                <option value={20}>20 条/页</option>
                <option value={50}>50 条/页</option>
              </select>
              <button
                disabled={page === 1}
                onClick={() => setPage(p => p - 1)}
                className="btn-secondary"
                style={{ padding: "6px 12px", fontSize: 12 }}
              >
                上一页
              </button>
              <span style={{ fontSize: 14 }}>第 {page} / {totalPages} 页</span>
              <button
                disabled={page >= totalPages}
                onClick={() => setPage(p => p + 1)}
                className="btn-secondary"
                style={{ padding: "6px 12px", fontSize: 12 }}
              >
                下一页
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

