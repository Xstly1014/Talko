export type User = {
  profile_change?: {
    id: string;
    nickname: string;
    bio: string;
    avatar_id: string;
    status: string;
    reason: string;
  };
  id: string;
  nickname: string;
  bio: string;
  avatar_id: string;
  email: string;
  followers: number;
  following: number;
};
export type Opportunity = {
  publisher: string;
  city: string;
  compensation: string;
  settlement: string;
  expires_at: string;
  contact: string;
};
export type Post = {
  id: string;
  author_id: string;
  author: string;
  avatar_id: string;
  revision_id: string;
  type: string;
  status: string;
  title: string;
  body: string;
  category: string;
  media_ids: string[];
  opportunity?: Opportunity;
  risk_disclosure?: string;
  advertising?: boolean;
  reason: string;
  task_id: string;
  published_at: string | null;
  likes: number;
  favorites: number;
  liked: boolean;
  saved: boolean;
  following: boolean;
};
export async function api<T = unknown>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && typeof init.body === "string")
    headers.set("Content-Type", "application/json");
  const response = await fetch("/api/v1" + path, {
    ...init,
    headers,
    credentials: "same-origin",
  });
  const data = await response
    .json()
    .catch(() => ({ message: "服务器响应异常" }));
  if (!response.ok) throw new Error(data.message || "请求失败");
  return data as T;
}
export const send = (
  path: string,
  body: unknown,
  method = "POST",
  key?: string,
) =>
  api<any>(path, {
    method,
    body: JSON.stringify(body),
    headers: key ? { "Idempotency-Key": key } : undefined,
  });
export const media = (id: string) => "/api/v1/media/" + id;
