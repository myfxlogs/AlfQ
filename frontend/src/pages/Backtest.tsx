// Backtest page — ALFQ
import { useState, useEffect } from "react";
import PageHeader from "../components/PageHeader";
import DataTable from "../components/DataTable";
import { backtestClient } from "../api/client";
import type { BacktestTask } from "../gen/alfq/v1/strategy_pb";

export default function Backtest() {
  const [tasks, setTasks] = useState<BacktestTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    backtestClient.listBacktests({ strategyId: "" })
      .then(res => { setTasks(res.tasks ?? []); setLoading(false); })
      .catch((e: unknown) => { setError(e instanceof Error ? e.message : "加载失败"); setLoading(false); });
  }, []);

  return (
    <div style={{ padding: "2rem" }}>
      <PageHeader title="回测管理" description="策略回测任务提交与结果查看" />
      {error && <div style={{ color: "red", marginTop: 12 }}>{error}</div>}
      {loading ? <p>加载中...</p> : (
        <DataTable
          columns={["ID", "策略ID", "状态", "结果"]}
          rows={tasks.map((t: BacktestTask) => [t.id, t.strategyId, t.status, t.resultJson ? "有结果" : "—"])}
          emptyText="暂无回测任务"
        />
      )}
    </div>
  );
}
