import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { BookOpen, Heart, ArrowUpRight } from "lucide-react";
import { media, type Post } from "../api";

import { useData } from "../state/useData";
import { Feedback, Avatar, Empty, Require } from "../components/ui";
import { categories } from "../constants";
export function Card({ post: p }: { post: Post }) {
  return (
    <article className="card">
      <Link to={"/posts/" + p.id}>
        {p.media_ids?.length ? (
          <img
            className="cover"
            src={media(p.media_ids[0])}
            alt={p.title}
            loading="lazy"
          />
        ) : (
          <div className="text-cover">
            <small>{p.category}</small>
            <strong>{p.title}</strong>
            <span>TALKO / NOTES ↗</span>
          </div>
        )}
        <div className="card-content">
          {p.risk_disclosure && (
            <div className="risk-label">信息来源未核实，请谨慎甄别</div>
          )}
          <h3>{p.title}</h3>
        </div>
      </Link>
      <div className="card-meta">
        <Link to={"/users/" + p.author_id}>
          <Avatar id={p.avatar_id} name={p.author} size={25} />
          {p.author}
        </Link>
        <span>
          <Heart size={14} /> {p.likes}
        </span>
      </div>
    </article>
  );
}

export function Feed({ mode = "discover" }: { mode?: string }) {
  const [params, setParams] = useSearchParams(),
    [category, setCategory] = useState("全部"),
    [cursor, setCursor] = useState(""),
    [items, setItems] = useState<Post[]>([]);
  const q = params.get("q") || "";
  const key = mode + "|" + category + "|" + q;
  useEffect(() => {
    setCursor("");
    setItems([]);
  }, [key]);
  let path =
    "/feed?limit=18" +
    (mode === "following"
      ? "&following=1"
      : mode === "opportunity"
        ? "&type=opportunity"
        : mode === "saved"
          ? "&saved=1"
          : "") +
    (category !== "全部" ? "&category=" + encodeURIComponent(category) : "") +
    (q ? "&q=" + encodeURIComponent(q) : "") +
    (cursor ? "&cursor=" + encodeURIComponent(cursor) : "");
  const { data, error, loading } = useData<{
    items: Post[];
    next_cursor: string;
  }>(path);
  useEffect(() => {
    if (data)
      setItems((old) =>
        cursor
          ? [
              ...old,
              ...data.items.filter((p) => !old.some((x) => x.id === p.id)),
            ]
          : data.items,
      );
  }, [data]);
  const body = (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">CAMPUS / GROW TOGETHER</span>
          <h1>
            {mode === "following"
              ? "关注动态"
              : mode === "opportunity"
                ? "校园机会"
                : mode === "saved"
                  ? "我的收藏"
                  : q
                    ? "搜索：" + q
                    : "校园的每一步，都有同行的人。"}
          </h1>
          <p>
            {mode === "opportunity"
              ? "认真核验每一条机会，保护自己的信息和资金。"
              : "分享真实经历，记录学习与生活，发现新的可能。"}
          </p>
        </div>
      </div>
      {mode === "discover" && !q && (
        <div className="banner">
          <div>
            <span className="tag">写下你的第一篇校园笔记</span>
            <h2>从一个小小的分享开始。</h2>
            <p>一次面试、一段备考经历，或是校园里的普通一天。</p>
            <Link to="/compose">
              开始分享 <ArrowUpRight size={16} />
            </Link>
          </div>
          <div className="banner-art">
            <BookOpen size={76} />
            <span>
              IDEAS
              <br />
              WORTH SHARING
            </span>
          </div>
        </div>
      )}
      <div className="tabs">
        {categories.map((c) => (
          <button
            key={c}
            className={category === c ? "active" : ""}
            onClick={() => setCategory(c)}
          >
            {c}
          </button>
        ))}
        {q && <button onClick={() => setParams({})}>清除搜索</button>}
      </div>
      <Feedback error={error} loading={loading} />
      {!loading && !error && !items.length ? (
        <Empty
          title={
            mode === "following" ? "关注的作者还没有新内容" : "暂时没有匹配内容"
          }
        />
      ) : (
        <div className="cards">
          {items.map((p) => (
            <Card post={p} key={p.id} />
          ))}
        </div>
      )}
      {data?.next_cursor && !loading && (
        <button
          className="button load-more"
          onClick={() => setCursor(data.next_cursor)}
        >
          加载更多
        </button>
      )}
    </>
  );
  return ["following", "saved"].includes(mode) ? (
    <Require>{body}</Require>
  ) : (
    body
  );
}
