import React, { useState, useContext } from "react";
import { Link, useParams } from "react-router-dom";
import { Lock, Clock } from "lucide-react";
import { post } from "../api";
import { StaffContext, useData } from "../state";
import { Feedback, Heading, label, time } from "../components/ui";
export function TaskDetail() {
  const { id } = useParams(),
    { staff } = useContext(StaffContext);
  const { data: t, error, loading, reload } = useData<any>("/tasks/" + id);
  const [reason, setReason] = useState(""),
    [advertising, setAd] = useState(false),
    [feedback, msg] = useState(""),
    [busy, setBusy] = useState(false);
  async function action(decision?: string) {
    setBusy(true);
    msg("");
    try {
      if (!decision) {
        await post("/tasks/" + id + "/claim");
        msg("任务已领取，15 分钟内有效。");
        reload();
      } else {
        await post("/tasks/" + id + "/decision", {
          decision,
          reason,
          advertising,
        });
        msg("审核结论已保存，业务状态与通知将按规则更新。");
        reload();
      }
    } catch (e) {
      msg((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  if (!t) return <Feedback error={error} loading={loading} />;
  const claimed =
    t.assigned_to === staff?.id && new Date(t.claim_expires_at) > new Date();
  return (
    <>
      <Heading
        title="审核任务详情"
        sub={"任务 " + t.id + " · " + label(t.kind)}
      >
        <Link className="button" to="/tasks">
          返回任务列表
        </Link>
      </Heading>
      <div className="detail-grid">
        <article className="panel">
          <div className="panel-heading">
            <span className="badge">{label(t.status)}</span>
            <span className="muted">{time(t.created_at)}</span>
          </div>
          <h2>{t.title || t.profile_nickname || "评论内容"}</h2>
          <p className="muted">
            作者：{t.author} · 对象作者 ID：{t.author_id}
          </p>
          <div className="text-body">
            {t.body || t.comment_body || t.profile_bio || "无正文"}
          </div>
          <div className="images">
            {(t.media_ids || []).map((mid: string) => (
              <a
                href={"/review-api/v1/media/" + mid}
                target="_blank"
                rel="noreferrer"
                key={mid}
              >
                <img src={"/review-api/v1/media/" + mid} alt="待审核图片" />
              </a>
            ))}
            {t.profile_avatar && (
              <img
                src={"/review-api/v1/media/" + t.profile_avatar}
                alt="待审核头像"
              />
            )}
          </div>
          {t.opportunity && (
            <section className="opportunity">
              <h3>机会信息核验</h3>
              <dl>
                {Object.entries(t.opportunity).map(([k, v]) => (
                  <React.Fragment key={k}>
                    <dt>
                      {{
                        publisher: "发布方",
                        city: "城市 / 地点",
                        compensation: "报酬",
                        settlement: "结算方式",
                        expires_at: "有效期",
                        contact: "联系渠道",
                      }[k] || k}
                    </dt>
                    <dd>{String(v)}</dd>
                  </React.Fragment>
                ))}
              </dl>
              <div className="warning">
                检查主体描述、工作内容、费用、押金和误导承诺。审核通过不代表信息来源已核实。
              </div>
            </section>
          )}
          {t.kind === "profile" && (
            <div className="note">
              通过后才替换当前公开资料；新资料版本提交后，本任务可能被取消。
            </div>
          )}
          {t.kind === "comment" && (
            <section className="opportunity">
              <h3>评论关联内容</h3>
              <h4>{t.parent_title || "原内容当前不可用"}</h4>
              <p className="text-body">{t.parent_body}</p>
              <div className="images">
                {(t.parent_media_ids || []).map((mid: string) => (
                  <img
                    key={mid}
                    src={"/review-api/v1/media/" + mid}
                    alt="关联内容图片"
                  />
                ))}
              </div>
            </section>
          )}
          <div className="audit-snapshot">
            <h3>已有结论</h3>
            <p>
              {t.decision
                ? label(t.decision) + " · " + t.reason
                : "尚无审核结论"}
            </p>
            {t.reviewer && (
              <p className="muted">
                {t.reviewer} · {time(t.decided_at)}
              </p>
            )}
          </div>
        </article>
        <aside className="panel decision">
          <h2>审核操作</h2>
          <p className="muted">先领取，再核查和提交结论。</p>
          {t.status === "pending" ? (
            <>
              <button
                className="full"
                disabled={busy}
                onClick={() => void action()}
              >
                <Lock size={15} />
                {claimed ? "续期领取" : "领取任务"}
              </button>
              <div className="claim">
                <Clock size={14} />
                {claimed
                  ? "当前由你处理"
                  : t.assigned_to
                    ? "其他人员已领取或领取已过期"
                    : "任务尚未领取"}
              </div>
              {t.kind === "post" && (
                <label className="checkbox">
                  <input
                    type="checkbox"
                    checked={advertising}
                    onChange={(e) => setAd(e.target.checked)}
                  />
                  经判断属于广告内容
                </label>
              )}
              <label>
                处理原因（必填）
                <textarea
                  maxLength={400}
                  minLength={2}
                  rows={6}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder="记录判断依据，作者可能会看到该原因"
                />
              </label>
              <div className="decision-actions">
                <button
                  disabled={busy || !claimed || reason.trim().length < 2}
                  onClick={() => void action("reject")}
                >
                  驳回
                </button>
                <button
                  className="primary"
                  disabled={busy || !claimed || reason.trim().length < 2}
                  onClick={() => void action("approve")}
                >
                  审核通过
                </button>
              </div>
            </>
          ) : (
            <p className="note">本任务已结束，不能再次修改结论。</p>
          )}
          {feedback && (
            <p role="status" className="note">
              {feedback}
            </p>
          )}
        </aside>
      </div>
    </>
  );
}
