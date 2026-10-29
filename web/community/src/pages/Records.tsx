import { useState } from "react";
import { Link } from "react-router-dom";
import { send } from "../api";
import { useData } from "../state/useData";
import { Feedback, Require, status } from "../components/ui";
const names: Record<string, string> = {
  notifications: "消息通知",
  reports: "内容举报",
  "comment-reports": "评论举报",
  appeals: "我的申诉",
  "review-decisions": "审核记录",
};
export function Records({ type }: { type: string }) {
  const [page, setPage] = useState(1),
    [message, msg] = useState(""),
    [appealing, setAppealing] = useState(""),
    [reason, setReason] = useState(""),
    [busy, setBusy] = useState(false);
  const { data, error, loading, reload } = useData<{
    items: any[];
    has_more: boolean;
  }>(
    "/" +
      (type === "notifications" ? "notifications" : "me/" + type) +
      "?page=" +
      page,
  );
  return (
    <Require>
      <div className="page-heading">
        <h1>{names[type]}</h1>
        <p>仅本人可见。审核与处置结果在此保留。</p>
      </div>
      <nav className="record-tabs">
        {Object.entries(names).map(([k, v]) => (
          <Link
            className={k === type ? "selected" : ""}
            key={k}
            to={"/" + k}
            onClick={() => {
              setPage(1);
              msg("");
            }}
          >
            {v}
          </Link>
        ))}
      </nav>
      <Feedback error={error} loading={loading} />
      {message && (
        <div className="risk" role="status">
          {message}
        </div>
      )}
      {data?.items.map((item) => (
        <article className="panel" key={item.id}>
          <div className="author-row">
            <span className="tag">
              {status(item.status || (!item.read_at ? "未读" : "已读"))}
            </span>
            <small className="fine">
              {new Date(item.created_at).toLocaleString()}
            </small>
          </div>
          {item.kind && (
            <p className="fine">
              {
                (
                  {
                    post: "图文 / 机会",
                    comment: "评论",
                    profile: "个人资料",
                  } as Record<string, string>
                )[item.kind]
              }
            </p>
          )}
          <p>{item.message || item.reason || "正在等待人工审核"}</p>
          {item.decision && (
            <p>审核结论：{item.decision === "reject" ? "驳回" : "通过"}</p>
          )}
          {item.result && <p>处理结果：{item.result}</p>}
          {item.action && (
            <p className="fine">
              {item.action === "reopen"
                ? "已重新送审，待新任务审核"
                : "维持原决定"}
            </p>
          )}
          {item.post_id && (
            <Link to={"/posts/" + item.post_id}>查看相关内容</Link>
          )}
          {type === "review-decisions" && item.decision === "reject" && (
            <>
              <button
                className="text"
                onClick={() => {
                  setAppealing(item.id);
                  setReason("");
                  msg("");
                }}
              >
                对本次决定提出申诉
              </button>
              {appealing === item.id && (
                <form
                  onSubmit={async (e) => {
                    e.preventDefault();
                    setBusy(true);
                    msg("");
                    try {
                      await send("/appeals", { task_id: item.task_id, reason });
                      setAppealing("");
                      msg("申诉已提交，可在我的申诉查看结果。");
                    } catch (e) {
                      msg((e as Error).message);
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  <label>
                    申诉说明
                    <textarea
                      aria-label="申诉说明"
                      required
                      minLength={5}
                      maxLength={1000}
                      value={reason}
                      onChange={(e) => setReason(e.target.value)}
                      placeholder="请补充事实、说明判断存在的问题"
                    />
                  </label>
                  <div className="toolbar">
                    <button type="button" onClick={() => setAppealing("")}>
                      取消
                    </button>
                    <button className="primary" disabled={busy}>
                      提交申诉
                    </button>
                  </div>
                </form>
              )}
            </>
          )}
          {type === "notifications" && !item.read_at && (
            <button
              onClick={async () => {
                try {
                  await send("/notifications/" + item.id + "/read", {});
                  reload();
                } catch (e) {
                  msg((e as Error).message);
                }
              }}
            >
              标记已读
            </button>
          )}
        </article>
      ))}
      {!loading && !error && !data?.items.length && (
        <div className="empty">暂无记录</div>
      )}
      <div className="toolbar pagination">
        <span>第 {page} 页</span>
        <button disabled={page <= 1} onClick={() => setPage(page - 1)}>
          上一页
        </button>
        <button disabled={!data?.has_more} onClick={() => setPage(page + 1)}>
          下一页
        </button>
      </div>
    </Require>
  );
}
