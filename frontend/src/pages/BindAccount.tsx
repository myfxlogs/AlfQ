// BindAccount — 三步交易账户绑定向导
import { useState } from "react";
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
  const [brokers, setBrokers] = useState<Broker[]>([]);
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

  // Step 1: Load brokers
  const loadBrokers = async (platform: MtType) => {
    setLoading(true);
    try {
      const res = await brokerClient.listBrokers({ tenantId: "" });
      setBrokers(
        (res.brokers ?? []).filter((b: Broker) => !platform || b.platform?.toLowerCase() === platform.toLowerCase())
      );
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  };

  const searchBrokers = async (platform: MtType, kw: string) => {
    if (!kw) {
      setMatches([]);
      return;
    }
    try {
      const res = await brokerClient.searchBroker({ platform, keyword: kw });
      setMatches(res.matches ?? []);
    } catch {
      // search not available, filter locally
    }
  };

  const onSelectPlatform = (p: MtType) => {
    setMtType(p);
    setSelected({ ...selected, platform: p });
    loadBrokers(p);
  };

  const onSelectBroker = (b: Broker) => {
    setSelected({
      platform: mtType,
      brokerId: b.id ?? "",
      server: b.mtapiEndpoint ?? "",
    });
    setStep(2);
  };

  const onSubmit = async () => {
    setLoading(true);
    setError("");
    try {
      const res = await accountClient.createAccount({
        tenantId: "",
        brokerId: selected.brokerId,
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
    <div className="glass-card" style={{ padding: "2rem", maxWidth: 480, margin: "2rem auto" }}>
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
        {step === 1 ? "选择交易平台和经纪商" : step === 2 ? "输入 MT 交易账号和密码" : result?.status === "error" ? error || "连接失败" : "账户已成功连接，账户信息已更新"}
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
            placeholder="搜索经纪商名称..."
            value={keyword}
            onChange={(e) => {
              setKeyword(e.target.value);
              searchBrokers(mtType, e.target.value);
            }}
          />
          {loading ? (
            <p style={{ color: "var(--color-text-muted)", fontSize: 14 }}>加载中...</p>
          ) : (
            <div style={{ maxHeight: 240, overflow: "auto" }} className="scrollbar-thin">
              {(keyword ? matches.map((m) => ({ id: "", name: m.company, mtapiEndpoint: m.servers?.[0] ?? "" })) : brokers).map(
                (b: Broker | { id: string; name: string; mtapiEndpoint: string }, i: number) => (
                  <div
                    key={i}
                    onClick={() => onSelectBroker({ id: b.id, mtapiEndpoint: b.mtapiEndpoint } as Broker)}
                    style={{
                      padding: "0.75rem 1rem",
                      cursor: "pointer",
                      borderBottom: "1px solid var(--color-border)",
                      transition: "background 0.2s",
                    }}
                    onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-bg-secondary)")}
                    onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                  >
                    <div style={{ fontWeight: 600, color: "var(--color-text)" }}>{b.name}</div>
                    <div style={{ fontSize: 12, color: "var(--color-text-muted)", marginTop: 2 }}>
                      {b.mtapiEndpoint}
                    </div>
                  </div>
                )
              )}
              {keyword && matches.length === 0 && brokers.length === 0 && (
                <p style={{ color: "var(--color-text-muted)", fontSize: 14, padding: "0.5rem" }}>
                  未找到匹配的经纪商
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
  );
}
