import React from "react";
export type Page = { items: any[]; page: number; has_more: boolean };
export const label = (s: string) =>
  ({
    published: "已公开",
    hidden: "已下架",
    deleted: "已删除",
    draft: "草稿",
    rejected: "已驳回",
    pending: "待审核",
    decided: "已处理",
    cancelled: "已撤回",
    resolved: "已处理",
    post: "图文 / 机会",
    comment: "评论",
    profile: "个人资料",
    approve: "通过",
    reject: "驳回",
  })[s] || s;
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
    <div className="empty">正在加载…</div>
  ) : null;
}
export function Heading({
  title,
  sub,
  children,
}: {
  title: string;
  sub: string;
  children?: React.ReactNode;
}) {
  return (
    <div className="heading">
      <div>
        <span className="eyebrow">TALKO / TRUST & SAFETY</span>
        <h1>{title}</h1>
        <p>{sub}</p>
      </div>
      {children}
    </div>
  );
}
export function Pager({
  data,
  page,
  setPage,
}: {
  data: Page | null;
  page: number;
  setPage: (p: number) => void;
}) {
  return (
    <div className="pager">
      <span>第 {page} 页 · 每页最多 40 条</span>
      <button disabled={page <= 1} onClick={() => setPage(page - 1)}>
        上一页
      </button>
      <button disabled={!data?.has_more} onClick={() => setPage(page + 1)}>
        下一页
      </button>
    </div>
  );
}

export const time = (s: string) => (s ? new Date(s).toLocaleString() : "—");
