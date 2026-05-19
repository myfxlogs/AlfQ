// LoadingSpinner — simple loading indicator

export default function LoadingSpinner({ text = "加载中..." }: { text?: string }) {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", padding: "2rem", gap: 8 }}>
      <svg width="20" height="20" viewBox="0 0 20 20" style={{ animation: "spin 1s linear infinite" }}>
        <circle cx="10" cy="10" r="8" stroke="#ccc" strokeWidth="3" fill="none" strokeDasharray="40" strokeLinecap="round" />
      </svg>
      <span style={{ color: "#666" }}>{text}</span>
      <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
    </div>
  );
}
