import { ReportDialog } from "../components/ReportDialog";
import React, { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Heart, Bookmark, Flag, ChevronRight } from "lucide-react";
import { api, send, media, type Post } from "../api";
import { useAuth } from "../state/auth";
import { useData } from "../state/useData";
import { Feedback, Avatar, Require, status } from "../components/ui";

export function Detail() {
  const { id } = useParams(),
    { user } = useAuth();
  const { data: p, error, loading, reload } = useData<Post>("/posts/" + id);
  const [commentPage, setCommentPage] = useState(1);
  const {
    data: comments,
    error: commentError,
    loading: commentLoading,
    reload: reloadComments,
  } = useData<{ items: any[]; has_more: boolean }>(
    "/posts/" + id + "/comments?page=" + commentPage,
  );
  const [body, setBody] = useState(""),
    [failure, fail] = useState(""),
    [busy, setBusy] = useState(false),
    [reportTarget, setReportTarget] = useState<{
      path: string;
      title: string;
    } | null>(null);
  if (!p) return <Feedback error={error} loading={loading} />;
  async function act(f: () => Promise<unknown>) {
    fail("");
    try {
      await f();
      reload();
    } catch (e) {
      fail((e as Error).message);
    }
  }
  return (
    <>
      {reportTarget && (
        <ReportDialog {...reportTarget} onClose={() => setReportTarget(null)} />
      )}
      <div className="detail-layout">
        <article className="panel detail">
          <div className="author-row">
            <Link to={"/users/" + p.author_id}>
              <Avatar name={p.author} id={p.avatar_id} />
              <b>{p.author}</b>
            </Link>
            {user && user.id !== p.author_id && (
              <button
                onClick={() =>
                  void act(() =>
                    api("/users/" + p.author_id + "/follow", {
                      method: p.following ? "DELETE" : "PUT",
                    }),
                  )
                }
              >
                {p.following ? "已关注" : "＋ 关注"}
              </button>
            )}
          </div>
          <div className="gallery">
            {p.media_ids.map((mid) => (
              <img key={mid} src={media(mid)} alt={p.title} />
            ))}
          </div>
          <span className="tag">{p.category}</span>
          {p.advertising && <span className="tag">广告</span>}
          <h1>{p.title}</h1>
          <p className="post-body">{p.body}</p>
          {p.opportunity && (
            <section className="opportunity">
              <h3>机会详情</h3>
              <dl>
                {[
                  ["发布方", p.opportunity.publisher],
                  ["地点", p.opportunity.city],
                  ["报酬", p.opportunity.compensation],
                  ["结算", p.opportunity.settlement],
                  [
                    "有效期",
                    new Date(p.opportunity.expires_at).toLocaleString(),
                  ],
                  ["联系渠道", p.opportunity.contact],
                ].map(([k, v]) => (
                  <React.Fragment key={k}>
                    <dt>{k}</dt>
                    <dd>{v}</dd>
                  </React.Fragment>
                ))}
              </dl>
              <div className="risk">{p.risk_disclosure}</div>
            </section>
          )}
          <p className="fine">
            发布于{" "}
            {p.published_at ? new Date(p.published_at).toLocaleString() : ""}
          </p>
          <div className="toolbar">
            <button
              disabled={!user}
              className={p.liked ? "selected" : ""}
              onClick={() =>
                void act(() =>
                  api("/posts/" + id + "/reactions/like", {
                    method: p.liked ? "DELETE" : "PUT",
                  }),
                )
              }
            >
              <Heart size={17} /> {p.likes}
            </button>
            <button
              disabled={!user}
              onClick={() =>
                void act(() =>
                  api("/posts/" + id + "/reactions/favorite", {
                    method: p.saved ? "DELETE" : "PUT",
                  }),
                )
              }
            >
              <Bookmark size={17} />
              {p.saved ? "已收藏" : "收藏"}
            </button>
            <button
              disabled={!user}
              onClick={() =>
                setReportTarget({
                  path: "/posts/" + id + "/reports",
                  title: "举报内容",
                })
              }
            >
              <Flag size={16} />
              举报
            </button>
          </div>
          <Feedback error={failure} />
          <h2>评论与讨论</h2>
          <Require>
            <form
              onSubmit={async (e) => {
                e.preventDefault();
                setBusy(true);
                fail("");
                try {
                  await send(
                    "/posts/" + id + "/comments",
                    { body },
                    "POST",
                    crypto.randomUUID(),
                  );
                  setBody("");
                  setCommentPage(1);
                  reloadComments();
                } catch (e) {
                  fail((e as Error).message);
                } finally {
                  setBusy(false);
                }
              }}
            >
              <textarea
                aria-label="评论内容"
                required
                maxLength={1000}
                value={body}
                onChange={(e) => setBody(e.target.value)}
                placeholder="友善交流，评论审核通过后公开"
              />
              <button className="primary" disabled={busy}>
                {busy ? "提交中…" : "提交评论"}
              </button>
            </form>
          </Require>
          <Feedback error={commentError} loading={commentLoading} />
          <div className="comments">
            {comments?.items.map((c) => (
              <div className="comment" key={c.id}>
                <Avatar name={c.author} />
                <div>
                  <b>{c.author}</b>
                  {c.status === "pending" && (
                    <span className="tag">待审核，仅自己可见</span>
                  )}
                  <p>{c.body}</p>
                  {user && c.status === "published" && (
                    <button
                      className="text"
                      onClick={() =>
                        setReportTarget({
                          path: "/comments/" + c.id + "/reports",
                          title: "举报评论",
                        })
                      }
                    >
                      举报评论
                    </button>
                  )}
                  {user?.id === c.author_id && (
                    <button
                      className="text"
                      onClick={async () => {
                        try {
                          await api("/comments/" + c.id, { method: "DELETE" });
                          reloadComments();
                        } catch (e) {
                          fail((e as Error).message);
                        }
                      }}
                    >
                      删除
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
          {!commentLoading && !commentError && !comments?.items.length && (
            <p className="muted">还没有公开评论，欢迎分享你的看法。</p>
          )}
          <div className="toolbar pagination">
            <span>评论第 {commentPage} 页</span>
            <button
              disabled={commentPage <= 1}
              onClick={() => setCommentPage(commentPage - 1)}
            >
              上一页
            </button>
            <button
              disabled={!comments?.has_more}
              onClick={() => setCommentPage(commentPage + 1)}
            >
              下一页
            </button>
          </div>
        </article>
        <aside className="panel guidance">
          <h3>一起维护社区</h3>
          <p>
            请分享真实经验，尊重不同选择。发现可疑机会，可通过举报入口反馈。
          </p>
          <Link to="/help">
            阅读社区规则 <ChevronRight size={14} />
          </Link>
        </aside>
      </div>
    </>
  );
}
