import React from "react";
import { Link } from "react-router-dom";
import { BookOpen } from "lucide-react";
import { media } from "../api";
import { useAuth } from "../state/auth";
export function Feedback({
  error,
  loading,
}: {
  error?: string;
  loading?: boolean;
}) {
  return error ? (
    <div className="error" role="alert">
      {error}
    </div>
  ) : loading ? (
    <div className="loading">正在加载…</div>
  ) : null;
}

export function Avatar({
  id,
  name,
  size = 32,
}: {
  id?: string;
  name: string;
  size?: number;
}) {
  return id ? (
    <img
      className="avatar"
      style={{ width: size, height: size }}
      src={media(id)}
      alt=""
    />
  ) : (
    <span className="avatar" style={{ width: size, height: size }}>
      {name.slice(0, 1)}
    </span>
  );
}

export function Empty({
  title = "暂时没有内容",
  body = "分享一段经历，让同路人看见。",
}: {
  title?: string;
  body?: string;
}) {
  return (
    <div className="empty">
      <BookOpen size={32} />
      <h3>{title}</h3>
      <p>{body}</p>
      <Link className="button" to="/compose">
        发布第一篇内容
      </Link>
    </div>
  );
}

export function Require({ children }: { children: React.ReactNode }) {
  const { user } = useAuth();
  return user ? (
    <>{children}</>
  ) : (
    <section className="panel auth-required">
      <h2>登录后继续</h2>
      <p>公开内容可以直接浏览，发布和互动需要登录。</p>
      <Link className="button primary" to="/auth">
        登录 / 注册
      </Link>
    </section>
  );
}

export function status(s: string) {
  return (
    (
      {
        draft: "草稿",
        pending: "待审核",
        published: "已发布",
        rejected: "已驳回",
        withdrawn: "已撤回",
        hidden: "已下架",
        resolved: "已处理",
        approved: "已通过",
        superseded: "已被新版本替代",
        decided: "已处理",
        cancelled: "已撤回",
      } as Record<string, string>
    )[s] || s
  );
}
