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
    <div style={{ padding: "2rem" }}>
      <h1>ALFQ 仪表盘</h1>
      {error && <div style={{ color: "red", marginTop: 12 }}>{error}</div>}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: "1rem", marginTop: "2rem" }}>
        <StatCard title="账户数" value={loading ? "..." : String(accounts)} />
        <StatCard title="经纪商数" value={loading ? "..." : String(brokers)} />
        <StatCard title="策略数" value={loading ? "..." : String(strategies)} />
        <StatCard title="今日成交" value="—" />
      </div>
      <div style={{ marginTop: "2rem", padding: "1rem", background: "#f5f5f5", borderRadius: 8 }}>
        <h3>最近订单</h3>
        <p style={{ color: "#888" }}>暂无数据</p>
      </div>
    </div>
  );
}

function StatCard({title, value, color}:{title:string;value:string;color?:string}) {
  return <div style={{background:"#fff",padding:"1.5rem",borderRadius:8,boxShadow:"0 1px 3px rgba(0,0,0,0.1)"}}>
    <div style={{color:"#888",fontSize:14}}>{title}</div>
    <div style={{fontSize:24,fontWeight:700,color:color||"#333",marginTop:8}}>{value}</div>
  </div>;
}
