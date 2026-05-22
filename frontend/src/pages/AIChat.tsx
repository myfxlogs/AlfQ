// AIChat page — ALFQ R10: multi-tenant API keys + usage tracking + RAG knowledge base.
// Layout: gear config panel + chart usage panel + chat area, all on one page.
import { useState, useEffect, useRef, useCallback } from "react";
import { getToken, settingsClient } from "../api/client";

// ── Types ──

interface ChatMessage {
  role: "user" | "assistant";
  text: string;
}

interface UsageStats {
  today_tokens: number;
  month_tokens: number;
  month_cost_cents: number;
  quota_limit_cents: number;
}

type Provider = "openai" | "anthropic" | "deepseek" | "groq" | "ollama" | "custom";

const PROVIDER_DEFAULTS: Record<Provider, { endpoint: string; model: string; label: string }> = {
  openai:    { endpoint: "https://api.openai.com",          model: "gpt-4o",           label: "OpenAI" },
  anthropic: { endpoint: "https://api.anthropic.com",        model: "claude-sonnet-4-20250514", label: "Anthropic" },
  deepseek:  { endpoint: "https://api.deepseek.com",         model: "deepseek-chat",   label: "DeepSeek" },
  groq:      { endpoint: "https://api.groq.com/openai",      model: "llama-3.3-70b",   label: "Groq" },
  ollama:    { endpoint: "http://localhost:11434/v1",        model: "llama3",          label: "Ollama (本地)" },
  custom:    { endpoint: "",                                  model: "",                label: "自定义…" },
};

// ── API helpers ──

const ASSISTANT_URL = (window as any).__ENV__?.VITE_ASSISTANT_URL || "http://localhost:9003";
const API_BASE = (window as any).__ENV__?.VITE_REST_BASE_URL || "/api";

async function fetchUsageStats(): Promise<UsageStats | null> {
  try {
    const resp = await settingsClient.getAIUsageStats({});
    const s = resp.stats;
    return s ? {
      today_tokens: s.todayTokens,
      month_tokens: s.monthTokens,
      month_cost_cents: s.monthCostCents,
      quota_limit_cents: s.quotaLimitCents,
    } : null;
  } catch { return null; }
}

async function testKey(provider: Provider): Promise<{ status: string; latency_ms: number; error?: string }> {
  try {
    const resp = await settingsClient.testAPIKey({ provider });
    return {
      status: resp.status,
      latency_ms: resp.latencyMs,
      error: resp.error || undefined,
    };
  } catch (e: any) {
    return { status: "error", latency_ms: 0, error: e.message };
  }
}

async function chat(message: string, provider: Provider, rag: boolean, endpoint?: string): Promise<{ response: string }> {
  const token = getToken();
  const res = await fetch(`${ASSISTANT_URL}/chat`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ message, provider, rag, endpoint: endpoint || undefined }),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || "请求失败");
  }
  return await res.json();
}

// ── Component ──

export default function AIChat() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  // Config panel
  const [showConfig, setShowConfig] = useState(false);
  const [provider, setProvider] = useState<Provider>("openai");
  const [apiKey, setApiKey] = useState("");
  const [customEndpoint, setCustomEndpoint] = useState("");
  const [model, setModel] = useState("gpt-4o");
  const [ragEnabled, setRagEnabled] = useState(false);
  const [quotaLimit, setQuotaLimit] = useState("5.00");
  const [keyStatus, setKeyStatus] = useState<{ status: string; latency_ms: number } | null>(null);
  const [keyPrefix, setKeyPrefix] = useState(""); // masked key from server

  // Usage panel
  const [showUsage, setShowUsage] = useState(false);
  const [usage, setUsage] = useState<UsageStats | null>(null);

  // Auto-scroll
  const chatEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => { chatEndRef.current?.scrollIntoView({ behavior: "smooth" }); }, [messages]);

  // Load usage on mount and refresh periodically
  useEffect(() => {
    fetchUsageStats().then(setUsage);
    const iv = setInterval(() => fetchUsageStats().then(setUsage), 30000);
    return () => clearInterval(iv);
  }, []);

  // Load saved key info + set defaults on provider change
  useEffect(() => {
    const def = PROVIDER_DEFAULTS[provider];
    if (!customEndpoint) setCustomEndpoint(def.endpoint);
    setModel(def.model || model);
    loadSavedKey(provider).then(info => {
      if (info) {
        setKeyPrefix(info.key_prefix || "");
        if (info.model) setModel(info.model);
        if (info.endpoint) setCustomEndpoint(info.endpoint);
        setQuotaLimit(String((info.quota || 500) / 100));
      }
    });
  }, [provider]);

  // ── Handlers ──

  const send = useCallback(async () => {
    if (!input.trim() || loading) return;
    const msg = input.trim();
    setMessages(prev => [...prev, { role: "user", text: msg }]);
    setInput("");
    setLoading(true);
    setError("");
    try {
      const data = await chat(msg, provider, ragEnabled, customEndpoint);
      setMessages(prev => [...prev, { role: "assistant", text: data.response || "..." }]);
      // Refresh usage after chat
      fetchUsageStats().then(setUsage);
    } catch (e: any) {
      setError(e.message || "请求失败");
      setMessages(prev => [...prev, { role: "assistant", text: "❌ " + (e.message || "请求失败") }]);
    } finally {
      setLoading(false);
    }
  }, [input, loading, provider, ragEnabled, customEndpoint]);

  const saveKey = useCallback(async () => {
    if (!apiKey.trim()) return;
    try {
      await settingsClient.updateSystemSetting({
        key: `provider:${provider}.key`,
        value: apiKey.trim(),
      });

      // Update key prefix
      const masked = apiKey.length > 12
        ? apiKey.slice(0, 7) + "..." + "****" + apiKey.slice(-4)
        : "****";
      setKeyPrefix(masked);
      setApiKey("");
      setError("");

      // Also save model, endpoint, and quota
      await settingsClient.updateSystemSetting({ key: `provider:${provider}.model`, value: model });
      await settingsClient.updateSystemSetting({ key: `provider:${provider}.quota`, value: String(Math.round(parseFloat(quotaLimit) * 100)) });
      if (customEndpoint) {
        await settingsClient.updateSystemSetting({ key: `provider:${provider}.endpoint`, value: customEndpoint });
      }
    } catch (e: any) {
      setError(e.message || "保存 API Key 失败");
    }
  }, [apiKey, provider, model, quotaLimit]);

  const handleTestKey = useCallback(async () => {
    setKeyStatus(null);
    const result = await testKey(provider);
    setKeyStatus({ status: result.status, latency_ms: result.latency_ms });
  }, [provider]);

  const budgetPct = usage && usage.quota_limit_cents > 0
    ? Math.min(100, Math.round((usage.month_cost_cents / usage.quota_limit_cents) * 100))
    : 0;

  // ── Render ──

  return (
    <div style={styles.wrapper}>
      {/* Header */}
      <div style={styles.header}>
        <h1 style={styles.title}>AI 助手</h1>
        <div style={styles.headerBtns}>
          <button
            style={{ ...styles.iconBtn, background: showConfig ? "#e6f7ff" : "transparent" }}
            onClick={() => { setShowConfig(!showConfig); setShowUsage(false); }}
            title="配置"
          >⚙️</button>
          <button
            style={{ ...styles.iconBtn, background: showUsage ? "#e6f7ff" : "transparent" }}
            onClick={() => { setShowUsage(!showUsage); setShowConfig(false); }}
            title="用量统计"
          >📊</button>
        </div>
      </div>

      {/* Config Panel */}
      {showConfig && (
        <div style={styles.panel}>
          <div style={styles.row}>
            <label style={styles.label}>服务商</label>
            <select value={provider} onChange={e => setProvider(e.target.value as Provider)} style={styles.select}>
              {Object.entries(PROVIDER_DEFAULTS).map(([k, v]) => (
                <option key={k} value={k}>{v.label}</option>
              ))}
            </select>
          </div>
          {provider === "custom" && (
            <div style={styles.row}>
              <label style={styles.label}>端点 URL</label>
              <input
                value={customEndpoint}
                onChange={e => setCustomEndpoint(e.target.value)}
                placeholder="https://your-llm-api.com/v1"
                style={styles.input}
              />
            </div>
          )}
          <div style={styles.row}>
            <label style={styles.label}>API Key</label>
            <input
              value={apiKey}
              onChange={e => setApiKey(e.target.value)}
              placeholder={keyPrefix || "sk-..."}
              style={styles.input}
              type="password"
            />
            <button onClick={saveKey} style={styles.smallBtn}>保存</button>
          </div>
          <div style={styles.row}>
            <label style={styles.label}>模型</label>
            <input value={model} onChange={e => setModel(e.target.value)} style={{ ...styles.input, flex: 1 }} placeholder="gpt-4o" />
          </div>
          <div style={styles.row}>
            <label style={styles.label}>月预算($)</label>
            <input value={quotaLimit} onChange={e => setQuotaLimit(e.target.value)} style={{ ...styles.inputSmall }} type="number" min="0" step="0.01" />
          </div>
          <div style={styles.row}>
            <label style={styles.label}>状态</label>
            <span style={{ fontSize: 13 }}>
              {keyStatus
                ? <span style={{ color: keyStatus.status === "connected" ? "#52c41a" : "#ff4d4f" }}>
                    {keyStatus.status === "connected" ? "● 已连接" : "● " + keyStatus.status}
                    {" "}{keyStatus.latency_ms}ms
                  </span>
                : keyPrefix ? "● 已配置" : "○ 未配置"}
            </span>
            <button onClick={handleTestKey} style={{ ...styles.smallBtn, marginLeft: 8 }}>测试</button>
          </div>
          <div style={styles.row}>
            <label style={styles.label}>RAG 知识库</label>
            <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 13, cursor: "pointer" }}>
              <input type="checkbox" checked={ragEnabled} onChange={e => setRagEnabled(e.target.checked)} />
              使用文档库增强回复
            </label>
          </div>
        </div>
      )}

      {/* Usage Panel */}
      {showUsage && (
        <div style={styles.panel}>
          <div style={styles.row}>
            <span style={styles.label}>今日</span>
            <span style={styles.val}>{(usage?.today_tokens ?? 0).toLocaleString()} tokens</span>
          </div>
          <div style={styles.row}>
            <span style={styles.label}>本月</span>
            <span style={styles.val}>{(usage?.month_tokens ?? 0).toLocaleString()} tokens</span>
          </div>
          <div style={styles.row}>
            <span style={styles.label}>预算</span>
            <span style={styles.val}>
              ${((usage?.month_cost_cents ?? 0) / 100).toFixed(2)} / ${((usage?.quota_limit_cents ?? 500) / 100).toFixed(2)}
              {" "}({budgetPct}%)
            </span>
          </div>
          <div style={styles.barOuter}>
            <div style={{ ...styles.barInner, width: `${budgetPct}%`, background: budgetPct > 80 ? "#ff4d4f" : budgetPct > 50 ? "#faad14" : "#52c41a" }} />
          </div>
        </div>
      )}

      {/* Error banner */}
      {error && (
        <div style={styles.errorBanner}>
          {error}
          <button onClick={() => setError("")} style={styles.errorClose}>×</button>
        </div>
      )}

      {/* Chat area */}
      <div style={styles.chatArea}>
        {messages.length === 0 && (
          <div style={styles.welcome}>
            <div style={styles.welcomeIcon}>🤖</div>
            <p>你好，我是 ALFQ 策略助手。我可以：</p>
            <ul style={styles.welcomeList}>
              <li>帮您编写策略因子 DSL</li>
              <li>查询账户状态和持仓</li>
              <li>解释技术指标和回测结果</li>
            </ul>
          </div>
        )}
        {messages.map((m, i) => (
          <div key={i} style={{ ...styles.msgRow, justifyContent: m.role === "user" ? "flex-end" : "flex-start" }}>
            <div style={{
              ...styles.msgBubble,
              background: m.role === "user" ? "#1677ff" : "#fff",
              color: m.role === "user" ? "#fff" : "#333",
              alignSelf: m.role === "user" ? "flex-end" : "flex-start",
            }}>
              {m.text}
            </div>
          </div>
        ))}
        {loading && <div style={styles.thinking}>AI 思考中...</div>}
        <div ref={chatEndRef} />
      </div>

      {/* Input */}
      <div style={styles.inputRow}>
        <input
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={e => e.key === "Enter" && send()}
          style={styles.chatInput}
          placeholder="输入消息..."
          disabled={loading}
        />
        <button onClick={send} disabled={loading || !input.trim()} style={styles.sendBtn}>
          发送
        </button>
      </div>
    </div>
  );
}

// ── Helpers ──

async function loadSavedKey(provider: Provider): Promise<{ key_prefix?: string; model?: string; endpoint?: string; quota?: number } | null> {
  try {
    const resp = await settingsClient.getSystemSettings({});
    const settings = resp.settings || [];
    const info: any = {};
    for (const s of settings) {
      if (s.key === `provider:${provider}.key_prefix`) info.key_prefix = s.value;
      if (s.key === `provider:${provider}.model`) info.model = s.value;
      if (s.key === `provider:${provider}.endpoint`) info.endpoint = s.value;
      if (s.key === `provider:${provider}.quota`) info.quota = parseInt(s.value) || 500;
    }
    return Object.keys(info).length > 0 ? info : null;
  } catch { return null; }
}

// ── Inline Styles ──

const styles: Record<string, React.CSSProperties> = {
  wrapper: { padding: "1.5rem", maxWidth: 800, margin: "0 auto", fontFamily: "system-ui, sans-serif" },
  header: { display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 },
  title: { margin: 0, fontSize: 20, fontWeight: 600 },
  headerBtns: { display: "flex", gap: 4 },
  iconBtn: { border: "1px solid #d9d9d9", borderRadius: 6, padding: "4px 12px", cursor: "pointer", fontSize: 18, background: "transparent" },
  panel: { border: "1px solid #e8e8e8", borderRadius: 8, padding: "1rem", marginBottom: 12, background: "#fafafa" },
  row: { display: "flex", alignItems: "center", marginBottom: 8, gap: 8 },
  label: { width: 70, fontSize: 13, color: "#666", flexShrink: 0 },
  select: { flex: 1, padding: "6px 8px", border: "1px solid #d9d9d9", borderRadius: 4, fontSize: 13 },
  input: { flex: 1, padding: "6px 8px", border: "1px solid #d9d9d9", borderRadius: 4, fontSize: 13 },
  inputSmall: { width: 100, padding: "6px 8px", border: "1px solid #d9d9d9", borderRadius: 4, fontSize: 13 },
  smallBtn: { padding: "5px 14px", border: "1px solid #1677ff", borderRadius: 4, background: "#fff", color: "#1677ff", cursor: "pointer", fontSize: 12 },
  val: { fontSize: 13, fontWeight: 500 },
  barOuter: { height: 8, borderRadius: 4, background: "#f0f0f0", marginTop: 6, overflow: "hidden" },
  barInner: { height: "100%", borderRadius: 4, transition: "width 0.3s" },
  errorBanner: { background: "#fff2f0", border: "1px solid #ffccc7", borderRadius: 6, padding: "8px 12px", marginBottom: 12, fontSize: 13, color: "#ff4d4f", display: "flex", justifyContent: "space-between" },
  errorClose: { background: "none", border: "none", cursor: "pointer", fontSize: 16, color: "#ff4d4f" },
  chatArea: { border: "1px solid #e8e8e8", borderRadius: 8, padding: "1rem", minHeight: 320, maxHeight: 480, overflowY: "auto", background: "#fafafa", marginBottom: 12 },
  welcome: { textAlign: "center", color: "#999", padding: "2rem 0" },
  welcomeIcon: { fontSize: 40, marginBottom: 8 },
  welcomeList: { textAlign: "left", display: "inline-block", margin: "0 auto", padding: 0, listStyle: "none", fontSize: 13 },
  msgRow: { display: "flex", marginBottom: 10 },
  msgBubble: { display: "inline-block", padding: "10px 14px", borderRadius: 14, maxWidth: "80%", fontSize: 14, lineHeight: 1.5, boxShadow: "0 1px 3px rgba(0,0,0,0.08)", whiteSpace: "pre-wrap", wordBreak: "break-word" },
  thinking: { color: "#bbb", fontSize: 13, fontStyle: "italic", paddingLeft: 12 },
  inputRow: { display: "flex", gap: 8 },
  chatInput: { flex: 1, padding: 12, border: "1px solid #d9d9d9", borderRadius: 8, fontSize: 14, outline: "none" },
  sendBtn: { padding: "0 28px", background: "#1677ff", color: "#fff", border: "none", borderRadius: 8, cursor: "pointer", fontSize: 14, fontWeight: 500 },
};
