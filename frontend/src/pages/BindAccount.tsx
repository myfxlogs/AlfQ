// BindAccount — 三步交易账户绑定向导 (AntTrader 风格)
// 改动: login 纯数字验证 + "只读密码/观摩密码" 文案 + 30s 超时
import { useState, useRef } from "react";
import { brokerClient, accountClient } from "../api/client";
import type { BrokerMatch, BrokerServer } from "../gen/alfq/v1/broker_pb";

type MtType = "MT4" | "MT5";

// ── Login 数字过滤 ──
const ALLOWED_KEYS = new Set([
  "Backspace", "Delete", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown",
  "Home", "End", "Tab", "Enter", "Escape",
]);

export default function BindAccount({ onDone }: { onDone?: () => void }) {
  const done = onDone || (() => { window.location.hash = "#/"; });
  const [step, setStep] = useState(1);
  const [mtType, setMtType] = useState<MtType>("MT5");
  const [companySearch, setCompanySearch] = useState("");
  const [searchResults, setSearchResults] = useState<BrokerMatch[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedCompany, setSelectedCompany] = useState<BrokerMatch | null>(null);
  const [selectedServer, setSelectedServer] = useState<BrokerServer | null>(null);
  const [brokerId, setBrokerId] = useState("");
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");
  const [result, setResult] = useState<{ balance?: number; equity?: number; status?: string } | null>(null);
  const loginRef = useRef<HTMLInputElement>(null);

  // ── Login 输入处理（仅保留数字过滤作为输入辅助，业务验证由后端负责）──
  const handleLoginChange = (raw: string) => {
    setLogin(raw.replace(/\D/g, ""));
  };

  const handleLoginKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (ALLOWED_KEYS.has(e.key)) return;
    if (e.key >= "0" && e.key <= "9") return;
    if ((e.ctrlKey || e.metaKey) && (e.key === "a" || e.key === "c" || e.key === "v")) return;
    e.preventDefault();
  };

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
    if (!login) { setError("请输入交易账号"); return; }
    if (!password) { setError("请输入只读密码/观摩密码"); return; }
    setLoading(true); setError("");

    const ctrl = new AbortController();
    const timer = setTimeout(() => { ctrl.abort(); }, 30_000);

    try {
      const res = await accountClient.createAccount({
        tenantId: "", brokerId,
        login, password, server: selectedServer.access, serverName: selectedServer.name, accountType: "demo",
        mtType,
      });
      clearTimeout(timer);
      setPassword(""); // 成功后丢弃密码
      setResult({ balance: res.balance, equity: res.equity, status: res.status });
      setStep(3);
    } catch (e: unknown) {
      clearTimeout(timer);
      setPassword(""); // 失败后丢弃密码
      const msg = e instanceof Error ? e.message : "绑定失败";
      setError(msg);
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
                  setBrokerId(c?.id ?? "");
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
            <button className="btn-primary" onClick={() => { setStep(2); setError(""); }}
              style={{ width: "100%", height: 48, fontSize: 16 }}>
              下一步
            </button>
          )}
        </div>
      )}

      {/* Step 2: 账号信息 */}
      {step === 2 && (
        <div>
          <div style={{ textAlign: "center", marginBottom: "1.5rem" }}>
            <h2 style={{ margin: 0, color: "var(--color-text)", fontSize: 20 }}>输入账号信息</h2>
            <p style={{ margin: "0.5rem 0 0", color: "var(--color-text-muted)", fontSize: 14 }}>
              平台: {mtType} · {selectedCompany?.company}
            </p>
          </div>
          <div className="stat-card" style={{ marginBottom: "1rem", fontSize: 14, display: "flex", justifyContent: "space-between" }}>
            <span style={{ color: "var(--color-text-muted)" }}>服务器</span>
            <span style={{ fontWeight: 600 }}>{selectedServer?.name}</span>
          </div>

          {/* 交易账号 */}
          <div style={{ marginBottom: "0.75rem" }}>
            <input
              ref={loginRef}
              className="input"
              inputMode="numeric"
              style={{ width: "100%", height: 48, fontSize: 16, fontFamily: "monospace" }}
              placeholder="交易账号 (Login)"
              value={login}
              onChange={(e) => handleLoginChange(e.target.value)}
              onKeyDown={handleLoginKeyDown}
            />
          </div>

          {/* 只读密码/观摩密码 */}
          <input className="input" type="text" style={{ width: "100%", height: 48, fontSize: 16, marginBottom: "1rem", fontFamily: "monospace" }}
            placeholder="只读密码/观摩密码 (Investor Password)" value={password} onChange={(e) => setPassword(e.target.value)} />

          {error && <p style={{ color: "var(--color-danger)", fontSize: 13, marginBottom: "0.75rem" }}>{error}</p>}

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
            <div className="glass-card" style={{ padding: "1.25rem 1.5rem", margin: "0 auto 1.5rem", maxWidth: 260 }}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: "1.5rem", padding: "0.3rem 0" }}>
                <span style={{ color: "var(--color-text-muted)", fontSize: 14 }}>余额</span>
                <span style={{ fontWeight: 600, fontSize: 15, color: "var(--color-success)" }}>${result.balance.toFixed(2)}</span>
              </div>
              <div style={{ display: "flex", justifyContent: "space-between", gap: "1.5rem", padding: "0.3rem 0" }}>
                <span style={{ color: "var(--color-text-muted)", fontSize: 14 }}>净值</span>
                <span style={{ fontWeight: 600, fontSize: 15, color: "var(--color-success)" }}>${result.equity?.toFixed(2)}</span>
              </div>
            </div>
          )}
          {error && <p style={{ color: "var(--color-danger)", fontSize: 13, marginBottom: "1rem" }}>{error}</p>}
          <button className="btn-primary" onClick={done} style={{ width: "100%", maxWidth: 300, height: 48, margin: "0 auto" }}>
            {result?.status === "connected" ? "完成" : "返回"}
          </button>
        </div>
      )}
    </div>
  );
}
