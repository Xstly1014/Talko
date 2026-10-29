import { useState } from "react";
import { Link } from "react-router-dom";

import { api, send, type Post } from "../api";

import { useData } from "../state/useData";
import { Feedback, Empty, Require, status } from "../components/ui";

export function Mine() {
  const { data, error, loading, reload } = useData<{ items: Post[] }>(
    "/me/posts",
  );
  const [message, setMessage] = useState("");
  return (
    <Require>
      <div className="page-heading">
        <h1>我的发布</h1>
        <p>查看草稿、审核结果和内容状态。</p>
      </div>
      <Feedback error={error || message} loading={loading} />
      {data?.items.length
        ? data.items.map((p) => (
            <article className="panel own-post" key={p.id}>
              <div>
                <span className="tag">{status(p.status)}</span>
                <h3>{p.title}</h3>
                <p className="muted">
                  {p.category} · {p.reason || "尚无审核结论"}
                </p>
              </div>
              <div className="actions">
                {p.status === "pending" ? (
                  <button
                    onClick={async () => {
                      try {
                        await send("/posts/" + p.id + "/withdraw", {});
                        reload();
                      } catch (e) {
                        setMessage((e as Error).message);
                      }
                    }}
                  >
                    撤回审核
                  </button>
                ) : (
                  <Link className="button" to={"/edit/" + p.id}>
                    编辑
                  </Link>
                )}
                {p.status === "rejected" && (
                  <button
                    onClick={async () => {
                      const reason = prompt("请填写申诉理由（至少 5 字）");
                      if (!reason) return;
                      try {
                        await send("/appeals", { task_id: p.task_id, reason });
                        setMessage("申诉已提交，请查看申诉记录。");
                      } catch (e) {
                        setMessage((e as Error).message);
                      }
                    }}
                  >
                    申诉
                  </button>
                )}
                <button
                  className="danger"
                  onClick={async () => {
                    if (!confirm("删除后内容将停止展示，是否继续？")) return;
                    try {
                      await api("/posts/" + p.id, { method: "DELETE" });
                      reload();
                    } catch (e) {
                      setMessage((e as Error).message);
                    }
                  }}
                >
                  删除
                </button>
              </div>
            </article>
          ))
        : !loading && !error && <Empty />}
      <div className="toolbar">
        <Link to="/reports">我的举报</Link>
        <Link to="/appeals">我的申诉</Link>
      </div>
    </Require>
  );
}
