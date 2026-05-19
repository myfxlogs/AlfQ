// BindAccount — 三步交易账户绑定向导（AntTrader 风格：按钮触发搜索）
import { useState } from "react";
import { brokerClient, accountClient } from "../api/client";
import type { BrokerMatch } from "../gen/alfq/v1/broker_pb";

type MtType = "MT4" | "MT5";

export default function BindAccount({ onDone }: { onDone: () => void }) {
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
  const [msg, setMsg] = useState("");
  const [result, setResult] = useState<{ balance?: number; equity?: number; status?: string } | null>(null);

  // Step 1: Search broker (button-triggered, like AntTrader)
  const handleSearch = async () => {
    if (!companySearch.trim()) {
      setMsg("请输入经纪商名称关键词");
      return;
    }
    setSearching(true);
    setMsg("");
    setSearchResults([]);
    setSelectedCompany(null);
    setSelectedServer("");
    try {
      const res = await brokerClient.searchBroker({ platform: mtType, keyword: companySearch.trim() });
      setSearchResults(res.matches ?? []);
      if (!res.matches?.length) {
        setMsg("未找到匹配的经纪商，请尝试其他关键词");
      }
    } catch (e: unknown) {
      setMsg(e instanceof Error ? e.message : "搜索失败");
    } finally {
      setSearching(false);
    }
  };

  const handleSelectCompany = (company: BrokerMatch) => {
    setSelectedCompany(company);
    setSelectedServer(company.servers?.[0] ?? "");
    setStep(2);
  };

  const handleSubmit = async () => {
    setLoading(true);
    setError("");
    try {
      const res = await accountClient.createAccount({
        tenantId: "",
        brokerId: "00000000-0000-0000-0000-000000000000",
        login,
        password,
        server: selectedServer,
        accountType: "demo",
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

  // Reset and go back to step 1
  const handleBack = () => {
    setStep(1);
    setSelectedCompany(null);
    setSelectedServer("");
    setLogin("");
    setPassword("");
    setError("");
  };

  return (
    <div className="page" style={{ maxWidth: 520 }}>
      <div className="glass-card" style={{ padding: "2rem" }}>
        {/* Step indicator */}
        <div style={{ display: "flex", gap: "0.5rem", marginBottom: "1.5rem" }}>
          {[1, 2, 3].map((n) => (
            <div key={n} style={{
              flex: 1, height: 4, borderRadius: 2,
              background: n <= step ? "var(--color-primary)" : "var(--color-bg-tertiary)",
              transition: "background 0.3s",
            }} />
          ))}
        </div>

        <h2 style={{ margin: "0 0 0.5rem", color: "var(--color-text)" }}>
          {step === 1 ? "选择经纪商" : step === 2 ? "输入交易凭据" : result?.status === "error" ? "绑定失败" : "绑定成功"}
        </h2>

        {step === 1 && (
          <div>
            {/* Platform selector */}
            <div style={{ display: "flex", gap: "0.5rem", marginBottom: "1rem" }}>
              {(["MT4", "MT5"] as MtType[]).map((p) => (
                <button key={p} className={mtType === p ? "btn-primary" : "btn-secondary"}
                  onClick={() => { setMtType(p); setSearchResults([]); setMsg(""); }}
                  style={{ flex: 1 }}>{p}</button>
              ))}
            </div>

            {/* Search input + button */}
            <div style={{ display: "flex", gap: "0.5rem", marginBottom: "1rem" }}>
              <input className="input" style={{ flex: 1 }} placeholder="输入经纪商名称（如 RoboForex）"
                value={companySearch}
                onChange={(e) => setCompanySearch(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleSearch()} />
              <button className="btn-primary" onClick={handleSearch} disabled={searching}
                style={{ whiteSpace: "nowrap" }}>
                {searching ? "搜索中..." : "搜索"}
              </button>
            </div>

            {msg && <p style={{ color: "var(--color-text-muted)", fontSize: 14, margin: "0 0 0.5rem" }}>{msg}</p>}

            {/* Results */}
            <div style={{ maxHeight: 300, overflow: "auto" }} className="scrollbar-thin">
              {searchResults.map((m, i) =>
                m.servers.map((server, j) => (
                  <div key={`${i}-${j}`}
                    onClick={() => { setSelectedCompany(m); setSelectedServer(server); setStep(2); }}
                    style={{
                      padding: "0.75rem 1rem", cursor: "pointer",
                      borderBottom: "1px solid var(--color-border)", transition: "background 0.2s",
                    }}
                    onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-bg-secondary)")}
                    onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}>
                    <div style={{ fontWeight: 600, color: "var(--color-text)" }}>{m.company}</div>
                    <div style={{ fontSize: 12, color: "var(--color-text-muted)", marginTop: 2 }}>{server}</div>
                  </div>
                ))
              )}
              {searchResults.length === 0 && !searching && !msg && (
                <p style={{ color: "var(--color-text-muted)", fontSize: 14, padding: "1rem 0" }}>
                  输入经纪商名称后点击「搜索」
                </p>
              )}
            </div>
          </div>
        )}

        {/* Step 2: Enter Credentials */}
        {step === 2 && (
          <div>
            <div className="stat-card" style={{ marginBottom: "1rem", fontSize: 14, display: "flex", justifyContent: "space-between" }}>
              <span style={{ color: "var(--color-text-muted)" }}>平台</span>
              <span style={{ fontWeight: 600 }}>{mtType}</span>
            </div>
            <div className="stat-card" style={{ marginBottom: "1rem", fontSize: 14, display: "flex", justifyContent: "space-between" }}>
              <span style={{ color: "var(--color-text-muted)" }}>经纪商</span>
              <span style={{ fontWeight: 600 }}>{selectedCompany?.company}</span>
            </div>
            <div className="stat-card" style={{ marginBottom: "1rem", fontSize: 14, display: "flex", justifyContent: "space-between" }}>
              <span style={{ color: "var(--color-text-muted)" }}>服务器</span>
              <span style={{ fontWeight: 600, fontSize: 12 }}>{selectedServer}</span>
            </div>
            <input className="input" style={{ width: "100%", marginBottom: "0.75rem" }}
              placeholder="交易账号 (Login)" value={login}
              onChange={(e) => setLogin(e.target.value)} />
            <input className="input" type="text" style={{ width: "100%", marginBottom: "1rem" }}
              placeholder="交易密码 (Password)" value={password}
              onChange={(e) => setPassword(e.target.value)} />
            <div style={{ display: "flex", gap: "0.5rem" }}>
              <button className="btn-secondary" onClick={handleBack} style={{ flex: 1 }}>返回</button>
              <button className="btn-primary" onClick={handleSubmit} disabled={!login || !password || loading}
                style={{ flex: 1, opacity: !login || !password ? 0.5 : 1 }}>
                {loading ? "验证中..." : "确认绑定"}
              </button>
            </div>
          </div>
        )}

        {/* Step 3: Result */}
        {step === 3 && (
          <div>
            <div className="stat-card" style={{ marginBottom: "1rem" }}>
              <div style={{ display: "flex", justifyContent: "space-between", padding: "0.25rem 0" }}>
                <span style={{ color: "var(--color-text-muted)" }}>状态</span>
                <span style={{
                  color: result?.status === "connected" ? "var(--color-success)" : "var(--color-danger)",
                  fontWeight: 600,
                }}>{result?.status === "connected" ? "已连接" : "失败"}</span>
              </div>
              {result?.balance !== undefined && (
                <>
                  <div style={{ display: "flex", justifyContent: "space-between", padding: "0.25rem 0" }}>
                    <span style={{ color: "var(--color-text-muted)" }}>余额</span>
                    <span style={{ fontWeight: 600 }}>${result.balance?.toFixed(2)}</span>
                  </div>
                  <div style={{ display: "flex", justifyContent: "space-between", padding: "0.25rem 0" }}>
                    <span style={{ color: "var(--color-text-muted)" }}>净值</span>
                    <span style={{ fontWeight: 600 }}>${result.equity?.toFixed(2)}</span>
                  </div>
                </>
              )}
            </div>
            {error && <p style={{ color: "var(--color-danger)", fontSize: 14, marginBottom: "0.5rem" }}>{error}</p>}
            {result?.status === "connected" ? (
              <button className="btn-primary" onClick={onDone} style={{ width: "100%" }}>完成</button>
            ) : (
              <div style={{ display: "flex", gap: "0.5rem" }}>
                <button className="btn-secondary" onClick={handleBack} style={{ flex: 1 }}>重试</button>
                <button className="btn-primary" onClick={onDone} style={{ flex: 1 }}>返回列表</button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
