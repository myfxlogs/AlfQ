// Notifications page — stub (no Connect RPC service yet)
// TODO: add NotifyService to proto and implement backend handler
import PageHeader from "../components/PageHeader";

export default function Notifications() {
  return (
    <div style={{ padding: "2rem" }}>
      <PageHeader title="通知" description="系统通知与告警" />
      <p style={{ color: "var(--color-text-muted)", marginTop: "2rem", textAlign: "center" }}>
        通知功能即将上线
      </p>
    </div>
  );
}
