// DataTable — simple table component

interface Props {
  columns: string[];
  rows: string[][];
  emptyText?: string;
}

export default function DataTable({ columns, rows, emptyText = "暂无数据" }: Props) {
  return (
    <div style={{ overflowX: "auto" }}>
      <table style={{ width: "100%", borderCollapse: "collapse", marginTop: 16 }}>
        <thead>
          <tr style={{ textAlign: "left", borderBottom: "2px solid #ddd" }}>
            {columns.map((col, i) => (
              <th key={i} style={{ padding: 8, whiteSpace: "nowrap" }}>{col}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={columns.length} style={{ padding: 16, color: "#888", textAlign: "center" }}>{emptyText}</td>
            </tr>
          ) : (
            rows.map((row, ri) => (
              <tr key={ri} style={{ borderBottom: "1px solid #eee" }}>
                {row.map((cell, ci) => (
                  <td key={ci} style={{ padding: 8, maxWidth: 300, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{cell}</td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
