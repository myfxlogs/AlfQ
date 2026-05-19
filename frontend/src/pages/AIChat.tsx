// AIChat page — ALFQ
import { useState } from "react";

interface ChatMessage {
  role: string;
  text: string;
}

interface WindowEnv {
  VITE_REST_BASE_URL?: string;
}

declare global {
  interface Window {
    __ENV__?: WindowEnv;
  }
}

export default function AIChat() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);

  async function send() {
    if (!input.trim() || loading) return;
    const userMsg = input.trim();
    setMessages(prev => [...prev, { role: "user", text: userMsg }]);
    setInput("");
    setLoading(true);
    try {
      const base = window.__ENV__?.VITE_REST_BASE_URL || "http://localhost:9006";
      const res = await fetch(`${base}/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: new URLSearchParams({ message: userMsg }),
      });
      const data = await res.json() as { response?: string };
      setMessages(prev => [...prev, { role: "assistant", text: data.response || "..." }]);
    } catch (e: unknown) {
      setMessages(prev => [...prev, { role: "assistant", text: "Error: " + (e instanceof Error ? e.message : "请求失败") }]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{ padding: "2rem", maxWidth: 800, margin: "0 auto" }}>
      <h1>AI 助手</h1>
      <div style={{ border: "1px solid #ddd", borderRadius: 8, padding: "1rem", minHeight: 300, marginTop: 16, background: "#fafafa" }}>
        {messages.map((m, i) => (
          <div key={i} style={{ marginBottom: 12, textAlign: m.role === "user" ? "right" : "left" }}>
            <span style={{ display: "inline-block", padding: "8px 12px", borderRadius: 12, background: m.role === "user" ? "#1677ff" : "#fff", color: m.role === "user" ? "#fff" : "#333", boxShadow: "0 1px 3px rgba(0,0,0,0.1)" }}>
              {m.text}
            </span>
          </div>
        ))}
        {loading && <div style={{ color: "#888" }}>AI 思考中...</div>}
      </div>
      <div style={{ display: "flex", gap: 8, marginTop: 16 }}>
        <input value={input} onChange={e => setInput(e.target.value)} onKeyDown={e => e.key === "Enter" && send()}
          style={{ flex: 1, padding: 12, border: "1px solid #ddd", borderRadius: 6 }} placeholder="输入消息..." />
        <button onClick={send} disabled={loading} style={{ padding: "0 24px", background: "#1677ff", color: "#fff", border: "none", borderRadius: 6, cursor: "pointer" }}>
          发送
        </button>
      </div>
    </div>
  );
}
