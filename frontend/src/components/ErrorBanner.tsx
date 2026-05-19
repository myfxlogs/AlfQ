// ErrorBanner — error display component

export default function ErrorBanner({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div style={{ background: "#fff2f0", border: "1px solid #ffccc7", borderRadius: 8, padding: "12px 16px", marginTop: 12, display: "flex", alignItems: "center", gap: 12 }}>
      <span style={{ color: "#ff4d4f", flex: 1 }}>{message}</span>
      {onRetry && (
        <button onClick={onRetry} style={{ background: "#ff4d4f", color: "#fff", border: "none", borderRadius: 4, padding: "4px 12px", cursor: "pointer" }}>
          重试
        </button>
      )}
    </div>
  );
}
