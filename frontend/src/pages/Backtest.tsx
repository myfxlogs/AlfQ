// Backtest page — submit and view backtest results.
import { useEffect, useState } from "react";
import { backtestClient, strategyClient } from "../api/client";
import type { Strategy } from "../gen/alfq/v1/strategy_pb";
import type { BacktestTask } from "../gen/alfq/v1/strategy_pb";

export default function Backtest() {
  const [strategies, setStrategies] = useState<Strategy[]>([]);
  const [tasks, setTasks] = useState<BacktestTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");
  const [selectedStrategy, setSelectedStrategy] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const load = () => {
    setLoading(true);
    Promise.all([
      strategyClient.listStrategies({ tenantId: "" }),
      backtestClient.listBacktests({ strategyId: "" }),
    ])
      .then(([sRes, bRes]) => {
        setStrategies(sRes.strategies ?? []);
        setTasks(bRes.tasks ?? []);
        setLoading(false);
      })
      .catch(e => { setError(e instanceof Error ? e.message : "加载失败"); setLoading(false); });
  };

  useEffect(() => { load(); }, []);

  const handleRun = async () => {
    if (!selectedStrategy) return;
    setSubmitting(true); setMsg(""); setError("");
    try {
      const endMs = BigInt(Date.now());
      const startMs = endMs - BigInt(90 * 24 * 3600 * 1000); // 90 days back
      await backtestClient.runBacktest({ strategyId: selectedStrategy, startTsMs: startMs, endTsMs: endMs });
      setMsg("回测已提交");
      setTimeout(load, 3000);
    } catch (e: unknown) { setError(e instanceof Error ? e.message : "提交失败"); }
    finally { setSubmitting(false); }
  };

  return (
    <div className="page">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1.5rem" }}>
        <div>
          <h1 className="page-title" style={{ marginBottom: "0.25rem" }}>回测管理</h1>
          <p style={{ color: "var(--color-text-muted)", margin: 0, fontSize: 14 }}>
            {tasks.length} 个回测任务
          </p>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <select className="input" value={selectedStrategy} onChange={e => setSelectedStrategy(e.target.value)}
            style={{ height: 40, fontSize: 14, minWidth: 180 }}>
            <option value="">选择策略...</option>
            {strategies.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
          <button className="btn-primary" onClick={handleRun} disabled={!selectedStrategy || submitting}
            style={{ height: 40, fontSize: 14 }}>
            {submitting ? "提交中..." : "运行回测"}
          </button>
        </div>
      </div>

      {error && <div style={{ color: "var(--color-danger)", marginBottom: 12 }}>{error}</div>}
      {msg && <div style={{ color: "var(--color-success)", marginBottom: 12 }}>{msg}</div>}

      <div className="glass-card" style={{ overflow: "auto" }}>
        {loading ? <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>加载中...</p>
        : tasks.length === 0 ? <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>暂无回测任务</p>
        : (
          <table className="table">
            <thead><tr><th>ID</th><th>策略</th><th>状态</th><th>结果</th></tr></thead>
            <tbody>
              {tasks.map(t => (
                <tr key={t.id}>
                  <td style={{ fontFamily: "monospace", fontSize: 11 }}>{t.id?.slice(0, 12)}</td>
                  <td>{t.strategyId?.slice(0, 12)}</td>
                  <td><span style={{
                    padding: "2px 8px", borderRadius: 12, fontSize: 12, fontWeight: 600,
                    background: t.status === "passed" ? "rgba(0,166,81,0.1)" :
                      t.status === "failed" ? "rgba(229,57,53,0.1)" : "var(--color-bg-tertiary)",
                    color: t.status === "passed" ? "var(--color-success)" :
                      t.status === "failed" ? "var(--color-danger)" : "var(--color-text-muted)",
                  }}>{t.status || "pending"}</span></td>
                  <td style={{ fontSize: 12 }}>{t.resultJson ? "有结果" : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
