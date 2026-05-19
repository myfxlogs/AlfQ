// BindAccount — AntTrader-style 3-step broker binding wizard
import { useState } from "react";
import { brokerClient, accountClient } from "../api/client";
import type { BrokerMatch } from "../gen/alfq/v1/broker_pb";

type MtType = "MT4" | "MT5";

export default function BindAccount({ onDone }: { onDone?: () => void }) {
  const done = onDone || (() => { window.location.hash = "#/"; });
  const [step, setStep] = useState(1);
  const [mtType, setMtType] = useState<MtType>("MT5");
  const [companySearch, setCompanySearch] = useState("");
  const [searchResults, setSearchResults] = useState<BrokerMatch[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedCompany, setSelectedCompany] = useState<BrokerMatch | null>(null);
  const [selectedServer, setSelectedServer] = useState("");
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<{ balance?: number; equity?: number; status?: string } | null>(null);

  const handleSearch = async () => {
    if (!companySearch.trim()) return;
    setSearching(true);
    setSearchResults([]);
    setSelectedCompany(null);
    setSelectedServer("");
    try {
      const res = await brokerClient.searchBroker({ platform: mtType, keyword: companySearch.trim() });
      setSearchResults(res.matches ?? []);
    } catch {
      setSearchResults([]);
    } finally {
      setSearching(false);
    }
  };

  const handleSubmit = async () => {
    if (!selectedCompany || !selectedServer || !login || !password) return;
    setLoading(true);
    setError("");
    try {
      const res = await accountClient.createAccount({
        tenantId: "", brokerId: "00000000-0000-0000-0000-000000000000",
        login, password, server: selectedServer, accountType: "demo",
      });
      setResult({ balance: res.balance, equity: res.equity, status: res.status });
      setStep(3);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "绑定失败");
      setResult({ status: "error" });
      setStep(3);
    } finally {
      setLoading(false);
    }
  };

  const serverOptions = selectedCompany
    ? selectedCompany.servers.filter((s) => s)
    : [];

  return (
    <div style={{ maxWidth: 560, margin: "2rem auto" }}>
      {/* Step indicator */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 0, marginBottom: "2rem" }}>
        {[1, 2, 3].map((s) => (
          <div key={s} style={{ display: "flex", alignItems: "center" }}>
            <div style={{
              width: 32, height: 32, borderRadius: "50%", display: "flex", alignItems: "center", justifyContent: "center",
              background: step >= s ? "linear-gradient(135deg, #D4AF37, #B8960B)" : "var(--color-bg-tertiary)",
              color: step >= s ? "#fff" : "var(--color-text-muted)", fontWeight: 700, fontSize: 14,
            }}>
              {step > s ? "✓" : s}
            </div>
            {s < 3 && <div style={{ width: 48, height: 2, background: step > s ? "var(--color-primary)" : "var(--color-bg-tertiary)", margin: "0 8px" }} />}
          </div>
        ))}
      </div>

      <div className="glass-card" style={{ padding: "2rem" }}>
        {/* Step 1: Select platform & broker */}
        {step === 1 && (
          <div>
            <h2 style={{ textAlign: "center", margin: "0 0 0.5rem", color: "var(--color-text)" }}>选择经纪商</h2>
            <p style={{ textAlign: "center", margin: "0 0 1.5rem", color: "var(--color-text-muted)", fontSize: 14 }}>
              选择交易平台并搜索经纪商
            </p>

            {/* Platform cards */}
            <div style={{ display: "flex", gap: "1rem", marginBottom: "1.5rem" }}>
              {(["MT4", "MT5"] as MtType[]).map((p) => (
                <div key={p} onClick={() => { setMtType(p); setSearchResults([]); }}
                  style={{
                    flex: 1, padding: "1.25rem", borderRadius: 12, cursor: "pointer", textAlign: "center",
                    background: mtType === p ? "rgba(212,175,55,0.1)" : "var(--color-bg-secondary)",
                    border: `2px solid ${mtType === p ? "var(--color-primary)" : "transparent"}`,
                    transition: "all 0.2s",
                  }}>
                  <div style={{ fontSize: 24, fontWeight: 700, color: mtType === p ? "var(--color-primary)" : "var(--color-text)" }}>
                    {p}
                  </div>
                  <div style={{ fontSize: 12, color: "var(--color-text-muted)", marginTop: 4 }}>
                    {p === "MT4" ? "MetaTrader 4" : "MetaTrader 5"}
                  </div>
                </div>
              ))}
            </div>

            {/* Search box */}
            <div style={{ display: "flex", gap: "0.5rem", marginBottom: "1rem" }}>
              <input className="input" style={{ flex: 1, height: 48, fontSize: 16 }}
                placeholder="输入经纪商名称（如 RoboForex、IC Markets）"
                value={companySearch}
                onChange={(e) => setCompanySearch(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleSearch()} />
              <button className="btn-primary" onClick={handleSearch} disabled={searching}
                style={{ padding: "0 24px", height: 48, whiteSpace: "nowrap" }}>
                {searching ? "搜索中..." : "搜索"}
              </button>
            </div>

            {/* Company dropdown */}
            {searchResults.length > 0 && (
              <div style={{ marginBottom: "1rem" }}>
                <label style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600, color: "var(--color-text)", fontSize: 14 }}>
                  选择经纪商
                </label>
                <select className="input" style={{ width: "100%", height: 44, fontSize: 14 }}
                  value={selectedCompany?.company ?? ""}
                  onChange={(e) => {
                    const c = searchResults.find((m) => m.company === e.target.value);
                    setSelectedCompany(c || null);
                    setSelectedServer("");
                  }}>
                  <option value="">请选择...</option>
                  {searchResults.map((m) => (
                    <option key={m.company} value={m.company}>{m.company} ({m.servers.length} 服务器)</option>
                  ))}
                </select>
              </div>
            )}

            {/* Server dropdown */}
            {selectedCompany && serverOptions.length > 0 && (
              <div style={{ marginBottom: "1rem" }}>
                <label style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600, color: "var(--color-text)", fontSize: 14 }}>
                  选择服务器
                </label>
                <select className="input" style={{ width: "100%", height: 44, fontSize: 14 }}
                  value={selectedServer}
                  onChange={(e) => setSelectedServer(e.target.value)}>
                  <option value="">请选择...</option>
                  {serverOptions.map((s) => (
                    <option key={s} value={s}>{s}</option>
                  ))}
                </select>
              </div>
            )}

            {selectedCompany && selectedServer && (
              <button className="btn-primary" onClick={() => setStep(2)} style={{ width: "100%", height: 48, fontSize: 16 }}>
                下一步
              </button>
            )}
          </div>
        )}

        {/* Step 2: Enter credentials */}
        {step === 2 && (
          <div>
            <h2 style={{ textAlign: "center", margin: "0 0 1.5rem", color: "var(--color-text)" }}>输入交易凭据</h2>

            <div className="stat-card" style={{ marginBottom: "1rem", fontSize: 14, display: "flex", justifyContent: "space-between" }}>
              <span style={{ color: "var(--color-text-muted)" }}>平台</span>
              <span style={{ fontWeight: 600 }}>{mtType}</span>
            </div>
            <div className="stat-card" style={{ marginBottom: "1rem", fontSize: 14, display: "flex", justifyContent: "space-between" }}>
              <span style={{ color: "var(--color-text-muted)" }}>经纪商</span>
              <span style={{ fontWeight: 600 }}>{selectedCompany?.company}</span>
            </div>
            <div className="stat-card" style={{ marginBottom: "1.5rem", fontSize: 14, display: "flex", justifyContent: "space-between" }}>
              <span style={{ color: "var(--color-text-muted)" }}>服务器</span>
              <span style={{ fontWeight: 600, fontSize: 12 }}>{selectedServer}</span>
            </div>

            <input className="input" style={{ width: "100%", height: 48, fontSize: 16, marginBottom: "0.75rem" }}
              placeholder="交易账号 (Login)" value={login}
              onChange={(e) => setLogin(e.target.value)} />
            <input className="input" type="text" style={{ width: "100%", height: 48, fontSize: 16, marginBottom: "1.5rem" }}
              placeholder="交易密码 (Password)" value={password}
              onChange={(e) => setPassword(e.target.value)} />

            <div style={{ display: "flex", gap: "0.5rem" }}>
              <button className="btn-secondary" onClick={() => setStep(1)} style={{ flex: 1, height: 48 }}>返回</button>
              <button className="btn-primary" onClick={handleSubmit}
                disabled={!login || !password || loading}
                style={{ flex: 1, height: 48, fontSize: 16, opacity: !login || !password ? 0.5 : 1 }}>
                {loading ? "验证中..." : "确认绑定"}
              </button>
            </div>
          </div>
        )}

        {/* Step 3: Result */}
        {step === 3 && (
          <div>
            <h2 style={{ textAlign: "center", margin: "0 0 1.5rem", color: "var(--color-text)" }}>
              {result?.status === "connected" ? "绑定成功" : "绑定失败"}
            </h2>

            <div className="stat-card" style={{ marginBottom: "1.5rem" }}>
              <div style={{ display: "flex", justifyContent: "space-between", padding: "0.5rem 0" }}>
                <span style={{ color: "var(--color-text-muted)" }}>状态</span>
                <span style={{
                  color: result?.status === "connected" ? "var(--color-success)" : "var(--color-danger)",
                  fontWeight: 600,
                }}>{result?.status === "connected" ? "已连接" : "失败"}</span>
              </div>
              {result?.balance !== undefined && (
                <>
                  <div style={{ display: "flex", justifyContent: "space-between", padding: "0.5rem 0" }}>
                    <span style={{ color: "var(--color-text-muted)" }}>余额</span>
                    <span style={{ fontWeight: 600 }}>${result.balance?.toFixed(2)}</span>
                  </div>
                  <div style={{ display: "flex", justifyContent: "space-between", padding: "0.5rem 0" }}>
                    <span style={{ color: "var(--color-text-muted)" }}>净值</span>
                    <span style={{ fontWeight: 600 }}>${result.equity?.toFixed(2)}</span>
                  </div>
                </>
              )}
            </div>

            {error && <p style={{ color: "var(--color-danger)", fontSize: 14, marginBottom: "1rem" }}>{error}</p>}

            {result?.status === "connected" ? (
              <button className="btn-primary" onClick={done} style={{ width: "100%", height: 48, fontSize: 16 }}>完成</button>
            ) : (
              <div style={{ display: "flex", gap: "0.5rem" }}>
                <button className="btn-secondary" onClick={() => { setStep(1); setError(""); }} style={{ flex: 1, height: 48 }}>重试</button>
                <button className="btn-primary" onClick={done} style={{ flex: 1, height: 48 }}>返回列表</button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
