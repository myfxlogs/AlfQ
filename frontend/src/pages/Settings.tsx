// Settings page — ALFQ
import PageHeader from "../components/PageHeader";

export default function Settings() {
  return (
    <div style={{ padding: "2rem" }}>
      <PageHeader title="系统设置" description="平台全局配置" />
      <div style={{ marginTop: 24 }}>
        <SettingsSection title="交易设置">
          <SettingRow label="默认杠杆" value="100:1" />
          <SettingRow label="最大持仓数" value="10" />
          <SettingRow label="止损模式" value="固定点数" />
        </SettingsSection>
        <SettingsSection title="通知设置">
          <SettingRow label="邮件通知" value="已开启" />
          <SettingRow label="站内通知" value="已开启" />
          <SettingRow label="Webhook" value="未配置" />
        </SettingsSection>
      </div>
    </div>
  );
}

function SettingsSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 24 }}>
      <h3 style={{ borderBottom: "1px solid #eee", paddingBottom: 8 }}>{title}</h3>
      <div style={{ paddingLeft: 8 }}>{children}</div>
    </div>
  );
}

function SettingRow({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", padding: "8px 0", borderBottom: "1px solid #f5f5f5" }}>
      <span>{label}</span>
      <span style={{ color: "#666" }}>{value}</span>
    </div>
  );
}
