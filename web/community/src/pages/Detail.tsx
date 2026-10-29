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
  const { data: comments, reload: reloadComments } = useData<{ items: any[] }>(
    "/posts/" + id + "/comments",
  );
  const [body, setBody] = useState(""),
    [failure, fail] = useState(""),
    [busy, setBusy] = useState(false);
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
              onClick={() => {
                const reason = prompt("请说明举报原因（至少 5 字）");
                if (reason)
                  void act(() =>
                    send("/posts/" + id + "/reports", { reason }).then(() =>
                      alert("举报已提交，可在“我的举报”查看进度。"),
                    ),
                  );
              }}
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
                  {user?.id === c.author_id && (
                    <button
                      className="text"
                      onClick={async () => {
                        await api("/comments/" + c.id, { method: "DELETE" });
                        reloadComments();
                      }}
                    >
                      删除
                    </button>
                  )}
                </div>
              </div>
            ))}
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
