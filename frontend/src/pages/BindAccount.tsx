// BindAccount — 三步交易账户绑定向导 (AntTrader 风格)
import { useState } from "react";
import { brokerClient, accountClient } from "../api/client";
import type { BrokerMatch, BrokerServer } from "../gen/alfq/v1/broker_pb";

type MtType = "MT4" | "MT5";

export default function BindAccount({ onDone }: { onDone?: () => void }) {
  const done = onDone || (() => { window.location.hash = "#/"; });
  const [step, setStep] = useState(1);
  const [mtType, setMtType] = useState<MtType>("MT5");
  const [companySearch, setCompanySearch] = useState("");
  const [searchResults, setSearchResults] = useState<BrokerMatch[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedCompany, setSelectedCompany] = useState<BrokerMatch | null>(null);
  const [selectedServer, setSelectedServer] = useState<BrokerServer | null>(null);
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");
  const [result, setResult] = useState<{ balance?: number; equity?: number; status?: string } | null>(null);

  const handleSearch = async () => {
    if (!companySearch.trim()) { setMsg("请输入经纪商名称"); return; }
    setSearching(true); setMsg(""); setSearchResults([]);
    setSelectedCompany(null); setSelectedServer(null);
    try {
      const res = await brokerClient.searchBroker({ platform: mtType, keyword: companySearch.trim() });
      setSearchResults(res.matches ?? []);
      if (!res.matches?.length) setMsg("未找到匹配的经纪商");
    } catch (e: unknown) { setMsg(e instanceof Error ? e.message : "搜索失败"); }
    finally { setSearching(false); }
  };

  const handleBind = async () => {
    if (!selectedServer) return;
    setLoading(true); setError("");
    try {
      const res = await accountClient.createAccount({
        tenantId: "", brokerId: "00000000-0000-0000-0000-000000000000",
        login, password, server: selectedServer.access, accountType: "demo",
        mtType,
      });
      setResult({ balance: res.balance, equity: res.equity, status: res.status });
      setStep(3);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "绑定失败");
      setResult({ status: "error" }); setStep(3);
    } finally { setLoading(false); }
  };

  // Step indicator
  const StepIndicator = () => (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 0, marginBottom: "2rem" }}>
      {[1, 2, 3].map((s) => (
        <div key={s} style={{ display: "flex", alignItems: "center" }}>
          <div style={{
            width: 32, height: 32, borderRadius: "50%", display: "flex", alignItems: "center", justifyContent: "center",
            fontSize: 14, fontWeight: 600,
            background: step >= s ? "linear-gradient(135deg, #D4AF37 0%, #B8960B 100%)" : "var(--color-bg-tertiary)",
            color: step >= s ? "#fff" : "var(--color-text-muted)",
          }}>
            {step > s ? "✓" : s}
          </div>
          {s < 3 && <div style={{ width: 48, height: 2, margin: "0 4px", background: step > s ? "var(--color-primary)" : "var(--color-bg-tertiary)" }} />}
        </div>
      ))}
    </div>
  );

  return (
    <div style={{ maxWidth: 560, margin: "0 auto", padding: "2rem 1rem" }}>
      <StepIndicator />

      {step === 1 && (
        <div>
          <div style={{ textAlign: "center", marginBottom: "1.5rem" }}>
            <h2 style={{ margin: 0, color: "var(--color-text)", fontSize: 20 }}>选择经纪商</h2>
            <p style={{ margin: "0.5rem 0 0", color: "var(--color-text-muted)", fontSize: 14 }}>
              搜索并选择要绑定的交易平台和服务器
            </p>
          </div>

          {/* Platform cards */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={{ display: "block", marginBottom: "0.75rem", fontWeight: 600, color: "var(--color-text)", fontSize: 14 }}>交易平台</label>
            <div style={{ display: "flex", gap: "1rem" }}>
              {(["MT4","MT5"] as MtType[]).map((p) => (
                <div key={p} onClick={() => { setMtType(p); setSearchResults([]); setSelectedCompany(null); setSelectedServer(null); setMsg(""); }}
                  style={{
                    flex: 1, padding: "1rem", borderRadius: 12, cursor: "pointer", textAlign: "center", transition: "all 0.2s",
                    background: mtType === p ? "rgba(212,175,55,0.1)" : "var(--color-bg-secondary)",
                    border: `2px solid ${mtType === p ? "var(--color-primary)" : "transparent"}`,
                  }}>
                  <div style={{ fontSize: 24, fontWeight: 700, color: mtType === p ? "var(--color-primary)" : "var(--color-text)" }}>{p}</div>
                  <div style={{ fontSize: 12, color: "var(--color-text-muted)", marginTop: 4 }}>MetaTrader {p === "MT4" ? "4" : "5"}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Search */}
          <div style={{ marginBottom: "1rem" }}>
            <label style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600, color: "var(--color-text)", fontSize: 14 }}>经纪商名称</label>
            <div style={{ display: "flex", gap: "0.5rem" }}>
              <input className="input" style={{ flex: 1, height: 48, fontSize: 16 }} placeholder="如 RoboForex, IC Markets..."
                value={companySearch} onChange={(e) => setCompanySearch(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleSearch()} />
              <button className="btn-primary" onClick={handleSearch} disabled={searching}
                style={{ padding: "0 1.5rem", height: 48, whiteSpace: "nowrap" }}>
                {searching ? "搜索中..." : "搜索"}
              </button>
            </div>
          </div>
          {msg && <p style={{ color: "var(--color-text-muted)", fontSize: 14, marginBottom: "0.5rem" }}>{msg}</p>}

          {/* Company dropdown */}
          {searchResults.length > 0 && (
            <div style={{ marginBottom: "1rem" }}>
              <label style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600, color: "var(--color-text)", fontSize: 14 }}>经纪商</label>
              <select className="input" style={{ width: "100%", height: 48, fontSize: 14, cursor: "pointer" }}
                value={selectedCompany?.company ?? ""}
                onChange={(e) => {
                  const c = searchResults.find((m) => m.company === e.target.value);
                  setSelectedCompany(c || null); setSelectedServer(null);
                }}>
                <option value="">选择经纪商</option>
                {searchResults.map((m) => (
                  <option key={m.company} value={m.company}>{m.company}</option>
                ))}
              </select>
            </div>
          )}

          {/* Server dropdown */}
          {selectedCompany && (
            <div style={{ marginBottom: "1rem" }}>
              <label style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600, color: "var(--color-text)", fontSize: 14 }}>交易服务器</label>
              <select className="input" style={{ width: "100%", height: 48, fontSize: 14, cursor: "pointer" }}
                value={selectedServer?.name ?? ""}
                onChange={(e) => {
                  const s = selectedCompany.servers.find((sv) => sv.name === e.target.value);
                  setSelectedServer(s || null);
                }}>
                <option value="">选择服务器</option>
                {[...new Map(selectedCompany.servers.map((s) => [s.name, s] as const)).values()]
                  .sort((a, b) => a.name.localeCompare(b.name))
                  .map((s) => (
                    <option key={s.name} value={s.name}>{s.name}</option>
                  ))}
              </select>
            </div>
          )}

          {/* Next button */}
          {selectedServer && (
            <button className="btn-primary" onClick={() => setStep(2)} style={{ width: "100%", height: 48, fontSize: 16 }}>
              下一步
            </button>
          )}
        </div>
      )}

      {/* Step 2: Credentials */}
      {step === 2 && (
        <div>
          <div style={{ textAlign: "center", marginBottom: "1.5rem" }}>
            <h2 style={{ margin: 0, color: "var(--color-text)", fontSize: 20 }}>输入交易凭据</h2>
            <p style={{ margin: "0.5rem 0 0", color: "var(--color-text-muted)", fontSize: 14 }}>
              平台: {mtType} · {selectedCompany?.company}
            </p>
          </div>
          <div className="stat-card" style={{ marginBottom: "1rem", fontSize: 14, display: "flex", justifyContent: "space-between" }}>
            <span style={{ color: "var(--color-text-muted)" }}>服务器</span>
            <span style={{ fontWeight: 600 }}>{selectedServer?.name}</span>
          </div>
          <input className="input" style={{ width: "100%", height: 48, fontSize: 16, marginBottom: "0.75rem" }}
            placeholder="交易账号 (Login)" value={login} onChange={(e) => setLogin(e.target.value)} />
          <input className="input" type="text" style={{ width: "100%", height: 48, fontSize: 16, marginBottom: "1rem" }}
            placeholder="交易密码 (Password)" value={password} onChange={(e) => setPassword(e.target.value)} />
          <div style={{ display: "flex", gap: "0.5rem" }}>
            <button className="btn-secondary" onClick={() => setStep(1)} style={{ flex: 1, height: 48 }}>返回</button>
            <button className="btn-primary" onClick={handleBind} disabled={!login || !password || loading}
              style={{ flex: 1, height: 48, opacity: !login || !password ? 0.5 : 1 }}>
              {loading ? "验证中..." : "确认绑定"}
            </button>
          </div>
        </div>
      )}

      {/* Step 3: Result */}
      {step === 3 && (
        <div style={{ textAlign: "center" }}>
          <div style={{ fontSize: 48, marginBottom: "1rem" }}>
            {result?.status === "connected" ? "✅" : "❌"}
          </div>
          <h2 style={{ color: "var(--color-text)", marginBottom: "0.5rem" }}>
            {result?.status === "connected" ? "绑定成功" : "绑定失败"}
          </h2>
          {result?.status === "connected" && result.balance !== undefined && (
            <div className="glass-card" style={{ padding: "1.5rem", marginBottom: "1.5rem", display: "inline-block", textAlign: "left" }}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: "2rem", padding: "0.25rem 0" }}>
                <span style={{ color: "var(--color-text-muted)" }}>余额</span>
                <span style={{ fontWeight: 600 }}>${result.balance.toFixed(2)}</span>
              </div>
              <div style={{ display: "flex", justifyContent: "space-between", gap: "2rem", padding: "0.25rem 0" }}>
                <span style={{ color: "var(--color-text-muted)" }}>净值</span>
                <span style={{ fontWeight: 600 }}>${result.equity?.toFixed(2)}</span>
              </div>
            </div>
          )}
          {error && <p style={{ color: "var(--color-danger)", marginBottom: "1rem" }}>{error}</p>}
          <button className="btn-primary" onClick={done} style={{ width: "100%", maxWidth: 300, height: 48 }}>
            {result?.status === "connected" ? "完成" : "返回"}
          </button>
        </div>
      )}
    </div>
  );
}
