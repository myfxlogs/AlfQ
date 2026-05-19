// AI Assistant page — ALFQ
import PageHeader from "../components/PageHeader";

export default function AIAssistant() {
  return (
    <div style={{ padding: "2rem" }}>
      <PageHeader title="AI 策略助手" description="基于 LLM 的策略分析、因子解释与风险评估" />
      <div style={{ marginTop: 24, display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))", gap: 16 }}>
        <AssistantCard title="因子解释" desc="输入因子表达式，AI 解析其含义与计算逻辑" href="/assistant?tool=explain_factor" />
        <AssistantCard title="策略建议" desc="描述交易思路，AI 推荐因子组合与参数" href="/assistant?tool=suggest_strategy" />
        <AssistantCard title="风险分析" desc="分析当前持仓风险敞口与潜在回撤" href="/assistant?tool=analyze_risk" />
        <AssistantCard title="因子列表" desc="浏览所有已注册因子及其描述" href="/assistant?tool=list_factors" />
      </div>
    </div>
  );
}

function AssistantCard({ title, desc, href }: { title: string; desc: string; href: string }) {
  return (
    <a href={href} style={{ border: "1px solid #d9d9d9", borderRadius: 8, padding: 20, textDecoration: "none", color: "inherit", background: "#f6ffed" }}>
      <h3 style={{ margin: 0, color: "#389e0d" }}>{title}</h3>
      <p style={{ margin: "8px 0 0", color: "#666", fontSize: 14 }}>{desc}</p>
    </a>
  );
}
