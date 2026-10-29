import { useState } from "react";
import { useData } from "../state";
import { Feedback, Heading, Pager, time, type Page } from "../components/ui";
export function Audit() {
  const [page, setPage] = useState(1);
  const { data, error, loading } = useData<Page>("/audit?page=" + page);
  return (
    <>
      <Heading
        title="操作审计"
        sub="仅管理员可访问；内容审核与处置原因保留记录。"
      />
      <section className="panel">
        <Feedback error={error} loading={loading} />
        <div className="table">
          <table>
            <thead>
              <tr>
                <th>操作时间</th>
                <th>操作人</th>
                <th>动作</th>
                <th>对象</th>
                <th>依据</th>
              </tr>
            </thead>
            <tbody>
              {data?.items.map((a) => (
                <tr key={a.id}>
                  <td>{time(a.created_at)}</td>
                  <td>{a.actor}</td>
                  <td>
                    {(
                      {
                        review_decision: "内容审核",
                        profile_decision: "资料审核",
                        resolve_report: "内容举报处置",
                        comment_report: "评论举报处置",
                        resolve_appeal: "申诉复核",
                        cancel_stale_task: "历史任务校正",
                      } as Record<string, string>
                    )[a.action] || a.action}
                  </td>
                  <td>
                    <small>{a.target_id}</small>
                  </td>
                  <td>{a.reason}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <Pager data={data} page={page} setPage={setPage} />
      </section>
    </>
  );
}
