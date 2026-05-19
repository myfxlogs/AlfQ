// BindAccount — 三步交易账户绑定向导（在线搜索经纪商）
import { useEffect, useState } from "react";
import { brokerClient, accountClient } from "../api/client";
import type { Broker, BrokerMatch } from "../gen/alfq/v1/broker_pb";

type MtType = "MT4" | "MT5";

interface BrokerSelection {
  platform: MtType;
  brokerId: string;
  server: string;
}

export default function BindAccount({ onDone }: { onDone: () => void }) {
  const [step, setStep] = useState(1);
  const [mtType, setMtType] = useState<MtType>("MT5");
  const [matches, setMatches] = useState<BrokerMatch[]>([]);
  const [keyword, setKeyword] = useState("");
  const [selected, setSelected] = useState<BrokerSelection>({
    platform: "MT5",
    brokerId: "",
    server: "",
  });
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<{ balance?: number; equity?: number; status?: string } | null>(null);

  // Online search broker
  const searchBrokers = async (platform: MtType, kw: string) => {
    setLoading(true);
    setError("");
    try {
      const res = await brokerClient.searchBroker({ platform, keyword: kw });
      setMatches(res.matches ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "搜索失败，请检查网络连接");
    } finally {
      setLoading(false);
    }
  };

  // Auto-search on platform change
  useEffect(() => { searchBrokers(mtType, keyword); }, [mtType]);

  const onSelectPlatform = (p: MtType) => {
    setMtType(p);
    setMatches([]);
    setKeyword("");
  };

  // Filtered display list
  const filtered = keyword
    ? matches.filter((m) => m.company.toLowerCase().includes(keyword.toLowerCase()))
    : matches;

  const onSelectBroker = (m: BrokerMatch, server: string) => {
    setSelected({
      platform: mtType,
      brokerId: "", // will be created on-the-fly
      server,
    });
    setStep(2);
  };

  const onSubmit = async () => {
    setLoading(true);
    setError("");
    try {
      const res = await accountClient.createAccount({
        tenantId: "",
        brokerId: "00000000-0000-0000-0000-000000000000", // placeholder, server is the key
        login,
        password,
        server: selected.server,
        accountType: "demo",
      });
      setResult({
        balance: res.balance,
        equity: res.equity,
        status: res.status,
      });
      setStep(3);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "绑定失败");
      setResult({ status: "error" });
      setStep(3);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="page" style={{ maxWidth: 520 }}>
      <div className="glass-card" style={{ padding: "2rem" }}>
        {/* Progress indicator */}
        <div style={{ display: "flex", gap: "0.5rem", marginBottom: "1.5rem" }}>
          {[1, 2, 3].map((n) => (
            <div
              key={n}
              style={{
                flex: 1,
                height: 4,
                borderRadius: 2,
                background: n <= step ? "var(--color-primary)" : "var(--color-bg-tertiary)",
                transition: "background 0.3s",
              }}
            />
          ))}
        </div>

        <h2 style={{ margin: "0 0 0.5rem", color: "var(--color-text)" }}>
          {step === 1 ? "选择经纪商" : step === 2 ? "输入交易凭据" : result?.status === "error" ? "绑定失败" : "绑定成功"}
        </h2>
        <p style={{ margin: "0 0 1.5rem", color: "var(--color-text-muted)", fontSize: 14 }}>
          {step === 1
            ? "在线搜索全球经纪商服务器"
            : step === 2
            ? `平台: ${mtType}  ·  服务器: ${selected.server}`
            : result?.status === "error"
            ? error || "连接失败"
            : "账户已成功连接"}
        </p>

        {/* Step 1: Select Broker */}
        {step === 1 && (
          <div>
            <div style={{ display: "flex", gap: "0.5rem", marginBottom: "1rem" }}>
              {(["MT4", "MT5"] as MtType[]).map((p) => (
                <button
                  key={p}
                  className={mtType === p ? "btn-primary" : "btn-secondary"}
                  onClick={() => onSelectPlatform(p)}
                  style={{ flex: 1 }}
                >
                  {p}
                </button>
              ))}
            </div>
            <input
              className="input"
              style={{ width: "100%", marginBottom: "0.75rem" }}
              placeholder="输入关键词筛选（如 Robo、IC、XM）..."
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
            />
            {loading ? (
              <p style={{ color: "var(--color-text-muted)", fontSize: 14, padding: "1rem 0" }}>
                正在搜索在线经纪商...
              </p>
            ) : error ? (
              <p style={{ color: "var(--color-danger)", fontSize: 14, padding: "1rem 0" }}>{error}</p>
            ) : (
              <div style={{ maxHeight: 300, overflow: "auto" }} className="scrollbar-thin">
                {filtered.map((m, i) =>
                  m.servers.map((server, j) => (
                    <div
                      key={`${i}-${j}`}
                      onClick={() => onSelectBroker(m, server)}
                      style={{
                        padding: "0.75rem 1rem",
                        cursor: "pointer",
                        borderBottom: "1px solid var(--color-border)",
                        transition: "background 0.2s",
                      }}
                      onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-bg-secondary)")}
                      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                    >
                      <div style={{ fontWeight: 600, color: "var(--color-text)" }}>
                        {m.company}
                      </div>
                      <div style={{ fontSize: 12, color: "var(--color-text-muted)", marginTop: 2 }}>
                        {server}
                      </div>
                    </div>
                  ))
                )}
                {filtered.length === 0 && !loading && (
                  <p style={{ color: "var(--color-text-muted)", fontSize: 14, padding: "0.5rem" }}>
                    {keyword ? "未找到匹配的经纪商" : "未找到经纪商，请尝试切换平台或输入关键词"}
                  </p>
                )}
              </div>
            )}
          </div>
        )}

        {/* Step 2: Enter Credentials */}
        {step === 2 && (
          <div>
            <div
              className="stat-card"
              style={{ marginBottom: "1rem", fontSize: 14, display: "flex", justifyContent: "space-between" }}
            >
              <span style={{ color: "var(--color-text-muted)" }}>平台</span>
              <span style={{ fontWeight: 600 }}>{mtType}</span>
            </div>
            <div
              className="stat-card"
              style={{ marginBottom: "1rem", fontSize: 14, display: "flex", justifyContent: "space-between" }}
            >
              <span style={{ color: "var(--color-text-muted)" }}>服务器</span>
              <span style={{ fontWeight: 600, fontSize: 12 }}>{selected.server}</span>
            </div>
            <input
              className="input"
              style={{ width: "100%", marginBottom: "0.75rem" }}
              placeholder="交易账号 (Login)"
              value={login}
              onChange={(e) => setLogin(e.target.value)}
            />
            <input
              className="input"
              type="text"
              style={{ width: "100%", marginBottom: "1rem" }}
              placeholder="交易密码 (Password)"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <div style={{ display: "flex", gap: "0.5rem" }}>
              <button className="btn-secondary" onClick={() => setStep(1)} style={{ flex: 1 }}>
                返回
              </button>
              <button
                className="btn-primary"
                onClick={onSubmit}
                disabled={!login || !password || loading}
                style={{ flex: 1, opacity: !login || !password ? 0.5 : 1 }}
              >
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
                <span
                  style={{
                    color: result?.status === "connected" ? "var(--color-success)" : "var(--color-danger)",
                    fontWeight: 600,
                  }}
                >
                  {result?.status === "connected" ? "已连接" : "失败"}
                </span>
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
            {result?.status === "connected" ? (
              <button className="btn-primary" onClick={onDone} style={{ width: "100%" }}>
                完成
              </button>
            ) : (
              <div style={{ display: "flex", gap: "0.5rem" }}>
                <button className="btn-secondary" onClick={() => { setStep(1); setError(""); }} style={{ flex: 1 }}>
                  重试
                </button>
                <button className="btn-primary" onClick={onDone} style={{ flex: 1 }}>
                  返回列表
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
