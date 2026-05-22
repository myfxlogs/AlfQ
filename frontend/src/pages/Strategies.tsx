// Strategies page — create, deploy, stop strategies.
import { useEffect, useState } from "react";
import { strategyClient, accountClient } from "../api/client";
import type { Strategy } from "../gen/alfq/v1/strategy_pb";
import type { Account } from "../gen/alfq/v1/broker_pb";

export default function Strategies() {
  const [strategies, setStrategies] = useState<Strategy[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [deployTarget, setDeployTarget] = useState<string | null>(null);

  // Form state
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [specJson, setSpecJson] = useState("");

  const load = () => {
    setLoading(true);
    Promise.all([
      strategyClient.listStrategies({ tenantId: "" }),
      accountClient.listAccounts({ tenantId: "" }),
    ])
      .then(([sRes, aRes]) => {
        setStrategies(sRes.strategies ?? []);
        setAccounts(aRes.accounts ?? []);
        setLoading(false);
      })
      .catch(e => { setError(e instanceof Error ? e.message : "加载失败"); setLoading(false); });
  };

  useEffect(() => { load(); }, []);

  const handleCreate = async () => {
    if (!name || !specJson) { setError("名称和规格 JSON 不能为空"); return; }
    setError(""); setMsg("");
    try {
      await strategyClient.createStrategy({ tenantId: "", name, description, specJson });
      setMsg("策略创建成功");
      setShowCreate(false); setName(""); setDescription(""); setSpecJson("");
      load();
    } catch (e: unknown) { setError(e instanceof Error ? e.message : "创建失败"); }
  };

  const handleDeploy = async (strategyId: string) => {
    if (!deployTarget) return;
    setError(""); setMsg("");
    try {
      await strategyClient.deployStrategy({ id: strategyId, accountId: deployTarget });
      setMsg("策略已部署");
      setDeployTarget(null);
      load();
    } catch (e: unknown) { setError(e instanceof Error ? e.message : "部署失败"); }
  };

  const handleStop = async (strategyId: string) => {
    setError(""); setMsg("");
    try {
      await strategyClient.stopStrategy({ id: strategyId });
      setMsg("策略已停止");
      load();
    } catch (e: unknown) { setError(e instanceof Error ? e.message : "停止失败"); }
  };

  const demoSpec = JSON.stringify({
    canonical_symbols: ["EURUSD"],
    period: "1h",
    factors: { sma20: "sma($close, 20)", sma60: "sma($close, 60)" },
    signal_rule: "sma20 > sma60 ? 1 : -1",
    sizing: { type: "fixed_lots", lots: 0.1 },
  }, null, 2);

  return (
    <div className="page">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1.5rem" }}>
        <div>
          <h1 className="page-title" style={{ marginBottom: "0.25rem" }}>策略管理</h1>
          <p style={{ color: "var(--color-text-muted)", margin: 0, fontSize: 14 }}>
            {strategies.length} 个策略
          </p>
        </div>
        <button className="btn-primary" onClick={() => setShowCreate(!showCreate)}>
          + 创建策略
        </button>
      </div>

      {error && <div style={{ color: "var(--color-danger)", marginBottom: 12 }}>{error}</div>}
      {msg && <div style={{ color: "var(--color-success)", marginBottom: 12 }}>{msg}</div>}

      {showCreate && (
        <div className="glass-card" style={{ padding: "1.5rem", marginBottom: "1rem" }}>
          <h3 style={{ marginTop: 0 }}>新建策略</h3>
          <div style={{ display: "grid", gap: "0.75rem" }}>
            <input className="input" placeholder="策略名称" value={name} onChange={e => setName(e.target.value)} />
            <input className="input" placeholder="描述（可选）" value={description} onChange={e => setDescription(e.target.value)} />
            <textarea className="input" rows={8} placeholder="规格 JSON" value={specJson}
              onChange={e => setSpecJson(e.target.value)}
              style={{ fontFamily: "monospace", fontSize: 13 }} />
            <button className="btn-secondary" onClick={() => setSpecJson(demoSpec)}
              style={{ width: "fit-content", fontSize: 12 }}>填入 SMA 示例</button>
            <button className="btn-primary" onClick={handleCreate}>创建</button>
          </div>
        </div>
      )}

      <div className="glass-card" style={{ overflow: "auto" }}>
        {loading ? <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>加载中...</p>
        : strategies.length === 0 ? <p style={{ padding: "2rem", textAlign: "center", color: "var(--color-text-muted)" }}>暂无策略</p>
        : (
          <table className="table">
            <thead><tr><th>ID</th><th>名称</th><th>描述</th><th>状态</th><th>操作</th></tr></thead>
            <tbody>
              {strategies.map(s => (
                <tr key={s.id}>
                  <td style={{ fontFamily: "monospace", fontSize: 11, maxWidth: 120, overflow: "hidden", textOverflow: "ellipsis" }}>{s.id}</td>
                  <td style={{ fontWeight: 600 }}>{s.name}</td>
                  <td style={{ fontSize: 13, color: "var(--color-text-secondary)" }}>{s.description || "—"}</td>
                  <td>
                    <span style={{
                      padding: "2px 8px", borderRadius: 12, fontSize: 12, fontWeight: 600,
                      background: s.status === "live" ? "rgba(0,166,81,0.1)" :
                        s.status === "paper" ? "rgba(33,150,243,0.1)" :
                        s.status === "ready" ? "rgba(212,175,55,0.1)" : "var(--color-bg-tertiary)",
                      color: s.status === "live" ? "var(--color-success)" :
                        s.status === "paper" ? "#1976D2" : s.status === "ready" ? "#B8960B" : "var(--color-text-muted)",
                    }}>{s.status || "draft"}</span>
                  </td>
                  <td>
                    <div style={{ display: "flex", gap: 6 }}>
                      {s.status !== "live" && s.status !== "paper" && (
                        <>
                          <select className="input" style={{ height: 30, fontSize: 12, width: 140 }}
                            value={deployTarget === s.id ? deployTarget : ""}
                            onChange={e => setDeployTarget(e.target.value || null)}>
                            <option value="">选择账户...</option>
                            {accounts.filter(a => a.status === "connected").map(a => (
                              <option key={a.id} value={a.id}>{a.login} ({a.platform})</option>
                            ))}
                          </select>
                          <button className="btn-primary" style={{ height: 30, fontSize: 12, padding: "0 12px" }}
                            onClick={() => handleDeploy(s.id)}
                            disabled={deployTarget !== s.id}>部署</button>
                        </>
                      )}
                      <button className="btn-secondary" style={{ height: 30, fontSize: 12, padding: "0 12px" }}
                        onClick={() => handleStop(s.id)}>停止</button>
                    </div>
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
