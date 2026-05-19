// PageHeader — consistent page title component

export default function PageHeader({ title, description }: { title: string; description?: string }) {
  return (
    <div style={{ marginBottom: 16 }}>
      <h1 style={{ margin: 0, fontSize: 24 }}>{title}</h1>
      {description && <p style={{ margin: "4px 0 0", color: "#666", fontSize: 14 }}>{description}</p>}
    </div>
  );
}
