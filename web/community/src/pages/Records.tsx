import { Link } from "react-router-dom";

import { send } from "../api";

import { useData } from "../state/useData";
import { Feedback, Require, status } from "../components/ui";

export function Records({ type }: { type: string }) {
  const { data, error, loading, reload } = useData<{ items: any[] }>(
    "/" + (type === "notifications" ? "notifications" : "me/" + type),
  );
  return (
    <Require>
      <div className="page-heading">
        <h1>
          {type === "notifications"
            ? "消息通知"
            : type === "reports"
              ? "我的举报"
              : "我的申诉"}
        </h1>
        <p>只有你可以访问这些记录。</p>
      </div>
      <Feedback error={error} loading={loading} />
      {data?.items.map((item) => (
        <article className="panel" key={item.id}>
          <span className="tag">
            {status(item.status || (!item.read_at ? "未读" : "已读"))}
          </span>
          <p>{item.message || item.reason}</p>
          {item.result && <p>处理结果：{item.result}</p>}
          {item.post_id && (
            <Link to={"/posts/" + item.post_id}>查看相关内容</Link>
          )}
          {type === "notifications" && !item.read_at && (
            <button
              onClick={async () => {
                await send("/notifications/" + item.id + "/read", {});
                reload();
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
    </Require>
  );
}
