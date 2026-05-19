// Notifications page — ALFQ
import { useState, useEffect } from "react";
import PageHeader from "../components/PageHeader";
import { apiFetch } from "../api/client";

interface Notification {
  id: string;
  title?: string;
  body: string;
}

export default function Notifications() {
  const [items, setItems] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    apiFetch<{ notifications: Notification[] }>("/alfq.v1.NotifyService/ListNotifications", {
      method: "POST",
      body: JSON.stringify({}),
    })
      .then(res => { setItems(res.notifications ?? []); setLoading(false); })
      .catch((e: unknown) => { setError(e instanceof Error ? e.message : "加载失败"); setLoading(false); });
  }, []);

  return (
    <div style={{ padding: "2rem" }}>
      <PageHeader title="通知中心" description="站内消息与告警" />
      {error && <div style={{ color: "red", marginTop: 12 }}>{error}</div>}
      {loading ? <p>加载中...</p> : (
        <div>
          {items.length === 0 && <p style={{ color: "#888" }}>暂无通知</p>}
          {items.map((n: Notification) => (
            <div key={n.id} style={{ border: "1px solid #eee", borderRadius: 8, padding: 12, marginBottom: 8 }}>
              <strong>{n.title ?? "无标题"}</strong>
              <p style={{ margin: "4px 0 0", color: "#666", fontSize: 14 }}>{n.body}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
