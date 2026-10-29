import React, { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { RefreshCw } from "lucide-react";
import { post } from "../api";
import { useData } from "../state";
import {
  Feedback,
  Heading,
  Pager,
  label,
  time,
  type Page,
} from "../components/ui";
function ReportEvidence({
  record,
  onReady,
}: {
  record: any;
  onReady: (b: boolean) => void;
}) {
  const {
    data: d,
    error,
    loading,
  } = useData<any>("/reports/" + record.id + "?kind=" + record.kind);
  useEffect(() => {
    onReady(!!d && !error && !loading);
  }, [d, error, loading, onReady]);
  return (
    <>
      <Feedback error={error} loading={loading} />
      {d && (
        <section className="evidence">
          <div className="panel-heading">
            <h3>被举报内容证据</h3>
            <span className="badge">当前状态：{label(d.current_status)}</span>
          </div>
          <p className="muted">作者：{d.author}</p>
          {record.kind === "post" && !d.evidence_revision_id ? (
            <div className="warning">
              历史举报未留存版本证据。请先核查原始材料，不能仅凭举报说明处置。
            </div>
          ) : (
            <>
              <h3>{d.title || d.parent_title || "评论内容"}</h3>
              <div className="text-body">{d.body || d.comment_body}</div>
              <div className="images">
                {(d.media_ids || []).map((mid: string) => (
                  <a
                    key={mid}
                    href={"/review-api/v1/media/" + mid}
                    target="_blank"
                    rel="noreferrer"
                  >
                    <img
                      src={"/review-api/v1/media/" + mid}
                      alt="举报证据图片"
                    />
                  </a>
                ))}
              </div>
              {d.opportunity && (
                <dl>
                  {Object.entries(d.opportunity).map(([k, v]) => (
                    <React.Fragment key={k}>
                      <dt>
                        {(
                          {
                            publisher: "发布方",
                            city: "地点",
                            compensation: "报酬",
                            settlement: "结算方式",
                            expires_at: "有效期",
                            contact: "联系渠道",
                          } as Record<string, string>
                        )[k] || k}
                      </dt>
                      <dd>{String(v)}</dd>
                    </React.Fragment>
                  ))}
                </dl>
              )}
              {d.evidence_revision_id && (
                <p className="fine">证据版本 {d.evidence_revision_id}</p>
              )}
              {d.evidence_revision_id &&
                d.current_revision_id !== d.evidence_revision_id && (
                  <p className="warning">
                    当前内容版本已变化；以上为举报发生时的版本。
                  </p>
                )}
              {d.parent_body && (
                <details>
                  <summary>查看评论关联内容</summary>
                  <p className="text-body">{d.parent_body}</p>
                </details>
              )}
            </>
          )}
          {d.resolved_by && (
            <p className="fine">
              处理人：{d.resolved_by} ·{" "}
              {d.action === "hide" ? "下架" : "未处置"}
            </p>
          )}
        </section>
      )}
    </>
  );
}
export function Cases({ type }: { type: "reports" | "appeals" }) {
  const [page, setPage] = useState(1);
  const { data, error, loading, reload } = useData<Page>(
    "/" + type + "?page=" + page,
  );
  const [selected, select] = useState<any>(null),
    [reason, setReason] = useState(""),
    [action, setAction] = useState("dismiss"),
    [message, msg] = useState(""),
    [busy, setBusy] = useState(false),
    [evidenceReady, setEvidenceReady] = useState(false);
  useEffect(() => {
    select(null);
    setPage(1);
    setAction(type === "reports" ? "dismiss" : "uphold");
    msg("");
  }, [type]);
  return (
    <>
      <Heading
        title={type === "reports" ? "举报处理" : "申诉复核"}
        sub={
          type === "reports"
            ? "核查内容证据与举报理由，处置结果进入通知。"
            : "由独立人员复核；重开只重新送审，不直接发布。"
        }
      />
      <div className="two cases">
        <section className="panel">
          <div className="panel-heading">
            <h2>处理队列</h2>
            <button aria-label="刷新处理队列" onClick={reload}>
              <RefreshCw size={14} />
            </button>
          </div>
          <Feedback error={error} loading={loading} />
          {data?.items.map((c) => (
            <button
              className={"case " + (selected?.id === c.id ? "selected" : "")}
              key={c.id}
              onClick={() => {
                select(c);
                msg("");
                setReason("");
                setEvidenceReady(false);
                setAction(type === "reports" ? "dismiss" : "uphold");
              }}
            >
              <span className="badge">{label(c.status)}</span>
              <b>{c.reason}</b>
              <small>
                {c.kind ? label(c.kind) : "申诉"} · {time(c.created_at)}
              </small>
            </button>
          ))}
          {!loading && !data?.items.length && (
            <div className="empty">暂无记录</div>
          )}
          <Pager data={data} page={page} setPage={setPage} />
        </section>
        <section className="panel">
          {selected ? (
            <>
              <h2>处理详情</h2>
              <p className="fine">{selected.id}</p>
              <p className="text-body">{selected.reason}</p>
              {type === "reports" ? (
                <ReportEvidence
                  key={selected.id}
                  record={selected}
                  onReady={setEvidenceReady}
                />
              ) : (
                <>
                  <p>原结论：{selected.original_reason}</p>
                  <Link to={"/tasks/" + selected.task_id}>
                    查看原审核上下文 →
                  </Link>
                  {selected.new_task_id && (
                    <p>
                      <Link to={"/tasks/" + selected.new_task_id}>
                        查看重新送审任务 →
                      </Link>
                    </p>
                  )}
                </>
              )}
              {selected.result && (
                <div className="note">已处理：{selected.result}</div>
              )}
              {selected.status === "pending" && (
                <form
                  onSubmit={async (e) => {
                    e.preventDefault();
                    setBusy(true);
                    msg("");
                    try {
                      await post(
                        "/" + type + "/" + selected.id + "/resolve",
                        type === "reports"
                          ? { kind: selected.kind, action, reason }
                          : { action, reason },
                      );
                      msg("处理结果已保存并进入通知链路。");
                      select(null);
                      reload();
                    } catch (e) {
                      msg((e as Error).message);
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  <label>
                    处理动作
                    <select
                      value={action}
                      onChange={(e) => setAction(e.target.value)}
                    >
                      {type === "reports" ? (
                        <>
                          <option value="dismiss">核查后不予处置</option>
                          <option value="hide">下架对应内容 / 评论</option>
                        </>
                      ) : (
                        <>
                          <option value="uphold">维持原审核决定</option>
                          <option value="reopen">重新送审有效版本</option>
                        </>
                      )}
                    </select>
                  </label>
                  <label>
                    处理依据
                    <textarea
                      required
                      minLength={2}
                      maxLength={400}
                      rows={6}
                      value={reason}
                      onChange={(e) => setReason(e.target.value)}
                    />
                  </label>
                  <button
                    className="primary"
                    disabled={busy || (type === "reports" && !evidenceReady)}
                  >
                    {busy ? "处理中…" : "确认并记录处理结果"}
                  </button>
                </form>
              )}
            </>
          ) : (
            <div className="empty">从左侧选择一条记录查看处理。</div>
          )}
          {message && (
            <div className="note" role="status">
              {message}
            </div>
          )}
        </section>
      </div>
    </>
  );
}
