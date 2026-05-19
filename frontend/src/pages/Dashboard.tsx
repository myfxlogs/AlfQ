// Dashboard page — ALFQ main overview.
import { useEffect, useState } from "react";
import { brokerClient, accountClient, strategyClient } from "../api/client";

export default function Dashboard() {
  const [accounts, setAccounts] = useState(0);
  const [brokers, setBrokers] = useState(0);
  const [strategies, setStrategies] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      try {
        const [accRes, brkRes, stratRes] = await Promise.all([
          accountClient.listAccounts({}),
          brokerClient.listBrokers({}),
          strategyClient.listStrategies({}),
        ]);
        setAccounts(accRes.accounts?.length ?? 0);
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

  return (
    <div className="page">
      <h1 className="page-title">仪表盘</h1>
      {error && <div style={{ color: "var(--color-danger)", marginTop: 12 }}>{error}</div>}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: "1rem", marginTop: "2rem" }}>
        <StatCard title="账户数" value={loading ? "..." : String(accounts)} />
        <StatCard title="经纪商数" value={loading ? "..." : String(brokers)} />
        <StatCard title="策略数" value={loading ? "..." : String(strategies)} />
        <StatCard title="今日成交" value="—" />
      </div>
      <div className="glass-card" style={{ marginTop: "2rem", padding: "1.5rem" }}>
        <h3 style={{ margin: "0 0 1rem", color: "var(--color-text)" }}>最近订单</h3>
        <p style={{ color: "var(--color-text-muted)" }}>暂无数据</p>
      </div>
    </div>
  );
}

function StatCard({title, value, color}:{title:string;value:string;color?:string}) {
  return <div className="stat-card">
    <div style={{color:"var(--color-text-muted)",fontSize:14}}>{title}</div>
    <div style={{fontSize:24,fontWeight:700,color:color||"var(--color-text)",marginTop:8}}>{value}</div>
  </div>;
}
