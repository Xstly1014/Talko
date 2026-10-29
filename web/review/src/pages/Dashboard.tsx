import { Link } from "react-router-dom";
import { RefreshCw, ArrowRight } from "lucide-react";
import { useData } from "../state";
import { Feedback, Heading } from "../components/ui";
export function Dashboard() {
  const { data, error, loading, reload } =
    useData<Record<string, number>>("/summary");
  return (
    <>
      <Heading
        title="审核工作台"
        sub="从待处理任务开始，保留每一次决策的上下文。"
      >
        <button onClick={reload}>
          <RefreshCw size={15} />
          刷新
        </button>
      </Heading>
      <Feedback error={error} loading={loading} />
      <div className="metrics">
        {[
          ["pending", "待审核内容", "进入任务队列"],
          ["decided", "累计审核完成", "包含通过与驳回"],
          ["reports", "待处理举报", "图文与评论举报"],
          ["appeals", "待复核申诉", "独立复核原决定"],
        ].map(([k, t, s]) => (
          <div className="metric" key={k}>
            <span>{t}</span>
            <strong>{data?.[k] ?? "—"}</strong>
            <small>{s}</small>
          </div>
        ))}
      </div>
      <div className="two">
        <section className="panel">
          <div className="panel-heading">
            <h2>当前审核流程</h2>
            <span className="badge">人工审核</span>
          </div>
          <div className="steps">
            {[
              ["1", "领取任务", "15 分钟领取保护"],
              ["2", "核查上下文", "查看具体内容版本"],
              ["3", "记录结论", "原因与人员可追溯"],
            ].map(([n, t, s]) => (
              <div key={n}>
                <b>{n}</b>
                <h3>{t}</h3>
                <p>{s}</p>
              </div>
            ))}
          </div>
          <Link className="button primary" to="/tasks">
            开始处理任务 <ArrowRight size={16} />
          </Link>
        </section>
        <section className="panel">
          <h2>处理原则</h2>
          <ul>
            <li>模型尚未接入，内容不会自动公开。</li>
            <li>兼职、家教需核查收费、主体与有效期。</li>
            <li>不能审核自己的内容，申诉由其他人员复核。</li>
            <li>风险提示不等于平台担保。</li>
          </ul>
        </section>
      </div>
    </>
  );
}
