import { useState } from "react";
import { Link } from "react-router-dom";
import { RefreshCw, Check } from "lucide-react";
import { useData } from "../state";
import {
  Feedback,
  Heading,
  Pager,
  label,
  time,
  type Page,
} from "../components/ui";
export function Tasks() {
  const [status, setStatus] = useState("pending"),
    [kind, setKind] = useState(""),
    [q, setQ] = useState(""),
    [page, setPage] = useState(1);
  const { data, error, loading, reload } = useData<Page>(
    "/tasks?status=" + status + "&kind=" + kind + "&page=" + page,
  );
  const list =
    data?.items.filter((t) => (t.title + t.author + t.id).includes(q)) || [];
  return (
    <>
      <Heading
        title="内容审核"
        sub="图文、机会、评论和资料变更均按具体版本审核。"
      />
      <div className="panel">
        <div className="toolbar">
          <div className="segments">
            {["pending", "decided", "cancelled"].map((s) => (
              <button
                key={s}
                className={status === s ? "selected" : ""}
                onClick={() => {
                  setStatus(s);
                  setPage(1);
                }}
              >
                {label(s)}
              </button>
            ))}
          </div>
          <select
            aria-label="内容类型"
            value={kind}
            onChange={(e) => {
              setKind(e.target.value);
              setPage(1);
            }}
          >
            <option value="">全部类型</option>
            <option value="post">图文 / 机会</option>
            <option value="comment">评论</option>
            <option value="profile">个人资料</option>
          </select>
          <input
            aria-label="搜索审核任务"
            placeholder="搜索当前列表标题、作者或任务号"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          <button onClick={reload}>
            <RefreshCw size={15} />
          </button>
        </div>
        <Feedback error={error} loading={loading} />
        <div className="table">
          <table>
            <thead>
              <tr>
                <th>待审核对象</th>
                <th>类型</th>
                <th>作者</th>
                <th>提交时间</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {list.map((t) => (
                <tr key={t.id}>
                  <td>
                    <b>{t.title || "未命名任务"}</b>
                    <small>{t.id}</small>
                  </td>
                  <td>{label(t.kind)}</td>
                  <td>{t.author}</td>
                  <td>{time(t.created_at)}</td>
                  <td>
                    <span className="badge">{label(t.status)}</span>
                  </td>
                  <td>
                    <Link to={"/tasks/" + t.id}>查看处理 →</Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {!loading && !list.length && (
          <div className="empty">
            <Check size={28} />
            <h3>当前没有匹配任务</h3>
            <p>可以调整状态或类型查看其他记录。</p>
          </div>
        )}
        <Pager data={data} page={page} setPage={setPage} />
      </div>
    </>
  );
}
